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

// --- Path helpers ---

func TestPathHelpers(t *testing.T) {
	root := "/fake/root"
	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{"AuditPath", AuditPath, filepath.Join(root, "audit.log")},
		{"EventLogPath", EventLogPath, filepath.Join(root, "memory", "events.jsonl")},
		{"FeedbackLogPath", FeedbackLogPath, filepath.Join(root, "memory", "feedback.jsonl")},
		{"ScoresPath", ScoresPath, filepath.Join(root, "memory", "scores.toml")},
		{"ConsolidationPath", ConsolidationPath, filepath.Join(root, "memory", "consolidation.toml")},
		{"ContextProfilePath", ContextProfilePath, filepath.Join(root, "memory", "context.toml")},
		{"ScanStatePath", ScanStatePath, filepath.Join(root, "memory", "scan_state.toml")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(root); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- UpsertLock / RemoveLock ---

func TestUpsertLockInsertsNew(t *testing.T) {
	lock := Lockfile{Version: LockVersion}
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"})
	if len(lock.Skills) != 1 || lock.Skills[0].SkillRef != "a/skill" {
		t.Fatalf("expected one skill, got %+v", lock.Skills)
	}
}

func TestUpsertLockUpdatesExisting(t *testing.T) {
	lock := Lockfile{
		Version: LockVersion,
		Skills: []LockSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"},
		},
	}
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "2.0.0", Checksum: "sha256:b", SourceRef: "src:b"})
	if len(lock.Skills) != 1 {
		t.Fatalf("expected one skill after upsert, got %d", len(lock.Skills))
	}
	if lock.Skills[0].ResolvedVersion != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", lock.Skills[0].ResolvedVersion)
	}
}

func TestRemoveLockExisting(t *testing.T) {
	lock := Lockfile{
		Version: LockVersion,
		Skills: []LockSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"},
			{SkillRef: "b/skill", ResolvedVersion: "2.0.0", Checksum: "sha256:b", SourceRef: "src:b"},
		},
	}
	ok := RemoveLock(&lock, "a/skill")
	if !ok {
		t.Fatalf("expected RemoveLock to return true")
	}
	if len(lock.Skills) != 1 || lock.Skills[0].SkillRef != "b/skill" {
		t.Fatalf("expected only b/skill remaining, got %+v", lock.Skills)
	}
}

func TestRemoveLockNotFound(t *testing.T) {
	lock := Lockfile{Version: LockVersion}
	ok := RemoveLock(&lock, "nonexistent/skill")
	if ok {
		t.Fatalf("expected RemoveLock to return false for nonexistent skill")
	}
}

// --- UpsertInstalled / RemoveInstalled / SetInjection ---

func TestUpsertInstalledInsertsNew(t *testing.T) {
	st := State{Version: StateVersion}
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0"})
	if len(st.Installed) != 1 || st.Installed[0].SkillRef != "a/skill" {
		t.Fatalf("expected one installed skill, got %+v", st.Installed)
	}
}

func TestUpsertInstalledUpdatesExisting(t *testing.T) {
	st := State{
		Version: StateVersion,
		Installed: []InstalledSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0"},
		},
	}
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "2.0.0"})
	if len(st.Installed) != 1 {
		t.Fatalf("expected one installed skill after upsert, got %d", len(st.Installed))
	}
	if st.Installed[0].ResolvedVersion != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", st.Installed[0].ResolvedVersion)
	}
}

func TestRemoveInstalledExisting(t *testing.T) {
	st := State{
		Version: StateVersion,
		Installed: []InstalledSkill{
			{SkillRef: "a/skill"},
			{SkillRef: "b/skill"},
		},
	}
	ok := RemoveInstalled(&st, "a/skill")
	if !ok {
		t.Fatalf("expected RemoveInstalled to return true")
	}
	if len(st.Installed) != 1 || st.Installed[0].SkillRef != "b/skill" {
		t.Fatalf("expected only b/skill remaining, got %+v", st.Installed)
	}
}

func TestRemoveInstalledNotFound(t *testing.T) {
	st := State{Version: StateVersion}
	ok := RemoveInstalled(&st, "nonexistent/skill")
	if ok {
		t.Fatalf("expected RemoveInstalled to return false for nonexistent skill")
	}
}

func TestSetInjectionInsertsNew(t *testing.T) {
	st := State{Version: StateVersion}
	now := time.Now().UTC()
	SetInjection(&st, InjectionState{Agent: "agent1", Skills: []string{"a/skill"}, UpdatedAt: now})
	if len(st.Injections) != 1 || st.Injections[0].Agent != "agent1" {
		t.Fatalf("expected one injection, got %+v", st.Injections)
	}
}

func TestSetInjectionUpdatesExisting(t *testing.T) {
	now := time.Now().UTC()
	st := State{
		Version: StateVersion,
		Injections: []InjectionState{
			{Agent: "agent1", Skills: []string{"a/skill"}, UpdatedAt: now},
		},
	}
	later := now.Add(time.Hour)
	SetInjection(&st, InjectionState{Agent: "agent1", Skills: []string{"b/skill"}, UpdatedAt: later})
	if len(st.Injections) != 1 {
		t.Fatalf("expected one injection after update, got %d", len(st.Injections))
	}
	if st.Injections[0].Skills[0] != "b/skill" {
		t.Fatalf("expected b/skill, got %s", st.Injections[0].Skills[0])
	}
}

// --- LoadLockfile validation error paths ---

func TestLoadLockfileUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	if err := os.WriteFile(path, []byte("version = 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_VERSION") {
		t.Fatalf("expected DOC_LOCK_VERSION error, got %v", err)
	}
}

func TestLoadLockfileMissingSkillRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	data := "version = 1\n[[skills]]\nresolvedVersion = \"1.0\"\nchecksum = \"sha256:x\"\nsourceRef = \"src:x\"\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_SCHEMA: missing skillRef") {
		t.Fatalf("expected DOC_LOCK_SCHEMA missing skillRef, got %v", err)
	}
}

func TestLoadLockfileDuplicateSkillRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	data := `version = 1
[[skills]]
skillRef = "a/skill"
resolvedVersion = "1.0"
checksum = "sha256:x"
sourceRef = "src:x"
[[skills]]
skillRef = "a/skill"
resolvedVersion = "2.0"
checksum = "sha256:y"
sourceRef = "src:y"
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_SCHEMA: duplicate skillRef") {
		t.Fatalf("expected DOC_LOCK_SCHEMA duplicate, got %v", err)
	}
}

func TestLoadLockfileIncompleteRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	data := "version = 1\n[[skills]]\nskillRef = \"a/skill\"\nresolvedVersion = \"1.0\"\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_SCHEMA: incomplete record") {
		t.Fatalf("expected DOC_LOCK_SCHEMA incomplete, got %v", err)
	}
}

func TestLoadLockfileReadPermissionError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.lock")
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLockfile(path)
	if err == nil {
		t.Fatalf("expected permission error, got nil")
	}
}

func TestLoadLockfileZeroVersionDefaultsToLockVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	// version = 0 means the key is absent in TOML; write an empty file
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lock.Version != LockVersion {
		t.Fatalf("expected version %d, got %d", LockVersion, lock.Version)
	}
}

// --- LoadState validation error paths ---

func TestLoadStateUnsupportedVersion(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StatePath(root), []byte("version = 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root)
	if err == nil || !strings.Contains(err.Error(), "DOC_STATE_VERSION") {
		t.Fatalf("expected DOC_STATE_VERSION error, got %v", err)
	}
}

func TestLoadStateMissingSkillRef(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	data := "version = 1\n[[installed]]\nresolved_version = \"1.0\"\n"
	if err := os.WriteFile(StatePath(root), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root)
	if err == nil || !strings.Contains(err.Error(), "DOC_STATE_SCHEMA") {
		t.Fatalf("expected DOC_STATE_SCHEMA error, got %v", err)
	}
}

func TestLoadStateReadPermissionError(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(StatePath(root), []byte("version = 1\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(root)
	if err == nil {
		t.Fatalf("expected permission error, got nil")
	}
}

func TestLoadStateZeroVersionDefaultsToStateVersion(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	// Empty TOML file → version field defaults to 0
	if err := os.WriteFile(StatePath(root), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if st.Version != StateVersion {
		t.Fatalf("expected version %d, got %d", StateVersion, st.Version)
	}
}

// --- EnsureLayout creates memory directory ---

func TestEnsureLayoutCreatesMemoryDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	info, err := os.Stat(MemoryRoot(root))
	if err != nil {
		t.Fatalf("expected memory dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected memory root to be a directory")
	}
}
