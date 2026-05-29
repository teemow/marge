package process

import "testing"

func TestIsBudgetBlockMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{
			// Exact annotation message captured from the live GitHub API on a
			// real budget-blocked check run (conclusion "failure", message in
			// the annotation). Locked in as a regression guard.
			name: "actions budget message (API-verified annotation message)",
			msg:  "The job was not started because an Actions budget is preventing further use.",
			want: true,
		},
		{
			name: "actions budget message with trailing text",
			msg:  "The job was not started because an Actions budget is preventing further use. This job failed",
			want: true,
		},
		{
			name: "actions budget phrase case-insensitive",
			msg:  "ACTIONS BUDGET IS PREVENTING FURTHER USE",
			want: true,
		},
		// Unverified billing/payment/spending-limit wording is intentionally
		// NOT matched: an unrecognized block degrades to the normal Failed path
		// rather than risking a real failure being hidden.
		{
			name: "unverified spending-limit wording does not match",
			msg:  "The job was not started because recent account payments have failed or your spending limit needs to be increased.",
			want: false,
		},
		{
			name: "unverified billing-issue wording does not match",
			msg:  "The job was not started because your account is locked due to a billing issue.",
			want: false,
		},
		// A genuine failure whose text merely mentions billing words must not be
		// reclassified -- that would hide a real failure from the rescue path.
		{
			name: "genuine failure mentioning billing words does not match",
			msg:  "billing_test.go: expected spending limit 100, got 0",
			want: false,
		},
		{
			name: "real test failure does not match",
			msg:  "Expected 200 but got 500",
			want: false,
		},
		{
			name: "lint failure does not match",
			msg:  "undefined: foo",
			want: false,
		},
		{
			name: "empty message",
			msg:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBudgetBlockMessage(tt.msg); got != tt.want {
				t.Errorf("isBudgetBlockMessage(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestIsBudgetBlockOutput(t *testing.T) {
	const budget = "The job was not started because an Actions budget is preventing further use."

	tests := []struct {
		name        string
		title       string
		summary     string
		text        string
		annotations []string
		want        bool
	}{
		{
			name:  "budget message in title",
			title: budget,
			want:  true,
		},
		{
			name:    "budget message in summary",
			summary: budget,
			want:    true,
		},
		{
			name: "budget message in text",
			text: budget,
			want: true,
		},
		{
			name:        "budget message in annotation",
			annotations: []string{"some context", budget},
			want:        true,
		},
		{
			name:    "genuine failure mentioning billing in output does not match",
			title:   "Tests failed",
			summary: "billing service: spending limit assertion failed",
			want:    false,
		},
		{
			name:        "genuine failure annotations do not match",
			title:       "Tests failed",
			annotations: []string{"assertion failed", "exit code 1"},
			want:        false,
		},
		{
			name: "all empty",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBudgetBlockOutput(tt.title, tt.summary, tt.text, tt.annotations)
			if got != tt.want {
				t.Errorf("isBudgetBlockOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockedDetail(t *testing.T) {
	tests := []struct {
		name          string
		blockedChecks []string
		want          string
	}{
		{
			name:          "no checks",
			blockedChecks: nil,
			want:          "Actions budget exhausted; no jobs ran",
		},
		{
			name:          "one check",
			blockedChecks: []string{"Test"},
			want:          "Actions budget exhausted; no jobs ran: Test",
		},
		{
			name:          "three checks",
			blockedChecks: []string{"Test", "Lint", "Frontend"},
			want:          "Actions budget exhausted; no jobs ran: Test, Lint, Frontend",
		},
		{
			name:          "more than three checks",
			blockedChecks: []string{"Test", "Lint", "Frontend", "backend"},
			want:          "Actions budget exhausted; no jobs ran: Test, Lint, Frontend (+1 more)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blockedDetail(tt.blockedChecks); got != tt.want {
				t.Errorf("blockedDetail(%v) = %q, want %q", tt.blockedChecks, got, tt.want)
			}
		})
	}
}
