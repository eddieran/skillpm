package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeProjectDir(t *testing.T) {
	home := "/Users/testuser"
	dir := claudeProjectDir(home, "/Users/testuser/project/myapp")
	want := filepath.Join(home, ".claude", "projects", "-Users-testuser-project-myapp", "memory")
	if dir != want {
		t.Errorf("claudeProjectDir = %q, want %q", dir, want)
	}
}

func TestClaudeProjectDir_RootPath(t *testing.T) {
	home := "/home/user"
	dir := claudeProjectDir(home, "/")
	want := filepath.Join(home, ".claude", "projects", "-", "memory")
	if dir != want {
		t.Errorf("claudeProjectDir(/) = %q, want %q", dir, want)
	}
}

func TestReadMemorySignals_FileNotExist(t *testing.T) {
	sig := ReadMemorySignals("/nonexistent", "/nonexistent/project")
	if sig.PackageManager != "" || sig.TestFramework != "" {
		t.Errorf("expected empty signals for missing file, got %+v", sig)
	}
}

func TestParseMemoryFile_PackageManager(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"This project uses pnpm for package management.", "pnpm"},
		{"Always use yarn for installs.", "yarn"},
		{"We prefer bun as our runtime.", "bun"},
		{"Run npm install to set up.", "npm"},
		{"Uses poetry for Python deps.", "poetry"},
		{"Use uv pip for fast installs.", "uv"},
		{"No package manager mentioned.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "MEMORY.md")
			os.WriteFile(path, []byte(tt.content), 0o644)

			sig := parseMemoryFile(path)
			if sig.PackageManager != tt.want {
				t.Errorf("PackageManager = %q, want %q", sig.PackageManager, tt.want)
			}
		})
	}
}

func TestParseMemoryFile_TestFramework(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"Uses vitest for unit tests.", "vitest"},
		{"Use jest for testing components.", "jest"},
		{"Uses pytest for Python tests.", "pytest"},
		{"Run go test to verify.", "go-test"},
		{"No test framework mentioned.", ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "MEMORY.md")
			os.WriteFile(path, []byte(tt.content), 0o644)

			sig := parseMemoryFile(path)
			if sig.TestFramework != tt.want {
				t.Errorf("TestFramework = %q, want %q", sig.TestFramework, tt.want)
			}
		})
	}
}

func TestParseMemoryFile_Frameworks(t *testing.T) {
	content := `# Project Notes
This is a Next.js project using React and Tailwind CSS.
The backend uses Express.js for the API.`

	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(content), 0o644)

	sig := parseMemoryFile(path)
	wantFW := []string{"next", "react", "tailwind", "express"}
	for _, fw := range wantFW {
		if !contains(sig.Frameworks, fw) {
			t.Errorf("Frameworks missing %q, got %v", fw, sig.Frameworks)
		}
	}
}

func TestParseMemoryFile_Languages(t *testing.T) {
	content := `# Tech Stack
Built with TypeScript and Python for scripting.
Uses Rust for the native module.`

	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(content), 0o644)

	sig := parseMemoryFile(path)
	wantLangs := []string{"typescript", "python", "rust"}
	for _, lang := range wantLangs {
		if !contains(sig.Languages, lang) {
			t.Errorf("Languages missing %q, got %v", lang, sig.Languages)
		}
	}
}

func TestParseMemoryFile_NoDuplicates(t *testing.T) {
	content := `React is great.
We love React.
React components are key.`

	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(content), 0o644)

	sig := parseMemoryFile(path)
	count := 0
	for _, fw := range sig.Frameworks {
		if fw == "react" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 'react', got %d in %v", count, sig.Frameworks)
	}
}

func TestParseMemoryFile_FullExample(t *testing.T) {
	content := `# Project Memory

## Tech
- This project uses pnpm
- Uses vitest for testing
- Built with Next.js and TypeScript
- Tailwind for styling

## Preferences
- Always use strict TypeScript
- Never use any type
- Prefer functional components`

	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(content), 0o644)

	sig := parseMemoryFile(path)
	if sig.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want pnpm", sig.PackageManager)
	}
	if sig.TestFramework != "vitest" {
		t.Errorf("TestFramework = %q, want vitest", sig.TestFramework)
	}
	if !contains(sig.Frameworks, "next") {
		t.Errorf("missing framework 'next'")
	}
	if !contains(sig.Frameworks, "tailwind") {
		t.Errorf("missing framework 'tailwind'")
	}
	if !contains(sig.Languages, "typescript") {
		t.Errorf("missing language 'typescript'")
	}
}

func TestParseMemoryFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(""), 0o644)

	sig := parseMemoryFile(path)
	if sig.PackageManager != "" || sig.TestFramework != "" {
		t.Errorf("expected empty signals for empty file")
	}
	if sig.Preferences != nil {
		t.Errorf("expected nil preferences for empty file")
	}
}

func TestReadMemorySignals_Integration(t *testing.T) {
	home := t.TempDir()
	projectPath := "/test/myproject"

	// Create the Claude project memory directory
	memDir := claudeProjectDir(home, projectPath)
	os.MkdirAll(memDir, 0o755)

	// Write a MEMORY.md
	content := "This project uses yarn and jest for testing."
	os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(content), 0o644)

	sig := ReadMemorySignals(home, projectPath)
	if sig.PackageManager != "yarn" {
		t.Errorf("PackageManager = %q, want yarn", sig.PackageManager)
	}
	if sig.TestFramework != "jest" {
		t.Errorf("TestFramework = %q, want jest", sig.TestFramework)
	}
}

func TestExtractPreference(t *testing.T) {
	tests := []struct {
		line    string
		wantKey string
		wantVal string
	}{
		{"- Always use strict mode", "always", "use strict mode"},
		{"- Never use var", "never", "use var"},
		{"- Prefer const over let", "prefer", "const over let"},
		{"Regular line", "", ""},
		{"- Something else", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			k, v := extractPreference(tt.line)
			if k != tt.wantKey {
				t.Errorf("key = %q, want %q", k, tt.wantKey)
			}
			if v != tt.wantVal {
				t.Errorf("val = %q, want %q", v, tt.wantVal)
			}
		})
	}
}

func TestDetectPackageManager_FirstMatchWins(t *testing.T) {
	// When multiple mentions, first line wins
	content := `Use pnpm for speed.
Also npm is available.`

	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(path, []byte(content), 0o644)

	sig := parseMemoryFile(path)
	if sig.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want pnpm (first match)", sig.PackageManager)
	}
}
