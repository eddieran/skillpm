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
		{name: "codex", path: filepath.Join(home, ".codex"), reason: "default codex root exists"},
		{name: "claude", path: filepath.Join(home, ".claude"), reason: "default claude root exists"},
		{name: "cursor", path: filepath.Join(home, ".cursor"), reason: "default cursor root exists"},
		{name: "gemini", path: filepath.Join(home, ".gemini"), reason: "default gemini root exists"},
		{name: "qwen", path: filepath.Join(home, ".qwen"), reason: "default qwen root exists"},
		{name: "vscode", path: filepath.Join(home, ".vscode"), reason: "default vscode root exists"},
		{name: "opcode", path: filepath.Join(home, ".opcode"), reason: "default opcode root exists"},
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
