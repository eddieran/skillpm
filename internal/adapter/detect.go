package adapter

import (
	"os"
	"path/filepath"
	"sort"
)

type Detection struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func DetectAvailable() []Detection {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	checks := []struct {
		name   string
		path   string
		reason string
	}{
		{name: "claude", path: filepath.Join(home, ".claude"), reason: "claude root exists"},
		{name: "codex", path: filepath.Join(home, ".codex"), reason: "codex root exists"},
		{name: "copilot", path: filepath.Join(home, ".copilot"), reason: "copilot root exists"},
		{name: "cursor", path: filepath.Join(home, ".cursor"), reason: "cursor root exists"},
		{name: "gemini", path: filepath.Join(home, ".gemini"), reason: "gemini root exists"},
		{name: "antigravity", path: filepath.Join(home, ".gemini"), reason: "gemini/antigravity root exists"},
		{name: "kiro", path: filepath.Join(home, ".kiro"), reason: "kiro root exists"},
		{name: "opencode", path: filepath.Join(home, ".config", "opencode"), reason: "opencode config exists"},
		{name: "trae", path: filepath.Join(home, ".trae"), reason: "trae root exists"},
		{name: "vscode", path: filepath.Join(home, ".vscode"), reason: "vscode root exists"},
	}
	openClawState := os.Getenv("OPENCLAW_STATE_DIR")
	if openClawState == "" {
		openClawState = filepath.Join(home, ".openclaw", "state")
	}
	checks = append(checks, struct {
		name   string
		path   string
		reason string
	}{name: "openclaw", path: openClawState, reason: "openclaw state path exists"})

	out := make([]Detection, 0, len(checks))
	seen := map[string]struct{}{}
	for _, c := range checks {
		if _, ok := seen[c.name]; ok {
			continue
		}
		if stat, err := os.Stat(c.path); err == nil && stat.IsDir() {
			seen[c.name] = struct{}{}
			out = append(out, Detection{Name: c.name, Path: c.path, Reason: c.reason})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
