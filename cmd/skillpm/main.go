package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"skillpm/internal/app"
	"skillpm/internal/leaderboard"

	"github.com/spf13/cobra"
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

	cmd.AddCommand(newSourceCmd(newSvc))
	cmd.AddCommand(newSearchCmd(newSvc))
	cmd.AddCommand(newInstallCmd(newSvc))
	cmd.AddCommand(newUninstallCmd(newSvc))
	cmd.AddCommand(newUpgradeCmd(newSvc))
	cmd.AddCommand(newInjectCmd(newSvc))
	cmd.AddCommand(newSyncCmd(newSvc))
	cmd.AddCommand(newScheduleCmd(newSvc))
	cmd.AddCommand(newDoctorCmd(newSvc))
	cmd.AddCommand(newVersionCmd(newSvc))
	cmd.AddCommand(newSelfCmd(newSvc))
	cmd.AddCommand(newLeaderboardCmd(newSvc))

	cmd.CompletionOptions.DisableDefaultCmd = true
	return cmd
}

func newSourceCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			fmt.Printf("added source %s (%s)\n", src.Name, src.Kind)
			return nil
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
			fmt.Printf("removed source %s\n", args[0])
			return nil
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
		Use:     "update [name]",
		Aliases: []string{"up"},
		Short:   "Update source",
		Args:    cobra.MaximumNArgs(1),
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
			for _, u := range updated {
				fmt.Printf("updated %s: %s\n", u.Source.Name, u.Note)
			}
			return nil
		},
	}

	sourceCmd.AddCommand(addCmd, removeCmd, listCmd, updateCmd)
	return sourceCmd
}

func newSearchCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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

func newInstallCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			fmt.Printf("📦 Resolving and installing %d skill(s)...\n", len(args))
			installed, err := svc.Install(context.Background(), args, lockfile, force)
			if err != nil {
				return err
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

func newUninstallCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			for _, r := range removed {
				fmt.Printf("uninstalled %s\n", r)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	return cmd
}

func newUpgradeCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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

func newInjectCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			fmt.Printf("injected %d skill(s) into %s\n", len(res.Injected), agentName)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	return cmd
}

func newRemoveCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			fmt.Printf("removed %d skill(s) from %s\n", len(res.Removed), agentName)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "target agent")
	return cmd
}

func newSyncCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			if dryRun {
				fmt.Printf("sync plan (dry-run): sources=%d upgrades=%d reinjected=%d skipped=%d failed=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected), len(report.SkippedReinjects), len(report.FailedReinjects))
				if strict && (len(report.SkippedReinjects) > 0 || len(report.FailedReinjects) > 0) {
					return fmt.Errorf("SYNC_RISK: sync plan includes %d risk items (strict mode)", len(report.SkippedReinjects)+len(report.FailedReinjects))
				}
				return nil
			}
			fmt.Printf("sync complete: sources=%d upgrades=%d reinjected=%d skipped=%d failed=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected), len(report.SkippedReinjects), len(report.FailedReinjects))
			if strict && (len(report.SkippedReinjects) > 0 || len(report.FailedReinjects) > 0) {
				return fmt.Errorf("SYNC_RISK: sync completed with %d risk items (strict mode)", len(report.SkippedReinjects)+len(report.FailedReinjects))
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

func newScheduleCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
				_, err = svc.Schedule("install", interval)
				if err != nil {
					return err
				}
				fmt.Printf("schedule enabled interval=%s\n", interval)
				return nil
			}
			syncCfg, err := svc.Schedule("list", "")
			if err != nil {
				return err
			}
			fmt.Printf("schedule mode=%s interval=%s\n", syncCfg.Mode, syncCfg.Interval)
			return nil
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
			fmt.Printf("schedule enabled interval=%s\n", syncCfg.Interval)
			return nil
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
			fmt.Printf("schedule mode=%s interval=%s\n", syncCfg.Mode, syncCfg.Interval)
			return nil
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
			_, err = svc.Schedule("remove", "")
			if err != nil {
				return err
			}
			fmt.Println("schedule disabled")
			return nil
		},
	}

	scheduleCmd.AddCommand(installCmd, listCmd, removeCmd)
	return scheduleCmd
}

func newDoctorCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
				if len(enabled) > 0 {
					fmt.Printf("enabled detected adapters: %s\n", strings.Join(enabled, ", "))
				}
			}
			report := svc.DoctorRun(context.Background())
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

func newSelfCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
			fmt.Println("updated")
			return nil
		},
	}
	updateCmd.Flags().StringVar(&channel, "channel", "stable", "release channel")
	selfCmd.AddCommand(updateCmd)
	return selfCmd
}

func newLeaderboardCmd(newSvc func() (*app.Service, error)) *cobra.Command {
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
