package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "marge",
	Short: "A housekeeping tool for dependency update PRs",
	Long: `Marge automates the approve-and-merge workflow for Renovate and Dependabot PRs
that request your review. It searches GitHub for open PRs, groups them
interactively, waits for CI checks to pass, approves, and merges them.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
