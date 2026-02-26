package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/store"
)

func TestInstallRollbackOnInjectedCommitFailure(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})
	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)

	stateRoot := filepath.Join(home, ".skillpm")
	before, err := store.LoadState(stateRoot)
	if err != nil {
		t.Fatalf("load state before failed: %v", err)
	}
	if len(before.Installed) != 1 {
		t.Fatalf("expected one installed skill before failure")
	}
	beforeVersion := before.Installed[0].ResolvedVersion

	out := runCLIExpectFailWithEnv(t, bin, env, map[string]string{"SKILLPM_TEST_FAIL_INSTALL_COMMIT": "1"},
		"--config", cfgPath, "install", "local/demo@2.0.0", "--lockfile", lockPath)
	assertContains(t, out, "INS_TEST_FAIL_COMMIT")

	after, err := store.LoadState(stateRoot)
	if err != nil {
		t.Fatalf("load state after failed: %v", err)
	}
	if len(after.Installed) != 1 {
		t.Fatalf("expected one installed skill after rollback")
	}
	if after.Installed[0].ResolvedVersion != beforeVersion {
		t.Fatalf("expected rollback to preserve version %s, got %s", beforeVersion, after.Installed[0].ResolvedVersion)
	}
}

func TestInjectRollbackOnInjectedFailure(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo":  {"SKILL.md": "# demo\nDemo skill"},
		"alpha": {"SKILL.md": "# alpha\nAlpha skill"},
	})
	// Create claude adapter dir so it's detected
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("create claude dir failed: %v", err)
	}

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "doctor")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "local/alpha", "--lockfile", lockPath)
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "claude", "local/demo")

	out := runCLIExpectFailWithEnv(t, bin, env, map[string]string{"SKILLPM_TEST_FAIL_INJECT_AFTER_WRITE": "1"},
		"--config", cfgPath, "inject", "--agent", "claude", "local/alpha")
	assertContains(t, out, "ADP_TEST_FAIL_INJECT")

	injectedPath := filepath.Join(claudeDir, "skillpm", "injected.toml")
	blob, err := os.ReadFile(injectedPath)
	if err != nil {
		t.Fatalf("read injected state failed: %v", err)
	}
	text := string(blob)
	if !strings.Contains(text, "local/demo") {
		t.Fatalf("expected injected state to preserve previous skill")
	}
	if strings.Contains(text, "local/alpha") {
		t.Fatalf("expected rollback to remove failed injected skill")
	}
}
