package server

import "testing"

func assertChineseReporterLabel(t *testing.T, label, msg string) {
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

// The reporter's display name and description are user-visible labels. Keep them
// in Chinese without changing the stable agent key, prompt, or trigger behavior.
func TestReporterSeedLabelsLocalized(t *testing.T) {
	assertChineseReporterLabel(t, "reporter_name", reporterAgentName)
	assertChineseReporterLabel(t, "reporter_description", reporterAgentDescription)
}
