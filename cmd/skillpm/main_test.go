package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/app"
	"skillpm/internal/config"
	"skillpm/internal/store"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func TestNewRootCmdIncludesCoreCommands(t *testing.T) {
	cmd := newRootCmd()
	got := map[string]bool{}
	for _, c := range cmd.Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"source", "search", "install", "uninstall", "upgrade", "inject", "sync", "schedule", "doctor", "self", "leaderboard"} {
		if !got[want] {
			t.Fatalf("expected command %q", want)
		}
	}
}

func TestScheduleEnableDisableExecuteWithArgs(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	newSvc := func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}

	installCmd := newScheduleCmd(newSvc)
	installOut := captureStdout(t, func() {
		installCmd.SetArgs([]string{"install", "15m"})
		if err := installCmd.Execute(); err != nil {
			t.Fatalf("schedule install failed: %v", err)
		}
	})
	if !strings.Contains(installOut, "schedule enabled interval=15m") {
		t.Fatalf("expected install output to include enabled interval, got %q", installOut)
	}

	removeCmd := newScheduleCmd(newSvc)
	removeOut := captureStdout(t, func() {
		removeCmd.SetArgs([]string{"remove"})
		if err := removeCmd.Execute(); err != nil {
			t.Fatalf("schedule remove failed: %v", err)
		}
	})
	if !strings.Contains(removeOut, "schedule disabled") {
		t.Fatalf("expected remove output to include disabled message, got %q", removeOut)
	}

	svc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new service for verify failed: %v", err)
	}
	if svc.Config.Sync.Mode != "off" {
		t.Fatalf("expected sync mode off after remove, got %q", svc.Config.Sync.Mode)
	}
	if svc.Config.Sync.Interval != "15m" {
		t.Fatalf("expected sync interval to persist from install argument, got %q", svc.Config.Sync.Interval)
	}
}

func TestScheduleDirectIntervalArgEnablesSchedule(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	cmd := newScheduleCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"20m"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("schedule direct interval failed: %v", err)
		}
	})
	if !strings.Contains(out, "schedule enabled interval=20m") {
		t.Fatalf("expected direct interval output to include enabled interval, got %q", out)
	}
}

func TestScheduleRootAcceptsIntervalFlag(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	cmd := newScheduleCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--interval", "30m"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("schedule --interval failed: %v", err)
		}
	})
	if !strings.Contains(out, "schedule enabled interval=30m") {
		t.Fatalf("expected --interval output to include enabled interval, got %q", out)
	}
}

func TestScheduleRootRejectsConflictingIntervalInputs(t *testing.T) {
	cmd := newScheduleCmd(func() (*app.Service, error) {
		return nil, nil
	})
	cmd.SetArgs([]string{"25m", "--interval", "30m"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "SCH_INTERVAL_CONFLICT") {
		t.Fatalf("expected SCH_INTERVAL_CONFLICT, got %v", err)
	}
}

func TestScheduleWithoutSubcommandShowsCurrentSettings(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	cmd := newScheduleCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("schedule without subcommand failed: %v", err)
		}
	})
	if !strings.Contains(out, "schedule mode=") {
		t.Fatalf("expected schedule mode output, got %q", out)
	}
}

func TestScheduleEnableAcceptsIntervalFlag(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")

	cmd := newScheduleCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"install", "--interval", "20m"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("schedule install with --interval failed: %v", err)
		}
	})
	if !strings.Contains(out, "schedule enabled interval=20m") {
		t.Fatalf("expected --interval output to include enabled interval, got %q", out)
	}
}

func TestScheduleEnableRejectsConflictingIntervalInputs(t *testing.T) {
	cmd := newScheduleCmd(func() (*app.Service, error) {
		return nil, nil
	})
	cmd.SetArgs([]string{"install", "15m", "--interval", "20m"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "SCH_INTERVAL_CONFLICT") {
		t.Fatalf("expected SCH_INTERVAL_CONFLICT, got %v", err)
	}
}

func TestInjectRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newInjectCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	})
	cmd.SetArgs([]string{"demo/skill"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestRemoveRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newRemoveCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	})
	cmd.SetArgs([]string{"demo/skill"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestSyncCmdHasDryRunFlag(t *testing.T) {
	cmd := newSyncCmd(func() (*app.Service, error) {
		t.Fatalf("newSvc should not be called for flag check")
		return nil, nil
	})
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Fatalf("expected --dry-run flag to be registered")
	}
}

func TestSyncDryRunOutputShowsPlanAndSkipsMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/forms"},
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	statePath := store.StatePath(seedSvc.StateRoot)
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before failed: %v", err)
	}
	lockBefore, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock before failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync dry-run failed: %v", err)
		}
	})
	if !strings.Contains(out, "sync plan (dry-run):") {
		t.Fatalf("expected dry-run plan heading, got %q", out)
	}

	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after failed: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatalf("expected state file unchanged in dry-run")
	}
	lockAfter, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock after failed: %v", err)
	}
	if string(lockAfter) != string(lockBefore) {
		t.Fatalf("expected lockfile unchanged in dry-run")
	}
}

func TestSyncOutputShowsAppliedSummaryDetails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: nil,
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	})
	if !strings.Contains(out, "sync complete: sources=1 upgrades=1 reinjected=0") {
		t.Fatalf("expected sync summary counts, got %q", out)
	}

}

func TestSyncOutputShowsChangedWithRiskOutcome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/forms"},
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	})

}

func boolPtr(v bool) *bool { return &v }

func TestSyncCmdStrictFlagFailsOnRisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/forms"},
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	cmd.SetArgs([]string{"--lockfile", lockPath, "--strict"})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected strict sync to fail on risk")
	}
	if !strings.Contains(err.Error(), "SYNC_RISK: sync completed with 1 risk items (strict mode)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSyncCmdStrictFlagFailsOnRiskDuringDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/forms"},
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	cmd.SetArgs([]string{"--lockfile", lockPath, "--strict", "--dry-run"})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected strict dry-run sync to fail on planned risk")
	}
	if !strings.Contains(err.Error(), "SYNC_RISK: sync plan includes 1 risk items (strict mode)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSyncCmdStrictFlagDryRunJSONFailsOnPlannedRisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/forms"},
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	var execErr error
	captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--strict", "--dry-run"})
		execErr = cmd.Execute()
	})
	if execErr == nil {
		t.Fatalf("expected strict dry-run json sync to fail on planned risk")
	}
	if !strings.Contains(execErr.Error(), "SYNC_RISK: sync plan includes 1 risk items (strict mode)") {
		t.Fatalf("unexpected error message: %v", execErr)
	}
}

func TestSyncCmdStrictFlagDryRunSucceedsWithoutPlannedRisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"forms": {"SKILL.md": "# forms\nForms skill"},
	})
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed: []store.InstalledSkill{{
			SkillRef:         "local/forms",
			Source:           "local",
			Skill:            "forms",
			ResolvedVersion:  "1.0.0",
			Checksum:         "sha256:old",
			SourceRef:        "https://example.com/skills.git@1.0.0",
			TrustTier:        "review",
			IsSuspicious:     false,
			IsMalwareBlocked: false,
		}},
		Injections: nil,
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "local/forms",
			ResolvedVersion: "0.0.0+git.latest",
			Checksum:        "sha256:new",
			SourceRef:       "https://example.com/skills.git@0.0.0+git.latest",
		}},
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	cmd.SetArgs([]string{"--lockfile", lockPath, "--strict", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected strict dry-run sync to succeed when planned risk is zero, got: %v", err)
	}
}

func TestLeaderboardDefaultOutput(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	cmd := newLeaderboardCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("leaderboard failed: %v", err)
		}
	})
	if !strings.Contains(out, "Skill Leaderboard") {
		t.Fatalf("expected header, got %q", out)
	}
	if !strings.Contains(out, "SKILL") {
		t.Fatalf("expected column header SKILL, got %q", out)
	}
	if !strings.Contains(out, "DLs") {
		t.Fatalf("expected column header DLs, got %q", out)
	}
	if !strings.Contains(out, "INSTALL COMMAND") {
		t.Fatalf("expected column header INSTALL COMMAND, got %q", out)
	}
	if !strings.Contains(out, "code-review") {
		t.Fatalf("expected top skill code-review, got %q", out)
	}
	if !strings.Contains(out, "Showing 15 entries") {
		t.Fatalf("expected 15 entries footer, got %q", out)
	}
}

func TestLeaderboardCategoryFilter(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	cmd := newLeaderboardCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--category", "security"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("leaderboard --category security failed: %v", err)
		}
	})
	if !strings.Contains(out, "SECURITY") {
		t.Fatalf("expected SECURITY in header, got %q", out)
	}
	if !strings.Contains(out, "secret-scanner") {
		t.Fatalf("expected secret-scanner, got %q", out)
	}
	if strings.Contains(out, "code-review") {
		t.Fatalf("code-review (tool) should not appear in security filter")
	}
}

func TestLeaderboardLimitFlag(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	cmd := newLeaderboardCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--limit", "3"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("leaderboard --limit 3 failed: %v", err)
		}
	})
	if !strings.Contains(out, "Showing 3 entries") {
		t.Fatalf("expected 3 entries footer, got %q", out)
	}
}

func TestLeaderboardInvalidCategory(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	cmd := newLeaderboardCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	})
	cmd.SetArgs([]string{"--category", "bogus"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "LB_CATEGORY") {
		t.Fatalf("expected LB_CATEGORY error, got %v", err)
	}
}

func TestFormatDownloads(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{12480, "12,480"},
		{1000000, "1,000,000"},
	}
	for _, tt := range tests {
		got := formatDownloads(tt.in)
		if got != tt.want {
			t.Fatalf("formatDownloads(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
