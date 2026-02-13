package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root failed: %v", err)
	}
	return root
}

func buildCLI(t *testing.T, home string) (string, []string) {
	t.Helper()
	root := repoRoot(t)
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
	bin := filepath.Join(home, "bin", "skillpm")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatalf("create bin dir failed: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/skillpm")
	cmd.Dir = root
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build cli failed: %v\n%s", err, string(out))
	}
	return bin, env
}

func runCLI(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	return runCLIWithEnv(t, bin, env, nil, args...)
}

func runCLIWithEnv(t *testing.T, bin string, env []string, extra map[string]string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = mergeEnv(env, extra)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s\nargs=%v\noutput=%s", err, args, string(out))
	}
	return string(out)
}

func runCLIExpectFail(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	return runCLIExpectFailWithEnv(t, bin, env, nil, args...)
}

func runCLIExpectFailWithEnv(t *testing.T, bin string, env []string, extra map[string]string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = mergeEnv(env, extra)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected command to fail\nargs=%v\noutput=%s", args, string(out))
	}
	return string(out)
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	values := map[string]string{}
	for _, item := range base {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}
	for k, v := range extra {
		values[k] = v
	}
	out := make([]string, 0, len(values))
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out
}

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, out)
	}
}
