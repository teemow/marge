package pr

import (
	"strings"
	"testing"
	"time"
)

func TestParseRescueMarker(t *testing.T) {
	body := `Rescue attempt failed: nock v14 is ESM-only and breaks Jest CJS resolution.

<!-- ai-rescue: {"tool":"klaus","outcome":"failed","reason":"ESM-only breaks Jest CJS","head_sha":"d9f00bf2","at":"2026-06-09T18:40:00Z"} -->`

	m := ParseRescueMarker(body)
	if m == nil {
		t.Fatal("ParseRescueMarker returned nil")
	}
	if m.Tool != "klaus" {
		t.Errorf("Tool = %q, want %q", m.Tool, "klaus")
	}
	if m.Outcome != "failed" {
		t.Errorf("Outcome = %q, want %q", m.Outcome, "failed")
	}
	if m.Reason != "ESM-only breaks Jest CJS" {
		t.Errorf("Reason = %q", m.Reason)
	}
	if m.HeadSHA != "d9f00bf2" {
		t.Errorf("HeadSHA = %q", m.HeadSHA)
	}
	want := time.Date(2026, 6, 9, 18, 40, 0, 0, time.UTC)
	if !m.At.Equal(want) {
		t.Errorf("At = %v, want %v", m.At, want)
	}
}

func TestParseRescueMarker_noMarker(t *testing.T) {
	if m := ParseRescueMarker("just a regular comment"); m != nil {
		t.Errorf("expected nil, got %+v", m)
	}
}

func TestParseRescueMarker_malformedJSONIgnored(t *testing.T) {
	if m := ParseRescueMarker(`<!-- ai-rescue: {not json} -->`); m != nil {
		t.Errorf("expected nil for malformed JSON, got %+v", m)
	}
}

func TestParseRescueMarker_missingOutcomeIgnored(t *testing.T) {
	if m := ParseRescueMarker(`<!-- ai-rescue: {"tool":"klaus"} -->`); m != nil {
		t.Errorf("expected nil for marker without outcome, got %+v", m)
	}
}

func TestParseRescueMarker_lastMarkerWins(t *testing.T) {
	body := `<!-- ai-rescue: {"outcome":"failed","reason":"first"} -->
<!-- ai-rescue: {"outcome":"blocked","reason":"second"} -->`

	m := ParseRescueMarker(body)
	if m == nil {
		t.Fatal("ParseRescueMarker returned nil")
	}
	if m.Reason != "second" {
		t.Errorf("Reason = %q, want %q (last marker should win)", m.Reason, "second")
	}
}

func TestMarkStale(t *testing.T) {
	tests := []struct {
		name      string
		markerSHA string
		headSHA   string
		wantStale bool
	}{
		{"same full SHA", "d9f00bf2d90e7dc6b3013a975c08d80766542745", "d9f00bf2d90e7dc6b3013a975c08d80766542745", false},
		{"short marker SHA matches head prefix", "d9f00bf2", "d9f00bf2d90e7dc6b3013a975c08d80766542745", false},
		{"different SHA", "d9f00bf2", "edf33f4d1cff7cab190b39689bd3ac77b6bd0910", true},
		{"marker without SHA never stale", "", "edf33f4d", false},
		{"unknown current head never stale", "d9f00bf2", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &RescueMarker{Outcome: "failed", HeadSHA: tt.markerSHA}
			m.MarkStale(tt.headSHA)
			if m.Stale != tt.wantStale {
				t.Errorf("Stale = %v, want %v", m.Stale, tt.wantStale)
			}
		})
	}
}

func TestCommentBody_roundTrips(t *testing.T) {
	orig := &RescueMarker{
		Tool:    "klaus",
		Outcome: "failed",
		Reason:  "pytest-helm-charts requires pytest<9",
		HeadSHA: "d9f00bf2",
		At:      time.Date(2026, 6, 9, 18, 40, 0, 0, time.UTC),
	}

	body := orig.CommentBody()

	if !strings.Contains(body, "**AI rescue failed** (klaus): pytest-helm-charts requires pytest<9") {
		t.Errorf("human-readable part missing or wrong:\n%s", body)
	}

	parsed := ParseRescueMarker(body)
	if parsed == nil {
		t.Fatal("CommentBody output did not parse back")
	}
	if parsed.Tool != orig.Tool || parsed.Outcome != orig.Outcome || parsed.Reason != orig.Reason || parsed.HeadSHA != orig.HeadSHA || !parsed.At.Equal(orig.At) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", parsed, orig)
	}
}

func TestFormatRescue(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)

	fresh := &RescueMarker{Tool: "klaus", Outcome: "failed", Reason: "ESM-only", At: now.Add(-25 * time.Hour)}
	if got := FormatRescue(fresh, now); got != "rescue failed 1d ago (klaus): ESM-only" {
		t.Errorf("fresh = %q", got)
	}

	stale := &RescueMarker{Tool: "klaus", Outcome: "failed", Reason: "ESM-only", At: now.Add(-25 * time.Hour), Stale: true}
	if got := FormatRescue(stale, now); got != "rescue failed 1d ago (klaus), stale: new commits since" {
		t.Errorf("stale = %q", got)
	}

	if got := FormatRescue(nil, now); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
}
