package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/teemow/marge/internal/pr"
	"github.com/teemow/marge/internal/process"
)

// RunOptions holds the configuration shared between the run and sweep commands.
type RunOptions struct {
	DryRun         bool
	Watch          bool
	NoTUI          bool
	Author         string
	TrustedAuthors string
	MergeAuto      bool
	Org            string
	ReposFile      string
	Grouping       string
	Cols           []pr.TableColumn
	OnComplete     func(*pr.PRStatus)
}

func processOnce(ctx context.Context, client *github.Client, login string, prs []pr.PRInfo, opts RunOptions) error {
	if len(prs) == 0 {
		fmt.Fprintln(os.Stderr, "No matching PRs found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Processing %d PR(s)...\n\n", len(prs))

	sort.Slice(prs, func(i, j int) bool {
		ri := prs[i].Owner + "/" + prs[i].Repo
		rj := prs[j].Owner + "/" + prs[j].Repo
		if ri != rj {
			return ri < rj
		}
		return prs[i].Number < prs[j].Number
	})

	status := pr.NewPRStatus()
	indices := make([]int, len(prs))
	for i, p := range prs {
		indices[i] = status.Add(p)
	}

	cols := opts.Cols
	if cols == nil {
		cols = pr.FullColumns()
	}
	pr.AdjustColumnWidths(cols, prs)

	if !opts.NoTUI {
		pr.PrintTableHeader(os.Stdout, cols)
		for _, e := range status.Snapshot() {
			pr.PrintRow(os.Stdout, e, cols)
		}
	}

	stopRefresh := make(chan struct{})
	refreshStopped := make(chan struct{})
	if !opts.NoTUI {
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

	proc := process.NewProcessor(client, opts.DryRun, opts.MergeAuto, login, parseTrustedAuthors(opts.TrustedAuthors))

	// Build a per-repo index so we can look up each PR's status table index.
	indexByPR := make(map[string]int, len(prs))
	for i, p := range prs {
		key := fmt.Sprintf("%s/%s#%d", p.Owner, p.Repo, p.Number)
		indexByPR[key] = indices[i]
	}

	// Group PRs by owner/repo. PRs within the same repo are processed
	// sequentially to avoid "base branch was modified" failures. Different
	// repo groups run in parallel, bounded by the semaphore.
	repoGroups := pr.GroupByRepo(prs)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, group := range repoGroups {
		wg.Add(1)
		go func(repoPRs []pr.PRInfo) {
			defer wg.Done()
			for _, info := range repoPRs {
				sem <- struct{}{}
				key := fmt.Sprintf("%s/%s#%d", info.Owner, info.Repo, info.Number)
				idx := indexByPR[key]
				proc.ProcessPR(ctx, info, status, idx)
				<-sem
			}
		}(group.PRs)
	}

	wg.Wait()

	close(stopRefresh)
	<-refreshStopped

	if opts.NoTUI {
		pr.PrintPlainResults(os.Stdout, status)
	} else {
		pr.UpdateTable(os.Stdout, status.Snapshot(), cols)
	}

	fmt.Fprintf(os.Stderr, "\n%s\n", status.FormatSummary())

	if opts.OnComplete != nil {
		opts.OnComplete(status)
	}

	return nil
}

func watchLoop(ctx context.Context, watch bool, fn func(ctx context.Context) error) error {
	for {
		if err := fn(ctx); err != nil {
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
