package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_WithPaths(t *testing.T) {
	e := &Engine{rulesDir: t.TempDir()}
	meta := SkillRuleMeta{
		SkillRef:  "clawhub/go-test-helper",
		SkillName: "go-test-helper",
		Name:      "Go Test Helper",
		Paths:     []string{"**/*_test.go", "**/testdata/**"},
		Summary:   "Prefer table-driven tests with subtests.",
	}
	fname, content := e.Generate(meta)

	if fname != "go-test-helper.md" {
		t.Errorf("filename = %q, want %q", fname, "go-test-helper.md")
	}

	// Check frontmatter
	if !strings.HasPrefix(content, "---\n") {
		t.Error("expected YAML frontmatter")
	}
	if !strings.Contains(content, `"**/*_test.go"`) {
		t.Error("expected path pattern in frontmatter")
	}

	// Check heading
	if !strings.Contains(content, "# Go Test Helper (managed by skillpm)") {
		t.Error("expected skill name in heading")
	}

	// Check summary
	if !strings.Contains(content, "Prefer table-driven tests") {
		t.Error("expected summary in content")
	}

	// Check managed marker
	if !strings.Contains(content, "<!-- skillpm:managed ref=clawhub/go-test-helper") {
		t.Error("expected managed marker with ref")
	}
}

func TestGenerate_WithoutPaths(t *testing.T) {
	e := &Engine{rulesDir: t.TempDir()}
	meta := SkillRuleMeta{
		SkillRef:  "my-repo/generic",
		SkillName: "generic",
		Summary:   "A generic skill.",
	}
	_, content := e.Generate(meta)

	// No frontmatter when no paths
	if strings.HasPrefix(content, "---") {
		t.Error("should not have frontmatter without paths")
	}

	// Heading uses SkillName when Name is empty
	if !strings.Contains(content, "# generic (managed by skillpm)") {
		t.Errorf("content = %q, expected SkillName in heading", content)
	}
}

func TestSync_CreateNew(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	metas := []SkillRuleMeta{
		{
			SkillRef:  "a/go-helper",
			SkillName: "go-helper",
			Paths:     []string{"**/*.go"},
			Summary:   "Go helper skill.",
		},
		{
			SkillRef:  "a/react-guide",
			SkillName: "react-guide",
			Paths:     []string{"**/*.tsx"},
			Summary:   "React guide skill.",
		},
	}

	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Verify files created
	files, err := e.ListManaged()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 managed files, got %d", len(files))
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(rulesDir, "go-helper.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "**/*.go") {
		t.Error("expected Go path pattern")
	}
}

func TestSync_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	// Initial sync
	metas := []SkillRuleMeta{
		{
			SkillRef:  "a/skill",
			SkillName: "skill",
			Paths:     []string{"**/*.go"},
			Summary:   "Original summary.",
		},
	}
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Update with different summary
	metas[0].Summary = "Updated summary."
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(rulesDir, "skill.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Updated summary") {
		t.Error("expected updated summary")
	}
	if strings.Contains(string(data), "Original summary") {
		t.Error("should not contain old summary")
	}
}

func TestSync_RemoveStale(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	// Initial sync with two skills
	metas := []SkillRuleMeta{
		{SkillRef: "a/kept", SkillName: "kept", Paths: []string{"**/*.go"}, Summary: "Keep."},
		{SkillRef: "a/removed", SkillName: "removed", Paths: []string{"**/*.py"}, Summary: "Remove."},
	}
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Sync with only one skill — the other should be removed
	metas = metas[:1]
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	files, err := e.ListManaged()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 managed file, got %d: %v", len(files), files)
	}
	if files[0] != "kept.md" {
		t.Errorf("expected kept.md, got %q", files[0])
	}

	// Verify removed file is gone
	if _, err := os.Stat(filepath.Join(rulesDir, "removed.md")); !os.IsNotExist(err) {
		t.Error("expected removed.md to be deleted")
	}
}

func TestSync_PreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a user-owned file (no managed marker)
	userFile := filepath.Join(rulesDir, "user-custom.md")
	if err := os.WriteFile(userFile, []byte("# My Custom Rule\n\nDo things.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &Engine{rulesDir: rulesDir}

	// Sync with no skills — should NOT delete user file
	if err := e.Sync(nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(userFile); os.IsNotExist(err) {
		t.Fatal("user file should not be deleted by Sync")
	}
}

func TestSync_EmptyMetasRemovesAllManaged(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	// Create one
	metas := []SkillRuleMeta{
		{SkillRef: "a/b", SkillName: "b", Paths: []string{"**/*.go"}, Summary: "Test."},
	}
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Sync with empty — removes all
	if err := e.Sync(nil); err != nil {
		t.Fatal(err)
	}

	files, err := e.ListManaged()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 managed files after empty sync, got %d", len(files))
	}
}

func TestSync_SkipsMetasWithNoPaths(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	metas := []SkillRuleMeta{
		{SkillRef: "a/no-paths", SkillName: "no-paths"}, // no paths, no summary
	}

	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Should not create directory since there's nothing to write
	if _, err := os.Stat(rulesDir); !os.IsNotExist(err) {
		t.Error("should not create rules dir for empty targets")
	}
}

func TestCleanup(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	// Create managed files
	metas := []SkillRuleMeta{
		{SkillRef: "a/x", SkillName: "x", Paths: []string{"**/*.go"}, Summary: "X."},
		{SkillRef: "a/y", SkillName: "y", Paths: []string{"**/*.py"}, Summary: "Y."},
	}
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	// Also create a user file
	userFile := filepath.Join(rulesDir, "user.md")
	if err := os.WriteFile(userFile, []byte("# User\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := e.Cleanup(); err != nil {
		t.Fatal(err)
	}

	// Managed files should be gone
	managed, _ := e.ListManaged()
	if len(managed) != 0 {
		t.Errorf("expected 0 managed files after cleanup, got %d", len(managed))
	}

	// User file should remain
	if _, err := os.Stat(userFile); os.IsNotExist(err) {
		t.Error("user file should survive cleanup")
	}
}

func TestCleanup_NoDir(t *testing.T) {
	e := &Engine{rulesDir: filepath.Join(t.TempDir(), "nonexistent")}
	if err := e.Cleanup(); err != nil {
		t.Errorf("cleanup on nonexistent dir should not error: %v", err)
	}
}

func TestListManaged_Empty(t *testing.T) {
	e := &Engine{rulesDir: filepath.Join(t.TempDir(), "nonexistent")}
	files, err := e.ListManaged()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestNewEngine_GlobalScope(t *testing.T) {
	e := NewEngine("global", "", "/home/user")
	want := "/home/user/.claude/rules/skillpm"
	if e.RulesDir() != want {
		t.Errorf("RulesDir = %q, want %q", e.RulesDir(), want)
	}
}

func TestNewEngine_ProjectScope(t *testing.T) {
	e := NewEngine("project", "/projects/myapp", "/home/user")
	want := "/projects/myapp/.claude/rules/skillpm"
	if e.RulesDir() != want {
		t.Errorf("RulesDir = %q, want %q", e.RulesDir(), want)
	}
}

func TestNewEngine_EmptyScope(t *testing.T) {
	e := NewEngine("", "", "/home/user")
	want := "/home/user/.claude/rules/skillpm"
	if e.RulesDir() != want {
		t.Errorf("RulesDir = %q, want %q", e.RulesDir(), want)
	}
}

func TestSync_Idempotent(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".claude", "rules", "skillpm")
	e := &Engine{rulesDir: rulesDir}

	metas := []SkillRuleMeta{
		{SkillRef: "a/stable", SkillName: "stable", Paths: []string{"**/*.go"}, Summary: "Stable."},
	}

	// Run sync twice
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}
	if err := e.Sync(metas); err != nil {
		t.Fatal(err)
	}

	files, err := e.ListManaged()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file after idempotent sync, got %d", len(files))
	}
}

func TestIsManaged(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"<!-- skillpm:managed ref=a/b checksum=abc -->", true},
		{"some text\n<!-- skillpm:managed -->", true},
		{"# Just a heading\nNo marker.", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isManaged(tt.content); got != tt.want {
			t.Errorf("isManaged(%q) = %v, want %v", tt.content[:min(len(tt.content), 40)], got, tt.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
