package installer

import (
	"context"
	"os"
	"path/filepath"
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
