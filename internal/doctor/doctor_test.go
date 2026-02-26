package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/store"
)

// setupTestEnv creates a minimal environment for doctor tests.
func setupTestEnv(t *testing.T) (home string, cfgPath string, stateRoot string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	cfgPath = filepath.Join(home, ".skillpm", "config.toml")
	stateRoot = filepath.Join(home, ".skillpm")
	return
}

func saveConfig(t *testing.T, cfgPath string, cfg config.Config) {
	t.Helper()
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func saveState(t *testing.T, stateRoot string, st store.State) {
	t.Helper()
	if err := store.SaveState(stateRoot, st); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func newService(t *testing.T, cfgPath, stateRoot, lockPath, projectRoot string, scope config.Scope) *Service {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		// Config might not exist yet for some tests; that's ok.
		cfg = config.DefaultConfig()
	}
	rt, err := adapter.NewRuntime(stateRoot, cfg, projectRoot)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return &Service{
		ConfigPath:  cfgPath,
		StateRoot:   stateRoot,
		LockPath:    lockPath,
		Runtime:     rt,
		Scope:       scope,
		ProjectRoot: projectRoot,
	}
}

// --- check 1: config ---

func TestCheckConfig_OK(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	_ = home
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkConfig()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s: %s", r.Status, r.Fix)
	}
}

func TestCheckConfig_FixedMissingConfig(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	if err := store.EnsureLayout(stateRoot); err != nil {
		t.Fatal(err)
	}
	svc := &Service{ConfigPath: cfgPath, StateRoot: stateRoot, Scope: config.ScopeGlobal}
	r := svc.checkConfig()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s: %s", r.Status, r.Message)
	}
	if r.Fix == "" {
		t.Fatal("expected fix description")
	}
	// Config should now exist.
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config not created: %v", err)
	}
}

func TestCheckConfig_FixedEnablesDetected(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	// Create a cursor dir so it's detected.
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Config exists but cursor not enabled.
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{}
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})

	svc := &Service{ConfigPath: cfgPath, StateRoot: stateRoot, Scope: config.ScopeGlobal}
	r := svc.checkConfig()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
}

// --- check 2: state ---

func TestCheckState_OK(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkState()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s", r.Status)
	}
}

func TestCheckState_FixedCorrupt(t *testing.T) {
	_, _, stateRoot := setupTestEnv(t)
	if err := store.EnsureLayout(stateRoot); err != nil {
		t.Fatal(err)
	}
	// Write corrupt state.
	statePath := store.StatePath(stateRoot)
	if err := os.WriteFile(statePath, []byte("not valid toml {{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &Service{StateRoot: stateRoot}
	r := svc.checkState()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s: %s", r.Status, r.Message)
	}
	// State should be loadable now.
	st, err := store.LoadState(stateRoot)
	if err != nil {
		t.Fatalf("state still corrupt: %v", err)
	}
	if st.Version != store.StateVersion {
		t.Fatalf("expected version %d, got %d", store.StateVersion, st.Version)
	}
}

// --- check 3: installed-dirs ---

func TestCheckInstalledDirs_OK(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	st := store.State{Version: store.StateVersion, Installed: []store.InstalledSkill{
		{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "abc"},
	}}
	saveState(t, stateRoot, st)
	// Create matching dir.
	dirName := safeEntryName("hub/demo") + "@1.0.0"
	if err := os.MkdirAll(filepath.Join(store.InstalledRoot(stateRoot), dirName), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInstalledDirs()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s: %s", r.Status, r.Fix)
	}
}

func TestCheckInstalledDirs_Ghost(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	// State has entry but dir does NOT exist.
	st := store.State{Version: store.StateVersion, Installed: []store.InstalledSkill{
		{SkillRef: "hub/ghost", ResolvedVersion: "1.0.0", Source: "hub", Skill: "ghost", Checksum: "abc", SourceRef: "abc"},
	}}
	saveState(t, stateRoot, st)
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInstalledDirs()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// State should no longer have the ghost entry.
	reloaded, _ := store.LoadState(stateRoot)
	if len(reloaded.Installed) != 0 {
		t.Fatalf("expected ghost removed from state, got %d", len(reloaded.Installed))
	}
}

func TestCheckInstalledDirs_Orphan(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	// Create orphan dir on disk (no matching state entry).
	orphanDir := filepath.Join(store.InstalledRoot(stateRoot), "unknown_skill@v0.0.0")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInstalledDirs()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// Orphan dir should be removed.
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Fatal("orphan dir should be removed")
	}
}

// --- check 4: injections ---

func TestCheckInjections_OK(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "abc"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/demo"}},
		},
	}
	saveState(t, stateRoot, st)
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInjections()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s: %s", r.Status, r.Fix)
	}
}

func TestCheckInjections_StaleRef(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	// Injection references an uninstalled skill.
	st := store.State{
		Version:   store.StateVersion,
		Installed: []store.InstalledSkill{},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/gone"}},
		},
	}
	saveState(t, stateRoot, st)
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInjections()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// State injections should be empty (agent entry removed because no valid skills).
	reloaded, _ := store.LoadState(stateRoot)
	if len(reloaded.Injections) != 0 {
		t.Fatalf("expected injections cleared, got %d", len(reloaded.Injections))
	}
}

func TestCheckInjections_PartialStale(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/keep", ResolvedVersion: "1.0.0", Source: "hub", Skill: "keep", Checksum: "abc", SourceRef: "abc"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/keep", "hub/gone"}},
		},
	}
	saveState(t, stateRoot, st)
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkInjections()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	reloaded, _ := store.LoadState(stateRoot)
	if len(reloaded.Injections) != 1 {
		t.Fatalf("expected 1 injection entry, got %d", len(reloaded.Injections))
	}
	if len(reloaded.Injections[0].Skills) != 1 || reloaded.Injections[0].Skills[0] != "hub/keep" {
		t.Fatalf("expected only hub/keep, got %v", reloaded.Injections[0].Skills)
	}
}

// --- check 5: adapter-state ---

func TestCheckAdapterState_OK(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	_ = home
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkAdapterState()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s", r.Status)
	}
}

func TestCheckAdapterState_NilRuntime(t *testing.T) {
	svc := &Service{StateRoot: t.TempDir()}
	if err := store.EnsureLayout(svc.StateRoot); err != nil {
		t.Fatal(err)
	}
	saveState(t, svc.StateRoot, store.State{Version: store.StateVersion})
	r := svc.checkAdapterState()
	if r.Status != StatusOK {
		t.Fatalf("expected ok with nil runtime, got %s", r.Status)
	}
}

func TestCheckAdapterState_Drift(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	// Create claude adapter dir so it works.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "claude", Enabled: true, Scope: "global"}}
	saveConfig(t, cfgPath, cfg)

	// Create an installed skill.
	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "abc"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/demo"}, UpdatedAt: time.Now()},
		},
	}
	saveState(t, stateRoot, st)

	// Create installed dir with a skill file.
	dirName := safeEntryName("hub/demo") + "@1.0.0"
	skillDir := filepath.Join(store.InstalledRoot(stateRoot), dirName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	// adapter's injected.toml is empty (drift).
	adapterDir := filepath.Join(claudeDir, "skillpm")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	type injectedState struct {
		Skills []string `toml:"skills"`
	}
	blob, _ := toml.Marshal(injectedState{Skills: []string{}})
	if err := os.WriteFile(filepath.Join(adapterDir, "injected.toml"), blob, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkAdapterState()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s: %s", r.Status, r.Message)
	}
}

// --- check 6: agent-skills ---

func TestCheckAgentSkills_OK(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	claudeSkillsDir := filepath.Join(home, ".claude", "skills", "demo")
	if err := os.MkdirAll(claudeSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "abc"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/demo"}},
		},
	}
	saveState(t, stateRoot, st)
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkAgentSkills()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s: %s", r.Status, r.Fix)
	}
}

func TestCheckAgentSkills_FixedMissing(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	_ = home
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)

	// Installed dir has the skill.
	dirName := safeEntryName("hub/demo") + "@1.0.0"
	skillSrc := filepath.Join(store.InstalledRoot(stateRoot), dirName)
	if err := os.MkdirAll(skillSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "abc"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/demo"}},
		},
	}
	saveState(t, stateRoot, st)

	// Agent skills dir does NOT have the file (missing).
	svc := newService(t, cfgPath, stateRoot, "", "", config.ScopeGlobal)
	r := svc.checkAgentSkills()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// Should have been copied.
	destPath := filepath.Join(home, ".claude", "skills", "demo", "SKILL.md")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected skill file restored: %v", err)
	}
}

// --- check 7: lockfile ---

func TestCheckLockfile_OK(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	lockPath := filepath.Join(stateRoot, "skills.lock")

	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "hub@main"},
		},
	}
	saveState(t, stateRoot, st)

	lock := store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Checksum: "abc", SourceRef: "hub@main"},
		},
	}
	if err := store.SaveLockfile(lockPath, lock); err != nil {
		t.Fatal(err)
	}

	svc := newService(t, cfgPath, stateRoot, lockPath, "", config.ScopeGlobal)
	r := svc.checkLockfile()
	if r.Status != StatusOK {
		t.Fatalf("expected ok, got %s: %s", r.Status, r.Fix)
	}
}

func TestCheckLockfile_Stale(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	lockPath := filepath.Join(stateRoot, "skills.lock")

	// State is empty but lock has an entry.
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	lock := store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{
			{SkillRef: "hub/stale", ResolvedVersion: "1.0.0", Checksum: "abc", SourceRef: "hub@main"},
		},
	}
	if err := store.SaveLockfile(lockPath, lock); err != nil {
		t.Fatal(err)
	}

	svc := newService(t, cfgPath, stateRoot, lockPath, "", config.ScopeGlobal)
	r := svc.checkLockfile()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// Lock should be empty now.
	reloaded, _ := store.LoadLockfile(lockPath)
	if len(reloaded.Skills) != 0 {
		t.Fatalf("expected stale entry removed, got %d", len(reloaded.Skills))
	}
}

func TestCheckLockfile_Missing(t *testing.T) {
	_, cfgPath, stateRoot := setupTestEnv(t)
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	lockPath := filepath.Join(stateRoot, "skills.lock")

	// State has entry but lock is empty.
	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "hub@main"},
		},
	}
	saveState(t, stateRoot, st)
	if err := store.SaveLockfile(lockPath, store.Lockfile{Version: store.LockVersion}); err != nil {
		t.Fatal(err)
	}

	svc := newService(t, cfgPath, stateRoot, lockPath, "", config.ScopeGlobal)
	r := svc.checkLockfile()
	if r.Status != StatusFixed {
		t.Fatalf("expected fixed, got %s", r.Status)
	}
	// Lock should now have the entry.
	reloaded, _ := store.LoadLockfile(lockPath)
	if len(reloaded.Skills) != 1 || reloaded.Skills[0].SkillRef != "hub/demo" {
		t.Fatalf("expected hub/demo in lock, got %v", reloaded.Skills)
	}
}

func TestCheckLockfile_NoPath(t *testing.T) {
	svc := &Service{LockPath: ""}
	r := svc.checkLockfile()
	if r.Status != StatusOK {
		t.Fatalf("expected ok when no lockpath, got %s", r.Status)
	}
}

// --- idempotency ---

func TestRunIdempotent(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	lockPath := filepath.Join(stateRoot, "skills.lock")

	// Create claude dir so it's detected and enabled.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a complete, consistent setup.
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "claude", Enabled: true, Scope: "global"}}
	saveConfig(t, cfgPath, cfg)

	dirName := safeEntryName("hub/demo") + "@1.0.0"
	skillDir := filepath.Join(store.InstalledRoot(stateRoot), dirName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Source: "hub", Skill: "demo", Checksum: "abc", SourceRef: "hub@main"},
		},
		Injections: []store.InjectionState{
			{Agent: "claude", Skills: []string{"hub/demo"}, UpdatedAt: time.Now()},
		},
	}
	saveState(t, stateRoot, st)

	// Setup adapter state.
	adapterDir := filepath.Join(claudeDir, "skillpm")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	type injState struct {
		Skills []string `toml:"skills"`
	}
	blob, _ := toml.Marshal(injState{Skills: []string{"hub/demo"}})
	if err := os.WriteFile(filepath.Join(adapterDir, "injected.toml"), blob, 0o644); err != nil {
		t.Fatal(err)
	}

	// Setup agent skills dir.
	agentSkill := filepath.Join(claudeDir, "skills", "demo")
	if err := os.MkdirAll(agentSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentSkill, "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Setup lockfile.
	lock := store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{
			{SkillRef: "hub/demo", ResolvedVersion: "1.0.0", Checksum: "abc", SourceRef: "hub@main"},
		},
	}
	if err := store.SaveLockfile(lockPath, lock); err != nil {
		t.Fatal(err)
	}

	// Run 1.
	svc := newService(t, cfgPath, stateRoot, lockPath, "", config.ScopeGlobal)
	r1 := svc.Run(context.Background())

	// Run 2: should be identical (idempotent).
	svc2 := newService(t, cfgPath, stateRoot, lockPath, "", config.ScopeGlobal)
	r2 := svc2.Run(context.Background())

	// Both should have zero fixes on a consistent system.
	for _, r := range []Report{r1, r2} {
		for _, c := range r.Checks {
			if c.Status != StatusOK {
				t.Errorf("run: check %s expected ok, got %s (fix: %s)", c.Name, c.Status, c.Fix)
			}
		}
	}
	if r1.Fixed != 0 || r2.Fixed != 0 {
		t.Fatalf("expected 0 fixes on consistent system, got run1=%d run2=%d", r1.Fixed, r2.Fixed)
	}
}

func TestRunIdempotent_FixThenClean(t *testing.T) {
	home, cfgPath, stateRoot := setupTestEnv(t)
	lockPath := filepath.Join(stateRoot, "skills.lock")

	// Create empty state and an orphan dir.
	cfg := config.DefaultConfig()
	saveConfig(t, cfgPath, cfg)
	saveState(t, stateRoot, store.State{Version: store.StateVersion})
	orphanDir := filepath.Join(store.InstalledRoot(stateRoot), "orphan_skill@v0.0.0")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLockfile(lockPath, store.Lockfile{Version: store.LockVersion}); err != nil {
		t.Fatal(err)
	}

	_ = home
	// Run 1: should fix.
	svc := &Service{
		ConfigPath: cfgPath,
		StateRoot:  stateRoot,
		LockPath:   lockPath,
		Scope:      config.ScopeGlobal,
	}
	r1 := svc.Run(context.Background())
	if r1.Fixed == 0 {
		t.Fatal("expected at least 1 fix on run 1")
	}

	// Run 2: everything should be ok.
	svc2 := &Service{
		ConfigPath: cfgPath,
		StateRoot:  stateRoot,
		LockPath:   lockPath,
		Scope:      config.ScopeGlobal,
	}
	r2 := svc2.Run(context.Background())
	for _, c := range r2.Checks {
		if c.Status == StatusFixed {
			t.Errorf("run 2: check %s should be ok, got fixed (fix: %s)", c.Name, c.Fix)
		}
	}
}
