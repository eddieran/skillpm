package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillpm/internal/app"
	"skillpm/internal/config"
	"skillpm/internal/store"
	syncsvc "skillpm/internal/sync"
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
	if !strings.Contains(out, "planned strict failure reason: strict-disabled") {
		t.Fatalf("expected planned strict failure reason output, got %q", out)
	}
	if !strings.Contains(out, "planned actions total: 3") {
		t.Fatalf("expected planned actions total output, got %q", out)
	}
	if !strings.Contains(out, "planned outcome: changed-with-risk") {
		t.Fatalf("expected planned outcome output, got %q", out)
	}
	if !strings.Contains(out, "planned progress status: progress-made") {
		t.Fatalf("expected planned progress status output, got %q", out)
	}
	if !strings.Contains(out, "planned progress hotspot: local/forms") {
		t.Fatalf("expected planned progress hotspot output, got %q", out)
	}
	if !strings.Contains(out, "planned progress focus: local/forms") {
		t.Fatalf("expected planned progress focus output, got %q", out)
	}
	if !strings.Contains(out, "planned progress target: local/forms") {
		t.Fatalf("expected planned progress target output, got %q", out)
	}
	if !strings.Contains(out, "planned progress signal: upgrade:local/forms") {
		t.Fatalf("expected planned progress signal output, got %q", out)
	}
	if !strings.Contains(out, "planned actions breakdown: sources=1 upgrades=1 reinjected=0 skipped=0 failed=1") {
		t.Fatalf("expected planned action breakdown output, got %q", out)
	}
	if !strings.Contains(out, "planned action samples: sources=local upgrades=local/forms reinjected=none") {
		t.Fatalf("expected planned action samples output, got %q", out)
	}
	if !strings.Contains(out, "planned next action: resolve-failures-then-apply-plan") {
		t.Fatalf("expected planned next action output, got %q", out)
	}
	if !strings.Contains(out, "planned primary action: Sync plan includes progress with failed reinjections; clear failures before applying this iteration.") {
		t.Fatalf("expected planned primary action output, got %q", out)
	}
	if !strings.Contains(out, "planned execution priority: stabilize-failures") {
		t.Fatalf("expected planned execution priority output, got %q", out)
	}
	if !strings.Contains(out, "planned follow-up gate: blocked-by-risk") {
		t.Fatalf("expected planned follow-up gate output, got %q", out)
	}
	if !strings.Contains(out, "planned next step hint: reinject-failed-agents") {
		t.Fatalf("expected planned next step hint output, got %q", out)
	}
	if !strings.Contains(out, "planned recommended command: skillpm inject --agent ghost <skill-ref>") {
		t.Fatalf("expected planned recommended command output, got %q", out)
	}
	if !strings.Contains(out, "planned recommended agent: ghost") {
		t.Fatalf("expected planned recommended agent output, got %q", out)
	}
	if !strings.Contains(out, "planned summary line: outcome=changed-with-risk progress=2 risk=1 mode=dry-run") {
		t.Fatalf("expected planned summary line output, got %q", out)
	}
	if !strings.Contains(out, "planned can proceed: false") {
		t.Fatalf("expected planned can proceed output, got %q", out)
	}
	if !strings.Contains(out, "planned next batch ready: false") {
		t.Fatalf("expected planned next batch ready output, got %q", out)
	}
	if !strings.Contains(out, "planned next batch blocker: risk-present") {
		t.Fatalf("expected planned next batch blocker output, got %q", out)
	}
	if !strings.Contains(out, "planned risk status: attention-needed") {
		t.Fatalf("expected planned risk status output, got %q", out)
	}
	if !strings.Contains(out, "planned risk level: high") {
		t.Fatalf("expected planned risk level output, got %q", out)
	}
	if !strings.Contains(out, "planned risk class: failed-only") {
		t.Fatalf("expected planned risk class output, got %q", out)
	}
	if !strings.Contains(out, "planned risk breakdown: skipped=0 failed=1") {
		t.Fatalf("expected planned risk breakdown output, got %q", out)
	}
	if !strings.Contains(out, "planned risk inject commands: skillpm inject --agent ghost <skill-ref>") {
		t.Fatalf("expected planned risk inject commands output, got %q", out)
	}
	if !strings.Contains(out, "planned risk hotspot: ghost (ADP_NOT_SUPPORTED: adapter \"ghost\" is not configured)") {
		t.Fatalf("expected planned risk hotspot output, got %q", out)
	}
	if !strings.Contains(out, "planned risk agents total: 1") {
		t.Fatalf("expected planned risk agents total output, got %q", out)
	}
	if !strings.Contains(out, "planned risk samples: skipped=none failed=ghost (ADP_NOT_SUPPORTED: adapter \"ghost\" is not configured)") {
		t.Fatalf("expected planned risk samples output, got %q", out)
	}
	if !strings.Contains(out, "planned source updates: local") {
		t.Fatalf("expected planned source update output, got %q", out)
	}
	if !strings.Contains(out, "planned upgrades: local/forms") {
		t.Fatalf("expected planned upgrade output, got %q", out)
	}
	if !strings.Contains(out, "planned reinjections: none") {
		t.Fatalf("expected planned reinjection output, got %q", out)
	}
	if !strings.Contains(out, "planned skipped reinjections: none") {
		t.Fatalf("expected planned skipped reinjections output, got %q", out)
	}
	if !strings.Contains(out, "planned failed reinjections: ghost (ADP_NOT_SUPPORTED: adapter \"ghost\" is not configured)") {
		t.Fatalf("expected planned failed reinjections output, got %q", out)
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
	if !strings.Contains(out, "applied actions total: 2") {
		t.Fatalf("expected applied actions total output, got %q", out)
	}
	if !strings.Contains(out, "applied outcome: changed") {
		t.Fatalf("expected applied outcome output, got %q", out)
	}
	if !strings.Contains(out, "applied progress status: progress-made") {
		t.Fatalf("expected applied progress status output, got %q", out)
	}
	if !strings.Contains(out, "applied progress hotspot: local/forms") {
		t.Fatalf("expected applied progress hotspot output, got %q", out)
	}
	if !strings.Contains(out, "applied progress focus: local/forms") {
		t.Fatalf("expected applied progress focus output, got %q", out)
	}
	if !strings.Contains(out, "applied progress target: local/forms") {
		t.Fatalf("expected applied progress target output, got %q", out)
	}
	if !strings.Contains(out, "applied progress signal: upgrade:local/forms") {
		t.Fatalf("expected applied progress signal output, got %q", out)
	}
	if !strings.Contains(out, "applied actions breakdown: sources=1 upgrades=1 reinjected=0 skipped=0 failed=0") {
		t.Fatalf("expected applied action breakdown output, got %q", out)
	}
	if !strings.Contains(out, "applied action samples: sources=local upgrades=local/forms reinjected=none") {
		t.Fatalf("expected applied action samples output, got %q", out)
	}
	if !strings.Contains(out, "applied next action: verify-and-continue") {
		t.Fatalf("expected applied next action output, got %q", out)
	}
	if !strings.Contains(out, "applied primary action: Progress is applied and clear; move directly to the next feature increment.") {
		t.Fatalf("expected primary action output, got %q", out)
	}
	if !strings.Contains(out, "applied execution priority: feature-iteration") {
		t.Fatalf("expected execution priority output, got %q", out)
	}
	if !strings.Contains(out, "applied follow-up gate: ready-for-next-iteration") {
		t.Fatalf("expected follow-up gate output, got %q", out)
	}
	if !strings.Contains(out, "applied next step hint: start-next-feature-iteration") {
		t.Fatalf("expected next step hint output, got %q", out)
	}
	if !strings.Contains(out, "applied recommended command: skillpm source list") {
		t.Fatalf("expected recommended command output, got %q", out)
	}
	if !strings.Contains(out, "applied recommended agent: none") {
		t.Fatalf("expected recommended agent output, got %q", out)
	}
	if !strings.Contains(out, "applied summary line: outcome=changed progress=2 risk=0 mode=apply") {
		t.Fatalf("expected summary line output, got %q", out)
	}
	if !strings.Contains(out, "applied can proceed: true") {
		t.Fatalf("expected can proceed output, got %q", out)
	}
	if !strings.Contains(out, "applied next batch ready: true") {
		t.Fatalf("expected next batch ready output, got %q", out)
	}
	if !strings.Contains(out, "applied next batch blocker: none") {
		t.Fatalf("expected next batch blocker output, got %q", out)
	}
	if !strings.Contains(out, "applied risk items total: 0") {
		t.Fatalf("expected risk item total output, got %q", out)
	}
	if !strings.Contains(out, "applied risk status: clear") {
		t.Fatalf("expected risk status output, got %q", out)
	}
	if !strings.Contains(out, "applied risk level: none") {
		t.Fatalf("expected risk level output, got %q", out)
	}
	if !strings.Contains(out, "applied risk breakdown: skipped=0 failed=0") {
		t.Fatalf("expected risk breakdown output, got %q", out)
	}
	if !strings.Contains(out, "applied risk inject commands: none") {
		t.Fatalf("expected risk inject commands output, got %q", out)
	}
	if !strings.Contains(out, "applied risk hotspot: none") {
		t.Fatalf("expected risk hotspot output, got %q", out)
	}
	if !strings.Contains(out, "applied risk samples: skipped=none failed=none") {
		t.Fatalf("expected risk samples output, got %q", out)
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

func TestSyncOutputShowsChangedWithRiskOutcome(t *testing.T) {
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

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(false))
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	})
	if !strings.Contains(out, "applied outcome: changed-with-risk") {
		t.Fatalf("expected changed-with-risk outcome output, got %q", out)
	}
	if !strings.Contains(out, "applied risk status: attention-needed") {
		t.Fatalf("expected attention-needed risk status output, got %q", out)
	}
	if !strings.Contains(out, "applied risk level: high") {
		t.Fatalf("expected high risk level output, got %q", out)
	}
	if !strings.Contains(out, "applied risk class: failed-only") {
		t.Fatalf("expected failed-only risk class output, got %q", out)
	}
	if !strings.Contains(out, "applied risk breakdown: skipped=0 failed=1") {
		t.Fatalf("expected failed risk breakdown output, got %q", out)
	}
	if !strings.Contains(out, "applied recommended command: skillpm inject --agent ghost <skill-ref>") {
		t.Fatalf("expected remediation command output, got %q", out)
	}
	if !strings.Contains(out, "applied recommended commands: skillpm inject --agent ghost <skill-ref> -> skillpm source list -> go test ./... -> skillpm sync --dry-run") {
		t.Fatalf("expected remediation command sequence output, got %q", out)
	}
	if !strings.Contains(out, "applied recommended agent: ghost") {
		t.Fatalf("expected remediation agent output, got %q", out)
	}
	if !strings.Contains(out, "applied can proceed: false") {
		t.Fatalf("expected can proceed output, got %q", out)
	}
	if !strings.Contains(out, "applied next batch blocker: risk-present") {
		t.Fatalf("expected next batch blocker output, got %q", out)
	}
	if !strings.Contains(out, "applied risk hotspot: ghost (ADP_NOT_SUPPORTED:") {
		t.Fatalf("expected risk hotspot output, got %q", out)
	}
	if !strings.Contains(out, "applied risk agents total: 1") {
		t.Fatalf("expected risk agents total output, got %q", out)
	}
	if !strings.Contains(out, "applied risk agents: ghost") {
		t.Fatalf("expected risk agents output, got %q", out)
	}
	if !strings.Contains(out, "failed reinjections: ghost (ADP_NOT_SUPPORTED:") {
		t.Fatalf("expected failed reinjection details, got %q", out)
	}
}

func TestSyncJSONOutputIncludesStructuredSummaryForDryRun(t *testing.T) {
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

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(true))
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync dry-run failed: %v", err)
		}
	})
	got, keys := decodeSyncJSONOutput(t, out)

	for _, key := range []string{"schemaVersion", "actionCounts", "riskCounts", "outcome", "progressStatus", "progressClass", "progressHotspot", "progressFocus", "progressTarget", "progressSignal", "actionBreakdown", "nextAction", "primaryAction", "executionPriority", "followUpGate", "nextStepHint", "recommendedCommand", "recommendedCommands", "recommendedAgent", "summaryLine", "noopReason", "riskStatus", "riskLevel", "riskClass", "riskBreakdown", "riskInjectCommands", "riskHotspot", "riskAgents", "riskAgentsTotal", "topSamples", "dryRun", "strictMode", "strictStatus", "strictFailureReason", "mode", "hasProgress", "hasRisk", "canProceed", "nextBatchReady", "nextBatchBlocker"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected key %q in json output, got %q", key, out)
		}
	}
	if got.Mode != "dry-run" {
		t.Fatalf("expected dry-run mode, got %q", got.Mode)
	}
	if got.RiskAgentsTotal != 1 {
		t.Fatalf("expected riskAgentsTotal=1, got %d", got.RiskAgentsTotal)
	}
	if !got.DryRun {
		t.Fatalf("expected dryRun=true")
	}
	if got.StrictMode {
		t.Fatalf("expected strictMode=false")
	}
	if got.StrictStatus != "disabled" {
		t.Fatalf("expected strictStatus=disabled, got %q", got.StrictStatus)
	}
	if got.StrictFailureReason != "strict-disabled" {
		t.Fatalf("expected strictFailureReason=strict-disabled, got %q", got.StrictFailureReason)
	}
	if got.Outcome != "changed-with-risk" {
		t.Fatalf("expected changed-with-risk outcome, got %q", got.Outcome)
	}
	if got.ProgressStatus != "progress-made" {
		t.Fatalf("expected progress-made status, got %q", got.ProgressStatus)
	}
	if got.ProgressClass != "upgrade" {
		t.Fatalf("expected upgrade progress class, got %q", got.ProgressClass)
	}
	if got.ProgressHotspot != "local/forms" {
		t.Fatalf("expected local/forms progress hotspot, got %q", got.ProgressHotspot)
	}
	if got.ProgressFocus != "local/forms" {
		t.Fatalf("expected local/forms progress focus, got %q", got.ProgressFocus)
	}
	if got.ProgressTarget != "local/forms" {
		t.Fatalf("expected local/forms progress target, got %q", got.ProgressTarget)
	}
	if got.ProgressSignal != "upgrade:local/forms" {
		t.Fatalf("expected upgrade:local/forms progress signal, got %q", got.ProgressSignal)
	}
	if got.ActionBreakdown != "sources=1 upgrades=1 reinjected=0 skipped=0 failed=1" {
		t.Fatalf("expected action breakdown, got %q", got.ActionBreakdown)
	}
	if got.NextAction != "resolve-failures-then-apply-plan" {
		t.Fatalf("expected apply-plan next action, got %q", got.NextAction)
	}
	if got.PrimaryAction != "Sync plan includes progress with failed reinjections; clear failures before applying this iteration." {
		t.Fatalf("unexpected primary action, got %q", got.PrimaryAction)
	}
	if got.ExecutionPriority != "stabilize-failures" {
		t.Fatalf("expected apply-feature-iteration execution priority, got %q", got.ExecutionPriority)
	}
	if got.FollowUpGate != "blocked-by-risk" {
		t.Fatalf("expected blocked-by-risk follow-up gate, got %q", got.FollowUpGate)
	}
	if got.NextStepHint != "reinject-failed-agents" {
		t.Fatalf("expected reinject-failed-agents next step hint, got %q", got.NextStepHint)
	}
	if got.RecommendedCommand != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("expected skillpm sync recommended command, got %q", got.RecommendedCommand)
	}
	if !reflect.DeepEqual(got.RecommendedCommands, []string{"skillpm inject --agent ghost <skill-ref>", "skillpm source list", "skillpm sync --dry-run", "skillpm sync", "go test ./..."}) {
		t.Fatalf("expected recommended command sequence for dry-run follow-up validation, got %+v", got.RecommendedCommands)
	}
	if got.RecommendedAgent != "ghost" {
		t.Fatalf("expected none recommended agent, got %q", got.RecommendedAgent)
	}
	if got.SummaryLine != "outcome=changed-with-risk progress=2 risk=1 mode=dry-run" {
		t.Fatalf("unexpected summary line, got %q", got.SummaryLine)
	}
	if got.NoopReason != "not-applicable" {
		t.Fatalf("expected not-applicable noop reason, got %q", got.NoopReason)
	}
	if got.RiskStatus != "attention-needed" {
		t.Fatalf("expected clear risk status, got %q", got.RiskStatus)
	}
	if got.RiskLevel != "high" {
		t.Fatalf("expected high risk level, got %q", got.RiskLevel)
	}
	if got.RiskClass != "failed-only" {
		t.Fatalf("expected failed-only risk class, got %q", got.RiskClass)
	}
	if got.RiskBreakdown != "skipped=0 failed=1" {
		t.Fatalf("expected zero risk breakdown, got %q", got.RiskBreakdown)
	}
	if got.RiskHotspot != "ghost (ADP_NOT_SUPPORTED: adapter \"ghost\" is not configured)" {
		t.Fatalf("expected none risk hotspot, got %q", got.RiskHotspot)
	}
	if !reflect.DeepEqual(got.RiskAgents, []string{"ghost"}) {
		t.Fatalf("expected risk agents [ghost], got %+v", got.RiskAgents)
	}
	if !got.HasProgress || !got.HasRisk || got.CanProceed || got.NextBatchReady {
		t.Fatalf("expected hasProgress=true hasRisk=true canProceed=false nextBatchReady=false, got hasProgress=%v hasRisk=%v canProceed=%v nextBatchReady=%v", got.HasProgress, got.HasRisk, got.CanProceed, got.NextBatchReady)
	}
	if got.NextBatchBlocker != "risk-present" {
		t.Fatalf("expected nextBatchBlocker=risk-present, got %q", got.NextBatchBlocker)
	}
	if got.ActionCounts.Sources != 1 || got.ActionCounts.Upgrades != 1 || got.ActionCounts.Reinjected != 0 {
		t.Fatalf("unexpected action counts: %+v", got.ActionCounts)
	}
	if got.ActionCounts.Skipped != 0 || got.ActionCounts.Failed != 1 {
		t.Fatalf("unexpected risk action counts: %+v", got.ActionCounts)
	}
	if got.ActionCounts.ProgressTotal != 2 || got.ActionCounts.RiskTotal != 1 || got.ActionCounts.Total != 3 {
		t.Fatalf("unexpected action totals: %+v", got.ActionCounts)
	}
	if got.RiskCounts.Skipped != 0 || got.RiskCounts.Failed != 1 || got.RiskCounts.Total != 1 {
		t.Fatalf("unexpected risk counts: %+v", got.RiskCounts)
	}
	if len(got.TopSamples.Sources.Items) != 1 || got.TopSamples.Sources.Items[0] != "local" || got.TopSamples.Sources.Remaining != 0 {
		t.Fatalf("unexpected source sample: %+v", got.TopSamples.Sources)
	}
	if len(got.TopSamples.Upgrades.Items) != 1 || got.TopSamples.Upgrades.Items[0] != "local/forms" || got.TopSamples.Upgrades.Remaining != 0 {
		t.Fatalf("unexpected upgrade sample: %+v", got.TopSamples.Upgrades)
	}
	if len(got.TopSamples.Reinjected.Items) != 0 || got.TopSamples.Reinjected.Remaining != 0 {
		t.Fatalf("unexpected reinjected sample: %+v", got.TopSamples.Reinjected)
	}
	if len(got.TopSamples.Failed.Items) != 1 || got.TopSamples.Failed.Items[0] != "ghost (ADP_NOT_SUPPORTED: adapter \"ghost\" is not configured)" || got.TopSamples.Failed.Remaining != 0 {
		t.Fatalf("unexpected failed sample: %+v", got.TopSamples.Failed)
	}
	if got.TopSamples.Skipped.Items == nil {
		t.Fatalf("expected stable empty skipped sample array, got %+v", got.TopSamples)
	}
}

func TestSyncJSONOutputIncludesStructuredSummaryForApply(t *testing.T) {
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
	}, boolPtr(true))
	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	})
	got, keys := decodeSyncJSONOutput(t, out)

	for _, key := range []string{"schemaVersion", "actionCounts", "riskCounts", "outcome", "progressStatus", "progressClass", "progressHotspot", "progressFocus", "progressTarget", "progressSignal", "actionBreakdown", "nextAction", "primaryAction", "executionPriority", "followUpGate", "nextStepHint", "recommendedCommand", "recommendedCommands", "recommendedAgent", "summaryLine", "noopReason", "riskStatus", "riskLevel", "riskClass", "riskBreakdown", "riskInjectCommands", "riskHotspot", "riskAgents", "riskAgentsTotal", "topSamples", "dryRun", "strictMode", "strictStatus", "strictFailureReason", "mode", "hasProgress", "hasRisk", "canProceed", "nextBatchReady", "nextBatchBlocker"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected key %q in json output, got %q", key, out)
		}
	}
	if got.Mode != "apply" {
		t.Fatalf("expected apply mode, got %q", got.Mode)
	}
	if got.DryRun {
		t.Fatalf("expected dryRun=false")
	}
	if got.StrictMode {
		t.Fatalf("expected strictMode=false")
	}
	if got.StrictStatus != "disabled" {
		t.Fatalf("expected strictStatus=disabled, got %q", got.StrictStatus)
	}
	if got.StrictFailureReason != "strict-disabled" {
		t.Fatalf("expected strictFailureReason=strict-disabled, got %q", got.StrictFailureReason)
	}
	if got.Outcome != "changed" {
		t.Fatalf("expected changed outcome, got %q", got.Outcome)
	}
	if got.ProgressStatus != "progress-made" {
		t.Fatalf("expected progress-made status, got %q", got.ProgressStatus)
	}
	if got.ProgressClass != "upgrade" {
		t.Fatalf("expected upgrade progress class, got %q", got.ProgressClass)
	}
	if got.ProgressHotspot != "local/forms" {
		t.Fatalf("expected local/forms progress hotspot, got %q", got.ProgressHotspot)
	}
	if got.ProgressFocus != "local/forms" {
		t.Fatalf("expected local/forms progress focus, got %q", got.ProgressFocus)
	}
	if got.ProgressTarget != "local/forms" {
		t.Fatalf("expected local/forms progress target, got %q", got.ProgressTarget)
	}
	if got.ProgressSignal != "upgrade:local/forms" {
		t.Fatalf("expected upgrade:local/forms progress signal, got %q", got.ProgressSignal)
	}
	if got.ActionBreakdown != "sources=1 upgrades=1 reinjected=0 skipped=0 failed=0" {
		t.Fatalf("expected action breakdown, got %q", got.ActionBreakdown)
	}
	if got.NextAction != "verify-and-continue" {
		t.Fatalf("expected verify-and-continue next action, got %q", got.NextAction)
	}
	if got.PrimaryAction != "Progress is applied and clear; move directly to the next feature increment." {
		t.Fatalf("unexpected primary action, got %q", got.PrimaryAction)
	}
	if got.ExecutionPriority != "feature-iteration" {
		t.Fatalf("expected feature-iteration execution priority, got %q", got.ExecutionPriority)
	}
	if got.FollowUpGate != "ready-for-next-iteration" {
		t.Fatalf("expected ready-for-next-iteration follow-up gate, got %q", got.FollowUpGate)
	}
	if got.NextStepHint != "start-next-feature-iteration" {
		t.Fatalf("expected start-next-feature-iteration next step hint, got %q", got.NextStepHint)
	}
	if got.RecommendedCommand != "skillpm source list" {
		t.Fatalf("expected skillpm source list recommended command, got %q", got.RecommendedCommand)
	}
	if !reflect.DeepEqual(got.RecommendedCommands, []string{"skillpm source list", "go test ./...", "skillpm sync --dry-run"}) {
		t.Fatalf("expected recommended command sequence for follow-up monitoring, got %+v", got.RecommendedCommands)
	}
	if got.RecommendedAgent != "none" {
		t.Fatalf("expected none recommended agent, got %q", got.RecommendedAgent)
	}
	if got.SummaryLine != "outcome=changed progress=2 risk=0 mode=apply" {
		t.Fatalf("unexpected summary line, got %q", got.SummaryLine)
	}
	if got.NoopReason != "not-applicable" {
		t.Fatalf("expected not-applicable noop reason, got %q", got.NoopReason)
	}
	if got.RiskStatus != "clear" {
		t.Fatalf("expected clear risk status, got %q", got.RiskStatus)
	}
	if got.RiskLevel != "none" {
		t.Fatalf("expected none risk level, got %q", got.RiskLevel)
	}
	if got.RiskClass != "none" {
		t.Fatalf("expected none risk class, got %q", got.RiskClass)
	}
	if got.RiskBreakdown != "skipped=0 failed=0" {
		t.Fatalf("expected zero risk breakdown, got %q", got.RiskBreakdown)
	}
	if got.RiskHotspot != "none" {
		t.Fatalf("expected none risk hotspot, got %q", got.RiskHotspot)
	}
	if len(got.RiskAgents) != 0 {
		t.Fatalf("expected no risk agents, got %+v", got.RiskAgents)
	}
	if !got.HasProgress || got.HasRisk || !got.CanProceed || !got.NextBatchReady {
		t.Fatalf("expected hasProgress=true hasRisk=false canProceed=true nextBatchReady=true, got hasProgress=%v hasRisk=%v canProceed=%v nextBatchReady=%v", got.HasProgress, got.HasRisk, got.CanProceed, got.NextBatchReady)
	}
	if got.NextBatchBlocker != "none" {
		t.Fatalf("expected nextBatchBlocker=none, got %q", got.NextBatchBlocker)
	}
	if got.ActionCounts.Sources != 1 || got.ActionCounts.Upgrades != 1 || got.ActionCounts.Reinjected != 0 {
		t.Fatalf("unexpected action counts: %+v", got.ActionCounts)
	}
	if got.ActionCounts.Skipped != 0 || got.ActionCounts.Failed != 0 {
		t.Fatalf("unexpected risk action counts: %+v", got.ActionCounts)
	}
	if got.ActionCounts.ProgressTotal != 2 || got.ActionCounts.RiskTotal != 0 || got.ActionCounts.Total != 2 {
		t.Fatalf("unexpected action totals: %+v", got.ActionCounts)
	}
	if got.RiskCounts.Skipped != 0 || got.RiskCounts.Failed != 0 || got.RiskCounts.Total != 0 {
		t.Fatalf("unexpected risk counts: %+v", got.RiskCounts)
	}
	if len(got.TopSamples.Sources.Items) != 1 || got.TopSamples.Sources.Items[0] != "local" || got.TopSamples.Sources.Remaining != 0 {
		t.Fatalf("unexpected source sample: %+v", got.TopSamples.Sources)
	}
	if len(got.TopSamples.Upgrades.Items) != 1 || got.TopSamples.Upgrades.Items[0] != "local/forms" || got.TopSamples.Upgrades.Remaining != 0 {
		t.Fatalf("unexpected upgrade sample: %+v", got.TopSamples.Upgrades)
	}
	if got.TopSamples.Reinjected.Items == nil || got.TopSamples.Skipped.Items == nil || got.TopSamples.Failed.Items == nil {
		t.Fatalf("expected stable empty sample arrays, got %+v", got.TopSamples)
	}
}

func TestSyncProgressClassPriorityAndHotspot(t *testing.T) {
	tests := []struct {
		name    string
		report  syncsvc.Report
		class   string
		hotspot string
		focus   string
		target  string
		signal  string
	}{
		{
			name:    "source refresh only",
			report:  syncsvc.Report{UpdatedSources: []string{"beta", "alpha"}},
			class:   "source-refresh",
			hotspot: "alpha",
			focus:   "alpha",
			target:  "alpha",
			signal:  "source-refresh:alpha",
		},
		{
			name:    "upgrade takes priority over source",
			report:  syncsvc.Report{UpdatedSources: []string{"alpha"}, UpgradedSkills: []string{"zeta/skill", "beta/skill"}},
			class:   "upgrade",
			hotspot: "beta/skill",
			focus:   "beta/skill",
			target:  "beta/skill",
			signal:  "upgrade:beta/skill",
		},
		{
			name:    "reinjection class with upgrade hotspot precedence",
			report:  syncsvc.Report{UpdatedSources: []string{"alpha"}, UpgradedSkills: []string{"beta/skill"}, Reinjected: []string{"agent-z", "agent-a"}},
			class:   "reinjection",
			hotspot: "beta/skill",
			focus:   "agent-a",
			target:  "agent-a",
			signal:  "reinjection:agent-a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncProgressClass(tc.report); got != tc.class {
				t.Fatalf("expected progress class %q, got %q", tc.class, got)
			}
			if got := syncProgressHotspot(tc.report); got != tc.hotspot {
				t.Fatalf("expected progress hotspot %q, got %q", tc.hotspot, got)
			}
			if got := syncProgressFocus(tc.report); got != tc.focus {
				t.Fatalf("expected progress focus %q, got %q", tc.focus, got)
			}
			if got := syncProgressTarget(tc.report); got != tc.target {
				t.Fatalf("expected progress target %q, got %q", tc.target, got)
			}
			if got := syncProgressSignal(tc.report); got != tc.signal {
				t.Fatalf("expected progress signal %q, got %q", tc.signal, got)
			}
		})
	}
}

func TestSyncStrictStatus(t *testing.T) {
	if got := syncStrictStatus(true); got != "enabled" {
		t.Fatalf("expected enabled strict status, got %q", got)
	}
	if got := syncStrictStatus(false); got != "disabled" {
		t.Fatalf("expected disabled strict status, got %q", got)
	}
}

func TestSyncStrictFailureReason(t *testing.T) {
	tests := []struct {
		name   string
		report syncsvc.Report
		strict bool
		want   string
	}{
		{name: "strict disabled", report: syncsvc.Report{}, strict: false, want: "strict-disabled"},
		{name: "strict enabled no risk", report: syncsvc.Report{}, strict: true, want: "none"},
		{name: "strict enabled skipped only", report: syncsvc.Report{SkippedReinjects: []string{"agent-a"}}, strict: true, want: "risk-present-skipped"},
		{name: "strict enabled failed only", report: syncsvc.Report{FailedReinjects: []string{"agent-b"}}, strict: true, want: "risk-present-failed"},
		{name: "strict enabled mixed risk", report: syncsvc.Report{SkippedReinjects: []string{"agent-a"}, FailedReinjects: []string{"agent-b"}}, strict: true, want: "risk-present-mixed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncStrictFailureReason(tc.report, tc.strict); got != tc.want {
				t.Fatalf("expected strict failure reason %q, got %q", tc.want, got)
			}
		})
	}
}

func TestTotalSyncActions(t *testing.T) {
	report := syncReportFixture()
	if got := totalSyncActions(report); got != 7 {
		t.Fatalf("expected total actions 7, got %d", got)
	}
	if got := totalSyncProgressActions(report); got != 4 {
		t.Fatalf("expected total progress actions 4, got %d", got)
	}
	if got := totalSyncIssues(report); got != 3 {
		t.Fatalf("expected total issues 3, got %d", got)
	}
	if got := syncActionBreakdown(report); got != "sources=2 upgrades=1 reinjected=1 skipped=1 failed=2" {
		t.Fatalf("unexpected action breakdown: %q", got)
	}
	if got := syncOutcome(report); got != "changed-with-risk" {
		t.Fatalf("unexpected action outcome: %q", got)
	}
	if got := syncProgressStatus(report); got != "progress-made" {
		t.Fatalf("unexpected progress status: %q", got)
	}
	if got := syncProgressClass(report); got != "reinjection" {
		t.Fatalf("unexpected progress class: %q", got)
	}
	if got := syncProgressHotspot(report); got != "c" {
		t.Fatalf("unexpected progress hotspot: %q", got)
	}
	if got := syncProgressTarget(report); got != "d" {
		t.Fatalf("unexpected progress target: %q", got)
	}
	if got := syncProgressSignal(report); got != "reinjection:d" {
		t.Fatalf("unexpected progress signal: %q", got)
	}
	if got := syncPrimaryAction(report); got != "Progress landed with failed reinjections; fix failures before expanding scope." {
		t.Fatalf("unexpected primary action: %q", got)
	}
	if got := syncNextAction(report); got != "review-failed-risk-items" {
		t.Fatalf("unexpected next action: %q", got)
	}
	if got := syncExecutionPriority(report); got != "stabilize-failures" {
		t.Fatalf("unexpected execution priority: %q", got)
	}
	if got := syncFollowUpGate(report); got != "blocked-by-risk" {
		t.Fatalf("unexpected follow-up gate: %q", got)
	}
	if got := syncRecommendedCommand(report); got != "skillpm inject --agent f <skill-ref>" {
		t.Fatalf("unexpected recommended command: %q", got)
	}
	if got := syncRecommendedAgent(report); got != "f" {
		t.Fatalf("unexpected recommended agent: %q", got)
	}
	if got := syncSummaryLine(report); got != "outcome=changed-with-risk progress=4 risk=3 mode=apply" {
		t.Fatalf("unexpected summary line: %q", got)
	}
	if got := syncNoopReason(report); got != "not-applicable" {
		t.Fatalf("unexpected non-noop reason marker: %q", got)
	}
	if got := syncRiskBreakdown(report); got != "skipped=1 failed=2" {
		t.Fatalf("unexpected risk breakdown: %q", got)
	}
	if got := syncRiskStatus(report); got != "attention-needed" {
		t.Fatalf("unexpected risk status: %q", got)
	}
	if got := syncRiskLevel(report); got != "high" {
		t.Fatalf("unexpected risk level: %q", got)
	}
	if got := syncRiskClass(report); got != "mixed" {
		t.Fatalf("unexpected risk class: %q", got)
	}
	if got := syncRiskHotspot(report); got != "f" {
		t.Fatalf("unexpected risk hotspot: %q", got)
	}
	if got := syncRiskAgents(report); !reflect.DeepEqual(got, []string{"e", "f", "g"}) {
		t.Fatalf("unexpected risk agents: %v", got)
	}

	empty := syncReportFixtureEmpty()
	if got := totalSyncActions(empty); got != 0 {
		t.Fatalf("expected empty total actions 0, got %d", got)
	}
	if got := totalSyncProgressActions(empty); got != 0 {
		t.Fatalf("expected empty progress actions 0, got %d", got)
	}
	if got := totalSyncIssues(empty); got != 0 {
		t.Fatalf("expected empty total issues 0, got %d", got)
	}
	if got := syncActionBreakdown(empty); got != "sources=0 upgrades=0 reinjected=0 skipped=0 failed=0" {
		t.Fatalf("unexpected empty action breakdown: %q", got)
	}
	if got := syncOutcome(empty); got != "noop" {
		t.Fatalf("unexpected empty action outcome: %q", got)
	}
	if got := syncProgressStatus(empty); got != "no-progress" {
		t.Fatalf("unexpected empty progress status: %q", got)
	}
	if got := syncProgressClass(empty); got != "none" {
		t.Fatalf("unexpected empty progress class: %q", got)
	}
	if got := syncProgressHotspot(empty); got != "none" {
		t.Fatalf("unexpected empty progress hotspot: %q", got)
	}
	if got := syncProgressTarget(empty); got != "none" {
		t.Fatalf("unexpected empty progress target: %q", got)
	}
	if got := syncProgressSignal(empty); got != "none" {
		t.Fatalf("unexpected empty progress signal: %q", got)
	}
	if got := syncPrimaryAction(empty); got != "No changes detected; keep monitoring and retry on the next cycle." {
		t.Fatalf("unexpected empty primary action: %q", got)
	}
	if got := syncExecutionPriority(empty); got != "monitor-next-cycle" {
		t.Fatalf("unexpected empty execution priority: %q", got)
	}
	if got := syncFollowUpGate(empty); got != "monitor-next-cycle" {
		t.Fatalf("unexpected empty follow-up gate: %q", got)
	}
	if got := syncRecommendedCommand(empty); got != "skillpm sync --dry-run" {
		t.Fatalf("unexpected empty recommended command: %q", got)
	}
	if got := syncRecommendedCommands(empty); !reflect.DeepEqual(got, []string{"skillpm sync --dry-run", "skillpm source list"}) {
		t.Fatalf("unexpected empty recommended commands: %v", got)
	}
	if got := syncRecommendedAgent(empty); got != "none" {
		t.Fatalf("unexpected empty recommended agent: %q", got)
	}
	if got := syncSummaryLine(empty); got != "outcome=noop progress=0 risk=0 mode=apply" {
		t.Fatalf("unexpected empty summary line: %q", got)
	}
	if got := syncNoopReason(empty); got != "no source updates, skill upgrades, or reinjection changes detected" {
		t.Fatalf("unexpected empty noop reason: %q", got)
	}

	emptyDryRun := syncsvc.Report{DryRun: true}
	if got := syncExecutionPriority(emptyDryRun); got != "plan-feature-iteration" {
		t.Fatalf("unexpected empty dry-run execution priority: %q", got)
	}
	if got := syncFollowUpGate(emptyDryRun); got != "plan-next-iteration" {
		t.Fatalf("unexpected empty dry-run follow-up gate: %q", got)
	}
	if got := syncRecommendedCommand(emptyDryRun); got != "skillpm sync" {
		t.Fatalf("unexpected empty dry-run recommended command: %q", got)
	}
	if got := syncRecommendedCommands(emptyDryRun); !reflect.DeepEqual(got, []string{"skillpm sync", "skillpm source list"}) {
		t.Fatalf("unexpected empty dry-run recommended commands: %v", got)
	}
	if got := syncSummaryLine(emptyDryRun); got != "outcome=noop progress=0 risk=0 mode=dry-run" {
		t.Fatalf("unexpected empty dry-run summary line: %q", got)
	}
	if got := syncProgressTarget(emptyDryRun); got != "none" {
		t.Fatalf("unexpected empty dry-run progress target: %q", got)
	}
	if got := syncProgressSignal(emptyDryRun); got != "none" {
		t.Fatalf("unexpected empty dry-run progress signal: %q", got)
	}
	if got := syncNoopReason(emptyDryRun); got != "dry-run detected no source/upgrade/reinjection deltas" {
		t.Fatalf("unexpected empty dry-run noop reason: %q", got)
	}
	if got := syncRiskBreakdown(empty); got != "skipped=0 failed=0" {
		t.Fatalf("unexpected empty risk breakdown: %q", got)
	}
	if got := syncRiskStatus(empty); got != "clear" {
		t.Fatalf("unexpected empty risk status: %q", got)
	}
	if got := syncRiskLevel(empty); got != "none" {
		t.Fatalf("unexpected empty risk level: %q", got)
	}
	if got := syncRiskClass(empty); got != "none" {
		t.Fatalf("unexpected empty risk class: %q", got)
	}
	if got := syncRiskHotspot(empty); got != "none" {
		t.Fatalf("unexpected empty risk hotspot: %q", got)
	}
	if got := syncRiskAgents(empty); len(got) != 0 {
		t.Fatalf("unexpected empty risk agents: %v", got)
	}
	if got := syncFollowUpGate(empty); got != "monitor-next-cycle" {
		t.Fatalf("unexpected empty follow-up gate: %q", got)
	}
	if got := syncExecutionPriority(empty); got != "monitor-next-cycle" {
		t.Fatalf("unexpected empty execution priority: %q", got)
	}
	if got := syncFollowUpGate(emptyDryRun); got != "plan-next-iteration" {
		t.Fatalf("unexpected empty dry-run follow-up gate: %q", got)
	}
	if got := syncExecutionPriority(emptyDryRun); got != "plan-feature-iteration" {
		t.Fatalf("unexpected empty dry-run execution priority: %q", got)
	}

	blocked := syncsvc.Report{SkippedReinjects: []string{"ghost"}}
	if got := totalSyncActions(blocked); got != 1 {
		t.Fatalf("expected blocked total actions 1, got %d", got)
	}
	if got := totalSyncProgressActions(blocked); got != 0 {
		t.Fatalf("expected blocked progress actions 0, got %d", got)
	}
	if got := totalSyncIssues(blocked); got != 1 {
		t.Fatalf("expected blocked issues 1, got %d", got)
	}
	if got := syncOutcome(blocked); got != "blocked" {
		t.Fatalf("unexpected blocked action outcome: %q", got)
	}
	if got := syncPrimaryAction(blocked); got != "Reinjection is blocked; resolve skipped/failed agents first before adding new work." {
		t.Fatalf("unexpected blocked primary action: %q", got)
	}
	if got := syncNextAction(blocked); got != "resolve-reinjection-skips" {
		t.Fatalf("unexpected blocked next action: %q", got)
	}
	if got := syncRecommendedCommand(blocked); got != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("unexpected blocked recommended command: %q", got)
	}
	if got := syncRecommendedAgent(blocked); got != "ghost" {
		t.Fatalf("unexpected blocked recommended agent: %q", got)
	}
	blockedDryRun := syncsvc.Report{DryRun: true, SkippedReinjects: []string{"ghost"}}
	if got := syncPrimaryAction(blockedDryRun); got != "Sync plan is blocked by reinjection risk; resolve skipped/failed agents before applying changes." {
		t.Fatalf("unexpected blocked dry-run primary action: %q", got)
	}
	if got := syncNextAction(blockedDryRun); got != "resolve-skips-then-apply" {
		t.Fatalf("unexpected blocked dry-run next action: %q", got)
	}
	if got := syncRecommendedCommand(blockedDryRun); got != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("unexpected blocked dry-run recommended command: %q", got)
	}
	if got := syncRecommendedCommands(blockedDryRun); !reflect.DeepEqual(got, []string{"skillpm inject --agent ghost <skill-ref>", "skillpm source list", "skillpm sync --dry-run", "skillpm sync"}) {
		t.Fatalf("unexpected blocked dry-run recommended commands: %v", got)
	}
	if got := syncExecutionPriority(blocked); got != "stabilize-risks" {
		t.Fatalf("unexpected blocked execution priority: %q", got)
	}
	if got := syncFollowUpGate(blocked); got != "blocked-by-risk" {
		t.Fatalf("unexpected blocked follow-up gate: %q", got)
	}
	if got := syncProgressHotspot(blocked); got != "none" {
		t.Fatalf("unexpected blocked progress hotspot: %q", got)
	}
	if got := syncRiskLevel(blocked); got != "medium" {
		t.Fatalf("unexpected blocked risk level: %q", got)
	}
	if got := syncRiskClass(blocked); got != "skipped-only" {
		t.Fatalf("unexpected blocked risk class: %q", got)
	}
	if got := syncRiskHotspot(blocked); got != "ghost" {
		t.Fatalf("unexpected blocked risk hotspot: %q", got)
	}

	changedWithRiskDryRun := syncsvc.Report{DryRun: true, UpgradedSkills: []string{"local/forms"}, FailedReinjects: []string{"ghost (boom)"}}
	if got := syncPrimaryAction(changedWithRiskDryRun); got != "Sync plan includes progress with failed reinjections; clear failures before applying this iteration." {
		t.Fatalf("unexpected changed-with-risk dry-run primary action: %q", got)
	}
	if got := syncNextAction(changedWithRiskDryRun); got != "resolve-failures-then-apply-plan" {
		t.Fatalf("unexpected changed-with-risk dry-run next action: %q", got)
	}
	if got := syncRiskClass(changedWithRiskDryRun); got != "failed-only" {
		t.Fatalf("unexpected changed-with-risk dry-run risk class: %q", got)
	}
	if got := syncRecommendedCommand(changedWithRiskDryRun); got != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("unexpected changed-with-risk dry-run recommended command: %q", got)
	}
	if got := syncRecommendedCommands(changedWithRiskDryRun); !reflect.DeepEqual(got, []string{"skillpm inject --agent ghost <skill-ref>", "skillpm source list", "skillpm sync --dry-run", "skillpm sync", "go test ./..."}) {
		t.Fatalf("unexpected changed-with-risk dry-run recommended commands: %v", got)
	}

	changedWithSkippedRisk := syncsvc.Report{UpdatedSources: []string{"local"}, SkippedReinjects: []string{"ghost"}}
	if got := syncOutcome(changedWithSkippedRisk); got != "changed-with-risk" {
		t.Fatalf("unexpected changed-with-skipped-risk outcome: %q", got)
	}
	if got := syncRecommendedCommand(changedWithSkippedRisk); got != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("unexpected changed-with-skipped-risk recommended command: %q", got)
	}
	if got := syncPrimaryAction(changedWithSkippedRisk); got != "Progress landed with skipped reinjections; clear skips before expanding scope." {
		t.Fatalf("unexpected changed-with-skipped-risk primary action: %q", got)
	}
	if got := syncNextAction(changedWithSkippedRisk); got != "review-skipped-risk-items" {
		t.Fatalf("unexpected changed-with-skipped-risk next action: %q", got)
	}
	if got := syncRecommendedCommands(changedWithSkippedRisk); !reflect.DeepEqual(got, []string{"skillpm inject --agent ghost <skill-ref>", "skillpm source list", "go test ./...", "skillpm sync --dry-run"}) {
		t.Fatalf("unexpected changed-with-skipped-risk recommended commands: %v", got)
	}

	changedWithSkippedRiskDryRun := syncsvc.Report{DryRun: true, UpdatedSources: []string{"local"}, SkippedReinjects: []string{"ghost"}}
	if got := syncPrimaryAction(changedWithSkippedRiskDryRun); got != "Sync plan includes progress with skipped reinjections; clear skips before applying this iteration." {
		t.Fatalf("unexpected changed-with-skipped-risk dry-run primary action: %q", got)
	}

	changedWithMixedRisk := syncsvc.Report{UpdatedSources: []string{"local"}, FailedReinjects: []string{"zeta (boom)"}, SkippedReinjects: []string{"alpha"}}
	if got := syncRecommendedAgent(changedWithMixedRisk); got != "zeta" {
		t.Fatalf("unexpected changed-with-mixed-risk recommended agent: %q", got)
	}
	if got := syncRiskAgents(changedWithMixedRisk); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("unexpected changed-with-mixed-risk risk agents: %v", got)
	}
	if got := syncRecommendedCommand(changedWithMixedRisk); got != "skillpm inject --agent zeta <skill-ref>" {
		t.Fatalf("unexpected changed-with-mixed-risk recommended command: %q", got)
	}
	if got := syncRecommendedCommands(changedWithMixedRisk); !reflect.DeepEqual(got, []string{"skillpm inject --agent zeta <skill-ref>", "skillpm inject --agent alpha <skill-ref>", "skillpm source list", "go test ./...", "skillpm sync --dry-run"}) {
		t.Fatalf("unexpected changed-with-mixed-risk recommended commands: %v", got)
	}

	changedClear := syncsvc.Report{UpdatedSources: []string{"local"}, UpgradedSkills: []string{"local/forms"}, Reinjected: []string{"ghost"}}
	if got := syncFollowUpGate(changedClear); got != "ready-for-next-iteration" {
		t.Fatalf("unexpected changed-clear follow-up gate: %q", got)
	}
	if got := syncRecommendedCommands(changedClear); !reflect.DeepEqual(got, []string{"skillpm source list", "go test ./...", "skillpm sync --dry-run"}) {
		t.Fatalf("unexpected changed-clear recommended commands: %v", got)
	}

	changedClearDryRun := syncsvc.Report{DryRun: true, UpdatedSources: []string{"local"}, UpgradedSkills: []string{"local/forms"}}
	if got := syncRecommendedCommands(changedClearDryRun); !reflect.DeepEqual(got, []string{"skillpm sync", "skillpm source list", "go test ./...", "skillpm sync --dry-run"}) {
		t.Fatalf("unexpected changed-clear dry-run recommended commands: %v", got)
	}
}

func TestSyncNextStepHint(t *testing.T) {
	tests := []struct {
		name   string
		report syncsvc.Report
		want   string
	}{
		{name: "failed risk", report: syncsvc.Report{FailedReinjects: []string{"ghost (boom)"}}, want: "reinject-failed-agents"},
		{name: "skipped risk", report: syncsvc.Report{SkippedReinjects: []string{"ghost"}}, want: "reinject-skipped-agents"},
		{name: "dry-run progress", report: syncsvc.Report{DryRun: true, UpdatedSources: []string{"local"}}, want: "apply-sync-plan"},
		{name: "dry-run noop", report: syncsvc.Report{DryRun: true}, want: "queue-feature-iteration"},
		{name: "apply progress", report: syncsvc.Report{UpgradedSkills: []string{"local/forms"}}, want: "start-next-feature-iteration"},
		{name: "apply noop", report: syncsvc.Report{}, want: "wait-next-sync-cycle"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncNextStepHint(tc.report); got != tc.want {
				t.Fatalf("expected next step hint %q, got %q", tc.want, got)
			}
		})
	}
}

func TestJoinSortedCopiesAndSorts(t *testing.T) {
	items := []string{"zeta", "alpha", "mike"}
	got := joinSorted(items)
	if got != "alpha, mike, zeta" {
		t.Fatalf("unexpected sorted output: %q", got)
	}
	if strings.Join(items, ", ") != "zeta, alpha, mike" {
		t.Fatalf("joinSorted should not mutate input, got %v", items)
	}
}

func TestSyncRiskInjectCommands(t *testing.T) {
	report := syncsvc.Report{
		FailedReinjects:  []string{"zeta (boom)", "alpha: timeout"},
		SkippedReinjects: []string{"alpha", "ghost"},
	}
	got := syncRiskInjectCommands(report)
	want := []string{
		"skillpm inject --agent alpha <skill-ref>",
		"skillpm inject --agent ghost <skill-ref>",
		"skillpm inject --agent zeta <skill-ref>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected risk inject commands: %v", got)
	}
}

func TestJoinSortedWithCustomSeparator(t *testing.T) {
	items := []string{"b", "a"}
	if got := joinSortedWith(items, "; "); got != "a; b" {
		t.Fatalf("unexpected sorted output with separator: %q", got)
	}
}

func TestSummarizeTop(t *testing.T) {
	if got := summarizeTop(nil, 3); got != "none" {
		t.Fatalf("expected none for empty items, got %q", got)
	}
	if got := summarizeTop([]string{"z", "a"}, 3); got != "a, z" {
		t.Fatalf("expected sorted full list, got %q", got)
	}
	if got := summarizeTop([]string{"d", "c", "b", "a"}, 2); got != "a, b ... (+2 more)" {
		t.Fatalf("expected truncated summary, got %q", got)
	}
}

func TestTopSampleSortsAndFallsBackToLimitOne(t *testing.T) {
	got := topSample([]string{"z", "a", "m"}, 0)
	if !reflect.DeepEqual(got.Items, []string{"a"}) {
		t.Fatalf("expected fallback single sorted item, got %+v", got.Items)
	}
	if got.Remaining != 2 {
		t.Fatalf("expected remaining=2, got %d", got.Remaining)
	}

	full := topSample([]string{"b", "a"}, 3)
	if !reflect.DeepEqual(full.Items, []string{"a", "b"}) || full.Remaining != 0 {
		t.Fatalf("expected full sorted sample with remaining 0, got %+v", full)
	}
}

func TestUniqueNonEmptyTrimsAndDeduplicates(t *testing.T) {
	got := uniqueNonEmpty([]string{"  skillpm sync  ", "", "skillpm sync", "go test ./...", "go test ./...", "   "})
	if !reflect.DeepEqual(got, []string{"skillpm sync", "go test ./..."}) {
		t.Fatalf("unexpected unique non-empty output: %+v", got)
	}
}

func TestRiskAgentName(t *testing.T) {
	if got := riskAgentName("ghost: runtime unavailable"); got != "ghost" {
		t.Fatalf("expected ghost agent, got %q", got)
	}
	if got := riskAgentName(" ghost "); got != "ghost" {
		t.Fatalf("expected trimmed agent, got %q", got)
	}
	if got := riskAgentName("ghost (boom)"); got != "ghost" {
		t.Fatalf("expected parsed agent without error suffix, got %q", got)
	}
	if got := riskAgentName("   "); got != "" {
		t.Fatalf("expected empty agent for blank input, got %q", got)
	}
}

func TestBuildSyncJSONSummarySortsOutputArrays(t *testing.T) {
	report := syncsvc.Report{
		UpdatedSources:   []string{"zeta", "alpha"},
		UpgradedSkills:   []string{"b/skill", "a/skill"},
		Reinjected:       []string{"ghost-b", "ghost-a"},
		SkippedReinjects: []string{"skip-b", "skip-a"},
		FailedReinjects:  []string{"fail-b", "fail-a"},
	}
	summary := buildSyncJSONSummary(report, false)

	assertSorted := func(name string, got []string, want []string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("expected %s sorted as %v, got %v", name, want, got)
		}
	}

	assertSorted("updatedSources", summary.UpdatedSources, []string{"alpha", "zeta"})
	assertSorted("upgradedSkills", summary.UpgradedSkills, []string{"a/skill", "b/skill"})
	assertSorted("reinjectedAgents", summary.Reinjected, []string{"ghost-a", "ghost-b"})
	assertSorted("skippedReinjects", summary.SkippedReinjects, []string{"skip-a", "skip-b"})
	assertSorted("failedReinjects", summary.FailedReinjects, []string{"fail-a", "fail-b"})
	assertSorted("riskAgents", summary.RiskAgents, []string{"fail-a", "fail-b", "skip-a", "skip-b"})
	if summary.RiskAgentsTotal != 4 {
		t.Fatalf("expected riskAgentsTotal=4, got %d", summary.RiskAgentsTotal)
	}
	if summary.RecommendedAgent != "fail-a" {
		t.Fatalf("expected recommended agent fail-a, got %q", summary.RecommendedAgent)
	}
}

func TestBuildSyncJSONSummarySkippedRiskClassification(t *testing.T) {
	report := syncsvc.Report{
		UpdatedSources:   []string{"local"},
		SkippedReinjects: []string{"ghost"},
	}
	summary := buildSyncJSONSummary(report, false)

	if summary.Outcome != "changed-with-risk" {
		t.Fatalf("expected changed-with-risk outcome, got %q", summary.Outcome)
	}
	if summary.RiskClass != "skipped-only" {
		t.Fatalf("expected skipped-only risk class, got %q", summary.RiskClass)
	}
	if summary.PrimaryAction != "Progress landed with skipped reinjections; clear skips before expanding scope." {
		t.Fatalf("unexpected primary action for skipped-only risk: %q", summary.PrimaryAction)
	}
	if summary.NextAction != "review-skipped-risk-items" {
		t.Fatalf("expected review-skipped-risk-items next action, got %q", summary.NextAction)
	}
	if summary.NextStepHint != "reinject-skipped-agents" {
		t.Fatalf("expected reinject-skipped-agents next step hint, got %q", summary.NextStepHint)
	}
	if summary.RecommendedCommand != "skillpm inject --agent ghost <skill-ref>" {
		t.Fatalf("unexpected recommended command for skipped-only risk: %q", summary.RecommendedCommand)
	}
}

func syncReportFixture() syncsvc.Report {
	return syncsvc.Report{
		UpdatedSources:   []string{"a", "b"},
		UpgradedSkills:   []string{"c"},
		Reinjected:       []string{"d"},
		SkippedReinjects: []string{"e"},
		FailedReinjects:  []string{"f", "g"},
	}
}

func syncReportFixtureEmpty() syncsvc.Report {
	return syncsvc.Report{}
}

func decodeSyncJSONOutput(t *testing.T, out string) (syncJSONSummary, map[string]json.RawMessage) {
	t.Helper()
	var got syncJSONSummary
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("expected valid sync json output, got %q: %v", out, err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		t.Fatalf("expected sync json object output, got %q: %v", out, err)
	}
	return got, keys
}

func boolPtr(v bool) *bool { return &v }

func TestSyncCmdStrictFlagFailsOnRisk(t *testing.T) {
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

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(false))

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

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(false))

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

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(true))

	var out string
	var execErr error
	out = captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--strict", "--dry-run"})
		execErr = cmd.Execute()
	})
	if execErr == nil {
		t.Fatalf("expected strict dry-run json sync to fail on planned risk")
	}
	if !strings.Contains(execErr.Error(), "SYNC_RISK: sync plan includes 1 risk items (strict mode)") {
		t.Fatalf("unexpected error message: %v", execErr)
	}
	got, _ := decodeSyncJSONOutput(t, out)
	if !got.StrictMode {
		t.Fatalf("expected strictMode=true in strict json output")
	}
	if got.StrictStatus != "enabled" {
		t.Fatalf("expected strictStatus=enabled in strict json output, got %q", got.StrictStatus)
	}
	if got.StrictFailureReason != "risk-present-failed" {
		t.Fatalf("expected strictFailureReason=risk-present-failed in strict json output, got %q", got.StrictFailureReason)
	}
	if got.NextBatchBlocker != "risk-present" {
		t.Fatalf("expected JSON output nextBatchBlocker=risk-present, got %q", got.NextBatchBlocker)
	}
}

func TestSyncCmdStrictFlagDryRunSucceedsWithoutPlannedRisk(t *testing.T) {
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

	cmd.SetArgs([]string{"--lockfile", lockPath, "--strict", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected strict dry-run sync to succeed when planned risk is zero, got: %v", err)
	}
}

func TestSyncJSONOutputReflectsNoopState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	seedSvc, err := app.New(app.Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("new seed service failed: %v", err)
	}
	seedSvc.Config.Sources = nil
	if err := seedSvc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	// Setup empty state -> noop
	if err := store.SaveState(seedSvc.StateRoot, store.State{
		Installed:  nil,
		Injections: nil,
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	if err := store.SaveLockfile(lockPath, store.Lockfile{
		Version: store.LockVersion,
		Skills:  nil,
	}); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	cmd := newSyncCmd(func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: cfgPath})
	}, boolPtr(true))

	var out string
	out = captureStdout(t, func() {
		cmd.SetArgs([]string{"--lockfile", lockPath, "--dry-run"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync dry-run failed: %v", err)
		}
	})
	got, keys := decodeSyncJSONOutput(t, out)

	// Validate stability of output keys
	for _, key := range []string{"schemaVersion", "actionCounts", "riskCounts", "outcome", "progressStatus", "progressClass", "progressHotspot", "progressFocus", "progressTarget", "progressSignal", "actionBreakdown", "nextAction", "primaryAction", "executionPriority", "followUpGate", "nextStepHint", "recommendedCommand", "recommendedCommands", "recommendedAgent", "summaryLine", "noopReason", "riskStatus", "riskLevel", "riskClass", "riskBreakdown", "riskInjectCommands", "riskHotspot", "riskAgents", "riskAgentsTotal", "topSamples", "dryRun", "strictMode", "strictStatus", "strictFailureReason", "mode", "hasProgress", "hasRisk", "canProceed", "nextBatchReady", "nextBatchBlocker"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected key %q in json output, got %q", key, out)
		}
	}

	// Validate No-Op Specifics
	if got.Outcome != "noop" {
		t.Fatalf("expected noop outcome, got %q", got.Outcome)
	}
	if got.ProgressClass != "none" {
		t.Fatalf("expected none progress class, got %q", got.ProgressClass)
	}
	if got.ProgressFocus != "none" {
		t.Fatalf("expected none progress focus, got %q", got.ProgressFocus)
	}
	if got.ProgressTarget != "none" {
		t.Fatalf("expected none progress target, got %q", got.ProgressTarget)
	}
	if got.ProgressSignal != "none" {
		t.Fatalf("expected none progress signal, got %q", got.ProgressSignal)
	}
	if got.RiskClass != "none" {
		t.Fatalf("expected none risk class, got %q", got.RiskClass)
	}
	if got.SummaryLine != "outcome=noop progress=0 risk=0 mode=dry-run" {
		t.Fatalf("unexpected summary line, got %q", got.SummaryLine)
	}
	if got.NoopReason != "dry-run detected no source/upgrade/reinjection deltas" {
		t.Fatalf("unexpected noop reason, got %q", got.NoopReason)
	}
	if got.HasProgress {
		t.Fatalf("expected hasProgress=false")
	}
	if got.HasRisk {
		t.Fatalf("expected hasRisk=false")
	}
	if !got.CanProceed {
		t.Fatalf("expected canProceed=true")
	}
	if got.NextBatchReady {
		t.Fatalf("expected nextBatchReady=false in dry-run mode")
	}
	if got.NextBatchBlocker != "dry-run-mode" {
		t.Fatalf("expected nextBatchBlocker=dry-run-mode, got %q", got.NextBatchBlocker)
	}
}
