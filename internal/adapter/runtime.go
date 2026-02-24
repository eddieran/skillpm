package adapter

import (
	"context"
	"fmt"
	"io/fs"
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

// agentSkillsDir returns the path where an agent reads skills from.
func agentSkillsDir(name, home string) string {
	switch name {
	case "claude":
		return filepath.Join(home, ".claude", "skills")
	case "codex":
		return filepath.Join(home, ".codex", "skills")
	case "cursor":
		return filepath.Join(home, ".cursor", "skills")
	case "gemini", "antigravity":
		return filepath.Join(home, ".gemini", "skills")
	case "copilot", "vscode":
		return filepath.Join(home, ".copilot", "skills")
	case "trae":
		return filepath.Join(home, ".trae", "skills")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "skills")
	case "kiro":
		return filepath.Join(home, ".kiro", "skills")
	case "openclaw":
		stateDir := os.Getenv("OPENCLAW_STATE_DIR")
		if stateDir == "" {
			stateDir = filepath.Join(home, ".openclaw", "state")
		}
		return filepath.Join(stateDir, "..", "workspace", "skills")
	default:
		return filepath.Join(home, "."+name, "skills")
	}
}

func buildAdapter(name, stateRoot string) (adapterapi.Adapter, error) {
	home, _ := os.UserHomeDir()
	snapshotRoot := filepath.Join(store.SnapshotRoot(stateRoot), "adapters")
	if err := os.MkdirAll(snapshotRoot, 0o755); err != nil {
		return nil, err
	}

	skillsDir := agentSkillsDir(name, home)

	// For openclaw, also track config path and state dir as root paths for harvest/validate.
	rootPaths := []string{skillsDir}
	if name == "openclaw" {
		stateDir := os.Getenv("OPENCLAW_STATE_DIR")
		if stateDir == "" {
			stateDir = filepath.Join(home, ".openclaw", "state")
		}
		configPath := os.Getenv("OPENCLAW_CONFIG_PATH")
		if configPath == "" {
			configPath = filepath.Join(home, ".openclaw", "config.toml")
		}
		rootPaths = append(rootPaths, stateDir, configPath)
	}

	// targetDir is where skillpm's own state (injected.toml) is stored.
	var targetDir string
	switch name {
	case "openclaw":
		stateDir := os.Getenv("OPENCLAW_STATE_DIR")
		if stateDir == "" {
			stateDir = filepath.Join(home, ".openclaw", "state")
		}
		targetDir = filepath.Join(stateDir, "skillpm")
	case "opencode":
		targetDir = filepath.Join(home, ".config", "opencode", "skillpm")
	case "antigravity":
		targetDir = filepath.Join(home, ".gemini", "skillpm-antigravity")
	case "vscode":
		targetDir = filepath.Join(home, ".copilot", "skillpm-vscode")
	default:
		targetDir = filepath.Join(home, "."+name, "skillpm")
	}

	return &fileAdapter{
		name:         name,
		targetDir:    targetDir,
		skillsDir:    skillsDir,
		snapshotRoot: snapshotRoot,
		stateRoot:    stateRoot,
		rootPaths:    rootPaths,
	}, nil
}

type injectedState struct {
	Skills []string `toml:"skills"`
}

type fileAdapter struct {
	name         string
	targetDir    string // where injected.toml lives
	skillsDir    string // where agent reads skills from (e.g. ~/.claude/skills/)
	snapshotRoot string
	stateRoot    string
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

	// Write state TOML
	if err := f.writeState(injectedState{Skills: next}); err != nil {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_INJECT_WRITE: %w", err)
	}

	if os.Getenv("SKILLPM_TEST_FAIL_INJECT_AFTER_WRITE") == "1" {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_TEST_FAIL_INJECT: injected failure after write")
	}

	// Copy skill folders to agent's skills directory
	if err := f.copySkillsToAgent(req.SkillRefs); err != nil {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_INJECT_COPY: %w", err)
	}

	return adapterapi.InjectResult{Agent: f.name, Injected: next, SnapshotPath: snapshot, RollbackPossible: true}, nil
}

// copySkillsToAgent copies each skill's installed content into the agent's skills dir.
func (f *fileAdapter) copySkillsToAgent(skillRefs []string) error {
	if err := os.MkdirAll(f.skillsDir, 0o755); err != nil {
		return err
	}
	for _, ref := range skillRefs {
		srcDir := f.findInstalledSkillDir(ref)
		if srcDir == "" {
			continue
		}
		skillName := extractSkillName(ref)
		destDir := filepath.Join(f.skillsDir, skillName)
		if err := copyDir(srcDir, destDir); err != nil {
			return fmt.Errorf("copy %s to %s: %w", ref, destDir, err)
		}
	}
	return nil
}

// findInstalledSkillDir locates the installed directory for a skill ref.
func (f *fileAdapter) findInstalledSkillDir(ref string) string {
	installedRoot := store.InstalledRoot(f.stateRoot)
	entries, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	prefix := safeEntryName(ref) + "@"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			return filepath.Join(installedRoot, e.Name())
		}
	}
	return ""
}

// extractSkillName gets the skill name from a ref like "myhub/code-review" → "code-review"
func extractSkillName(ref string) string {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ref
}

// copyDir copies a directory tree, skipping metadata.toml (skillpm internal).
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		// Skip skillpm internal metadata
		if rel == "metadata.toml" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
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

	var kept, removed []string
	if len(removeSet) == 0 {
		// Remove all
		removed = prev.Skills
		kept = []string{}
	} else {
		for _, s := range prev.Skills {
			if _, ok := removeSet[s]; ok {
				removed = append(removed, s)
			} else {
				kept = append(kept, s)
			}
		}
	}

	if err := f.writeState(injectedState{Skills: kept}); err != nil {
		_ = f.writeState(prev)
		return adapterapi.RemoveResult{}, err
	}

	// Delete skill folders from agent's skills directory
	for _, ref := range removed {
		skillName := extractSkillName(ref)
		skillDir := filepath.Join(f.skillsDir, skillName)
		_ = os.RemoveAll(skillDir)
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

func safeEntryName(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "@", "_", " ", "-")
	out := r.Replace(v)
	if out == "" {
		return "unknown"
	}
	return out
}
