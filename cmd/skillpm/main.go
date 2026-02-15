package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"skillpm/internal/app"
	syncsvc "skillpm/internal/sync"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var configPath string
	var jsonOutput bool

	newSvc := func() (*app.Service, error) {
		return app.New(app.Options{ConfigPath: configPath})
	}

	cmd := &cobra.Command{
		Use:           "skillpm",
		Short:         "Local-first skill package manager for AI agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config file")
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output JSON")

	cmd.AddCommand(newSourceCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newSearchCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newInstallCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newUninstallCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newUpgradeCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newInjectCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newRemoveCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newSyncCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newScheduleCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newHarvestCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newValidateCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newDoctorCmd(newSvc, &jsonOutput))
	cmd.AddCommand(newSelfCmd(newSvc, &jsonOutput))

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
			installed, err := svc.Install(context.Background(), args, lockfile, force)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, installed, "")
			}
			for _, item := range installed {
				fmt.Printf("installed %s@%s\n", item.SkillRef, item.ResolvedVersion)
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
			fmt.Println("removed:", strings.Join(removed, ", "))
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
	cmd := &cobra.Command{
		Use:   "inject [source/skill ...]",
		Short: "Inject selected skills to target agent",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentName == "" {
				return fmt.Errorf("ADP_INJECT: --agent is required")
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			res, err := svc.Inject(context.Background(), agentName, args)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, res, "")
			}
			fmt.Printf("injected %d skill(s) into %s\n", len(res.Injected), agentName)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	return cmd
}

func newRemoveCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var agentName string
	cmd := &cobra.Command{
		Use:   "remove [source/skill ...]",
		Short: "Remove injected skills from target agent",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentName == "" {
				return fmt.Errorf("ADP_REMOVE: --agent is required")
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			res, err := svc.RemoveInjected(context.Background(), agentName, args)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, res, "")
			}
			fmt.Printf("removed %d skill(s) from %s\n", len(res.Removed), agentName)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	return cmd
}

func newSyncCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var lockfile string
	var force bool
	var dryRun bool
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
				return print(true, buildSyncJSONSummary(report), "")
			}
			if dryRun {
				totalActions := totalSyncActions(report)
				issueCount := totalSyncIssues(report)
				fmt.Printf("sync plan (dry-run): sources=%d upgrades=%d reinjected=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected))
				fmt.Printf("planned actions total: %d\n", totalActions)
				fmt.Printf("planned outcome: %s\n", syncOutcome(report))
				fmt.Printf("planned progress status: %s\n", syncProgressStatus(report))
				fmt.Printf("planned progress hotspot: %s\n", syncProgressHotspot(report))
				fmt.Printf("planned actions breakdown: %s\n", syncActionBreakdown(report))
				fmt.Printf("planned action samples: sources=%s upgrades=%s reinjected=%s\n", summarizeTop(report.UpdatedSources, 3), summarizeTop(report.UpgradedSkills, 3), summarizeTop(report.Reinjected, 3))
				fmt.Printf("planned next action: %s\n", syncNextAction(report))
				fmt.Printf("planned primary action: %s\n", syncPrimaryAction(report))
				fmt.Printf("planned risk items total: %d\n", issueCount)
				fmt.Printf("planned risk status: %s\n", syncRiskStatus(report))
				fmt.Printf("planned risk level: %s\n", syncRiskLevel(report))
				fmt.Printf("planned risk breakdown: %s\n", syncRiskBreakdown(report))
				fmt.Printf("planned risk hotspot: %s\n", syncRiskHotspot(report))
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
				return nil
			}
			totalActions := totalSyncActions(report)
			issueCount := totalSyncIssues(report)
			fmt.Printf("sync complete: sources=%d upgrades=%d reinjected=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected))
			fmt.Printf("applied actions total: %d\n", totalActions)
			fmt.Printf("applied outcome: %s\n", syncOutcome(report))
			fmt.Printf("applied progress status: %s\n", syncProgressStatus(report))
			fmt.Printf("applied progress hotspot: %s\n", syncProgressHotspot(report))
			fmt.Printf("applied actions breakdown: %s\n", syncActionBreakdown(report))
			fmt.Printf("applied action samples: sources=%s upgrades=%s reinjected=%s\n", summarizeTop(report.UpdatedSources, 3), summarizeTop(report.UpgradedSkills, 3), summarizeTop(report.Reinjected, 3))
			fmt.Printf("applied next action: %s\n", syncNextAction(report))
			fmt.Printf("primary action: %s\n", syncPrimaryAction(report))
			fmt.Printf("risk items total: %d\n", issueCount)
			fmt.Printf("risk status: %s\n", syncRiskStatus(report))
			fmt.Printf("risk level: %s\n", syncRiskLevel(report))
			fmt.Printf("risk breakdown: %s\n", syncRiskBreakdown(report))
			fmt.Printf("risk hotspot: %s\n", syncRiskHotspot(report))
			fmt.Printf("risk samples: skipped=%s failed=%s\n", summarizeTop(report.SkippedReinjects, 3), summarizeTop(report.FailedReinjects, 3))
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
			return nil
		},
	}
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	cmd.Flags().BoolVar(&force, "force", false, "allow suspicious skills")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned sync actions without mutating state/config")
	return cmd
}

func newScheduleCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	scheduleCmd := &cobra.Command{Use: "schedule", Short: "Manage scheduler settings"}

	installCmd := &cobra.Command{
		Use:   "install [interval]",
		Short: "Enable scheduler mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			interval := ""
			if len(args) == 1 {
				interval = args[0]
			}
			syncCfg, err := svc.Schedule("install", interval)
			if err != nil {
				return err
			}
			return print(*jsonOutput, syncCfg, fmt.Sprintf("schedule enabled interval=%s", syncCfg.Interval))
		},
	}

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

func newHarvestCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var agentName string
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Harvest candidate skills from agent side",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentName == "" {
				return fmt.Errorf("HRV_AGENT_REQUIRED: --agent is required")
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			entries, path, err := svc.HarvestRun(context.Background(), agentName)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, map[string]any{"entries": entries, "inbox": path}, "")
			}
			fmt.Printf("harvested %d candidates -> %s\n", len(entries), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	return cmd
}

func newValidateCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate skill package shape and policy basics",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if err := svc.Validate(path); err != nil {
				return err
			}
			return print(*jsonOutput, map[string]any{"valid": true}, "validation passed")
		},
	}
	return cmd
}

func newDoctorCmd(newSvc func() (*app.Service, error), jsonOutput *bool) *cobra.Command {
	var enableDetected bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			if enableDetected {
				enabled, err := svc.EnableDetectedAdapters()
				if err != nil {
					return err
				}
				if !*jsonOutput && len(enabled) > 0 {
					fmt.Printf("enabled detected adapters: %s\n", strings.Join(enabled, ", "))
				}
			}
			report := svc.DoctorRun(context.Background())
			if *jsonOutput {
				return print(true, report, "")
			}
			if report.Healthy {
				fmt.Println("healthy")
				return nil
			}
			fmt.Println("issues found:")
			for _, f := range report.Findings {
				fmt.Printf("- [%s] %s\n", f.Code, f.Message)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&enableDetected, "enable-detected", false, "enable detected adapters in config")
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

type syncJSONSummary struct {
	UpdatedSources   []string           `json:"updatedSources"`
	UpgradedSkills   []string           `json:"upgradedSkills"`
	Reinjected       []string           `json:"reinjectedAgents"`
	SkippedReinjects []string           `json:"skippedReinjects"`
	FailedReinjects  []string           `json:"failedReinjects"`
	DryRun           bool               `json:"dryRun"`
	Mode             string             `json:"mode"`
	Outcome          string             `json:"outcome"`
	ProgressStatus   string             `json:"progressStatus"`
	ProgressHotspot  string             `json:"progressHotspot"`
	ActionBreakdown  string             `json:"actionBreakdown"`
	NextAction       string             `json:"nextAction"`
	PrimaryAction    string             `json:"primaryAction"`
	RiskStatus       string             `json:"riskStatus"`
	RiskLevel        string             `json:"riskLevel"`
	RiskBreakdown    string             `json:"riskBreakdown"`
	RiskHotspot      string             `json:"riskHotspot"`
	HasProgress      bool               `json:"hasProgress"`
	HasRisk          bool               `json:"hasRisk"`
	ActionCounts     syncJSONCounts     `json:"actionCounts"`
	RiskCounts       syncJSONRiskCounts `json:"riskCounts"`
	TopSamples       syncJSONTopSamples `json:"topSamples"`
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

func buildSyncJSONSummary(report syncsvc.Report) syncJSONSummary {
	progressTotal := totalSyncProgressActions(report)
	riskTotal := totalSyncIssues(report)
	return syncJSONSummary{
		UpdatedSources:   sortedStringSlice(report.UpdatedSources),
		UpgradedSkills:   sortedStringSlice(report.UpgradedSkills),
		Reinjected:       sortedStringSlice(report.Reinjected),
		SkippedReinjects: sortedStringSlice(report.SkippedReinjects),
		FailedReinjects:  sortedStringSlice(report.FailedReinjects),
		DryRun:           report.DryRun,
		Mode:             syncMode(report),
		Outcome:          syncOutcome(report),
		ProgressStatus:   syncProgressStatus(report),
		ProgressHotspot:  syncProgressHotspot(report),
		ActionBreakdown:  syncActionBreakdown(report),
		NextAction:       syncNextAction(report),
		PrimaryAction:    syncPrimaryAction(report),
		RiskStatus:       syncRiskStatus(report),
		RiskLevel:        syncRiskLevel(report),
		RiskBreakdown:    syncRiskBreakdown(report),
		RiskHotspot:      syncRiskHotspot(report),
		HasProgress:      progressTotal > 0,
		HasRisk:          riskTotal > 0,
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
		return "resolve-reinjection-failures"
	case "changed-with-risk":
		return "review-risk-items"
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
		return "Reinjection is blocked; resolve skipped/failed agents first before adding new work."
	case "changed-with-risk":
		return "Progress landed with risk; fix failed reinjections before expanding scope."
	default:
		if report.DryRun {
			return "Apply this sync plan to convert planned progress into committed state."
		}
		return "Progress is applied and clear; move directly to the next feature increment."
	}
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

func syncRiskHotspot(report syncsvc.Report) string {
	if len(report.FailedReinjects) > 0 {
		return sortedStringSlice(report.FailedReinjects)[0]
	}
	if len(report.SkippedReinjects) > 0 {
		return sortedStringSlice(report.SkippedReinjects)[0]
	}
	return "none"
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
