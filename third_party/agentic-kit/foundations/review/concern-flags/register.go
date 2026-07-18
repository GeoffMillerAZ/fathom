package concernflags

import (
	"fmt"
	"sync"
)

// Register holds all raised ConcernFlags and provides safe concurrent access.
type Register struct {
	mu    sync.RWMutex
	flags []ConcernFlag
	byID  map[string]struct{}
}

// Raise records a new ConcernFlag in the register.
// Returns an error if a flag with the same ID has already been raised.
func (r *Register) Raise(flag ConcernFlag) error {
	if flag.ID == "" {
		return fmt.Errorf("concernflags: ConcernFlag.ID must not be empty")
	}
	if flag.Target == "" {
		return fmt.Errorf("concernflags: ConcernFlag.Target must not be empty")
	}
	if err := validateSeverity(flag.Severity); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byID == nil {
		r.byID = make(map[string]struct{})
	}
	if _, exists := r.byID[flag.ID]; exists {
		return fmt.Errorf("concernflags: flag with ID %q already registered", flag.ID)
	}
	r.byID[flag.ID] = struct{}{}
	r.flags = append(r.flags, flag)
	return nil
}

// List returns a snapshot of all raised ConcernFlags in registration order.
// The returned slice must not be modified by the caller.
func (r *Register) List() []ConcernFlag {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConcernFlag, len(r.flags))
	copy(out, r.flags)
	return out
}

// ByTarget returns all ConcernFlags whose Target matches the given target string.
func (r *Register) ByTarget(target string) []ConcernFlag {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ConcernFlag
	for _, f := range r.flags {
		if f.Target == target {
			out = append(out, f)
		}
	}
	return out
}

// validateSeverity ensures the provided Severity is one of the defined enum values.
func validateSeverity(s Severity) error {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("concernflags: invalid Severity %q; must be low|medium|high|critical", s)
	}
}
