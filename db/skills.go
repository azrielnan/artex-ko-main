package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// defaultSkillAgentKeys are the agents that execute or directly coordinate
// authorized security work. Skills are kept out of the reverse agent so its
// conversation stays isolated and uses only the dedicated REA skill/MCP.
var defaultSkillAgentKeys = []string{"auto", "pentest", "planner", "worker"}

// defaultSkillExclusions preserve the existing isolation for skills that are
// tied to a dedicated MCP or conversation mode.
var defaultSkillExclusions = map[string]struct{}{
	"playwright-cli":          {},
	"rea-reverse-engineering": {},
	"scopesentry":             {},
}

// DiscoverBuiltinSkillNames returns the directory names of shipped skills.
// A directory is a skill only when its SKILL.md exists; this keeps incomplete
// or locally deleted entries from becoming visible rows in the database.
func DiscoverBuiltinSkillNames(skillDir string) ([]string, error) {
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, excluded := defaultSkillExclusions[entry.Name()]; excluded {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillDir, entry.Name(), "SKILL.md")); err != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// SyncBuiltinSkillVisibility makes every shipped, general-purpose skill
// callable by the normal security agents. Inserts are idempotent and never
// re-enable an existing row, so an operator's deliberate visibility toggle is
// preserved while newly shipped skills become usable automatically.
func (d *DB) SyncBuiltinSkillVisibility(skillDir string) (int, error) {
	names, err := DiscoverBuiltinSkillNames(skillDir)
	if err != nil {
		return 0, fmt.Errorf("discover skills: %w", err)
	}
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	inserted := 0
	for _, agentKey := range defaultSkillAgentKeys {
		for _, skillName := range names {
			res, err := tx.Exec(`
INSERT INTO agent_skill_visibility(agent_id, skill_name, enabled)
SELECT id, $2, true FROM agents WHERE key=$1
ON CONFLICT (agent_id, skill_name) DO NOTHING`, agentKey, skillName)
			if err != nil {
				return 0, fmt.Errorf("skill %s → agent %s: %w", skillName, agentKey, err)
			}
			if n, err := res.RowsAffected(); err == nil {
				inserted += int(n)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}
