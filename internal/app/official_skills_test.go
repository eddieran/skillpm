package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialSkillsCatalogIsComplete(t *testing.T) {
	root := repoRootForCatalog(t)
	skillsRoot := filepath.Join(root, "skills")
	expected := []string{
		"code-reviewer",
		"dependency-auditor",
		"doc-sync",
		"git-conventional",
		"test-writer",
	}

	for _, slug := range expected {
		skillDir := filepath.Join(skillsRoot, slug)
		skillMD := mustReadFile(t, filepath.Join(skillDir, "SKILL.md"))
		readme := mustReadFile(t, filepath.Join(skillDir, "README.md"))
		cases := mustReadFile(t, filepath.Join(skillDir, "tests", "cases.yaml"))

		for _, content := range []string{skillMD, readme} {
			if !strings.Contains(content, "Claude Code") {
				t.Fatalf("%s: expected Claude Code guidance", slug)
			}
			if !strings.Contains(content, "Codex") {
				t.Fatalf("%s: expected Codex guidance", slug)
			}
		}
		if !strings.Contains(skillMD, "name: "+slug) {
			t.Fatalf("%s: missing frontmatter name", slug)
		}
		if !strings.Contains(skillMD, "version: 1.0.0") {
			t.Fatalf("%s: missing version", slug)
		}
		if countCaseIDs(cases) < 3 {
			t.Fatalf("%s: expected at least 3 cases", slug)
		}
		for _, rel := range listedRepoFiles(cases) {
			if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
				t.Fatalf("%s: listed repo file %q missing: %v", slug, rel, err)
			}
		}
	}
}

func repoRootForCatalog(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func countCaseIDs(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- id:") {
			count++
		}
	}
	return count
}

func listedRepoFiles(content string) []string {
	lines := strings.Split(content, "\n")
	var files []string
	inFiles := false
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "files:" {
			inFiles = true
			continue
		}
		if inFiles {
			if strings.HasPrefix(trimmed, "- ") {
				files = append(files, strings.TrimPrefix(trimmed, "- "))
				continue
			}
			if trimmed != "" && !strings.HasPrefix(raw, "      ") && !strings.HasPrefix(raw, "        ") {
				inFiles = false
			}
		}
	}
	return files
}
