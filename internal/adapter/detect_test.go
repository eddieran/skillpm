package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAvailableFindsKnownRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))

	for _, dir := range []string{filepath.Join(home, ".codex"), filepath.Join(home, ".cursor"), filepath.Join(home, "openclaw-state")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}
	found := DetectAvailable()
	set := map[string]struct{}{}
	for _, f := range found {
		set[f.Name] = struct{}{}
	}
	for _, want := range []string{"codex", "cursor", "openclaw"} {
		if _, ok := set[want]; !ok {
			t.Fatalf("expected %s to be detected, got %+v", want, found)
		}
	}
}
