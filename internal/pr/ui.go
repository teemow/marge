package pr

import (
	"fmt"
	"os"
	"strings"
)

const (
	colPR     = 8
	colStatus = 24
)

type TableColumn struct {
	Label string
	Width int
	Fn    func(PRInfo) string
}

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

func VersionInfoFunc(p PRInfo) string {
	return ExtractVersion(p.Title)
}

func AuthorInfoFunc(p PRInfo) string {
	return p.Author
}

func FullColumns() []TableColumn {
	return []TableColumn{
		{"Repository", 22, RepoInfoFunc},
		{"Dependency", 22, DependencyInfoFunc},
		{"Version", 18, VersionInfoFunc},
		{"Author", 16, AuthorInfoFunc},
	}
}

func RepoSelectedColumns() []TableColumn {
	return []TableColumn{
		{"Dependency", 28, DependencyInfoFunc},
		{"Version", 18, VersionInfoFunc},
		{"Author", 16, AuthorInfoFunc},
	}
}

func DependencySelectedColumns() []TableColumn {
	return []TableColumn{
		{"Repository", 28, RepoInfoFunc},
		{"Version", 18, VersionInfoFunc},
		{"Author", 16, AuthorInfoFunc},
	}
}

func MakeHyperlink(text, url string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

func tableWidth(cols []TableColumn) int {
	w := colPR + colStatus
	for _, c := range cols {
		w += c.Width
	}
	w += len(cols) + 1
	return w
}

func PrintTableHeader(w *os.File, cols []TableColumn) {
	parts := make([]string, 0, len(cols)+2)
	parts = append(parts, fmt.Sprintf("%-*s", colPR, "PR"))
	for _, c := range cols {
		parts = append(parts, fmt.Sprintf("%-*s", c.Width, c.Label))
	}
	parts = append(parts, fmt.Sprintf("%-*s", colStatus, "Status"))
	header := strings.Join(parts, " ")
	divider := strings.Repeat("-", tableWidth(cols))
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintln(w, divider)
}

func PrintRow(w *os.File, e StatusEntry, cols []TableColumn) {
	prLabel := fmt.Sprintf("%-*s", colPR, fmt.Sprintf("#%d", e.PR.Number))
	prLink := MakeHyperlink(prLabel, e.PR.URL)

	parts := make([]string, 0, len(cols)+2)
	parts = append(parts, prLink)
	for _, c := range cols {
		parts = append(parts, fmt.Sprintf("%-*s", c.Width, truncate(c.Fn(e.PR), c.Width)))
	}
	parts = append(parts, ColorizeStatus(e.State, e.Detail))

	_, _ = fmt.Fprintf(w, "\033[2K%s\n", strings.Join(parts, " "))
}

func UpdateTable(w *os.File, entries []StatusEntry, cols []TableColumn) {
	lineCount := len(entries) + 2

	_, _ = fmt.Fprintf(w, "\033[%dA", lineCount)

	PrintTableHeader(w, cols)

	for _, e := range entries {
		PrintRow(w, e, cols)
	}
}

func ColorizeStatus(state StatusState, detail string) string {
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
			printPlainEntry(w, e)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(failed) > 0 {
		_, _ = fmt.Fprintf(w, "Failed (%d):\n", len(failed))
		for _, e := range failed {
			printPlainEntry(w, e)
			_, _ = fmt.Fprintf(w, "         %s\n", e.PR.URL)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(skipped) > 0 {
		_, _ = fmt.Fprintf(w, "Skipped (%d):\n", len(skipped))
		for _, e := range skipped {
			printPlainEntry(w, e)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func printPlainEntry(w *os.File, e StatusEntry) {
	detail := e.State.String()
	if e.Detail != "" {
		detail = fmt.Sprintf("%s (%s)", detail, e.Detail)
	}
	dep := ExtractDependencyName(e.PR.Title)
	ver := ExtractVersion(e.PR.Title)
	if dep == "" {
		dep = e.PR.Title
	}
	verStr := ""
	if ver != "" {
		verStr = " " + ver
	}
	_, _ = fmt.Fprintf(w, "  #%-6d %s/%s  %s%s  [%s] [%s]\n",
		e.PR.Number, e.PR.Owner, e.PR.Repo, dep, verStr, e.PR.Author, detail)
}

// AdjustColumnWidths widens each column so every value fits without truncation.
func AdjustColumnWidths(cols []TableColumn, prs []PRInfo) {
	for i, c := range cols {
		w := len(c.Label)
		for _, p := range prs {
			if v := len(c.Fn(p)); v > w {
				w = v
			}
		}
		cols[i].Width = w
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
