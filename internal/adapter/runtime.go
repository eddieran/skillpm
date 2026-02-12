package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"skillpm/internal/config"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

type Runtime struct {
	adapters map[string]adapterapi.Adapter
}

func NewRuntime(stateRoot string, cfg config.Config) (*Runtime, error) {
	if err := store.EnsureLayout(stateRoot); err != nil {
		return nil, err
	}
	r := &Runtime{adapters: map[string]adapterapi.Adapter{}}
	for _, a := range cfg.Adapters {
		if !a.Enabled {
			continue
		}
		name := strings.ToLower(a.Name)
		adapter, err := buildAdapter(name, stateRoot)
		if err != nil {
			return nil, err
		}
		r.adapters[name] = adapter
	}
	return r, nil
}

func (r *Runtime) Get(name string) (adapterapi.Adapter, error) {
	a, ok := r.adapters[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("ADP_NOT_SUPPORTED: adapter %q is not configured", name)
	}
	return a, nil
}

func (r *Runtime) ProbeAll(ctx context.Context) ([]adapterapi.ProbeResult, error) {
	out := make([]adapterapi.ProbeResult, 0, len(r.adapters))
	for name, adp := range r.adapters {
		res, err := adp.Probe(ctx)
		if err != nil {
			return nil, fmt.Errorf("ADP_PROBE_%s: %w", name, err)
		}
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func buildAdapter(name, stateRoot string) (adapterapi.Adapter, error) {
	home, _ := os.UserHomeDir()
	snapshotRoot := filepath.Join(store.SnapshotRoot(stateRoot), "adapters")
	if err := os.MkdirAll(snapshotRoot, 0o755); err != nil {
		return nil, err
	}
	switch name {
	case "codex":
		target := filepath.Join(home, ".codex", "skillpm")
		return &fileAdapter{name: "codex", targetDir: target, snapshotRoot: snapshotRoot, rootPaths: []string{target}}, nil
	case "openclaw":
		configPath := os.Getenv("OPENCLAW_CONFIG_PATH")
		if configPath == "" {
			configPath = filepath.Join(home, ".openclaw", "config.toml")
		}
		stateDir := os.Getenv("OPENCLAW_STATE_DIR")
		if stateDir == "" {
			stateDir = filepath.Join(home, ".openclaw", "state")
		}
		target := filepath.Join(stateDir, "skillpm")
		return &fileAdapter{name: "openclaw", targetDir: target, snapshotRoot: snapshotRoot, rootPaths: []string{target, configPath, stateDir}}, nil
	case "claude", "cursor", "qwen", "vscode", "opcode":
		target := filepath.Join(home, "."+name, "skillpm")
		return &fileAdapter{name: name, targetDir: target, snapshotRoot: snapshotRoot, rootPaths: []string{target}}, nil
	default:
		return nil, fmt.Errorf("ADP_NOT_SUPPORTED: unknown adapter %q", name)
	}
}

type injectedState struct {
	Skills []string `toml:"skills"`
}

type fileAdapter struct {
	name         string
	targetDir    string
	snapshotRoot string
	rootPaths    []string
}

func (f *fileAdapter) statePath() string {
	return filepath.Join(f.targetDir, "injected.toml")
}

func (f *fileAdapter) Probe(_ context.Context) (adapterapi.ProbeResult, error) {
	if err := os.MkdirAll(f.targetDir, 0o755); err != nil {
		return adapterapi.ProbeResult{}, err
	}
	caps := []string{"inject", "remove", "list", "harvest", "validate"}
	return adapterapi.ProbeResult{Name: f.name, Available: true, Capabilities: caps}, nil
}

func (f *fileAdapter) Inject(_ context.Context, req adapterapi.InjectRequest) (adapterapi.InjectResult, error) {
	if err := os.MkdirAll(f.targetDir, 0o755); err != nil {
		return adapterapi.InjectResult{}, err
	}
	snapshot, prev, err := f.snapshot()
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
	set := map[string]struct{}{}
	for _, s := range prev.Skills {
		set[s] = struct{}{}
	}
	for _, s := range req.SkillRefs {
		set[s] = struct{}{}
	}
	next := make([]string, 0, len(set))
	for s := range set {
		next = append(next, s)
	}
	sort.Strings(next)
	if err := f.writeState(injectedState{Skills: next}); err != nil {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_INJECT_WRITE: %w", err)
	}
	return adapterapi.InjectResult{Agent: f.name, Injected: next, SnapshotPath: snapshot, RollbackPossible: true}, nil
}

func (f *fileAdapter) Remove(_ context.Context, req adapterapi.RemoveRequest) (adapterapi.RemoveResult, error) {
	snapshot, prev, err := f.snapshot()
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	removeSet := map[string]struct{}{}
	for _, s := range req.SkillRefs {
		removeSet[s] = struct{}{}
	}
	if len(removeSet) == 0 {
		if err := f.writeState(injectedState{Skills: []string{}}); err != nil {
			_ = f.writeState(prev)
			return adapterapi.RemoveResult{}, err
		}
		return adapterapi.RemoveResult{Agent: f.name, Removed: prev.Skills, SnapshotPath: snapshot}, nil
	}
	kept := make([]string, 0, len(prev.Skills))
	removed := make([]string, 0, len(prev.Skills))
	for _, s := range prev.Skills {
		if _, ok := removeSet[s]; ok {
			removed = append(removed, s)
			continue
		}
		kept = append(kept, s)
	}
	if err := f.writeState(injectedState{Skills: kept}); err != nil {
		_ = f.writeState(prev)
		return adapterapi.RemoveResult{}, err
	}
	sort.Strings(removed)
	return adapterapi.RemoveResult{Agent: f.name, Removed: removed, SnapshotPath: snapshot}, nil
}

func (f *fileAdapter) ListInjected(_ context.Context, _ adapterapi.ListInjectedRequest) (adapterapi.ListInjectedResult, error) {
	st, err := f.readState()
	if err != nil {
		return adapterapi.ListInjectedResult{}, err
	}
	sort.Strings(st.Skills)
	return adapterapi.ListInjectedResult{Agent: f.name, Skills: st.Skills}, nil
}

func (f *fileAdapter) HarvestCandidates(_ context.Context, _ adapterapi.HarvestRequest) (adapterapi.HarvestResult, error) {
	candidates := []adapterapi.HarvestCandidate{}
	seen := map[string]struct{}{}
	for _, root := range f.rootPaths {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() != "SKILL.md" {
				return nil
			}
			dir := filepath.Dir(p)
			if _, ok := seen[dir]; ok {
				return nil
			}
			seen[dir] = struct{}{}
			candidates = append(candidates, adapterapi.HarvestCandidate{Path: dir, Name: filepath.Base(dir), Adapter: f.name})
			return nil
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return adapterapi.HarvestResult{Agent: f.name, Candidates: candidates, Supported: true}, nil
}

func (f *fileAdapter) ValidateEnvironment(_ context.Context) (adapterapi.ValidateResult, error) {
	warnings := []string{}
	for _, p := range f.rootPaths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, err.Error())
		}
	}
	valid := len(warnings) == 0
	return adapterapi.ValidateResult{Agent: f.name, Valid: valid, Warnings: warnings, RootPaths: f.rootPaths}, nil
}

func (f *fileAdapter) snapshot() (string, injectedState, error) {
	prev, err := f.readState()
	if err != nil {
		return "", injectedState{}, err
	}
	if err := os.MkdirAll(f.snapshotRoot, 0o755); err != nil {
		return "", injectedState{}, err
	}
	snap := filepath.Join(f.snapshotRoot, fmt.Sprintf("%s-%d.toml", f.name, time.Now().UnixNano()))
	blob, err := toml.Marshal(prev)
	if err != nil {
		return "", injectedState{}, err
	}
	if err := os.WriteFile(snap, blob, 0o644); err != nil {
		return "", injectedState{}, err
	}
	return snap, prev, nil
}

func (f *fileAdapter) readState() (injectedState, error) {
	blob, err := os.ReadFile(f.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return injectedState{Skills: []string{}}, nil
		}
		return injectedState{}, err
	}
	var st injectedState
	if err := toml.Unmarshal(blob, &st); err != nil {
		return injectedState{}, fmt.Errorf("ADP_STATE_PARSE: %w", err)
	}
	return st, nil
}

func (f *fileAdapter) writeState(st injectedState) error {
	if err := os.MkdirAll(f.targetDir, 0o755); err != nil {
		return err
	}
	blob, err := toml.Marshal(st)
	if err != nil {
		return err
	}
	tmp := f.statePath() + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.statePath())
}
