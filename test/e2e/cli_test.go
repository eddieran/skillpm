package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIEndToEndLocalFlow(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	goModCache := filepath.Join(os.TempDir(), "skillpm-gomodcache")
	goCache := filepath.Join(os.TempDir(), "skillpm-gocache")
	if err := os.MkdirAll(goModCache, 0o755); err != nil {
		t.Fatalf("create mod cache failed: %v", err)
	}
	if err := os.MkdirAll(goCache, 0o755); err != nil {
		t.Fatalf("create go cache failed: %v", err)
	}

	env := append(os.Environ(),
		"HOME="+home,
		"OPENCLAW_STATE_DIR="+filepath.Join(home, "openclaw-state"),
		"OPENCLAW_CONFIG_PATH="+filepath.Join(home, "openclaw-config.toml"),
		"GOMODCACHE="+goModCache,
		"GOCACHE="+goCache,
	)

	mustRun(t, repoRoot, env, "--config", cfgPath, "source", "add", "local", "https://example.com/skills.git", "--kind", "git")
	mustRun(t, repoRoot, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)
	mustRun(t, repoRoot, env, "--config", cfgPath, "inject", "--agent", "openclaw")
	out := mustRun(t, repoRoot, env, "--config", cfgPath, "doctor")
	if !strings.Contains(out, "healthy") {
		t.Fatalf("expected healthy output, got %q", out)
	}

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile to exist: %v", err)
	}
}

func mustRun(t *testing.T, cwd string, env []string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/skillpm"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = cwd
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed (%v): %s", args, string(out))
	}
	return string(out)
}
