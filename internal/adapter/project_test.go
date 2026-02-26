package adapter

import (
	"path/filepath"
	"testing"

	"skillpm/internal/config"
)

func TestAgentProjectSkillsDir(t *testing.T) {
	projectRoot := "/home/user/myproject"
	cases := []struct {
		agent string
		want  string
	}{
		{"claude", filepath.Join(projectRoot, ".claude", "skills")},
		{"codex", filepath.Join(projectRoot, ".codex", "skills")},
		{"cursor", filepath.Join(projectRoot, ".cursor", "skills")},
		{"gemini", filepath.Join(projectRoot, ".gemini", "skills")},
		{"antigravity", filepath.Join(projectRoot, ".gemini", "skills")},
		{"copilot", filepath.Join(projectRoot, ".copilot", "skills")},
		{"vscode", filepath.Join(projectRoot, ".copilot", "skills")},
		{"trae", filepath.Join(projectRoot, ".trae", "skills")},
		{"opencode", filepath.Join(projectRoot, ".opencode", "skills")},
		{"kiro", filepath.Join(projectRoot, ".kiro", "skills")},
		{"openclaw", filepath.Join(projectRoot, ".openclaw", "skills")},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			got := agentProjectSkillsDir(tc.agent, projectRoot)
			if got != tc.want {
				t.Fatalf("agentProjectSkillsDir(%q) = %q, want %q", tc.agent, got, tc.want)
			}
		})
	}
}

func TestAgentProjectTargetDir(t *testing.T) {
	projectRoot := "/home/user/myproject"
	cases := []struct {
		agent string
		want  string
	}{
		{"claude", filepath.Join(projectRoot, ".claude", "skillpm")},
		{"codex", filepath.Join(projectRoot, ".codex", "skillpm")},
		{"antigravity", filepath.Join(projectRoot, ".gemini", "skillpm-antigravity")},
		{"vscode", filepath.Join(projectRoot, ".copilot", "skillpm-vscode")},
	}
	for _, tc := range cases {
		t.Run(tc.agent, func(t *testing.T) {
			got := agentProjectTargetDir(tc.agent, projectRoot)
			if got != tc.want {
				t.Fatalf("agentProjectTargetDir(%q) = %q, want %q", tc.agent, got, tc.want)
			}
		})
	}
}

func TestNewRuntimeProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	stateRoot := filepath.Join(projectRoot, ".skillpm")

	cfg := config.Config{
		Version:  1,
		Sync:     config.SyncConfig{Mode: "system", Interval: "6h"},
		Security: config.SecurityConfig{Profile: "strict"},
		Storage:  config.StorageConfig{Root: stateRoot},
		Logging:  config.LoggingConfig{Level: "info", Format: "text"},
		Adapters: []config.AdapterConfig{
			{Name: "claude", Enabled: true, Scope: "project"},
		},
	}

	runtime, err := NewRuntime(stateRoot, cfg, projectRoot)
	if err != nil {
		t.Fatalf("NewRuntime with projectRoot failed: %v", err)
	}

	adp, err := runtime.Get("claude")
	if err != nil {
		t.Fatalf("Get claude adapter failed: %v", err)
	}

	fa, ok := adp.(*fileAdapter)
	if !ok {
		t.Fatal("expected *fileAdapter")
	}

	wantSkillsDir := filepath.Join(projectRoot, ".claude", "skills")
	if fa.skillsDir != wantSkillsDir {
		t.Fatalf("skillsDir = %q, want %q", fa.skillsDir, wantSkillsDir)
	}

	wantTargetDir := filepath.Join(projectRoot, ".claude", "skillpm")
	if fa.targetDir != wantTargetDir {
		t.Fatalf("targetDir = %q, want %q", fa.targetDir, wantTargetDir)
	}
}

func TestNewRuntimeGlobalUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(home, ".skillpm")
	cfg := config.Config{
		Version:  1,
		Sync:     config.SyncConfig{Mode: "system", Interval: "6h"},
		Security: config.SecurityConfig{Profile: "strict"},
		Storage:  config.StorageConfig{Root: stateRoot},
		Logging:  config.LoggingConfig{Level: "info", Format: "text"},
		Adapters: []config.AdapterConfig{
			{Name: "claude", Enabled: true, Scope: "global"},
		},
	}

	runtime, err := NewRuntime(stateRoot, cfg, "")
	if err != nil {
		t.Fatalf("NewRuntime with empty projectRoot failed: %v", err)
	}

	adp, err := runtime.Get("claude")
	if err != nil {
		t.Fatalf("Get claude adapter failed: %v", err)
	}

	fa, ok := adp.(*fileAdapter)
	if !ok {
		t.Fatal("expected *fileAdapter")
	}

	wantSkillsDir := filepath.Join(home, ".claude", "skills")
	if fa.skillsDir != wantSkillsDir {
		t.Fatalf("skillsDir = %q, want %q (should use home dir for global)", fa.skillsDir, wantSkillsDir)
	}

	wantTargetDir := filepath.Join(home, ".claude", "skillpm")
	if fa.targetDir != wantTargetDir {
		t.Fatalf("targetDir = %q, want %q (should use home dir for global)", fa.targetDir, wantTargetDir)
	}
}
