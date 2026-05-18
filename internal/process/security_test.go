package process

import "testing"

func TestClassifySecurityFailure_defaults(t *testing.T) {
	tests := []struct {
		name          string
		failedChecks  []string
		wantNonEmpty  bool
		wantMatchedIs string
	}{
		{
			name:          "govulncheck check matches",
			failedChecks:  []string{"build", "govulncheck"},
			wantNonEmpty:  true,
			wantMatchedIs: "govulncheck",
		},
		{
			name:          "Trivy check matches case-insensitively",
			failedChecks:  []string{"TRIVY: container scan"},
			wantNonEmpty:  true,
			wantMatchedIs: "TRIVY: container scan",
		},
		{
			name:          "CodeQL Analysis matches",
			failedChecks:  []string{"CodeQL Analysis (go)"},
			wantNonEmpty:  true,
			wantMatchedIs: "CodeQL Analysis (go)",
		},
		{
			name:          "Security Scan matches",
			failedChecks:  []string{"Security Scan"},
			wantNonEmpty:  true,
			wantMatchedIs: "Security Scan",
		},
		{
			name:         "build/test failure does not match",
			failedChecks: []string{"build", "unit-tests", "lint"},
			wantNonEmpty: false,
		},
		{
			name:         "no failed checks returns empty",
			failedChecks: nil,
			wantNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySecurityFailure(tt.failedChecks, DefaultSecurityCheckPatterns)
			if tt.wantNonEmpty && got == "" {
				t.Errorf("classifySecurityFailure(%v) returned empty, want non-empty", tt.failedChecks)
			}
			if !tt.wantNonEmpty && got != "" {
				t.Errorf("classifySecurityFailure(%v) = %q, want empty", tt.failedChecks, got)
			}
			if tt.wantNonEmpty && tt.wantMatchedIs != "" && got != tt.wantMatchedIs {
				t.Errorf("classifySecurityFailure(%v) = %q, want %q", tt.failedChecks, got, tt.wantMatchedIs)
			}
		})
	}
}

func TestClassifySecurityFailure_customPatterns(t *testing.T) {
	patterns := []string{"my-custom-scan"}

	if got := classifySecurityFailure([]string{"govulncheck"}, patterns); got != "" {
		t.Errorf("govulncheck should not match custom patterns; got %q", got)
	}

	if got := classifySecurityFailure([]string{"My-Custom-Scan failed"}, patterns); got == "" {
		t.Errorf("expected custom pattern to match case-insensitively")
	}
}

func TestClassifySecurityFailure_emptyPatternsDisableClassification(t *testing.T) {
	if got := classifySecurityFailure([]string{"govulncheck"}, []string{}); got != "" {
		t.Errorf("empty patterns should disable classification; got %q", got)
	}
}

func TestClassifySecurityFailure_ignoresBlankPatternEntries(t *testing.T) {
	patterns := []string{"", "   ", "trivy"}
	if got := classifySecurityFailure([]string{"build"}, patterns); got != "" {
		t.Errorf("blank pattern entries must not match arbitrary check names; got %q", got)
	}
	if got := classifySecurityFailure([]string{"Trivy"}, patterns); got == "" {
		t.Errorf("expected non-blank pattern to still match")
	}
}

func TestProcessorSecurityPatterns_defaultWhenNil(t *testing.T) {
	p := &Processor{}
	if got := p.securityPatterns(); len(got) == 0 {
		t.Fatal("expected default security patterns when SecurityCheckPatterns is nil")
	}
}

func TestProcessorSecurityPatterns_emptyDisablesClassification(t *testing.T) {
	p := &Processor{SecurityCheckPatterns: []string{}}
	got := p.securityPatterns()
	if len(got) != 0 {
		t.Fatalf("expected empty slice to be preserved (classification disabled); got %v", got)
	}
}

func TestFailureDetail(t *testing.T) {
	tests := []struct {
		name         string
		failedChecks []string
		want         string
	}{
		{"no checks", nil, "checks failed"},
		{"one check", []string{"build"}, "checks failed: build"},
		{"three checks", []string{"a", "b", "c"}, "checks failed: a, b, c"},
		{"more than three", []string{"a", "b", "c", "d", "e"}, "checks failed: a, b, c (+2 more)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := failureDetail(tt.failedChecks); got != tt.want {
				t.Errorf("failureDetail(%v) = %q, want %q", tt.failedChecks, got, tt.want)
			}
		})
	}
}
