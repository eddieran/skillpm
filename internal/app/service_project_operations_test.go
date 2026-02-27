package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
)

// setupProjectWithSkill creates a project-scoped service with one skill installed.
func setupProjectWithSkill(t *testing.T, skillName string) (*Service, string) {
	t.Helper()

	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatalf("mkdir openclaw state: %v", err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		skillName: {"SKILL.md": "# " + skillName + "\n" + skillName + " skill"},
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
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SourceAdd("testrepo", repoURL, "", "", ""); err != nil {
		t.Fatalf("source add: %v", err)
	}
	if _, err := svc.SourceUpdate(context.Background(), "testrepo"); err != nil {
		t.Fatalf("source update: %v", err)
	}

	if _, err := svc.Install(context.Background(), []string{"testrepo/" + skillName}, "", false); err != nil {
		t.Fatalf("install: %v", err)
	}

	return svc, projectDir
}

// setupProjectWithMultipleSkills creates a project-scoped service with "alpha" and "beta" installed.
func setupProjectWithMultipleSkills(t *testing.T) (*Service, string) {
	t.Helper()

	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatalf("mkdir openclaw state: %v", err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"alpha": {"SKILL.md": "# alpha\nAlpha skill"},
		"beta":  {"SKILL.md": "# beta\nBeta skill"},
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
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SourceAdd("testrepo", repoURL, "", "", ""); err != nil {
		t.Fatalf("source add: %v", err)
	}
	if _, err := svc.SourceUpdate(context.Background(), "testrepo"); err != nil {
		t.Fatalf("source update: %v", err)
	}

	if _, err := svc.Install(context.Background(), []string{"testrepo/alpha", "testrepo/beta"}, "", false); err != nil {
		t.Fatalf("install: %v", err)
	}

	return svc, projectDir
}

// --- Upgrade tests ---

func TestProjectUpgradeNoChanges(t *testing.T) {
	svc, projectDir := setupProjectWithSkill(t, "review")

	upgraded, err := svc.Upgrade(context.Background(), nil, "", false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if len(upgraded) != 0 {
		t.Fatalf("expected no upgrades when version unchanged, got %d", len(upgraded))
	}

	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Installed) != 1 {
		t.Fatalf("expected 1 installed after no-op upgrade, got %d", len(st.Installed))
	}
}

func TestProjectUpgradeAllInstalled(t *testing.T) {
	svc, _ := setupProjectWithMultipleSkills(t)

	// No version change → no upgrades
	upgraded, err := svc.Upgrade(context.Background(), nil, "", false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if len(upgraded) != 0 {
		t.Fatalf("expected 0 upgrades, got %d", len(upgraded))
	}
}

func TestProjectUpgradeSpecificRef(t *testing.T) {
	svc, projectDir := setupProjectWithMultipleSkills(t)

	// Upgrade only alpha — should succeed with no changes (same version)
	upgraded, err := svc.Upgrade(context.Background(), []string{"testrepo/alpha"}, "", false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if len(upgraded) != 0 {
		t.Fatalf("expected 0 upgrades for same version, got %d", len(upgraded))
	}

	// Verify manifest still has both skills
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(manifest.Skills) != 2 {
		t.Fatalf("expected 2 skills in manifest, got %d", len(manifest.Skills))
	}
}

func TestProjectUpgradeNoInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "oc"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "oc.toml"))
	os.MkdirAll(filepath.Join(home, "oc"), 0o755)

	projectDir := filepath.Join(t.TempDir(), "empty")
	os.MkdirAll(projectDir, 0o755)
	config.InitProject(projectDir)

	svc, err := New(Options{
		ConfigPath:  filepath.Join(home, ".skillpm", "config.toml"),
		Scope:       config.ScopeProject,
		ProjectRoot: projectDir,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	upgraded, err := svc.Upgrade(context.Background(), nil, "", false)
	if err != nil {
		t.Fatalf("upgrade with no installed: %v", err)
	}
	if upgraded != nil {
		t.Fatalf("expected nil, got %d", len(upgraded))
	}
}

// --- Inject tests ---

func TestProjectInjectAll(t *testing.T) {
	svc, projectDir := setupProjectWithSkill(t, "review")

	result, err := svc.Inject(context.Background(), "openclaw", nil)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if len(result.Injected) != 1 {
		t.Fatalf("expected 1 injected, got %d", len(result.Injected))
	}

	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Injections) != 1 {
		t.Fatalf("expected 1 injection record, got %d", len(st.Injections))
	}
	if st.Injections[0].Agent != "openclaw" {
		t.Fatalf("expected agent openclaw, got %q", st.Injections[0].Agent)
	}
}

func TestProjectInjectSpecificRefs(t *testing.T) {
	svc, projectDir := setupProjectWithMultipleSkills(t)

	result, err := svc.Inject(context.Background(), "openclaw", []string{"testrepo/alpha"})
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if len(result.Injected) != 1 {
		t.Fatalf("expected 1 injected, got %d", len(result.Injected))
	}

	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Injections) != 1 {
		t.Fatalf("expected 1 injection record, got %d", len(st.Injections))
	}
}

func TestProjectInjectMissingAdapter(t *testing.T) {
	svc, _ := setupProjectWithSkill(t, "review")

	_, err := svc.Inject(context.Background(), "nonexistent-adapter", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent adapter")
	}
}

// --- RemoveInjected tests ---

func TestProjectRemoveInjected(t *testing.T) {
	svc, projectDir := setupProjectWithSkill(t, "review")

	// First inject
	_, err := svc.Inject(context.Background(), "openclaw", nil)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	// Then remove
	_, err = svc.RemoveInjected(context.Background(), "openclaw", []string{"testrepo/review"})
	if err != nil {
		t.Fatalf("remove injected: %v", err)
	}

	// Verify injection state no longer contains the skill
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	for _, inj := range st.Injections {
		if inj.Agent == "openclaw" {
			for _, s := range inj.Skills {
				if s == "testrepo/review" {
					t.Fatal("expected testrepo/review removed from injection state")
				}
			}
		}
	}
}

func TestProjectRemoveInjectedMissingAdapter(t *testing.T) {
	svc, _ := setupProjectWithSkill(t, "review")

	_, err := svc.RemoveInjected(context.Background(), "nonexistent-adapter", []string{"testrepo/review"})
	if err == nil {
		t.Fatal("expected error for nonexistent adapter")
	}
}

func TestProjectRemovePreservesOther(t *testing.T) {
	svc, projectDir := setupProjectWithMultipleSkills(t)

	// Inject both
	_, err := svc.Inject(context.Background(), "openclaw", nil)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	// Remove only alpha
	_, err = svc.RemoveInjected(context.Background(), "openclaw", []string{"testrepo/alpha"})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Verify beta is still injected
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	foundBeta := false
	for _, inj := range st.Injections {
		if inj.Agent == "openclaw" {
			for _, s := range inj.Skills {
				if s == "testrepo/beta" {
					foundBeta = true
				}
			}
		}
	}
	if !foundBeta {
		t.Fatal("expected testrepo/beta to remain in injection state")
	}
}

// --- Sync tests ---

func TestProjectSyncNoUpgrades(t *testing.T) {
	svc, _ := setupProjectWithSkill(t, "review")

	report, err := svc.SyncRun(context.Background(), "", false, false)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(report.UpgradedSkills) != 0 {
		t.Fatalf("expected no upgrades, got %v", report.UpgradedSkills)
	}
	if len(report.UpdatedSources) == 0 {
		t.Fatal("expected at least one source update")
	}
}

func TestProjectSyncDryRun(t *testing.T) {
	svc, projectDir := setupProjectWithSkill(t, "review")

	stBefore, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state before: %v", err)
	}

	report, err := svc.SyncRun(context.Background(), "", false, true)
	if err != nil {
		t.Fatalf("sync dry-run: %v", err)
	}
	if !report.DryRun {
		t.Fatal("expected DryRun=true in report")
	}

	stAfter, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state after: %v", err)
	}
	if len(stBefore.Installed) != len(stAfter.Installed) {
		t.Fatalf("state changed during dry-run: before=%d after=%d",
			len(stBefore.Installed), len(stAfter.Installed))
	}
}

func TestProjectSyncReinjection(t *testing.T) {
	svc, projectDir := setupProjectWithSkill(t, "review")

	// Inject first
	_, err := svc.Inject(context.Background(), "openclaw", nil)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	// Sync should attempt reinjection
	report, err := svc.SyncRun(context.Background(), "", false, false)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Verify injection state persists after sync
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Injections) == 0 {
		t.Fatal("expected injection state to persist after sync")
	}

	totalReinjects := len(report.Reinjected) + len(report.SkippedReinjects) + len(report.FailedReinjects)
	if totalReinjects == 0 {
		t.Fatal("expected at least one reinject attempt in report")
	}
}
