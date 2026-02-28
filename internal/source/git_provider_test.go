package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
)

// mockGitExec returns a gitExecFunc that records calls and returns canned responses.
func mockGitExec(calls *[]string, responses map[string]string, errors map[string]error) gitExecFunc {
	return func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		*calls = append(*calls, key)
		if err, ok := errors[key]; ok {
			return nil, err
		}
		// Match by prefix for flexibility
		for pattern, resp := range responses {
			if strings.HasPrefix(key, pattern) {
				return []byte(resp), nil
			}
		}
		return []byte(""), nil
	}
}

// setupFakeCache creates a fake git cache directory with skill files.
func setupFakeCache(t *testing.T, cacheDir string, skills map[string]map[string]string) {
	t.Helper()
	// Create .git dir to mark as git repo
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("create .git dir failed: %v", err)
	}
	// Create skills under "skills/" scan path
	for skillName, files := range skills {
		for relPath, content := range files {
			fullPath := filepath.Join(cacheDir, "skills", skillName, relPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatalf("mkdir failed: %v", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				t.Fatalf("write file failed: %v", err)
			}
		}
	}
}

func testSourceConfig(name, url string) config.SourceConfig {
	return config.SourceConfig{
		Name:      name,
		Kind:      "git",
		URL:       url,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}
}

func TestGitProviderUpdateClonesWhenNoCache(t *testing.T) {
	var calls []string
	p := &gitProvider{
		cacheRoot: t.TempDir(),
		execGit:   mockGitExec(&calls, nil, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	_, err := p.Update(context.Background(), src)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 git call, got %d: %v", len(calls), calls)
	}
	if !strings.HasPrefix(calls[0], "clone") {
		t.Fatalf("expected clone command, got %q", calls[0])
	}
	if !strings.Contains(calls[0], "--depth 1") {
		t.Fatalf("expected shallow clone, got %q", calls[0])
	}
}

func TestGitProviderUpdateFetchesWhenCacheExists(t *testing.T) {
	var calls []string
	cacheRoot := t.TempDir()
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, nil, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	// Pre-create cache dir with .git
	cacheDir := p.repoCacheDir(src)
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	_, err := p.Update(context.Background(), src)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 git calls (fetch+reset), got %d: %v", len(calls), calls)
	}
	if !strings.HasPrefix(calls[0], "fetch") {
		t.Fatalf("expected fetch command, got %q", calls[0])
	}
	if !strings.HasPrefix(calls[1], "reset") {
		t.Fatalf("expected reset command, got %q", calls[1])
	}
}

func TestGitProviderUpdateErrorOnEmptyURL(t *testing.T) {
	p := &gitProvider{cacheRoot: t.TempDir(), execGit: defaultGitExec}
	src := testSourceConfig("test", "")
	src.URL = ""

	_, err := p.Update(context.Background(), src)
	if err == nil || !strings.Contains(err.Error(), "SRC_GIT_UPDATE") {
		t.Fatalf("expected SRC_GIT_UPDATE error, got %v", err)
	}
}

func TestGitProviderResolveReadsContent(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string
	responses := map[string]string{
		"rev-parse": "abc1234\n",
	}
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, responses, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	// Setup fake cache
	cacheDir := p.repoCacheDir(src)
	setupFakeCache(t, cacheDir, map[string]map[string]string{
		"docx": {
			"SKILL.md":      "# docx\nDocument creation skill",
			"tools/run.sh":  "#!/bin/bash\necho hello",
			"references.md": "Some references",
		},
	})

	result, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "docx"})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if result.Content != "# docx\nDocument creation skill" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if len(result.Files) != 2 {
		t.Fatalf("expected 2 ancillary files, got %d: %v", len(result.Files), result.Files)
	}
	if result.Files["tools/run.sh"] != "#!/bin/bash\necho hello" {
		t.Fatalf("unexpected tools/run.sh content: %q", result.Files["tools/run.sh"])
	}
	if result.Files["references.md"] != "Some references" {
		t.Fatalf("unexpected references.md content: %q", result.Files["references.md"])
	}
	if !strings.HasPrefix(result.ResolvedVersion, "0.0.0+git.abc1234") {
		t.Fatalf("unexpected version: %q", result.ResolvedVersion)
	}
	if !strings.HasPrefix(result.Checksum, "sha256:") {
		t.Fatalf("unexpected checksum: %q", result.Checksum)
	}
}

func TestGitProviderResolveSkillNotFound(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, nil, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	// Setup fake cache with no skills
	cacheDir := p.repoCacheDir(src)
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	_, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "nonexistent"})
	if err == nil || !strings.Contains(err.Error(), "SRC_GIT_RESOLVE: skill \"nonexistent\" not found") {
		t.Fatalf("expected skill not found error, got %v", err)
	}
}

func TestGitProviderResolveAutoClones(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string

	// We need clone to create the cache dir with skills.
	// Simulate by having the mock execGit create the directory structure on clone.
	cloneCalled := false
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit: func(ctx context.Context, dir string, args ...string) ([]byte, error) {
			key := strings.Join(args, " ")
			calls = append(calls, key)
			if strings.HasPrefix(key, "clone") && !cloneCalled {
				cloneCalled = true
				// Extract the target directory (last arg)
				targetDir := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(targetDir, ".git"), 0o755); err != nil {
					return nil, err
				}
				skillDir := filepath.Join(targetDir, "skills", "demo")
				if err := os.MkdirAll(skillDir, 0o755); err != nil {
					return nil, err
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# demo\nDemo skill"), 0o644); err != nil {
					return nil, err
				}
				return []byte("Cloning...\n"), nil
			}
			if strings.HasPrefix(key, "rev-parse") {
				return []byte("deadbeef\n"), nil
			}
			return []byte(""), nil
		},
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	result, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "demo"})
	if err != nil {
		t.Fatalf("resolve with auto-clone failed: %v", err)
	}
	if !cloneCalled {
		t.Fatalf("expected auto-clone to be triggered")
	}
	if result.Content != "# demo\nDemo skill" {
		t.Fatalf("unexpected content after auto-clone: %q", result.Content)
	}
}

func TestGitProviderResolveWithConstraint(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, nil, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	cacheDir := p.repoCacheDir(src)
	setupFakeCache(t, cacheDir, map[string]map[string]string{
		"docx": {"SKILL.md": "# docx\nA skill"},
	})

	result, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "docx", Constraint: "1.2.3"})
	if err != nil {
		t.Fatalf("resolve with constraint failed: %v", err)
	}
	if result.ResolvedVersion != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", result.ResolvedVersion)
	}
}

func TestGitProviderSearchFindsSkills(t *testing.T) {
	cacheRoot := t.TempDir()
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   nil, // Search doesn't use execGit
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	cacheDir := p.repoCacheDir(src)
	setupFakeCache(t, cacheDir, map[string]map[string]string{
		"docx":  {"SKILL.md": "# docx\nDocument skill"},
		"forms": {"SKILL.md": "# forms\nForms skill"},
		"pdf":   {"SKILL.md": "# pdf\nPDF skill"},
	})

	// Search for "doc" should find "docx"
	results, err := p.Search(context.Background(), src, "doc")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	if results[0].Name != "docx" {
		t.Fatalf("expected docx, got %q", results[0].Name)
	}

	// Search with empty query should return all
	allResults, err := p.Search(context.Background(), src, "")
	if err != nil {
		t.Fatalf("search all failed: %v", err)
	}
	if len(allResults) != 3 {
		t.Fatalf("expected 3 results, got %d", len(allResults))
	}
}

func TestGitProviderSearchRequiresCache(t *testing.T) {
	p := &gitProvider{
		cacheRoot: t.TempDir(),
		execGit:   nil,
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	_, err := p.Search(context.Background(), src, "docx")
	if err == nil || !strings.Contains(err.Error(), "SRC_GIT_SEARCH") {
		t.Fatalf("expected SRC_GIT_SEARCH error, got %v", err)
	}
	if !strings.Contains(err.Error(), "not cloned") {
		t.Fatalf("expected 'not cloned' in error message, got %v", err)
	}
}

func TestGitProviderResolveScanPathReturnsError(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, nil, nil),
	}
	// Use ScanPaths=["."] to simulate URL-based install
	src := config.SourceConfig{
		Name:      "eddieran_skills",
		Kind:      "git",
		URL:       "https://github.com/eddieran/skills.git",
		Branch:    "main",
		ScanPaths: []string{"."},
		TrustTier: "review",
	}

	// Setup fake cache: skills/ is a scan-path directory, not a skill itself
	cacheDir := p.repoCacheDir(src)
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	for _, skill := range []string{"docx", "pdf"} {
		dir := filepath.Join(cacheDir, "skills", skill)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+skill), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	// Resolve "skills" should return a ScanPathError
	_, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "skills"})
	if err == nil {
		t.Fatalf("expected ScanPathError, got nil")
	}
	var scanErr *ScanPathError
	if !errors.As(err, &scanErr) {
		t.Fatalf("expected *ScanPathError, got %T: %v", err, err)
	}
	if scanErr.Path != "skills" {
		t.Fatalf("expected path 'skills', got %q", scanErr.Path)
	}
	if len(scanErr.AvailableSkills) != 2 {
		t.Fatalf("expected 2 available skills, got %d: %v", len(scanErr.AvailableSkills), scanErr.AvailableSkills)
	}
	// Skills should be sorted and include the prefix
	if scanErr.AvailableSkills[0] != "skills/docx" || scanErr.AvailableSkills[1] != "skills/pdf" {
		t.Fatalf("unexpected available skills: %v", scanErr.AvailableSkills)
	}
}

func TestListSkillsInDirFindsNestedSkills(t *testing.T) {
	cacheDir := t.TempDir()
	// Create nested skill structure: skills/author/my-skill/SKILL.md
	nested := filepath.Join(cacheDir, "skills", "author", "my-skill")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("# nested"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// Create flat skill: skills/simple/SKILL.md
	flat := filepath.Join(cacheDir, "skills", "simple")
	if err := os.MkdirAll(flat, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(flat, "SKILL.md"), []byte("# simple"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	skills := listSkillsInDir(cacheDir, []string{"."}, "skills")
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(skills), skills)
	}
	if skills[0] != "skills/author/my-skill" || skills[1] != "skills/simple" {
		t.Fatalf("unexpected skills: %v", skills)
	}
}

// Verify computeChecksum is deterministic.
func TestComputeChecksumDeterministic(t *testing.T) {
	content := []byte("# skill\nContent")
	files := map[string]string{
		"b.txt": "BBB",
		"a.txt": "AAA",
	}
	c1 := computeChecksum(content, files)
	c2 := computeChecksum(content, files)
	if c1 != c2 {
		t.Fatalf("checksum not deterministic: %q != %q", c1, c2)
	}
	if !strings.HasPrefix(c1, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", c1)
	}
}

func TestReadFirstHeading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\ntitle: test\n---\n# My Skill\nSome content"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	heading := readFirstHeading(path)
	if heading != "My Skill" {
		t.Fatalf("expected 'My Skill', got %q", heading)
	}

	// File without heading
	noHeadingPath := filepath.Join(dir, "no-heading.md")
	if err := os.WriteFile(noHeadingPath, []byte("no heading here"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := readFirstHeading(noHeadingPath); got != "" {
		t.Fatalf("expected empty heading, got %q", got)
	}
}

// --- Git quiet mode tests ---

func TestGitProviderQuietField(t *testing.T) {
	p := &gitProvider{cacheRoot: t.TempDir(), execGit: newGitExec(true), quiet: true}
	if !p.quiet {
		t.Fatal("expected quiet=true")
	}
	p2 := &gitProvider{cacheRoot: t.TempDir(), execGit: newGitExec(false), quiet: false}
	if p2.quiet {
		t.Fatal("expected quiet=false")
	}
}

func TestNewGitExecQuietModeDoesNotPanic(t *testing.T) {
	// Verify newGitExec returns a callable function for both modes
	qFn := newGitExec(true)
	if qFn == nil {
		t.Fatal("expected non-nil gitExecFunc for quiet=true")
	}
	vFn := newGitExec(false)
	if vFn == nil {
		t.Fatal("expected non-nil gitExecFunc for quiet=false")
	}
}

// --- Path traversal tests ---

func TestFindSkillDirRejectsPathTraversal(t *testing.T) {
	cacheDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	tests := []string{
		"../../etc/passwd",
		"../secret",
		"foo/../../../bar",
		"..",
	}
	for _, skill := range tests {
		_, err := findSkillDir(cacheDir, []string{"."}, skill)
		if err == nil {
			t.Fatalf("expected error for skill name %q, got nil", skill)
		}
		if !strings.Contains(err.Error(), "invalid skill name") {
			t.Fatalf("expected 'invalid skill name' error for %q, got: %v", skill, err)
		}
	}
}

func TestFindSkillDirAllowsNestedPaths(t *testing.T) {
	cacheDir := t.TempDir()
	// Create a nested skill: skills/author/my-skill/SKILL.md
	nested := filepath.Join(cacheDir, "skills", "author", "my-skill")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("# nested"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result, err := findSkillDir(cacheDir, []string{"skills"}, "author/my-skill")
	if err != nil {
		t.Fatalf("expected nested path to be allowed, got: %v", err)
	}
	if result != nested {
		t.Fatalf("expected %q, got %q", nested, result)
	}
}

func TestListSkillsInDirRejectsPathTraversal(t *testing.T) {
	cacheDir := t.TempDir()
	result := listSkillsInDir(cacheDir, []string{"."}, "../../etc")
	if len(result) != 0 {
		t.Fatalf("expected empty result for path traversal prefix, got %v", result)
	}
}

func TestGitProviderResolveRejectsPathTraversal(t *testing.T) {
	cacheRoot := t.TempDir()
	var calls []string
	p := &gitProvider{
		cacheRoot: cacheRoot,
		execGit:   mockGitExec(&calls, nil, nil),
	}
	src := testSourceConfig("test", "https://github.com/test/skills.git")

	cacheDir := p.repoCacheDir(src)
	if err := os.MkdirAll(filepath.Join(cacheDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	_, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: "../../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path traversal skill name")
	}
	if !strings.Contains(err.Error(), "invalid skill name") {
		t.Fatalf("expected 'invalid skill name' error, got: %v", err)
	}
}

func TestDetectCurrentBranchFallback(t *testing.T) {
	var calls []string
	errs := map[string]error{
		"rev-parse --abbrev-ref HEAD": errors.New("not a git repo"),
	}
	p := &gitProvider{
		cacheRoot: t.TempDir(),
		execGit:   mockGitExec(&calls, nil, errs),
	}
	branch := detectCurrentBranch(p, context.Background(), "/nonexistent")
	if branch != "main" {
		t.Fatalf("expected fallback to 'main', got %q", branch)
	}
}

func TestDetectCurrentBranchDetachedHead(t *testing.T) {
	var calls []string
	responses := map[string]string{
		"rev-parse --abbrev-ref HEAD": "HEAD\n",
	}
	p := &gitProvider{
		cacheRoot: t.TempDir(),
		execGit:   mockGitExec(&calls, responses, nil),
	}
	branch := detectCurrentBranch(p, context.Background(), "/some/dir")
	if branch != "main" {
		t.Fatalf("expected fallback to 'main' for detached HEAD, got %q", branch)
	}
}

func TestGitProviderResolveEmptySkill(t *testing.T) {
	p := &gitProvider{cacheRoot: t.TempDir(), execGit: newGitExec(true)}
	src := testSourceConfig("test", "https://github.com/test/skills.git")
	_, err := p.Resolve(context.Background(), src, ResolveRequest{Skill: ""})
	if err == nil || !strings.Contains(err.Error(), "empty skill") {
		t.Fatalf("expected empty skill error, got %v", err)
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := t.TempDir()
	if isGitRepo(dir) {
		t.Fatal("expected false for dir without .git")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if !isGitRepo(dir) {
		t.Fatal("expected true for dir with .git")
	}
}

func TestScanPathErrorMessage(t *testing.T) {
	e := &ScanPathError{Path: "skills", AvailableSkills: []string{"docx", "pdf"}}
	msg := e.Error()
	if !strings.Contains(msg, "skills") {
		t.Fatalf("expected 'skills' in error message, got %q", msg)
	}
	if !strings.Contains(msg, "2 skill(s)") {
		t.Fatalf("expected '2 skill(s)' in error message, got %q", msg)
	}
}
