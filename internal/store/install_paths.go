package store

import (
	"os"
	"path/filepath"
	"strings"
)

// InstalledDirName returns the sanitized on-disk directory name for an
// installed skill artifact.
func InstalledDirName(skillRef, resolvedVersion string) string {
	return sanitizeInstalledName(skillRef) + "@" + sanitizeInstalledName(resolvedVersion)
}

// InstalledDirPath returns the absolute path to an installed skill artifact.
func InstalledDirPath(root, skillRef, resolvedVersion string) string {
	return filepath.Join(InstalledRoot(root), InstalledDirName(skillRef, resolvedVersion))
}

// InstalledDirPrefix returns the sanitized directory prefix for all installed
// versions of a skill ref.
func InstalledDirPrefix(skillRef string) string {
	return sanitizeInstalledName(skillRef) + "@"
}

// FindInstalledDir locates the on-disk installed directory for a skill ref.
func FindInstalledDir(root, skillRef string) string {
	entries, err := os.ReadDir(InstalledRoot(root))
	if err != nil {
		return ""
	}

	prefix := InstalledDirPrefix(skillRef)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			return filepath.Join(InstalledRoot(root), entry.Name())
		}
	}

	return ""
}

func sanitizeInstalledName(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "@", "_", " ", "-")
	out := r.Replace(v)
	if out == "" {
		return "unknown"
	}
	return out
}
