package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"skillpm/internal/config"
)

func newVersionCmd(jsonOutput *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version": config.Version,
				"commit":  config.Commit,
				"date":    config.Date,
			}
			if *jsonOutput {
				return print(true, info, "")
			}
			fmt.Printf("skillpm %s\ncommit: %s\nbuilt at: %s\n", config.Version, config.Commit, config.Date)
			return nil
		},
	}
}
