package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	gh "github.com/teemow/marge/internal/github"
	"github.com/teemow/marge/internal/pr"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a stdio MCP server exposing sweep as a tool",
	Long: `Start a Model Context Protocol (MCP) server over stdio.
The server exposes a "sweep" tool that mirrors the sweep CLI command,
returning structured JSON results instead of terminal output.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		mcpServer := server.NewMCPServer(
			"marge",
			version,
			server.WithToolCapabilities(true),
		)

		mcpServer.AddTool(
			mcp.NewTool("sweep",
				mcp.WithDescription("Sweep dependency update PRs: find, approve, and merge Renovate/Dependabot PRs"),
				mcp.WithString("org",
					mcp.Description("GitHub organization or user to limit the sweep to"),
				),
				mcp.WithString("repos_file",
					mcp.Description("Path to a file listing org/repo entries (one per line) to scan for bot PRs"),
				),
				mcp.WithArray("repos",
					mcp.Description("Explicit list of repos (org/repo format) to sweep"),
					mcp.WithStringItems(),
				),
				mcp.WithBoolean("merge_auto",
					mcp.Description("Also merge PRs that have auto-merge enabled (default: false)"),
				),
				mcp.WithBoolean("dry_run",
					mcp.Description("Show what would be done without making changes (default: false)"),
				),
				mcp.WithString("author",
					mcp.Description("Filter by PR author: \"renovate\", \"dependabot\", or \"all\" (default: \"all\")"),
					mcp.Enum("renovate", "dependabot", "all"),
				),
				mcp.WithString("trusted_authors",
					mcp.Description("Comma-separated list of trusted PR author logins (default: \"renovate[bot],dependabot[bot]\")"),
				),
			),
			handleSweep,
		)

		return server.ServeStdio(mcpServer)
	},
}

// SweepResult is the structured JSON output returned by the sweep MCP tool.
type SweepResult struct {
	Summary        SweepSummary    `json:"summary"`
	Merged         []SweepPREntry  `json:"merged,omitempty"`
	ActionRequired []SweepPREntry  `json:"action_required,omitempty"`
	Skipped        []SweepPREntry  `json:"skipped,omitempty"`
}

// SweepSummary contains aggregate counts from the sweep.
type SweepSummary struct {
	Total   int `json:"total"`
	Merged  int `json:"merged"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// SweepPREntry represents a single PR in the sweep results.
type SweepPREntry struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func handleSweep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from the request.
	org := request.GetString("org", "")
	reposFile := request.GetString("repos_file", "")
	mergeAuto := request.GetBool("merge_auto", false)
	dryRun := request.GetBool("dry_run", false)
	author := request.GetString("author", "all")
	trustedAuthors := request.GetString("trusted_authors", "renovate[bot],dependabot[bot]")
	reposParam := request.GetStringSlice("repos", nil)

	// Create a temporary repos file if repos array was provided.
	if len(reposParam) > 0 && reposFile == "" {
		tmpFile, err := createTempReposFile(reposParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating temp repos file: %v", err)), nil
		}
		reposFile = tmpFile
	}

	client, err := gh.NewClient(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("creating GitHub client: %v", err)), nil
	}

	me, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting authenticated user: %v", err)), nil
	}
	login := me.GetLogin()

	prs, err := searchPRs(ctx, client, "", login, author, reposFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("searching PRs: %v", err)), nil
	}

	if org != "" {
		filtered := prs[:0]
		for _, p := range prs {
			if strings.EqualFold(p.Owner, org) {
				filtered = append(filtered, p)
			}
		}
		prs = filtered
	}

	opts := RunOptions{
		DryRun:         dryRun,
		MergeAuto:      mergeAuto,
		NoTUI:          true,
		Author:         author,
		TrustedAuthors: trustedAuthors,
	}

	status, err := processOnceWithStatus(ctx, client, login, prs, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("processing PRs: %v", err)), nil
	}

	result := buildSweepResult(status)

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func buildSweepResult(status *pr.PRStatus) SweepResult {
	merged, failed, skipped := status.Summary()
	total := status.Len()

	result := SweepResult{
		Summary: SweepSummary{
			Total:   total,
			Merged:  merged,
			Failed:  failed,
			Skipped: skipped,
		},
	}

	for _, e := range status.MergedEntries() {
		result.Merged = append(result.Merged, SweepPREntry{
			Owner:  e.PR.Owner,
			Repo:   e.PR.Repo,
			Number: e.PR.Number,
			Title:  e.PR.Title,
			URL:    e.PR.URL,
			Status: e.State.String(),
			Detail: e.Detail,
		})
	}

	for _, e := range status.ActionRequired() {
		result.ActionRequired = append(result.ActionRequired, SweepPREntry{
			Owner:  e.PR.Owner,
			Repo:   e.PR.Repo,
			Number: e.PR.Number,
			Title:  e.PR.Title,
			URL:    e.PR.URL,
			Status: e.State.String(),
			Detail: e.Detail,
		})
	}

	for _, e := range status.SkippedEntries() {
		result.Skipped = append(result.Skipped, SweepPREntry{
			Owner:  e.PR.Owner,
			Repo:   e.PR.Repo,
			Number: e.PR.Number,
			Title:  e.PR.Title,
			URL:    e.PR.URL,
			Status: e.State.String(),
			Detail: e.Detail,
		})
	}

	return result
}

func createTempReposFile(repos []string) (string, error) {
	f, err := os.CreateTemp("", "marge-repos-*.txt")
	if err != nil {
		return "", err
	}

	for _, repo := range repos {
		if _, err := fmt.Fprintln(f, repo); err != nil {
			_ = f.Close()
			return "", err
		}
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return f.Name(), nil
}
