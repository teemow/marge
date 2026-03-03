package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	gh "github.com/teemow/marge/internal/github"
	"github.com/teemow/marge/internal/pr"
)

var sweepOpts RunOptions

func init() {
	sweepCmd.Flags().BoolVar(&sweepOpts.DryRun, "dry-run", false, "Show what would be done without making changes")
	sweepCmd.Flags().BoolVarP(&sweepOpts.Watch, "watch", "w", false, "Keep polling for new PRs (every 60s)")
	sweepCmd.Flags().StringVar(&sweepOpts.Author, "author", "all", "Filter by PR author: \"renovate\", \"dependabot\", or \"all\"")
	sweepCmd.Flags().StringVar(&sweepOpts.Org, "org", "", "Limit to repos owned by this org or user")
	sweepCmd.Flags().BoolVar(&sweepOpts.NoTUI, "no-tui", false, "Disable live table, print plain-text results instead")
	sweepCmd.Flags().BoolVar(&sweepOpts.MergeAuto, "merge-auto", false, "Also merge PRs that have auto-merge enabled")
	sweepCmd.Flags().StringVar(&sweepOpts.TrustedAuthors, "trusted-authors", "renovate[bot],dependabot[bot]", "Comma-separated list of trusted PR author logins")

	rootCmd.AddCommand(sweepCmd)
}

var sweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Merge all dependency update PRs, report failures",
	Long: `Automatically attempt to merge every open Renovate and Dependabot PR
that requests your review. After processing, a summary lists the PRs
that could not be merged so you can fix them manually.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		client, err := gh.NewClient(ctx)
		if err != nil {
			return err
		}

		me, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return fmt.Errorf("getting authenticated user: %w", err)
		}
		login := me.GetLogin()

		return watchLoop(ctx, sweepOpts.Watch, func(ctx context.Context) error {
			prs, err := searchPRs(ctx, client, "", login, sweepOpts.Author)
			if err != nil {
				return fmt.Errorf("searching PRs: %w", err)
			}

			if sweepOpts.Org != "" {
				filtered := prs[:0]
				for _, p := range prs {
					if strings.EqualFold(p.Owner, sweepOpts.Org) {
						filtered = append(filtered, p)
					}
				}
				prs = filtered
			}

			opts := sweepOpts
			if !opts.NoTUI {
				opts.OnComplete = func(status *pr.PRStatus) {
					actionRequired := status.ActionRequired()
					if len(actionRequired) > 0 {
						fmt.Fprintf(os.Stderr, "\nAction required (%d):\n\n", len(actionRequired))
						for _, e := range actionRequired {
							fmt.Fprintf(os.Stderr, "  #%-6d %s/%s\n", e.PR.Number, e.PR.Owner, e.PR.Repo)
							fmt.Fprintf(os.Stderr, "         %s\n", e.PR.Title)
							fmt.Fprintf(os.Stderr, "         %s  %s\n\n", e.PR.URL, pr.ColorizeStatus(e.State, e.Detail))
						}
					}
				}
			}

			return processOnce(ctx, client, login, prs, opts)
		})
	},
}
