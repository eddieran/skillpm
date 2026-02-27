package bridge

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// MemorySignals holds structured signals extracted from Claude Code's MEMORY.md.
type MemorySignals struct {
	PackageManager string            `json:"package_manager,omitempty"`
	TestFramework  string            `json:"test_framework,omitempty"`
	Frameworks     []string          `json:"frameworks,omitempty"`
	Languages      []string          `json:"languages,omitempty"`
	Preferences    map[string]string `json:"preferences,omitempty"`
}

// claudeProjectDir resolves the Claude Code project memory directory for a given project path.
// Claude Code encodes project paths by replacing "/" with "-" and storing under
// ~/.claude/projects/<encoded>/memory/.
func claudeProjectDir(home, projectPath string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return ""
	}
	encoded := strings.ReplaceAll(abs, "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded, "memory")
}

// ReadMemorySignals reads and parses Claude Code's MEMORY.md for a project,
// extracting structured signals. Returns empty signals (not error) if file
// doesn't exist or can't be read.
func ReadMemorySignals(home, projectPath string) MemorySignals {
	dir := claudeProjectDir(home, projectPath)
	if dir == "" {
		return MemorySignals{}
	}
	return parseMemoryFile(filepath.Join(dir, "MEMORY.md"))
}

// parseMemoryFile reads a MEMORY.md and extracts signals via keyword matching.
func parseMemoryFile(path string) MemorySignals {
	f, err := os.Open(path)
	if err != nil {
		return MemorySignals{}
	}
	defer f.Close()

	sig := MemorySignals{
		Preferences: map[string]string{},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)

		// Package manager detection
		if sig.PackageManager == "" {
			sig.PackageManager = detectPackageManager(lower)
		}

		// Test framework detection
		if sig.TestFramework == "" {
			sig.TestFramework = detectTestFramework(lower)
		}

		// Framework detection
		for _, fw := range detectFrameworksFromLine(lower) {
			if !contains(sig.Frameworks, fw) {
				sig.Frameworks = append(sig.Frameworks, fw)
			}
		}

		// Language detection
		for _, lang := range detectLanguagesFromLine(lower) {
			if !contains(sig.Languages, lang) {
				sig.Languages = append(sig.Languages, lang)
			}
		}

		// Preference extraction (key: value or key = value patterns)
		if k, v := extractPreference(line); k != "" {
			sig.Preferences[k] = v
		}
	}

	// Clean up empty preferences map
	if len(sig.Preferences) == 0 {
		sig.Preferences = nil
	}

	return sig
}

// detectPackageManager checks a line for package manager mentions.
func detectPackageManager(lower string) string {
	// Order matters: more specific first
	pmPatterns := []struct {
		keywords []string
		name     string
	}{
		{[]string{"uses pnpm", "use pnpm", "pnpm install", "pnpm run", "prefer pnpm"}, "pnpm"},
		{[]string{"uses yarn", "use yarn", "yarn install", "yarn run", "prefer yarn"}, "yarn"},
		{[]string{"uses bun", "use bun", "bun install", "bun run", "prefer bun"}, "bun"},
		{[]string{"uses npm", "use npm", "npm install", "npm run", "prefer npm"}, "npm"},
		{[]string{"uses pip", "use pip", "pip install", "prefer pip"}, "pip"},
		{[]string{"uses poetry", "use poetry", "poetry install", "prefer poetry"}, "poetry"},
		{[]string{"uses uv", "use uv", "uv pip", "prefer uv"}, "uv"},
	}
	for _, pm := range pmPatterns {
		for _, kw := range pm.keywords {
			if strings.Contains(lower, kw) {
				return pm.name
			}
		}
	}
	return ""
}

// detectTestFramework checks a line for test framework mentions.
func detectTestFramework(lower string) string {
	tfPatterns := []struct {
		keywords []string
		name     string
	}{
		{[]string{"uses vitest", "use vitest", "vitest for", "prefer vitest"}, "vitest"},
		{[]string{"uses jest", "use jest", "jest for", "prefer jest"}, "jest"},
		{[]string{"uses pytest", "use pytest", "pytest for", "prefer pytest"}, "pytest"},
		{[]string{"uses mocha", "use mocha", "mocha for", "prefer mocha"}, "mocha"},
		{[]string{"uses rspec", "use rspec", "rspec for", "prefer rspec"}, "rspec"},
		{[]string{"go test", "testing.t"}, "go-test"},
	}
	for _, tf := range tfPatterns {
		for _, kw := range tf.keywords {
			if strings.Contains(lower, kw) {
				return tf.name
			}
		}
	}
	return ""
}

// detectFrameworksFromLine extracts framework names from a line.
func detectFrameworksFromLine(lower string) []string {
	fwPatterns := []struct {
		keywords []string
		name     string
	}{
		{[]string{"next.js", "nextjs", "next app"}, "next"},
		{[]string{"react"}, "react"},
		{[]string{"vue.js", "vuejs", "vue 3"}, "vue"},
		{[]string{"angular"}, "angular"},
		{[]string{"svelte"}, "svelte"},
		{[]string{"express.js", "expressjs", "express server"}, "express"},
		{[]string{"django"}, "django"},
		{[]string{"flask"}, "flask"},
		{[]string{"fastapi"}, "fastapi"},
		{[]string{"spring boot", "springboot"}, "spring"},
		{[]string{"rails", "ruby on rails"}, "rails"},
		{[]string{"tailwind", "tailwindcss"}, "tailwind"},
		{[]string{"gin framework", "gin-gonic"}, "gin"},
	}
	var result []string
	for _, fw := range fwPatterns {
		for _, kw := range fw.keywords {
			if strings.Contains(lower, kw) {
				result = append(result, fw.name)
				break
			}
		}
	}
	return result
}

// detectLanguagesFromLine extracts language mentions from a line.
func detectLanguagesFromLine(lower string) []string {
	// Only match when language appears in context suggesting it's used
	langPatterns := []struct {
		keywords []string
		name     string
	}{
		{[]string{"typescript", ".ts file", ".tsx"}, "typescript"},
		{[]string{"javascript", ".js file", ".jsx"}, "javascript"},
		{[]string{"golang", "go module", "go.mod"}, "go"},
		{[]string{"python", ".py file"}, "python"},
		{[]string{"rust", "cargo.toml"}, "rust"},
		{[]string{"ruby", "gemfile"}, "ruby"},
		{[]string{"java ", "java project"}, "java"},
		{[]string{"c# ", "csharp", ".net"}, "csharp"},
	}
	var result []string
	for _, lp := range langPatterns {
		for _, kw := range lp.keywords {
			if strings.Contains(lower, kw) {
				result = append(result, lp.name)
				break
			}
		}
	}
	return result
}

// extractPreference looks for "always X", "never X", "prefer X" patterns.
func extractPreference(line string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(line))

	prefixes := []string{"- always ", "- never ", "- prefer "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			key := strings.TrimSuffix(strings.TrimPrefix(prefix, "- "), " ")
			value := strings.TrimSpace(line[len(prefix):])
			if len(value) > 0 && len(value) < 200 {
				return key, value
			}
		}
	}
	return "", ""
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
