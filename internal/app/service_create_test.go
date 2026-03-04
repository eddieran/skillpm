package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSkill_Default(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	skillDir, err := svc.CreateSkill("my-skill", dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	if skillDir != filepath.Join(dir, "my-skill") {
		t.Errorf("expected dir %q, got %q", filepath.Join(dir, "my-skill"), skillDir)
	}
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "name: my-skill") {
		t.Error("SKILL.md missing name frontmatter")
	}
	if !strings.Contains(content, "version: 0.1.0") {
		t.Error("SKILL.md missing version")
	}
}

func TestCreateSkill_PromptTemplate(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	skillDir, err := svc.CreateSkill("test-prompt", dir, "prompt")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "prompt-based skill") {
		t.Error("SKILL.md should mention prompt-based")
	}
}

func TestCreateSkill_ScriptTemplate(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	skillDir, err := svc.CreateSkill("test-script", dir, "script")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "script-based") {
		t.Error("SKILL.md should mention script-based")
	}
}

func TestCreateSkill_InvalidName(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	_, err := svc.CreateSkill("bad name!", dir, "default")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSkill_EmptyName(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	_, err := svc.CreateSkill("", dir, "default")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateSkill_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "existing"), 0o755)
	svc := &Service{}
	_, err := svc.CreateSkill("existing", dir, "default")
	if err == nil {
		t.Fatal("expected error for existing directory")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSkill_DefaultDir(t *testing.T) {
	// Test with empty dir (uses cwd).
	// On macOS, t.TempDir() may return a path under /var/folders which is a
	// symlink to /private/var/folders. os.Getwd() returns the resolved path,
	// so canonicalize tmpDir before comparing.
	svc := &Service{}
	tmpDir := t.TempDir()
	canonicalTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	skillDir, err := svc.CreateSkill("cwd-skill", "", "default")
	if err != nil {
		t.Fatal(err)
	}
	if skillDir != filepath.Join(canonicalTmpDir, "cwd-skill") {
		t.Errorf("expected dir in cwd, got %q", skillDir)
	}
}
