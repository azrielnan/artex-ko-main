package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Autumn-27/artex/db"
)

func (s *Server) deployTunnelRuntime(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "invalid id")
		return
	}
	t, err := s.m.pg.GetTunnel(id)
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	if t.Kind == "" {
		writeErr(w, 400, "隧道类型不能为空")
		return
	}
	if k := strings.ToLower(strings.TrimSpace(t.Kind)); k != "chisel" && k != "suo5" {
		t.Command = "builtin_tcp_forward " + t.Target
	}
	addr, err := s.tunnelRuntime.deploy(r.Context(), t)
	if err != nil {
		writeErr(w, 409, err.Error())
		return
	}
	t, _ = s.m.pg.GetTunnel(id)
	writeJSON(w, 200, map[string]any{"ok": true, "tunnel": t, "listen_addr": addr, "adapter": strings.ToLower(strings.TrimSpace(t.Kind))})
}

func (s *Server) healthTunnelRuntime(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "invalid id")
		return
	}
	t, err := s.m.pg.GetTunnel(id)
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	if err = s.tunnelRuntime.health(r.Context(), t); err != nil {
		writeErr(w, 503, err.Error())
		return
	}
	t, _ = s.m.pg.GetTunnel(id)
	writeJSON(w, 200, map[string]any{"ok": true, "tunnel": t})
}

func (s *Server) teardownTunnelRuntime(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "invalid id")
		return
	}
	t, err := s.m.pg.GetTunnel(id)
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	if err = s.tunnelRuntime.teardown(r.Context(), t, "closed"); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	t, _ = s.m.pg.GetTunnel(id)
	writeJSON(w, 200, map[string]any{"ok": true, "tunnel": t})
}

// tunnelRuntime owns listeners and external adapter processes. Built-in TCP
// forwarding and configured chisel/suo5 processes share the same lifecycle so
// persisted state reflects an actually bound endpoint or a real process PID.
type tunnelRuntime struct {
	mu    sync.Mutex
	items map[int64]*runtimeTunnel
	pg    *db.DB
}

type runtimeTunnel struct {
	listener net.Listener
	cancel   context.CancelFunc
	done     chan struct{}
	target   string
	cmd      *exec.Cmd
	adapter  string
}

func newTunnelRuntime(pg *db.DB) *tunnelRuntime {
	return &tunnelRuntime{items: make(map[int64]*runtimeTunnel), pg: pg}
}

// markOrphans makes a restart honest: process-owned listeners cannot survive
// an ARTEX restart, so persisted active rows must not remain active.
func (m *tunnelRuntime) markOrphans() {
	if m.pg == nil {
		return
	}
	rows, err := m.pg.ListTunnels("active", 500)
	if err != nil {
		return
	}
	for _, t := range rows {
		b, _ := json.Marshal(map[string]any{"adapter": "builtin_tcp_forward", "health": "orphaned_after_restart"})
		_ = m.pg.UpdateTunnelRuntime(t.ID, "failed", "", t.Command, b)
	}
}

func (m *tunnelRuntime) deploy(ctx context.Context, t *db.Tunnel) (string, error) {
	if t == nil || t.ID <= 0 {
		return "", fmt.Errorf("隧道记录不存在")
	}
	if k := strings.ToLower(strings.TrimSpace(t.Kind)); k == "chisel" || k == "suo5" {
		return m.deployExternal(ctx, t, k)
	}
	if !t.Approved {
		return "", fmt.Errorf("隧道尚未批准，先将 approved=true")
	}
	if strings.ToLower(strings.TrimSpace(t.Kind)) != "tcp_forward" {
		return "", fmt.Errorf("不支持的隧道适配器 %q；可部署类型是 tcp_forward、chisel、suo5", t.Kind)
	}
	target, err := normalizeTarget(t.Target)
	if err != nil {
		return "", err
	}
	listen := strings.TrimSpace(t.ListenAddr)
	if listen == "" {
		listen = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1") {
		return "", fmt.Errorf("内置 TCP 转发只允许监听本机回环地址")
	}
	m.mu.Lock()
	if old := m.items[t.ID]; old != nil {
		m.mu.Unlock()
		return "", fmt.Errorf("隧道已经运行")
	}
	m.mu.Unlock()
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return "", fmt.Errorf("监听 %s 失败: %w", listen, err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	r := &runtimeTunnel{listener: ln, cancel: cancel, done: make(chan struct{}), target: target, adapter: "builtin_tcp_forward"}
	m.mu.Lock()
	m.items[t.ID] = r
	m.mu.Unlock()
	go m.acceptLoop(runCtx, t.ID, r)
	if err := m.writeState(t, "active", true, ln.Addr().String(), map[string]any{"adapter": "builtin_tcp_forward", "listen_addr": ln.Addr().String(), "target": target}); err != nil {
		_ = m.teardown(context.Background(), t, "failed")
		return "", err
	}
	return ln.Addr().String(), nil
}

// deployExternal starts a configured chisel/suo5 process and keeps its PID in
// the runtime record. A loopback listen address is mandatory so health checks
// can prove that the process actually bound its endpoint.
func (m *tunnelRuntime) deployExternal(ctx context.Context, t *db.Tunnel, kind string) (string, error) {
	if !t.Approved {
		return "", fmt.Errorf("隧道尚未批准，先将 approved=true")
	}
	listen := strings.TrimSpace(t.ListenAddr)
	if listen == "" {
		return "", fmt.Errorf("%s 需要 listen_addr 用于健康检查", kind)
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1") {
		return "", fmt.Errorf("外部隧道只允许监听本机回环地址")
	}
	argv := strings.Fields(t.Command)
	if len(argv) == 0 {
		return "", fmt.Errorf("%s 需要 command（完整客户端参数）", kind)
	}
	exe := argv[0]
	resolved, lookErr := exec.LookPath(exe)
	if lookErr != nil {
		if p := os.Getenv("ARTEX_" + strings.ToUpper(kind) + "_PATH"); p != "" {
			resolved = p
		} else {
			return "", fmt.Errorf("未找到 %s 二进制：设置 ARTEX_%s_PATH 或把 %s 放入 PATH", kind, strings.ToUpper(kind), filepath.Base(exe))
		}
	}
	if !strings.Contains(strings.ToLower(filepath.Base(resolved)), kind) {
		return "", fmt.Errorf("command 可执行文件不是 %s：%s", kind, resolved)
	}
	cmdCtx, cancelCmd := context.WithCancel(context.Background())
	cmd := exec.CommandContext(cmdCtx, resolved, argv[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		cancelCmd()
		return "", fmt.Errorf("启动 %s 失败: %w", kind, err)
	}
	r := &runtimeTunnel{cancel: cancelCmd, done: make(chan struct{}), cmd: cmd, adapter: kind, target: t.Target}
	m.mu.Lock()
	if old := m.items[t.ID]; old != nil {
		m.mu.Unlock()
		_ = cmd.Process.Kill()
		cancelCmd()
		return "", fmt.Errorf("隧道已经运行")
	}
	m.items[t.ID] = r
	m.mu.Unlock()
	go func() {
		err := cmd.Wait()
		close(r.done)
		m.mu.Lock()
		_, still := m.items[t.ID]
		m.mu.Unlock()
		if still {
			status := "failed"
			if err == nil {
				status = "closed"
			}
			_ = m.writeState(t, status, t.Approved, listen, map[string]any{"adapter": kind, "pid": cmd.Process.Pid, "exit": fmt.Sprint(err)})
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, e := net.DialTimeout("tcp", listen, 300*time.Millisecond)
		if e == nil {
			_ = c.Close()
			_ = m.writeState(t, "active", true, listen, map[string]any{"adapter": kind, "pid": cmd.Process.Pid, "listen_addr": listen, "health": "bound"})
			return listen, nil
		}
		select {
		case <-r.done:
			return "", fmt.Errorf("%s 在绑定监听地址前退出", kind)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	_ = m.teardown(context.Background(), t, "failed")
	return "", fmt.Errorf("%s 未在 5 秒内监听 %s", kind, listen)
}

func (m *tunnelRuntime) acceptLoop(ctx context.Context, id int64, r *runtimeTunnel) {
	defer close(r.done)
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			return
		}
		go m.forward(ctx, conn, r.target)
	}
}

func (m *tunnelRuntime) forward(ctx context.Context, src net.Conn, target string) {
	defer src.Close()
	dialer := net.Dialer{Timeout: 10 * time.Second}
	dst, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return
	}
	defer dst.Close()
	ch := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(dst, src); _ = dst.Close(); ch <- struct{}{} }()
	go func() { _, _ = io.Copy(src, dst); _ = src.Close(); ch <- struct{}{} }()
	<-ch
}

func (m *tunnelRuntime) health(ctx context.Context, t *db.Tunnel) error {
	m.mu.Lock()
	r := m.items[t.ID]
	m.mu.Unlock()
	if r == nil {
		return fmt.Errorf("隧道未运行")
	}
	if r.cmd != nil {
		if r.cmd.ProcessState != nil && r.cmd.ProcessState.Exited() {
			return fmt.Errorf("外部隧道进程已退出")
		}
		return m.writeState(t, "active", true, t.ListenAddr, map[string]any{"adapter": r.adapter, "pid": r.cmd.Process.Pid, "health": "process_alive"})
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", r.target)
	if err != nil {
		_ = m.writeState(t, "failed", true, r.listener.Addr().String(), map[string]any{"adapter": "builtin_tcp_forward", "target": r.target, "health_error": err.Error()})
		return err
	}
	_ = conn.Close()
	return m.writeState(t, "active", true, r.listener.Addr().String(), map[string]any{"adapter": "builtin_tcp_forward", "listen_addr": r.listener.Addr().String(), "target": r.target, "health": "ok"})
}

func (m *tunnelRuntime) teardown(ctx context.Context, t *db.Tunnel, status string) error {
	m.mu.Lock()
	r := m.items[t.ID]
	delete(m.items, t.ID)
	m.mu.Unlock()
	if r != nil {
		r.cancel()
		if r.listener != nil {
			_ = r.listener.Close()
		}
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		select {
		case <-r.done:
		case <-time.After(2 * time.Second):
		}
	}
	return m.writeState(t, status, t.Approved, "", map[string]any{"adapter": "builtin_tcp_forward", "teardown": true})
}

func (m *tunnelRuntime) writeState(t *db.Tunnel, status string, approved bool, listenAddr string, state map[string]any) error {
	b, _ := json.Marshal(state)
	t.Status, t.Approved, t.Evidence = status, approved, b
	if m.pg == nil {
		return fmt.Errorf("数据库未就绪")
	}
	return m.pg.UpdateTunnelRuntime(t.ID, status, listenAddr, t.Command, b)
}

func normalizeTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("目标地址不能为空")
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil || host == "" || port == "" {
		return "", fmt.Errorf("目标地址必须是 host:port")
	}
	return net.JoinHostPort(host, port), nil
}
