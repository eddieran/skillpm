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
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	// Create agent dirs so adapters are detected.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}

	// Install a real skill via source so it appears in the manifest.
	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"test-skill": {"SKILL.md": "# Test Skill\nA test skill."},
	})
	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "doctor")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/test-skill")
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "claude")

	// 1. Enable memory.
	t.Run("enable", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "enable")
		assertContains(t, out, "enabled")
	})

	// 2. Observe — triggers mtime scan; output is event count.
	t.Run("observe", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "observe")
		// Observe outputs "observed N events"; N should be > 0 because
		// the inject above touched SKILL.md files.
		assertContains(t, out, "observed")
		if strings.Contains(out, "observed 0") {
			t.Fatalf("expected at least 1 event, got: %s", out)
		}
	})

	// 3. Events — query eventlog (JSON has skill refs).
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
	t.Run("context", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "context")
		if out == "" {
			t.Fatalf("context output is empty")
		}
	})

	// 6. Scores — compute activation scores. The installed SkillRef is
	// "local/test-skill" (source prefix + skill name).
	t.Run("scores", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "scores")
		assertContains(t, out, "local/test-skill")
	})

	// 7. Working set — may or may not contain the skill depending on score.
	// Just verify the command runs and produces valid JSON.
	t.Run("working-set", func(t *testing.T) {
		out := runCLI(t, bin, env, "--json", "memory", "working-set")
		out = strings.TrimSpace(out)
		if out != "null" && out != "" && out != "[]" {
			if !json.Valid([]byte(out)) {
				t.Fatalf("invalid JSON: %s", out)
			}
		}
	})

	// 8. Explain.
	t.Run("explain", func(t *testing.T) {
		out := runCLI(t, bin, env, "memory", "explain", "local/test-skill")
		assertContains(t, out, "local/test-skill")
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
		_ = out // May be empty; just verify no error.
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
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	// Set up agent dir.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Install a skill via source (the proper way).
	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# Demo\nA demo skill."},
	})
	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "doctor")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo")

	// Enable memory + adaptive.
	runCLI(t, bin, env, "memory", "enable")
	runCLI(t, bin, env, "memory", "set-adaptive", "on")

	// Observe to build baseline.
	runCLI(t, bin, env, "memory", "observe")

	// Inject with --adaptive flag.
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
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	// Create agent dirs.
	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Doctor + enable memory + adaptive.
	runCLI(t, bin, env, "--config", cfgPath, "doctor")
	runCLI(t, bin, env, "memory", "enable")
	runCLI(t, bin, env, "memory", "set-adaptive", "on")

	// Install + inject + list should all work normally.
	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"test-mem": {"SKILL.md": "# Test Mem\nTest skill for memory regression."},
	})

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/test-mem")
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "claude")

	out := runCLI(t, bin, env, "--scope", "global", "list")
	assertContains(t, out, "test-mem")

	// Verify the SKILL.md was actually injected.
	skillMdPath := filepath.Join(claudeSkills, "test-mem", "SKILL.md")
	data, err := os.ReadFile(skillMdPath)
	if err != nil {
		t.Fatalf("read injected SKILL.md failed: %v", err)
	}
	assertContains(t, string(data), "Test Mem")

	// Sync dry-run should work.
	runCLI(t, bin, env, "--config", cfgPath, "--scope", "global", "sync", "--dry-run")

	// Uninstall should work.
	runCLI(t, bin, env, "--config", cfgPath, "uninstall", "local/test-mem")
}
