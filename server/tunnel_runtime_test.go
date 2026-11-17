package server

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestBuiltinTCPForwardCopiesBytes(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		for {
			c, err := target.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) { defer conn.Close(); _, _ = io.Copy(conn, conn) }(c)
		}
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &runtimeTunnel{listener: listener, cancel: cancel, done: make(chan struct{}), target: target.Addr().String()}
	m := newTunnelRuntime(nil)
	go m.acceptLoop(ctx, 1, r)
	defer func() { cancel(); _ = listener.Close(); <-r.done }()

	c, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("artex-forward-check")); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, len("artex-forward-check"))
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "artex-forward-check" {
		t.Fatalf("echo=%q", buf)
	}
}

func TestNormalizeTargetRequiresHostPort(t *testing.T) {
	if _, err := normalizeTarget("example.invalid"); err == nil {
		t.Fatal("expected host:port validation")
	}
	if got, err := normalizeTarget("127.0.0.1:443"); err != nil || got != "127.0.0.1:443" {
		t.Fatalf("got %q, err %v", got, err)
	}
}
