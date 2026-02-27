package rules

import (
	"strings"
)

// SkillRuleMeta holds extracted metadata for rule generation.
type SkillRuleMeta struct {
	SkillRef    string   // e.g. "clawhub/go-test-helper"
	SkillName   string   // e.g. "go-test-helper"
	Name        string   // from frontmatter name field
	Description string   // from frontmatter description field
	Paths       []string // from frontmatter rules.paths or auto-detected
	Scope       string   // "global" or "project"
	Summary     string   // from frontmatter rules.summary or first paragraph
	Source      string   // "frontmatter" or "auto-detected"
}

// frontmatter holds parsed YAML-like frontmatter fields.
type frontmatter struct {
	Name        string
	Description string
	Rules       *frontmatterRules
}

type frontmatterRules struct {
	Paths   []string
	Scope   string
	Summary string
}

// ExtractRuleMeta parses SKILL.md content and returns rule metadata.
// Returns (meta, true) if any rules-relevant metadata was found.
func ExtractRuleMeta(skillRef, skillName, content string) (SkillRuleMeta, bool) {
	meta := SkillRuleMeta{
		SkillRef:  skillRef,
		SkillName: skillName,
	}

	// 1. Try frontmatter extraction
	if fm, ok := parseFrontmatter(content); ok {
		meta.Name = fm.Name
		meta.Description = fm.Description
		if fm.Rules != nil {
			meta.Paths = fm.Rules.Paths
			meta.Scope = fm.Rules.Scope
			meta.Summary = fm.Rules.Summary
			meta.Source = "frontmatter"
		}
	}

	// 2. If no paths from frontmatter, try auto-detection
	if len(meta.Paths) == 0 {
		meta.Paths = inferPathPatterns(content)
		if len(meta.Paths) > 0 {
			meta.Source = "auto-detected"
		}
	}

	// 3. If no summary, extract first meaningful paragraph
	if meta.Summary == "" {
		meta.Summary = extractSummary(content)
	}

	return meta, len(meta.Paths) > 0 || meta.Summary != ""
}

// parseFrontmatter parses a minimal YAML-like frontmatter block
// delimited by "---" lines. Handles name, description, and rules block.
func parseFrontmatter(content string) (frontmatter, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return frontmatter{}, false
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, false
	}

	// Find closing ---
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return frontmatter{}, false
	}

	fm := frontmatter{}
	inRules := false
	inPaths := false

	for i := 1; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Top-level fields (no indentation or minimal)
		if indent == 0 {
			inRules = false
			inPaths = false
			key, val := splitKeyValue(trimmed)
			switch key {
			case "name":
				fm.Name = unquote(val)
			case "description":
				fm.Description = unquote(val)
			case "rules":
				inRules = true
				fm.Rules = &frontmatterRules{}
			}
			continue
		}

		// Inside rules block
		if inRules && indent >= 2 {
			if inPaths {
				// Collecting path list items
				if strings.HasPrefix(trimmed, "- ") {
					item := unquote(strings.TrimPrefix(trimmed, "- "))
					if item != "" {
						fm.Rules.Paths = append(fm.Rules.Paths, item)
					}
					continue
				}
				// No longer a list item — exit paths
				inPaths = false
			}

			key, val := splitKeyValue(trimmed)
			switch key {
			case "paths":
				inPaths = true
				// Could be inline: paths: ["a", "b"] — but we only support block list
			case "scope":
				fm.Rules.Scope = unquote(val)
			case "summary":
				fm.Rules.Summary = unquote(val)
			}
		}
	}

	return fm, fm.Name != "" || fm.Description != "" || fm.Rules != nil
}

// patternRule maps content keywords to file glob patterns.
type patternRule struct {
	keywords []string
	patterns []string
}

var patternRules = []patternRule{
	// Go
	{[]string{"go test", "_test.go", "testing.t", "testify"},
		[]string{"**/*_test.go"}},
	{[]string{"go.mod", "go module", "go build", "package main", "func main()"},
		[]string{"**/*.go"}},

	// TypeScript/JavaScript tests
	{[]string{".spec.ts", ".test.ts", ".spec.js", ".test.js", "jest", "vitest", "mocha"},
		[]string{"**/*.test.ts", "**/*.spec.ts", "**/*.test.js", "**/*.spec.js"}},
	// React
	{[]string{"react", "jsx", "tsx", "usestate", "useeffect"},
		[]string{"**/*.tsx", "**/*.jsx"}},
	// TypeScript general
	{[]string{"typescript", "tsconfig", ".ts file"},
		[]string{"**/*.ts", "**/*.tsx"}},

	// Python tests
	{[]string{"pytest", "unittest", "test_", "def test_"},
		[]string{"**/test_*.py", "**/*_test.py"}},
	// Python general
	{[]string{"python", "django", "flask", "fastapi", "pip install"},
		[]string{"**/*.py"}},

	// Rust
	{[]string{"cargo test", "#[test]", "rust test", "rustfmt"},
		[]string{"**/*.rs"}},

	// Config/CI
	{[]string{"dockerfile", "docker-compose", "container image"},
		[]string{"**/Dockerfile*", "**/docker-compose*.yml"}},
	{[]string{"github actions", ".github/workflows", "workflow_dispatch"},
		[]string{".github/workflows/*.yml", ".github/workflows/*.yaml"}},
	{[]string{"makefile", "make target", "make build"},
		[]string{"**/Makefile", "**/makefile"}},

	// Documentation
	{[]string{"markdown lint", "markdownlint", "readme convention"},
		[]string{"**/*.md"}},

	// SQL/Database
	{[]string{"sql query", "select from", "create table", "migration"},
		[]string{"**/*.sql"}},

	// Shell
	{[]string{"bash script", "shell script", "#!/bin/bash", "#!/bin/sh"},
		[]string{"**/*.sh"}},
}

// inferPathPatterns analyzes SKILL.md content to guess relevant file patterns.
func inferPathPatterns(content string) []string {
	lower := strings.ToLower(content)
	seen := map[string]struct{}{}
	var patterns []string
	for _, rule := range patternRules {
		matched := false
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if matched {
			for _, p := range rule.patterns {
				if _, ok := seen[p]; !ok {
					seen[p] = struct{}{}
					patterns = append(patterns, p)
				}
			}
		}
	}
	return patterns
}

// extractSummary extracts the first meaningful paragraph from SKILL.md content,
// skipping the frontmatter block, headings, and code blocks.
func extractSummary(content string) string {
	lines := strings.Split(content, "\n")
	start := 0

	// Skip frontmatter
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	// Find first non-empty, non-heading paragraph (skip code blocks)
	var paragraph []string
	inParagraph := false
	inCodeBlock := false
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Track code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if inParagraph {
				break
			}
			continue
		}

		// Skip content inside code blocks
		if inCodeBlock {
			continue
		}

		if trimmed == "" {
			if inParagraph {
				break // End of paragraph
			}
			continue
		}

		// Skip headings and HTML comments
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!--") {
			if inParagraph {
				break
			}
			continue
		}

		inParagraph = true
		paragraph = append(paragraph, trimmed)
	}

	result := strings.Join(paragraph, " ")
	if len(result) > 200 {
		result = result[:200]
		// Try to break at word boundary
		if idx := strings.LastIndex(result, " "); idx > 150 {
			result = result[:idx]
		}
		result += "..."
	}
	return result
}

// splitKeyValue splits "key: value" into (key, value).
func splitKeyValue(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return s, ""
	}
	return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
}

// unquote removes surrounding quotes (single or double) from a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
