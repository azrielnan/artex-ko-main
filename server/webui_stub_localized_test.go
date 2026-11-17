//go:build !embedui

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWebUIStubErrorLocalized guards the no-embed stub's HTTP response and keeps
// the fallback message in Simplified Chinese.
func TestWebUIStubErrorLocalized(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	(&Server{}).webuiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("상태 코드가 404 가 아닙니다: %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if strings.ContainsAny(body, "가나다라마바사아자차카타파하") {
		t.Fatalf("响应中不应包含韩文: %q", body)
	}
	// 명령 참조는 원문 그대로 보존되어야 한다(번역 대상 아님).
	for _, want := range []string{"前端", "next dev", "-tags embedui"} {
		if !strings.Contains(body, want) {
			t.Fatalf("명령 참조 %q 가 응답에 없습니다: %q", want, body)
		}
	}
}
