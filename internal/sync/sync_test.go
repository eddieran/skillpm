package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/installer"
	"skillpm/internal/resolver"
	"skillpm/internal/source"
	"skillpm/internal/store"
)

func TestRunRequiresConfiguredDependencies(t *testing.T) {
	svc := &Service{}
	cfg := testConfig(t)
	_, err := svc.Run(context.Background(), cfg, filepath.Join(t.TempDir(), "skills.lock"), false)
	if err == nil || !strings.Contains(err.Error(), "SYNC_SETUP") {
		t.Fatalf("expected SYNC_SETUP error, got %v", err)
	}
}

func TestRunPropagatesSourceUpdateError(t *testing.T) {
	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: t.TempDir()},
		StateRoot: t.TempDir(),
	}
	_, err := svc.Run(context.Background(), nil, filepath.Join(t.TempDir(), "skills.lock"), false)
	if err == nil || !strings.Contains(err.Error(), "SRC_UPDATE") {
		t.Fatalf("expected SRC_UPDATE error, got %v", err)
	}
}

func TestRunReturnsEarlyWhenNoInstalledSkills(t *testing.T) {
	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: t.TempDir()},
		StateRoot: t.TempDir(),
	}
	cfg := testConfig(t)
	report, err := svc.Run(context.Background(), cfg, filepath.Join(t.TempDir(), "skills.lock"), false)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(report.UpdatedSources) != 1 || report.UpdatedSources[0] != "local" {
		t.Fatalf("unexpected updated sources: %+v", report.UpdatedSources)
	}
	if len(report.UpgradedSkills) != 0 || len(report.Reinjected) != 0 {
		t.Fatalf("expected no upgrades/reinjections, got %+v", report)
	}
}

func TestRunReturnsLockfileParseError(t *testing.T) {
	stateRoot := t.TempDir()
	if err := store.SaveState(stateRoot, store.State{Installed: []store.InstalledSkill{{SkillRef: "local/alpha", ResolvedVersion: "1.0.0"}}}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	if err := os.WriteFile(lockPath, []byte("version = ["), 0o644); err != nil {
		t.Fatalf("write invalid lockfile failed: %v", err)
	}

	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: t.TempDir()},
		StateRoot: stateRoot,
	}
	_, err := svc.Run(context.Background(), testConfig(t), lockPath, false)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_PARSE") {
		t.Fatalf("expected DOC_LOCK_PARSE error, got %v", err)
	}
}

func TestRunReturnsResolverErrorForUnknownSource(t *testing.T) {
	stateRoot := t.TempDir()
	if err := store.SaveState(stateRoot, store.State{Installed: []store.InstalledSkill{{SkillRef: "missing/alpha", ResolvedVersion: "1.0.0"}}}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}

	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: t.TempDir()},
		StateRoot: stateRoot,
	}
	_, err := svc.Run(context.Background(), testConfig(t), filepath.Join(t.TempDir(), "missing.lock"), false)
	if err == nil || !strings.Contains(err.Error(), `SRC_RESOLVE: source "missing" not found`) {
		t.Fatalf("expected resolver source-not-found error, got %v", err)
	}
}

func TestRunReturnsInstallerErrorDuringUpgrade(t *testing.T) {
	stateRoot := t.TempDir()
	if err := store.SaveState(stateRoot, store.State{Installed: []store.InstalledSkill{{SkillRef: "local/alpha", ResolvedVersion: "1.0.0"}}}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}

	badInstallRoot := filepath.Join(t.TempDir(), "install-root-file")
	if err := os.WriteFile(badInstallRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("write install root file failed: %v", err)
	}

	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: badInstallRoot},
		StateRoot: stateRoot,
	}
	_, err := svc.Run(context.Background(), testConfig(t), filepath.Join(t.TempDir(), "skills.lock"), false)
	if err == nil {
		t.Fatalf("expected installer error during upgrade")
	}
}

func TestRunReturnsRuntimeGetErrorDuringReinject(t *testing.T) {
	stateRoot := t.TempDir()
	st := store.State{
		Installed: []store.InstalledSkill{{SkillRef: "local/alpha", ResolvedVersion: "0.0.0+git.latest"}},
		Injections: []store.InjectionState{{
			Agent:  "ghost",
			Skills: []string{"local/alpha"},
		}},
	}
	if err := store.SaveState(stateRoot, st); err != nil {
		t.Fatalf("save state failed: %v", err)
	}

	sources := source.NewManager(nil)
	svc := &Service{
		Sources:   sources,
		Resolver:  &resolver.Service{Sources: sources},
		Installer: &installer.Service{Root: t.TempDir()},
		Runtime:   &adapter.Runtime{},
		StateRoot: stateRoot,
	}
	_, err := svc.Run(context.Background(), testConfig(t), filepath.Join(t.TempDir(), "skills.lock"), false)
	if err == nil || !strings.Contains(err.Error(), "ADP_NOT_SUPPORTED") {
		t.Fatalf("expected ADP_NOT_SUPPORTED error, got %v", err)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Version: 1,
		Sync: config.SyncConfig{
			Mode:     "system",
			Interval: "1h",
		},
		Security: config.SecurityConfig{Profile: "strict"},
		Storage:  config.StorageConfig{Root: t.TempDir()},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Sources: []config.SourceConfig{{
			Name:      "local",
			Kind:      "dir",
			URL:       t.TempDir(),
			TrustTier: "review",
		}},
	}
	if err := config.Validate(*cfg); err != nil {
		t.Fatalf("invalid test config: %v", err)
	}
	return cfg
}
