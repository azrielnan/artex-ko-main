package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
)

func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.m.pg.SearchKnowledgeEntries(r.URL.Query().Get("q"), r.URL.Query().Get("type"), r.URL.Query().Get("status"), limit)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}

// syncNVDFeeds 使用 NVD 官方年度压缩 Feed。它比在线 API 更适合首次导入十年数据，
// 下载结果会在 HTTP 层逐年处理并写入当前数据库；不会把数百 MB 数据塞进可执行文件。
func (s *Server) syncNVDFeeds(ctx context.Context, startYear, endYear int) (int, int, error) {
	imported, feeds := 0, 0
	client := &http.Client{Timeout: 15 * time.Minute}
	cacheDir := filepath.Join(s.dataDir, "cve-cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return 0, 0, err
	}
	for year := startYear; year <= endYear; year++ {
		select {
		case <-ctx.Done():
			return imported, feeds, ctx.Err()
		default:
		}
		endpoint := fmt.Sprintf("https://nvd.nist.gov/feeds/json/cve/2.0/nvdcve-2.0-%d.json.gz", year)
		cachePath := filepath.Join(cacheDir, fmt.Sprintf("nvdcve-2.0-%d.json.gz", year))
		compressed, err := os.ReadFile(cachePath)
		if err != nil {
			resp, reqErr := nvdGet(ctx, client, endpoint)
			if reqErr != nil {
				return imported, feeds, fmt.Errorf("下载 NVD %d feed 失败: %w", year, reqErr)
			}
			if resp.StatusCode == http.StatusNotFound && year == endYear {
				resp.Body.Close()
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return imported, feeds, fmt.Errorf("NVD %d feed 返回状态 %s", year, resp.Status)
			}
			compressed, err = io.ReadAll(io.LimitReader(resp.Body, 256<<20))
			resp.Body.Close()
			if err != nil {
				return imported, feeds, fmt.Errorf("读取 NVD %d feed 失败: %w", year, err)
			}
			tmp := cachePath + ".tmp"
			if err = os.WriteFile(tmp, compressed, 0o644); err == nil {
				err = os.Rename(tmp, cachePath)
			}
			if err != nil {
				_ = os.Remove(tmp)
				return imported, feeds, fmt.Errorf("缓存 NVD %d feed 失败: %w", year, err)
			}
		}
		gz, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			_ = os.Remove(cachePath)
			return imported, feeds, fmt.Errorf("解压 NVD %d feed 失败: %w", year, err)
		}
		n, err := s.m.pg.ImportNVDReader(gz)
		gz.Close()
		if err != nil {
			return imported, feeds, fmt.Errorf("导入 NVD %d 失败: %w", year, err)
		}
		imported += n
		feeds++
	}
	return imported, feeds, nil
}
func (s *Server) createKnowledge(w http.ResponseWriter, r *http.Request) {
	var k db.KnowledgeEntry
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&k); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(k.Title) == "" || len(k.Title) > 512 || len(k.Content) > 2<<20 {
		writeErr(w, 400, "标题或正文无效")
		return
	}
	id, err := s.m.pg.UpsertKnowledgeEntry(&k)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	k.ID = id
	writeJSON(w, 201, k)
}
func (s *Server) getKnowledge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "ID 无效")
		return
	}
	k, e := s.m.pg.GetKnowledgeEntry(id)
	if e != nil {
		writeErr(w, 404, e.Error())
		return
	}
	writeJSON(w, 200, k)
}
func (s *Server) updateKnowledge(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "ID 无效")
		return
	}
	var k db.KnowledgeEntry
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&k); e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	k.ID = id
	if k.Title == "" {
		old, e := s.m.pg.GetKnowledgeEntry(id)
		if e != nil {
			writeErr(w, 404, e.Error())
			return
		}
		k = *old
	}
	if _, e := s.m.pg.UpsertKnowledgeEntry(&k); e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	out, e := s.m.pg.GetKnowledgeEntry(id)
	if e != nil {
		writeErr(w, 500, e.Error())
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) importKnowledge(w http.ResponseWriter, r *http.Request) {
	var items []db.KnowledgeEntry
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&items); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(items) > 1000 {
		writeErr(w, 400, "最多导入 1000 条")
		return
	}
	n := 0
	for i := range items {
		if _, e := s.m.pg.UpsertKnowledgeEntry(&items[i]); e != nil {
			writeErr(w, 400, e.Error())
			return
		}
		n++
	}
	writeJSON(w, 200, map[string]any{"imported": n})
}

func (s *Server) listCVEs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, e := s.m.pg.SearchCVEs(r.URL.Query().Get("q"), r.URL.Query().Get("severity"), limit)
	if e != nil {
		writeErr(w, 500, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}
func (s *Server) getCVE(w http.ResponseWriter, r *http.Request) {
	c, e := s.m.pg.GetCVE(r.PathValue("cve_id"))
	if e != nil {
		writeErr(w, 404, e.Error())
		return
	}
	writeJSON(w, 200, c)
}

func nvdGet(ctx context.Context, client *http.Client, endpoint string) (*http.Response, error) {
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "ARTEX-CVE-Cache/1.0")
		if key := strings.TrimSpace(os.Getenv("NVD_API_KEY")); key != "" {
			req.Header.Set("apiKey", key)
		}
		resp, err := client.Do(req)
		if err != nil {
			if attempt == 3 {
				return nil, err
			}
		} else if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return resp, nil
		} else {
			resp.Body.Close()
			if attempt == 3 {
				return nil, fmt.Errorf("NVD 更新被限流或服务异常，最后状态 %s", resp.Status)
			}
		}
		wait := time.Duration(3*(attempt+1)*(attempt+1)) * time.Second
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("NVD 请求重试用尽")
}

func (s *Server) importCVEs(w http.ResponseWriter, r *http.Request) {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	n, e := s.m.pg.ImportNVDJSON(b)
	if e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"imported": n})
}

// updateCVEs pulls a bounded page from NVD 2.0. It is deliberately explicit
// and capped so a UI refresh cannot accidentally start a full-database crawl.
func (s *Server) updateCVEs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Days    int    `json:"days"`
		Keyword string `json:"keyword"`
		CVEID   string `json:"cve_id"`
		Feed    bool   `json:"feed"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, 400, err.Error())
		return
	}
	if req.Days == 0 {
		req.Days = 7
	}
	if req.Days < 1 || req.Days > 3650 {
		writeErr(w, 400, "days 必须在 1 到 3650 之间")
		return
	}
	if req.Feed || req.Days >= 365 {
		// 年度 Feed 作为近十年本地基线；当前/最近更新继续走近 30 天 API，
		// 避免把当前年度尚在更新的大文件作为首次离线种子。
		endYear := time.Now().UTC().Year() - 1
		yearCount := (req.Days + 364) / 365
		if yearCount > 10 {
			yearCount = 10
		}
		startYear := endYear - yearCount + 1
		imported, feeds, err := s.syncNVDFeeds(r.Context(), startYear, endYear)
		if err != nil {
			writeErr(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"imported": imported, "feeds": feeds, "source": "nvd-yearly-feed", "start_year": startYear, "end_year": endYear})
		return
	}
	// 单个 NVD 查询最多覆盖约 120 天；长周期同步必须拆成窗口并逐页读取。
	// cve_id 查询仍保持单次请求语义，便于页面快速查看指定条目。
	if req.CVEID == "" {
		start := time.Now().UTC().AddDate(0, 0, -req.Days)
		end := time.Now().UTC()
		if strings.TrimSpace(req.Keyword) != "" {
			// 关键词同步也走同一分页循环，避免结果被截断。
		}
		imported, requests, err := s.syncNVDWindows(r.Context(), start, end, strings.TrimSpace(req.Keyword))
		if err != nil {
			writeErr(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"imported": imported, "requests": requests, "source": "nvd", "days": req.Days})
		return
	}
	q := url.Values{"resultsPerPage": {"2000"}}
	if req.CVEID != "" {
		q.Set("cveId", strings.TrimSpace(req.CVEID))
	} else {
		now := time.Now().UTC()
		q.Set("pubStartDate", now.AddDate(0, 0, -req.Days).Format("2006-01-02T15:04:05.000"+"Z"))
		q.Set("pubEndDate", now.Format("2006-01-02T15:04:05.000Z"))
		if strings.TrimSpace(req.Keyword) != "" {
			q.Set("keywordSearch", strings.TrimSpace(req.Keyword))
		}
	}
	endpoint := "https://services.nvd.nist.gov/rest/json/cves/2.0?" + q.Encode()
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := nvdGet(r.Context(), client, endpoint)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, 502, "NVD 返回状态 "+resp.Status)
		return
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	n, err := s.m.pg.ImportNVDJSON(b)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"imported": n, "source": "nvd", "query": q})
}

func (s *Server) syncNVDWindows(ctx context.Context, start, end time.Time, keyword string) (int, int, error) {
	const windowDays = 120
	const pageSize = 2000
	imported, requests := 0, 0
	client := &http.Client{Timeout: 120 * time.Second}
	for cursor := start; cursor.Before(end); {
		windowEnd := cursor.AddDate(0, 0, windowDays)
		if windowEnd.After(end) {
			windowEnd = end
		}
		for offset := 0; ; {
			q := url.Values{"resultsPerPage": {strconv.Itoa(pageSize)}, "startIndex": {strconv.Itoa(offset)},
				"pubStartDate": {cursor.Format("2006-01-02T15:04:05.000Z")}, "pubEndDate": {windowEnd.Format("2006-01-02T15:04:05.000Z")}}
			if keyword != "" {
				q.Set("keywordSearch", keyword)
			}
			resp, err := nvdGet(ctx, client, "https://services.nvd.nist.gov/rest/json/cves/2.0?"+q.Encode())
			requests++
			if err != nil {
				return imported, requests, err
			}
			b, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			status := resp.StatusCode
			resp.Body.Close()
			if readErr != nil {
				return imported, requests, readErr
			}
			if status != http.StatusOK {
				return imported, requests, fmt.Errorf("NVD 返回状态 %d", status)
			}
			n, err := s.m.pg.ImportNVDJSON(b)
			if err != nil {
				return imported, requests, err
			}
			imported += n
			var meta struct {
				TotalResults   int `json:"totalResults"`
				ResultsPerPage int `json:"resultsPerPage"`
			}
			if json.Unmarshal(b, &meta) != nil || meta.ResultsPerPage == 0 || offset+meta.ResultsPerPage >= meta.TotalResults {
				break
			}
			offset += meta.ResultsPerPage
			// 无 API key 时遵守 NVD 速率限制，避免长同步触发 403/429。
			select {
			case <-ctx.Done():
				return imported, requests, ctx.Err()
			case <-time.After(6500 * time.Millisecond):
			}
		}
		cursor = windowEnd
		select {
		case <-ctx.Done():
			return imported, requests, ctx.Err()
		case <-time.After(6500 * time.Millisecond):
		}
	}
	return imported, requests, nil
}

func (s *Server) listTunnels(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, e := s.m.pg.ListTunnels(r.URL.Query().Get("status"), limit)
	if e != nil {
		writeErr(w, 500, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}
func (s *Server) createTunnel(w http.ResponseWriter, r *http.Request) {
	var t db.Tunnel
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&t); e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	if t.Status == "" {
		t.Status = "planned"
	}
	t.SandboxRequired = true
	id, e := s.m.pg.CreateTunnel(&t)
	if e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	t.ID = id
	writeJSON(w, 201, t)
}
func (s *Server) updateTunnel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "invalid id")
		return
	}
	var req struct {
		Status   string `json:"status"`
		Approved bool   `json:"approved"`
	}
	if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
		writeErr(w, 400, e.Error())
		return
	}
	if req.Status != "planned" && req.Status != "dry_run" && req.Status != "active" && req.Status != "closed" && req.Status != "failed" {
		writeErr(w, 400, "invalid status")
		return
	}
	if e := s.m.pg.UpdateTunnelStatus(id, req.Status, req.Approved); e != nil {
		writeErr(w, 500, e.Error())
		return
	}
	t, e := s.m.pg.GetTunnel(id)
	if e != nil {
		writeErr(w, 404, e.Error())
		return
	}
	writeJSON(w, 200, t)
}

// planTunnel 只生成受控 dry-run 计划，不执行远端命令或建立网络连接。
func (s *Server) planTunnel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Target     string `json:"target"`
		Preferred  string `json:"preferred"`
		ListenAddr string `json:"listen_addr"`
		Command    string `json:"command"`
		TaskID     *int64 `json:"task_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Target) == "" {
		writeErr(w, 400, "名称和目标不能为空")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Preferred))
	if kind != "chisel" && kind != "suo5" && kind != "tcp_forward" {
		// 默认使用生态更广的 chisel；suo5 可由审批人显式选择。
		kind = "chisel"
	}
	rationale := "适合通用 TCP 转发和 SOCKS 场景；上线前应验证目标范围、监听地址和回连路径。"
	if kind == "tcp_forward" {
		rationale = "内置适配器：真实监听本机回环端口并转发到目标 host:port；部署前仍需要人工批准。"
	}
	if kind == "suo5" {
		rationale = "适合受控 HTTP 转发场景；上线前应验证目标范围、代理链路和流量审计要求。"
	}
	plan, _ := json.MarshalIndent(map[string]any{"mode": "dry_run", "tool": kind, "target": req.Target, "rationale": rationale, "requires_approval": true, "requires_sandbox": true}, "", "  ")
	t := &db.Tunnel{TaskID: req.TaskID, Name: req.Name, Kind: kind, Status: "dry_run", Target: req.Target, ListenAddr: strings.TrimSpace(req.ListenAddr), Command: strings.TrimSpace(req.Command), DryRunScript: string(plan), Evidence: json.RawMessage(plan), SandboxRequired: true, CreatedBy: "planner"}
	id, err := s.m.pg.CreateTunnel(t)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	t.ID = id
	writeJSON(w, 201, t)
}
