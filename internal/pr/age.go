package pr

import (
	"fmt"
	"time"
)

// Age thresholds: a PR older than ageWarnAfter is highlighted as aging
// (yellow); older than ageStaleAfter it is highlighted as stale (red).
// A dependency PR that has been open for more than a few days has, in
// practice, already survived at least one automated sweep -- it is the
// population most likely to need manual work.
const (
	ageWarnAfter  = 3 * 24 * time.Hour
	ageStaleAfter = 7 * 24 * time.Hour
)

// FormatAge renders the time since created as a compact human string
// ("5h", "3d", "2w"). It returns "" for a zero creation time so callers
// can omit the column value when the discovery path did not provide it.
func FormatAge(created, now time.Time) string {
	if created.IsZero() {
		return ""
	}
	d := now.Sub(created)
	if d < 0 {
		d = 0
	}
	days := int(d.Hours() / 24)
	switch {
	case days < 1:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case days < 14:
		return fmt.Sprintf("%dd", days)
	default:
		return fmt.Sprintf("%dw", days/7)
	}
}

// AgeDays returns the whole number of days since created, or 0 for a
// zero creation time.
func AgeDays(created, now time.Time) int {
	if created.IsZero() {
		return 0
	}
	d := now.Sub(created)
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}

// AgeColorCode returns the ANSI color prefix for a PR age: "" while the
// PR is fresh, yellow once it passes ageWarnAfter, red once it passes
// ageStaleAfter. Callers must reset with \033[0m when non-empty.
func AgeColorCode(created, now time.Time) string {
	if created.IsZero() {
		return ""
	}
	age := now.Sub(created)
	switch {
	case age >= ageStaleAfter:
		return "\033[31m" // red
	case age >= ageWarnAfter:
		return "\033[33m" // yellow
	default:
		return ""
	}
}

// AgeInfoFunc is the table-column accessor for the Age column.
func AgeInfoFunc(p PRInfo) string {
	return FormatAge(p.CreatedAt, time.Now())
}

// AgeColorFunc is the table-column color accessor for the Age column.
func AgeColorFunc(p PRInfo) string {
	return AgeColorCode(p.CreatedAt, time.Now())
}
