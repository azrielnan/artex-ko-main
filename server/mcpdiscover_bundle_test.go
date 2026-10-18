package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Autumn-27/artex/db"
)

func TestResolveBundledREAMCP(t *testing.T) {
	base := t.TempDir()
	bin := filepath.Join(base, "runtime", "rea", "node.exe")
	entry := filepath.Join(base, "runtime", "rea", "node_modules", "rea-agents", "scripts", "rea.mjs")
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defaultServer := &db.MCPServer{
		Name:      "REA",
		Transport: "stdio",
		Command:   "npx",
		Args:      json.RawMessage(`["-y","rea-agents@6.5.0","mcp"]`),
	}

	command, args := resolveBundledREAMCP(defaultServer, base, "windows")
	if command != bin || !reflect.DeepEqual(args, []string{entry, "mcp"}) {
		t.Fatalf("bundled command = %q %#v, want %q %#v", command, args, bin, []string{entry, "mcp"})
	}

	custom := *defaultServer
	custom.Command = "custom-node"
	custom.Args = json.RawMessage(`["custom-server"]`)
	command, args = resolveBundledREAMCP(&custom, base, "windows")
	if command != "custom-node" || !reflect.DeepEqual(args, []string{"custom-server"}) {
		t.Fatalf("custom command was changed: %q %#v", command, args)
	}

	command, args = resolveBundledREAMCP(defaultServer, t.TempDir(), "windows")
	if command != "npx" || !reflect.DeepEqual(args, []string{"-y", "rea-agents@6.5.0", "mcp"}) {
		t.Fatalf("missing bundle must preserve the npx fallback: %q %#v", command, args)
	}
}
