package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"skillpm/internal/app"
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
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile source updates with installed/injected state",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
			}
			report, err := svc.SyncRun(context.Background(), lockfile, force)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return print(true, report, "")
			}
			fmt.Printf("sync complete: sources=%d upgrades=%d reinjected=%d\n", len(report.UpdatedSources), len(report.UpgradedSkills), len(report.Reinjected))
			return nil
		},
	}
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "skills.lock path")
	cmd.Flags().BoolVar(&force, "force", false, "allow suspicious skills")
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
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newSvc()
			if err != nil {
				return err
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
