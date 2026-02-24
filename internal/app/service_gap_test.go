package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestServiceRemoveInjectedPaths(t *testing.T) {
	svc, _ := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")

	if _, err := svc.Install(ctx, []string{"local/forms@1.0.0"}, lockPath, false); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if _, err := svc.Inject(ctx, "openclaw", nil); err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	removed, err := svc.RemoveInjected(ctx, "openclaw", nil)
	if err != nil {
		t.Fatalf("remove injected failed: %v", err)
	}
	if len(removed.Removed) == 0 {
		t.Fatalf("expected removed skills in result")
	}

	if _, err := svc.RemoveInjected(ctx, "missing-adapter", nil); err == nil {
		t.Fatalf("expected remove injected error for missing adapter")
	}
}

func TestServiceSyncRunSaveConfigFailure(t *testing.T) {
	svc, _ := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "sync", "skills.lock")

	svc.ConfigPath = "/dev/null/config.toml"
	if _, err := svc.SyncRun(ctx, lockPath, false, false); err == nil {
		t.Fatalf("expected sync run error when saving config fails")
	}
}

func TestServiceSyncRunDryRunSkipsConfigSave(t *testing.T) {
	svc, _ := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "sync", "skills.lock")

	svc.ConfigPath = "/dev/null/config.toml"
	if _, err := svc.SyncRun(ctx, lockPath, false, true); err != nil {
		t.Fatalf("dry-run sync should not save config: %v", err)
	}
}

func TestServiceScheduleListDoesNotPersistConfig(t *testing.T) {
	svc, _ := newFlowTestService(t)
	svc.ConfigPath = "/dev/null/config.toml"

	if _, err := svc.Schedule("list", ""); err != nil {
		t.Fatalf("schedule list should not save config: %v", err)
	}
}
