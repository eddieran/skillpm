package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"skillpm/internal/app"
	"skillpm/internal/config"
)

func newVersionCmd(newSvc func() (*app.Service, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("skillpm %s\ncommit: %s\nbuilt at: %s\n", config.Version, config.Commit, config.Date)
			return nil
		},
	}
}
