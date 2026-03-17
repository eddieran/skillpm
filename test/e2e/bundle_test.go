package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
)

// TestCLIBundleListEmpty verifies "bundle list" shows nothing when no bundles are defined.
func TestCLIBundleListEmpty(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Init project
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Bundle list should show "No bundles"
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "list")
	assertContains(t, out, "No bundles defined")
}

// TestCLIBundleListJSON verifies "bundle list --json" returns an empty JSON array when no bundles exist,
// and a populated array after bundles are created.
func TestCLIBundleListJSON(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Empty bundle list JSON should be null or empty array
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "list")
	out = trimJSONOutput(out)
	var bundles []config.BundleEntry
	if err := json.Unmarshal([]byte(out), &bundles); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(bundles) != 0 && bundles != nil {
		t.Fatalf("expected empty bundles, got %d", len(bundles))
	}

	// Create a bundle, then list again
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "web", "local/react", "local/typescript")

	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "list")
	out = trimJSONOutput(out)
	if err := json.Unmarshal([]byte(out), &bundles); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}
	if bundles[0].Name != "web" {
		t.Fatalf("expected bundle name 'web', got %q", bundles[0].Name)
	}
	if len(bundles[0].Skills) != 2 {
		t.Fatalf("expected 2 skills in bundle, got %d", len(bundles[0].Skills))
	}
}

// TestCLIBundleCreateAndList verifies the full create+list flow.
func TestCLIBundleCreateAndList(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Create two bundles
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "web", "local/react", "local/css")
	assertContains(t, out, "Created bundle")
	assertContains(t, out, "web")

	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "backend", "local/golang", "local/docker", "local/postgres")
	assertContains(t, out, "Created bundle")
	assertContains(t, out, "backend")

	// List should show both bundles
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "list")
	assertContains(t, out, "web")
	assertContains(t, out, "backend")
	assertContains(t, out, "2 skills")
	assertContains(t, out, "3 skills")

	// Verify manifest file was updated
	manifestData, err := os.ReadFile(filepath.Join(projectDir, ".skillpm", "skills.toml"))
	if err != nil {
		t.Fatalf("read manifest failed: %v", err)
	}
	assertContains(t, string(manifestData), "web")
	assertContains(t, string(manifestData), "backend")
}

// TestCLIBundleInstall verifies the bundle install flow: create a bundle with real skills,
// then install it and verify all skills get installed.
func TestCLIBundleInstall(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact development skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Init project, add source, update source
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// Create bundle
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "frontend", "testrepo/react", "testrepo/typescript")

	// Install the bundle
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "install", "frontend")
	assertContains(t, out, "Installed 2 skills from bundle")
	assertContains(t, out, "react")
	assertContains(t, out, "typescript")

	// List installed skills to verify
	out = runCLIInDir(t, bin, env, projectDir, "--config", configPath, "list")
	assertContains(t, out, "react")
	assertContains(t, out, "typescript")
}

// TestCLIBundleInstallJSON verifies bundle install with --json output.
func TestCLIBundleInstallJSON(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"linter": {"SKILL.md": "# linter\nLinting skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")
	runCLI(t, bin, env, "--config", configPath, "source", "add", "testrepo", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", configPath, "source", "update", "testrepo")

	// Create bundle with one skill
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "lint", "testrepo/linter")

	// Install with JSON output
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "install", "lint")
	out = trimJSONOutput(out)

	var installed []struct {
		SkillRef        string `json:"skillRef"`
		ResolvedVersion string `json:"resolvedVersion"`
	}
	if err := json.Unmarshal([]byte(out), &installed); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(installed))
	}
	if installed[0].SkillRef != "testrepo/linter" {
		t.Fatalf("expected skillRef 'testrepo/linter', got %q", installed[0].SkillRef)
	}
}

// TestCLIBundleInstallMissing verifies error when installing a non-existent bundle.
func TestCLIBundleInstallMissing(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Try to install a bundle that doesn't exist
	out := runCLIInDirExpectFail(t, bin, env, projectDir, "--config", configPath, "bundle", "install", "nonexistent")
	assertContains(t, out, "not found")
}

// TestCLIBundleInstallEmptyBundle verifies error when installing a bundle with no skills.
func TestCLIBundleInstallEmptyBundle(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Manually create a bundle with no skills by writing a manifest with an empty bundle
	manifestPath := filepath.Join(projectDir, ".skillpm", "skills.toml")
	manifestContent := `version = 1

[[bundles]]
name = "empty"
skills = []
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	// Try to install the empty bundle
	out := runCLIInDirExpectFail(t, bin, env, projectDir, "--config", configPath, "bundle", "install", "empty")
	assertContains(t, out, "no skills")
}

// TestCLIBundleInstallInvalidSkillRef verifies error when a bundle references skills
// that cannot be resolved.
func TestCLIBundleInstallInvalidSkillRef(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Create a bundle referencing a non-existent source/skill
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "broken", "nosource/noskill")

	// Install should fail because the source doesn't exist
	out := runCLIInDirExpectFail(t, bin, env, projectDir, "--config", configPath, "bundle", "install", "broken")
	// Should fail with a resolution or source error
	if len(out) == 0 {
		t.Fatal("expected error output")
	}
}

// TestCLIBundleCreateNoProjectManifest verifies error when creating a bundle without
// a project manifest (no init).
func TestCLIBundleCreateNoProjectManifest(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// Ensure config exists
	runCLI(t, bin, env, "--config", configPath, "source", "list")

	// Try to create a bundle from a non-project directory with explicit project scope
	nonProjectDir := t.TempDir()
	out := runCLIInDirExpectFail(t, bin, env, nonProjectDir, "--config", configPath, "--scope", "project", "bundle", "create", "test", "a/b")
	assertContains(t, out, "no project manifest found")
}

// TestCLIBundleInstallNoProjectManifest verifies error when installing a bundle
// without a project manifest.
func TestCLIBundleInstallNoProjectManifest(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	// Ensure config exists
	runCLI(t, bin, env, "--config", configPath, "source", "list")

	// Try bundle install from global scope (no manifest)
	out := runCLIExpectFail(t, bin, env, "--config", configPath, "--scope", "global", "bundle", "install", "test")
	assertContains(t, out, "bundles require a project manifest")
}

// TestCLIBundleCreateUpsert verifies that creating a bundle with the same name updates it.
func TestCLIBundleCreateUpsert(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	configPath := filepath.Join(home, ".skillpm", "config.toml")

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "init")

	// Create initial bundle
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "dev", "local/a")

	// Upsert with different skills
	runCLIInDir(t, bin, env, projectDir, "--config", configPath, "bundle", "create", "dev", "local/b", "local/c")

	// List should show the updated bundle
	out := runCLIInDir(t, bin, env, projectDir, "--config", configPath, "--json", "bundle", "list")
	out = trimJSONOutput(out)
	var bundles []config.BundleEntry
	if err := json.Unmarshal([]byte(out), &bundles); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw=%s", err, out)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle after upsert, got %d", len(bundles))
	}
	if len(bundles[0].Skills) != 2 {
		t.Fatalf("expected 2 skills after upsert, got %d", len(bundles[0].Skills))
	}
	if bundles[0].Skills[0] != "local/b" || bundles[0].Skills[1] != "local/c" {
		t.Fatalf("expected skills [local/b, local/c] after upsert, got %v", bundles[0].Skills)
	}
}

// trimJSONOutput strips any non-JSON prefix from CLI output (e.g. git progress lines).
func trimJSONOutput(s string) string {
	for i, c := range s {
		if c == '[' || c == '{' || c == 'n' { // '[', '{', or 'null'
			return s[i:]
		}
	}
	return s
}
