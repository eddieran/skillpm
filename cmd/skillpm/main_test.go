package main

import (
	"bytes"
	"encoding/json"
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

func TestNewRootCmdIncludesCoreCommands(t *testing.T) {
	cmd := newRootCmd()
	got := map[string]bool{}
	for _, c := range cmd.Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"source", "search", "install", "uninstall", "upgrade", "inject", "remove", "sync", "schedule", "harvest", "validate", "doctor", "self"} {
		if !got[want] {
			t.Fatalf("expected command %q", want)
		}
	}
}

func TestInjectRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newInjectCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	}, boolPtr(false))
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
	}, boolPtr(false))
	cmd.SetArgs([]string{"demo/skill"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestHarvestRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newHarvestCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	}, boolPtr(false))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestPrintMessageAndJSON(t *testing.T) {
	msgOut := captureStdout(t, func() {
		if err := print(false, nil, "ok-message"); err != nil {
			t.Fatalf("print message failed: %v", err)
		}
	})
	if !strings.Contains(msgOut, "ok-message") {
		t.Fatalf("expected message output, got %q", msgOut)
	}

	jsonOut := captureStdout(t, func() {
		if err := print(true, map[string]string{"k": "v"}, "ignored"); err != nil {
			t.Fatalf("print json failed: %v", err)
		}
	})
	var parsed map[string]string
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("expected valid json output, got %q: %v", jsonOut, err)
	}
	if parsed["k"] != "v" {
		t.Fatalf("unexpected json payload: %+v", parsed)
	}
}

func TestSyncCmdHasDryRunFlag(t *testing.T) {
	cmd := newSyncCmd(func() (*app.Service, error) {
		t.Fatalf("newSvc should not be called for flag check")
		return nil, nil
	}, boolPtr(false))
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Fatalf("expected --dry-run flag to be registered")
	}
}

func TestSyncDryRunOutputShowsPlanAndSkipsMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       "https://example.com/skills.git",
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
	}, boolPtr(false))
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync dry-run failed: %v", err)
		}
	})
	if !strings.Contains(out, "sync plan (dry-run):") {
		t.Fatalf("expected dry-run plan heading, got %q", out)
	}
	if !strings.Contains(out, "planned source updates: local") {
		t.Fatalf("expected planned source update output, got %q", out)
	}
	if !strings.Contains(out, "planned upgrades: local/forms") {
		t.Fatalf("expected planned upgrade output, got %q", out)
	}
	if !strings.Contains(out, "planned reinjections: ghost") {
		t.Fatalf("expected planned reinjection output, got %q", out)
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

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       "https://example.com/skills.git",
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
	}, boolPtr(false))
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	})
	if !strings.Contains(out, "sync complete: sources=1 upgrades=1 reinjected=0") {
		t.Fatalf("expected sync summary counts, got %q", out)
	}
	if !strings.Contains(out, "updated sources: local") {
		t.Fatalf("expected updated source details, got %q", out)
	}
	if !strings.Contains(out, "upgraded skills: local/forms") {
		t.Fatalf("expected upgraded skill details, got %q", out)
	}
	if !strings.Contains(out, "reinjected agents: none") {
		t.Fatalf("expected reinjected agent details, got %q", out)
	}
	if !strings.Contains(out, "skipped reinjections: none") {
		t.Fatalf("expected skipped reinjection details, got %q", out)
	}
	if !strings.Contains(out, "failed reinjections: none") {
		t.Fatalf("expected failed reinjection details, got %q", out)
	}
}

func boolPtr(v bool) *bool { return &v }
