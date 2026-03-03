package process

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/teemow/marge/internal/pr"
)

const (
	checkPollInterval = 15 * time.Second
	checkPollTimeout  = 5 * time.Minute
)

var DefaultTrustedAuthors = map[string]bool{
	"renovate[bot]":  true,
	"dependabot[bot]": true,
}

type Processor struct {
	Client         *github.Client
	DryRun         bool
	MergeAutoMerge bool
	Login          string
	TrustedAuthors map[string]bool
}

func NewProcessor(client *github.Client, dryRun bool, mergeAutoMerge bool, login string, trustedAuthors map[string]bool) *Processor {
	if trustedAuthors == nil {
		trustedAuthors = DefaultTrustedAuthors
	}
	return &Processor{
		Client:         client,
		DryRun:         dryRun,
		MergeAutoMerge: mergeAutoMerge,
		Login:          login,
		TrustedAuthors: trustedAuthors,
	}
}

func (p *Processor) ProcessPR(ctx context.Context, info pr.PRInfo, status *pr.PRStatus, idx int) {
	pullReq, _, err := p.Client.PullRequests.Get(ctx, info.Owner, info.Repo, info.Number)
	if err != nil {
		status.Update(idx, pr.StatusFailed, ghErrorDetail("fetch error", err))
		return
	}

	actualAuthor := pullReq.GetUser().GetLogin()
	if !p.isAuthorTrusted(actualAuthor) {
		status.Update(idx, pr.StatusUntrustedAuthor, fmt.Sprintf("author %q not trusted", actualAuthor))
		return
	}

	if pullReq.GetMerged() {
		status.Update(idx, pr.StatusAlreadyMerged, "")
		return
	}

	if pullReq.GetMergeableState() == "dirty" {
		status.Update(idx, pr.StatusConflict, "merge conflict")
		return
	}

	status.Update(idx, pr.StatusChecking, "")
	if err := p.waitForChecks(ctx, info, status, idx); err != nil {
		return
	}

	if p.DryRun {
		status.Update(idx, pr.StatusSkipped, "dry-run")
		return
	}

	selfAuthored := strings.EqualFold(info.Author, p.Login)

	if !selfAuthored {
		if err := p.approve(ctx, info, status, idx); err != nil {
			return
		}
	}

	if pullReq.GetAutoMerge() != nil && !p.MergeAutoMerge {
		status.Update(idx, pr.StatusAutoMerge, "auto-merge enabled")
		return
	}

	p.merge(ctx, info, pullReq, status, idx)
}

func (p *Processor) waitForChecks(ctx context.Context, info pr.PRInfo, status *pr.PRStatus, idx int) error {
	deadline := time.After(checkPollTimeout)
	for {
		state, err := p.getCombinedCheckState(ctx, info)
		if err != nil {
			status.Update(idx, pr.StatusFailed, ghErrorDetail("check error", err))
			return err
		}

		switch state {
		case "success":
			return nil
		case "failure", "error":
			status.Update(idx, pr.StatusFailed, "checks failed")
			return fmt.Errorf("checks failed")
		}

		status.Update(idx, pr.StatusChecking, state)

		select {
		case <-ctx.Done():
			status.Update(idx, pr.StatusSkipped, "cancelled")
			return ctx.Err()
		case <-deadline:
			status.Update(idx, pr.StatusFailed, "checks timed out")
			return fmt.Errorf("checks timed out")
		case <-time.After(checkPollInterval):
		}
	}
}

func (p *Processor) getCombinedCheckState(ctx context.Context, info pr.PRInfo) (string, error) {
	combined, _, err := p.Client.Repositories.GetCombinedStatus(ctx, info.Owner, info.Repo, fmt.Sprintf("refs/pull/%d/head", info.Number), nil)
	if err != nil {
		return "", err
	}

	combinedState := combined.GetState()

	// Also check check-runs (GitHub Actions use check runs, not commit statuses)
	checkRuns, _, err := p.Client.Checks.ListCheckRunsForRef(ctx, info.Owner, info.Repo, fmt.Sprintf("refs/pull/%d/head", info.Number), nil)
	if err != nil {
		return "", err
	}

	if checkRuns.GetTotal() == 0 && len(combined.Statuses) == 0 {
		// No checks configured -- treat as success
		return "success", nil
	}

	allComplete := true
	hasFailure := false
	for _, cr := range checkRuns.CheckRuns {
		if cr.GetStatus() != "completed" {
			allComplete = false
			continue
		}
		conclusion := cr.GetConclusion()
		if conclusion == "failure" || conclusion == "timed_out" || conclusion == "cancelled" {
			hasFailure = true
		}
	}

	if hasFailure {
		return "failure", nil
	}
	if !allComplete {
		return "pending", nil
	}
	if combinedState == "failure" || combinedState == "error" {
		return combinedState, nil
	}
	if combinedState == "pending" && len(combined.Statuses) > 0 {
		return "pending", nil
	}

	return "success", nil
}

func (p *Processor) approve(ctx context.Context, info pr.PRInfo, status *pr.PRStatus, idx int) error {
	reviews, _, err := p.Client.PullRequests.ListReviews(ctx, info.Owner, info.Repo, info.Number, nil)
	if err != nil {
		status.Update(idx, pr.StatusFailed, ghErrorDetail("review list error", err))
		return err
	}

	for _, r := range reviews {
		if r.GetUser().GetLogin() == p.Login && r.GetState() == "APPROVED" {
			return nil
		}
	}

	status.Update(idx, pr.StatusApproving, "")

	event := "APPROVE"
	_, _, err = p.Client.PullRequests.CreateReview(ctx, info.Owner, info.Repo, info.Number, &github.PullRequestReviewRequest{
		Event: &event,
	})
	if err != nil {
		status.Update(idx, pr.StatusFailed, ghErrorDetail("approve error", err))
		return err
	}

	return nil
}

func (p *Processor) merge(ctx context.Context, info pr.PRInfo, pullReq *github.PullRequest, status *pr.PRStatus, idx int) {
	status.Update(idx, pr.StatusMerging, "")

	method := determineMergeMethod(pullReq)

	_, _, err := p.Client.PullRequests.Merge(ctx, info.Owner, info.Repo, info.Number, "", &github.PullRequestOptions{
		MergeMethod: method,
	})
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "409") || strings.Contains(errMsg, "conflict") {
			status.Update(idx, pr.StatusConflict, "merge conflict")
		} else {
			status.Update(idx, pr.StatusFailed, ghErrorDetail("merge error", err))
		}
		return
	}

	status.Update(idx, pr.StatusMerged, method)
}

func (p *Processor) isAuthorTrusted(login string) bool {
	if strings.EqualFold(login, p.Login) {
		return true
	}
	return p.TrustedAuthors[login]
}

func ghErrorDetail(prefix string, err error) string {
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		for _, e := range ghErr.Errors {
			if e.Message != "" {
				return fmt.Sprintf("%s: %s", prefix, e.Message)
			}
		}
		if ghErr.Message != "" {
			return fmt.Sprintf("%s: %s", prefix, ghErr.Message)
		}
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}

func determineMergeMethod(pullReq *github.PullRequest) string {
	// Prefer squash > merge > rebase
	// GitHub API doesn't expose allowed methods on the PR itself,
	// so default to squash which is the most common for dependency updates.
	return "squash"
}
