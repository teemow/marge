package pr

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		created time.Time
		want    string
	}{
		{"zero time", time.Time{}, ""},
		{"five hours", now.Add(-5 * time.Hour), "5h"},
		{"three days", now.Add(-3 * 24 * time.Hour), "3d"},
		{"thirteen days stays in days", now.Add(-13 * 24 * time.Hour), "13d"},
		{"two weeks", now.Add(-14 * 24 * time.Hour), "2w"},
		{"future clamps to zero", now.Add(2 * time.Hour), "0h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatAge(tt.created, now); got != tt.want {
				t.Errorf("FormatAge = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAgeColorCode(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	if got := AgeColorCode(time.Time{}, now); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
	if got := AgeColorCode(now.Add(-24*time.Hour), now); got != "" {
		t.Errorf("fresh = %q, want empty", got)
	}
	if got := AgeColorCode(now.Add(-4*24*time.Hour), now); got != "\033[33m" {
		t.Errorf("aging = %q, want yellow", got)
	}
	if got := AgeColorCode(now.Add(-8*24*time.Hour), now); got != "\033[31m" {
		t.Errorf("stale = %q, want red", got)
	}
}

func TestAgeDays(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	if got := AgeDays(time.Time{}, now); got != 0 {
		t.Errorf("zero time = %d, want 0", got)
	}
	if got := AgeDays(now.Add(-12*24*time.Hour), now); got != 12 {
		t.Errorf("twelve days = %d, want 12", got)
	}
}

func TestActionRequired_sortedOldestFirst(t *testing.T) {
	now := time.Now()
	s := NewPRStatus()

	young := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 2, CreatedAt: now.Add(-1 * 24 * time.Hour)})
	old := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 1, CreatedAt: now.Add(-10 * 24 * time.Hour)})
	noDate := s.Add(PRInfo{Owner: "o", Repo: "r", Number: 3})

	s.Update(young, StatusFailed, "")
	s.Update(old, StatusFailed, "")
	s.Update(noDate, StatusFailed, "")

	ar := s.ActionRequired()
	if len(ar) != 3 {
		t.Fatalf("ActionRequired len = %d, want 3", len(ar))
	}
	if ar[0].PR.Number != 1 {
		t.Errorf("first entry = #%d, want #1 (oldest)", ar[0].PR.Number)
	}
	if ar[1].PR.Number != 2 {
		t.Errorf("second entry = #%d, want #2", ar[1].PR.Number)
	}
	if ar[2].PR.Number != 3 {
		t.Errorf("third entry = #%d, want #3 (no creation date sorts last)", ar[2].PR.Number)
	}
}
