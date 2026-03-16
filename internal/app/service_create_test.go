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
	if !strings.Contains(content, "author:") {
		t.Error("SKILL.md missing author field")
	}
	if !strings.Contains(content, "description:") {
		t.Error("SKILL.md missing description field")
	}
	if !strings.Contains(content, "triggers:") {
		t.Error("SKILL.md missing triggers field")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("SKILL.md missing Instructions section")
	}
	if !strings.Contains(content, "## When to use") {
		t.Error("SKILL.md missing 'When to use' section")
	}
	if !strings.Contains(content, "## When NOT to use") {
		t.Error("SKILL.md missing 'When NOT to use' section")
	}
	if !strings.Contains(content, "## Examples") {
		t.Error("SKILL.md missing Examples section")
	}
	// Anthropic best practices: description should guide "pushy" triggering
	if !strings.Contains(content, "pushy") {
		t.Error("SKILL.md description TODO should mention 'pushy' triggering guidance")
	}
	// Anthropic best practices: instructions should emphasize explaining "why"
	if !strings.Contains(content, "why") {
		t.Error("SKILL.md Instructions should emphasize explaining 'why'")
	}
	// Anthropic best practices: mention bundled resources pattern
	if !strings.Contains(content, "## Resources") {
		t.Error("SKILL.md missing Resources section for bundled resources guidance")
	}
	if !strings.Contains(content, "references/") {
		t.Error("SKILL.md should mention references/ directory for progressive disclosure")
	}
	// Anthropic best practices: mention ~500 line limit
	if !strings.Contains(content, "500 lines") {
		t.Error("SKILL.md should mention ~500 line limit guidance")
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
	if !strings.Contains(content, "name: test-prompt") {
		t.Error("SKILL.md missing name frontmatter")
	}
	if !strings.Contains(content, "version: 0.1.0") {
		t.Error("SKILL.md missing version")
	}
	if !strings.Contains(content, "author:") {
		t.Error("SKILL.md missing author field")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("SKILL.md missing Instructions section")
	}
	if !strings.Contains(content, "## When to use") {
		t.Error("SKILL.md missing 'When to use' section")
	}
	if !strings.Contains(content, "## When NOT to use") {
		t.Error("SKILL.md missing 'When NOT to use' section")
	}
	// Anthropic best practices: instructions should emphasize explaining "why"
	if !strings.Contains(content, "why") {
		t.Error("SKILL.md Instructions should emphasize explaining 'why'")
	}
	// Anthropic best practices: mention bundled resources pattern
	if !strings.Contains(content, "## Resources") {
		t.Error("SKILL.md missing Resources section for bundled resources guidance")
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
	if !strings.Contains(content, "name: test-script") {
		t.Error("SKILL.md missing name frontmatter")
	}
	if !strings.Contains(content, "version: 0.1.0") {
		t.Error("SKILL.md missing version")
	}
	if !strings.Contains(content, "author:") {
		t.Error("SKILL.md missing author field")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("SKILL.md missing Instructions section")
	}
	if !strings.Contains(content, "## When to use") {
		t.Error("SKILL.md missing 'When to use' section")
	}
	if !strings.Contains(content, "## When NOT to use") {
		t.Error("SKILL.md missing 'When NOT to use' section")
	}
	if !strings.Contains(content, "scripts/") {
		t.Error("SKILL.md should reference scripts directory")
	}
	// Anthropic best practices: mention bundled resources pattern
	if !strings.Contains(content, "## Resources") {
		t.Error("SKILL.md missing Resources section for bundled resources guidance")
	}
	if !strings.Contains(content, "references/") {
		t.Error("SKILL.md should mention references/ directory for progressive disclosure")
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
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Chdir restore: %v", err)
		}
	}()

	skillDir, err := svc.CreateSkill("cwd-skill", "", "default")
	if err != nil {
		t.Fatal(err)
	}
	if skillDir != filepath.Join(canonicalTmpDir, "cwd-skill") {
		t.Errorf("expected dir in cwd, got %q", skillDir)
	}
}

// TestCreateSkill_FrontmatterBlock verifies the generated template starts with
// a proper YAML frontmatter block delimited by "---\n".
func TestCreateSkill_FrontmatterBlock(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	for _, tmpl := range []string{"default", "prompt", "script"} {
		t.Run(tmpl, func(t *testing.T) {
			skillDir, err := svc.CreateSkill("fm-"+tmpl, dir, tmpl)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			if !strings.HasPrefix(content, "---\n") {
				t.Error("SKILL.md should start with '---\\n' frontmatter delimiter")
			}
			// Must have a closing delimiter
			rest := content[4:] // skip opening "---\n"
			if !strings.Contains(rest, "\n---\n") {
				t.Error("SKILL.md missing closing '---\\n' frontmatter delimiter")
			}
		})
	}
}
