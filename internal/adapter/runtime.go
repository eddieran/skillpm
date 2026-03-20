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
	"skillpm/internal/fsutil"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

type Runtime struct {
	adapters map[string]adapterapi.Adapter
}

func NewRuntime(stateRoot string, cfg config.Config, projectRoot string) (*Runtime, error) {
	if err := store.EnsureLayout(stateRoot); err != nil {
		return nil, err
	}
	r := &Runtime{adapters: map[string]adapterapi.Adapter{}}
	for _, a := range cfg.Adapters {
		if !a.Enabled {
			continue
		}
		name := strings.ToLower(a.Name)
		adapter, err := buildAdapter(name, stateRoot, projectRoot)
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

// AgentNames returns all registered adapter names.
func (r *Runtime) AgentNames() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// AgentSkillsDir returns the skills directory path for the given agent.
func (r *Runtime) AgentSkillsDir(name string) string {
	home, _ := os.UserHomeDir()
	return agentSkillsDir(name, home)
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
	return resolveAgentLayout(name, home, "").skillsDir
}

// AgentSkillsDirForScope returns the skills directory for an agent,
// choosing the project-local path when projectRoot is non-empty.
func AgentSkillsDirForScope(name, projectRoot string) string {
	if projectRoot != "" {
		return agentProjectSkillsDir(name, projectRoot)
	}
	home, _ := os.UserHomeDir()
	return agentSkillsDir(name, home)
}

func buildAdapter(name, stateRoot, projectRoot string) (adapterapi.Adapter, error) {
	home, _ := os.UserHomeDir()
	snapshotRoot := filepath.Join(store.SnapshotRoot(stateRoot), "adapters")
	if err := os.MkdirAll(snapshotRoot, 0o755); err != nil {
		return nil, err
	}

	layout := resolveAgentLayout(name, home, projectRoot)

	return &fileAdapter{
		name:         name,
		targetDir:    layout.targetDir,
		skillsDir:    layout.skillsDir,
		snapshotRoot: snapshotRoot,
		stateRoot:    stateRoot,
		rootPaths:    layout.rootPaths,
		contract:     layout.contract,
	}, nil
}

// agentProjectSkillsDir returns the project-local skills directory for an agent.
func agentProjectSkillsDir(name, projectRoot string) string {
	home, _ := os.UserHomeDir()
	return resolveAgentLayout(name, home, projectRoot).skillsDir
}

// agentProjectTargetDir returns the project-local skillpm state directory for an agent.
func agentProjectTargetDir(name, projectRoot string) string {
	home, _ := os.UserHomeDir()
	return resolveAgentLayout(name, home, projectRoot).targetDir
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
	contract     skillContract
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
	plans, warnings, err := f.buildCopyPlan(req.SkillRefs)
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
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
	if err := f.copySkillsToAgent(plans); err != nil {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_INJECT_COPY: %w", err)
	}
	if err := f.verifyCopiedSkills(plans); err != nil {
		_ = f.writeState(prev)
		return adapterapi.InjectResult{}, err
	}

	// Build injected paths map for transparency
	paths := make(map[string]string, len(next))
	for _, ref := range next {
		paths[ref] = filepath.Join(f.skillsDir, ExtractSkillName(ref))
	}

	return adapterapi.InjectResult{
		Agent:              f.name,
		Injected:           next,
		SkillsDir:          f.skillsDir,
		InjectedPaths:      paths,
		SnapshotPath:       snapshot,
		RollbackPossible:   true,
		Validated:          true,
		ValidationWarnings: warnings,
	}, nil
}

// copySkillsToAgent copies each skill's installed content into the agent's skills dir.
func (f *fileAdapter) copySkillsToAgent(plans []skillCopyPlan) error {
	if err := os.MkdirAll(f.skillsDir, 0o755); err != nil {
		return err
	}
	for _, plan := range plans {
		if err := copyDir(plan.SrcDir, plan.DestDir); err != nil {
			return fmt.Errorf("copy %s to %s: %w", plan.Ref, plan.DestDir, err)
		}
		skillPath := filepath.Join(plan.DestDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(plan.SkillContent), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", skillPath, err)
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

// ExtractSkillName gets the leaf skill name from a ref.
// "myhub/code-review" → "code-review"
// "openai_skills/skills/.curated/gh-fix-ci" → "gh-fix-ci"
func ExtractSkillName(ref string) string {
	parts := strings.SplitN(ref, "/", 2)
	skill := ref
	if len(parts) == 2 {
		skill = parts[1]
	}
	// For nested paths, use only the last component
	if idx := strings.LastIndex(skill, "/"); idx >= 0 {
		skill = skill[idx+1:]
	}
	return skill
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
		skillName := ExtractSkillName(ref)
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
	return fsutil.AtomicWrite(f.statePath(), blob, 0o644)
}

func safeEntryName(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "@", "_", " ", "-")
	out := r.Replace(v)
	if out == "" {
		return "unknown"
	}
	return out
}
