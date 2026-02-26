package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/memory"
	"skillpm/internal/store"
)

// enabledCfg returns a MemoryConfig with Enabled=true and sensible defaults.
func enabledCfg() config.MemoryConfig {
	return config.MemoryConfig{
		Enabled:          true,
		WorkingMemoryMax: 5,
		Threshold:        0.2,
		RecencyHalfLife:  "7d",
		ObserveOnSync:    false,
		AdaptiveInject:   false,
	}
}

// disabledCfg returns a MemoryConfig with Enabled=false.
func disabledCfg() config.MemoryConfig {
	return config.MemoryConfig{Enabled: false}
}

// TestNewDisabled verifies that creating a Service with Enabled=false results
// in IsEnabled() returning false.
func TestNewDisabled(t *testing.T) {
	tmp := t.TempDir()
	if err := store.EnsureLayout(tmp); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	svc, err := memory.New(tmp, disabledCfg(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc.IsEnabled() {
		t.Error("expected IsEnabled()=false for disabled config")
	}
}

// TestNewEnabled verifies that creating a Service with Enabled=true results in
// IsEnabled() returning true and the memory directory being created.
func TestNewEnabled(t *testing.T) {
	tmp := t.TempDir()
	if err := store.EnsureLayout(tmp); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	svc, err := memory.New(tmp, enabledCfg(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !svc.IsEnabled() {
		t.Error("expected IsEnabled()=true for enabled config")
	}

	memDir := filepath.Join(tmp, "memory")
	if info, err := os.Stat(memDir); err != nil || !info.IsDir() {
		t.Errorf("expected memory directory to exist at %s", memDir)
	}
}

// TestIsEnabledNilService verifies that calling IsEnabled on a nil *Service
// returns false without panicking.
func TestIsEnabledNilService(t *testing.T) {
	var svc *memory.Service
	if svc.IsEnabled() {
		t.Error("expected IsEnabled()=false for nil receiver")
	}
}

// TestScoresPath verifies that ScoresPath returns a path containing "scores.toml".
func TestScoresPath(t *testing.T) {
	tmp := t.TempDir()
	if err := store.EnsureLayout(tmp); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	svc, err := memory.New(tmp, enabledCfg(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p := svc.ScoresPath()

	if !strings.Contains(p, "scores.toml") {
		t.Errorf("ScoresPath=%q does not contain 'scores.toml'", p)
	}
	// The path must also be rooted under stateRoot.
	if !strings.HasPrefix(p, tmp) {
		t.Errorf("ScoresPath=%q is not under stateRoot %s", p, tmp)
	}
}

// TestPurgeRemovesFiles creates files inside the memory directory and verifies
// that Purge removes them.
func TestPurgeRemovesFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := store.EnsureLayout(tmp); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	memDir := filepath.Join(tmp, "memory")

	// Create some dummy files inside the memory directory.
	files := []string{"events.jsonl", "scores.toml", "feedback.jsonl"}
	for _, name := range files {
		p := filepath.Join(memDir, name)
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	svc, err := memory.New(tmp, enabledCfg(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.Purge(); err != nil {
		t.Fatalf("Purge returned error: %v", err)
	}

	// All previously created files must be gone.
	for _, name := range files {
		p := filepath.Join(memDir, name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed by Purge, but it still exists", p)
		}
	}
}

// TestPurgeNonExistentDir verifies that Purge returns no error when the memory
// directory does not exist.
func TestPurgeNonExistentDir(t *testing.T) {
	tmp := t.TempDir()
	// Do NOT call EnsureLayout — the memory directory is intentionally absent.

	svc, err := memory.New(tmp, disabledCfg(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.Purge(); err != nil {
		t.Errorf("Purge on non-existent dir returned error: %v", err)
	}
}
