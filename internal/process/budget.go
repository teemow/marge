package process

import (
	"fmt"
	"strings"
)

// Detection is based on the messages GitHub actually emits when a job is
// blocked before it can start because of a billing / Actions-budget /
// spending-limit problem. Real-world examples (verified against GitHub
// documentation and reported incidents -- the conclusion is `failure` or
// `startup_failure`, and the text is surfaced as a failure-level annotation):
//
//	"The job was not started because an Actions budget is preventing further use."
//	"The job was not started because recent account payments have failed or your
//	 spending limit needs to be increased. ..."
//	"The job was not started because your account is locked due to a billing issue."
//
// The constant signal across every variant is the platform prefix
// "job was not started because": GitHub uses it for jobs that never reach the
// runner, and a genuine test/build/lint failure never produces it. We treat
// that prefix as the high-confidence anchor and require a billing marker
// alongside it so unrelated "job was not started" reasons (e.g. concurrency
// cancellation) are not misread as a budget block.

// jobNotStartedPhrase is the platform-level prefix GitHub uses for a job that
// never started. On its own it is not necessarily billing-related, so it is
// only treated as a budget block when paired with a billingMarker.
const jobNotStartedPhrase = "job was not started because"

// standaloneBudgetPhrases are unambiguous on their own: they cannot plausibly
// appear in a genuine CI failure, so finding one anywhere is sufficient.
var standaloneBudgetPhrases = []string{
	"actions budget is preventing further use",
}

// billingMarkers indicate a billing / budget / spending-limit cause. They are
// only trusted in combination with jobNotStartedPhrase, because words like
// "billing" or "payment" can legitimately appear in a real failure's output
// (e.g. a billing service's failing tests) and must never on their own cause
// a genuine failure to be hidden from the rescue path.
var billingMarkers = []string{
	"budget",
	"spending limit",
	"spending-limit",
	"billing",
	"payment",
}

// isBudgetBlockMessage reports whether msg looks like a GitHub job that was
// blocked before starting because of a billing / Actions-budget /
// spending-limit problem, as opposed to a real CI failure. A match requires
// either an unambiguous standalone phrase, or the "job was not started
// because" prefix together with a billing marker.
func isBudgetBlockMessage(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	for _, phrase := range standaloneBudgetPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	if !strings.Contains(lower, jobNotStartedPhrase) {
		return false
	}
	for _, marker := range billingMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// isBudgetBlockOutput reports whether any of a check run's output fields or
// annotation messages indicate a budget / billing block. Because
// isBudgetBlockMessage already anchors on the platform "job was not started
// because" prefix (or an unambiguous standalone phrase), it is safe to apply
// uniformly to the free-form output fields and the annotations alike. It is a
// pure helper so the classification can be exercised without hitting the
// GitHub API: the processor gathers the output strings and annotation
// messages and delegates the decision here.
func isBudgetBlockOutput(title, summary, text string, annotationMessages []string) bool {
	if isBudgetBlockMessage(title) || isBudgetBlockMessage(summary) || isBudgetBlockMessage(text) {
		return true
	}
	for _, m := range annotationMessages {
		if isBudgetBlockMessage(m) {
			return true
		}
	}
	return false
}

// blockedDetail builds a human-readable detail string for a PR whose CI
// could not run because of an Actions budget block, naming the affected
// checks when available. It mirrors failureDetail so the two buckets read
// consistently in the summary.
func blockedDetail(blockedChecks []string) string {
	if len(blockedChecks) == 0 {
		return "Actions budget exhausted; no jobs ran"
	}
	const maxShow = 3
	if len(blockedChecks) <= maxShow {
		return fmt.Sprintf("Actions budget exhausted; no jobs ran: %s", strings.Join(blockedChecks, ", "))
	}
	return fmt.Sprintf("Actions budget exhausted; no jobs ran: %s (+%d more)",
		strings.Join(blockedChecks[:maxShow], ", "), len(blockedChecks)-maxShow)
}
