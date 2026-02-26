package context

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// engine is a shared zero-value Engine used across all tests.
var engine = &Engine{}

// writeFile is a helper that creates a file with the given content inside dir.
// It calls t.Fatal on any write error so callers need not check the return value.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

// sortedStrings returns a sorted copy of ss so order-independent comparisons are
// straightforward.
func sortedStrings(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

// containsString reports whether target is present in ss.
func containsString(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// ---- tests -------------------------------------------------------------------

// TestDetectEmptyDir verifies that an empty directory returns a zero-value
// Profile whose Languages map is initialised and DetectedAt is set.
func TestDetectEmptyDir(t *testing.T) {
	dir := t.TempDir()

	before := time.Now().UTC()
	p, err := engine.Detect(dir)
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "" {
		t.Errorf("ProjectType = %q; want empty", p.ProjectType)
	}
	if p.BuildSystem != "" {
		t.Errorf("BuildSystem = %q; want empty", p.BuildSystem)
	}
	if len(p.Frameworks) != 0 {
		t.Errorf("Frameworks = %v; want empty", p.Frameworks)
	}
	if len(p.TaskSignals) != 0 {
		t.Errorf("TaskSignals = %v; want empty", p.TaskSignals)
	}
	if p.Languages == nil {
		t.Error("Languages map is nil; want initialised map")
	}
	if p.DetectedAt.IsZero() {
		t.Error("DetectedAt is zero; want a timestamp")
	}
	if p.DetectedAt.Before(before) || p.DetectedAt.After(after) {
		t.Errorf("DetectedAt %v is outside [%v, %v]", p.DetectedAt, before, after)
	}
}

// TestDetectGoProject verifies that a directory containing a go.mod that lists
// the cobra dependency is detected as a Go project with the "cobra" framework.
func TestDetectGoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/app

go 1.21

require (
	github.com/spf13/cobra v1.8.0
)
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "go" {
		t.Errorf("ProjectType = %q; want %q", p.ProjectType, "go")
	}
	if p.Languages["go"] != 1.0 {
		t.Errorf("Languages[go] = %v; want 1.0", p.Languages["go"])
	}
	if !containsString(p.Frameworks, "cobra") {
		t.Errorf("Frameworks = %v; want to contain %q", p.Frameworks, "cobra")
	}
}

// TestDetectRustProject verifies that a Cargo.toml marker sets project=rust,
// lang=rust and build=cargo.
func TestDetectRustProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "my-crate"
version = "0.1.0"
edition = "2021"
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "rust" {
		t.Errorf("ProjectType = %q; want %q", p.ProjectType, "rust")
	}
	if p.Languages["rust"] != 1.0 {
		t.Errorf("Languages[rust] = %v; want 1.0", p.Languages["rust"])
	}
	if p.BuildSystem != "cargo" {
		t.Errorf("BuildSystem = %q; want %q", p.BuildSystem, "cargo")
	}
}

// TestDetectPythonProject verifies that requirements.txt containing "flask"
// results in project=python and frameworks containing "flask".
func TestDetectPythonProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", `flask==3.0.0
requests>=2.31.0
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "python" {
		t.Errorf("ProjectType = %q; want %q", p.ProjectType, "python")
	}
	if p.Languages["python"] != 1.0 {
		t.Errorf("Languages[python] = %v; want 1.0", p.Languages["python"])
	}
	if !containsString(p.Frameworks, "flask") {
		t.Errorf("Frameworks = %v; want to contain %q", p.Frameworks, "flask")
	}
}

// TestDetectJSProject verifies that package.json containing the "react" key
// results in project=javascript and frameworks containing "react".
func TestDetectJSProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
  "name": "my-app",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  }
}
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "javascript" {
		t.Errorf("ProjectType = %q; want %q", p.ProjectType, "javascript")
	}
	if p.Languages["javascript"] != 1.0 {
		t.Errorf("Languages[javascript] = %v; want 1.0", p.Languages["javascript"])
	}
	if !containsString(p.Frameworks, "react") {
		t.Errorf("Frameworks = %v; want to contain %q", p.Frameworks, "react")
	}
}

// TestDetectBuildSystem verifies that a lone Makefile — with no language marker
// — sets BuildSystem to "make".
func TestDetectBuildSystem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Makefile", `all:
	echo "build"
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.BuildSystem != "make" {
		t.Errorf("BuildSystem = %q; want %q", p.BuildSystem, "make")
	}
	// No language markers present, so ProjectType should be empty.
	if p.ProjectType != "" {
		t.Errorf("ProjectType = %q; want empty when only Makefile is present", p.ProjectType)
	}
}

// TestDetectMultipleLanguages verifies that a directory with both go.mod and a
// Makefile yields project=go and build=make.  The Makefile build marker must
// not override the "cargo"/"npm" build set by language markers.
//
// In this case go.mod carries no build string so the Makefile wins.
func TestDetectMultipleLanguages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/multi

go 1.21
`)
	writeFile(t, dir, "Makefile", `all:
	go build ./...
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "go" {
		t.Errorf("ProjectType = %q; want %q", p.ProjectType, "go")
	}
	if p.BuildSystem != "make" {
		t.Errorf("BuildSystem = %q; want %q", p.BuildSystem, "make")
	}
	if p.Languages["go"] != 1.0 {
		t.Errorf("Languages[go] = %v; want 1.0", p.Languages["go"])
	}
}

// TestDetectTaskSignalFromBranch verifies that a .git/HEAD file referencing a
// "feat/…" branch results in the "feature" task signal.
func TestDetectTaskSignalFromBranch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/feat/new-thing\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if !containsString(p.TaskSignals, "feature") {
		t.Errorf("TaskSignals = %v; want to contain %q", p.TaskSignals, "feature")
	}
}

// TestDetectFixBranch verifies that a "fix/…" branch maps to the "debugging"
// task signal.
func TestDetectFixBranch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/fix/nasty-bug\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if !containsString(p.TaskSignals, "debugging") {
		t.Errorf("TaskSignals = %v; want to contain %q", p.TaskSignals, "debugging")
	}
}

// TestDetectNoDir verifies that a nonexistent path returns a non-nil error and
// an otherwise zero-value Profile (with DetectedAt set and Languages initialised).
func TestDetectNoDir(t *testing.T) {
	_, err := engine.Detect("/this/path/does/not/exist/ever")
	if err == nil {
		t.Fatal("Detect: expected an error for nonexistent directory, got nil")
	}
}

// TestDetectEmptyString verifies that an empty string directory returns an empty
// Profile and no error — the engine cannot stat "" so it returns early.
func TestDetectEmptyString(t *testing.T) {
	p, err := engine.Detect("")
	if err != nil {
		t.Fatalf("Detect: unexpected error for empty string: %v", err)
	}
	if p.ProjectType != "" {
		t.Errorf("ProjectType = %q; want empty", p.ProjectType)
	}
	if p.BuildSystem != "" {
		t.Errorf("BuildSystem = %q; want empty", p.BuildSystem)
	}
	if len(p.Frameworks) != 0 {
		t.Errorf("Frameworks = %v; want empty", p.Frameworks)
	}
	if len(p.TaskSignals) != 0 {
		t.Errorf("TaskSignals = %v; want empty", p.TaskSignals)
	}
}

// ---- table-driven signal tests ----------------------------------------------

// TestDetectBranchSignals covers all branch prefix → signal mappings in a
// single table-driven sub-test run so that adding a new mapping in the
// production code automatically surfaces a missing test.
func TestDetectBranchSignals(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"feat/login", "feature"},
		{"feature/dashboard", "feature"},
		{"fix/crash", "debugging"},
		{"bugfix/login", "debugging"},
		{"hotfix/prod", "debugging"},
		{"test/integration", "testing"},
		{"refactor/cleanup", "refactor"},
		{"chore/deps", "maintenance"},
		{"docs/readme", "documentation"},
		{"perf/query", "performance"},
	}

	for _, tc := range tests {
		tc := tc // capture range var
		t.Run(tc.branch, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, ".git/HEAD", "ref: refs/heads/"+tc.branch+"\n")

			p, err := engine.Detect(dir)
			if err != nil {
				t.Fatalf("Detect: unexpected error: %v", err)
			}
			if !containsString(p.TaskSignals, tc.want) {
				t.Errorf("branch %q: TaskSignals = %v; want to contain %q",
					tc.branch, p.TaskSignals, tc.want)
			}
		})
	}
}

// TestDetectBranchDetachedHEAD verifies that a detached HEAD (no "ref: …"
// prefix) produces no task signals without returning an error.
func TestDetectBranchDetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/HEAD", "abc123def456abc123def456abc123def456abc1\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if len(p.TaskSignals) != 0 {
		t.Errorf("TaskSignals = %v; want empty for detached HEAD", p.TaskSignals)
	}
}

// TestDetectGoFrameworks exercises every Go framework the engine knows about.
func TestDetectGoFrameworks(t *testing.T) {
	tests := []struct {
		dep string
		fw  string
	}{
		{"github.com/spf13/cobra", "cobra"},
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/gorilla/mux", "gorilla"},
		{"github.com/labstack/echo", "echo"},
		{"github.com/gofiber/fiber", "fiber"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.fw, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.21\n\nrequire "+tc.dep+" v1.0.0\n")

			p, err := engine.Detect(dir)
			if err != nil {
				t.Fatalf("Detect: unexpected error: %v", err)
			}
			if !containsString(p.Frameworks, tc.fw) {
				t.Errorf("dep %q: Frameworks = %v; want to contain %q", tc.dep, p.Frameworks, tc.fw)
			}
		})
	}
}

// TestDetectJSFrameworks exercises every JS framework the engine knows about.
func TestDetectJSFrameworks(t *testing.T) {
	tests := []struct {
		key string
		fw  string
	}{
		{`"react"`, "react"},
		{`"next"`, "next"},
		{`"vue"`, "vue"},
		{`"angular"`, "angular"},
		{`"express"`, "express"},
		{`"svelte"`, "svelte"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.fw, func(t *testing.T) {
			dir := t.TempDir()
			content := "{\n  \"dependencies\": {\n    " + tc.key + ": \"1.0.0\"\n  }\n}\n"
			writeFile(t, dir, "package.json", content)

			p, err := engine.Detect(dir)
			if err != nil {
				t.Fatalf("Detect: unexpected error: %v", err)
			}
			if !containsString(p.Frameworks, tc.fw) {
				t.Errorf("key %q: Frameworks = %v; want to contain %q", tc.key, p.Frameworks, tc.fw)
			}
		})
	}
}

// TestDetectPythonFrameworks exercises every Python framework the engine
// recognises from requirements.txt.
func TestDetectPythonFrameworks(t *testing.T) {
	tests := []struct {
		line string
		fw   string
	}{
		{"django>=4.0", "django"},
		{"Flask==3.0.0", "flask"},
		{"fastapi[all]>=0.100.0", "fastapi"},
		{"pytest>=7.4.0", "pytest"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.fw, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "requirements.txt", tc.line+"\n")

			p, err := engine.Detect(dir)
			if err != nil {
				t.Fatalf("Detect: unexpected error: %v", err)
			}
			if !containsString(p.Frameworks, tc.fw) {
				t.Errorf("line %q: Frameworks = %v; want to contain %q", tc.line, p.Frameworks, tc.fw)
			}
		})
	}
}

// TestDetectProjectTypePrecedence verifies that the first-matched language
// marker wins for ProjectType.  go.mod appears before package.json in the
// marker list, so a dir with both should be detected as "go".
func TestDetectProjectTypePrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.21\n")
	writeFile(t, dir, "package.json", `{"name": "app", "dependencies": {}}`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.ProjectType != "go" {
		t.Errorf("ProjectType = %q; want %q (first marker wins)", p.ProjectType, "go")
	}
	// Both languages should be recorded.
	if p.Languages["go"] != 1.0 {
		t.Errorf("Languages[go] = %v; want 1.0", p.Languages["go"])
	}
	if p.Languages["javascript"] != 1.0 {
		t.Errorf("Languages[javascript] = %v; want 1.0", p.Languages["javascript"])
	}
}

// TestDetectBuildSystemPrecedence verifies that a language-marker build value
// (e.g. "cargo" from Cargo.toml) takes precedence over a generic build marker
// (e.g. "make" from Makefile) because language markers are processed first.
func TestDetectBuildSystemPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2021\"\n")
	writeFile(t, dir, "Makefile", "all:\n\tcargo build\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	// Cargo.toml is processed before Makefile, so build should be "cargo".
	if p.BuildSystem != "cargo" {
		t.Errorf("BuildSystem = %q; want %q (language marker wins over build marker)", p.BuildSystem, "cargo")
	}
}

// TestDetectDockerfileOnly verifies that a lone Dockerfile sets build=docker.
func TestDetectDockerfileOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM alpine:3.18\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.BuildSystem != "docker" {
		t.Errorf("BuildSystem = %q; want %q", p.BuildSystem, "docker")
	}
}

// TestDetectCMakeOnly verifies that a lone CMakeLists.txt sets build=cmake.
func TestDetectCMakeOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CMakeLists.txt", "cmake_minimum_required(VERSION 3.20)\n")

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	if p.BuildSystem != "cmake" {
		t.Errorf("BuildSystem = %q; want %q", p.BuildSystem, "cmake")
	}
}

// TestDetectLanguagesMapAlwaysInitialised confirms that Languages is never nil,
// even for paths where Detect returns early.
func TestDetectLanguagesMapAlwaysInitialised(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"empty string", ""},
		{"temp dir", t.TempDir()},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p, _ := engine.Detect(tc.dir)
			if p.Languages == nil {
				t.Errorf("dir=%q: Languages map is nil", tc.dir)
			}
		})
	}
}

// TestDetectFrameworksMultiple verifies that multiple frameworks in the same
// file are all captured.
func TestDetectFrameworksMultiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
  "dependencies": {
    "react": "^18.0.0",
    "express": "^4.18.0"
  }
}
`)

	p, err := engine.Detect(dir)
	if err != nil {
		t.Fatalf("Detect: unexpected error: %v", err)
	}
	sorted := sortedStrings(p.Frameworks)
	// Both react and express should appear.
	if !containsString(sorted, "react") {
		t.Errorf("Frameworks %v missing %q", sorted, "react")
	}
	if !containsString(sorted, "express") {
		t.Errorf("Frameworks %v missing %q", sorted, "express")
	}
}

// ---- benchmarks -------------------------------------------------------------

// BenchmarkDetect measures the cost of a single Detect call against a realistic
// directory that contains go.mod, a Makefile, and a .git/HEAD file.
func BenchmarkDetect(b *testing.B) {
	dir := b.TempDir()
	// Write fixture files once before the timed loop.
	goMod := `module example.com/bench

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/gin-gonic/gin v1.9.1
)
`
	makefile := `build:
	go build ./...

test:
	go test ./...
`
	head := "ref: refs/heads/feat/benchmark-branch\n"

	for name, content := range map[string]string{
		"go.mod":    goMod,
		"Makefile":  makefile,
		".git/HEAD": head,
	} {
		path := filepath.Join(dir, name)
		if name == ".git/HEAD" {
			if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
				b.Fatalf("MkdirAll .git: %v", err)
			}
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Detect(dir); err != nil {
			b.Fatalf("Detect: %v", err)
		}
	}
}
