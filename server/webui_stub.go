//go:build !embedui

package server

import "net/http"

// webuiHandler is the no-embed stub (default build). The frontend is NOT bundled
// into this binary — run it separately with `cd web && npm run dev` during
// development. Build the bundled single-binary with:
//
//	cd web && npm run build:static     # produces web/out
//	cp -r web/out server/webui/dist    # (or use the build script)
//	go build -tags embedui ./cmd/artex
func (s *Server) webuiHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "此二进制未内置前端（开发环境请运行 next dev；发布版本请使用 -tags embedui 构建）。", http.StatusNotFound)
	})
}
