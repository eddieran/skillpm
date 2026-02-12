package adapter

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillpm/internal/config"
	"skillpm/pkg/adapterapi"
)

func TestOpenClawAdapterConformance(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, "openclaw-state")
	configPath := filepath.Join(home, "openclaw-config.toml")
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", stateDir)
	t.Setenv("OPENCLAW_CONFIG_PATH", configPath)

	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "openclaw", Enabled: true, Scope: "global"}}
	runtime, err := NewRuntime(filepath.Join(home, ".skillpm"), cfg)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	adp, err := runtime.Get("openclaw")
	if err != nil {
		t.Fatalf("get adapter failed: %v", err)
	}

	probe, err := adp.Probe(context.Background())
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if !probe.Available {
		t.Fatalf("expected probe available")
	}

	injectRes, err := adp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"anthropic/pdf", "clawhub/forms"}})
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}
	if !injectRes.RollbackPossible || len(injectRes.Injected) != 2 {
		t.Fatalf("unexpected inject result: %+v", injectRes)
	}

	listRes, err := adp.ListInjected(context.Background(), adapterapi.ListInjectedRequest{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	expected := []string{"anthropic/pdf", "clawhub/forms"}
	if !reflect.DeepEqual(listRes.Skills, expected) {
		t.Fatalf("unexpected injected list: %+v", listRes.Skills)
	}

	removeRes, err := adp.Remove(context.Background(), adapterapi.RemoveRequest{SkillRefs: []string{"clawhub/forms"}})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if len(removeRes.Removed) != 1 || removeRes.Removed[0] != "clawhub/forms" {
		t.Fatalf("unexpected remove result: %+v", removeRes)
	}

	targetDir := filepath.Join(stateDir, "skillpm")
	candidateDir := filepath.Join(targetDir, "candidate-one")
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatalf("mkdir candidate failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidateDir, "SKILL.md"), []byte("# test"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	harvestRes, err := adp.HarvestCandidates(context.Background(), adapterapi.HarvestRequest{})
	if err != nil {
		t.Fatalf("harvest failed: %v", err)
	}
	if len(harvestRes.Candidates) == 0 {
		t.Fatalf("expected harvest candidates")
	}

	validateRes, err := adp.ValidateEnvironment(context.Background())
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !validateRes.Valid {
		t.Fatalf("expected valid environment, warnings=%v", validateRes.Warnings)
	}
	if len(validateRes.RootPaths) == 0 {
		t.Fatalf("expected root path metadata")
	}
}
