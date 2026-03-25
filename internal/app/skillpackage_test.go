package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalSkillPackageCollectsRecursiveFilesAndFrontmatter(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "code-reviewer")
	if err := os.MkdirAll(filepath.Join(skillDir, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: code-reviewer
version: 1.2.3
description: "Structured review"
---
# Code Reviewer
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# Readme"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "tests", "cases.yaml"), []byte("cases: []"), 0o644); err != nil {
		t.Fatalf("write cases.yaml: %v", err)
	}

	pkg, err := loadLocalSkillPackage(skillDir)
	if err != nil {
		t.Fatalf("loadLocalSkillPackage failed: %v", err)
	}
	if pkg.Slug != "code-reviewer" {
		t.Fatalf("Slug = %q, want %q", pkg.Slug, "code-reviewer")
	}
	if pkg.Version != "1.2.3" {
		t.Fatalf("Version = %q, want %q", pkg.Version, "1.2.3")
	}
	if pkg.Description != "Structured review" {
		t.Fatalf("Description = %q, want %q", pkg.Description, "Structured review")
	}
	if got := pkg.Files["README.md"]; got != "# Readme" {
		t.Fatalf("README.md = %q", got)
	}
	if got := pkg.Files["tests/cases.yaml"]; got != "cases: []" {
		t.Fatalf("tests/cases.yaml = %q", got)
	}
}

func TestLoadLocalSkillPackageFallsBackToSummaryAndDefaultVersion(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "doc-sync")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: doc-sync
---
# Doc Sync

Compare docs with implementation and report factual drift.

## Details

More detail here.
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	pkg, err := loadLocalSkillPackage(skillDir)
	if err != nil {
		t.Fatalf("loadLocalSkillPackage failed: %v", err)
	}
	if pkg.Version != "0.1.0" {
		t.Fatalf("Version = %q, want %q", pkg.Version, "0.1.0")
	}
	if pkg.Description != "Compare docs with implementation and report factual drift." {
		t.Fatalf("Description = %q", pkg.Description)
	}
}

func TestLoadLocalSkillPackageUsesDirectoryNameForDotPath(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "test-writer")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`# Test Writer`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(skillDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	pkg, err := loadLocalSkillPackage(".")
	if err != nil {
		t.Fatalf("loadLocalSkillPackage failed: %v", err)
	}
	if pkg.Slug != "test-writer" {
		t.Fatalf("Slug = %q, want %q", pkg.Slug, "test-writer")
	}
}
