package pr

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RescueMarker records a prior automated rescue attempt on a PR. It is
// embedded as a machine-readable HTML comment inside an ordinary PR
// comment, so any tool that can comment on a PR (a klaus agent, a Cursor
// agent, a GitHub Action) can participate without coupling to marge:
//
//	<!-- ai-rescue: {"tool":"klaus","outcome":"failed","reason":"...","head_sha":"...","at":"2026-06-09T18:40:00Z"} -->
//
// HeadSHA ties the attempt to the code it was attempted against. Renovate
// force-pushes on rebase or version change, so a marker whose head_sha no
// longer matches the PR head describes code that no longer exists -- the
// attempt is stale and the PR is fair game for another rescue.
type RescueMarker struct {
	Tool    string    `json:"tool,omitempty"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason,omitempty"`
	HeadSHA string    `json:"head_sha,omitempty"`
	At      time.Time `json:"at,omitempty"`

	// Stale is computed by MarkStale, never serialized into the marker.
	Stale bool `json:"-"`
}

var rescueMarkerRE = regexp.MustCompile(`(?s)<!--\s*ai-rescue:\s*(\{.*?\})\s*-->`)

// ParseRescueMarker extracts the last ai-rescue marker from a comment
// body. It returns nil when the body contains no parseable marker; a
// malformed JSON payload is ignored rather than treated as an error so a
// mangled comment can never break a sweep.
func ParseRescueMarker(body string) *RescueMarker {
	matches := rescueMarkerRE.FindAllStringSubmatch(body, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		var m RescueMarker
		if err := json.Unmarshal([]byte(matches[i][1]), &m); err != nil {
			continue
		}
		if m.Outcome == "" {
			continue
		}
		return &m
	}
	return nil
}

// MarkStale sets Stale by comparing the marker's recorded head SHA with
// the PR's current head. Short-vs-full SHA prefixes match. A marker
// without a recorded SHA cannot be aged out, so it stays non-stale until
// a newer marker replaces it.
func (m *RescueMarker) MarkStale(currentHeadSHA string) {
	if m.HeadSHA == "" || currentHeadSHA == "" {
		m.Stale = false
		return
	}
	m.Stale = !shaPrefixMatch(m.HeadSHA, currentHeadSHA)
}

func shaPrefixMatch(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(a) > len(b) {
		a, b = b, a
	}
	return a != "" && strings.HasPrefix(b, a)
}

// CommentBody renders the full PR comment for this marker: a
// human-readable summary followed by the machine-readable marker.
func (m *RescueMarker) CommentBody() string {
	tool := m.Tool
	if tool == "" {
		tool = "ai"
	}
	human := fmt.Sprintf("**AI rescue %s** (%s)", m.Outcome, tool)
	if m.Reason != "" {
		human += ": " + m.Reason
	}

	payload, _ := json.Marshal(m)
	return fmt.Sprintf("%s\n\n<!-- ai-rescue: %s -->", human, payload)
}

// FormatRescue renders the marker as a short human-readable annotation
// for status output, e.g. "rescue failed 1d ago (klaus): ESM-only" or
// "rescue failed 3d ago (klaus), stale: new commits since".
func FormatRescue(m *RescueMarker, now time.Time) string {
	if m == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("rescue ")
	b.WriteString(m.Outcome)
	if !m.At.IsZero() {
		b.WriteString(" ")
		b.WriteString(FormatAge(m.At, now))
		b.WriteString(" ago")
	}
	if m.Tool != "" {
		fmt.Fprintf(&b, " (%s)", m.Tool)
	}
	if m.Stale {
		b.WriteString(", stale: new commits since")
	} else if m.Reason != "" {
		b.WriteString(": ")
		b.WriteString(m.Reason)
	}
	return b.String()
}

// ColorizeRescue wraps the FormatRescue annotation in ANSI colors: a
// stale marker renders yellow (retriable -- the code changed since the
// attempt), a fresh one bold red (a rescue already failed on exactly
// this code; a human is needed).
func ColorizeRescue(m *RescueMarker, now time.Time) string {
	s := FormatRescue(m, now)
	if s == "" {
		return ""
	}
	if m.Stale {
		return fmt.Sprintf("\033[33m[%s]\033[0m", s)
	}
	return fmt.Sprintf("\033[1;91m[%s]\033[0m", s)
}
