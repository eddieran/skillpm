package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleListEmpty(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "list")
	assertContains(t, out, "No bundles defined.")
}

func TestBundleListEmptyJSON(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "list")
	var bundles []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &bundles); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(bundles) != 0 {
		t.Fatalf("expected empty bundle list, got %d", len(bundles))
	}
}

func TestBundleListWithBundles(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)

	// Create a bundle
	runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "create", "web-dev", "testrepo/react", "testrepo/typescript")

	// List should show the bundle
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "list")
	assertContains(t, out, "web-dev")
	assertContains(t, out, "2 skills")
	assertContains(t, out, "testrepo/react")
	assertContains(t, out, "testrepo/typescript")
}

func TestBundleListWithBundlesJSON(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"eslint": {"SKILL.md": "# eslint\nESLint skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "create", "lint", "testrepo/eslint")

	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "list")

	var bundles []struct {
		Name   string   `json:"name"`
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal([]byte(out), &bundles); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}
	if bundles[0].Name != "lint" {
		t.Fatalf("expected bundle name 'lint', got %q", bundles[0].Name)
	}
	if len(bundles[0].Skills) != 1 || bundles[0].Skills[0] != "testrepo/eslint" {
		t.Fatalf("expected skills [testrepo/eslint], got %v", bundles[0].Skills)
	}
}

func TestBundleInstallValid(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// Create bundle
	runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "create", "web-dev", "testrepo/react", "testrepo/typescript")

	// Install bundle
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "install", "web-dev")
	assertContains(t, out, "2 skills")
	assertContains(t, out, "web-dev")

	// Verify skills are listed as installed
	listOut := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "list")
	assertContains(t, listOut, "react")
	assertContains(t, listOut, "typescript")
}

func TestBundleInstallInvalidName(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Install a non-existent bundle
	out := runCLIInDirExpectFail(t, bin, env, projectDir, "--config", configPath,
		"bundle", "install", "nonexistent")
	assertContains(t, out, "not found")
}

func TestBundleInstallIdempotent(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL)
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// Create bundle
	runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "create", "basics", "testrepo/demo")

	// First install
	out1 := runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "install", "basics")
	assertContains(t, out1, "1 skills")
	assertContains(t, out1, "basics")

	// Second install — should succeed with same result
	out2 := runCLIInDir(t, bin, env, projectDir, "--config", configPath,
		"bundle", "install", "basics")
	assertContains(t, out2, "1 skills")
	assertContains(t, out2, "basics")

	// Verify only one copy of the skill is installed
	listOut := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "list")
	var entries []struct {
		SkillRef string `json:"skillRef"`
	}
	if err := json.Unmarshal([]byte(listOut), &entries); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, listOut)
	}
	count := 0
	for _, e := range entries {
		if e.SkillRef == "testrepo/demo" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 instance of testrepo/demo, got %d", count)
	}
}
