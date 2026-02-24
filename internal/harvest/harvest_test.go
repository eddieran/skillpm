package harvest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/store"
)

func TestHarvestListsCandidatesAndWritesInbox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(t.TempDir(), "state")
	runtime, err := adapter.NewRuntime(stateRoot, config.Config{Adapters: []config.AdapterConfig{{Name: "codex", Enabled: true, Scope: "global"}}})
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}

	base := filepath.Join(home, ".codex", "skills")
	validDir := filepath.Join(base, "valid-skill")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatalf("mkdir valid dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write valid SKILL.md failed: %v", err)
	}

	brokenDir := filepath.Join(base, "broken-skill")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir broken dir failed: %v", err)
	}
	if err := os.Symlink("MISSING.md", filepath.Join(brokenDir, "SKILL.md")); err != nil {
		t.Skipf("symlink unsupported in environment: %v", err)
	}

	svc := &Service{Runtime: runtime, StateRoot: stateRoot}
	entries, inboxPath, err := svc.Harvest(context.Background(), "codex")
	if err != nil {
		t.Fatalf("harvest failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if filepath.Dir(inboxPath) != store.InboxRoot(stateRoot) {
		t.Fatalf("unexpected inbox path %q", inboxPath)
	}

	blob, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("read inbox file failed: %v", err)
	}
	var persisted []InboxEntry
	if err := json.Unmarshal(blob, &persisted); err != nil {
		t.Fatalf("unmarshal inbox json failed: %v", err)
	}
	if len(persisted) != len(entries) {
		t.Fatalf("expected persisted entries to match return value")
	}

	byName := map[string]InboxEntry{}
	for _, e := range entries {
		byName[e.SkillName] = e
	}
	valid := byName["valid-skill"]
	if !valid.Valid || valid.Reason != "" {
		t.Fatalf("expected valid-skill to be valid, got %+v", valid)
	}
	broken := byName["broken-skill"]
	if broken.Valid || !strings.Contains(broken.Reason, "missing SKILL.md") {
		t.Fatalf("expected broken-skill to be invalid with missing SKILL.md reason, got %+v", broken)
	}
}

func TestHarvestErrorsWhenRuntimeMissing(t *testing.T) {
	svc := &Service{StateRoot: t.TempDir()}
	_, _, err := svc.Harvest(context.Background(), "codex")
	if err == nil || !strings.Contains(err.Error(), "HRV_RUNTIME") {
		t.Fatalf("expected HRV_RUNTIME error, got %v", err)
	}
}

func TestHarvestReturnsPersistError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runtimeRoot := filepath.Join(t.TempDir(), "runtime-state")
	runtime, err := adapter.NewRuntime(runtimeRoot, config.Config{Adapters: []config.AdapterConfig{{Name: "codex", Enabled: true, Scope: "global"}}})
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}

	candidate := filepath.Join(home, ".codex", "skillpm", "valid-skill")
	if err := os.MkdirAll(candidate, 0o755); err != nil {
		t.Fatalf("mkdir candidate failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidate, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}

	badRoot := filepath.Join(t.TempDir(), "state-root-file")
	if err := os.WriteFile(badRoot, []byte("x"), 0o644); err != nil {
		t.Fatalf("write bad root file failed: %v", err)
	}

	svc := &Service{Runtime: runtime, StateRoot: badRoot}
	_, _, err = svc.Harvest(context.Background(), "codex")
	if err == nil {
		t.Fatalf("expected persist error when state root is a file")
	}
}
