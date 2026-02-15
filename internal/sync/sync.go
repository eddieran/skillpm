package sync

import (
	"context"
	"fmt"
	"sort"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/installer"
	"skillpm/internal/resolver"
	"skillpm/internal/source"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

type Service struct {
	Sources   *source.Manager
	Resolver  *resolver.Service
	Installer *installer.Service
	Runtime   *adapter.Runtime
	StateRoot string
}

type Report struct {
	UpdatedSources   []string `json:"updatedSources"`
	UpgradedSkills   []string `json:"upgradedSkills"`
	Reinjected       []string `json:"reinjectedAgents"`
	SkippedReinjects []string `json:"skippedReinjects,omitempty"`
	FailedReinjects  []string `json:"failedReinjects,omitempty"`
	DryRun           bool     `json:"dryRun,omitempty"`
}

func (s *Service) Run(ctx context.Context, cfg *config.Config, lockPath string, force bool, dryRun bool) (Report, error) {
	if s.Sources == nil || s.Resolver == nil || s.Installer == nil {
		return Report{}, fmt.Errorf("SYNC_SETUP: sync dependencies not configured")
	}
	if cfg == nil {
		return Report{}, fmt.Errorf("DOC_CONFIG_MISSING: sync requires loaded config")
	}
	runCfg := cfg
	if dryRun && cfg != nil {
		cloned := cloneConfig(*cfg)
		runCfg = &cloned
	}
	updates, err := s.Sources.Update(ctx, runCfg, "")
	if err != nil {
		return Report{}, err
	}
	report := Report{DryRun: dryRun}
	for _, u := range updates {
		report.UpdatedSources = append(report.UpdatedSources, u.Source.Name)
	}

	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return Report{}, err
	}
	if len(st.Installed) == 0 {
		sort.Strings(report.UpdatedSources)
		return report, nil
	}

	refs := make([]string, 0, len(st.Installed))
	installedVersion := map[string]string{}
	for _, rec := range st.Installed {
		refs = append(refs, rec.SkillRef)
		installedVersion[rec.SkillRef] = rec.ResolvedVersion
	}
	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		return Report{}, err
	}
	resolved, err := s.Resolver.ResolveMany(ctx, *runCfg, refs, lock)
	if err != nil {
		return Report{}, err
	}
	upgrades := make([]resolver.ResolvedSkill, 0, len(resolved))
	for _, rec := range resolved {
		if installedVersion[rec.SkillRef] != rec.ResolvedVersion {
			upgrades = append(upgrades, rec)
			report.UpgradedSkills = append(report.UpgradedSkills, rec.SkillRef)
		}
	}
	if len(upgrades) > 0 && !dryRun {
		if _, err := s.Installer.Install(ctx, upgrades, lockPath, force); err != nil {
			return Report{}, err
		}
	}

	if dryRun {
		for _, inj := range st.Injections {
			report.Reinjected = append(report.Reinjected, inj.Agent)
		}
	} else if s.Runtime != nil {
		for _, inj := range st.Injections {
			adp, err := s.Runtime.Get(inj.Agent)
			if err != nil {
				report.FailedReinjects = append(report.FailedReinjects, fmt.Sprintf("%s (%s)", inj.Agent, err))
				continue
			}
			if _, err := adp.Inject(ctx, adapterapi.InjectRequest{SkillRefs: inj.Skills}); err != nil {
				report.FailedReinjects = append(report.FailedReinjects, fmt.Sprintf("%s (%s)", inj.Agent, err))
				continue
			}
			report.Reinjected = append(report.Reinjected, inj.Agent)
		}
	} else {
		for _, inj := range st.Injections {
			report.SkippedReinjects = append(report.SkippedReinjects, inj.Agent)
		}
	}
	for _, list := range [][]string{report.UpdatedSources, report.UpgradedSkills, report.Reinjected, report.SkippedReinjects, report.FailedReinjects} {
		sort.Strings(list)
	}
	return report, nil
}

func cloneConfig(cfg config.Config) config.Config {
	out := cfg
	out.Sources = make([]config.SourceConfig, len(cfg.Sources))
	for i, src := range cfg.Sources {
		cloned := src
		cloned.ScanPaths = append([]string(nil), src.ScanPaths...)
		cloned.WellKnown = append([]string(nil), src.WellKnown...)
		out.Sources[i] = cloned
	}
	out.Adapters = append([]config.AdapterConfig(nil), cfg.Adapters...)
	return out
}
