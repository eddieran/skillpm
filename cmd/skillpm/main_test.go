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
	if !strings.Contains(out, "planned actions total: 3") {
		t.Fatalf("expected planned actions total output, got %q", out)
	}
	if !strings.Contains(out, "planned outcome: changed") {
		t.Fatalf("expected planned outcome output, got %q", out)
	}
	if !strings.Contains(out, "planned actions breakdown: sources=1 upgrades=1 reinjected=1 skipped=0 failed=0") {
		t.Fatalf("expected planned action breakdown output, got %q", out)
	}
	if !strings.Contains(out, "planned action samples: sources=local upgrades=local/forms reinjected=ghost") {
		t.Fatalf("expected planned action samples output, got %q", out)
	}
	if !strings.Contains(out, "planned risk status: clear") {
		t.Fatalf("expected planned risk status output, got %q", out)
	}
	if !strings.Contains(out, "planned risk breakdown: skipped=0 failed=0") {
		t.Fatalf("expected planned risk breakdown output, got %q", out)
	}
	if !strings.Contains(out, "planned risk samples: skipped=none failed=none") {
		t.Fatalf("expected planned risk samples output, got %q", out)
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
	if !strings.Contains(out, "planned skipped reinjections: none") {
		t.Fatalf("expected planned skipped reinjections output, got %q", out)
	}
	if !strings.Contains(out, "planned failed reinjections: none") {
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
	if !strings.Contains(out, "applied actions breakdown: sources=1 upgrades=1 reinjected=0 skipped=0 failed=0") {
		t.Fatalf("expected applied action breakdown output, got %q", out)
	}
	if !strings.Contains(out, "applied action samples: sources=local upgrades=local/forms reinjected=none") {
		t.Fatalf("expected applied action samples output, got %q", out)
	}
	if !strings.Contains(out, "risk items total: 0") {
		t.Fatalf("expected risk item total output, got %q", out)
	}
	if !strings.Contains(out, "risk status: clear") {
		t.Fatalf("expected risk status output, got %q", out)
	}
	if !strings.Contains(out, "risk breakdown: skipped=0 failed=0") {
		t.Fatalf("expected risk breakdown output, got %q", out)
	}
	if !strings.Contains(out, "risk samples: skipped=none failed=none") {
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
	if got := syncOutcome(report); got != "changed" {
		t.Fatalf("unexpected action outcome: %q", got)
	}
	if got := syncRiskBreakdown(report); got != "skipped=1 failed=2" {
		t.Fatalf("unexpected risk breakdown: %q", got)
	}
	if got := syncRiskStatus(report); got != "attention-needed" {
		t.Fatalf("unexpected risk status: %q", got)
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
	if got := syncRiskBreakdown(empty); got != "skipped=0 failed=0" {
		t.Fatalf("unexpected empty risk breakdown: %q", got)
	}
	if got := syncRiskStatus(empty); got != "clear" {
		t.Fatalf("unexpected empty risk status: %q", got)
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

func boolPtr(v bool) *bool { return &v }
