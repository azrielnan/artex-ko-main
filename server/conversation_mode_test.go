package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
)

func TestConversationModeAPIIsolation(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer m.Close()
	s := New(context.Background(), m, t.TempDir(), t.TempDir(), t.TempDir())
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path string, body any, out any) int {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if out != nil && rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatalf("decode %s %s response: %v (%s)", method, path, err, rec.Body.String())
			}
		}
		return rec.Code
	}

	title := fmt.Sprintf("mode-api-%d", time.Now().UnixNano())
	var pentest, reverse, dedicatedPentest db.Conversation
	if code := do(http.MethodPost, "/api/conversations", map[string]any{"agent_key": "auto", "mode": "pentest", "title": title + "-p"}, &pentest); code != http.StatusOK {
		t.Fatalf("create pentest status=%d", code)
	}
	if code := do(http.MethodPost, "/api/conversations", map[string]any{"agent_key": "reverse", "mode": "reverse", "title": title + "-r"}, &reverse); code != http.StatusOK {
		t.Fatalf("create reverse status=%d", code)
	}
	if code := do(http.MethodPost, "/api/conversations", map[string]any{"agent_key": "pentest", "mode": "pentest", "title": title + "-p-agent"}, &dedicatedPentest); code != http.StatusOK {
		t.Fatalf("create pentest agent conversation status=%d", code)
	}
	t.Cleanup(func() {
		_ = m.pg.DeleteConversation(pentest.ID)
		_ = m.pg.DeleteConversation(reverse.ID)
		_ = m.pg.DeleteConversation(dedicatedPentest.ID)
	})
	if pentest.Mode != db.ConversationModePentest || reverse.Mode != db.ConversationModeReverse {
		t.Fatalf("created modes = %q and %q", pentest.Mode, reverse.Mode)
	}

	for _, test := range []struct {
		mode string
		want int64
		miss int64
	}{{"pentest", pentest.ID, reverse.ID}, {"reverse", reverse.ID, pentest.ID}} {
		var response struct {
			Conversations []db.Conversation `json:"conversations"`
		}
		if code := do(http.MethodGet, "/api/conversations?mode="+test.mode, nil, &response); code != http.StatusOK {
			t.Fatalf("list %s status=%d", test.mode, code)
		}
		foundWant, foundOther := false, false
		for _, item := range response.Conversations {
			foundWant = foundWant || item.ID == test.want
			foundOther = foundOther || item.ID == test.miss
			if item.Mode != test.mode {
				t.Fatalf("list %s returned item mode %q", test.mode, item.Mode)
			}
		}
		if !foundWant || foundOther {
			t.Fatalf("list %s isolation failed (want found=%v, other found=%v)", test.mode, foundWant, foundOther)
		}
	}

	if code := do(http.MethodPost, "/api/conversations", map[string]any{"agent_key": "reverse", "mode": "pentest"}, nil); code != http.StatusBadRequest {
		t.Fatalf("reverse Agent in pentest mode status=%d, want 400", code)
	}
	if dedicatedPentest.Mode != db.ConversationModePentest {
		t.Fatalf("pentest Agent conversation mode = %q", dedicatedPentest.Mode)
	}
	if code := do(http.MethodPost, "/api/conversations", map[string]any{"agent_key": "auto", "mode": "reverse"}, nil); code != http.StatusBadRequest {
		t.Fatalf("non-reverse Agent in reverse mode status=%d, want 400", code)
	}
	if code := do(http.MethodGet, "/api/conversations?mode=unknown", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown mode list status=%d, want 400", code)
	}
	var defaultResponse struct {
		Conversations []db.Conversation `json:"conversations"`
	}
	if code := do(http.MethodGet, "/api/conversations", nil, &defaultResponse); code != http.StatusOK {
		t.Fatalf("default list status=%d", code)
	}
	for _, item := range defaultResponse.Conversations {
		if item.Mode != db.ConversationModePentest {
			t.Fatalf("default list returned conversation %d from mode %q", item.ID, item.Mode)
		}
	}
}
