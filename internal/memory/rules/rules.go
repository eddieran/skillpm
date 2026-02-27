package rules

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillpm/internal/fsutil"
)

// Engine manages the lifecycle of skillpm-generated Claude Code rules.
type Engine struct {
	rulesDir string // absolute path to the skillpm rules subdirectory
}

// NewEngine creates a rules engine.
// home is the user home directory; projectRoot is empty for global scope.
func NewEngine(scope, projectRoot, home string) *Engine {
	var rulesDir string
	if scope == "project" && projectRoot != "" {
		rulesDir = filepath.Join(projectRoot, ".claude", "rules", "skillpm")
	} else {
		rulesDir = filepath.Join(home, ".claude", "rules", "skillpm")
	}
	return &Engine{rulesDir: rulesDir}
}

// RulesDir returns the absolute path to the managed rules directory.
func (e *Engine) RulesDir() string {
	return e.rulesDir
}

// Generate produces the rule file content for a single skill.
func (e *Engine) Generate(meta SkillRuleMeta) (filename string, content string) {
	filename = meta.SkillName + ".md"

	var b strings.Builder

	// YAML frontmatter with paths
	if len(meta.Paths) > 0 {
		b.WriteString("---\n")
		b.WriteString("paths:\n")
		for _, p := range meta.Paths {
			b.WriteString("  - \"")
			b.WriteString(p)
			b.WriteString("\"\n")
		}
		b.WriteString("---\n\n")
	}

	// Heading
	b.WriteString("# ")
	if meta.Name != "" {
		b.WriteString(meta.Name)
	} else {
		b.WriteString(meta.SkillName)
	}
	b.WriteString(" (managed by skillpm)\n\n")

	// Summary
	if meta.Summary != "" {
		b.WriteString(meta.Summary)
		b.WriteString("\n\n")
	}

	// Managed marker with ref and checksum
	checksum := shortChecksum(meta.SkillRef + meta.Summary + strings.Join(meta.Paths, ","))
	b.WriteString(fmt.Sprintf("%s ref=%s checksum=%s -->\n", fsutil.ManagedMarkerPrefix, meta.SkillRef, checksum))

	return filename, b.String()
}

// Sync writes rules for the given skills and removes stale ones.
// It only creates/modifies/deletes files with the skillpm:managed marker.
func (e *Engine) Sync(metas []SkillRuleMeta) error {
	// 1. Build target state
	target := make(map[string]string) // filename → content
	for _, meta := range metas {
		if len(meta.Paths) == 0 && meta.Summary == "" {
			continue
		}
		fname, content := e.Generate(meta)
		target[fname] = content
	}

	// 2. Ensure directory exists if we have targets
	if len(target) > 0 {
		if err := os.MkdirAll(e.rulesDir, 0o755); err != nil {
			return fmt.Errorf("RULES_MKDIR: %w", err)
		}
	}

	// 3. Read existing files
	existing, err := e.listFiles()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("RULES_LIST: %w", err)
	}

	// 4. Process existing files
	for _, fname := range existing {
		path := filepath.Join(e.rulesDir, fname)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		content := string(data)

		if !fsutil.IsManagedFile([]byte(content)) {
			// Not our file — skip
			continue
		}

		targetContent, inTarget := target[fname]
		if !inTarget {
			// Managed file no longer needed — remove
			_ = os.Remove(path)
			continue
		}

		if content != targetContent {
			// Content changed — overwrite atomically
			if writeErr := fsutil.AtomicWrite(path, []byte(targetContent), 0o644); writeErr != nil {
				return fmt.Errorf("RULES_WRITE: %s: %w", fname, writeErr)
			}
		}

		// Remove from target so we don't write it again below
		delete(target, fname)
	}

	// 5. Write new files
	for fname, content := range target {
		path := filepath.Join(e.rulesDir, fname)
		if writeErr := fsutil.AtomicWrite(path, []byte(content), 0o644); writeErr != nil {
			return fmt.Errorf("RULES_WRITE: %s: %w", fname, writeErr)
		}
	}

	// 6. Clean up empty directory
	e.removeEmptyDir()

	return nil
}

// Cleanup removes all skillpm-managed rule files.
func (e *Engine) Cleanup() error {
	files, err := e.listFiles()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, fname := range files {
		path := filepath.Join(e.rulesDir, fname)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		if fsutil.IsManagedFile(data) {
			_ = os.Remove(path)
		}
	}

	e.removeEmptyDir()
	return nil
}

// ListManaged returns filenames of all skillpm-managed rule files.
func (e *Engine) ListManaged() ([]string, error) {
	files, err := e.listFiles()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var managed []string
	for _, fname := range files {
		path := filepath.Join(e.rulesDir, fname)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		if fsutil.IsManagedFile(data) {
			managed = append(managed, fname)
		}
	}
	return managed, nil
}

// listFiles returns .md filenames in the rules directory.
func (e *Engine) listFiles() ([]string, error) {
	entries, err := os.ReadDir(e.rulesDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// removeEmptyDir removes the rules directory if it is empty.
func (e *Engine) removeEmptyDir() {
	entries, err := os.ReadDir(e.rulesDir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(e.rulesDir)
	}
}

// shortChecksum returns a short SHA256 hex prefix for content.
func shortChecksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}
