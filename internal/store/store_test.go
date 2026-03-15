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

// --- UpsertInstalled ---

func TestUpsertInstalledAddsNew(t *testing.T) {
	st := State{}
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0"})
	if len(st.Installed) != 1 || st.Installed[0].SkillRef != "a/skill" {
		t.Fatalf("expected 1 installed entry, got %+v", st.Installed)
	}
}

func TestUpsertInstalledUpdatesExisting(t *testing.T) {
	st := State{
		Installed: []InstalledSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0"},
		},
	}
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "2.0.0"})
	if len(st.Installed) != 1 {
		t.Fatalf("expected 1 installed entry, got %d", len(st.Installed))
	}
	if st.Installed[0].ResolvedVersion != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", st.Installed[0].ResolvedVersion)
	}
}

func TestUpsertInstalledMultipleEntries(t *testing.T) {
	st := State{}
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0"})
	UpsertInstalled(&st, InstalledSkill{SkillRef: "b/skill", ResolvedVersion: "1.0.0"})
	UpsertInstalled(&st, InstalledSkill{SkillRef: "a/skill", ResolvedVersion: "3.0.0"})
	if len(st.Installed) != 2 {
		t.Fatalf("expected 2 installed entries, got %d", len(st.Installed))
	}
	if st.Installed[0].ResolvedVersion != "3.0.0" {
		t.Fatalf("expected a/skill at 3.0.0, got %s", st.Installed[0].ResolvedVersion)
	}
}

// --- RemoveInstalled ---

func TestRemoveInstalledExisting(t *testing.T) {
	st := State{
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
	st := State{
		Installed: []InstalledSkill{{SkillRef: "a/skill"}},
	}
	ok := RemoveInstalled(&st, "nonexistent/skill")
	if ok {
		t.Fatalf("expected RemoveInstalled to return false for nonexistent skill")
	}
	if len(st.Installed) != 1 {
		t.Fatalf("expected 1 installed entry unchanged, got %d", len(st.Installed))
	}
}

func TestRemoveInstalledFromEmpty(t *testing.T) {
	st := State{}
	ok := RemoveInstalled(&st, "a/skill")
	if ok {
		t.Fatalf("expected RemoveInstalled to return false on empty state")
	}
}

// --- SetInjection ---

func TestSetInjectionAddsNew(t *testing.T) {
	st := State{}
	now := time.Now().UTC()
	SetInjection(&st, InjectionState{Agent: "agent1", Skills: []string{"a/skill"}, UpdatedAt: now})
	if len(st.Injections) != 1 || st.Injections[0].Agent != "agent1" {
		t.Fatalf("expected 1 injection, got %+v", st.Injections)
	}
}

func TestSetInjectionUpdatesExisting(t *testing.T) {
	now := time.Now().UTC()
	st := State{
		Injections: []InjectionState{
			{Agent: "agent1", Skills: []string{"a/skill"}, UpdatedAt: now},
		},
	}
	later := now.Add(time.Hour)
	SetInjection(&st, InjectionState{Agent: "agent1", Skills: []string{"a/skill", "b/skill"}, UpdatedAt: later})
	if len(st.Injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(st.Injections))
	}
	if len(st.Injections[0].Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(st.Injections[0].Skills))
	}
	if st.Injections[0].UpdatedAt != later {
		t.Fatalf("expected updated timestamp")
	}
}

func TestSetInjectionMultipleAgents(t *testing.T) {
	st := State{}
	now := time.Now().UTC()
	SetInjection(&st, InjectionState{Agent: "agent1", Skills: []string{"a/skill"}, UpdatedAt: now})
	SetInjection(&st, InjectionState{Agent: "agent2", Skills: []string{"b/skill"}, UpdatedAt: now})
	if len(st.Injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(st.Injections))
	}
}

// --- UpsertLock ---

func TestUpsertLockAddsNew(t *testing.T) {
	lock := Lockfile{}
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"})
	if len(lock.Skills) != 1 || lock.Skills[0].SkillRef != "a/skill" {
		t.Fatalf("expected 1 lock skill, got %+v", lock.Skills)
	}
}

func TestUpsertLockUpdatesExisting(t *testing.T) {
	lock := Lockfile{
		Skills: []LockSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"},
		},
	}
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "2.0.0", Checksum: "sha256:a2", SourceRef: "src:a2"})
	if len(lock.Skills) != 1 {
		t.Fatalf("expected 1 lock skill, got %d", len(lock.Skills))
	}
	if lock.Skills[0].ResolvedVersion != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", lock.Skills[0].ResolvedVersion)
	}
}

func TestUpsertLockMultipleEntries(t *testing.T) {
	lock := Lockfile{}
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"})
	UpsertLock(&lock, LockSkill{SkillRef: "b/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:b", SourceRef: "src:b"})
	UpsertLock(&lock, LockSkill{SkillRef: "a/skill", ResolvedVersion: "3.0.0", Checksum: "sha256:a3", SourceRef: "src:a3"})
	if len(lock.Skills) != 2 {
		t.Fatalf("expected 2 lock skills, got %d", len(lock.Skills))
	}
	if lock.Skills[0].ResolvedVersion != "3.0.0" {
		t.Fatalf("expected a/skill at 3.0.0, got %s", lock.Skills[0].ResolvedVersion)
	}
}

// --- RemoveLock ---

func TestRemoveLockExisting(t *testing.T) {
	lock := Lockfile{
		Skills: []LockSkill{
			{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"},
			{SkillRef: "b/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:b", SourceRef: "src:b"},
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
	lock := Lockfile{
		Skills: []LockSkill{{SkillRef: "a/skill", ResolvedVersion: "1.0.0", Checksum: "sha256:a", SourceRef: "src:a"}},
	}
	ok := RemoveLock(&lock, "nonexistent/skill")
	if ok {
		t.Fatalf("expected RemoveLock to return false for nonexistent skill")
	}
	if len(lock.Skills) != 1 {
		t.Fatalf("expected 1 lock skill unchanged, got %d", len(lock.Skills))
	}
}

func TestRemoveLockFromEmpty(t *testing.T) {
	lock := Lockfile{}
	ok := RemoveLock(&lock, "a/skill")
	if ok {
		t.Fatalf("expected RemoveLock to return false on empty lockfile")
	}
}

// --- Path functions ---

func TestPathFunctions(t *testing.T) {
	root := "/test/root"
	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{"AuditPath", AuditPath, "/test/root/audit.log"},
		{"EventLogPath", EventLogPath, "/test/root/memory/events.jsonl"},
		{"FeedbackLogPath", FeedbackLogPath, "/test/root/memory/feedback.jsonl"},
		{"ScoresPath", ScoresPath, "/test/root/memory/scores.toml"},
		{"ConsolidationPath", ConsolidationPath, "/test/root/memory/consolidation.toml"},
		{"ContextProfilePath", ContextProfilePath, "/test/root/memory/context.toml"},
		{"ScanStatePath", ScanStatePath, "/test/root/memory/scan_state.toml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn(root)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

// --- LoadState error branches ---

func TestLoadStateUnsupportedVersion(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	if err := os.WriteFile(StatePath(root), []byte("version = 999\n"), 0o644); err != nil {
		t.Fatalf("write state failed: %v", err)
	}
	_, err := LoadState(root)
	if err == nil || !strings.Contains(err.Error(), "DOC_STATE_VERSION") {
		t.Fatalf("expected DOC_STATE_VERSION error, got %v", err)
	}
}

func TestLoadStateMissingSkillRef(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	content := "version = 1\n\n[[installed]]\nsource = \"test\"\n"
	if err := os.WriteFile(StatePath(root), []byte(content), 0o644); err != nil {
		t.Fatalf("write state failed: %v", err)
	}
	_, err := LoadState(root)
	if err == nil || !strings.Contains(err.Error(), "DOC_STATE_SCHEMA") {
		t.Fatalf("expected DOC_STATE_SCHEMA error, got %v", err)
	}
}

func TestLoadStateReadPermissionError(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	statePath := StatePath(root)
	if err := os.WriteFile(statePath, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("write state failed: %v", err)
	}
	if err := os.Chmod(statePath, 0o000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Cleanup(func() { os.Chmod(statePath, 0o644) })
	_, err := LoadState(root)
	if err == nil {
		t.Fatalf("expected permission error reading state file")
	}
}

func TestLoadStateVersionZeroDefaulted(t *testing.T) {
	root := t.TempDir()
	if err := EnsureLayout(root); err != nil {
		t.Fatalf("ensure layout failed: %v", err)
	}
	// version = 0 is omitted from TOML, so write a file with just installed entries
	content := "[[installed]]\nskill_ref = \"a/skill\"\n"
	if err := os.WriteFile(StatePath(root), []byte(content), 0o644); err != nil {
		t.Fatalf("write state failed: %v", err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if st.Version != StateVersion {
		t.Fatalf("expected version %d, got %d", StateVersion, st.Version)
	}
}

// --- LoadLockfile error branches ---

func TestLoadLockfileUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	if err := os.WriteFile(path, []byte("version = 999\n"), 0o644); err != nil {
		t.Fatalf("write lockfile failed: %v", err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_VERSION") {
		t.Fatalf("expected DOC_LOCK_VERSION error, got %v", err)
	}
}

func TestLoadLockfileMissingSkillRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	content := "version = 1\n\n[[skills]]\nresolvedVersion = \"1.0.0\"\nchecksum = \"sha256:a\"\nsourceRef = \"src:a\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile failed: %v", err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_LOCK_SCHEMA") {
		t.Fatalf("expected DOC_LOCK_SCHEMA error for missing skillRef, got %v", err)
	}
}

func TestLoadLockfileDuplicateSkillRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	content := `version = 1

[[skills]]
skillRef = "a/skill"
resolvedVersion = "1.0.0"
checksum = "sha256:a"
sourceRef = "src:a"

[[skills]]
skillRef = "a/skill"
resolvedVersion = "2.0.0"
checksum = "sha256:a2"
sourceRef = "src:a2"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile failed: %v", err)
	}
	_, err := LoadLockfile(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate skillRef") {
		t.Fatalf("expected duplicate skillRef error, got %v", err)
	}
}

func TestLoadLockfileIncompleteRecord(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "missing resolvedVersion",
			content: "version = 1\n\n[[skills]]\nskillRef = \"a/skill\"\nchecksum = \"sha256:a\"\nsourceRef = \"src:a\"\n",
		},
		{
			name:    "missing checksum",
			content: "version = 1\n\n[[skills]]\nskillRef = \"a/skill\"\nresolvedVersion = \"1.0.0\"\nsourceRef = \"src:a\"\n",
		},
		{
			name:    "missing sourceRef",
			content: "version = 1\n\n[[skills]]\nskillRef = \"a/skill\"\nresolvedVersion = \"1.0.0\"\nchecksum = \"sha256:a\"\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "skills.lock")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write lockfile failed: %v", err)
			}
			_, err := LoadLockfile(path)
			if err == nil || !strings.Contains(err.Error(), "incomplete record") {
				t.Fatalf("expected incomplete record error, got %v", err)
			}
		})
	}
}

func TestLoadLockfileReadPermissionError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("write lockfile failed: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	_, err := LoadLockfile(path)
	if err == nil {
		t.Fatalf("expected permission error reading lockfile")
	}
}

func TestLoadLockfileVersionZeroDefaulted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.lock")
	content := "[[skills]]\nskillRef = \"a/skill\"\nresolvedVersion = \"1.0.0\"\nchecksum = \"sha256:a\"\nsourceRef = \"src:a\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile failed: %v", err)
	}
	lock, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if lock.Version != LockVersion {
		t.Fatalf("expected version %d, got %d", LockVersion, lock.Version)
	}
}

// --- SaveState/SaveLockfile with EnsureLayout error ---

func TestSaveStateEnsureLayoutError(t *testing.T) {
	// root is a file, not a directory → EnsureLayout will fail
	root := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatalf("write root file failed: %v", err)
	}
	err := SaveState(root, State{})
	if err == nil {
		t.Fatalf("expected SaveState to fail when root is a file")
	}
}
