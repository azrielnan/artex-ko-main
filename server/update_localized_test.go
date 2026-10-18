package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertChineseUpdateMessage(t *testing.T, label, msg string) {
	t.Helper()
	hanzi, hangul := false, false
	for _, r := range msg {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF:
			hanzi = true
		case r >= 0xAC00 && r <= 0xD7A3:
			hangul = true
		}
	}
	if !hanzi || hangul {
		t.Errorf("%s: 应为中文且不含韩文: %q", label, msg)
	}
}

// The update endpoint exposes these messages directly in the settings UI; keep them Chinese.

func TestUpdateMessageConstantsLocalized(t *testing.T) {
	assertChineseUpdateMessage(t, "updateMsgPreparing", updateMsgPreparing)
	assertChineseUpdateMessage(t, "updateMsgFailed", updateMsgFailed)
	assertChineseUpdateMessage(t, "updateMsgStaged", updateMsgStaged)
	assertChineseUpdateMessage(t, "updateErrInProgress", updateErrInProgress)
	assertChineseUpdateMessage(t, "updateErrRollbackInProgress", updateErrRollbackInProgress)

	// 형식 문자열 상수는 플레이스홀더를 채운 뒤 검사한다(%q/%s 가 치환되고 한자 0).
	notRelease := fmt.Sprintf(updateErrNotReleaseFmt, "v0.0.0-dev")
	assertChineseUpdateMessage(t, "updateErrNotReleaseFmt", notRelease)
	if !strings.Contains(notRelease, "v0.0.0-dev") {
		t.Fatalf("updateErrNotReleaseFmt: 버전 플레이스홀더가 치환되지 않았습니다: %q", notRelease)
	}
	latest := fmt.Sprintf(updateErrAlreadyLatestFmt, "v1.2.3")
	assertChineseUpdateMessage(t, "updateErrAlreadyLatestFmt", latest)
	if !strings.Contains(latest, "v1.2.3") {
		t.Fatalf("updateErrAlreadyLatestFmt: 버전 플레이스홀더가 치환되지 않았습니다: %q", latest)
	}
}

// Verify the SSE progress messages on the actual begin/finish path using a local hub.
func TestUpdateProgressMessagesLocalized(t *testing.T) {
	h := &updateHub{subs: map[chan updateProgress]struct{}{}}

	if !h.begin("v1.2.3") {
		t.Fatal("begin 은 최초 호출에서 true 여야 합니다")
	}
	cur, running := h.snapshot()
	if !running {
		t.Fatal("begin 후 running 이어야 합니다")
	}
	if cur.Message != updateMsgPreparing {
		t.Fatalf("준비 중 메시지 불일치: %q", cur.Message)
	}
	if h.begin("v1.2.4") {
		t.Fatal("진행 중이면 begin 은 false 여야 합니다")
	}

	h.finish(errors.New("다운로드 실패"))
	cur, running = h.snapshot()
	if running {
		t.Fatal("finish 후 running 이 해제돼야 합니다")
	}
	if cur.Message != updateMsgFailed {
		t.Fatalf("실패 메시지 불일치: %q", cur.Message)
	}

	h.finish(nil)
	cur, _ = h.snapshot()
	if cur.Message != updateMsgStaged {
		t.Fatalf("준비 완료 메시지 불일치: %q", cur.Message)
	}
}

// Verify the rollback-in-progress 409 response over HTTP without a database.
func TestUpdateRollbackInProgressLocalized(t *testing.T) {
	updHub.mu.Lock()
	prev := updHub.running
	updHub.running = true
	updHub.mu.Unlock()
	defer func() {
		updHub.mu.Lock()
		updHub.running = prev
		updHub.mu.Unlock()
	}()

	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/update/rollback", nil)
	rec := httptest.NewRecorder()
	s.updateRollback(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("상태 코드 409 기대, 실제 %d (본문 %s)", rec.Code, rec.Body.Bytes())
	}
	msg := decodeErrorField(t, rec.Body.Bytes())
	assertChineseUpdateMessage(t, "updateRollback 进行中 409", msg)
	if msg != updateErrRollbackInProgress {
		t.Fatalf("롤백 거부 문구 불일치: %q", msg)
	}
}
