package pr

import (
	"strings"
	"testing"
)

func TestStatusFailedSecurity_string(t *testing.T) {
	if got := StatusFailedSecurity.String(); got != "Failed (security)" {
		t.Errorf("StatusFailedSecurity.String() = %q, want %q", got, "Failed (security)")
	}
}

func TestStatus_securityFailureCountedAsFailed(t *testing.T) {
	s := NewPRStatus()
	idx := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 1})
	s.Update(idx, StatusFailedSecurity, "security check failed: govulncheck")

	merged, failed, blocked, skipped := s.Summary()
	if failed != 1 {
		t.Errorf("Summary failed = %d, want 1", failed)
	}
	if merged != 0 || blocked != 0 || skipped != 0 {
		t.Errorf("Summary merged=%d blocked=%d skipped=%d, want zero", merged, blocked, skipped)
	}
}

func TestStatus_securityFailureInActionRequired(t *testing.T) {
	s := NewPRStatus()
	idx := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 7})
	s.Update(idx, StatusFailedSecurity, "security check failed: trivy")

	ar := s.ActionRequired()
	if len(ar) != 1 {
		t.Fatalf("ActionRequired len = %d, want 1", len(ar))
	}
	if ar[0].State != StatusFailedSecurity {
		t.Errorf("entry state = %v, want StatusFailedSecurity", ar[0].State)
	}

	sec := s.SecurityFailedEntries()
	if len(sec) != 1 {
		t.Fatalf("SecurityFailedEntries len = %d, want 1", len(sec))
	}
}

func TestStatusBlockedCI_string(t *testing.T) {
	if got := StatusBlockedCI.String(); got != "CI unavailable (budget)" {
		t.Errorf("StatusBlockedCI.String() = %q, want %q", got, "CI unavailable (budget)")
	}
}

func TestStatus_blockedNotCountedAsFailed(t *testing.T) {
	s := NewPRStatus()
	idx := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 1})
	s.Update(idx, StatusBlockedCI, "Actions budget exhausted; no jobs ran")

	merged, failed, blocked, skipped := s.Summary()
	if blocked != 1 {
		t.Errorf("Summary blocked = %d, want 1", blocked)
	}
	if failed != 0 {
		t.Errorf("Summary failed = %d, want 0 (budget block must not count as failed)", failed)
	}
	if merged != 0 || skipped != 0 {
		t.Errorf("Summary merged=%d skipped=%d, want zero", merged, skipped)
	}
}

func TestStatus_blockedNotInActionRequired(t *testing.T) {
	s := NewPRStatus()
	idx := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 42})
	s.Update(idx, StatusBlockedCI, "Actions budget exhausted; no jobs ran: Test, Lint")

	if ar := s.ActionRequired(); len(ar) != 0 {
		t.Errorf("ActionRequired len = %d, want 0 (budget block must be kept out of the rescue path)", len(ar))
	}

	blocked := s.BlockedEntries()
	if len(blocked) != 1 {
		t.Fatalf("BlockedEntries len = %d, want 1", len(blocked))
	}
	if blocked[0].PR.Number != 42 {
		t.Errorf("BlockedEntries[0].Number = %d, want 42", blocked[0].PR.Number)
	}
}

func TestFormatSummary_includesBlockedWhenPresent(t *testing.T) {
	s := NewPRStatus()
	idx := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 1})
	s.Update(idx, StatusBlockedCI, "Actions budget exhausted; no jobs ran")

	if got := s.FormatSummary(); !strings.Contains(got, "CI-unavailable") {
		t.Errorf("FormatSummary() = %q, want it to mention CI-unavailable", got)
	}
}

func TestColorizeStatus_blockedIsDistinct(t *testing.T) {
	failed := ColorizeStatus(StatusFailed, "checks failed")
	blocked := ColorizeStatus(StatusBlockedCI, "Actions budget exhausted")
	skipped := ColorizeStatus(StatusSkipped, "dry-run")

	if blocked == failed {
		t.Error("ColorizeStatus for StatusBlockedCI and StatusFailed must differ")
	}
	if blocked == skipped {
		t.Error("ColorizeStatus for StatusBlockedCI and StatusSkipped must differ")
	}
}

func TestColorizeStatus_securityIsDistinct(t *testing.T) {
	regular := ColorizeStatus(StatusFailed, "checks failed")
	security := ColorizeStatus(StatusFailedSecurity, "govulncheck")

	if regular == security {
		t.Fatal("ColorizeStatus for StatusFailed and StatusFailedSecurity must differ")
	}
	if !strings.Contains(security, "security") {
		t.Errorf("security colorize output %q should contain the word 'security'", security)
	}
}
