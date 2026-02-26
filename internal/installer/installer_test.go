package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/resolver"
	"skillpm/internal/security"
	"skillpm/internal/store"
)

func TestInstallWritesStateAndLockfile(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	svc := &Service{Root: root, Security: security.New(config.SecurityConfig{Profile: "strict"})}
	items := []resolver.ResolvedSkill{{
		SkillRef:        "anthropic/pdf",
		Source:          "anthropic",
		Skill:           "pdf",
		ResolvedVersion: "1.0.0",
		Checksum:        "sha256:abc",
		Content:         "# pdf\nA skill",
		SourceRef:       "https://github.com/anthropics/skills.git@abcd",
		TrustTier:       "review",
	}}
	installed, err := svc.Install(context.Background(), items, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected one install record")
	}

	st, err := store.LoadState(root)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if len(st.Installed) != 1 {
		t.Fatalf("expected installed state entry")
	}
	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if len(lock.Skills) != 1 || lock.Skills[0].SkillRef != "anthropic/pdf" {
		t.Fatalf("unexpected lockfile: %+v", lock.Skills)
	}

	entries, err := os.ReadDir(store.InstalledRoot(root))
	if err != nil {
		t.Fatalf("read installed dir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one installed artifact dir")
	}
	skillMdPath := filepath.Join(store.InstalledRoot(root), entries[0].Name(), "SKILL.md")
	if _, err := os.Stat(skillMdPath); err != nil {
		t.Fatalf("expected SKILL.md in installed dir: %v", err)
	}
}

func TestInstallRejectsEmptyContent(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	svc := &Service{Root: root, Security: security.New(config.SecurityConfig{Profile: "strict"})}
	items := []resolver.ResolvedSkill{{
		SkillRef:        "anthropic/empty",
		Source:          "anthropic",
		Skill:           "empty",
		ResolvedVersion: "1.0.0",
		Checksum:        "sha256:abc",
		Content:         "",
		SourceRef:       "https://github.com/anthropics/skills.git@abcd",
		TrustTier:       "review",
	}}
	_, err := svc.Install(context.Background(), items, lockPath, false)
	if err == nil {
		t.Fatalf("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "INS_EMPTY_CONTENT") {
		t.Fatalf("expected INS_EMPTY_CONTENT error, got %v", err)
	}
}

func TestInstallWritesAncillaryFiles(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	svc := &Service{Root: root, Security: security.New(config.SecurityConfig{Profile: "strict"})}
	items := []resolver.ResolvedSkill{{
		SkillRef:        "anthropic/docx",
		Source:          "anthropic",
		Skill:           "docx",
		ResolvedVersion: "1.0.0",
		Checksum:        "sha256:abc",
		Content:         "# docx\nDocument creation skill",
		Files: map[string]string{
			"tools/helper.sh": "#!/bin/bash\necho helper",
			"references.md":   "Some references",
		},
		SourceRef: "https://github.com/anthropics/skills.git@abcd",
		TrustTier: "review",
	}}
	installed, err := svc.Install(context.Background(), items, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected one install record")
	}

	entries, err := os.ReadDir(store.InstalledRoot(root))
	if err != nil {
		t.Fatalf("read installed dir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one installed artifact dir")
	}
	artifactDir := filepath.Join(store.InstalledRoot(root), entries[0].Name())

	// Check SKILL.md
	skillMd, err := os.ReadFile(filepath.Join(artifactDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md failed: %v", err)
	}
	if string(skillMd) != "# docx\nDocument creation skill" {
		t.Fatalf("unexpected SKILL.md content: %q", string(skillMd))
	}

	// Check ancillary files
	helperSh, err := os.ReadFile(filepath.Join(artifactDir, "tools", "helper.sh"))
	if err != nil {
		t.Fatalf("read tools/helper.sh failed: %v", err)
	}
	if string(helperSh) != "#!/bin/bash\necho helper" {
		t.Fatalf("unexpected tools/helper.sh content: %q", string(helperSh))
	}

	refMd, err := os.ReadFile(filepath.Join(artifactDir, "references.md"))
	if err != nil {
		t.Fatalf("read references.md failed: %v", err)
	}
	if string(refMd) != "Some references" {
		t.Fatalf("unexpected references.md content: %q", string(refMd))
	}
}

func TestInstallUpgradeRemovesOldVersionDir(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	svc := &Service{Root: root, Security: security.New(config.SecurityConfig{Profile: "strict"})}

	// Install v1
	items := []resolver.ResolvedSkill{{
		SkillRef:        "anthropic/docx",
		Source:          "anthropic",
		Skill:           "docx",
		ResolvedVersion: "1.0.0",
		Checksum:        "sha256:aaa",
		Content:         "# docx v1",
		SourceRef:       "https://github.com/anthropics/skills.git@aaa",
		TrustTier:       "review",
	}}
	_, err := svc.Install(context.Background(), items, lockPath, false)
	if err != nil {
		t.Fatalf("install v1 failed: %v", err)
	}
	entries, _ := os.ReadDir(store.InstalledRoot(root))
	if len(entries) != 1 {
		t.Fatalf("expected 1 dir after v1 install, got %d", len(entries))
	}
	oldDir := entries[0].Name()

	// Install v2 (upgrade)
	items[0].ResolvedVersion = "2.0.0"
	items[0].Checksum = "sha256:bbb"
	items[0].Content = "# docx v2"
	items[0].SourceRef = "https://github.com/anthropics/skills.git@bbb"
	_, err = svc.Install(context.Background(), items, lockPath, false)
	if err != nil {
		t.Fatalf("install v2 failed: %v", err)
	}
	entries, _ = os.ReadDir(store.InstalledRoot(root))
	if len(entries) != 1 {
		t.Fatalf("expected 1 dir after v2 upgrade, got %d: %v", len(entries), entryNames(entries))
	}
	if entries[0].Name() == oldDir {
		t.Fatalf("old version dir should have been replaced, still found %q", oldDir)
	}
	if !strings.Contains(entries[0].Name(), "2.0.0") {
		t.Fatalf("expected new version dir to contain '2.0.0', got %q", entries[0].Name())
	}
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestInstallDeniedBySecurityLeavesNoPartialState(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	svc := &Service{Root: root, Security: security.New(config.SecurityConfig{Profile: "strict"})}
	items := []resolver.ResolvedSkill{{
		SkillRef:        "unknown/bad",
		Source:          "unknown",
		Skill:           "bad",
		ResolvedVersion: "1.0.0",
		Checksum:        "sha256:abc",
		SourceRef:       "https://example.com/bad@1",
		TrustTier:       "untrusted",
	}}
	if _, err := svc.Install(context.Background(), items, lockPath, false); err == nil {
		t.Fatalf("expected security policy denial")
	}
	st, err := store.LoadState(root)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if len(st.Installed) != 0 {
		t.Fatalf("expected no installed entries after denied install")
	}
	entries, err := os.ReadDir(store.InstalledRoot(root))
	if err != nil {
		t.Fatalf("read installed dir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no committed artifacts after denied install")
	}
}
