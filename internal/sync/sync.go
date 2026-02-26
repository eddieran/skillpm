package sync

import (
	"context"
	"fmt"
	"sort"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/installer"
	"skillpm/internal/resolver"
	"skillpm/internal/security"
	"skillpm/internal/source"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

type Service struct {
	Sources     *source.Manager
	Resolver    *resolver.Service
	Installer   *installer.Service
	Runtime     *adapter.Runtime
	StateRoot   string
	Security    *security.Engine
	Manifest    *config.ProjectManifest
	ProjectRoot string
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
	seenSources := map[string]struct{}{}
	for _, u := range updates {
		appendUnique(&report.UpdatedSources, seenSources, u.Source.Name)
	}

	st, err := store.LoadState(s.StateRoot)
	if err != nil {
		return Report{}, err
	}

	// Determine refs to sync: from manifest (project scope) or state (global scope).
	var refs []string
	installedVersion := map[string]string{}
	if s.Manifest != nil && len(s.Manifest.Skills) > 0 {
		for _, skill := range s.Manifest.Skills {
			refs = append(refs, skill.Ref)
		}
	} else {
		for _, rec := range st.Installed {
			refs = append(refs, rec.SkillRef)
		}
	}
	if len(refs) == 0 {
		sort.Strings(report.UpdatedSources)
		return report, nil
	}
	for _, rec := range st.Installed {
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
	seenUpgrades := map[string]struct{}{}
	for _, rec := range resolved {
		if installedVersion[rec.SkillRef] != rec.ResolvedVersion {
			upgrades = append(upgrades, rec)
			appendUnique(&report.UpgradedSkills, seenUpgrades, rec.SkillRef)
		}
	}
	if len(upgrades) > 0 && !dryRun {
		if s.Security != nil && s.Security.Scanner != nil {
			contents := resolvedToScanContents(upgrades)
			scanReport := s.Security.Scanner.Scan(ctx, contents)
			if err := s.Security.Scanner.Enforce(scanReport, force); err != nil {
				return Report{}, err
			}
		}
		if _, err := s.Installer.Install(ctx, upgrades, lockPath, force); err != nil {
			return Report{}, err
		}
	}

	seenReinjected := map[string]struct{}{}
	seenSkipped := map[string]struct{}{}
	if dryRun {
		if s.Runtime != nil {
			for _, inj := range st.Injections {
				if _, err := s.Runtime.Get(inj.Agent); err != nil {
					report.FailedReinjects = append(report.FailedReinjects, fmt.Sprintf("%s (%s)", inj.Agent, err))
					continue
				}
				appendUnique(&report.Reinjected, seenReinjected, inj.Agent)
			}
		} else {
			for _, inj := range st.Injections {
				appendUnique(&report.SkippedReinjects, seenSkipped, inj.Agent)
			}
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
			appendUnique(&report.Reinjected, seenReinjected, inj.Agent)
		}
	} else {
		for _, inj := range st.Injections {
			appendUnique(&report.SkippedReinjects, seenSkipped, inj.Agent)
		}
	}
	sort.Strings(report.UpdatedSources)
	sort.Strings(report.UpgradedSkills)
	sort.Strings(report.Reinjected)
	sort.Strings(report.SkippedReinjects)
	sort.Strings(report.FailedReinjects)
	return report, nil
}

func appendUnique(target *[]string, seen map[string]struct{}, value string) {
	if _, ok := seen[value]; ok {
		return
	}
	seen[value] = struct{}{}
	*target = append(*target, value)
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
