package agent

import (
	"strings"
	"testing"
)

func TestListChainGuidesMatchesInjectedScenarios(t *testing.T) {
	guides := ListChainGuides()
	if len(guides) != 9 {
		t.Fatalf("got %d guide cards, want all 9 injected scenarios", len(guides))
	}
	for _, guide := range guides {
		if guide.Name == "" || guide.Summary == "" || len(guide.Questions) == 0 || len(guide.Turns) == 0 {
			t.Errorf("incomplete guide surfaced to UI: %+v", guide)
		}
	}
	if got := IntentChainBlock("Web API SSRF"); !strings.Contains(got, "web 场景反问") {
		t.Fatalf("worker injection is not reflected in guide source: %q", got)
	}
}
