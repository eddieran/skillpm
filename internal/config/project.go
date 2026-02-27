package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"skillpm/internal/fsutil"
)

const (
	projectDir          = ".skillpm"
	projectManifestFile = "skills.toml"
	projectLockFile     = "skills.lock"
	maxAncestorSearch   = 50
)

// DefaultProjectManifest returns an empty v1 project manifest.
func DefaultProjectManifest() ProjectManifest {
	return ProjectManifest{
		Version: SchemaVersion,
		Skills:  []ProjectSkillEntry{},
	}
}

// ProjectManifestPath returns the path to skills.toml for a project root.
func ProjectManifestPath(projectRoot string) string {
	return filepath.Join(projectRoot, projectDir, projectManifestFile)
}

// ProjectLockPath returns the path to skills.lock for a project root.
func ProjectLockPath(projectRoot string) string {
	return filepath.Join(projectRoot, projectDir, projectLockFile)
}

// ProjectStateRoot returns the .skillpm directory for a project root.
func ProjectStateRoot(projectRoot string) string {
	return filepath.Join(projectRoot, projectDir)
}

// FindProjectRoot walks up from startDir looking for .skillpm/skills.toml.
// Returns (projectRoot, true) if found, or ("", false) if not.
func FindProjectRoot(startDir string) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for i := 0; i < maxAncestorSearch; i++ {
		manifest := filepath.Join(dir, projectDir, projectManifestFile)
		if _, err := os.Stat(manifest); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return "", false
}

// ResolveScope determines the effective scope based on an explicit flag and CWD.
// If explicit is non-empty, it is validated and returned.
// Otherwise, auto-detection walks up from cwd looking for a project manifest.
func ResolveScope(explicit string, cwd string) (Scope, string, error) {
	switch Scope(explicit) {
	case ScopeGlobal:
		return ScopeGlobal, "", nil
	case ScopeProject:
		root, found := FindProjectRoot(cwd)
		if !found {
			return "", "", fmt.Errorf("PRJ_NO_MANIFEST: no project manifest found; run 'skillpm init' first")
		}
		return ScopeProject, root, nil
	case "":
		// auto-detect
		root, found := FindProjectRoot(cwd)
		if found {
			return ScopeProject, root, nil
		}
		return ScopeGlobal, "", nil
	default:
		return "", "", fmt.Errorf("PRJ_INVALID_SCOPE: invalid scope %q; use 'global' or 'project'", explicit)
	}
}

// LoadProjectManifest loads .skillpm/skills.toml from the given project root.
func LoadProjectManifest(projectRoot string) (ProjectManifest, error) {
	path := ProjectManifestPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectManifest{}, fmt.Errorf("PRJ_MANIFEST_READ: %w", err)
	}
	var m ProjectManifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return ProjectManifest{}, fmt.Errorf("PRJ_MANIFEST_PARSE: %w", err)
	}
	if m.Version == 0 {
		m.Version = SchemaVersion
	}
	if m.Skills == nil {
		m.Skills = []ProjectSkillEntry{}
	}
	return m, nil
}

// SaveProjectManifest writes .skillpm/skills.toml to the given project root.
func SaveProjectManifest(projectRoot string, m ProjectManifest) error {
	if m.Version == 0 {
		m.Version = SchemaVersion
	}
	dir := filepath.Join(projectRoot, projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("PRJ_MANIFEST_DIR: %w", err)
	}
	blob, err := toml.Marshal(m)
	if err != nil {
		return fmt.Errorf("PRJ_MANIFEST_ENCODE: %w", err)
	}
	if err := fsutil.AtomicWrite(ProjectManifestPath(projectRoot), blob, 0o644); err != nil {
		return fmt.Errorf("PRJ_MANIFEST_WRITE: %w", err)
	}
	return nil
}

// EnsureProjectLayout creates the .skillpm directory structure for a project.
func EnsureProjectLayout(projectRoot string) error {
	root := ProjectStateRoot(projectRoot)
	dirs := []string{
		root,
		filepath.Join(root, "installed"),
		filepath.Join(root, "staging"),
		filepath.Join(root, "snapshots"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("PRJ_LAYOUT: %w", err)
		}
	}
	return nil
}

// MergedSources returns global sources merged with project sources.
// Project sources with the same name override global sources.
func MergedSources(global Config, project ProjectManifest) []SourceConfig {
	if len(project.Sources) == 0 {
		return global.Sources
	}
	byName := make(map[string]SourceConfig, len(global.Sources))
	order := make([]string, 0, len(global.Sources)+len(project.Sources))
	for _, s := range global.Sources {
		byName[s.Name] = s
		order = append(order, s.Name)
	}
	for _, s := range project.Sources {
		if _, exists := byName[s.Name]; !exists {
			order = append(order, s.Name)
		}
		byName[s.Name] = s // project overrides global
	}
	merged := make([]SourceConfig, 0, len(order))
	for _, name := range order {
		merged = append(merged, byName[name])
	}
	return merged
}

// MergedAdapters returns global adapters merged with project adapter overrides.
// Project adapters with the same name override global adapters.
func MergedAdapters(global Config, project ProjectManifest) []AdapterConfig {
	if len(project.Adapters) == 0 {
		return global.Adapters
	}
	byName := make(map[string]AdapterConfig, len(global.Adapters))
	order := make([]string, 0, len(global.Adapters)+len(project.Adapters))
	for _, a := range global.Adapters {
		byName[a.Name] = a
		order = append(order, a.Name)
	}
	for _, a := range project.Adapters {
		if _, exists := byName[a.Name]; !exists {
			order = append(order, a.Name)
		}
		byName[a.Name] = a
	}
	merged := make([]AdapterConfig, 0, len(order))
	for _, name := range order {
		merged = append(merged, byName[name])
	}
	return merged
}

// UpsertManifestSkill adds or updates a skill entry in the manifest.
func UpsertManifestSkill(m *ProjectManifest, entry ProjectSkillEntry) {
	for i := range m.Skills {
		if m.Skills[i].Ref == entry.Ref {
			m.Skills[i] = entry
			return
		}
	}
	m.Skills = append(m.Skills, entry)
}

// RemoveManifestSkill removes a skill entry from the manifest by ref.
// Returns true if the skill was found and removed.
func RemoveManifestSkill(m *ProjectManifest, ref string) bool {
	for i := range m.Skills {
		if m.Skills[i].Ref == ref {
			m.Skills = append(m.Skills[:i], m.Skills[i+1:]...)
			return true
		}
	}
	return false
}

// InitProject creates a new project manifest at the given directory.
// Returns an error if a manifest already exists.
func InitProject(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("PRJ_INIT: %w", err)
	}
	path := ProjectManifestPath(abs)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("PRJ_INIT: project already initialized at %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("PRJ_INIT: %w", err)
	}
	if err := EnsureProjectLayout(abs); err != nil {
		return "", err
	}
	m := DefaultProjectManifest()
	if err := SaveProjectManifest(abs, m); err != nil {
		return "", err
	}
	return path, nil
}
