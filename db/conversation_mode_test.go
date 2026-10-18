package db

import (
	"fmt"
	"testing"
	"time"
)

func TestConversationModesRemainIsolated(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()

	suffix := time.Now().UnixNano()
	pentest, err := d.CreateConversationInMode("auto", ConversationModePentest, fmt.Sprintf("mode-pentest-%d", suffix), nil)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := d.CreateConversationInMode("reverse", ConversationModeReverse, fmt.Sprintf("mode-reverse-%d", suffix), nil)
	if err != nil {
		_ = d.DeleteConversation(pentest.ID)
		t.Fatal(err)
	}
	legacyReverse, err := d.CreateConversation("reverse", fmt.Sprintf("mode-legacy-reverse-%d", suffix), nil)
	if err != nil {
		_ = d.DeleteConversation(pentest.ID)
		_ = d.DeleteConversation(reverse.ID)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = d.DeleteConversation(pentest.ID)
		_ = d.DeleteConversation(reverse.ID)
		_ = d.DeleteConversation(legacyReverse.ID)
	})
	if legacyReverse.Mode != ConversationModeReverse {
		t.Fatalf("legacy CreateConversation(reverse) mode = %q", legacyReverse.Mode)
	}
	if _, err := d.CreateConversationInMode("reverse", ConversationModePentest, "invalid-agent-mode", nil); err == nil {
		t.Fatal("reverse agent should not be created in pentest mode")
	}
	if _, err := d.CreateConversationInMode("auto", ConversationModeReverse, "invalid-agent-mode", nil); err == nil {
		t.Fatal("non-reverse agent should not be created in reverse mode")
	}

	for _, test := range []struct {
		mode string
		want int64
		miss int64
	}{{ConversationModePentest, pentest.ID, reverse.ID}, {ConversationModeReverse, reverse.ID, pentest.ID}} {
		rows, err := d.ListConversationsInMode(test.mode)
		if err != nil {
			t.Fatal(err)
		}
		foundWant, foundOther := false, false
		for _, row := range rows {
			foundWant = foundWant || row.ID == test.want
			foundOther = foundOther || row.ID == test.miss
			if row.Mode != test.mode {
				t.Fatalf("mode %q returned conversation %d with mode %q", test.mode, row.ID, row.Mode)
			}
		}
		if !foundWant || foundOther {
			t.Fatalf("mode %q isolation failed: wanted %d (found=%v), forbidden %d (found=%v)", test.mode, test.want, foundWant, test.miss, foundOther)
		}
	}
	if _, err := d.CreateConversationInMode("auto", "unknown", "invalid-mode", nil); err == nil {
		t.Fatal("invalid conversation mode should be rejected")
	}
}
