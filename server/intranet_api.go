package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Autumn-27/artex/db"
)

// listFootholds exposes session metadata without decrypted secrets. Actual
// registration and execution are agent tools so they pass the normal guard and
// activity ledger; this endpoint is for operator visibility only.
func (s *Server) listFootholds(w http.ResponseWriter, r *http.Request) {
	if s.sessStore == nil {
		writeErr(w, 503, "会话子系统未就绪")
		return
	}
	rows, err := s.sessStore.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, x := range rows {
		out = append(out, map[string]any{"id": x.ID, "kind": x.Kind, "url": x.URL, "lang": x.Lang, "status": x.Status, "created_by_task": x.CreatedByTask, "created_by_intent": x.CreatedByIntent, "last_beat": x.LastBeat, "created_at": x.CreatedAt})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (s *Server) listIntranetSessions(w http.ResponseWriter, r *http.Request) {
	task, _ := strconv.ParseInt(r.URL.Query().Get("task_id"), 10, 64)
	rows, err := s.m.pg.ListIntranetSessions(task)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}

func (s *Server) createIntranetSession(w http.ResponseWriter, r *http.Request) {
	var x db.IntranetSession
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&x); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if x.Status == "active" {
		x.Status = "planned"
	}
	x.CreatedBy = "ui"
	id, err := s.m.pg.CreateIntranetSession(&x)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	x.ID = id
	writeJSON(w, 201, x)
}

func (s *Server) updateIntranetSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "无效的会话 ID")
		return
	}
	var x struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !map[string]bool{"planned": true, "active": true, "dead": true, "closed": true}[x.Status] {
		writeErr(w, 400, "无效的会话状态")
		return
	}
	if err := s.m.pg.UpdateIntranetSessionStatus(id, x.Status); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": x.Status})
}

func (s *Server) listCredentials(w http.ResponseWriter, r *http.Request) {
	rows, err := s.m.pg.ListCredentials(r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": rows})
}

func (s *Server) createCredential(w http.ResponseWriter, r *http.Request) {
	var x db.Credential
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&x); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if len(x.SecretHint) > 128 {
		writeErr(w, 400, "secret_hint 过长")
		return
	}
	x.CreatedBy = "ui"
	id, err := s.m.pg.UpsertCredential(&x)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	x.ID = id
	writeJSON(w, 201, x)
}

func (s *Server) intranetTopology(w http.ResponseWriter, r *http.Request) {
	task, _ := strconv.ParseInt(r.URL.Query().Get("task_id"), 10, 64)
	if task <= 0 {
		writeErr(w, 400, "需要 task_id")
		return
	}
	assets, err := s.m.assets.QueryByTask(task, "", 500, 0)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	nodes := make([]map[string]any, 0, len(assets))
	for _, a := range assets {
		var port any
		if a.Port != nil {
			port = *a.Port
		}
		nodes = append(nodes, map[string]any{"id": a.ID, "type": a.Type, "domain": a.Domain, "ip": a.IP, "url": a.URL, "port": port})
	}
	writeJSON(w, 200, map[string]any{"task_id": task, "nodes": nodes})
}
