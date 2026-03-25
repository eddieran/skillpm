package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/store"
)

func TestSyncRulesForSkillsReadsSanitizedInstalledDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	svc := newRulesEnabledService(t, home, config.ScopeGlobal, "", "global")

	const (
		skillRef = "anthropic/skill-creator"
		version  = "1.0.0"
	)
	writeInstalledSkill(t, svc.StateRoot, skillRef, version, `---
name: Skill Creator
rules:
  paths:
    - "**/*.md"
  summary: Create and maintain skill packages.
---

# Skill Creator
`)

	svc.syncRulesForSkills([]string{skillRef})

	rulePath := filepath.Join(home, ".claude", "rules", "skillpm", "skill-creator.md")
	data, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("expected rules file for %q: %v", skillRef, err)
	}
	if !strings.Contains(string(data), "Create and maintain skill packages.") {
		t.Fatalf("rules file missing extracted summary: %q", string(data))
	}
}

func TestSyncRulesForSkillsUsesProjectScopedRulesDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}
	if _, err := config.InitProject(projectDir); err != nil {
		t.Fatalf("init project failed: %v", err)
	}

	svc := newRulesEnabledService(t, home, config.ScopeProject, projectDir, "project")

	const (
		skillRef = "local/review"
		version  = "1.0.0"
	)
	writeInstalledSkill(t, svc.StateRoot, skillRef, version, `---
name: Review
rules:
  paths:
    - "**/*.go"
  summary: Review Go changes in this project.
---

# Review
`)

	svc.syncRulesForSkills([]string{skillRef})

	projectRulePath := filepath.Join(projectDir, ".claude", "rules", "skillpm", "review.md")
	if _, err := os.Stat(projectRulePath); err != nil {
		t.Fatalf("expected project-scoped rule at %q: %v", projectRulePath, err)
	}

	globalRulePath := filepath.Join(home, ".claude", "rules", "skillpm", "review.md")
	if _, err := os.Stat(globalRulePath); !os.IsNotExist(err) {
		t.Fatalf("expected no global rule at %q, got err=%v", globalRulePath, err)
	}
}

func newRulesEnabledService(t *testing.T, home string, scope config.Scope, projectRoot, rulesScope string) *Service {
	t.Helper()

	configPath := filepath.Join(home, ".skillpm", "config.toml")
	svc, err := New(Options{
		ConfigPath:  configPath,
		Scope:       scope,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	svc.Config.Memory.RulesInjection = true
	svc.Config.Memory.RulesScope = rulesScope
	if err := svc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}

	svc, err = New(Options{
		ConfigPath:  configPath,
		Scope:       scope,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		t.Fatalf("reload service failed: %v", err)
	}
	if svc.Memory == nil || svc.Memory.Rules == nil {
		t.Fatal("expected rules engine to be initialized")
	}

	return svc
}

func writeInstalledSkill(t *testing.T, stateRoot, skillRef, version, content string) {
	t.Helper()

	if err := store.SaveState(stateRoot, store.State{
		Version: store.StateVersion,
		Installed: []store.InstalledSkill{{
			SkillRef:        skillRef,
			ResolvedVersion: version,
		}},
	}); err != nil {
		t.Fatalf("save state failed: %v", err)
	}

	installedDir := store.InstalledDirPath(stateRoot, skillRef, version)
	if err := os.MkdirAll(installedDir, 0o755); err != nil {
		t.Fatalf("mkdir installed dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installedDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md failed: %v", err)
	}
}
