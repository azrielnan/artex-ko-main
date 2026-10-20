package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyConversationArtifactsIncludesTranscriptSessionAndNoa(t *testing.T) {
	dataDir := t.TempDir()
	s := &Server{m: &Manager{dir: dataDir}}
	for _, fixture := range []struct {
		path string
		data string
	}{
		{filepath.Join("transcripts", "conv-12.jsonl"), `{"type":"message"}`},
		{filepath.Join("sessions", "conv-12", "uploads", "sample.bin"), "sample"},
		{filepath.Join("noa", "chat-conv-12", "archive.json"), `{"summary":"state"}`},
	} {
		path := filepath.Join(dataDir, fixture.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(fixture.data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.copyConversationArtifacts(12, 34); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		path string
		want string
	}{
		{filepath.Join("transcripts", "conv-34.jsonl"), `{"type":"message"}`},
		{filepath.Join("sessions", "conv-34", "uploads", "sample.bin"), "sample"},
		{filepath.Join("noa", "chat-conv-34", "archive.json"), `{"summary":"state"}`},
	} {
		got, err := os.ReadFile(filepath.Join(dataDir, fixture.path))
		if err != nil {
			t.Fatalf("read copied artifact %q: %v", fixture.path, err)
		}
		if string(got) != fixture.want {
			t.Errorf("copied artifact %q = %q, want %q", fixture.path, got, fixture.want)
		}
	}
}
