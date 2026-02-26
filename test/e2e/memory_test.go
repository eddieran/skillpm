package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMemoryLifecycle exercises the full memory subsystem through the CLI:
// enable → observe → events → stats → context → scores → working-set →
// rate → feedback → consolidate → recommend → purge.
func TestMemoryLifecycle(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	// Create agent dirs so adapters are detected.
	agentDirs := map[string]string{
		"claude": filepath.Join(home, ".claude", "skills"),
		"cursor": filepath.Join(home, ".cursor", "skills"),
	}
	for _, dir := range agentDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir agent dir: %v", err)
		}
	}

	// Create a fake skill for observation detection.
	skillDir := filepath.Join(agentDirs["claude"], "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nA test."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Run doctor to initialise state.
	runCLI(t, bin, env, "doctor")

	// 1. Enable memory.
	t.Run("enable", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "enable")
		assertContains(t, out, "enabled")
	})

	// 2. Observe — triggers mtime scan.
	t.Run("observe", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "observe")
		// Should detect the test-skill via mtime.
		assertContains(t, out, "test-skill")
	})

	// 3. Events — query eventlog.
	t.Run("events", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "events")
		assertContains(t, out, "test-skill")
	})

	// 4. Stats — aggregate usage stats.
	t.Run("stats", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "stats")
		assertContains(t, out, "test-skill")
	})

	// 5. Context — detect project context.
	// Run from the repo root so it detects go.mod.
	t.Run("context", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "context")
		// Output should include some context detection (possibly empty for temp dir).
		if out == "" {
			t.Fatalf("context output is empty")
		}
	})

	// 6. Scores — compute activation scores.
	t.Run("scores", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "scores")
		assertContains(t, out, "test-skill")
	})

	// 7. Working set.
	t.Run("working-set", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "working-set")
		// Should include test-skill since it's the only skill and was recently accessed.
		assertContains(t, out, "test-skill")
	})

	// 8. Explain — explain scoring for a skill.
	t.Run("explain", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "explain", "test-skill")
		assertContains(t, out, "test-skill")
	})

	// 9. Rate — explicit feedback.
	t.Run("rate", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "rate", "test-skill", "5")
		assertContains(t, strings.ToLower(out), "rated")
	})

	// 10. Feedback — query signals.
	t.Run("feedback", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "feedback")
		assertContains(t, out, "test-skill")
	})

	// 11. Consolidate.
	t.Run("consolidate", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "consolidate")
		assertContains(t, out, "skills_evaluated")
	})

	// 12. Recommend.
	t.Run("recommend", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "recommend")
		// Output can be empty (no recommendations) or have content — just verify it doesn't error.
		_ = out
	})

	// 13. Disable.
	t.Run("disable", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "disable")
		assertContains(t, out, "disabled")
	})

	// 14. Re-enable then purge.
	t.Run("purge", func(t *testing.T) {
		runCLI(t, bin, env, "memory", "enable")
		out := runCLI(t, bin, env, "memory", "purge")
		assertContains(t, strings.ToLower(out), "purge")
	})
}

// TestMemoryAdaptiveInject tests the --adaptive flag on inject.
func TestMemoryAdaptiveInject(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	// Set up agents.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a bare repo with skills.
	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# Demo\nA demo skill."},
	})

	// Install a skill.
	runCLI(t, bin, env, "--scope", "global", "install", repoURL+"/tree/main/skills/demo", "--force")

	// Enable memory.
	runCLI(t, bin, env, "memory", "enable")

	// Enable adaptive inject.
	runCLI(t, bin, env, "memory", "set-adaptive", "true")

	// Run observe to build baseline.
	runCLI(t, bin, env, "memory", "observe")

	// Run inject with --adaptive flag.
	out := runCLI(t, bin, env, "--scope", "global", "inject", "--agent", "claude", "--adaptive")
	_ = out // Just verify it doesn't error.
}

// TestMemoryJSONOutput verifies memory commands produce valid JSON with --json.
func TestMemoryJSONOutput(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	// Create claude skills dir.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Doctor + enable.
	runCLI(t, bin, env, "doctor")
	runCLI(t, bin, env, "memory", "enable")

	jsonCmds := [][]string{
		{"memory", "events"},
		{"memory", "stats"},
		{"memory", "scores"},
		{"memory", "working-set"},
		{"memory", "feedback"},
	}

	for _, args := range jsonCmds {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			fullArgs := append([]string{"--json"}, args...)
			out := runCLI(t, bin, env, fullArgs...)
			out = strings.TrimSpace(out)
			if out == "" || out == "null" {
				return // empty JSON output is valid for no data
			}
			if !json.Valid([]byte(out)) {
				t.Fatalf("invalid JSON output for %v:\n%s", args, out)
			}
		})
	}
}

// TestMemoryDoesNotAffectExistingFlows verifies that enabling memory
// does not break existing install/inject/sync flows.
func TestMemoryDoesNotAffectExistingFlows(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	// Create agent dirs.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Enable memory + adaptive.
	runCLI(t, bin, env, "doctor")
	runCLI(t, bin, env, "memory", "enable")
	runCLI(t, bin, env, "memory", "set-adaptive", "true")

	// Install + inject + list should all work normally.
	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"test-mem": {"SKILL.md": "# Test Mem\nTest skill for memory regression."},
	})

	runCLI(t, bin, env, "--scope", "global", "install", repoURL+"/tree/main/skills/test-mem", "--force")
	runCLI(t, bin, env, "--scope", "global", "inject", "--agent", "claude")

	out := runCLI(t, bin, env, "--scope", "global", "list")
	assertContains(t, out, "test-mem")

	// Verify the SKILL.md was actually injected.
	target := filepath.Join(claudeSkills, "test-mem", "SKILL.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("SKILL.md not injected at %s: %v", target, err)
	}

	// Sync should work.
	runCLI(t, bin, env, "--scope", "global", "sync", "--dry-run")

	// Uninstall should work.
	runCLI(t, bin, env, "--scope", "global", "uninstall", "test-mem")
}
