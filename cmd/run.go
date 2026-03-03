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
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	gh "github.com/teemow/marge/internal/github"
	"github.com/teemow/marge/internal/pr"
	"github.com/teemow/marge/internal/process"
)

var (
	dryRun         bool
	watch          bool
	grouping       string
	author         string
	noTUI          bool
	trustedAuthors string
)

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	runCmd.Flags().BoolVarP(&watch, "watch", "w", false, "Keep polling for new PRs (every 60s)")
	runCmd.Flags().StringVar(&grouping, "grouping", "repo", "Group by \"repo\" or \"dependency\"")
	runCmd.Flags().StringVar(&author, "author", "all", "Filter by PR author: \"renovate\", \"dependabot\", or \"all\"")
	runCmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable live table, print plain-text results instead")
	runCmd.Flags().StringVar(&trustedAuthors, "trusted-authors", "renovate[bot],dependabot[bot]", "Comma-separated list of trusted PR author logins")

	rootCmd.AddCommand(runCmd)

	// Also make "run" the default when no subcommand is given
	rootCmd.RunE = runCmd.RunE
	rootCmd.Args = cobra.MaximumNArgs(1)
	rootCmd.Flags().AddFlagSet(runCmd.Flags())
}

var runCmd = &cobra.Command{
	Use:   "run [query]",
	Short: "Find, approve, and merge dependency update PRs",
	Long: `Search for open Renovate and Dependabot PRs requesting your review,
optionally group them interactively, then approve and merge them.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		client, err := gh.NewClient(ctx)
		if err != nil {
			return err
		}

		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		for {
			if err := runOnce(ctx, client, query); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			if !watch {
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

func runOnce(ctx context.Context, client *github.Client, query string) error {
	me, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("getting authenticated user: %w", err)
	}
	login := me.GetLogin()

	prs, err := searchPRs(ctx, client, query, login)
	if err != nil {
		return fmt.Errorf("searching PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "No matching PRs found.")
		return nil
	}

	cols := pr.FullColumns()

	// Interactive mode: if no query provided, let user pick a group
	if query == "" {
		selected, specificGroup, err := interactiveSelect(prs)
		if err != nil {
			return err
		}
		prs = selected

		if specificGroup {
			switch grouping {
			case "repo":
				cols = pr.RepoSelectedColumns()
			case "dependency":
				cols = pr.DependencySelectedColumns()
			}
		}
	}

	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "No PRs selected.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Processing %d PR(s)...\n\n", len(prs))

	status := pr.NewPRStatus()
	indices := make([]int, len(prs))
	for i, p := range prs {
		indices[i] = status.Add(p)
	}

	if !noTUI {
		pr.PrintTableHeader(os.Stdout, cols)
		for _, e := range status.Snapshot() {
			pr.PrintRow(os.Stdout, e, cols)
		}
	}

	stopRefresh := make(chan struct{})
	refreshStopped := make(chan struct{})
	if !noTUI {
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
					pr.UpdateTable(os.Stdout, status.Snapshot(), cols)
				}
			}
		}()
	} else {
		close(refreshStopped)
	}

	proc := process.NewProcessor(client, dryRun, false, login, parseTrustedAuthors(trustedAuthors))

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

	if noTUI {
		pr.PrintPlainResults(os.Stdout, status)
	} else {
		pr.UpdateTable(os.Stdout, status.Snapshot(), cols)
	}

	fmt.Fprintf(os.Stderr, "\n%s\n", status.FormatSummary())

	return nil
}

func searchPRs(ctx context.Context, client *github.Client, query string, login string) ([]pr.PRInfo, error) {
	var authorFilters []string
	switch author {
	case "renovate":
		authorFilters = []string{"author:app/renovate"}
	case "dependabot":
		authorFilters = []string{"author:app/dependabot"}
	default:
		authorFilters = []string{"author:app/renovate", "author:app/dependabot"}
	}

	// Two scope filters, deduplicated:
	// 1. review-requested:@me  -- PRs where you're explicitly requested (org repos)
	// 2. user:<login>          -- PRs in your own repos
	scopeFilters := []string{
		"review-requested:@me",
		fmt.Sprintf("user:%s", login),
	}

	seen := make(map[string]bool)
	var allPRs []pr.PRInfo

	for _, scope := range scopeFilters {
		for _, af := range authorFilters {
			searchQuery := fmt.Sprintf("%s is:pr is:open archived:false %s %s", query, scope, af)
			searchQuery = strings.TrimSpace(searchQuery)

			opts := &github.SearchOptions{
				Sort:        "updated",
				ListOptions: github.ListOptions{PerPage: 100},
			}

			for {
				result, resp, err := client.Search.Issues(ctx, searchQuery, opts)
				if err != nil {
					return nil, fmt.Errorf("search failed: %w", err)
				}

				for _, issue := range result.Issues {
					url := issue.GetHTMLURL()
					if seen[url] {
						continue
					}
					seen[url] = true

					owner, repo, err := extractOwnerRepo(url)
					if err != nil {
						continue
					}

					allPRs = append(allPRs, pr.PRInfo{
						Owner:  owner,
						Repo:   repo,
						Number: issue.GetNumber(),
						Title:  issue.GetTitle(),
						URL:    url,
						Author: issue.GetUser().GetLogin(),
					})
				}

				if resp.NextPage == 0 {
					break
				}
				opts.Page = resp.NextPage
			}
		}
	}

	// Also search for self-authored dependency update PRs in user's own repos.
	// This catches PRs created by self-hosted Renovate running under the user's PAT.
	selfQuery := fmt.Sprintf("%s is:pr is:open archived:false user:%s author:%s", query, login, login)
	selfQuery = strings.TrimSpace(selfQuery)

	selfOpts := &github.SearchOptions{
		Sort:        "updated",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := client.Search.Issues(ctx, selfQuery, selfOpts)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		for _, issue := range result.Issues {
			url := issue.GetHTMLURL()
			if seen[url] {
				continue
			}

			title := issue.GetTitle()
			if !pr.IsDependencyUpdateTitle(title) {
				continue
			}

			seen[url] = true

			owner, repo, err := extractOwnerRepo(url)
			if err != nil {
				continue
			}

			allPRs = append(allPRs, pr.PRInfo{
				Owner:  owner,
				Repo:   repo,
				Number: issue.GetNumber(),
				Title:  title,
				URL:    url,
				Author: issue.GetUser().GetLogin(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		selfOpts.Page = resp.NextPage
	}

	return allPRs, nil
}

func extractOwnerRepo(htmlURL string) (string, string, error) {
	// URL format: https://github.com/OWNER/REPO/pull/NUMBER
	parts := strings.Split(strings.TrimPrefix(htmlURL, "https://github.com/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected URL format: %s", htmlURL)
	}
	return parts[0], parts[1], nil
}

func interactiveSelect(prs []pr.PRInfo) ([]pr.PRInfo, bool, error) {
	var groups []pr.PRGroup
	switch grouping {
	case "dependency":
		groups = pr.GroupByDependency(prs)
	default:
		groups = pr.GroupByRepo(prs)
	}

	items := make([]string, 0, len(groups)+1)
	items = append(items, fmt.Sprintf("All (%d PRs)", len(prs)))
	for _, g := range groups {
		authors := uniqueAuthors(g.PRs)
		items = append(items, fmt.Sprintf("%s (%d PRs) [%s]", g.Key, g.Count, strings.Join(authors, ", ")))
	}

	prompt := promptui.Select{
		Label: "Select PRs to process",
		Items: items,
		Size:  20,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, false, fmt.Errorf("selection cancelled: %w", err)
	}

	if idx == 0 {
		return prs, false, nil
	}

	return groups[idx-1].PRs, true, nil
}

func parseTrustedAuthors(csv string) map[string]bool {
	m := make(map[string]bool)
	for _, a := range strings.Split(csv, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			m[a] = true
		}
	}
	return m
}

func uniqueAuthors(prs []pr.PRInfo) []string {
	seen := make(map[string]bool)
	var authors []string
	for _, p := range prs {
		if p.Author != "" && !seen[p.Author] {
			seen[p.Author] = true
			authors = append(authors, p.Author)
		}
	}
	return authors
}
