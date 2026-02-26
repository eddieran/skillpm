package doctor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

// CheckStatus represents the outcome of a single diagnostic check.
type CheckStatus string

const (
	StatusOK    CheckStatus = "ok"
	StatusFixed CheckStatus = "fixed"
	StatusWarn  CheckStatus = "warn"
	StatusError CheckStatus = "error"
)

// CheckResult holds the outcome of one diagnostic check.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Fix     string      `json:"fix,omitempty"`
}

// Report is the aggregate diagnostic output.
type Report struct {
	Healthy  bool          `json:"healthy"`
	Scope    string        `json:"scope"`
	Checks   []CheckResult `json:"checks"`
	Fixed    int           `json:"fixed"`
	Warnings int           `json:"warnings"`
	Errors   int           `json:"errors"`
}

// Service holds the dependencies needed by the doctor checks.
type Service struct {
	ConfigPath  string
	StateRoot   string
	LockPath    string
	Runtime     *adapter.Runtime
	Scope       config.Scope
	ProjectRoot string
}

// Run executes all checks in dependency order and returns a report.
func (s *Service) Run(_ context.Context) Report {
	checks := []CheckResult{
		s.checkConfig(),
		s.checkState(),
	}
	checks = append(checks, s.checkInstalledDirs())
	checks = append(checks, s.checkInjections())
	checks = append(checks, s.checkAdapterState())
	checks = append(checks, s.checkAgentSkills())
	checks = append(checks, s.checkLockfile())

	rpt := Report{
		Healthy: true,
		Scope:   string(s.Scope),
		Checks:  checks,
	}
	for _, c := range checks {
		switch c.Status {
		case StatusFixed:
			rpt.Fixed++
		case StatusWarn:
			rpt.Warnings++
		case StatusError:
			rpt.Healthy = false
			rpt.Errors++
		}
	}
	return rpt
}

// --- check 1: config ---

func (s *Service) checkConfig() CheckResult {
	name := "config"

	// Try loading; if missing, Ensure will create default.
	_, err := config.Load(s.ConfigPath)
	if err != nil {
		cfg, ensureErr := config.Ensure(s.ConfigPath)
		if ensureErr != nil {
			return CheckResult{Name: name, Status: StatusError, Message: ensureErr.Error()}
		}
		// Auto-enable detected adapters on fresh config.
		fix := "created default config"
		detected := adapter.DetectAvailable()
		var enabled []string
		for _, d := range detected {
			ok, eErr := config.EnableAdapter(&cfg, d.Name, "global")
			if eErr == nil && ok {
				enabled = append(enabled, d.Name)
			}
		}
		if len(enabled) > 0 {
			if saveErr := config.Save(s.ConfigPath, cfg); saveErr != nil {
				return CheckResult{Name: name, Status: StatusError, Message: saveErr.Error()}
			}
			fix += fmt.Sprintf("; enabled adapters: %s", strings.Join(enabled, ", "))
		}
		return CheckResult{Name: name, Status: StatusFixed, Message: "config valid", Fix: fix}
	}

	// Config exists — check if detected adapters need enabling.
	cfg, _ := config.Load(s.ConfigPath)
	detected := adapter.DetectAvailable()
	enabledSet := map[string]struct{}{}
	for _, a := range cfg.Adapters {
		if a.Enabled {
			enabledSet[strings.ToLower(a.Name)] = struct{}{}
		}
	}
	var newlyEnabled []string
	for _, d := range detected {
		if _, ok := enabledSet[strings.ToLower(d.Name)]; ok {
			continue
		}
		ok, eErr := config.EnableAdapter(&cfg, d.Name, "global")
		if eErr == nil && ok {
			newlyEnabled = append(newlyEnabled, d.Name)
		}
	}
	if len(newlyEnabled) > 0 {
		if saveErr := config.Save(s.ConfigPath, cfg); saveErr != nil {
			return CheckResult{Name: name, Status: StatusError, Message: saveErr.Error()}
		}
		return CheckResult{
			Name:    name,
			Status:  StatusFixed,
			Message: "config valid",
			Fix:     fmt.Sprintf("enabled adapters: %s", strings.Join(newlyEnabled, ", ")),
		}
	}

	return CheckResult{Name: name, Status: StatusOK, Message: "config valid"}
}

// --- check 2: state ---

func (s *Service) checkState() CheckResult {
	name := "state"
	_, err := store.LoadState(s.StateRoot)
	if err == nil {
		return CheckResult{Name: name, Status: StatusOK, Message: "state valid"}
	}
	// Reset to empty state.
	empty := store.State{Version: store.StateVersion}
	if saveErr := store.SaveState(s.StateRoot, empty); saveErr != nil {
		return CheckResult{Name: name, Status: StatusError, Message: saveErr.Error()}
	}
	return CheckResult{Name: name, Status: StatusFixed, Message: "state valid", Fix: "reset corrupt state"}
}

// --- check 3: installed-dirs ---

func (s *Service) checkInstalledDirs() CheckResult {
	name := "installed-dirs"
	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return CheckResult{Name: name, Status: StatusError, Message: err.Error()}
	}

	installedRoot := store.InstalledRoot(s.StateRoot)

	// Build set of dirs that should exist based on state.
	expectedDirs := map[string]struct{}{}
	for _, rec := range st.Installed {
		dirName := safeEntryName(rec.SkillRef) + "@" + rec.ResolvedVersion
		expectedDirs[dirName] = struct{}{}
	}

	// Check for orphan dirs (on disk but not in state).
	var orphans []string
	entries, _ := os.ReadDir(installedRoot)
	diskDirs := map[string]struct{}{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		diskDirs[e.Name()] = struct{}{}
		if _, ok := expectedDirs[e.Name()]; !ok {
			orphans = append(orphans, e.Name())
		}
	}

	// Check for ghost entries (in state but dir missing).
	var ghosts []string
	for _, rec := range st.Installed {
		dirName := safeEntryName(rec.SkillRef) + "@" + rec.ResolvedVersion
		if _, ok := diskDirs[dirName]; !ok {
			ghosts = append(ghosts, rec.SkillRef)
		}
	}

	if len(orphans) == 0 && len(ghosts) == 0 {
		return CheckResult{Name: name, Status: StatusOK, Message: "installed dirs reconciled"}
	}

	var fixes []string
	for _, o := range orphans {
		_ = os.RemoveAll(filepath.Join(installedRoot, o))
		fixes = append(fixes, "removed orphan dir: "+o)
	}
	for _, g := range ghosts {
		store.RemoveInstalled(&st, g)
		fixes = append(fixes, "removed ghost state entry: "+g)
	}
	if len(ghosts) > 0 {
		_ = store.SaveState(s.StateRoot, st)
	}

	return CheckResult{
		Name:    name,
		Status:  StatusFixed,
		Message: "installed dirs reconciled",
		Fix:     strings.Join(fixes, "; "),
	}
}

// --- check 4: injections ---

func (s *Service) checkInjections() CheckResult {
	name := "injections"
	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return CheckResult{Name: name, Status: StatusError, Message: err.Error()}
	}

	installedSet := map[string]struct{}{}
	for _, rec := range st.Installed {
		installedSet[rec.SkillRef] = struct{}{}
	}

	changed := false
	var fixes []string
	var kept []store.InjectionState
	for _, inj := range st.Injections {
		var valid []string
		for _, ref := range inj.Skills {
			if _, ok := installedSet[ref]; ok {
				valid = append(valid, ref)
			} else {
				fixes = append(fixes, fmt.Sprintf("removed stale ref %s from %s", ref, inj.Agent))
				changed = true
			}
		}
		if len(valid) == 0 {
			fixes = append(fixes, fmt.Sprintf("removed empty agent entry: %s", inj.Agent))
			changed = true
			continue
		}
		inj.Skills = valid
		kept = append(kept, inj)
	}

	if !changed {
		return CheckResult{Name: name, Status: StatusOK, Message: "injection refs valid"}
	}

	st.Injections = kept
	_ = store.SaveState(s.StateRoot, st)

	return CheckResult{
		Name:    name,
		Status:  StatusFixed,
		Message: "injection refs valid",
		Fix:     strings.Join(fixes, "; "),
	}
}

// --- check 5: adapter-state ---

func (s *Service) checkAdapterState() CheckResult {
	name := "adapter-state"
	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return CheckResult{Name: name, Status: StatusError, Message: err.Error()}
	}
	if s.Runtime == nil {
		return CheckResult{Name: name, Status: StatusOK, Message: "adapter state synced"}
	}

	ctx := context.Background()
	scope := string(s.Scope)
	var fixes []string
	for _, inj := range st.Injections {
		adp, aErr := s.Runtime.Get(inj.Agent)
		if aErr != nil {
			continue
		}
		listed, lErr := adp.ListInjected(ctx, adapterapi.ListInjectedRequest{Scope: scope})
		if lErr != nil {
			continue
		}
		if skillSetsEqual(inj.Skills, listed.Skills) {
			continue
		}
		// Re-inject to reconcile: remove all, then inject what state says.
		_, _ = adp.Remove(ctx, adapterapi.RemoveRequest{Scope: scope})
		if len(inj.Skills) > 0 {
			_, _ = adp.Inject(ctx, adapterapi.InjectRequest{SkillRefs: inj.Skills, Scope: scope})
		}
		fixes = append(fixes, fmt.Sprintf("%s: synced injected.toml", inj.Agent))
	}

	if len(fixes) == 0 {
		return CheckResult{Name: name, Status: StatusOK, Message: "adapter state synced"}
	}
	return CheckResult{
		Name:    name,
		Status:  StatusFixed,
		Message: "adapter state synced",
		Fix:     strings.Join(fixes, "; "),
	}
}

// --- check 6: agent-skills ---

func (s *Service) checkAgentSkills() CheckResult {
	name := "agent-skills"
	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return CheckResult{Name: name, Status: StatusError, Message: err.Error()}
	}

	var projectRoot string
	if s.Scope == config.ScopeProject {
		projectRoot = s.ProjectRoot
	}

	installedRoot := store.InstalledRoot(s.StateRoot)
	var fixes []string

	for _, inj := range st.Injections {
		skillsDir := adapter.AgentSkillsDirForScope(inj.Agent, projectRoot)
		for _, ref := range inj.Skills {
			skillName := adapter.ExtractSkillName(ref)
			destDir := filepath.Join(skillsDir, skillName)
			if _, statErr := os.Stat(destDir); statErr == nil {
				continue
			}
			srcDir := findInstalledDir(installedRoot, ref)
			if srcDir == "" {
				continue
			}
			if cpErr := copyDir(srcDir, destDir); cpErr == nil {
				fixes = append(fixes, fmt.Sprintf("restored %s for %s", skillName, inj.Agent))
			}
		}
	}

	if len(fixes) == 0 {
		return CheckResult{Name: name, Status: StatusOK, Message: "agent skill files present"}
	}
	return CheckResult{
		Name:    name,
		Status:  StatusFixed,
		Message: "agent skill files present",
		Fix:     strings.Join(fixes, "; "),
	}
}

// --- check 7: lockfile ---

func (s *Service) checkLockfile() CheckResult {
	name := "lockfile"
	if s.LockPath == "" {
		return CheckResult{Name: name, Status: StatusOK, Message: "no lockfile configured"}
	}

	lock, err := store.LoadLockfile(s.LockPath)
	if err != nil {
		return CheckResult{Name: name, Status: StatusWarn, Message: "lockfile unreadable: " + err.Error()}
	}

	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return CheckResult{Name: name, Status: StatusError, Message: err.Error()}
	}

	stateRefs := map[string]store.InstalledSkill{}
	for _, rec := range st.Installed {
		stateRefs[rec.SkillRef] = rec
	}
	lockRefs := map[string]struct{}{}
	for _, ls := range lock.Skills {
		lockRefs[ls.SkillRef] = struct{}{}
	}

	var fixes []string
	changed := false

	// Remove stale lock entries (in lock but not in state).
	var kept []store.LockSkill
	for _, ls := range lock.Skills {
		if _, ok := stateRefs[ls.SkillRef]; ok {
			kept = append(kept, ls)
		} else {
			fixes = append(fixes, "removed stale lock entry: "+ls.SkillRef)
			changed = true
		}
	}
	lock.Skills = kept

	// Add missing lock entries (in state but not in lock).
	for ref, rec := range stateRefs {
		if _, ok := lockRefs[ref]; !ok {
			store.UpsertLock(&lock, store.LockSkill{
				SkillRef:        ref,
				ResolvedVersion: rec.ResolvedVersion,
				Checksum:        rec.Checksum,
				SourceRef:       rec.SourceRef,
			})
			fixes = append(fixes, "added missing lock entry: "+ref)
			changed = true
		}
	}

	if !changed {
		count := len(lock.Skills)
		return CheckResult{Name: name, Status: StatusOK, Message: fmt.Sprintf("%d lock entries verified", count)}
	}

	if saveErr := store.SaveLockfile(s.LockPath, lock); saveErr != nil {
		return CheckResult{Name: name, Status: StatusError, Message: saveErr.Error()}
	}
	return CheckResult{
		Name:    name,
		Status:  StatusFixed,
		Message: fmt.Sprintf("%d lock entries verified", len(lock.Skills)),
		Fix:     strings.Join(fixes, "; "),
	}
}

// --- helpers ---

func safeEntryName(v string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "@", "_", " ", "-")
	out := r.Replace(v)
	if out == "" {
		return "unknown"
	}
	return out
}

func findInstalledDir(installedRoot, ref string) string {
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

// copyDir copies a directory tree, skipping metadata.toml.
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
		if rel == "metadata.toml" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, rErr := os.ReadFile(path)
		if rErr != nil {
			return rErr
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func skillSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := make([]string, len(a))
	copy(sa, a)
	sort.Strings(sa)
	sb := make([]string, len(b))
	copy(sb, b)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
