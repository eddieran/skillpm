package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/store"
)

func TestDoctorReportsDetectedDisabledAdapter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "codex", Enabled: true, Scope: "global"}}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	stateRoot := filepath.Join(home, ".skillpm")
	if err := store.SaveState(stateRoot, store.State{Version: store.StateVersion}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	runtimeSvc, err := adapter.NewRuntime(stateRoot, cfg, "")
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	svc := &Service{ConfigPath: cfgPath, StateRoot: stateRoot, Runtime: runtimeSvc}
	report := svc.Run(context.Background())
	if len(report.DetectedAdapters) == 0 {
		t.Fatalf("expected detected adapters")
	}
	foundWarn := false
	for _, f := range report.Findings {
		if f.Code == "ADP_DETECTED_DISABLED" {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Fatalf("expected ADP_DETECTED_DISABLED warning, got %+v", report.Findings)
	}
}
