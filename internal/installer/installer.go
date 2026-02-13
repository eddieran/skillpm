package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillpm/internal/audit"
	"skillpm/internal/resolver"
	"skillpm/internal/security"
	"skillpm/internal/store"
)

type Service struct {
	Root     string
	Security *security.Engine
	Audit    *audit.Logger
}

func (s *Service) Install(_ context.Context, skills []resolver.ResolvedSkill, lockPath string, force bool) ([]store.InstalledSkill, error) {
	if err := store.EnsureLayout(s.Root); err != nil {
		return nil, err
	}
	if s.Audit != nil {
		_ = s.Audit.Log(audit.Event{Operation: "install", Phase: "start", Status: "ok", Message: fmt.Sprintf("skills=%d", len(skills))})
	}
	state, err := store.LoadState(s.Root)
	if err != nil {
		return nil, err
	}
	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	stage := filepath.Join(store.StagingRoot(s.Root), fmt.Sprintf("install-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return nil, fmt.Errorf("INS_STAGE_CREATE: %w", err)
	}
	defer os.RemoveAll(stage)

	installed := make([]store.InstalledSkill, 0, len(skills))
	committed := make([]string, 0, len(skills))
	backups := map[string]string{}
	rollback := func() {
		for _, final := range committed {
			_ = os.RemoveAll(final)
		}
		for final, backup := range backups {
			_ = os.RemoveAll(final)
			_ = os.Rename(backup, final)
		}
	}

	for _, item := range skills {
		if s.Security != nil {
			if err := s.Security.CheckTrustTier(item.TrustTier); err != nil {
				rollback()
				return nil, err
			}
			if err := s.Security.CheckModeration(security.Moderation{IsMalwareBlocked: item.IsMalwareBlocked, IsSuspicious: item.IsSuspicious}, force); err != nil {
				rollback()
				return nil, err
			}
		}

		safeName := safeEntryName(item.SkillRef) + "@" + safeEntryName(item.ResolvedVersion)
		stagedDir := filepath.Join(stage, safeName)
		finalDir := filepath.Join(store.InstalledRoot(s.Root), safeName)

		if err := os.MkdirAll(stagedDir, 0o755); err != nil {
			rollback()
			return nil, fmt.Errorf("INS_STAGE_WRITE: %w", err)
		}
		meta := fmt.Sprintf("skill_ref=%q\nsource=%q\nskill=%q\nresolved_version=%q\nchecksum=%q\nsource_ref=%q\n", item.SkillRef, item.Source, item.Skill, item.ResolvedVersion, item.Checksum, item.SourceRef)
		if err := os.WriteFile(filepath.Join(stagedDir, "metadata.toml"), []byte(meta), 0o644); err != nil {
			rollback()
			return nil, fmt.Errorf("INS_STAGE_WRITE: %w", err)
		}

		if _, err := os.Stat(finalDir); err == nil {
			backup := finalDir + ".bak-" + fmt.Sprintf("%d", time.Now().UnixNano())
			if err := os.Rename(finalDir, backup); err != nil {
				rollback()
				return nil, fmt.Errorf("INS_COMMIT_BACKUP: %w", err)
			}
			backups[finalDir] = backup
		}
		if os.Getenv("SKILLPM_TEST_FAIL_INSTALL_COMMIT") == "1" {
			rollback()
			return nil, fmt.Errorf("INS_TEST_FAIL_COMMIT: injected commit failure")
		}
		if err := os.Rename(stagedDir, finalDir); err != nil {
			rollback()
			return nil, fmt.Errorf("INS_COMMIT_ATOMIC: %w", err)
		}
		committed = append(committed, finalDir)

		rec := store.InstalledSkill{
			SkillRef:         item.SkillRef,
			Source:           item.Source,
			Skill:            item.Skill,
			ResolvedVersion:  item.ResolvedVersion,
			Checksum:         item.Checksum,
			SourceRef:        item.SourceRef,
			InstalledAt:      time.Now().UTC(),
			TrustTier:        item.TrustTier,
			IsSuspicious:     item.IsSuspicious,
			IsMalwareBlocked: item.IsMalwareBlocked,
		}
		installed = append(installed, rec)
		store.UpsertInstalled(&state, rec)

		lockRec := store.LockSkill{
			SkillRef:        item.SkillRef,
			ResolvedVersion: item.ResolvedVersion,
			Checksum:        item.Checksum,
			SourceRef:       item.SourceRef,
		}
		if item.ResolverHash != "" {
			lockRec.Metadata = map[string]string{"resolverHash": item.ResolverHash}
		}
		store.UpsertLock(&lock, lockRec)
	}

	if err := store.SaveState(s.Root, state); err != nil {
		rollback()
		return nil, fmt.Errorf("INS_STATE_SAVE: %w", err)
	}
	if lockPath != "" {
		if err := store.SaveLockfile(lockPath, lock); err != nil {
			rollback()
			return nil, fmt.Errorf("INS_LOCK_SAVE: %w", err)
		}
	}

	for _, backup := range backups {
		_ = os.RemoveAll(backup)
	}
	if s.Audit != nil {
		_ = s.Audit.Log(audit.Event{Operation: "install", Phase: "commit", Status: "ok", Message: fmt.Sprintf("installed=%d", len(installed))})
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].SkillRef < installed[j].SkillRef })
	return installed, nil
}

func (s *Service) Uninstall(_ context.Context, skillRefs []string, lockPath string) ([]string, error) {
	state, err := store.LoadState(s.Root)
	if err != nil {
		return nil, err
	}
	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	removed := make([]string, 0, len(skillRefs))
	for _, skillRef := range skillRefs {
		if !store.RemoveInstalled(&state, skillRef) {
			continue
		}
		store.RemoveLock(&lock, skillRef)
		prefix := safeEntryName(skillRef) + "@"
		entries, _ := os.ReadDir(store.InstalledRoot(s.Root))
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) {
				_ = os.RemoveAll(filepath.Join(store.InstalledRoot(s.Root), e.Name()))
			}
		}
		removed = append(removed, skillRef)
	}
	if err := store.SaveState(s.Root, state); err != nil {
		return nil, err
	}
	if lockPath != "" {
		if err := store.SaveLockfile(lockPath, lock); err != nil {
			return nil, err
		}
	}
	if s.Audit != nil {
		_ = s.Audit.Log(audit.Event{Operation: "uninstall", Phase: "commit", Status: "ok", Message: fmt.Sprintf("removed=%d", len(removed))})
	}
	sort.Strings(removed)
	return removed, nil
}

func safeEntryName(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "@", "_", " ", "-")
	out := r.Replace(v)
	if out == "" {
		return "unknown"
	}
	return out
}
