package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillpm/internal/adapter"
	"skillpm/internal/audit"
	"skillpm/internal/config"
	"skillpm/internal/doctor"
	"skillpm/internal/harvest"
	"skillpm/internal/importer"
	"skillpm/internal/installer"
	"skillpm/internal/leaderboard"
	"skillpm/internal/memory"
	"skillpm/internal/memory/rules"
	"skillpm/internal/memory/scoring"
	"skillpm/internal/resolver"
	"skillpm/internal/scheduler"
	"skillpm/internal/security"
	"skillpm/internal/selfupdate"
	"skillpm/internal/source"
	storepkg "skillpm/internal/store"
	syncsvc "skillpm/internal/sync"
	"skillpm/pkg/adapterapi"
)

type Options struct {
	ConfigPath  string
	HTTPClient  *http.Client
	Scope       config.Scope
	ProjectRoot string
}

type Service struct {
	ConfigPath  string
	Config      config.Config
	StateRoot   string
	Scope       config.Scope
	ProjectRoot string
	Manifest    *config.ProjectManifest

	SourceMgr *source.Manager
	Resolver  *resolver.Service
	Installer *installer.Service
	Runtime   *adapter.Runtime
	Harvest   *harvest.Service
	Sync      *syncsvc.Service
	Doctor    *doctor.Service
	Audit     *audit.Logger
	Scheduler *scheduler.Manager
	Memory    *memory.Service

	httpClient *http.Client
}

func New(opts Options) (*Service, error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}
	cfg, err := config.Ensure(configPath)
	if err != nil {
		return nil, err
	}

	// Resolve scope
	scope := opts.Scope
	projectRoot := opts.ProjectRoot
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		cwd = "."
	}
	if scope == config.ScopeProject && projectRoot != "" {
		// Explicit scope + explicit root — skip auto-detection.
	} else if scope == "" {
		scope, projectRoot, _ = config.ResolveScope("", cwd)
	} else {
		scope, projectRoot, err = config.ResolveScope(string(scope), cwd)
		if err != nil {
			return nil, err
		}
	}

	// Determine stateRoot and load manifest based on scope
	var stateRoot string
	var manifest *config.ProjectManifest
	if scope == config.ScopeProject && projectRoot != "" {
		stateRoot = config.ProjectStateRoot(projectRoot)
		m, loadErr := config.LoadProjectManifest(projectRoot)
		if loadErr != nil {
			return nil, loadErr
		}
		manifest = &m
		cfg.Sources = config.MergedSources(cfg, m)
		cfg.Adapters = config.MergedAdapters(cfg, m)
	} else {
		stateRoot, err = config.ResolveStorageRoot(cfg)
		if err != nil {
			return nil, err
		}
	}

	if err := storepkg.EnsureLayout(stateRoot); err != nil {
		return nil, err
	}
	logger := audit.New(storepkg.AuditPath(stateRoot))
	sourceMgr := source.NewManager(opts.HTTPClient, stateRoot)
	resolverSvc := &resolver.Service{Sources: sourceMgr}
	securityEngine := security.New(cfg.Security)
	installerSvc := &installer.Service{Root: stateRoot, Security: securityEngine, Audit: logger}
	runtimeSvc, err := adapter.NewRuntime(stateRoot, cfg, projectRoot)
	if err != nil {
		return nil, err
	}
	harvestSvc := &harvest.Service{Runtime: runtimeSvc, StateRoot: stateRoot}
	syncService := &syncsvc.Service{
		Sources:     sourceMgr,
		Resolver:    resolverSvc,
		Installer:   installerSvc,
		Runtime:     runtimeSvc,
		StateRoot:   stateRoot,
		Security:    securityEngine,
		Manifest:    manifest,
		ProjectRoot: projectRoot,
	}
	lockPath := ""
	if scope == config.ScopeProject && projectRoot != "" {
		lockPath = config.ProjectLockPath(projectRoot)
	}
	doctorSvc := &doctor.Service{
		ConfigPath:     configPath,
		StateRoot:      stateRoot,
		LockPath:       lockPath,
		Runtime:        runtimeSvc,
		Scope:          scope,
		ProjectRoot:    projectRoot,
		MemoryEnabled:  cfg.Memory.Enabled,
		RulesInjection: cfg.Memory.RulesInjection,
		BridgeEnabled:  cfg.Memory.BridgeEnabled,
	}
	schedulerSvc := scheduler.New()
	// Build agent directory map for memory observation
	agentDirs := map[string]string{}
	if runtimeSvc != nil {
		for _, name := range runtimeSvc.AgentNames() {
			if dir := runtimeSvc.AgentSkillsDir(name); dir != "" {
				agentDirs[name] = dir
			}
		}
	}
	// Collect installed skill refs for observation v2
	st, _ := storepkg.LoadState(stateRoot)
	var skillRefs []string
	for _, rec := range st.Installed {
		skillRefs = append(skillRefs, rec.SkillRef)
	}
	memorySvc, memErr := memory.New(stateRoot, cfg.Memory, agentDirs, skillRefs...)
	if memErr != nil {
		return nil, memErr
	}
	return &Service{
		ConfigPath:  configPath,
		Config:      cfg,
		StateRoot:   stateRoot,
		Scope:       scope,
		ProjectRoot: projectRoot,
		Manifest:    manifest,
		SourceMgr:   sourceMgr,
		Resolver:    resolverSvc,
		Installer:   installerSvc,
		Runtime:     runtimeSvc,
		Harvest:     harvestSvc,
		Sync:        syncService,
		Doctor:      doctorSvc,
		Audit:       logger,
		Scheduler:   schedulerSvc,
		Memory:      memorySvc,
		httpClient:  opts.HTTPClient,
	}, nil
}

func (s *Service) SaveConfig() error {
	return config.Save(s.ConfigPath, s.Config)
}

// InitProject creates a new project manifest in the given directory.
func (s *Service) InitProject(dir string) (string, error) {
	return config.InitProject(dir)
}

// ListInstalled returns installed skills for the current scope.
func (s *Service) ListInstalled() ([]storepkg.InstalledSkill, error) {
	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return nil, err
	}
	return st.Installed, nil
}

// SaveManifest persists the project manifest (only valid for project scope).
func (s *Service) SaveManifest() error {
	if s.Scope != config.ScopeProject || s.Manifest == nil || s.ProjectRoot == "" {
		return nil
	}
	return config.SaveProjectManifest(s.ProjectRoot, *s.Manifest)
}

func (s *Service) SourceAdd(name, target, kind, branch, trustTier string) (config.SourceConfig, error) {
	if name == "" || target == "" {
		return config.SourceConfig{}, fmt.Errorf("SRC_ADD: name and target are required")
	}
	if kind == "" {
		if strings.Contains(target, "clawhub") {
			kind = "clawhub"
		} else {
			kind = "git"
		}
	}
	if trustTier == "" {
		trustTier = "review"
	}
	src := config.SourceConfig{Name: name, Kind: kind, TrustTier: trustTier}
	switch kind {
	case "git", "dir":
		src.URL = target
		src.Branch = branch
		if kind == "git" {
			src.ScanPaths = []string{"skills"}
		}
	case "clawhub":
		src.Site = target
		src.Registry = target
		src.WellKnown = []string{"/.well-known/clawhub.json", "/.well-known/clawdhub.json"}
		src.APIVersion = "v1"
	default:
		return config.SourceConfig{}, fmt.Errorf("SRC_ADD: unsupported source kind %q", kind)
	}
	if err := config.AddSource(&s.Config, src); err != nil {
		return config.SourceConfig{}, err
	}
	if err := s.SaveConfig(); err != nil {
		return config.SourceConfig{}, err
	}
	return src, nil
}

func (s *Service) SourceRemove(name string) error {
	if err := config.RemoveSource(&s.Config, name); err != nil {
		return err
	}
	return s.SaveConfig()
}

func (s *Service) SourceList() []config.SourceConfig {
	out := append([]config.SourceConfig{}, s.Config.Sources...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Service) SourceUpdate(ctx context.Context, name string) ([]source.UpdateResult, error) {
	updated, err := s.SourceMgr.Update(ctx, &s.Config, name)
	if err != nil {
		return nil, err
	}
	if err := s.SaveConfig(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) Search(ctx context.Context, sourceName, query string) ([]source.SearchResult, error) {
	return s.SourceMgr.Search(ctx, s.Config, sourceName, query)
}

func (s *Service) Install(ctx context.Context, refs []string, lockPath string, force bool) ([]storepkg.InstalledSkill, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("INS_INSTALL: at least one skill ref is required")
	}
	lockPath = s.resolveLockPath(lockPath)
	lock, err := storepkg.LoadLockfile(lockPath)
	if err != nil {
		return nil, err
	}

	configMutated := false
	for _, raw := range refs {
		pr, pErr := resolver.ParseRef(raw)
		if pErr == nil && pr.IsURL {
			if _, ok := config.FindSource(s.Config, pr.Source); !ok {
				newSrc := config.SourceConfig{
					Name:      pr.Source,
					Kind:      "git",
					URL:       pr.URL,
					Branch:    pr.Branch,
					ScanPaths: []string{"."},
					TrustTier: "review",
				}
				if err := config.AddSource(&s.Config, newSrc); err == nil {
					configMutated = true
				}
			}
		}
	}
	if configMutated {
		if err := s.SaveConfig(); err != nil {
			return nil, err
		}
	}

	resolved, err := s.Resolver.ResolveMany(ctx, s.Config, refs, lock)
	if err != nil {
		return nil, err
	}
	if err := s.scanResolved(ctx, resolved, force); err != nil {
		return nil, err
	}
	installed, installErr := s.Installer.Install(ctx, resolved, lockPath, force)
	if installErr != nil {
		return nil, installErr
	}

	// Update project manifest with installed skills.
	// Use resolved skills (not original refs) so that expanded scan-path
	// directories produce individual manifest entries.
	if s.Scope == config.ScopeProject && s.Manifest != nil {
		// Build constraint map from original refs.
		constraintMap := make(map[string]string)
		for _, raw := range refs {
			parsed, pErr := resolver.ParseRef(raw)
			if pErr != nil {
				continue
			}
			c := parsed.Constraint
			if c == "" {
				c = "latest"
			}
			constraintMap[parsed.Source+"/"+parsed.Skill] = c
		}
		for _, r := range resolved {
			constraint := "latest"
			if c, ok := constraintMap[r.SkillRef]; ok {
				constraint = c
			} else {
				// Inherit constraint from parent scan-path ref if expanded.
				for origRef, c := range constraintMap {
					if strings.HasPrefix(r.SkillRef, origRef+"/") {
						constraint = c
						break
					}
				}
			}
			config.UpsertManifestSkill(s.Manifest, config.ProjectSkillEntry{
				Ref:        r.SkillRef,
				Constraint: constraint,
			})
		}
		if err := s.SaveManifest(); err != nil {
			return installed, err
		}
	}
	return installed, nil
}

func (s *Service) Uninstall(ctx context.Context, refs []string, lockPath string) ([]string, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("INS_UNINSTALL: at least one skill ref is required")
	}
	skillRefs := make([]string, 0, len(refs))
	for _, raw := range refs {
		parsed, err := resolver.ParseRef(raw)
		if err != nil {
			return nil, err
		}
		skillRefs = append(skillRefs, parsed.Source+"/"+parsed.Skill)
	}
	removed, err := s.Installer.Uninstall(ctx, skillRefs, s.resolveLockPath(lockPath))
	if err != nil {
		return nil, err
	}
	// Clean up: remove uninstalled skills from all agents that had them injected.
	if len(removed) > 0 && s.Runtime != nil {
		st, stErr := storepkg.LoadState(s.StateRoot)
		if stErr == nil {
			for _, inj := range st.Injections {
				adp, adpErr := s.Runtime.Get(inj.Agent)
				if adpErr != nil {
					continue
				}
				_, _ = adp.Remove(ctx, adapterapi.RemoveRequest{SkillRefs: removed, Scope: string(s.Scope)})
			}
		}
	}

	// Update project manifest
	if s.Scope == config.ScopeProject && s.Manifest != nil && len(removed) > 0 {
		for _, ref := range removed {
			config.RemoveManifestSkill(s.Manifest, ref)
		}
		if err := s.SaveManifest(); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (s *Service) Upgrade(ctx context.Context, refs []string, lockPath string, force bool) ([]storepkg.InstalledSkill, error) {
	state, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return nil, err
	}
	if len(state.Installed) == 0 {
		return nil, nil
	}
	if len(refs) == 0 {
		for _, rec := range state.Installed {
			refs = append(refs, rec.SkillRef)
		}
	}
	cleanRefs := make([]string, 0, len(refs))
	for _, r := range refs {
		if strings.Contains(r, "@") {
			r = strings.SplitN(r, "@", 2)[0]
		}
		cleanRefs = append(cleanRefs, r)
	}
	lockPath = s.resolveLockPath(lockPath)
	lock, err := storepkg.LoadLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	resolved, err := s.Resolver.ResolveMany(ctx, s.Config, cleanRefs, lock)
	if err != nil {
		return nil, err
	}
	installedVersion := map[string]string{}
	for _, rec := range state.Installed {
		installedVersion[rec.SkillRef] = rec.ResolvedVersion
	}
	upgrades := make([]resolver.ResolvedSkill, 0, len(resolved))
	for _, rec := range resolved {
		if installedVersion[rec.SkillRef] != rec.ResolvedVersion {
			upgrades = append(upgrades, rec)
		}
	}
	if len(upgrades) == 0 {
		return nil, nil
	}
	if err := s.scanResolved(ctx, upgrades, force); err != nil {
		return nil, err
	}
	return s.Installer.Install(ctx, upgrades, lockPath, force)
}

func (s *Service) Inject(ctx context.Context, agentName string, refs []string) (adapterapi.InjectResult, error) {
	if len(refs) == 0 {
		st, err := storepkg.LoadState(s.StateRoot)
		if err != nil {
			return adapterapi.InjectResult{}, err
		}
		for _, item := range st.Installed {
			refs = append(refs, item.SkillRef)
		}
	}
	if len(refs) == 0 {
		return adapterapi.InjectResult{}, fmt.Errorf("ADP_INJECT: no installed skills to inject")
	}
	adp, err := s.Runtime.Get(agentName)
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
	res, err := adp.Inject(ctx, adapterapi.InjectRequest{SkillRefs: refs, Scope: string(s.Scope)})
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
	storepkg.SetInjection(&st, storepkg.InjectionState{Agent: agentName, Skills: res.Injected, UpdatedAt: time.Now().UTC()})
	if err := storepkg.SaveState(s.StateRoot, st); err != nil {
		return adapterapi.InjectResult{}, err
	}

	// Sync rules for Claude Code if rules injection is enabled
	if agentName == "claude" {
		s.syncRulesForSkills(refs)
	}

	return res, nil
}

// AdaptiveInject injects only the working-memory subset of skills for the given agent.
func (s *Service) AdaptiveInject(ctx context.Context, agentName string) (adapterapi.InjectResult, error) {
	if s.Memory == nil || !s.Memory.IsEnabled() {
		return s.Inject(ctx, agentName, nil)
	}
	cwd, _ := os.Getwd()
	profile, _ := s.Memory.Context.Detect(cwd)

	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return adapterapi.InjectResult{}, err
	}
	skills := make([]scoring.SkillInput, 0, len(st.Installed))
	for _, rec := range st.Installed {
		skills = append(skills, scoring.SkillInput{SkillRef: rec.SkillRef})
	}

	board, err := s.Memory.Scoring.Compute(skills, profile, s.Config.Memory.WorkingMemoryMax, s.Config.Memory.Threshold)
	if err != nil {
		return s.Inject(ctx, agentName, nil)
	}

	workingSet := scoring.WorkingSet(board)
	if len(workingSet) == 0 {
		return s.Inject(ctx, agentName, nil)
	}
	return s.Inject(ctx, agentName, workingSet)
}

func (s *Service) RemoveInjected(ctx context.Context, agentName string, refs []string) (adapterapi.RemoveResult, error) {
	adp, err := s.Runtime.Get(agentName)
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	res, err := adp.Remove(ctx, adapterapi.RemoveRequest{SkillRefs: refs, Scope: string(s.Scope)})
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	listed, err := adp.ListInjected(ctx, adapterapi.ListInjectedRequest{Scope: string(s.Scope)})
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	storepkg.SetInjection(&st, storepkg.InjectionState{Agent: agentName, Skills: listed.Skills, UpdatedAt: time.Now().UTC()})
	if err := storepkg.SaveState(s.StateRoot, st); err != nil {
		return adapterapi.RemoveResult{}, err
	}

	// Re-sync rules for Claude Code after removal
	if agentName == "claude" {
		s.syncRulesForSkills(listed.Skills)
	}

	return res, nil
}

func (s *Service) SyncRun(ctx context.Context, lockPath string, force bool, dryRun bool) (syncsvc.Report, error) {
	report, err := s.Sync.Run(ctx, &s.Config, s.resolveLockPath(lockPath), force, dryRun)
	if err != nil {
		return syncsvc.Report{}, err
	}
	if dryRun {
		return report, nil
	}
	if err := s.SaveConfig(); err != nil {
		return syncsvc.Report{}, err
	}
	return report, nil
}

func (s *Service) Schedule(action, interval string) (config.SyncConfig, error) {
	persist := false
	switch action {
	case "install":
		s.Config.Sync.Mode = "system"
		if interval != "" {
			s.Config.Sync.Interval = interval
		}
		if s.Scheduler != nil {
			if _, err := s.Scheduler.Install(context.Background(), s.Config.Sync.Interval); err != nil {
				return config.SyncConfig{}, err
			}
		}
		persist = true
	case "remove":
		s.Config.Sync.Mode = "off"
		if s.Scheduler != nil {
			if _, err := s.Scheduler.Remove(context.Background()); err != nil {
				return config.SyncConfig{}, err
			}
		}
		persist = true
	case "list", "":
		if s.Scheduler != nil {
			if _, err := s.Scheduler.List(); err != nil {
				return config.SyncConfig{}, err
			}
		}
	default:
		return config.SyncConfig{}, fmt.Errorf("SYNC_SCHEDULE: unsupported action %q", action)
	}
	if persist {
		if err := s.SaveConfig(); err != nil {
			return config.SyncConfig{}, err
		}
	}
	return s.Config.Sync, nil
}

func (s *Service) HarvestRun(ctx context.Context, agentName string) ([]harvest.InboxEntry, string, error) {
	return s.Harvest.Harvest(ctx, agentName)
}

func (s *Service) Validate(path string) error {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = cwd
	}
	_, err := importer.ValidateSkillDir(path)
	return err
}

func (s *Service) DoctorRun(ctx context.Context) doctor.Report {
	return s.Doctor.Run(ctx)
}

func (s *Service) DetectAdapters() []adapter.Detection {
	return adapter.DetectAvailable()
}

func (s *Service) EnableDetectedAdapters() ([]string, error) {
	detected := s.DetectAdapters()
	enabled := make([]string, 0, len(detected))
	changed := false
	for _, d := range detected {
		ok, err := config.EnableAdapter(&s.Config, d.Name, "global")
		if err != nil {
			return nil, err
		}
		if ok {
			changed = true
			enabled = append(enabled, d.Name)
		}
	}
	if !changed {
		sort.Strings(enabled)
		return enabled, nil
	}
	if err := s.SaveConfig(); err != nil {
		return nil, err
	}
	// Reload runtime-bound services so newly enabled adapters are active immediately.
	runtimeSvc, err := adapter.NewRuntime(s.StateRoot, s.Config, s.ProjectRoot)
	if err != nil {
		return nil, err
	}
	s.Runtime = runtimeSvc
	s.Harvest = &harvest.Service{Runtime: runtimeSvc, StateRoot: s.StateRoot}
	s.Sync.Runtime = runtimeSvc
	s.Doctor.Runtime = runtimeSvc
	sort.Strings(enabled)
	return enabled, nil
}

func (s *Service) SelfUpdate(ctx context.Context, channel string) error {
	if channel == "" {
		channel = "stable"
	}
	updater := selfupdate.New(s.httpClient)
	_, err := updater.Update(ctx, channel, s.Config.Security.RequireSignatures)
	return err
}

// Leaderboard returns trending skills filtered by category and limited to n entries.
func (s *Service) Leaderboard(category string, limit int) []leaderboard.Entry {
	return leaderboard.Get(leaderboard.Options{Category: category, Limit: limit})
}

func (s *Service) scanResolved(ctx context.Context, resolved []resolver.ResolvedSkill, force bool) error {
	if s.Installer.Security == nil || s.Installer.Security.Scanner == nil {
		return nil
	}
	contents := resolvedToScanContents(resolved)
	report := s.Installer.Security.Scanner.Scan(ctx, contents)
	if s.Audit != nil {
		_ = s.Audit.Log(audit.Event{
			Operation: "security_scan",
			Phase:     "complete",
			Status:    report.MaxSeverity().String(),
			Message:   fmt.Sprintf("skills=%d findings=%d max_severity=%s", len(resolved), len(report.Findings), report.MaxSeverity()),
		})
	}
	return s.Installer.Security.Scanner.Enforce(report, force)
}

func resolvedToScanContents(skills []resolver.ResolvedSkill) []security.SkillContent {
	out := make([]security.SkillContent, len(skills))
	for i, s := range skills {
		out[i] = security.SkillContent{
			SkillRef:  s.SkillRef,
			Content:   s.Content,
			Files:     s.Files,
			Source:    s.Source,
			TrustTier: s.TrustTier,
			Version:   s.ResolvedVersion,
		}
	}
	return out
}

// syncRulesForSkills syncs Claude Code rules for the given skill refs.
// This is a best-effort operation — errors are logged but not returned.
func (s *Service) syncRulesForSkills(skillRefs []string) {
	if s.Memory == nil || s.Memory.Rules == nil {
		return
	}
	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return
	}
	metas := make([]rules.SkillRuleMeta, 0, len(skillRefs))
	for _, ref := range skillRefs {
		skillName := adapter.ExtractSkillName(ref)
		content := readSkillContent(st, s.StateRoot, ref)
		if content == "" {
			continue
		}
		meta, ok := rules.ExtractRuleMeta(ref, skillName, content)
		if ok {
			metas = append(metas, meta)
		}
	}
	_ = s.Memory.Rules.Sync(metas)
}

// readSkillContent reads the SKILL.md content for an installed skill from pre-loaded state.
func readSkillContent(st storepkg.State, stateRoot, skillRef string) string {
	for _, rec := range st.Installed {
		if rec.SkillRef == skillRef {
			dirName := rec.SkillRef + "@" + rec.ResolvedVersion
			path := filepath.Join(storepkg.InstalledRoot(stateRoot), dirName, "SKILL.md")
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return string(data)
		}
	}
	return ""
}

func (s *Service) resolveLockPath(lockPath string) string {
	if lockPath != "" {
		return lockPath
	}
	if s.Scope == config.ScopeProject && s.ProjectRoot != "" {
		return config.ProjectLockPath(s.ProjectRoot)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "skills.lock"
	}
	return filepath.Join(cwd, "skills.lock")
}
