package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"skillpm/internal/app"
	"skillpm/internal/config"
	"skillpm/internal/leaderboard"
	"skillpm/internal/memory/eventlog"
	"skillpm/internal/memory/scoring"
	"skillpm/internal/store"
	syncsvc "skillpm/internal/sync"
)

type ExitCoder interface {
	ExitCode() int
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if ex, ok := err.(ExitCoder); ok {
			os.Exit(ex.ExitCode())
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var configPath string
	var jsonOutput bool
	var scopeFlag string

	newSvc := func() (*app.Service, error) {
		return app.New(app.Options{
			ConfigPath: configPath,
			Scope:      config.Scope(scopeFlag),
		})
	}

	cmd := &cobra.Command{
		Use:           "skillpm",
		Short:         "Local-first skill package manager for AI agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config file")
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output JSON")
	cmd.PersistentFlags().StringVar(&scopeFlag, "scope", "", "scope: global or project (auto-detected if omitted)")

	cmd.AddCommand(newSourceCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newSearchCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newInstallCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newUninstallCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newUpgradeCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newInjectCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newSyncCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newScheduleCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newDoctorCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newVersionCmd(&jsonOutput))
	cmd.AddCommand(newSelfCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newLeaderboardCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newInitCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newListCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newMemoryCmd(newSvc, &jsonOutput))

	cmd.CompletionOptions.DisableDefaultCmd = true
	return cmd
}

func newSourceCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var kind string
	var branch string
	var trustTier string

	sourceCmd := &cobra.Command{Use: "source", Short: "Manage skill sources"}

	addCmd := &cobra.Command{
		Use:   "add <name> <url-or-site>",
		Short: "Add source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			src, err := svc.SourceAdd(args[0], args[1], kind, branch, trustTier)
			if err != nil {
				return err
			}
			return print(*jsonOutput, src, fmt.Sprintf("added source %s (%s)", src.Name, src.Kind))
		},
	}
	addCmd.Flags().StringVar(&kind, "kind", "", "source kind: git|dir|clawhub")
	addCmd.Flags().StringVar(&branch, "branch", "main", "git branch")
	addCmd.Flags().StringVar(&trustTier, "trust-tier", "review", "trusted|review|untrusted")

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if err := svc.SourceRemove(args[0]); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]string{"removed": args[0]}, "removed source "+args[0])
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			sources := svc.SourceList()
			if *jsonOutput {
				return print(true, sources, "")
			}
			if len(sources) == 0 {
				fmt.Println("no sources configured")
				return nil
			}
			for _, s := range sources {
				target := s.URL
				if s.Kind == "clawhub" {
					target = s.Registry
				}
				fmt.Printf("- %s (%s) %s trust=%s\n", s.Name, s.Kind, target, s.TrustTier)
			}
			return nil
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update source metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			updated, err := svc.SourceUpdate(context.Background(), name)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, updated, "")
			}
			for _, u := range updated {
				fmt.Printf("updated %s: %s\n", u.Source.Name, u.Note)
			}
			return nil
		},
	}

	sourceCmd.AddCommand(addCmd, removeCmd, listCmd, updateCmd)
	return sourceCmd
}

func newSearchCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var sourceName string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search available skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			items, err := svc.Search(context.Background(), sourceName, args[0])
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, items, "")
			}
			if len(items) == 0 {
				fmt.Println("no results")
				return nil
			}
			for _, item := range items {
				fmt.Printf("- %s/%s: %s\n", item.Source, item.Slug, item.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceName, "source", "", "source name")
	return cmd
}

func newInstallCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var force bool
	var lockfile string
	cmd := &cobra.Command{
		Use:   "install <source/skill[@constraint]>...",
		Short: "Install skills",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !*jsonOutput {
				fmt.Printf("📦 Resolving and installing %d skill(s)...\n", len(args))
			}
			installed, err := svc.Install(context.Background(), args, lockfile, force)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, installed, "")
			}
			for _, item := range installed {
				fmt.Printf("installed %s@%s\n", item.SkillRef, item.ResolvedVersion)
				fmt.Printf("  -> %s\n", store.InstalledRoot(svc.StateRoot))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "allow suspicious skills")
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	return cmd
}

func newUninstallCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var lockfile string
	cmd := &cobra.Command{
		Use:   "uninstall <source/skill>...",
		Short: "Uninstall skills",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			removed, err := svc.Uninstall(context.Background(), args, lockfile)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, removed, "")
			}
			if len(removed) == 0 {
				fmt.Println("no skills removed")
				return nil
			}
			for _, ref := range removed {
				fmt.Printf("removed %s\n", ref)
			}
			fmt.Printf("  -> cleaned %s\n", store.InstalledRoot(svc.StateRoot))
			return nil
		},
	}
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	return cmd
}

func newUpgradeCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var force bool
	var lockfile string
	cmd := &cobra.Command{
		Use:   "upgrade [source/skill ...]",
		Short: "Upgrade installed skills",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			upgraded, err := svc.Upgrade(context.Background(), args, lockfile, force)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, upgraded, "")
			}
			if len(upgraded) == 0 {
				fmt.Println("no upgrades available")
				return nil
			}
			for _, item := range upgraded {
				fmt.Printf("upgraded %s@%s\n", item.SkillRef, item.ResolvedVersion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "allow suspicious skills")
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	return cmd
}

func newInjectCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var agentName string
	var allAgents bool
	var adaptive bool
	cmd := &cobra.Command{
		Use:   "inject [source/skill ...]",
		Short: "Inject selected skills to target agent(s)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentName == "" && !allAgents {
				return fmt.Errorf("either --agent or --all is required")
			}
			if agentName != "" && allAgents {
				return fmt.Errorf("cannot specify both --agent and --all")
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			var targets []string
			if allAgents {
				for _, a := range svc.Config.Adapters {
					if a.Enabled {
						targets = append(targets, a.Name)
					}
				}
			} else {
				targets = []string{agentName}
			}
			type agentResult struct {
				Agent    string `json:"agent"`
				Injected int    `json:"injected"`
			}
			var results []agentResult
			for _, target := range targets {
				if adaptive {
					r, aErr := svc.AdaptiveInject(context.Background(), target)
					if aErr != nil {
						return aErr
					}
					results = append(results, agentResult{Agent: target, Injected: len(r.Injected)})
					if !*jsonOutput {
						fmt.Printf("injected into %s (adaptive):\n", target)
						for _, ref := range r.Injected {
							if p, ok := r.InjectedPaths[ref]; ok {
								fmt.Printf("  %s -> %s\n", ref, p)
							} else {
								fmt.Printf("  %s\n", ref)
							}
						}
					}
					continue
				}
				r, iErr := svc.Inject(context.Background(), target, args)
				if iErr != nil {
					return iErr
				}
				results = append(results, agentResult{Agent: target, Injected: len(r.Injected)})
				if !*jsonOutput {
					fmt.Printf("injected into %s:\n", target)
					for _, ref := range r.Injected {
						if p, ok := r.InjectedPaths[ref]; ok {
							fmt.Printf("  %s -> %s\n", ref, p)
						} else {
							fmt.Printf("  %s\n", ref)
						}
					}
				}
			}
			if *jsonOutput {
				return print(true, results, "")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	cmd.Flags().BoolVar(&allAgents, "all", false, "inject into all enabled agents")
	cmd.Flags().BoolVar(&adaptive, "adaptive", false, "inject only skills in working memory (requires memory enabled)")
	return cmd
}

func newSyncCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var lockfile string
	var force bool
	var dryRun bool
	var strict bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile source updates with installed/injected state",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			report, err := svc.SyncRun(context.Background(), lockfile, force, dryRun)
			if err != nil {
				return err
			}
			if *jsonOutput {
				if err := print(true, buildSyncJSONSummary(report, strict), ""); err != nil {
					return err
				}
				issueCount := totalSyncIssues(report)
				if strict && issueCount > 0 {
					if dryRun {
						return &exitError{code: 2, msg: fmt.Sprintf("SYNC_RISK: sync plan includes %d risk items (strict mode)", issueCount)}
					}
					return &exitError{code: 2, msg: fmt.Sprintf("SYNC_RISK: sync completed with %d risk items (strict mode)", issueCount)}
				}
				return nil
			}
			if dryRun {
				totalActions := totalSyncActions(report)
				issueCount := totalSyncIssues(report)
				fmt.Printf("sync plan (dry-run): sources=%d upgrades=%d reinjected=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected))
				fmt.Printf("strict mode: %s\n", syncStrictStatus(strict))
				fmt.Printf("planned strict failure reason: %s\n", syncStrictFailureReason(report, strict))
				fmt.Printf("planned actions total: %d\n", totalActions)
				fmt.Printf("planned outcome: %s\n", syncOutcome(report))
				fmt.Printf("planned progress status: %s\n", syncProgressStatus(report))
				fmt.Printf("planned progress class: %s\n", syncProgressClass(report))
				fmt.Printf("planned progress hotspot: %s\n", syncProgressHotspot(report))
				fmt.Printf("planned progress focus: %s\n", syncProgressFocus(report))
				fmt.Printf("planned progress target: %s\n", syncProgressTarget(report))
				fmt.Printf("planned progress signal: %s\n", syncProgressSignal(report))
				fmt.Printf("planned actions breakdown: %s\n", syncActionBreakdown(report))
				fmt.Printf("planned action samples: sources=%s upgrades=%s reinjected=%s\n", summarizeTop(report.UpdatedSources, 3), summarizeTop(report.UpgradedSkills, 3), summarizeTop(report.Reinjected, 3))
				fmt.Printf("planned next action: %s\n", syncNextAction(report))
				fmt.Printf("planned primary action: %s\n", syncPrimaryAction(report))
				fmt.Printf("planned execution priority: %s\n", syncExecutionPriority(report))
				fmt.Printf("planned follow-up gate: %s\n", syncFollowUpGate(report))
				fmt.Printf("planned next step hint: %s\n", syncNextStepHint(report))
				fmt.Printf("planned recommended command: %s\n", syncRecommendedCommand(report))
				fmt.Printf("planned recommended commands: %s\n", strings.Join(syncRecommendedCommands(report), " -> "))
				fmt.Printf("planned recommended agent: %s\n", syncRecommendedAgent(report))
				fmt.Printf("planned summary line: %s\n", syncSummaryLine(report))
				fmt.Printf("planned noop reason: %s\n", syncNoopReason(report))
				fmt.Printf("planned can proceed: %t\n", issueCount == 0)
				fmt.Printf("planned next batch ready: %t\n", syncNextBatchReady(report))
				fmt.Printf("planned next batch blocker: %s\n", syncNextBatchBlocker(report))
				fmt.Printf("planned risk items total: %d\n", issueCount)
				fmt.Printf("planned risk status: %s\n", syncRiskStatus(report))
				fmt.Printf("planned risk level: %s\n", syncRiskLevel(report))
				fmt.Printf("planned risk class: %s\n", syncRiskClass(report))
				fmt.Printf("planned risk breakdown: %s\n", syncRiskBreakdown(report))
				riskInjectCommands := syncRiskInjectCommands(report)
				fmt.Printf("planned risk inject commands: %s\n", summarizeTop(riskInjectCommands, 3))
				riskAgents := syncRiskAgents(report)
				fmt.Printf("planned risk hotspot: %s\n", syncRiskHotspot(report))
				fmt.Printf("planned risk agents total: %d\n", len(riskAgents))
				fmt.Printf("planned risk agents: %s\n", summarizeTop(riskAgents, 3))
				fmt.Printf("planned risk samples: skipped=%s failed=%s\n", summarizeTop(report.SkippedReinjects, 3), summarizeTop(report.FailedReinjects, 3))
				if totalActions == 0 {
					fmt.Println("planned actions: none")
				}
				if len(report.UpdatedSources) == 0 {
					fmt.Println("planned source updates: none")
				} else {
					fmt.Printf("planned source updates: %s\n", joinSorted(report.UpdatedSources))
				}
				if len(report.UpgradedSkills) == 0 {
					fmt.Println("planned upgrades: none")
				} else {
					fmt.Printf("planned upgrades: %s\n", joinSorted(report.UpgradedSkills))
				}
				if len(report.Reinjected) == 0 {
					fmt.Println("planned reinjections: none")
				} else {
					fmt.Printf("planned reinjections: %s\n", joinSorted(report.Reinjected))
				}
				if len(report.SkippedReinjects) == 0 {
					fmt.Println("planned skipped reinjections: none")
				} else {
					fmt.Printf("planned skipped reinjections: %s\n", joinSorted(report.SkippedReinjects))
				}
				if len(report.FailedReinjects) == 0 {
					fmt.Println("planned failed reinjections: none")
				} else {
					fmt.Printf("planned failed reinjections: %s\n", joinSortedWith(report.FailedReinjects, "; "))
				}
				if strict && issueCount > 0 {
					return &exitError{code: 2, msg: fmt.Sprintf("SYNC_RISK: sync plan includes %d risk items (strict mode)", issueCount)}
				}
				return nil
			}
			totalActions := totalSyncActions(report)
			issueCount := totalSyncIssues(report)
			fmt.Printf("sync complete: sources=%d upgrades=%d reinjected=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected))
			fmt.Printf("strict mode: %s\n", syncStrictStatus(strict))
			fmt.Printf("applied strict failure reason: %s\n", syncStrictFailureReason(report, strict))
			fmt.Printf("applied actions total: %d\n", totalActions)
			fmt.Printf("applied outcome: %s\n", syncOutcome(report))
			fmt.Printf("applied progress status: %s\n", syncProgressStatus(report))
			fmt.Printf("applied progress class: %s\n", syncProgressClass(report))
			fmt.Printf("applied progress hotspot: %s\n", syncProgressHotspot(report))
			fmt.Printf("applied progress focus: %s\n", syncProgressFocus(report))
			fmt.Printf("applied progress target: %s\n", syncProgressTarget(report))
			fmt.Printf("applied progress signal: %s\n", syncProgressSignal(report))
			fmt.Printf("applied actions breakdown: %s\n", syncActionBreakdown(report))
			fmt.Printf("applied action samples: sources=%s upgrades=%s reinjected=%s\n", summarizeTop(report.UpdatedSources, 3), summarizeTop(report.UpgradedSkills, 3), summarizeTop(report.Reinjected, 3))
			fmt.Printf("applied next action: %s\n", syncNextAction(report))
			fmt.Printf("applied primary action: %s\n", syncPrimaryAction(report))
			fmt.Printf("applied execution priority: %s\n", syncExecutionPriority(report))
			fmt.Printf("applied follow-up gate: %s\n", syncFollowUpGate(report))
			fmt.Printf("applied next step hint: %s\n", syncNextStepHint(report))
			fmt.Printf("applied recommended command: %s\n", syncRecommendedCommand(report))
			fmt.Printf("applied recommended commands: %s\n", strings.Join(syncRecommendedCommands(report), " -> "))
			fmt.Printf("applied recommended agent: %s\n", syncRecommendedAgent(report))
			fmt.Printf("applied summary line: %s\n", syncSummaryLine(report))
			fmt.Printf("applied noop reason: %s\n", syncNoopReason(report))
			fmt.Printf("applied can proceed: %t\n", issueCount == 0)
			fmt.Printf("applied next batch ready: %t\n", syncNextBatchReady(report))
			fmt.Printf("applied next batch blocker: %s\n", syncNextBatchBlocker(report))
			fmt.Printf("applied risk items total: %d\n", issueCount)
			fmt.Printf("applied risk status: %s\n", syncRiskStatus(report))
			fmt.Printf("applied risk level: %s\n", syncRiskLevel(report))
			fmt.Printf("applied risk class: %s\n", syncRiskClass(report))
			fmt.Printf("applied risk breakdown: %s\n", syncRiskBreakdown(report))
			riskInjectCommands := syncRiskInjectCommands(report)
			fmt.Printf("applied risk inject commands: %s\n", summarizeTop(riskInjectCommands, 3))
			riskAgents := syncRiskAgents(report)
			fmt.Printf("applied risk hotspot: %s\n", syncRiskHotspot(report))
			fmt.Printf("applied risk agents total: %d\n", len(riskAgents))
			fmt.Printf("applied risk agents: %s\n", summarizeTop(riskAgents, 3))
			fmt.Printf("applied risk samples: skipped=%s failed=%s\n", summarizeTop(report.SkippedReinjects, 3), summarizeTop(report.FailedReinjects, 3))
			if totalActions == 0 {
				fmt.Println("applied actions: none")
			}
			if len(report.UpdatedSources) == 0 {
				fmt.Println("updated sources: none")
			} else {
				fmt.Printf("updated sources: %s\n", joinSorted(report.UpdatedSources))
			}
			if len(report.UpgradedSkills) == 0 {
				fmt.Println("upgraded skills: none")
			} else {
				fmt.Printf("upgraded skills: %s\n", joinSorted(report.UpgradedSkills))
			}
			if len(report.Reinjected) == 0 {
				fmt.Println("reinjected agents: none")
			} else {
				fmt.Printf("reinjected agents: %s\n", joinSorted(report.Reinjected))
			}
			if len(report.SkippedReinjects) == 0 {
				fmt.Println("skipped reinjections: none")
			} else {
				fmt.Printf("skipped reinjections: %s (runtime unavailable)\n", joinSorted(report.SkippedReinjects))
			}
			if len(report.FailedReinjects) == 0 {
				fmt.Println("failed reinjections: none")
			} else {
				fmt.Printf("failed reinjections: %s\n", joinSortedWith(report.FailedReinjects, "; "))
			}
			if strict && issueCount > 0 {
				return &exitError{code: 2, msg: fmt.Sprintf("SYNC_RISK: sync completed with %d risk items (strict mode)", issueCount)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	cmd.Flags().BoolVar(&force, "force", false, "allow suspicious skills")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned sync actions without mutating state/config")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail if sync encounters risks")
	return cmd
}

func newScheduleCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var scheduleInterval string
	scheduleCmd := &cobra.Command{
		Use:   "schedule [interval]",
		Short: "Manage scheduler settings",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			interval := scheduleInterval
			if len(args) == 1 {
				if interval != "" && interval != args[0] {
					return fmt.Errorf("SCH_INTERVAL_CONFLICT: use either positional interval or --interval")
				}
				interval = args[0]
			}
			if interval != "" {
				syncCfg, err := svc.Schedule("install", interval)
				if err != nil {
					return err
				}
				return print(*jsonOutput, syncCfg, fmt.Sprintf("schedule enabled interval=%s", syncCfg.Interval))
			}
			syncCfg, err := svc.Schedule("list", "")
			if err != nil {
				return err
			}
			return print(*jsonOutput, syncCfg, fmt.Sprintf("schedule mode=%s interval=%s", syncCfg.Mode, syncCfg.Interval))
		},
	}
	scheduleCmd.Flags().StringVar(&scheduleInterval, "interval", "", "scheduler interval (e.g. 15m)")

	var installInterval string
	installCmd := &cobra.Command{
		Use:   "install [interval]",
		Short: "Enable scheduler mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			interval := installInterval
			if len(args) == 1 {
				if interval != "" && interval != args[0] {
					return fmt.Errorf("SCH_INTERVAL_CONFLICT: use either positional interval or --interval")
				}
				interval = args[0]
			}
			syncCfg, err := svc.Schedule("install", interval)
			if err != nil {
				return err
			}
			return print(*jsonOutput, syncCfg, fmt.Sprintf("schedule enabled interval=%s", syncCfg.Interval))
		},
	}
	installCmd.Flags().StringVar(&installInterval, "interval", "", "scheduler interval (e.g. 15m)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show scheduler settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			syncCfg, err := svc.Schedule("list", "")
			if err != nil {
				return err
			}
			return print(*jsonOutput, syncCfg, fmt.Sprintf("schedule mode=%s interval=%s", syncCfg.Mode, syncCfg.Interval))
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Disable scheduler mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			syncCfg, err := svc.Schedule("remove", "")
			if err != nil {
				return err
			}
			return print(*jsonOutput, syncCfg, "schedule disabled")
		},
	}

	scheduleCmd.AddCommand(installCmd, listCmd, removeCmd)
	return scheduleCmd
}

func newDoctorCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run self-healing diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			report := svc.DoctorRun(context.Background())
			if *jsonOutput {
				return print(true, report, "")
			}
			for _, c := range report.Checks {
				fmt.Printf("[%-5s] %-16s %s\n", c.Status, c.Name, c.Message)
				if c.Fix != "" {
					fmt.Printf("  -> %s\n", c.Fix)
				}
			}
			fmt.Println()
			if report.Fixed == 0 && report.Warnings == 0 && report.Errors == 0 {
				fmt.Println("all checks passed")
			} else {
				parts := []string{}
				if report.Fixed > 0 {
					parts = append(parts, fmt.Sprintf("%d fixed", report.Fixed))
				}
				if report.Warnings > 0 {
					parts = append(parts, fmt.Sprintf("%d warnings", report.Warnings))
				}
				if report.Errors > 0 {
					parts = append(parts, fmt.Sprintf("%d errors", report.Errors))
				}
				fmt.Printf("done: %s\n", strings.Join(parts, ", "))
			}
			return nil
		},
	}
	return cmd
}

func newSelfCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	selfCmd := &cobra.Command{Use: "self", Short: "Manage skillpm itself"}
	var channel string
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update skillpm binary with verification",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if err := svc.SelfUpdate(context.Background(), channel); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]string{"channel": channel}, "updated")
		},
	}
	updateCmd.Flags().StringVar(&channel, "channel", "stable", "release channel")
	selfCmd.AddCommand(updateCmd)
	return selfCmd
}

const syncJSONSchemaVersion = "v1"

type syncJSONSummary struct {
	SchemaVersion       string             `json:"schemaVersion"`
	UpdatedSources      []string           `json:"updatedSources"`
	UpgradedSkills      []string           `json:"upgradedSkills"`
	Reinjected          []string           `json:"reinjectedAgents"`
	SkippedReinjects    []string           `json:"skippedReinjects"`
	FailedReinjects     []string           `json:"failedReinjects"`
	DryRun              bool               `json:"dryRun"`
	StrictMode          bool               `json:"strictMode"`
	StrictStatus        string             `json:"strictStatus"`
	StrictFailureReason string             `json:"strictFailureReason"`
	Mode                string             `json:"mode"`
	Outcome             string             `json:"outcome"`
	ProgressStatus      string             `json:"progressStatus"`
	ProgressClass       string             `json:"progressClass"`
	ProgressHotspot     string             `json:"progressHotspot"`
	ProgressFocus       string             `json:"progressFocus"`
	ProgressTarget      string             `json:"progressTarget"`
	ProgressSignal      string             `json:"progressSignal"`
	ActionBreakdown     string             `json:"actionBreakdown"`
	NextAction          string             `json:"nextAction"`
	PrimaryAction       string             `json:"primaryAction"`
	ExecutionPriority   string             `json:"executionPriority"`
	FollowUpGate        string             `json:"followUpGate"`
	NextStepHint        string             `json:"nextStepHint"`
	RecommendedCommand  string             `json:"recommendedCommand"`
	RecommendedCommands []string           `json:"recommendedCommands"`
	RecommendedAgent    string             `json:"recommendedAgent"`
	SummaryLine         string             `json:"summaryLine"`
	NoopReason          string             `json:"noopReason"`
	RiskStatus          string             `json:"riskStatus"`
	RiskLevel           string             `json:"riskLevel"`
	RiskClass           string             `json:"riskClass"`
	RiskBreakdown       string             `json:"riskBreakdown"`
	RiskInjectCommands  []string           `json:"riskInjectCommands"`
	RiskHotspot         string             `json:"riskHotspot"`
	RiskAgents          []string           `json:"riskAgents"`
	RiskAgentsTotal     int                `json:"riskAgentsTotal"`
	HasProgress         bool               `json:"hasProgress"`
	HasRisk             bool               `json:"hasRisk"`
	CanProceed          bool               `json:"canProceed"`
	NextBatchReady      bool               `json:"nextBatchReady"`
	NextBatchBlocker    string             `json:"nextBatchBlocker"`
	ActionCounts        syncJSONCounts     `json:"actionCounts"`
	RiskCounts          syncJSONRiskCounts `json:"riskCounts"`
	TopSamples          syncJSONTopSamples `json:"topSamples"`
}

type syncJSONCounts struct {
	Sources       int `json:"sources"`
	Upgrades      int `json:"upgrades"`
	Reinjected    int `json:"reinjected"`
	Skipped       int `json:"skipped"`
	Failed        int `json:"failed"`
	ProgressTotal int `json:"progressTotal"`
	RiskTotal     int `json:"riskTotal"`
	Total         int `json:"total"`
}

type syncJSONRiskCounts struct {
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

type syncJSONTopSamples struct {
	Sources    syncJSONSample `json:"sources"`
	Upgrades   syncJSONSample `json:"upgrades"`
	Reinjected syncJSONSample `json:"reinjected"`
	Skipped    syncJSONSample `json:"skipped"`
	Failed     syncJSONSample `json:"failed"`
}

type syncJSONSample struct {
	Items     []string `json:"items"`
	Remaining int      `json:"remaining"`
}

func buildSyncJSONSummary(report syncsvc.Report, strictMode bool) syncJSONSummary {
	progressTotal := totalSyncProgressActions(report)
	riskTotal := totalSyncIssues(report)
	riskAgents := syncRiskAgents(report)
	return syncJSONSummary{
		SchemaVersion:       syncJSONSchemaVersion,
		UpdatedSources:      sortedStringSlice(report.UpdatedSources),
		UpgradedSkills:      sortedStringSlice(report.UpgradedSkills),
		Reinjected:          sortedStringSlice(report.Reinjected),
		SkippedReinjects:    sortedStringSlice(report.SkippedReinjects),
		FailedReinjects:     sortedStringSlice(report.FailedReinjects),
		DryRun:              report.DryRun,
		StrictMode:          strictMode,
		StrictStatus:        syncStrictStatus(strictMode),
		StrictFailureReason: syncStrictFailureReason(report, strictMode),
		Mode:                syncMode(report),
		Outcome:             syncOutcome(report),
		ProgressStatus:      syncProgressStatus(report),
		ProgressClass:       syncProgressClass(report),
		ProgressHotspot:     syncProgressHotspot(report),
		ProgressFocus:       syncProgressFocus(report),
		ProgressTarget:      syncProgressTarget(report),
		ProgressSignal:      syncProgressSignal(report),
		ActionBreakdown:     syncActionBreakdown(report),
		NextAction:          syncNextAction(report),
		PrimaryAction:       syncPrimaryAction(report),
		ExecutionPriority:   syncExecutionPriority(report),
		FollowUpGate:        syncFollowUpGate(report),
		NextStepHint:        syncNextStepHint(report),
		RecommendedCommand:  syncRecommendedCommand(report),
		RecommendedCommands: syncRecommendedCommands(report),
		RecommendedAgent:    syncRecommendedAgent(report),
		SummaryLine:         syncSummaryLine(report),
		NoopReason:          syncNoopReason(report),
		RiskStatus:          syncRiskStatus(report),
		RiskLevel:           syncRiskLevel(report),
		RiskClass:           syncRiskClass(report),
		RiskBreakdown:       syncRiskBreakdown(report),
		RiskInjectCommands:  syncRiskInjectCommands(report),
		RiskHotspot:         syncRiskHotspot(report),
		RiskAgents:          riskAgents,
		RiskAgentsTotal:     len(riskAgents),
		HasProgress:         progressTotal > 0,
		HasRisk:             riskTotal > 0,
		CanProceed:          riskTotal == 0,
		NextBatchReady:      syncNextBatchReady(report),
		NextBatchBlocker:    syncNextBatchBlocker(report),
		ActionCounts: syncJSONCounts{
			Sources:       len(report.UpdatedSources),
			Upgrades:      len(report.UpgradedSkills),
			Reinjected:    len(report.Reinjected),
			Skipped:       len(report.SkippedReinjects),
			Failed:        len(report.FailedReinjects),
			ProgressTotal: progressTotal,
			RiskTotal:     riskTotal,
			Total:         progressTotal + riskTotal,
		},
		RiskCounts: syncJSONRiskCounts{
			Skipped: len(report.SkippedReinjects),
			Failed:  len(report.FailedReinjects),
			Total:   riskTotal,
		},
		TopSamples: syncJSONTopSamples{
			Sources:    topSample(report.UpdatedSources, 3),
			Upgrades:   topSample(report.UpgradedSkills, 3),
			Reinjected: topSample(report.Reinjected, 3),
			Skipped:    topSample(report.SkippedReinjects, 3),
			Failed:     topSample(report.FailedReinjects, 3),
		},
	}
}

func topSample(items []string, limit int) syncJSONSample {
	if limit <= 0 {
		limit = 1
	}
	sorted := sortedStringSlice(items)
	if len(sorted) <= limit {
		return syncJSONSample{Items: sorted}
	}
	return syncJSONSample{
		Items:     sorted[:limit],
		Remaining: len(sorted) - limit,
	}
}

func stableStringSlice(items []string) []string {
	out := make([]string, len(items))
	copy(out, items)
	return out
}

func sortedStringSlice(items []string) []string {
	sorted := stableStringSlice(items)
	sort.Strings(sorted)
	return sorted
}

func syncMode(report syncsvc.Report) string {
	if report.DryRun {
		return "dry-run"
	}
	return "apply"
}

func syncStrictStatus(strict bool) string {
	if strict {
		return "enabled"
	}
	return "disabled"
}

func syncStrictFailureReason(report syncsvc.Report, strictMode bool) string {
	if !strictMode {
		return "strict-disabled"
	}
	failed := len(report.FailedReinjects)
	skipped := len(report.SkippedReinjects)
	if failed > 0 && skipped > 0 {
		return "risk-present-mixed"
	}
	if failed > 0 {
		return "risk-present-failed"
	}
	if skipped > 0 {
		return "risk-present-skipped"
	}
	return "none"
}

func syncNextBatchReady(report syncsvc.Report) bool {
	return !report.DryRun && totalSyncIssues(report) == 0
}

func syncNextBatchBlocker(report syncsvc.Report) string {
	if syncNextBatchReady(report) {
		return "none"
	}
	if totalSyncIssues(report) > 0 {
		return "risk-present"
	}
	if report.DryRun {
		return "dry-run-mode"
	}
	return "unknown"
}

func totalSyncActions(report syncsvc.Report) int {
	return totalSyncProgressActions(report) + totalSyncIssues(report)
}

func totalSyncProgressActions(report syncsvc.Report) int {
	return len(report.UpdatedSources) + len(report.UpgradedSkills) + len(report.Reinjected)
}

func totalSyncIssues(report syncsvc.Report) int {
	return len(report.SkippedReinjects) + len(report.FailedReinjects)
}

func syncProgressStatus(report syncsvc.Report) string {
	if totalSyncProgressActions(report) > 0 {
		return "progress-made"
	}
	return "no-progress"
}

func syncProgressClass(report syncsvc.Report) string {
	hasSourceUpdates := len(report.UpdatedSources) > 0
	hasUpgrades := len(report.UpgradedSkills) > 0
	hasReinjections := len(report.Reinjected) > 0

	switch {
	case hasReinjections:
		return "reinjection"
	case hasUpgrades:
		return "upgrade"
	case hasSourceUpdates:
		return "source-refresh"
	default:
		return "none"
	}
}

func syncActionBreakdown(report syncsvc.Report) string {
	return fmt.Sprintf("sources=%d upgrades=%d reinjected=%d skipped=%d failed=%d", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected), len(report.SkippedReinjects), len(report.FailedReinjects))
}

func syncProgressHotspot(report syncsvc.Report) string {
	if len(report.UpgradedSkills) > 0 {
		return sortedStringSlice(report.UpgradedSkills)[0]
	}
	if len(report.Reinjected) > 0 {
		return sortedStringSlice(report.Reinjected)[0]
	}
	if len(report.UpdatedSources) > 0 {
		return sortedStringSlice(report.UpdatedSources)[0]
	}
	return "none"
}

func syncProgressFocus(report syncsvc.Report) string {
	if len(report.Reinjected) > 0 {
		return sortedStringSlice(report.Reinjected)[0]
	}
	if len(report.UpgradedSkills) > 0 {
		return sortedStringSlice(report.UpgradedSkills)[0]
	}
	if len(report.UpdatedSources) > 0 {
		return sortedStringSlice(report.UpdatedSources)[0]
	}
	return "none"
}

func syncProgressTarget(report syncsvc.Report) string {
	if totalSyncProgressActions(report) == 0 {
		return "none"
	}
	if syncProgressClass(report) == "reinjection" {
		return syncProgressFocus(report)
	}
	return syncProgressHotspot(report)
}

func syncProgressSignal(report syncsvc.Report) string {
	if totalSyncProgressActions(report) == 0 {
		return "none"
	}
	return fmt.Sprintf("%s:%s", syncProgressClass(report), syncProgressTarget(report))
}

func syncOutcome(report syncsvc.Report) string {
	if totalSyncActions(report) == 0 {
		return "noop"
	}
	issues := totalSyncIssues(report)
	progress := totalSyncProgressActions(report)
	if progress == 0 && issues > 0 {
		return "blocked"
	}
	if progress > 0 && issues > 0 {
		return "changed-with-risk"
	}
	return "changed"
}

func syncNextAction(report syncsvc.Report) string {
	switch syncOutcome(report) {
	case "noop":
		if report.DryRun {
			return "plan-next-iteration"
		}
		return "monitor"
	case "blocked":
		if len(report.FailedReinjects) > 0 {
			if report.DryRun {
				return "resolve-failures-then-apply"
			}
			return "resolve-reinjection-failures"
		}
		if report.DryRun {
			return "resolve-skips-then-apply"
		}
		return "resolve-reinjection-skips"
	case "changed-with-risk":
		if len(report.FailedReinjects) > 0 {
			if report.DryRun {
				return "resolve-failures-then-apply-plan"
			}
			return "review-failed-risk-items"
		}
		if report.DryRun {
			return "resolve-skips-then-apply-plan"
		}
		return "review-skipped-risk-items"
	default:
		if report.DryRun {
			return "apply-plan"
		}
		return "verify-and-continue"
	}
}

func syncPrimaryAction(report syncsvc.Report) string {
	switch syncOutcome(report) {
	case "noop":
		if report.DryRun {
			return "No changes detected; queue the next iteration to keep momentum."
		}
		return "No changes detected; keep monitoring and retry on the next cycle."
	case "blocked":
		if report.DryRun {
			return "Sync plan is blocked by reinjection risk; resolve skipped/failed agents before applying changes."
		}
		return "Reinjection is blocked; resolve skipped/failed agents first before adding new work."
	case "changed-with-risk":
		if len(report.FailedReinjects) > 0 {
			if report.DryRun {
				return "Sync plan includes progress with failed reinjections; clear failures before applying this iteration."
			}
			return "Progress landed with failed reinjections; fix failures before expanding scope."
		}
		if report.DryRun {
			return "Sync plan includes progress with skipped reinjections; clear skips before applying this iteration."
		}
		return "Progress landed with skipped reinjections; clear skips before expanding scope."
	default:
		if report.DryRun {
			return "Apply this sync plan to convert planned progress into committed state."
		}
		return "Progress is applied and clear; move directly to the next feature increment."
	}
}

func syncExecutionPriority(report syncsvc.Report) string {
	if totalSyncIssues(report) > 0 {
		if len(report.FailedReinjects) > 0 {
			return "stabilize-failures"
		}
		return "stabilize-risks"
	}
	if totalSyncProgressActions(report) > 0 {
		if report.DryRun {
			return "apply-feature-iteration"
		}
		return "feature-iteration"
	}
	if report.DryRun {
		return "plan-feature-iteration"
	}
	return "monitor-next-cycle"
}

func syncFollowUpGate(report syncsvc.Report) string {
	if totalSyncIssues(report) > 0 {
		return "blocked-by-risk"
	}
	if totalSyncProgressActions(report) > 0 {
		if report.DryRun {
			return "ready-to-apply"
		}
		return "ready-for-next-iteration"
	}
	if report.DryRun {
		return "plan-next-iteration"
	}
	return "monitor-next-cycle"
}

func syncNextStepHint(report syncsvc.Report) string {
	if totalSyncIssues(report) > 0 {
		if len(report.FailedReinjects) > 0 {
			return "reinject-failed-agents"
		}
		return "reinject-skipped-agents"
	}
	if report.DryRun {
		if totalSyncProgressActions(report) > 0 {
			return "apply-sync-plan"
		}
		return "queue-feature-iteration"
	}
	if totalSyncProgressActions(report) > 0 {
		return "start-next-feature-iteration"
	}
	return "wait-next-sync-cycle"
}

func syncRecommendedCommand(report syncsvc.Report) string {
	switch syncOutcome(report) {
	case "noop":
		if report.DryRun {
			return "skillpm sync"
		}
		return "skillpm sync --dry-run"
	case "blocked", "changed-with-risk":
		if totalSyncIssues(report) == 0 {
			if report.DryRun {
				return "skillpm sync"
			}
			return "skillpm sync --dry-run"
		}
		agent := syncRecommendedAgent(report)
		if agent != "" && agent != "none" {
			return fmt.Sprintf("skillpm inject --agent %s <skill-ref>", agent)
		}
		return "skillpm inject --agent <agent> <skill-ref>"
	default:
		if report.DryRun {
			return "skillpm sync"
		}
		return "skillpm source list"
	}
}

func syncRecommendedCommands(report syncsvc.Report) []string {
	commands := []string{syncRecommendedCommand(report)}
	hasIssues := totalSyncIssues(report) > 0
	hasProgress := totalSyncProgressActions(report) > 0
	if hasIssues {
		commands = append(commands, syncRiskInjectCommands(report)...)
		commands = append(commands, "skillpm source list")
		if hasProgress && !report.DryRun {
			commands = append(commands, "go test ./...")
		}
		commands = append(commands, "skillpm sync --dry-run")
		if report.DryRun {
			commands = append(commands, "skillpm sync")
			if hasProgress {
				commands = append(commands, "go test ./...")
			}
		}
		return uniqueNonEmpty(commands)
	}
	if hasProgress {
		commands = append(commands, "skillpm source list", "go test ./...", "skillpm sync --dry-run")
		return uniqueNonEmpty(commands)
	}
	commands = append(commands, "skillpm source list")
	return uniqueNonEmpty(commands)
}

func syncRiskInjectCommands(report syncsvc.Report) []string {
	agents := syncRiskAgents(report)
	commands := make([]string, 0, len(agents))
	for _, agent := range agents {
		commands = append(commands, fmt.Sprintf("skillpm inject --agent %s <skill-ref>", agent))
	}
	return commands
}

func uniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func syncRecommendedAgent(report syncsvc.Report) string {
	for _, item := range sortedStringSlice(report.FailedReinjects) {
		agent := riskAgentName(item)
		if agent != "" {
			return agent
		}
	}
	for _, item := range sortedStringSlice(report.SkippedReinjects) {
		agent := riskAgentName(item)
		if agent != "" {
			return agent
		}
	}
	return "none"
}

func riskAgentName(item string) string {
	agent := strings.TrimSpace(item)
	if agent == "" {
		return ""
	}
	if idx := strings.Index(agent, ":"); idx >= 0 {
		agent = strings.TrimSpace(agent[:idx])
	}
	if idx := strings.Index(agent, " "); idx >= 0 {
		agent = strings.TrimSpace(agent[:idx])
	}
	if idx := strings.Index(agent, "("); idx >= 0 {
		agent = strings.TrimSpace(agent[:idx])
	}
	return agent
}

func syncSummaryLine(report syncsvc.Report) string {
	return fmt.Sprintf("outcome=%s progress=%d risk=%d mode=%s", syncOutcome(report), totalSyncProgressActions(report), totalSyncIssues(report), syncMode(report))
}

func syncNoopReason(report syncsvc.Report) string {
	if syncOutcome(report) != "noop" {
		return "not-applicable"
	}
	if report.DryRun {
		return "dry-run detected no source/upgrade/reinjection deltas"
	}
	return "no source updates, skill upgrades, or reinjection changes detected"
}

func syncRiskBreakdown(report syncsvc.Report) string {
	return fmt.Sprintf("skipped=%d failed=%d", len(report.SkippedReinjects), len(report.FailedReinjects))
}

func syncRiskStatus(report syncsvc.Report) string {
	if totalSyncIssues(report) > 0 {
		return "attention-needed"
	}
	return "clear"
}

func syncRiskLevel(report syncsvc.Report) string {
	if len(report.FailedReinjects) > 0 {
		return "high"
	}
	if len(report.SkippedReinjects) > 0 {
		return "medium"
	}
	return "none"
}

func syncRiskClass(report syncsvc.Report) string {
	failed := len(report.FailedReinjects) > 0
	skipped := len(report.SkippedReinjects) > 0
	switch {
	case failed && skipped:
		return "mixed"
	case failed:
		return "failed-only"
	case skipped:
		return "skipped-only"
	default:
		return "none"
	}
}

func syncRiskHotspot(report syncsvc.Report) string {
	if len(report.FailedReinjects) > 0 {
		return sortedStringSlice(report.FailedReinjects)[0]
	}
	if len(report.SkippedReinjects) > 0 {
		return sortedStringSlice(report.SkippedReinjects)[0]
	}
	return "none"
}

func syncRiskAgents(report syncsvc.Report) []string {
	agents := make([]string, 0, len(report.FailedReinjects)+len(report.SkippedReinjects))
	seen := map[string]struct{}{}
	for _, item := range report.FailedReinjects {
		agent := riskAgentName(item)
		if agent == "" {
			continue
		}
		if _, ok := seen[agent]; ok {
			continue
		}
		seen[agent] = struct{}{}
		agents = append(agents, agent)
	}
	for _, item := range report.SkippedReinjects {
		agent := riskAgentName(item)
		if agent == "" {
			continue
		}
		if _, ok := seen[agent]; ok {
			continue
		}
		seen[agent] = struct{}{}
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	return agents
}

func joinSorted(items []string) string {
	return joinSortedWith(items, ", ")
}

func summarizeTop(items []string, limit int) string {
	if len(items) == 0 {
		return "none"
	}
	if limit <= 0 {
		limit = 1
	}
	sorted := append([]string(nil), items...)
	sort.Strings(sorted)
	if len(sorted) <= limit {
		return strings.Join(sorted, ", ")
	}
	return fmt.Sprintf("%s ... (+%d more)", strings.Join(sorted[:limit], ", "), len(sorted)-limit)
}

func joinSortedWith(items []string, sep string) string {
	copied := append([]string(nil), items...)
	sort.Strings(copied)
	return strings.Join(copied, sep)
}

func newLeaderboardCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var category string
	var limit int
	cmd := &cobra.Command{
		Use:   "leaderboard",
		Short: "Show trending skills",
		Long:  "Display a ranked leaderboard of the most popular and trending skills across all categories.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if category != "" && !leaderboard.IsValidCategory(category) {
				return fmt.Errorf("LB_CATEGORY: invalid category %q (valid: %s)",
					category, strings.Join(leaderboard.ValidCategories(), ", "))
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			entries := svc.Leaderboard(category, limit)
			if *jsonOutput {
				return print(true, entries, "")
			}
			if len(entries) == 0 {
				fmt.Println("no entries found")
				return nil
			}

			// header
			fmt.Println()
			if category != "" {
				fmt.Printf("🏆 Skill Leaderboard — %s\n", strings.ToUpper(category))
			} else {
				fmt.Println("🏆 Skill Leaderboard")
			}
			fmt.Println()

			// column headers
			fmt.Printf("  %-3s  %-26s %-10s %10s  %s\n",
				"#", "SKILL", "CATEGORY", "⬇ DLs", "INSTALL COMMAND")
			fmt.Println("  " + strings.Repeat("─", 85))

			for _, e := range entries {
				medal := fmt.Sprintf("%-3d", e.Rank)
				switch e.Rank {
				case 1:
					medal = "🥇 "
				case 2:
					medal = "🥈 "
				case 3:
					medal = "🥉 "
				}

				installCmd := fmt.Sprintf("skillpm install %s/%s", e.Source, e.Slug)
				if e.Source == "" {
					installCmd = fmt.Sprintf("skillpm install %s", e.Slug)
				}

				fmt.Printf("  %s  %-26s %-10s %10s  %s\n",
					medal, e.Slug, e.Category,
					formatDownloads(e.Downloads), installCmd)
			}

			fmt.Println()
			fmt.Printf("  Showing %d entries", len(entries))
			if category != "" {
				fmt.Printf(" in category %q", category)
			}
			fmt.Printf(" • Categories: %s\n", strings.Join(leaderboard.ValidCategories(), ", "))
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "filter by category (agent, tool, workflow, data, security)")
	cmd.Flags().IntVar(&limit, "limit", 15, "maximum entries to show")
	return cmd
}

func formatDownloads(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{s[start:i]}, parts...)
	}
	return strings.Join(parts, ",")
}

func print(jsonOutput bool, payload any, message string) error {
	if jsonOutput {
		blob, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(blob))
		return nil
	}
	if message != "" {
		fmt.Println(message)
	}
	return nil
}

func newInitCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a project for skillpm",
		Long:  "Creates a .skillpm/skills.toml project manifest in the current directory.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			path, err := config.InitProject(cwd)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, map[string]string{
					"manifest":             path,
					"gitignore_suggestion": ".skillpm/installed/\n.skillpm/state.toml\n.skillpm/staging/\n.skillpm/snapshots/",
				}, "")
			}
			fmt.Printf("initialized project at %s\n", path)
			fmt.Println("\nadd to .gitignore:")
			fmt.Println("  .skillpm/installed/")
			fmt.Println("  .skillpm/state.toml")
			fmt.Println("  .skillpm/staging/")
			fmt.Println("  .skillpm/snapshots/")
			return nil
		},
	}
}

func newListCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			installed, err := svc.ListInstalled()
			if err != nil {
				return err
			}
			if *jsonOutput {
				type listEntry struct {
					SkillRef string `json:"skillRef"`
					Version  string `json:"version"`
					Scope    string `json:"scope"`
				}
				entries := make([]listEntry, len(installed))
				for i, item := range installed {
					entries[i] = listEntry{
						SkillRef: item.SkillRef,
						Version:  item.ResolvedVersion,
						Scope:    string(svc.Scope),
					}
				}
				return print(true, entries, "")
			}
			if len(installed) == 0 {
				scope := string(svc.Scope)
				if scope == "" {
					scope = "global"
				}
				fmt.Printf("no installed skills (%s)\n", scope)
				return nil
			}
			scope := string(svc.Scope)
			if scope == "" {
				scope = "global"
			}
			header := "GLOBAL"
			if svc.Scope == config.ScopeProject {
				header = fmt.Sprintf("PROJECT (%s)", svc.ProjectRoot)
			}
			fmt.Printf("%s:\n", header)
			fmt.Printf("  state: %s\n", svc.StateRoot)
			for _, item := range installed {
				fmt.Printf("  %s@%s\n", item.SkillRef, item.ResolvedVersion)
			}
			return nil
		},
	}
}

func newMemoryCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage procedural memory for skill activation",
	}

	// --- enable ---
	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Enable the memory subsystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			svc.Config.Memory.Enabled = true
			if err := svc.SaveConfig(); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]string{"status": "enabled"}, "memory enabled")
		},
	})

	// --- disable ---
	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable the memory subsystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			svc.Config.Memory.Enabled = false
			if err := svc.SaveConfig(); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]string{"status": "disabled"}, "memory disabled")
		},
	})

	// --- observe ---
	cmd.AddCommand(&cobra.Command{
		Use:   "observe",
		Short: "Scan agent skill directories for usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !svc.Config.Memory.Enabled {
				return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
			}
			events, err := svc.Memory.Observer.ScanAll()
			if err != nil {
				return err
			}
			type observeResult struct {
				EventCount int `json:"event_count"`
			}
			return print(*jsonOutput, observeResult{EventCount: len(events)}, fmt.Sprintf("observed %d events", len(events)))
		},
	})

	// --- events ---
	{
		var since string
		var skill string
		eventsCmd := &cobra.Command{
			Use:   "events",
			Short: "Show recorded usage events",
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if !svc.Config.Memory.Enabled {
					return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
				}
				filter := eventlog.QueryFilter{SkillRef: skill}
				if since != "" {
					d, pErr := parseDuration(since)
					if pErr != nil {
						return pErr
					}
					filter.Since = time.Now().UTC().Add(-d)
				}
				events, err := svc.Memory.EventLog.Query(filter)
				if err != nil {
					return err
				}
				if *jsonOutput {
					return print(true, events, "")
				}
				if len(events) == 0 {
					fmt.Println("no events recorded")
					return nil
				}
				fmt.Printf("%-20s %-15s %-10s %s\n", "TIMESTAMP", "SKILL", "AGENT", "KIND")
				for _, ev := range events {
					fmt.Printf("%-20s %-15s %-10s %s\n",
						ev.Timestamp.Format("2006-01-02 15:04:05"),
						ev.SkillRef, ev.Agent, ev.Kind)
				}
				return nil
			},
		}
		eventsCmd.Flags().StringVar(&since, "since", "", "filter events since duration (e.g. 7d, 24h)")
		eventsCmd.Flags().StringVar(&skill, "skill", "", "filter by skill ref")
		cmd.AddCommand(eventsCmd)
	}

	// --- stats ---
	{
		var since string
		statsCmd := &cobra.Command{
			Use:   "stats",
			Short: "Show per-skill usage statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if !svc.Config.Memory.Enabled {
					return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
				}
				sinceTime := time.Time{}
				if since != "" {
					d, pErr := parseDuration(since)
					if pErr != nil {
						return pErr
					}
					sinceTime = time.Now().UTC().Add(-d)
				}
				stats, err := svc.Memory.EventLog.Stats(sinceTime)
				if err != nil {
					return err
				}
				if *jsonOutput {
					return print(true, stats, "")
				}
				if len(stats) == 0 {
					fmt.Println("no usage data")
					return nil
				}
				fmt.Printf("%-25s %-8s %-20s %s\n", "SKILL", "COUNT", "LAST ACCESS", "AGENTS")
				for _, s := range stats {
					fmt.Printf("%-25s %-8d %-20s %s\n",
						s.SkillRef, s.EventCount,
						s.LastAccess.Format("2006-01-02 15:04:05"),
						strings.Join(s.Agents, ","))
				}
				return nil
			},
		}
		statsCmd.Flags().StringVar(&since, "since", "", "stats since duration (e.g. 7d)")
		cmd.AddCommand(statsCmd)
	}

	// --- context ---
	{
		var dir string
		ctxCmd := &cobra.Command{
			Use:   "context",
			Short: "Detect current project context",
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if dir == "" {
					dir, _ = os.Getwd()
				}
				profile, err := svc.Memory.Context.Detect(dir)
				if err != nil {
					return err
				}
				if *jsonOutput {
					return print(true, profile, "")
				}
				fmt.Printf("Project Type: %s\n", profile.ProjectType)
				fmt.Printf("Build System: %s\n", profile.BuildSystem)
				if len(profile.Frameworks) > 0 {
					fmt.Printf("Frameworks:   %s\n", strings.Join(profile.Frameworks, ", "))
				}
				if len(profile.TaskSignals) > 0 {
					fmt.Printf("Task Signals: %s\n", strings.Join(profile.TaskSignals, ", "))
				}
				return nil
			},
		}
		ctxCmd.Flags().StringVar(&dir, "dir", "", "directory to analyze")
		cmd.AddCommand(ctxCmd)
	}

	// --- scores ---
	cmd.AddCommand(&cobra.Command{
		Use:   "scores",
		Short: "Show activation scores for all skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !svc.Config.Memory.Enabled {
				return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
			}
			cwd, _ := os.Getwd()
			profile, _ := svc.Memory.Context.Detect(cwd)
			st, err := store.LoadState(svc.StateRoot)
			if err != nil {
				return err
			}
			skills := make([]scoring.SkillInput, 0, len(st.Installed))
			for _, rec := range st.Installed {
				skills = append(skills, scoring.SkillInput{SkillRef: rec.SkillRef})
			}
			board, err := svc.Memory.Scoring.Compute(skills, profile, svc.Config.Memory.WorkingMemoryMax, svc.Config.Memory.Threshold)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, board, "")
			}
			if len(board.Scores) == 0 {
				fmt.Println("no scores computed")
				return nil
			}
			fmt.Printf("%-25s %-8s %-6s %-6s %-6s %-6s %s\n", "SKILL", "SCORE", "R", "F", "C", "FB", "STATUS")
			for _, s := range board.Scores {
				status := " "
				if s.InWorkingMemory {
					status = "active"
				}
				fmt.Printf("%-25s %-8.3f %-6.2f %-6.2f %-6.2f %-6.2f %s\n",
					s.SkillRef, s.ActivationLevel,
					s.Recency, s.Frequency, s.ContextMatch, s.FeedbackBoost, status)
			}
			return nil
		},
	})

	// --- working-set ---
	cmd.AddCommand(&cobra.Command{
		Use:   "working-set",
		Short: "Show skills currently in working memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !svc.Config.Memory.Enabled {
				return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
			}
			cwd, _ := os.Getwd()
			profile, _ := svc.Memory.Context.Detect(cwd)
			st, stErr := store.LoadState(svc.StateRoot)
			if stErr != nil {
				return stErr
			}
			skills := make([]scoring.SkillInput, 0, len(st.Installed))
			for _, rec := range st.Installed {
				skills = append(skills, scoring.SkillInput{SkillRef: rec.SkillRef})
			}
			board, err := svc.Memory.Scoring.Compute(skills, profile, svc.Config.Memory.WorkingMemoryMax, svc.Config.Memory.Threshold)
			if err != nil {
				return err
			}
			ws := scoring.WorkingSet(board)
			if *jsonOutput {
				return print(true, ws, "")
			}
			if len(ws) == 0 {
				fmt.Println("working set is empty")
				return nil
			}
			for _, ref := range ws {
				fmt.Println(ref)
			}
			return nil
		},
	})

	// --- explain ---
	cmd.AddCommand(&cobra.Command{
		Use:   "explain [skill-ref]",
		Short: "Explain activation score for a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !svc.Config.Memory.Enabled {
				return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
			}
			ref := args[0]
			cwd, _ := os.Getwd()
			profile, _ := svc.Memory.Context.Detect(cwd)
			skills := []scoring.SkillInput{{SkillRef: ref}}
			board, err := svc.Memory.Scoring.Compute(skills, profile, svc.Config.Memory.WorkingMemoryMax, svc.Config.Memory.Threshold)
			if err != nil {
				return err
			}
			if len(board.Scores) == 0 {
				return fmt.Errorf("MEM_SCORE_COMPUTE: no score for %s", ref)
			}
			s := board.Scores[0]
			if *jsonOutput {
				return print(true, s, "")
			}
			fmt.Printf("Skill:           %s\n", s.SkillRef)
			fmt.Printf("Activation:      %.3f\n", s.ActivationLevel)
			fmt.Printf("  Recency:       %.3f (weight 0.35)\n", s.Recency)
			fmt.Printf("  Frequency:     %.3f (weight 0.25)\n", s.Frequency)
			fmt.Printf("  Context Match: %.3f (weight 0.25)\n", s.ContextMatch)
			fmt.Printf("  Feedback:      %.3f (weight 0.15)\n", s.FeedbackBoost)
			if s.InWorkingMemory {
				fmt.Println("Status:          in working memory")
			} else {
				fmt.Println("Status:          inactive")
			}
			return nil
		},
	})

	// --- rate ---
	{
		var reason string
		rateCmd := &cobra.Command{
			Use:   "rate [skill-ref] [1-5]",
			Short: "Rate a skill (1=poor, 5=excellent)",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if !svc.Config.Memory.Enabled {
					return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
				}
				ref := args[0]
				rating := 0
				fmt.Sscanf(args[1], "%d", &rating)
				if rating < 1 || rating > 5 {
					return fmt.Errorf("MEM_FEEDBACK_RANGE: rating must be 1-5")
				}
				if err := svc.Memory.Feedback.Rate(ref, "", rating, reason); err != nil {
					return err
				}
				return print(*jsonOutput, map[string]interface{}{"skill": ref, "rating": rating}, fmt.Sprintf("rated %s: %d/5", ref, rating))
			},
		}
		rateCmd.Flags().StringVar(&reason, "reason", "", "reason for rating")
		cmd.AddCommand(rateCmd)
	}

	// --- feedback ---
	{
		var since string
		fbCmd := &cobra.Command{
			Use:   "feedback",
			Short: "Show feedback signals",
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if !svc.Config.Memory.Enabled {
					return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
				}
				sinceTime := time.Time{}
				if since != "" {
					d, pErr := parseDuration(since)
					if pErr != nil {
						return pErr
					}
					sinceTime = time.Now().UTC().Add(-d)
				}
				signals, err := svc.Memory.Feedback.QuerySignals(sinceTime)
				if err != nil {
					return err
				}
				if *jsonOutput {
					return print(true, signals, "")
				}
				if len(signals) == 0 {
					fmt.Println("no feedback signals")
					return nil
				}
				fmt.Printf("%-20s %-20s %-10s %-8s %s\n", "TIMESTAMP", "SKILL", "KIND", "RATING", "REASON")
				for _, s := range signals {
					fmt.Printf("%-20s %-20s %-10s %-8.2f %s\n",
						s.Timestamp.Format("2006-01-02 15:04:05"),
						s.SkillRef, s.Kind, s.Rating, s.Reason)
				}
				return nil
			},
		}
		fbCmd.Flags().StringVar(&since, "since", "", "feedback since duration")
		cmd.AddCommand(fbCmd)
	}

	// --- consolidate ---
	{
		var dryRun bool
		consCmd := &cobra.Command{
			Use:   "consolidate",
			Short: "Run memory consolidation",
			RunE: func(cmd *cobra.Command, args []string) error {
				svc, err := newSvc()
				if err != nil {
					return err
				}
				if !svc.Config.Memory.Enabled {
					return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
				}
				cwd, _ := os.Getwd()
				profile, _ := svc.Memory.Context.Detect(cwd)
				st, stErr := store.LoadState(svc.StateRoot)
				if stErr != nil {
					return stErr
				}
				skills := make([]scoring.SkillInput, 0, len(st.Installed))
				for _, rec := range st.Installed {
					skills = append(skills, scoring.SkillInput{SkillRef: rec.SkillRef})
				}
				if dryRun {
					board, cErr := svc.Memory.Scoring.Compute(skills, profile, svc.Config.Memory.WorkingMemoryMax, svc.Config.Memory.Threshold)
					if cErr != nil {
						return cErr
					}
					return print(*jsonOutput, board, fmt.Sprintf("dry-run: %d skills scored", len(board.Scores)))
				}
				stats, cErr := svc.Memory.Consolidation.Consolidate(context.Background(), skills, profile, svc.Config.Memory.WorkingMemoryMax, svc.Config.Memory.Threshold)
				if cErr != nil {
					return cErr
				}
				// Write rankings to Claude Code memory bridge (best-effort)
				if svc.Memory.Bridge != nil {
					if board := svc.Memory.Consolidation.LoadScores(); board != nil {
						_ = svc.Memory.Bridge.WriteRankings(board)
					}
				}
				return print(*jsonOutput, stats, fmt.Sprintf("consolidated: %d skills evaluated", stats.SkillsEvaluated))
			},
		}
		consCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without persisting")
		cmd.AddCommand(consCmd)
	}

	// --- recommend ---
	cmd.AddCommand(&cobra.Command{
		Use:   "recommend",
		Short: "Show skill recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if !svc.Config.Memory.Enabled {
				return fmt.Errorf("MEM_DISABLED: memory not enabled; run 'skillpm memory enable'")
			}
			recs, err := svc.Memory.Consolidation.Recommend()
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, recs, "")
			}
			if len(recs) == 0 {
				fmt.Println("no recommendations")
				return nil
			}
			for _, r := range recs {
				fmt.Printf("[%s] %s - %s (score: %.2f)\n", r.Kind, r.Skill, r.Reason, r.Score)
			}
			return nil
		},
	})

	// --- set-adaptive ---
	cmd.AddCommand(&cobra.Command{
		Use:   "set-adaptive [on|off]",
		Short: "Toggle adaptive injection mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			switch args[0] {
			case "on":
				svc.Config.Memory.AdaptiveInject = true
			case "off":
				svc.Config.Memory.AdaptiveInject = false
			default:
				return fmt.Errorf("MEM_ADAPTIVE_CONFIG: use 'on' or 'off'")
			}
			if err := svc.SaveConfig(); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]bool{"adaptive_inject": svc.Config.Memory.AdaptiveInject},
				fmt.Sprintf("adaptive injection: %s", args[0]))
		},
	})

	// --- purge ---
	cmd.AddCommand(&cobra.Command{
		Use:   "purge",
		Short: "Delete all memory data",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if err := svc.Memory.Purge(); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]string{"status": "purged"}, "memory data purged")
		},
	})

	return cmd
}

// parseDuration parses human-friendly durations like "7d", "24h", "30m".
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		days := 0
		fmt.Sscanf(s, "%d", &days)
		if days > 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}
