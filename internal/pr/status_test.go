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

	merged, failed, skipped := s.Summary()
	if failed != 1 {
		t.Errorf("Summary failed = %d, want 1", failed)
	}
	if merged != 0 || skipped != 0 {
		t.Errorf("Summary merged=%d skipped=%d, want zero", merged, skipped)
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
