package pr

import (
	"testing"
)

func samplePRs() []PRInfo {
	return []PRInfo{
		{Owner: "giantswarm", Repo: "flux-app", Number: 1, Title: "Update dependency foo to v1.0.0"},
		{Owner: "giantswarm", Repo: "flux-app", Number: 2, Title: "Bump bar from 1.0 to 2.0"},
		{Owner: "giantswarm", Repo: "kyverno-app", Number: 10, Title: "Update dependency foo to v1.0.0"},
		{Owner: "giantswarm", Repo: "kyverno-app", Number: 11, Title: "Update dependency baz to v3.0.0"},
		{Owner: "giantswarm", Repo: "kyverno-app", Number: 12, Title: "Bump bar from 2.0 to 3.0"},
	}
}

func TestGroupByRepo(t *testing.T) {
	groups := GroupByRepo(samplePRs())

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// kyverno-app has 3 PRs so it should be first (sorted by count desc)
	if groups[0].Key != "giantswarm/kyverno-app" {
		t.Errorf("first group key = %q, want %q", groups[0].Key, "giantswarm/kyverno-app")
	}
	if groups[0].Count != 3 {
		t.Errorf("first group count = %d, want 3", groups[0].Count)
	}

	if groups[1].Key != "giantswarm/flux-app" {
		t.Errorf("second group key = %q, want %q", groups[1].Key, "giantswarm/flux-app")
	}
	if groups[1].Count != 2 {
		t.Errorf("second group count = %d, want 2", groups[1].Count)
	}
}

func TestGroupByDependency(t *testing.T) {
	groups := GroupByDependency(samplePRs())

	// Should produce groups for: foo (2 PRs), bar (2 PRs), baz (1 PR)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	groupMap := make(map[string]PRGroup)
	for _, g := range groups {
		groupMap[g.Key] = g
	}

	if g, ok := groupMap["foo"]; !ok {
		t.Error("expected group for dependency 'foo'")
	} else if g.Count != 2 {
		t.Errorf("foo group count = %d, want 2", g.Count)
	}

	if g, ok := groupMap["bar"]; !ok {
		t.Error("expected group for dependency 'bar'")
	} else if g.Count != 2 {
		t.Errorf("bar group count = %d, want 2", g.Count)
	}

	if g, ok := groupMap["baz"]; !ok {
		t.Error("expected group for dependency 'baz'")
	} else if g.Count != 1 {
		t.Errorf("baz group count = %d, want 1", g.Count)
	}
}

func TestGroupByDependency_unknownTitle(t *testing.T) {
	prs := []PRInfo{
		{Owner: "org", Repo: "repo", Number: 1, Title: "Fix a random bug"},
	}
	groups := GroupByDependency(prs)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Key != "(unknown)" {
		t.Errorf("group key = %q, want %q", groups[0].Key, "(unknown)")
	}
}

func TestGroupByRepo_empty(t *testing.T) {
	groups := GroupByRepo(nil)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestGroupByDependency_empty(t *testing.T) {
	groups := GroupByDependency(nil)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestSortedGroups_tiebreakByKey(t *testing.T) {
	prs := []PRInfo{
		{Owner: "org", Repo: "bravo", Number: 1, Title: "Update dependency x to v1"},
		{Owner: "org", Repo: "alpha", Number: 2, Title: "Update dependency y to v1"},
	}
	groups := GroupByRepo(prs)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	// Same count (1 each), so should be alphabetical
	if groups[0].Key != "org/alpha" {
		t.Errorf("first group key = %q, want %q (alphabetical tiebreak)", groups[0].Key, "org/alpha")
	}
	if groups[1].Key != "org/bravo" {
		t.Errorf("second group key = %q, want %q", groups[1].Key, "org/bravo")
	}
}

func TestGroupPreservesPRData(t *testing.T) {
	prs := []PRInfo{
		{Owner: "org", Repo: "repo", Number: 42, Title: "Bump foo from 1.0 to 2.0", URL: "https://github.com/org/repo/pull/42", Author: "app/dependabot"},
	}
	groups := GroupByRepo(prs)

	if len(groups) != 1 || len(groups[0].PRs) != 1 {
		t.Fatalf("expected exactly 1 group with 1 PR")
	}

	pr := groups[0].PRs[0]
	if pr.Number != 42 {
		t.Errorf("PR number = %d, want 42", pr.Number)
	}
	if pr.URL != "https://github.com/org/repo/pull/42" {
		t.Errorf("PR URL = %q, want %q", pr.URL, "https://github.com/org/repo/pull/42")
	}
	if pr.Author != "app/dependabot" {
		t.Errorf("PR author = %q, want %q", pr.Author, "app/dependabot")
	}
}
