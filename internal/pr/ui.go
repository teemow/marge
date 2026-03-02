package pr

import (
	"fmt"
	"os"
	"strings"
)

const (
	colPR     = 10
	colInfo   = 50
	colStatus = 30
)

func MakeHyperlink(text, url string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

func PrintTableHeader(w *os.File) {
	header := fmt.Sprintf("%-*s %-*s %-*s", colPR, "PR", colInfo, "Repository", colStatus, "Status")
	divider := strings.Repeat("-", colPR+colInfo+colStatus+2)
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintln(w, divider)
}

func UpdateTable(w *os.File, entries []StatusEntry) {
	lineCount := len(entries) + 2 // +2 for header and divider

	// Move cursor up to overwrite the table
	_, _ = fmt.Fprintf(w, "\033[%dA", lineCount)

	PrintTableHeader(w)

	for _, e := range entries {
		prLabel := fmt.Sprintf("#%d", e.PR.Number)
		prLink := MakeHyperlink(prLabel, e.PR.URL)
		repoName := fmt.Sprintf("%s/%s", e.PR.Owner, e.PR.Repo)

		statusStr := colorizeStatus(e.State, e.Detail)

		_, _ = fmt.Fprintf(w, "\033[2K%-*s %-*s %s\n", colPR, prLink, colInfo, truncate(repoName, colInfo), statusStr)
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
	case StatusFailed, StatusConflict:
		return fmt.Sprintf("\033[31m%s\033[0m", label) // red
	case StatusSkipped:
		return fmt.Sprintf("\033[33m%s\033[0m", label) // yellow
	case StatusChecking, StatusApproving, StatusMerging:
		return fmt.Sprintf("\033[36m%s\033[0m", label) // cyan
	default:
		return label
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
