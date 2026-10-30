package db

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDiscoverBuiltinSkillNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"z-skill", "a-skill", "playwright-cli", "rea-reverse-engineering", "scopesentry"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if name != "z-skill" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte("---\nname: z-skill\n---\nbody"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a-skill", "SKILL.md"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverBuiltinSkillNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a-skill", "z-skill"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovered skills = %v, want %v", got, want)
	}
}

func TestSyncBuiltinSkillVisibilityAddsOnceAndPreservesOptOut(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()

	name := fmt.Sprintf("t-runtime-skill-%d", time.Now().UnixNano())
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	agent, err := d.GetAgentByKey("pentest")
	if err != nil || agent == nil {
		t.Fatalf("pentest agent: agent=%v err=%v", agent, err)
	}
	t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM agent_skill_visibility WHERE skill_name=$1`, name) })

	if n, err := d.SyncBuiltinSkillVisibility(filepath.Dir(dir)); err != nil || n == 0 {
		t.Fatalf("first sync inserted %d rows, err=%v", n, err)
	}
	if err := d.ToggleSkillVisibility(agent.ID, name, false); err != nil {
		t.Fatal(err)
	}
	if n, err := d.SyncBuiltinSkillVisibility(filepath.Dir(dir)); err != nil || n != 0 {
		t.Fatalf("second sync inserted %d rows, err=%v; existing rows must not be rewritten", n, err)
	}
	names, err := d.AgentSkillNames(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range names {
		if got == name {
			t.Fatal("a deliberately hidden skill was re-enabled by startup sync")
		}
	}
	var enabled bool
	if err := d.QueryRow(`SELECT enabled FROM agent_skill_visibility WHERE agent_id=$1 AND skill_name=$2`, agent.ID, name).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("the explicit opt-out row was not retained")
	}
}
