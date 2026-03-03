package pr

import (
	"fmt"
	"os"
	"strings"
)

const (
	colPR     = 10
	colInfo   = 40
	colAuthor = 18
	colStatus = 30
)

type InfoFunc func(PRInfo) string

func RepoInfoFunc(p PRInfo) string {
	return fmt.Sprintf("%s/%s", p.Owner, p.Repo)
}

func DependencyInfoFunc(p PRInfo) string {
	dep := ExtractDependencyName(p.Title)
	if dep == "" {
		return p.Title
	}
	return dep
}

func MakeHyperlink(text, url string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

func PrintTableHeader(w *os.File, infoLabel string) {
	header := fmt.Sprintf("%-*s %-*s %-*s %-*s", colPR, "PR", colInfo, infoLabel, colAuthor, "Author", colStatus, "Status")
	divider := strings.Repeat("-", colPR+colInfo+colAuthor+colStatus+3)
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintln(w, divider)
}

func PrintRow(w *os.File, e StatusEntry, infoFn InfoFunc) {
	prLabel := fmt.Sprintf("%-*s", colPR, fmt.Sprintf("#%d", e.PR.Number))
	prLink := MakeHyperlink(prLabel, e.PR.URL)
	info := infoFn(e.PR)
	statusStr := colorizeStatus(e.State, e.Detail)
	_, _ = fmt.Fprintf(w, "\033[2K%s %-*s %-*s %s\n", prLink, colInfo, truncate(info, colInfo), colAuthor, truncate(e.PR.Author, colAuthor), statusStr)
}

func UpdateTable(w *os.File, entries []StatusEntry, infoLabel string, infoFn InfoFunc) {
	lineCount := len(entries) + 2 // +2 for header and divider

	// Move cursor up to overwrite the table
	_, _ = fmt.Fprintf(w, "\033[%dA", lineCount)

	PrintTableHeader(w, infoLabel)

	for _, e := range entries {
		PrintRow(w, e, infoFn)
	}
}

func colorizeStatus(state StatusState, detail string) string {
	label := state.String()
	if detail != "" {
		label = fmt.Sprintf("%s (%s)", label, detail)
	}

	switch state {
	case StatusMerged, StatusAlreadyMerged, StatusAutoMerge:
		return fmt.Sprintf("\033[32m%s\033[0m", label) // green
	case StatusFailed, StatusConflict, StatusUntrustedAuthor:
		return fmt.Sprintf("\033[31m%s\033[0m", label) // red
	case StatusSkipped:
		return fmt.Sprintf("\033[33m%s\033[0m", label) // yellow
	case StatusChecking, StatusApproving, StatusMerging:
		return fmt.Sprintf("\033[36m%s\033[0m", label) // cyan
	default:
		return label
	}
}

func PrintPlainResults(w *os.File, status *PRStatus) {
	merged := status.MergedEntries()
	failed := status.ActionRequired()
	skipped := status.SkippedEntries()

	if len(merged) > 0 {
		_, _ = fmt.Fprintf(w, "Merged (%d):\n", len(merged))
		for _, e := range merged {
			detail := e.State.String()
			if e.Detail != "" {
				detail = fmt.Sprintf("%s (%s)", detail, e.Detail)
			}
			_, _ = fmt.Fprintf(w, "  #%-6d %s/%s - %s [%s]\n", e.PR.Number, e.PR.Owner, e.PR.Repo, e.PR.Title, detail)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(failed) > 0 {
		_, _ = fmt.Fprintf(w, "Failed (%d):\n", len(failed))
		for _, e := range failed {
			detail := e.State.String()
			if e.Detail != "" {
				detail = fmt.Sprintf("%s (%s)", detail, e.Detail)
			}
			_, _ = fmt.Fprintf(w, "  #%-6d %s/%s - %s [%s]\n", e.PR.Number, e.PR.Owner, e.PR.Repo, e.PR.Title, detail)
			_, _ = fmt.Fprintf(w, "         %s\n", e.PR.URL)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(skipped) > 0 {
		_, _ = fmt.Fprintf(w, "Skipped (%d):\n", len(skipped))
		for _, e := range skipped {
			detail := e.State.String()
			if e.Detail != "" {
				detail = fmt.Sprintf("%s (%s)", detail, e.Detail)
			}
			_, _ = fmt.Fprintf(w, "  #%-6d %s/%s - %s [%s]\n", e.PR.Number, e.PR.Owner, e.PR.Repo, e.PR.Title, detail)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
