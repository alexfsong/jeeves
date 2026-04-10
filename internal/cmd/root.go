package cmd

import (
	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	rootCmd    = &cobra.Command{
		Use:   "jeeves",
		Short: "Your personal research assistant",
		Long:  "Jeeves — a research CLI for fetching, synthesizing, and accumulating knowledge.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			dir, created, err := config.EnsureDir()
			if err == nil && created {
				r := render.New(false)
				r.Welcome(dir)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

func Execute() error {
	return rootCmd.Execute()
}
