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
	"skillpm/internal/resolver"
	"skillpm/internal/security"
	"skillpm/internal/source"
	storepkg "skillpm/internal/store"
	syncsvc "skillpm/internal/sync"
	"skillpm/pkg/adapterapi"
)

type Options struct {
	ConfigPath string
	HTTPClient *http.Client
}

type Service struct {
	ConfigPath string
	Config     config.Config
	StateRoot  string

	SourceMgr *source.Manager
	Resolver  *resolver.Service
	Installer *installer.Service
	Runtime   *adapter.Runtime
	Harvest   *harvest.Service
	Sync      *syncsvc.Service
	Doctor    *doctor.Service
	Audit     *audit.Logger
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
	stateRoot, err := config.ResolveStorageRoot(cfg)
	if err != nil {
		return nil, err
	}
	if err := storepkg.EnsureLayout(stateRoot); err != nil {
		return nil, err
	}
	logger := audit.New(storepkg.AuditPath(stateRoot))
	sourceMgr := source.NewManager(opts.HTTPClient)
	resolverSvc := &resolver.Service{Sources: sourceMgr}
	securityEngine := security.New(cfg.Security)
	installerSvc := &installer.Service{Root: stateRoot, Security: securityEngine, Audit: logger}
	runtimeSvc, err := adapter.NewRuntime(stateRoot, cfg)
	if err != nil {
		return nil, err
	}
	harvestSvc := &harvest.Service{Runtime: runtimeSvc, StateRoot: stateRoot}
	syncService := &syncsvc.Service{
		Sources:   sourceMgr,
		Resolver:  resolverSvc,
		Installer: installerSvc,
		Runtime:   runtimeSvc,
		StateRoot: stateRoot,
	}
	doctorSvc := &doctor.Service{ConfigPath: configPath, StateRoot: stateRoot, Runtime: runtimeSvc}
	return &Service{
		ConfigPath: configPath,
		Config:     cfg,
		StateRoot:  stateRoot,
		SourceMgr:  sourceMgr,
		Resolver:   resolverSvc,
		Installer:  installerSvc,
		Runtime:    runtimeSvc,
		Harvest:    harvestSvc,
		Sync:       syncService,
		Doctor:     doctorSvc,
		Audit:      logger,
	}, nil
}

func (s *Service) SaveConfig() error {
	return config.Save(s.ConfigPath, s.Config)
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
	resolved, err := s.Resolver.ResolveMany(ctx, s.Config, refs, lock)
	if err != nil {
		return nil, err
	}
	return s.Installer.Install(ctx, resolved, lockPath, force)
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
	return s.Installer.Uninstall(ctx, skillRefs, s.resolveLockPath(lockPath))
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
	return s.Installer.Install(ctx, upgrades, lockPath, force)
}

func (s *Service) Inject(ctx context.Context, agentName string, refs []string) (adapterapi.InjectResult, error) {
	if refs == nil || len(refs) == 0 {
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
	res, err := adp.Inject(ctx, adapterapi.InjectRequest{SkillRefs: refs, Scope: "global"})
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
	return res, nil
}

func (s *Service) RemoveInjected(ctx context.Context, agentName string, refs []string) (adapterapi.RemoveResult, error) {
	adp, err := s.Runtime.Get(agentName)
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	res, err := adp.Remove(ctx, adapterapi.RemoveRequest{SkillRefs: refs, Scope: "global"})
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	st, err := storepkg.LoadState(s.StateRoot)
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	listed, err := adp.ListInjected(ctx, adapterapi.ListInjectedRequest{Scope: "global"})
	if err != nil {
		return adapterapi.RemoveResult{}, err
	}
	storepkg.SetInjection(&st, storepkg.InjectionState{Agent: agentName, Skills: listed.Skills, UpdatedAt: time.Now().UTC()})
	if err := storepkg.SaveState(s.StateRoot, st); err != nil {
		return adapterapi.RemoveResult{}, err
	}
	return res, nil
}

func (s *Service) SyncRun(ctx context.Context, lockPath string, force bool) (syncsvc.Report, error) {
	report, err := s.Sync.Run(ctx, &s.Config, s.resolveLockPath(lockPath), force)
	if err != nil {
		return syncsvc.Report{}, err
	}
	if err := s.SaveConfig(); err != nil {
		return syncsvc.Report{}, err
	}
	return report, nil
}

func (s *Service) Schedule(action, interval string) (config.SyncConfig, error) {
	switch action {
	case "install":
		s.Config.Sync.Mode = "system"
		if interval != "" {
			s.Config.Sync.Interval = interval
		}
	case "remove":
		s.Config.Sync.Mode = "off"
	case "list", "":
		// no-op
	default:
		return config.SyncConfig{}, fmt.Errorf("SYNC_SCHEDULE: unsupported action %q", action)
	}
	if err := s.SaveConfig(); err != nil {
		return config.SyncConfig{}, err
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

func (s *Service) SelfUpdate(_ context.Context, channel string) error {
	if channel == "" {
		channel = "stable"
	}
	return fmt.Errorf("SEC_SELF_UPDATE: channel %q is not implemented in v1 local build", channel)
}

func (s *Service) resolveLockPath(lockPath string) string {
	if lockPath != "" {
		return lockPath
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "skills.lock"
	}
	return filepath.Join(cwd, "skills.lock")
}
