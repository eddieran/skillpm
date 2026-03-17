package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
)

// setupProjectWithBundle creates a project-scoped service with a git source
// containing the given skills, and a bundle referencing bundleSkills.
func setupProjectWithBundle(t *testing.T, skills map[string]map[string]string, bundleName string, bundleSkills []string) (*Service, string) {
	t.Helper()

	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatalf("mkdir openclaw state: %v", err)
	}

	repoURL := setupBareRepo(t, skills)

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

	// Create the bundle in the manifest
	if bundleName != "" && len(bundleSkills) > 0 {
		if err := svc.BundleCreate(bundleName, bundleSkills); err != nil {
			t.Fatalf("bundle create: %v", err)
		}
	}

	return svc, projectDir
}

// --- BundleList tests ---

func TestBundleListEmpty(t *testing.T) {
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

	bundles := svc.BundleList()
	if len(bundles) != 0 {
		t.Fatalf("expected 0 bundles, got %d", len(bundles))
	}
}

func TestBundleListNoManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc, err := New(Options{
		ConfigPath: filepath.Join(home, ".skillpm", "config.toml"),
		Scope:      config.ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	bundles := svc.BundleList()
	if bundles != nil {
		t.Fatalf("expected nil bundles for global scope, got %v", bundles)
	}
}

func TestBundleListWithBundles(t *testing.T) {
	skills := map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
		"eslint":     {"SKILL.md": "# eslint\nESLint skill"},
	}
	svc, _ := setupProjectWithBundle(t, skills, "web-dev", []string{"testrepo/react", "testrepo/typescript"})

	// Create a second bundle
	if err := svc.BundleCreate("linting", []string{"testrepo/eslint"}); err != nil {
		t.Fatalf("create second bundle: %v", err)
	}

	bundles := svc.BundleList()
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(bundles))
	}

	// Verify bundle contents
	found := map[string]bool{}
	for _, b := range bundles {
		found[b.Name] = true
	}
	if !found["web-dev"] || !found["linting"] {
		t.Fatalf("expected bundles web-dev and linting, got %v", bundles)
	}
}

// --- BundleCreate tests ---

func TestBundleCreateBasic(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "", nil)

	if err := svc.BundleCreate("my-bundle", []string{"testrepo/forms"}); err != nil {
		t.Fatalf("bundle create: %v", err)
	}

	// Verify bundle persisted to manifest
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(manifest.Bundles) != 1 {
		t.Fatalf("expected 1 bundle in manifest, got %d", len(manifest.Bundles))
	}
	if manifest.Bundles[0].Name != "my-bundle" {
		t.Fatalf("expected bundle name 'my-bundle', got %q", manifest.Bundles[0].Name)
	}
	if len(manifest.Bundles[0].Skills) != 1 || manifest.Bundles[0].Skills[0] != "testrepo/forms" {
		t.Fatalf("unexpected skills: %v", manifest.Bundles[0].Skills)
	}
}

func TestBundleCreateUpsert(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
		"lint":  {"SKILL.md": "# lint\nLint skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "my-bundle", []string{"testrepo/forms"})

	// Upsert the same bundle with different skills
	if err := svc.BundleCreate("my-bundle", []string{"testrepo/lint"}); err != nil {
		t.Fatalf("bundle upsert: %v", err)
	}

	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(manifest.Bundles) != 1 {
		t.Fatalf("expected 1 bundle after upsert, got %d", len(manifest.Bundles))
	}
	if len(manifest.Bundles[0].Skills) != 1 || manifest.Bundles[0].Skills[0] != "testrepo/lint" {
		t.Fatalf("expected upserted skills [testrepo/lint], got %v", manifest.Bundles[0].Skills)
	}
}

func TestBundleCreateNoManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc, err := New(Options{
		ConfigPath: filepath.Join(home, ".skillpm", "config.toml"),
		Scope:      config.ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = svc.BundleCreate("test", []string{"a/b"})
	if err == nil {
		t.Fatal("expected error creating bundle without manifest")
	}
}

func TestBundleCreateEmptyName(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, _ := setupProjectWithBundle(t, skills, "", nil)

	err := svc.BundleCreate("", []string{"testrepo/forms"})
	if err == nil {
		t.Fatal("expected error for empty bundle name")
	}
}

func TestBundleCreateNoSkills(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, _ := setupProjectWithBundle(t, skills, "", nil)

	err := svc.BundleCreate("empty-bundle", nil)
	if err == nil {
		t.Fatal("expected error for bundle with no skills")
	}
}

// --- BundleInstall tests ---

func TestBundleInstallBasic(t *testing.T) {
	skills := map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "web-dev", []string{"testrepo/react", "testrepo/typescript"})

	installed, err := svc.BundleInstall(context.Background(), "web-dev", "", false)
	if err != nil {
		t.Fatalf("bundle install: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("expected 2 installed skills, got %d", len(installed))
	}

	// Verify state has both skills
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Installed) != 2 {
		t.Fatalf("expected 2 installed in state, got %d", len(st.Installed))
	}

	// Verify manifest updated with skill entries
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(manifest.Skills) != 2 {
		t.Fatalf("expected 2 skills in manifest, got %d", len(manifest.Skills))
	}

	// Verify installed refs
	refs := map[string]bool{}
	for _, s := range installed {
		refs[s.SkillRef] = true
	}
	if !refs["testrepo/react"] || !refs["testrepo/typescript"] {
		t.Fatalf("expected testrepo/react and testrepo/typescript installed, got %v", refs)
	}
}

func TestBundleInstallSingleSkill(t *testing.T) {
	skills := map[string]map[string]string{
		"eslint": {"SKILL.md": "# eslint\nESLint skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "linting", []string{"testrepo/eslint"})

	installed, err := svc.BundleInstall(context.Background(), "linting", "", false)
	if err != nil {
		t.Fatalf("bundle install: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(installed))
	}
	if installed[0].SkillRef != "testrepo/eslint" {
		t.Fatalf("expected testrepo/eslint, got %q", installed[0].SkillRef)
	}

	// Verify lockfile was created
	lockPath := config.ProjectLockPath(projectDir)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile at %s: %v", lockPath, err)
	}
}

func TestBundleInstallMissingBundle(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, _ := setupProjectWithBundle(t, skills, "existing", []string{"testrepo/forms"})

	_, err := svc.BundleInstall(context.Background(), "nonexistent", "", false)
	if err == nil {
		t.Fatal("expected error for missing bundle")
	}
}

func TestBundleInstallNoManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc, err := New(Options{
		ConfigPath: filepath.Join(home, ".skillpm", "config.toml"),
		Scope:      config.ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.BundleInstall(context.Background(), "test", "", false)
	if err == nil {
		t.Fatal("expected error installing bundle without manifest")
	}
}

func TestBundleInstallEmptyBundle(t *testing.T) {
	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))
	os.MkdirAll(openclawState, 0o755)

	projectDir := filepath.Join(t.TempDir(), "myproject")
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

	// Manually add an empty bundle to the manifest
	config.UpsertBundle(svc.Manifest, config.BundleEntry{Name: "empty", Skills: []string{}})
	if err := svc.SaveManifest(); err != nil {
		t.Fatalf("save manifest: %v", err)
	}

	_, err = svc.BundleInstall(context.Background(), "empty", "", false)
	if err == nil {
		t.Fatal("expected error for empty bundle")
	}
}

func TestBundleInstallWithInvalidSkillRef(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, _ := setupProjectWithBundle(t, skills, "bad-bundle", []string{"testrepo/nonexistent-skill"})

	_, err := svc.BundleInstall(context.Background(), "bad-bundle", "", false)
	if err == nil {
		t.Fatal("expected error for bundle with invalid skill ref")
	}
}

// --- Full bundle workflow tests ---

func TestBundleCreateListInstallWorkflow(t *testing.T) {
	skills := map[string]map[string]string{
		"react":      {"SKILL.md": "# react\nReact skill"},
		"typescript": {"SKILL.md": "# typescript\nTypeScript skill"},
		"eslint":     {"SKILL.md": "# eslint\nESLint skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "", nil)
	ctx := context.Background()

	// Step 1: No bundles initially
	bundles := svc.BundleList()
	if len(bundles) != 0 {
		t.Fatalf("expected 0 bundles initially, got %d", len(bundles))
	}

	// Step 2: Create a bundle
	if err := svc.BundleCreate("frontend", []string{"testrepo/react", "testrepo/typescript"}); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	// Step 3: List shows the bundle
	bundles = svc.BundleList()
	if len(bundles) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(bundles))
	}
	if bundles[0].Name != "frontend" {
		t.Fatalf("expected bundle 'frontend', got %q", bundles[0].Name)
	}
	if len(bundles[0].Skills) != 2 {
		t.Fatalf("expected 2 skills in bundle, got %d", len(bundles[0].Skills))
	}

	// Step 4: Install the bundle
	installed, err := svc.BundleInstall(ctx, "frontend", "", false)
	if err != nil {
		t.Fatalf("bundle install: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("expected 2 installed, got %d", len(installed))
	}

	// Step 5: Verify state
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Installed) != 2 {
		t.Fatalf("expected 2 in state, got %d", len(st.Installed))
	}

	// Step 6: Verify manifest has both bundle and skill entries
	manifest, err := config.LoadProjectManifest(projectDir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(manifest.Bundles) != 1 {
		t.Fatalf("expected 1 bundle in manifest, got %d", len(manifest.Bundles))
	}
	if len(manifest.Skills) != 2 {
		t.Fatalf("expected 2 skills in manifest, got %d", len(manifest.Skills))
	}

	// Step 7: Create another bundle and install it
	if err := svc.BundleCreate("linting", []string{"testrepo/eslint"}); err != nil {
		t.Fatalf("create second bundle: %v", err)
	}
	installed2, err := svc.BundleInstall(ctx, "linting", "", false)
	if err != nil {
		t.Fatalf("install second bundle: %v", err)
	}
	if len(installed2) != 1 {
		t.Fatalf("expected 1 installed from second bundle, got %d", len(installed2))
	}

	// Verify total state
	st, err = store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Installed) != 3 {
		t.Fatalf("expected 3 total installed, got %d", len(st.Installed))
	}

	bundles = svc.BundleList()
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles after second create, got %d", len(bundles))
	}
}

func TestBundleInstallIdempotent(t *testing.T) {
	skills := map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	}
	svc, projectDir := setupProjectWithBundle(t, skills, "tools", []string{"testrepo/forms"})
	ctx := context.Background()

	// Install once
	installed1, err := svc.BundleInstall(ctx, "tools", "", false)
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	if len(installed1) != 1 {
		t.Fatalf("expected 1 installed, got %d", len(installed1))
	}

	// Install again — should succeed (idempotent)
	installed2, err := svc.BundleInstall(ctx, "tools", "", false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if len(installed2) != 1 {
		t.Fatalf("expected 1 installed on re-install, got %d", len(installed2))
	}

	// Verify state has exactly 1 skill (not duplicated)
	st, err := store.LoadState(config.ProjectStateRoot(projectDir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(st.Installed) != 1 {
		t.Fatalf("expected 1 installed in state (not duplicated), got %d", len(st.Installed))
	}
}
