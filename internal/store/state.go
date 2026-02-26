package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

func EnsureLayout(root string) error {
	dirs := []string{root, InstalledRoot(root), StagingRoot(root), SnapshotRoot(root), InboxRoot(root), AdapterStateRoot(root), MemoryRoot(root)}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func LoadState(root string) (State, error) {
	if err := EnsureLayout(root); err != nil {
		return State{}, err
	}
	path := StatePath(root)
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Version: StateVersion}, nil
		}
		return State{}, err
	}
	var st State
	if err := toml.Unmarshal(blob, &st); err != nil {
		return State{}, fmt.Errorf("DOC_STATE_PARSE: %w", err)
	}
	if st.Version == 0 {
		st.Version = StateVersion
	}
	if st.Version != StateVersion {
		return State{}, fmt.Errorf("DOC_STATE_VERSION: unsupported state version %d", st.Version)
	}
	for i := range st.Installed {
		if st.Installed[i].SkillRef == "" {
			return State{}, fmt.Errorf("DOC_STATE_SCHEMA: installed entry missing skill_ref")
		}
	}
	return st, nil
}

func SaveState(root string, st State) error {
	if err := EnsureLayout(root); err != nil {
		return err
	}
	st.Version = StateVersion
	sort.Slice(st.Installed, func(i, j int) bool {
		return st.Installed[i].SkillRef < st.Installed[j].SkillRef
	})
	sort.Slice(st.Injections, func(i, j int) bool {
		return st.Injections[i].Agent < st.Injections[j].Agent
	})
	blob, err := toml.Marshal(st)
	if err != nil {
		return fmt.Errorf("DOC_STATE_ENCODE: %w", err)
	}
	path := StatePath(root)
	tmp := filepath.Join(filepath.Dir(path), ".state.toml.tmp")
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func UpsertInstalled(st *State, rec InstalledSkill) {
	for i := range st.Installed {
		if st.Installed[i].SkillRef == rec.SkillRef {
			st.Installed[i] = rec
			return
		}
	}
	st.Installed = append(st.Installed, rec)
}

func RemoveInstalled(st *State, skillRef string) bool {
	for i := range st.Installed {
		if st.Installed[i].SkillRef == skillRef {
			st.Installed = append(st.Installed[:i], st.Installed[i+1:]...)
			return true
		}
	}
	return false
}

func SetInjection(st *State, in InjectionState) {
	for i := range st.Injections {
		if st.Injections[i].Agent == in.Agent {
			st.Injections[i] = in
			return
		}
	}
	st.Injections = append(st.Injections, in)
}
