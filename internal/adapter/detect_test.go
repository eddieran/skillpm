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

	// Create dirs for all agents
	dirs := []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".copilot"),
		filepath.Join(home, ".cursor"),
		filepath.Join(home, ".gemini"),
		filepath.Join(home, ".kiro"),
		filepath.Join(home, ".config", "opencode"),
		filepath.Join(home, ".trae"),
		filepath.Join(home, ".vscode"),
		filepath.Join(home, "openclaw-state"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}
	found := DetectAvailable()
	set := map[string]struct{}{}
	for _, f := range found {
		set[f.Name] = struct{}{}
	}
	// Verify all agents detected
	for _, want := range []string{
		"claude", "codex", "copilot", "cursor", "gemini",
		"antigravity", "kiro", "opencode", "trae", "vscode", "openclaw",
	} {
		if _, ok := set[want]; !ok {
			t.Errorf("expected %s to be detected, got %+v", want, found)
		}
	}
}

// TestDetectAvailablePartial verifies only existing dirs are detected.
func TestDetectAvailablePartial(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "missing-openclaw"))

	// Only create trae and kiro
	for _, dir := range []string{
		filepath.Join(home, ".trae"),
		filepath.Join(home, ".kiro"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}
	found := DetectAvailable()
	set := map[string]struct{}{}
	for _, f := range found {
		set[f.Name] = struct{}{}
	}
	if _, ok := set["trae"]; !ok {
		t.Errorf("expected trae detected")
	}
	if _, ok := set["kiro"]; !ok {
		t.Errorf("expected kiro detected")
	}
	// Should NOT detect agents without dirs
	for _, notWanted := range []string{"claude", "codex", "copilot", "cursor", "openclaw"} {
		if _, ok := set[notWanted]; ok {
			t.Errorf("should not detect %s without dir", notWanted)
		}
	}
}
