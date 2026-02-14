package app

import (
	"context"
	"os"
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

func TestServiceValidatePaths(t *testing.T) {
	svc, _ := newFlowTestService(t)

	validDir := filepath.Join(t.TempDir(), "valid-skill")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	if err := svc.Validate(validDir); err != nil {
		t.Fatalf("validate should pass for valid dir: %v", err)
	}

	invalidDir := filepath.Join(t.TempDir(), "invalid-skill")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := svc.Validate(invalidDir); err == nil {
		t.Fatalf("expected validate error when SKILL.md missing")
	}
}

func TestServiceSyncRunSaveConfigFailure(t *testing.T) {
	svc, _ := newFlowTestService(t)
	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "sync", "skills.lock")

	svc.ConfigPath = "/dev/null/config.toml"
	if _, err := svc.SyncRun(ctx, lockPath, false); err == nil {
		t.Fatalf("expected sync run error when saving config fails")
	}
}

func TestServiceScheduleSaveConfigFailure(t *testing.T) {
	svc, _ := newFlowTestService(t)
	svc.ConfigPath = "/dev/null/config.toml"

	if _, err := svc.Schedule("list", ""); err == nil {
		t.Fatalf("expected schedule error when saving config fails")
	}
}
