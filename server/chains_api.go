package server

import (
	"encoding/json"
	"net/http"

	"github.com/Autumn-27/artex/agent"
)

// listChains returns the same guide table that is injected into worker prompts.
func (s *Server) listChains(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": agent.ListChainGuides()})
}
