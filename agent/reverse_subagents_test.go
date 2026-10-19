package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/permission"
	actool "github.com/Autumn-27/norma/tool"
)

func TestReverseSubagentDefinitionsAreIsolatedRoles(t *testing.T) {
	coordination := reverseCoordinationBlock()
	for _, marker := range []string{"reverse-worker", "reverse-regression", "reverse-supervisor", "安全测试上下文隔离"} {
		if !strings.Contains(coordination, marker) {
			t.Fatalf("runtime coordination block missing %q", marker)
		}
	}
	defs := reverseSubagentDefinitions()
	for _, key := range []string{"reverse-worker", "reverse-regression", "reverse-supervisor"} {
		def, ok := defs[key]
		if !ok {
			t.Fatalf("missing reverse subagent %q", key)
		}
		if len(def.Prompt) == 0 || def.MaxTurns <= 0 {
			t.Fatalf("subagent %q is not runnable: %+v", key, def)
		}
	}
}

func TestReverseSubagentToolFilterExcludesPentestSurface(t *testing.T) {
	tools := []actool.CoreTool{
		stubCoreTool{name: "Read"},
		stubCoreTool{name: "Grep"},
		stubCoreTool{name: "mcp__REA__analyze"},
		stubCoreTool{name: "report_finding"},
		stubCoreTool{name: "list_assets"},
	}
	got := reverseSubagentToolNames(tools)
	want := map[string]bool{"Read": true, "Grep": true, "mcp__REA__analyze": true}
	if len(got) != len(want) {
		t.Fatalf("allowed tools = %v, want only read/REA tools", got)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unsafe tool leaked into reverse child: %q", name)
		}
	}
}

type stubCoreTool struct{ name string }

func (s stubCoreTool) Name() string                           { return s.name }
func (s stubCoreTool) Description() string                    { return "test" }
func (s stubCoreTool) Prompt() string                         { return "" }
func (s stubCoreTool) InputSchema() map[string]any            { return map[string]any{"type": "object"} }
func (s stubCoreTool) IsReadOnly(json.RawMessage) bool        { return true }
func (s stubCoreTool) IsConcurrencySafe(json.RawMessage) bool { return true }
func (s stubCoreTool) CheckPermissions(context.Context, json.RawMessage, permission.Context) permission.Decision {
	return permission.Allowed()
}
func (s stubCoreTool) Call(context.Context, json.RawMessage, *actool.ToolContext) (actool.Result, error) {
	return actool.Text("ok"), nil
}
