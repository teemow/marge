package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/spf13/cobra"
	gh "github.com/teemow/marge/internal/github"
	"github.com/teemow/marge/internal/pr"
	"github.com/teemow/marge/internal/process"
)

func init() {
	sweepCmd.Flags().BoolVar(&sweepDryRun, "dry-run", false, "Show what would be done without making changes")
	sweepCmd.Flags().BoolVarP(&sweepWatch, "watch", "w", false, "Keep polling for new PRs (every 60s)")
	sweepCmd.Flags().StringVar(&sweepAuthor, "author", "all", "Filter by PR author: \"renovate\", \"dependabot\", or \"all\"")
	sweepCmd.Flags().StringVar(&sweepOrg, "org", "", "Limit to repos owned by this org or user (e.g. \"giantswarm\")")
	sweepCmd.Flags().BoolVar(&sweepNoTUI, "no-tui", false, "Disable live table, print plain-text results instead")

	rootCmd.AddCommand(sweepCmd)
}

var (
	sweepDryRun bool
	sweepWatch  bool
	sweepAuthor string
	sweepOrg    string
	sweepNoTUI  bool
)

var sweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Merge all dependency update PRs, report failures",
	Long: `Automatically attempt to merge every open Renovate and Dependabot PR
that requests your review. After processing, a summary lists the PRs
that could not be merged so you can fix them manually.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Temporarily set shared vars so searchPRs picks them up.
		origAuthor := author
		author = sweepAuthor
		defer func() { author = origAuthor }()

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		client, err := gh.NewClient(ctx)
		if err != nil {
			return err
		}

		for {
			if err := sweepOnce(ctx, client); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			if !sweepWatch {
				return nil
			}

			fmt.Fprintf(os.Stderr, "\nWaiting 60s before next poll... (Ctrl+C to stop)\n")
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(60 * time.Second):
			}
		}
	},
}

func sweepOnce(ctx context.Context, client *github.Client) error {
	me, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("getting authenticated user: %w", err)
	}
	login := me.GetLogin()

	prs, err := searchPRs(ctx, client, "", login)
	if err != nil {
		return fmt.Errorf("searching PRs: %w", err)
	}

	if sweepOrg != "" {
		filtered := prs[:0]
		for _, p := range prs {
			if strings.EqualFold(p.Owner, sweepOrg) {
				filtered = append(filtered, p)
			}
		}
		prs = filtered
	}

	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "No matching PRs found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Sweeping %d PR(s)...\n\n", len(prs))

	status := pr.NewPRStatus()
	indices := make([]int, len(prs))
	for i, p := range prs {
		indices[i] = status.Add(p)
	}

	infoLabel := "Repository"
	infoFn := pr.InfoFunc(pr.RepoInfoFunc)

	if !sweepNoTUI {
		pr.PrintTableHeader(os.Stdout, infoLabel)
		for _, e := range status.Snapshot() {
			pr.PrintRow(os.Stdout, e, infoFn)
		}
	}

	stopRefresh := make(chan struct{})
	refreshStopped := make(chan struct{})
	if !sweepNoTUI {
		go func() {
			defer close(refreshStopped)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-stopRefresh:
					return
				case <-ticker.C:
					pr.UpdateTable(os.Stdout, status.Snapshot(), infoLabel, infoFn)
				}
			}
		}()
	} else {
		close(refreshStopped)
	}

	proc := process.NewProcessor(client, sweepDryRun, login)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for i, p := range prs {
		wg.Add(1)
		go func(info pr.PRInfo, idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			proc.ProcessPR(ctx, info, status, idx)
		}(p, indices[i])
	}

	wg.Wait()

	close(stopRefresh)
	<-refreshStopped

	if sweepNoTUI {
		pr.PrintPlainResults(os.Stdout, status)
	} else {
		pr.UpdateTable(os.Stdout, status.Snapshot(), infoLabel, infoFn)
	}

	fmt.Fprintf(os.Stderr, "\n%s\n", status.FormatSummary())

	if !sweepNoTUI {
		actionRequired := status.ActionRequired()
		if len(actionRequired) > 0 {
			fmt.Fprintf(os.Stderr, "\nAction required (%d):\n\n", len(actionRequired))
			for _, e := range actionRequired {
				detail := e.State.String()
				if e.Detail != "" {
					detail = fmt.Sprintf("%s (%s)", detail, e.Detail)
				}
				fmt.Fprintf(os.Stderr, "  #%-6d %s/%s\n", e.PR.Number, e.PR.Owner, e.PR.Repo)
				fmt.Fprintf(os.Stderr, "         %s\n", e.PR.Title)
				fmt.Fprintf(os.Stderr, "         %s  %s\n\n", e.PR.URL, colorStatus(detail, e.State))
			}
		}
	}

	return nil
}

func colorStatus(text string, state pr.StatusState) string {
	switch state {
	case pr.StatusFailed:
		return fmt.Sprintf("\033[31m%s\033[0m", text)
	case pr.StatusConflict:
		return fmt.Sprintf("\033[33m%s\033[0m", text)
	default:
		return text
	}
}
