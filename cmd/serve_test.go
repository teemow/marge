package cmd

import (
	"testing"

	"github.com/teemow/marge/internal/pr"
)

// TestBuildSweepResult_failedAndSecurityAreDisjoint guards the contract
// documented on SweepSummary: a security failure must be counted in
// SecurityFailures and not in Failed, so consumers can sum them without
// double-counting.
func TestBuildSweepResult_failedAndSecurityAreDisjoint(t *testing.T) {
	status := pr.NewPRStatus()
	idx1 := status.Add(pr.PRInfo{Owner: "o", Repo: "r", Number: 1})
	status.Update(idx1, pr.StatusFailed, "checks failed")
	idx2 := status.Add(pr.PRInfo{Owner: "o", Repo: "r", Number: 2})
	status.Update(idx2, pr.StatusFailedSecurity, "security check failed: trivy")
	idx3 := status.Add(pr.PRInfo{Owner: "o", Repo: "r", Number: 3})
	status.Update(idx3, pr.StatusMerged, "squash")
	idx4 := status.Add(pr.PRInfo{Owner: "o", Repo: "r", Number: 4})
	status.Update(idx4, pr.StatusSkipped, "dry-run")

	got := buildSweepResult(status)

	if got.Summary.Total != 4 {
		t.Errorf("Total = %d, want 4", got.Summary.Total)
	}
	if got.Summary.Merged != 1 {
		t.Errorf("Merged = %d, want 1", got.Summary.Merged)
	}
	if got.Summary.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (security failures must be excluded)", got.Summary.Failed)
	}
	if got.Summary.SecurityFailures != 1 {
		t.Errorf("SecurityFailures = %d, want 1", got.Summary.SecurityFailures)
	}
	if got.Summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", got.Summary.Skipped)
	}

	if len(got.SecurityFailures) != 1 {
		t.Fatalf("SecurityFailures slice len = %d, want 1", len(got.SecurityFailures))
	}
	if got.SecurityFailures[0].Number != 2 {
		t.Errorf("SecurityFailures[0].Number = %d, want 2", got.SecurityFailures[0].Number)
	}

	if len(got.ActionRequired) != 1 {
		t.Fatalf("ActionRequired slice len = %d, want 1", len(got.ActionRequired))
	}
	if got.ActionRequired[0].Number != 1 {
		t.Errorf("ActionRequired[0].Number = %d, want 1", got.ActionRequired[0].Number)
	}
}
