package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCLIInDir runs the CLI binary with a specific working directory.
func runCLIInDir(t *testing.T, bin string, env []string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s\nargs=%v\ndir=%s\noutput=%s", err, args, dir, string(out))
	}
	return string(out)
}

func runCLIInDirExpectFail(t *testing.T, bin string, env []string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected command to fail\nargs=%v\ndir=%s\noutput=%s", args, dir, string(out))
	}
	return string(out)
}

func TestCLIProjectInitAndInstall(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"code-review": {"SKILL.md": "# code-review\nCode review skill"},
	})

	// Create a project directory
	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// 1. Init (run from project dir)
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	assertContains(t, out, "initialized project at")

	// 2. Source add
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// 3. Install (run from project dir → auto-detects project scope)
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "install", "testrepo/code-review")
	assertContains(t, out, "testrepo/code-review")

	// 4. List (run from project dir)
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "list")
	assertContains(t, out, "code-review")

	// 5. Verify manifest was updated
	manifestData, err := os.ReadFile(filepath.Join(projectDir, ".skillpm", "skills.toml"))
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	assertContains(t, string(manifestData), "testrepo/code-review")

	// 6. Verify lockfile was created in project .skillpm/
	lockPath := filepath.Join(projectDir, ".skillpm", "skills.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile at %s: %v", lockPath, err)
	}

	// 7. Uninstall (run from project dir)
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "uninstall", "testrepo/code-review")
	assertContains(t, out, "testrepo/code-review")

	// 8. Verify manifest skill was removed
	manifestData, err = os.ReadFile(filepath.Join(projectDir, ".skillpm", "skills.toml"))
	if err != nil {
		t.Fatalf("read manifest after uninstall failed: %v", err)
	}
	if strings.Contains(string(manifestData), "testrepo/code-review") {
		t.Fatal("expected manifest to not contain code-review after uninstall")
	}

	// 9. List after uninstall
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "list")
	assertContains(t, out, "no installed skills")
}

func TestCLIProjectGlobalIsolation(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"skill-a": {"SKILL.md": "# skill-a\nSkill A"},
	})

	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// Source add
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// Install globally (--scope global)
	out := runCLI(t, bin, env, "--config", configPath, "--scope", "global", "install", "testrepo/skill-a")
	assertContains(t, out, "skill-a")

	// Global list should show the skill
	out = runCLI(t, bin, env, "--config", configPath, "--scope", "global", "list")
	assertContains(t, out, "skill-a")

	// Create project directory
	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Init project
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Project list should be empty
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "list")
	assertContains(t, out, "no installed skills")
}

func TestCLIProjectListJSON(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// Init, add source, install
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "install", "testrepo/demo")

	// List in JSON
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "list")

	var entries []struct {
		SkillRef string `json:"skillRef"`
		Version  string `json:"version"`
		Scope    string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SkillRef != "testrepo/demo" {
		t.Fatalf("got skillRef %q, want testrepo/demo", entries[0].SkillRef)
	}
	if entries[0].Scope != "project" {
		t.Fatalf("got scope %q, want project", entries[0].Scope)
	}
}

func TestCLIScopeFlag(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// Make sure we have a valid config
	runCLI(t, bin, env, "--config", configPath, "source", "list")

	// Invalid scope should error
	nonProjectDir := t.TempDir()
	out := runCLIInDirExpectFail(t, bin, env, nonProjectDir, "--config", configPath, "--scope", "workspace", "list")
	assertContains(t, out, "invalid scope")

	// --scope project without manifest should error
	out = runCLIInDirExpectFail(t, bin, env, nonProjectDir, "--config", configPath, "--scope", "project", "list")
	assertContains(t, out, "no project manifest found")
}
