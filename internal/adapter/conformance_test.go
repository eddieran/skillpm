package adapter

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
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
}

// TestInjectCopiesSkillToAgentSkillsDir verifies that inject creates the skill folder
// under the agent's skills directory with the correct SKILL.md content.
func TestInjectCopiesSkillToAgentSkillsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(home, ".skillpm")
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{
		{Name: "claude", Enabled: true, Scope: "global"},
		{Name: "codex", Enabled: true, Scope: "global"},
	}

	// Set up a fake installed skill with SKILL.md
	_ = store.SaveState(stateRoot, store.State{
		Installed: []store.InstalledSkill{{SkillRef: "test/code-review", ResolvedVersion: "1.0.0"}},
	})
	installedDir := filepath.Join(store.InstalledRoot(stateRoot), "test_code-review@1.0.0")
	if err := os.MkdirAll(installedDir, 0o755); err != nil {
		t.Fatalf("mkdir installed dir failed: %v", err)
	}
	skillContent := "---\nname: code-review\ndescription: Review code for quality\n---\n\n# Code Review\n\nReview code systematically.\n"
	if err := os.WriteFile(filepath.Join(installedDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "metadata.toml"), []byte("internal=true\n"), 0o644); err != nil {
		t.Fatalf("write metadata.toml failed: %v", err)
	}

	runtime, err := NewRuntime(stateRoot, cfg)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}

	// Inject into claude
	claudeAdp, err := runtime.Get("claude")
	if err != nil {
		t.Fatalf("get claude adapter failed: %v", err)
	}
	res, err := claudeAdp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"test/code-review"}})
	if err != nil {
		t.Fatalf("inject claude failed: %v", err)
	}
	if len(res.Injected) != 1 || res.Injected[0] != "test/code-review" {
		t.Fatalf("unexpected inject result: %+v", res)
	}

	// Verify SKILL.md was copied to ~/.claude/skills/code-review/SKILL.md
	skillPath := filepath.Join(home, ".claude", "skills", "code-review", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected SKILL.md at %s: %v", skillPath, err)
	}
	if string(data) != skillContent {
		t.Fatalf("expected SKILL.md content match, got %q", string(data))
	}

	// Verify metadata.toml was NOT copied (internal file)
	metaPath := filepath.Join(home, ".claude", "skills", "code-review", "metadata.toml")
	if _, err := os.Stat(metaPath); err == nil {
		t.Fatalf("metadata.toml should not be copied to agent skills dir")
	}

	// Inject into codex too
	codexAdp, err := runtime.Get("codex")
	if err != nil {
		t.Fatalf("get codex adapter failed: %v", err)
	}
	res2, err := codexAdp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"test/code-review"}})
	if err != nil {
		t.Fatalf("inject codex failed: %v", err)
	}
	if len(res2.Injected) != 1 {
		t.Fatalf("unexpected codex inject result: %+v", res2)
	}
	codexSkillPath := filepath.Join(home, ".codex", "skills", "code-review", "SKILL.md")
	if _, err := os.Stat(codexSkillPath); err != nil {
		t.Fatalf("expected SKILL.md at %s: %v", codexSkillPath, err)
	}
}

// TestRemoveDeletesSkillFromAgentDir verifies that remove deletes the skill folder.
func TestRemoveDeletesSkillFromAgentDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(home, ".skillpm")
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "claude", Enabled: true, Scope: "global"}}

	// Set up installed skill
	_ = store.SaveState(stateRoot, store.State{
		Installed: []store.InstalledSkill{{SkillRef: "test/my-skill", ResolvedVersion: "1.0.0"}},
	})
	installedDir := filepath.Join(store.InstalledRoot(stateRoot), "test_my-skill@1.0.0")
	if err := os.MkdirAll(installedDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "SKILL.md"), []byte("# My Skill"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	runtime, err := NewRuntime(stateRoot, cfg)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	adp, _ := runtime.Get("claude")

	// Inject
	_, err = adp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"test/my-skill"}})
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}
	skillDir := filepath.Join(home, ".claude", "skills", "my-skill")
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("skill dir should exist after inject: %v", err)
	}

	// Remove
	removeRes, err := adp.Remove(context.Background(), adapterapi.RemoveRequest{SkillRefs: []string{"test/my-skill"}})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if len(removeRes.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removeRes.Removed))
	}

	// Verify folder deleted
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("skill dir should be deleted after remove")
	}

	// Verify list is empty
	listRes, _ := adp.ListInjected(context.Background(), adapterapi.ListInjectedRequest{})
	if len(listRes.Skills) != 0 {
		t.Fatalf("expected empty injected list after remove, got %+v", listRes.Skills)
	}
}

// TestAgentSkillsDir verifies each agent maps to the correct path.
func TestAgentSkillsDir(t *testing.T) {
	home := "/fake/home"
	tests := []struct {
		name string
		want string
	}{
		{"claude", "/fake/home/.claude/skills"},
		{"codex", "/fake/home/.codex/skills"},
		{"copilot", "/fake/home/.copilot/skills"},
		{"cursor", "/fake/home/.cursor/skills"},
		{"gemini", "/fake/home/.gemini/skills"},
		{"antigravity", "/fake/home/.gemini/skills"},
		{"vscode", "/fake/home/.copilot/skills"},
		{"trae", "/fake/home/.trae/skills"},
		{"opencode", "/fake/home/.config/opencode/skills"},
		{"kiro", "/fake/home/.kiro/skills"},
	}
	for _, tt := range tests {
		got := agentSkillsDir(tt.name, home)
		if got != tt.want {
			t.Errorf("agentSkillsDir(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestInjectWithSubdirectories verifies scripts/ and other dirs are copied.
func TestInjectWithSubdirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(home, ".skillpm")
	cfg := config.DefaultConfig()
	cfg.Adapters = []config.AdapterConfig{{Name: "gemini", Enabled: true, Scope: "global"}}

	// Create installed skill with scripts/ and references/ subdirs
	_ = store.SaveState(stateRoot, store.State{
		Installed: []store.InstalledSkill{{SkillRef: "hub/deploy", ResolvedVersion: "2.0.0"}},
	})
	installedDir := filepath.Join(store.InstalledRoot(stateRoot), "hub_deploy@2.0.0")
	scriptsDir := filepath.Join(installedDir, "scripts")
	refsDir := filepath.Join(installedDir, "references")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts failed: %v", err)
	}
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatalf("mkdir refs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "SKILL.md"), []byte("# Deploy\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "metadata.toml"), []byte("internal=true\n"), 0o644); err != nil {
		t.Fatalf("write metadata failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "deploy.sh"), []byte("#!/bin/bash\necho deploy"), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "REFERENCE.md"), []byte("# Ref\n"), 0o644); err != nil {
		t.Fatalf("write ref failed: %v", err)
	}

	runtime, err := NewRuntime(stateRoot, cfg)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	adp, _ := runtime.Get("gemini")

	_, err = adp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"hub/deploy"}})
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	// Verify SKILL.md
	base := filepath.Join(home, ".gemini", "skills", "deploy")
	if _, err := os.Stat(filepath.Join(base, "SKILL.md")); err != nil {
		t.Fatalf("expected SKILL.md: %v", err)
	}
	// Verify scripts/deploy.sh copied
	if _, err := os.Stat(filepath.Join(base, "scripts", "deploy.sh")); err != nil {
		t.Fatalf("expected scripts/deploy.sh: %v", err)
	}
	// Verify references/REFERENCE.md copied
	if _, err := os.Stat(filepath.Join(base, "references", "REFERENCE.md")); err != nil {
		t.Fatalf("expected references/REFERENCE.md: %v", err)
	}
	// Verify metadata.toml NOT copied
	if _, err := os.Stat(filepath.Join(base, "metadata.toml")); err == nil {
		t.Fatalf("metadata.toml should not be copied")
	}
}

// TestInjectNewAgents verifies injection paths for all new agents.
func TestInjectNewAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateRoot := filepath.Join(home, ".skillpm")

	agents := []struct {
		name     string
		wantPath string // relative to home
	}{
		{"copilot", ".copilot/skills/my-skill/SKILL.md"},
		{"trae", ".trae/skills/my-skill/SKILL.md"},
		{"opencode", ".config/opencode/skills/my-skill/SKILL.md"},
		{"kiro", ".kiro/skills/my-skill/SKILL.md"},
		{"antigravity", ".gemini/skills/my-skill/SKILL.md"},
		{"vscode", ".copilot/skills/my-skill/SKILL.md"},
	}

	cfg := config.DefaultConfig()
	for _, a := range agents {
		cfg.Adapters = append(cfg.Adapters, config.AdapterConfig{Name: a.name, Enabled: true, Scope: "global"})
	}

	// Set up installed skill
	_ = store.SaveState(stateRoot, store.State{
		Installed: []store.InstalledSkill{{SkillRef: "test/my-skill", ResolvedVersion: "1.0.0"}},
	})
	installedDir := filepath.Join(store.InstalledRoot(stateRoot), "test_my-skill@1.0.0")
	if err := os.MkdirAll(installedDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "SKILL.md"), []byte("# My Skill\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	runtime, err := NewRuntime(stateRoot, cfg)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}

	for _, a := range agents {
		adp, err := runtime.Get(a.name)
		if err != nil {
			t.Fatalf("get %s adapter failed: %v", a.name, err)
		}
		res, err := adp.Inject(context.Background(), adapterapi.InjectRequest{SkillRefs: []string{"test/my-skill"}})
		if err != nil {
			t.Fatalf("inject %s failed: %v", a.name, err)
		}
		if len(res.Injected) != 1 {
			t.Fatalf("%s: expected 1 injected, got %d", a.name, len(res.Injected))
		}

		skillPath := filepath.Join(home, a.wantPath)
		if _, err := os.Stat(skillPath); err != nil {
			t.Errorf("%s: expected SKILL.md at %s: %v", a.name, skillPath, err)
		}
	}
}
