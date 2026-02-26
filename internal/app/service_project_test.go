package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
)

func TestInitProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := svc.InitProject(projectDir)
	if err != nil {
		t.Fatalf("init project failed: %v", err)
	}

	if path != config.ProjectManifestPath(projectDir) {
		t.Fatalf("got path %q, want %q", path, config.ProjectManifestPath(projectDir))
	}

	// Verify manifest is loadable
	m, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest failed: %v", err)
	}
	if m.Version != 1 {
		t.Fatalf("version %d, want 1", m.Version)
	}

	// Verify double init fails
	if _, err := svc.InitProject(projectDir); err == nil {
		t.Fatal("expected error for double init")
	}
}

func TestProjectScopeAutoDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Resolve symlinks (macOS /var -> /private/var)
	projectDir, _ = filepath.EvalSymlinks(projectDir)

	// Init the project first
	if _, err := config.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}

	// Change to a subdirectory of the project
	subDir := filepath.Join(projectDir, "src", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	origWd, _ := os.Getwd()
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	// Create a service without explicit scope — should auto-detect project
	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	if svc.Scope != config.ScopeProject {
		t.Fatalf("got scope %q, want project (auto-detected)", svc.Scope)
	}
	if svc.ProjectRoot != projectDir {
		t.Fatalf("got project root %q, want %q", svc.ProjectRoot, projectDir)
	}
	if svc.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
}

func TestProjectInstallAndUninstall(t *testing.T) {
	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatal(err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"review": {"SKILL.md": "# review\nCode review skill"},
		"lint":   {"SKILL.md": "# lint\nLint rules skill"},
	})

	// Create project
	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}

	// Create service with explicit project scope
	svc, err := New(Options{
		ConfigPath:  filepath.Join(home, ".skillpm", "config.toml"),
		Scope:       config.ScopeProject,
		ProjectRoot: projectDir,
	})
	if err != nil {
		t.Fatalf("new service with project scope failed: %v", err)
	}

	// Add source
	if _, err := svc.SourceAdd("testrepo", repoURL, "", "", ""); err != nil {
		t.Fatalf("source add failed: %v", err)
	}
	if _, err := svc.SourceUpdate(context.Background(), "testrepo"); err != nil {
		t.Fatalf("source update failed: %v", err)
	}

	// Install at project scope
	installed, err := svc.Install(context.Background(), []string{"testrepo/review"}, "", false)
	if err != nil {
		t.Fatalf("project install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(installed))
	}

	// Verify state is in project .skillpm/
	projectState, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load project state failed: %v", err)
	}
	if len(projectState.Installed) != 1 {
		t.Fatalf("expected 1 installed in project state, got %d", len(projectState.Installed))
	}

	// Verify manifest was updated
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest failed: %v", err)
	}
	if len(manifest.Skills) != 1 {
		t.Fatalf("expected 1 skill in manifest, got %d", len(manifest.Skills))
	}
	if manifest.Skills[0].Ref != "testrepo/review" {
		t.Fatalf("manifest skill ref = %q, want testrepo/review", manifest.Skills[0].Ref)
	}

	// Verify lockfile is in project .skillpm/
	lockPath := config.ProjectLockPath(projectDir)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected project lockfile at %s: %v", lockPath, err)
	}

	// Uninstall
	removed, err := svc.Uninstall(context.Background(), []string{"testrepo/review"}, "")
	if err != nil {
		t.Fatalf("project uninstall failed: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}

	// Verify manifest updated
	manifest, err = config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest after uninstall failed: %v", err)
	}
	if len(manifest.Skills) != 0 {
		t.Fatalf("expected 0 skills in manifest after uninstall, got %d", len(manifest.Skills))
	}
}

func TestProjectAndGlobalIsolation(t *testing.T) {
	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatal(err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"skill-a": {"SKILL.md": "# skill-a\nSkill A"},
	})

	// Create project
	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}

	// Create global service
	globalSvc, err := New(Options{
		ConfigPath: filepath.Join(home, ".skillpm", "config.toml"),
		Scope:      config.ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("global service failed: %v", err)
	}
	if _, err := globalSvc.SourceAdd("testrepo", repoURL, "", "", ""); err != nil {
		t.Fatalf("source add failed: %v", err)
	}
	if _, err := globalSvc.SourceUpdate(context.Background(), "testrepo"); err != nil {
		t.Fatalf("source update failed: %v", err)
	}

	// Install globally
	_, err = globalSvc.Install(context.Background(), []string{"testrepo/skill-a"}, "", false)
	if err != nil {
		t.Fatalf("global install failed: %v", err)
	}

	// Verify global state has the skill
	globalState, err := store.LoadState(globalSvc.StateRoot)
	if err != nil {
		t.Fatalf("load global state failed: %v", err)
	}
	if len(globalState.Installed) != 1 {
		t.Fatalf("expected 1 global installed, got %d", len(globalState.Installed))
	}

	// Verify project state is empty
	projectState, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load project state failed: %v", err)
	}
	if len(projectState.Installed) != 0 {
		t.Fatalf("expected 0 project installed, got %d", len(projectState.Installed))
	}
}

func TestListInstalled(t *testing.T) {
	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatal(err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})

	projectDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitProject(projectDir); err != nil {
		t.Fatal(err)
	}

	svc, err := New(Options{
		ConfigPath:  filepath.Join(home, ".skillpm", "config.toml"),
		Scope:       config.ScopeProject,
		ProjectRoot: projectDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SourceAdd("testrepo", repoURL, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SourceUpdate(context.Background(), "testrepo"); err != nil {
		t.Fatal(err)
	}

	// List before install
	list, err := svc.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 installed before install, got %d", len(list))
	}

	// Install and list again
	if _, err := svc.Install(context.Background(), []string{"testrepo/forms"}, "", false); err != nil {
		t.Fatal(err)
	}
	list, err = svc.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 installed after install, got %d", len(list))
	}
	if list[0].SkillRef != "testrepo/forms" {
		t.Fatalf("got skill ref %q, want testrepo/forms", list[0].SkillRef)
	}
}
