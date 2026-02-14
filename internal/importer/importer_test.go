package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSkillDirSuccess(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "example-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	desc, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("validate skill dir: %v", err)
	}
	if desc.Name != "example-skill" {
		t.Fatalf("expected name example-skill, got %q", desc.Name)
	}
	if desc.RootPath != filepath.Clean(skillDir) {
		t.Fatalf("expected root path %q, got %q", filepath.Clean(skillDir), desc.RootPath)
	}
	if desc.SkillFile != filepath.Join(filepath.Clean(skillDir), "SKILL.md") {
		t.Fatalf("unexpected skill file: %q", desc.SkillFile)
	}
}

func TestValidateSkillDirMissingSkillFile(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "missing-skill-file")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}

	_, err := ValidateSkillDir(skillDir)
	if err == nil {
		t.Fatalf("expected missing SKILL.md error")
	}
	if !strings.Contains(err.Error(), "IMP_SKILL_SHAPE") || !strings.Contains(err.Error(), "missing SKILL.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSkillDirSkillFileIsDirectory(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "bad-shape")
	if err := os.MkdirAll(filepath.Join(skillDir, "SKILL.md"), 0o755); err != nil {
		t.Fatalf("create SKILL.md directory: %v", err)
	}

	_, err := ValidateSkillDir(skillDir)
	if err == nil {
		t.Fatalf("expected SKILL.md directory error")
	}
	if !strings.Contains(err.Error(), "IMP_SKILL_SHAPE") || !strings.Contains(err.Error(), "SKILL.md is a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSkillDirInvalidDirectoryName(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "   ")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	_, err := ValidateSkillDir(skillDir)
	if err == nil {
		t.Fatalf("expected invalid directory name error")
	}
	if !strings.Contains(err.Error(), "IMP_SKILL_SHAPE") || !strings.Contains(err.Error(), "invalid skill directory name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "Example Skill", want: "example-skill"},
		{in: "  Mixed Case Name  ", want: "mixed-case-name"},
		{in: "already-kebab", want: "already-kebab"},
		{in: "two  spaces", want: "two--spaces"},
	}
	for _, tc := range cases {
		if got := NormalizeName(tc.in); got != tc.want {
			t.Fatalf("normalize %q: got %q want %q", tc.in, got, tc.want)
		}
	}
}
