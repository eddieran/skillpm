package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

const LockVersion = 1

func LoadLockfile(path string) (Lockfile, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Lockfile{Version: LockVersion}, nil
		}
		return Lockfile{}, err
	}
	var lock Lockfile
	if err := toml.Unmarshal(blob, &lock); err != nil {
		return Lockfile{}, fmt.Errorf("DOC_LOCK_PARSE: %w", err)
	}
	if lock.Version == 0 {
		lock.Version = LockVersion
	}
	if lock.Version != LockVersion {
		return Lockfile{}, fmt.Errorf("DOC_LOCK_VERSION: unsupported version %d", lock.Version)
	}
	seen := map[string]struct{}{}
	for _, s := range lock.Skills {
		if s.SkillRef == "" {
			return Lockfile{}, fmt.Errorf("DOC_LOCK_SCHEMA: missing skillRef")
		}
		if _, ok := seen[s.SkillRef]; ok {
			return Lockfile{}, fmt.Errorf("DOC_LOCK_SCHEMA: duplicate skillRef %q", s.SkillRef)
		}
		seen[s.SkillRef] = struct{}{}
		if s.ResolvedVersion == "" || s.Checksum == "" || s.SourceRef == "" {
			return Lockfile{}, fmt.Errorf("DOC_LOCK_SCHEMA: incomplete record for %q", s.SkillRef)
		}
	}
	return lock, nil
}

func SaveLockfile(path string, lock Lockfile) error {
	lock.Version = LockVersion
	sort.Slice(lock.Skills, func(i, j int) bool {
		return lock.Skills[i].SkillRef < lock.Skills[j].SkillRef
	})
	blob, err := toml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("DOC_LOCK_ENCODE: %w", err)
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func UpsertLock(lock *Lockfile, rec LockSkill) {
	for i := range lock.Skills {
		if lock.Skills[i].SkillRef == rec.SkillRef {
			lock.Skills[i] = rec
			return
		}
	}
	lock.Skills = append(lock.Skills, rec)
}

func RemoveLock(lock *Lockfile, skillRef string) bool {
	for i := range lock.Skills {
		if lock.Skills[i].SkillRef == skillRef {
			lock.Skills = append(lock.Skills[:i], lock.Skills[i+1:]...)
			return true
		}
	}
	return false
}
