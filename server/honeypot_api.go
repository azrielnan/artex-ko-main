package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Autumn-27/artex/db"
)

func (s *Server) listHoneypotAssessments(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(r.URL.Query().Get("task_id"), 10, 64)
	if taskID <= 0 {
		writeErr(w, 400, "需要有效的 task_id")
		return
	}
	min, _ := strconv.ParseFloat(r.URL.Query().Get("min_score"), 64)
	rows, err := s.m.pg.ListHoneypotAssessments(taskID, min)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}

// scanHoneypots 使用当前资产中的 banner、页面标题、正文和证书元数据做静态识别，
// 结果写入数据库并返回。它不访问目标，不发送探测流量。
func (s *Server) scanHoneypots(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID int64 `json:"task_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || req.TaskID <= 0 {
		writeErr(w, 400, "需要有效的 task_id")
		return
	}
	assets, err := s.m.assets.QueryByTask(req.TaskID, "", 1000, 0)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	count := 0
	for _, asset := range assets {
		score, evidence := db.EvaluateAssetHoneypot(asset)
		if score <= 0 {
			continue
		}
		products := make([]string, 0, len(evidence))
		for _, item := range evidence {
			if p := strings.Index(item, "("); p > 0 {
				body := item[p+1:]
				if comma := strings.Index(body, ","); comma > 0 {
					products = append(products, body[:comma])
				}
			}
		}
		ev, _ := json.Marshal(evidence)
		if err := s.m.pg.UpsertHoneypotAssessment(&db.HoneypotAssessment{AssetID: asset.ID, TaskID: req.TaskID, Score: score, Products: products, Evidence: ev}); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		count++
	}
	rows, err := s.m.pg.ListHoneypotAssessments(req.TaskID, 0)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"scanned": len(assets), "matched": count, "items": rows})
}
