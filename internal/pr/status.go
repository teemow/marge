package pr

import (
	"fmt"
	"sort"
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
	StatusFailedSecurity
	StatusBlockedCI
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
	case StatusFailedSecurity:
		return "Failed (security)"
	case StatusBlockedCI:
		return "CI unavailable (budget)"
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
	// Rescue is the most recent prior automated rescue attempt found on
	// the PR, if any. Only populated for failure-state entries.
	Rescue *RescueMarker
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

// SetRescue attaches a prior rescue-attempt marker to an entry.
func (s *PRStatus) SetRescue(idx int, marker *RescueMarker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.entries) {
		s.entries[idx].Rescue = marker
	}
}

// StateAt returns the current state of the entry at idx, or
// StatusPending when idx is out of range.
func (s *PRStatus) StateAt(idx int) StatusState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < len(s.entries) {
		return s.entries[idx].State
	}
	return StatusPending
}

func (s *PRStatus) Snapshot() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := make([]StatusEntry, len(s.entries))
	copy(snap, s.entries)
	return snap
}

// countsLocked tallies entries by category. blocked (CI could not run because
// of an Actions budget block) is counted separately from failed so a billing
// block is never mistaken for a genuine CI failure. Callers must hold s.mu.
func (s *PRStatus) countsLocked() (merged, failed, blocked, skipped int) {
	for _, e := range s.entries {
		switch e.State {
		case StatusMerged, StatusAlreadyMerged, StatusAutoMerge:
			merged++
		case StatusFailed, StatusFailedSecurity, StatusConflict, StatusUntrustedAuthor:
			failed++
		case StatusBlockedCI:
			blocked++
		case StatusSkipped:
			skipped++
		}
	}
	return
}

// Summary returns aggregate counts across all entries.
func (s *PRStatus) Summary() (merged, failed, blocked, skipped int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countsLocked()
}

func (s *PRStatus) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *PRStatus) FormatSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged, failed, blocked, skipped := s.countsLocked()
	total := len(s.entries)
	if blocked > 0 {
		return fmt.Sprintf("%d PRs processed: %d merged, %d failed, %d CI-unavailable, %d skipped",
			total, merged, failed, blocked, skipped)
	}
	return fmt.Sprintf("%d PRs processed: %d merged, %d failed, %d skipped", total, merged, failed, skipped)
}

// ActionRequired returns the failure entries, oldest PR first: the
// longer a dependency PR has been open, the more sweeps it has already
// survived, so the old ones are the most likely to need manual work.
// Entries without a known creation time sort last, in insertion order.
func (s *PRStatus) ActionRequired() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		switch e.State {
		case StatusFailed, StatusFailedSecurity, StatusConflict, StatusUntrustedAuthor:
			result = append(result, e)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		ci, cj := result[i].PR.CreatedAt, result[j].PR.CreatedAt
		if ci.IsZero() || cj.IsZero() {
			return !ci.IsZero()
		}
		return ci.Before(cj)
	})
	return result
}

// BlockedEntries returns entries whose CI could not run because a GitHub
// Actions budget / spending-limit block prevented every job from starting.
// These are deliberately kept out of ActionRequired and the failed counts:
// the remedy is "raise or await the Actions budget", not "rescue the code".
func (s *PRStatus) BlockedEntries() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		if e.State == StatusBlockedCI {
			result = append(result, e)
		}
	}
	return result
}

// SecurityFailedEntries returns entries that failed specifically because a
// security-related check reported a problem.
func (s *PRStatus) SecurityFailedEntries() []StatusEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []StatusEntry
	for _, e := range s.entries {
		if e.State == StatusFailedSecurity {
			result = append(result, e)
		}
	}
	return result
}

// SplitActionRequired partitions the action-required list into security
// failures and everything else, preserving the input order in each group.
func SplitActionRequired(entries []StatusEntry) (security, other []StatusEntry) {
	for _, e := range entries {
		if e.State == StatusFailedSecurity {
			security = append(security, e)
		} else {
			other = append(other, e)
		}
	}
	return
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
