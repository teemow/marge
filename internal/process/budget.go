package process

import (
	"fmt"
	"strings"
)

// canonicalBudgetPhrases are GitHub's exact wording for a job that never
// started because an Actions budget / spending-limit block kicked in. They
// are specific enough that finding them anywhere in a check run -- including
// its free-form output, which carries job logs -- reliably means a budget
// block rather than a genuine failure. The canonical annotation reads:
//
//	"The job was not started because an Actions budget is preventing further use."
var canonicalBudgetPhrases = []string{
	"actions budget is preventing further use",
	"the job was not started because an actions budget",
}

// spendingLimitPhrases are broader spending-limit phrases GitHub uses for
// personal accounts and organizations. They are deliberately only matched
// against check run annotations -- GitHub's authoritative location for the
// block notice -- because a genuine failure's logs could legitimately mention
// a "spending limit" and we must never reclassify a real failure as a budget
// block (that would silently hide it from the rescue path).
var spendingLimitPhrases = []string{
	"spending limit",
	"spending-limit",
}

func containsAnyFold(msg string, phrases []string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// isBudgetBlockMessage reports whether msg looks like a GitHub Actions
// budget / spending-limit block (a job that never started because billing
// quota was exhausted), as opposed to a real CI failure. It matches the full
// phrase set and is intended for annotation messages.
func isBudgetBlockMessage(msg string) bool {
	return containsAnyFold(msg, canonicalBudgetPhrases) || containsAnyFold(msg, spendingLimitPhrases)
}

// isBudgetBlockOutput reports whether a check run's output fields or
// annotation messages indicate a budget/spending-limit block. Output fields
// (title/summary/text) carry job logs, so only the unambiguous canonical
// wording is trusted there; the broader spending-limit phrases are trusted
// only in annotations. It is a pure helper so the classification can be
// exercised without hitting the GitHub API: the processor gathers the output
// strings and annotation messages and delegates the decision here.
func isBudgetBlockOutput(title, summary, text string, annotationMessages []string) bool {
	if containsAnyFold(title, canonicalBudgetPhrases) ||
		containsAnyFold(summary, canonicalBudgetPhrases) ||
		containsAnyFold(text, canonicalBudgetPhrases) {
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
