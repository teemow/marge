package pr

import (
	"fmt"
	"sync"
)

type StatusState int

const (
	StatusPending StatusState = iota
	StatusChecking
	StatusApproving
	StatusMerging
	StatusRetrying
	StatusMerged
	StatusAlreadyMerged
	StatusAutoMerge
	StatusFailed
	StatusSkipped
	StatusConflict
	StatusUntrustedAuthor
)

func (s StatusState) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusChecking:
		return "Checking CI"
	case StatusApproving:
		return "Approving"
	case StatusMerging:
		return "Merging"
	case StatusRetrying:
		return "Retrying merge"
	case StatusMerged:
		return "Merged"
	case StatusAlreadyMerged:
		return "Already merged"
	case StatusAutoMerge:
		return "Auto-merge"
	case StatusFailed:
		return "Failed"
	case StatusSkipped:
		return "Skipped"
	case StatusConflict:
		return "Conflict"
	case StatusUntrustedAuthor:
		return "Untrusted author"
	default:
		return "Unknown"
	}
}

type PRStatus struct {
	mu      sync.Mutex
	entries []StatusEntry
}

type StatusEntry struct {
	PR     PRInfo
	State  StatusState
	Detail string
}

func NewPRStatus() *PRStatus {
	return &PRStatus{}
}

func (s *PRStatus) Add(pr PRInfo) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.entries)
	s.entries = append(s.entries, StatusEntry{
		PR:    pr,
		State: StatusPending,
	})
	return idx
}

func (s *PRStatus) Update(idx int, state StatusState, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.entries) {
		s.entries[idx].State = state
		s.entries[idx].Detail = detail
	}
}

func (s *PRStatus) Snapshot() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := make([]StatusEntry, len(s.entries))
	copy(snap, s.entries)
	return snap
}

func (s *PRStatus) Summary() (merged, failed, skipped int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		switch e.State {
		case StatusMerged, StatusAlreadyMerged, StatusAutoMerge:
			merged++
		case StatusFailed, StatusConflict, StatusUntrustedAuthor:
			failed++
		case StatusSkipped:
			skipped++
		}
	}
	return
}

func (s *PRStatus) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *PRStatus) FormatSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var merged, failed, skipped int
	for _, e := range s.entries {
		switch e.State {
		case StatusMerged, StatusAlreadyMerged, StatusAutoMerge:
			merged++
		case StatusFailed, StatusConflict, StatusUntrustedAuthor:
			failed++
		case StatusSkipped:
			skipped++
		}
	}
	total := len(s.entries)
	return fmt.Sprintf("%d PRs processed: %d merged, %d failed, %d skipped", total, merged, failed, skipped)
}

func (s *PRStatus) ActionRequired() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		switch e.State {
		case StatusFailed, StatusConflict, StatusUntrustedAuthor:
			result = append(result, e)
		}
	}
	return result
}

func (s *PRStatus) MergedEntries() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		switch e.State {
		case StatusMerged, StatusAlreadyMerged, StatusAutoMerge:
			result = append(result, e)
		}
	}
	return result
}

func (s *PRStatus) SkippedEntries() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		if e.State == StatusSkipped {
			result = append(result, e)
		}
	}
	return result
}
