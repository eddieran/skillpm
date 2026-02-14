package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureLayoutCreatesExpectedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	for _, dir := range []string{
		root,
		InstalledRoot(root),
		StagingRoot(root),
		SnapshotRoot(root),
		InboxRoot(root),
		AdapterStateRoot(root),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
}

func TestEnsureLayoutErrorsWhenRootIsAFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatalf("write root file failed: %v", err)
	}
	if err := EnsureLayout(root); err == nil {
		t.Fatalf("expected ensure layout to fail when root is a file")
	}
}

func TestLoadStateMissingFileReturnsDefaultState(t *testing.T) {
	root := t.TempDir()
	st, err := LoadState(root)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if st.Version != StateVersion {
		t.Fatalf("expected version %d, got %d", StateVersion, st.Version)
	}
	if len(st.Installed) != 0 || len(st.Injections) != 0 {
		t.Fatalf("expected empty state, got %+v", st)
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Round(time.Second)
	st := State{
		Version: 99,
		Installed: []InstalledSkill{
			{SkillRef: "z/skill", ResolvedVersion: "1.0.0", InstalledAt: now},
			{SkillRef: "a/skill", ResolvedVersion: "2.0.0", InstalledAt: now},
		},
		Injections: []InjectionState{
			{Agent: "zeta", Skills: []string{"z/skill"}, UpdatedAt: now},
			{Agent: "alpha", Skills: []string{"a/skill"}, UpdatedAt: now},
		},
	}
	if err := SaveState(root, st); err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if loaded.Version != StateVersion {
		t.Fatalf("expected version %d, got %d", StateVersion, loaded.Version)
	}
	if len(loaded.Installed) != 2 || loaded.Installed[0].SkillRef != "a/skill" || loaded.Installed[1].SkillRef != "z/skill" {
		t.Fatalf("expected installed entries to be sorted, got %+v", loaded.Installed)
	}
	if len(loaded.Injections) != 2 || loaded.Injections[0].Agent != "alpha" || loaded.Injections[1].Agent != "zeta" {
		t.Fatalf("expected injections to be sorted, got %+v", loaded.Injections)
	}
}

func TestLoadStateInvalidTOMLReturnsParseError(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	if err := os.WriteFile(StatePath(root), []byte("version = ["), 0o644); err != nil {
		t.Fatalf("write invalid state failed: %v", err)
	}
	_, err := LoadState(root)
	if err == nil || !strings.Contains(err.Error(), "DOC_STATE_PARSE") {
		t.Fatalf("expected DOC_STATE_PARSE error, got %v", err)
	}
}

func TestLoadLockfileMissingFileReturnsDefaultLockfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	lock, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if lock.Version != LockVersion {
		t.Fatalf("expected version %d, got %d", LockVersion, lock.Version)
	}
	if len(lock.Skills) != 0 {
		t.Fatalf("expected empty lockfile, got %+v", lock.Skills)
	}
}

func TestSaveAndLoadLockfileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "skills.lock")
	lock := Lockfile{
		Version: 99,
		Skills: []LockSkill{
			{SkillRef: "z/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:z", SourceRef: "src:z"},
			{SkillRef: "a/skill", ResolvedVersion: "2.0.0", Checksum: "sha256:a", SourceRef: "src:a"},
		},
	}
	if err := SaveLockfile(path, lock); err != nil {
		t.Fatalf("save lockfile failed: %v", err)
	}
	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if loaded.Version != LockVersion {
		t.Fatalf("expected version %d, got %d", LockVersion, loaded.Version)
	}
	if len(loaded.Skills) != 2 || loaded.Skills[0].SkillRef != "a/skill" || loaded.Skills[1].SkillRef != "z/skill" {
		t.Fatalf("expected sorted lockfile skills, got %+v", loaded.Skills)
	}
}

func TestLoadLockfileInvalidTOMLReturnsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	if err := os.WriteFile(path, []byte("version = ["), 0o644); err != nil {
		t.Fatalf("write invalid lockfile failed: %v", err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_PARSE") {
		t.Fatalf("expected DOC_LOCK_PARSE error, got %v", err)
	}
}
