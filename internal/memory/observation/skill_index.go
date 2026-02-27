package observation

import (
	"strings"
)

// SkillIndex maps skill directory names to full SkillRef strings.
type SkillIndex struct {
	dirToRef map[string]string
}

// NewSkillIndex creates an index from installed skill refs.
// Each ref like "source/skill-name" maps "skill-name" → "source/skill-name".
func NewSkillIndex(skillRefs []string) *SkillIndex {
	idx := &SkillIndex{dirToRef: make(map[string]string, len(skillRefs))}
	for _, ref := range skillRefs {
		dirName := extractDirName(ref)
		if _, exists := idx.dirToRef[dirName]; !exists {
			idx.dirToRef[dirName] = ref
		}
	}
	return idx
}

// Resolve maps a directory name to its full SkillRef.
// Falls back to dirName itself if not found.
func (idx *SkillIndex) Resolve(dirName string) string {
	if idx == nil {
		return dirName
	}
	if ref, ok := idx.dirToRef[dirName]; ok {
		return ref
	}
	return dirName
}

// KnownDirNames returns the set of known skill directory names.
func (idx *SkillIndex) KnownDirNames() map[string]bool {
	if idx == nil {
		return nil
	}
	m := make(map[string]bool, len(idx.dirToRef))
	for k := range idx.dirToRef {
		m[k] = true
	}
	return m
}

// extractDirName gets the leaf name from a skill ref.
// "source/path/skill-name" → "skill-name"
func extractDirName(ref string) string {
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}
