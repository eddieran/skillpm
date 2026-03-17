package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

var (
	sharedBinOnce sync.Once
	sharedBinPath string
	sharedBinErr  error
)

func buildCLI(t *testing.T, home string) (string, []string) {
	t.Helper()
	root := repoRoot(t)

	// Build the binary once and reuse across all e2e tests.
	// Previously each test rebuilt from scratch with a different HOME,
	// causing GOCACHE misses and ~30-60s builds per test.
	sharedBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "skillpm-e2e-*")
		if err != nil {
			sharedBinErr = fmt.Errorf("create temp dir: %w", err)
			return
		}
		sharedBinPath = filepath.Join(dir, "skillpm")
		cmd := exec.Command("go", "build", "-o", sharedBinPath, "./cmd/skillpm")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			sharedBinErr = fmt.Errorf("build cli: %w\n%s", err, string(out))
		}
	})
	if sharedBinErr != nil {
		t.Fatalf("buildCLI: %v", sharedBinErr)
	}

	// Capture real GOPATH before we override HOME, so runtime commands
	// that shell out to `go` don't download modules into the temp HOME dir.
	realGoPath := os.Getenv("GOPATH")
	if realGoPath == "" {
		realGoPath = filepath.Join(os.Getenv("HOME"), "go")
	}

	env := append(os.Environ(),
		"HOME="+home,
		"OPENCLAW_STATE_DIR="+filepath.Join(home, "openclaw-state"),
		"OPENCLAW_CONFIG_PATH="+filepath.Join(home, "openclaw-config.toml"),
		"GOPATH="+realGoPath,
	)
	return sharedBinPath, env
}

func runCLI(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	return runCLIWithEnv(t, bin, env, nil, args...)
}

func runCLIWithEnv(t *testing.T, bin string, env []string, extra map[string]string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = mergeEnv(env, extra)
	// Run from a temp dir to avoid auto-detecting a project scope from the
	// test runner's working directory.
	cmd.Dir = t.TempDir()
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
	cmd.Dir = t.TempDir()
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

// setupBareRepoE2E creates a local bare git repo with skill files and returns file:// URL.
// skills shape: {"demo": {"SKILL.md": "# demo\nDemo skill"}}.
// Files are placed under a "skills/" prefix in the repo.
func setupBareRepoE2E(t *testing.T, skills map[string]map[string]string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	workDir := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work failed: %v", err)
	}

	gitRun := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}

	gitRun(workDir, "init", "-b", "main")

	for skillName, files := range skills {
		for relPath, content := range files {
			fullPath := filepath.Join(workDir, "skills", skillName, relPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatalf("mkdir failed: %v", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				t.Fatalf("write file failed: %v", err)
			}
		}
	}

	gitRun(workDir, "add", "-A")
	gitRun(workDir, "commit", "-m", "initial")

	bareDir := filepath.Join(t.TempDir(), "repo.git")
	gitRun(workDir, "clone", "--bare", workDir, bareDir)

	return "file://" + bareDir
}
