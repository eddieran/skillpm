package context

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Profile describes the detected project context.
type Profile struct {
	ProjectType string             `toml:"project_type" json:"project_type"`
	Frameworks  []string           `toml:"frameworks" json:"frameworks"`
	TaskSignals []string           `toml:"task_signals" json:"task_signals"`
	BuildSystem string             `toml:"build_system" json:"build_system"`
	Languages   map[string]float64 `toml:"languages" json:"languages"`
	DetectedAt  time.Time          `toml:"detected_at" json:"detected_at"`
}

// SkillContextAffinity declares what contexts a skill is relevant to.
type SkillContextAffinity struct {
	ProjectTypes []string `yaml:"project_types" json:"project_types"`
	TaskSignals  []string `yaml:"task_signals" json:"task_signals"`
	Frameworks   []string `yaml:"frameworks" json:"frameworks"`
}

// Engine detects the current project context.
type Engine struct{}

// Detect examines a directory and returns a context profile.
func (e *Engine) Detect(dir string) (Profile, error) {
	p := Profile{
		Languages:  map[string]float64{},
		DetectedAt: time.Now().UTC(),
	}
	if dir == "" {
		return p, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return p, err
	}
	if !info.IsDir() {
		return p, nil
	}

	// Detect project type by marker files
	markers := []struct {
		file    string
		project string
		lang    string
		build   string
	}{
		{"go.mod", "go", "go", ""},
		{"Cargo.toml", "rust", "rust", "cargo"},
		{"pyproject.toml", "python", "python", ""},
		{"setup.py", "python", "python", ""},
		{"requirements.txt", "python", "python", ""},
		{"tsconfig.json", "typescript", "typescript", ""},
		{"package.json", "javascript", "javascript", "npm"},
		{"pom.xml", "java", "java", "maven"},
		{"build.gradle", "java", "java", "gradle"},
		{"Gemfile", "ruby", "ruby", "bundler"},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			if p.ProjectType == "" {
				p.ProjectType = m.project
			}
			p.Languages[m.lang] = 1.0
			if m.build != "" && p.BuildSystem == "" {
				p.BuildSystem = m.build
			}
		}
	}

	// Detect build system
	buildMarkers := []struct {
		file  string
		build string
	}{
		{"Makefile", "make"},
		{"CMakeLists.txt", "cmake"},
		{"Dockerfile", "docker"},
	}
	for _, m := range buildMarkers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			if p.BuildSystem == "" {
				p.BuildSystem = m.build
			}
		}
	}

	// Detect frameworks from dependency files
	p.Frameworks = detectFrameworks(dir)

	// Detect task signals from git
	p.TaskSignals = detectTaskSignals(dir)

	return p, nil
}

func detectFrameworks(dir string) []string {
	var frameworks []string

	// Go frameworks from go.mod
	if goMod := readLines(filepath.Join(dir, "go.mod")); len(goMod) > 0 {
		goFW := map[string]string{
			"github.com/spf13/cobra":   "cobra",
			"github.com/gin-gonic/gin": "gin",
			"github.com/gorilla/mux":   "gorilla",
			"github.com/labstack/echo": "echo",
			"github.com/gofiber/fiber": "fiber",
		}
		for _, line := range goMod {
			for dep, fw := range goFW {
				if strings.Contains(line, dep) {
					frameworks = append(frameworks, fw)
				}
			}
		}
	}

	// JS/TS frameworks from package.json
	if pkgJSON := readLines(filepath.Join(dir, "package.json")); len(pkgJSON) > 0 {
		jsFW := map[string]string{
			"\"react\"":   "react",
			"\"next\"":    "next",
			"\"vue\"":     "vue",
			"\"angular\"": "angular",
			"\"express\"": "express",
			"\"svelte\"":  "svelte",
		}
		for _, line := range pkgJSON {
			for dep, fw := range jsFW {
				if strings.Contains(line, dep) {
					frameworks = append(frameworks, fw)
				}
			}
		}
	}

	// Python frameworks from requirements.txt or pyproject.toml
	pyFiles := []string{"requirements.txt", "pyproject.toml"}
	for _, pyFile := range pyFiles {
		if lines := readLines(filepath.Join(dir, pyFile)); len(lines) > 0 {
			pyFW := map[string]string{
				"django":  "django",
				"flask":   "flask",
				"fastapi": "fastapi",
				"pytest":  "pytest",
			}
			for _, line := range lines {
				lower := strings.ToLower(line)
				for dep, fw := range pyFW {
					if strings.Contains(lower, dep) {
						frameworks = append(frameworks, fw)
					}
				}
			}
		}
	}

	return frameworks
}

func detectTaskSignals(dir string) []string {
	var signals []string

	// Try to read git HEAD for branch name
	headPath := filepath.Join(dir, ".git", "HEAD")
	blob, err := os.ReadFile(headPath)
	if err != nil {
		return signals
	}
	ref := strings.TrimSpace(string(blob))
	if !strings.HasPrefix(ref, "ref: refs/heads/") {
		return signals
	}
	branch := strings.TrimPrefix(ref, "ref: refs/heads/")

	branchSignals := map[string][]string{
		"feat":     {"feature"},
		"feature":  {"feature"},
		"fix":      {"debugging"},
		"bugfix":   {"debugging"},
		"hotfix":   {"debugging"},
		"test":     {"testing"},
		"refactor": {"refactor"},
		"chore":    {"maintenance"},
		"docs":     {"documentation"},
		"perf":     {"performance"},
	}

	lower := strings.ToLower(branch)
	for prefix, sigs := range branchSignals {
		if strings.HasPrefix(lower, prefix+"/") || strings.HasPrefix(lower, prefix+"-") || lower == prefix {
			signals = append(signals, sigs...)
		}
	}

	return signals
}

func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
