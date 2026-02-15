package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
	syncsvc "skillpm/internal/sync"
)

func TestServiceSourceFlowPaths(t *testing.T) {
	svc, _ := newFlowTestService(t)

	added, err := svc.SourceAdd("beta", "https://example.com/beta.git", "", "", "")
	if err != nil {
		t.Fatalf("source add (git infer) failed: %v", err)
	}
	if added.Kind != "git" {
		t.Fatalf("expected inferred git kind, got %q", added.Kind)
	}
	if added.TrustTier != "review" {
		t.Fatalf("expected default trust tier review, got %q", added.TrustTier)
	}
	if len(added.ScanPaths) != 1 || added.ScanPaths[0] != "skills" {
		t.Fatalf("expected default git scan paths, got %#v", added.ScanPaths)
	}

	if _, err := svc.SourceAdd("alpha", filepath.Join(t.TempDir(), "alpha"), "dir", "", "trusted"); err != nil {
		t.Fatalf("source add (dir) failed: %v", err)
	}

	listed := svc.SourceList()
	for i := 1; i < len(listed); i++ {
		if listed[i-1].Name > listed[i].Name {
			t.Fatalf("expected source list sorted by name: %q > %q", listed[i-1].Name, listed[i].Name)
		}
	}

	updated, err := svc.SourceUpdate(context.Background(), "beta")
	if err != nil {
		t.Fatalf("source update failed: %v", err)
	}
	if len(updated) != 1 || updated[0].Source.Name != "beta" {
		t.Fatalf("expected update result for beta, got %#v", updated)
	}

	if _, err := svc.SourceUpdate(context.Background(), "missing"); err == nil {
		t.Fatalf("expected source update error for missing source")
	}
	if err := svc.SourceRemove("alpha"); err != nil {
		t.Fatalf("source remove failed: %v", err)
	}
	if err := svc.SourceRemove("missing"); err == nil {
		t.Fatalf("expected source remove error for missing source")
	}

	if _, err := svc.SourceAdd("", "https://example.com/x.git", "", "", ""); err == nil {
		t.Fatalf("expected source add validation error for empty name")
	}
	if _, err := svc.SourceAdd("missing-target", "", "git", "", ""); err == nil {
		t.Fatalf("expected source add validation error for empty target")
	}
	if _, err := svc.SourceAdd("bad-kind", "https://example.com/x.git", "invalid", "", ""); err == nil {
		t.Fatalf("expected source add error for unsupported kind")
	}
}

func TestServiceInstallUpgradeUninstallPaths(t *testing.T) {
	svc, _ := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "project", "skills.lock")

	if _, err := svc.Install(ctx, nil, lockPath, false); err == nil {
		t.Fatalf("expected install error for empty refs")
	}

	installed, err := svc.Install(ctx, []string{"local/forms@1.0.0"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected one installed skill, got %d", len(installed))
	}
	if installed[0].ResolvedVersion != "1.0.0" {
		t.Fatalf("expected installed version 1.0.0, got %q", installed[0].ResolvedVersion)
	}

	if _, err := svc.Uninstall(ctx, []string{"bad-ref"}, lockPath); err == nil {
		t.Fatalf("expected uninstall parse error for invalid ref")
	}

	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if len(lock.Skills) != 1 {
		t.Fatalf("expected one lock record, got %d", len(lock.Skills))
	}
	lock.Skills[0].ResolvedVersion = "2.0.0"
	lock.Skills[0].Checksum = "sha256:forced"
	lock.Skills[0].SourceRef = "https://example.com/skills.git@2.0.0"
	if err := store.SaveLockfile(lockPath, lock); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}

	upgraded, err := svc.Upgrade(ctx, nil, lockPath, false)
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	if len(upgraded) != 1 {
		t.Fatalf("expected one upgraded skill, got %d", len(upgraded))
	}
	if upgraded[0].ResolvedVersion != "2.0.0" {
		t.Fatalf("expected upgraded version 2.0.0, got %q", upgraded[0].ResolvedVersion)
	}

	removed, err := svc.Uninstall(ctx, []string{"local/forms@latest"}, lockPath)
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if len(removed) != 1 || removed[0] != "local/forms" {
		t.Fatalf("expected local/forms removed, got %#v", removed)
	}

	if _, err := svc.Uninstall(ctx, nil, lockPath); err == nil {
		t.Fatalf("expected uninstall error for empty refs")
	}

	noUpgrades, err := svc.Upgrade(ctx, nil, lockPath, false)
	if err != nil {
		t.Fatalf("upgrade with no installed skills failed: %v", err)
	}
	if len(noUpgrades) != 0 {
		t.Fatalf("expected no upgrades with empty installed state, got %d", len(noUpgrades))
	}

	if _, err := svc.Install(ctx, []string{"local/forms@1.0.0"}, lockPath, false); err != nil {
		t.Fatalf("reinstall failed: %v", err)
	}
	if _, err := svc.Upgrade(ctx, []string{"bad-ref"}, lockPath, false); err == nil {
		t.Fatalf("expected upgrade parse error for invalid ref")
	}
}

func TestServiceSyncHarvestDoctorPaths(t *testing.T) {
	svc, openclawState := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "sync", "skills.lock")

	report, err := svc.SyncRun(ctx, lockPath, false, false)
	if err != nil {
		t.Fatalf("sync run failed: %v", err)
	}
	if len(report.UpdatedSources) != 1 || report.UpdatedSources[0] != "local" {
		t.Fatalf("expected local source update report, got %#v", report.UpdatedSources)
	}

	originalSync := svc.Sync
	svc.Sync = &syncsvc.Service{}
	if _, err := svc.SyncRun(ctx, lockPath, false, false); err == nil {
		t.Fatalf("expected sync setup error when dependencies are missing")
	}
	svc.Sync = originalSync

	candidateDir := filepath.Join(openclawState, "incoming-skill")
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatalf("mkdir candidate failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidateDir, "SKILL.md"), []byte("# Incoming Skill\n"), 0o644); err != nil {
		t.Fatalf("write candidate SKILL.md failed: %v", err)
	}

	entries, inboxPath, err := svc.HarvestRun(ctx, "openclaw")
	if err != nil {
		t.Fatalf("harvest run failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected harvest entries")
	}
	foundValid := false
	for _, e := range entries {
		if e.SkillName == "incoming-skill" {
			if !e.Valid {
				t.Fatalf("expected incoming-skill candidate to validate")
			}
			foundValid = true
			break
		}
	}
	if !foundValid {
		t.Fatalf("expected incoming-skill in harvested entries: %#v", entries)
	}
	if _, err := os.Stat(inboxPath); err != nil {
		t.Fatalf("expected inbox file to exist: %v", err)
	}

	if _, _, err := svc.HarvestRun(ctx, "missing-adapter"); err == nil {
		t.Fatalf("expected harvest error for unknown adapter")
	}

	healthy := svc.DoctorRun(ctx)
	if !healthy.Healthy {
		t.Fatalf("expected healthy doctor report, got %#v", healthy)
	}
	if err := os.Remove(svc.ConfigPath); err != nil {
		t.Fatalf("remove config failed: %v", err)
	}
	broken := svc.DoctorRun(ctx)
	if broken.Healthy {
		t.Fatalf("expected unhealthy doctor report when config is missing")
	}
	foundConfigMissing := false
	for _, f := range broken.Findings {
		if f.Code == "DOC_CONFIG_MISSING" {
			foundConfigMissing = true
			break
		}
	}
	if !foundConfigMissing {
		t.Fatalf("expected DOC_CONFIG_MISSING finding, got %#v", broken.Findings)
	}
}

func newFlowTestService(t *testing.T) (*Service, string) {
	t.Helper()

	home := t.TempDir()
	openclawState := filepath.Join(home, "openclaw-state")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", openclawState)
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	if err := os.MkdirAll(openclawState, 0o755); err != nil {
		t.Fatalf("mkdir openclaw state failed: %v", err)
	}

	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	svc.Config.Sources = []config.SourceConfig{{
		Name:      "local",
		Kind:      "git",
		URL:       "https://example.com/skills.git",
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	if err := svc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	return svc, openclawState
}
