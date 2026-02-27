package rules

import (
	"strings"
	"testing"
)

func TestExtractRuleMeta_WithFullFrontmatter(t *testing.T) {
	content := `---
name: "go-test-helper"
description: "Generate and improve Go tests"
rules:
  paths:
    - "**/*_test.go"
    - "**/testdata/**"
  scope: "project"
  summary: "Prefer table-driven tests with subtests"
---

# Go Test Helper

This skill helps you write better Go tests.
`
	meta, ok := ExtractRuleMeta("clawhub/go-test-helper", "go-test-helper", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Name != "go-test-helper" {
		t.Errorf("Name = %q, want %q", meta.Name, "go-test-helper")
	}
	if meta.Description != "Generate and improve Go tests" {
		t.Errorf("Description = %q, want %q", meta.Description, "Generate and improve Go tests")
	}
	if len(meta.Paths) != 2 {
		t.Fatalf("len(Paths) = %d, want 2", len(meta.Paths))
	}
	if meta.Paths[0] != "**/*_test.go" {
		t.Errorf("Paths[0] = %q, want %q", meta.Paths[0], "**/*_test.go")
	}
	if meta.Paths[1] != "**/testdata/**" {
		t.Errorf("Paths[1] = %q, want %q", meta.Paths[1], "**/testdata/**")
	}
	if meta.Scope != "project" {
		t.Errorf("Scope = %q, want %q", meta.Scope, "project")
	}
	if meta.Summary != "Prefer table-driven tests with subtests" {
		t.Errorf("Summary = %q, want %q", meta.Summary, "Prefer table-driven tests with subtests")
	}
	if meta.Source != "frontmatter" {
		t.Errorf("Source = %q, want %q", meta.Source, "frontmatter")
	}
	if meta.SkillRef != "clawhub/go-test-helper" {
		t.Errorf("SkillRef = %q", meta.SkillRef)
	}
	if meta.SkillName != "go-test-helper" {
		t.Errorf("SkillName = %q", meta.SkillName)
	}
}

func TestExtractRuleMeta_NameOnly(t *testing.T) {
	content := `---
name: "code-review"
description: "Review code for quality"
---

# Code Review

Review code for quality, security, and maintainability.
Use the OWASP top 10 checklist for security reviews.
`
	meta, ok := ExtractRuleMeta("my-repo/code-review", "code-review", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Name != "code-review" {
		t.Errorf("Name = %q", meta.Name)
	}
	// No frontmatter paths → should fall back to auto-detection or summary
	if meta.Source == "frontmatter" && len(meta.Paths) > 0 {
		t.Error("should not have frontmatter paths")
	}
	// Should have summary from content
	if meta.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestExtractRuleMeta_NoFrontmatter_GoContent(t *testing.T) {
	content := `# Go Test Helper

This skill helps you write better Go tests using testing.T and testify.

## Usage

When writing _test.go files, prefer table-driven tests.
`
	meta, ok := ExtractRuleMeta("my-repo/go-helper", "go-helper", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if meta.Source != "auto-detected" {
		t.Errorf("Source = %q, want %q", meta.Source, "auto-detected")
	}
	foundTestGo := false
	for _, p := range meta.Paths {
		if p == "**/*_test.go" {
			foundTestGo = true
		}
	}
	if !foundTestGo {
		t.Errorf("expected **/*_test.go in Paths, got %v", meta.Paths)
	}
}

func TestExtractRuleMeta_NoFrontmatter_ReactContent(t *testing.T) {
	content := `# React Component Guide

Build React components using TSX and functional patterns.
Use useState and useEffect hooks for state management.
`
	meta, ok := ExtractRuleMeta("my-repo/react-guide", "react-guide", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	foundTsx := false
	for _, p := range meta.Paths {
		if p == "**/*.tsx" {
			foundTsx = true
		}
	}
	if !foundTsx {
		t.Errorf("expected **/*.tsx in Paths, got %v", meta.Paths)
	}
}

func TestExtractRuleMeta_NoFrontmatter_PythonContent(t *testing.T) {
	content := `# Pytest Helper

Use pytest for all test files. Write fixtures in conftest.py.
`
	meta, ok := ExtractRuleMeta("my-repo/pytest-help", "pytest-help", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	foundPy := false
	for _, p := range meta.Paths {
		if p == "**/test_*.py" {
			foundPy = true
		}
	}
	if !foundPy {
		t.Errorf("expected **/test_*.py in Paths, got %v", meta.Paths)
	}
}

func TestExtractRuleMeta_NoFrontmatter_DockerContent(t *testing.T) {
	content := `# Docker Best Practices

When writing Dockerfile, use multi-stage builds.
`
	meta, ok := ExtractRuleMeta("my-repo/docker-bp", "docker-bp", content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	foundDocker := false
	for _, p := range meta.Paths {
		if p == "**/Dockerfile*" {
			foundDocker = true
		}
	}
	if !foundDocker {
		t.Errorf("expected **/Dockerfile* in Paths, got %v", meta.Paths)
	}
}

func TestExtractRuleMeta_EmptyContent(t *testing.T) {
	_, ok := ExtractRuleMeta("a/b", "b", "")
	if ok {
		t.Error("expected ok=false for empty content")
	}
}

func TestExtractRuleMeta_NoMatchContent(t *testing.T) {
	content := `---
name: "generic-skill"
---

Some generic instructions that don't match any technology.
`
	meta, ok := ExtractRuleMeta("a/generic", "generic", content)
	if !ok {
		t.Fatal("expected ok=true (has summary)")
	}
	if meta.Source == "auto-detected" && len(meta.Paths) > 0 {
		t.Errorf("unexpected auto-detected paths: %v", meta.Paths)
	}
	if meta.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestParseFrontmatter_Valid(t *testing.T) {
	content := `---
name: "my-skill"
description: "A skill"
rules:
  paths:
    - "**/*.go"
  scope: "global"
  summary: "Use this for Go files"
---
Body content.`
	fm, ok := parseFrontmatter(content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if fm.Name != "my-skill" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.Description != "A skill" {
		t.Errorf("Description = %q", fm.Description)
	}
	if fm.Rules == nil {
		t.Fatal("Rules is nil")
	}
	if len(fm.Rules.Paths) != 1 || fm.Rules.Paths[0] != "**/*.go" {
		t.Errorf("Rules.Paths = %v", fm.Rules.Paths)
	}
	if fm.Rules.Scope != "global" {
		t.Errorf("Rules.Scope = %q", fm.Rules.Scope)
	}
	if fm.Rules.Summary != "Use this for Go files" {
		t.Errorf("Rules.Summary = %q", fm.Rules.Summary)
	}
}

func TestParseFrontmatter_NoClosingDelimiter(t *testing.T) {
	content := `---
name: "broken"
`
	_, ok := parseFrontmatter(content)
	if ok {
		t.Error("expected ok=false for unclosed frontmatter")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a heading

No frontmatter here.`
	_, ok := parseFrontmatter(content)
	if ok {
		t.Error("expected ok=false for no frontmatter")
	}
}

func TestParseFrontmatter_MultiplePaths(t *testing.T) {
	content := `---
name: "multi"
rules:
  paths:
    - "**/*.ts"
    - "**/*.tsx"
    - "**/*.js"
---
`
	fm, ok := parseFrontmatter(content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if fm.Rules == nil || len(fm.Rules.Paths) != 3 {
		t.Fatalf("expected 3 paths, got %v", fm.Rules)
	}
	expected := []string{"**/*.ts", "**/*.tsx", "**/*.js"}
	for i, p := range fm.Rules.Paths {
		if p != expected[i] {
			t.Errorf("Paths[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestParseFrontmatter_SingleQuotes(t *testing.T) {
	content := `---
name: 'single-quoted'
description: 'A description'
---
`
	fm, ok := parseFrontmatter(content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if fm.Name != "single-quoted" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.Description != "A description" {
		t.Errorf("Description = %q", fm.Description)
	}
}

func TestParseFrontmatter_UnquotedValues(t *testing.T) {
	content := `---
name: unquoted-name
description: An unquoted description
---
`
	fm, ok := parseFrontmatter(content)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if fm.Name != "unquoted-name" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.Description != "An unquoted description" {
		t.Errorf("Description = %q", fm.Description)
	}
}

func TestInferPathPatterns_GitHubActions(t *testing.T) {
	content := `This skill helps with GitHub Actions workflows.`
	patterns := inferPathPatterns(content)
	found := false
	for _, p := range patterns {
		if p == ".github/workflows/*.yml" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GitHub Actions pattern, got %v", patterns)
	}
}

func TestInferPathPatterns_Shell(t *testing.T) {
	content := `Write bash scripts with proper error handling. Use #!/bin/bash.`
	patterns := inferPathPatterns(content)
	found := false
	for _, p := range patterns {
		if p == "**/*.sh" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected shell pattern, got %v", patterns)
	}
}

func TestInferPathPatterns_NoDuplicates(t *testing.T) {
	content := `Use go test and testing.T for all _test.go files.`
	patterns := inferPathPatterns(content)
	seen := map[string]int{}
	for _, p := range patterns {
		seen[p]++
		if seen[p] > 1 {
			t.Errorf("duplicate pattern: %q", p)
		}
	}
}

func TestInferPathPatterns_Empty(t *testing.T) {
	patterns := inferPathPatterns("nothing relevant here")
	if len(patterns) != 0 {
		t.Errorf("expected empty, got %v", patterns)
	}
}

func TestExtractSummary_Basic(t *testing.T) {
	content := `# Heading

This is the first paragraph of the skill.
It spans multiple lines.

This is the second paragraph.
`
	summary := extractSummary(content)
	if !strings.Contains(summary, "first paragraph") {
		t.Errorf("summary = %q, expected to contain 'first paragraph'", summary)
	}
	if strings.Contains(summary, "second paragraph") {
		t.Errorf("summary should not contain second paragraph")
	}
}

func TestExtractSummary_WithFrontmatter(t *testing.T) {
	content := `---
name: "test"
---

# Heading

The actual content starts here.
`
	summary := extractSummary(content)
	if !strings.Contains(summary, "actual content") {
		t.Errorf("summary = %q, expected to contain 'actual content'", summary)
	}
}

func TestExtractSummary_Truncation(t *testing.T) {
	longLine := strings.Repeat("word ", 50)
	content := "# Heading\n\n" + longLine + "\n"
	summary := extractSummary(content)
	if len(summary) > 210 { // 200 + "..."
		t.Errorf("summary too long: %d chars", len(summary))
	}
	if !strings.HasSuffix(summary, "...") {
		t.Errorf("expected truncated summary to end with '...'")
	}
}

func TestExtractSummary_SkipsCodeBlocks(t *testing.T) {
	content := `# Title

` + "```" + `
code block content
` + "```" + `

Real summary paragraph here.
`
	summary := extractSummary(content)
	if strings.Contains(summary, "code block") {
		t.Errorf("summary should not contain code block: %q", summary)
	}
	if !strings.Contains(summary, "Real summary") {
		t.Errorf("expected 'Real summary' in summary: %q", summary)
	}
}

func TestExtractSummary_Empty(t *testing.T) {
	summary := extractSummary("")
	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`""`, ""},
		{`"`, `"`},
		{`  "spaced"  `, "spaced"},
	}
	for _, tt := range tests {
		got := unquote(tt.input)
		if got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantKey   string
		wantValue string
	}{
		{"name: hello", "name", "hello"},
		{"name:hello", "name", "hello"},
		{"name:", "name", ""},
		{"nocolon", "nocolon", ""},
		{"key: value: with: colons", "key", "value: with: colons"},
	}
	for _, tt := range tests {
		k, v := splitKeyValue(tt.input)
		if k != tt.wantKey || v != tt.wantValue {
			t.Errorf("splitKeyValue(%q) = (%q, %q), want (%q, %q)",
				tt.input, k, v, tt.wantKey, tt.wantValue)
		}
	}
}
