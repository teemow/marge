package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/go-github/v69/github"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	gh "github.com/teemow/marge/internal/github"
	"github.com/teemow/marge/internal/pr"
)

var runOpts RunOptions

func init() {
	runCmd.Flags().BoolVar(&runOpts.DryRun, "dry-run", false, "Show what would be done without making changes")
	runCmd.Flags().BoolVarP(&runOpts.Watch, "watch", "w", false, "Keep polling for new PRs (every 60s)")
	runCmd.Flags().StringVar(&runOpts.Grouping, "grouping", "repo", "Group by \"repo\" or \"dependency\"")
	runCmd.Flags().StringVar(&runOpts.Author, "author", "all", "Filter by PR author: \"renovate\", \"dependabot\", or \"all\"")
	runCmd.Flags().BoolVar(&runOpts.NoTUI, "no-tui", false, "Disable live table, print plain-text results instead")
	runCmd.Flags().StringVar(&runOpts.TrustedAuthors, "trusted-authors", "renovate[bot],dependabot[bot]", "Comma-separated list of trusted PR author logins")

	rootCmd.AddCommand(runCmd)

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

		me, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return fmt.Errorf("getting authenticated user: %w", err)
		}
		login := me.GetLogin()

		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		return watchLoop(ctx, runOpts.Watch, func(ctx context.Context) error {
			prs, err := searchPRs(ctx, client, query, login, runOpts.Author)
			if err != nil {
				return fmt.Errorf("searching PRs: %w", err)
			}

			opts := runOpts
			opts.Cols = pr.FullColumns()

			if query == "" && len(prs) > 0 {
				selected, specificGroup, err := interactiveSelect(prs, runOpts.Grouping)
				if err != nil {
					return err
				}
				prs = selected

				if len(prs) == 0 {
					fmt.Fprintln(os.Stderr, "No PRs selected.")
					return nil
				}

				if specificGroup {
					switch runOpts.Grouping {
					case "repo":
						opts.Cols = pr.RepoSelectedColumns()
					case "dependency":
						opts.Cols = pr.DependencySelectedColumns()
					}
				}
			}

			return processOnce(ctx, client, login, prs, opts)
		})
	},
}

func searchPRs(ctx context.Context, client *github.Client, query string, login string, authorFilter string) ([]pr.PRInfo, error) {
	var authorFilters []string
	switch authorFilter {
	case "renovate":
		authorFilters = []string{"author:app/renovate"}
	case "dependabot":
		authorFilters = []string{"author:app/dependabot"}
	default:
		authorFilters = []string{"author:app/renovate", "author:app/dependabot"}
	}

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

					owner, repo, err := pr.ExtractOwnerRepo(url)
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

			owner, repo, err := pr.ExtractOwnerRepo(url)
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

func interactiveSelect(prs []pr.PRInfo, grouping string) ([]pr.PRInfo, bool, error) {
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
