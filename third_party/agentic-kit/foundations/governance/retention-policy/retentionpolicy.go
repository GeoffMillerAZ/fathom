package retentionpolicy

import (
	"fmt"
	"strings"
	"time"
)

// Due evaluates policies against inventory and returns the set of DueAction
// values that are due at now. An entry is due if:
//  1. At least one policy matches the entry (NamespacePrefix and Class filters).
//  2. The entry's age (now.Sub(entry.CreatedAt)) exceeds the matching policy's
//     MaxAge.
//
// Evaluation order: policies are evaluated in the order provided. For each
// inventory entry, the FIRST matching overdue policy wins (one DueAction per
// entry maximum — the caller may execute further policies after the first action
// if they choose, but Due does not fan out multiple actions per entry). This
// prevents double-actioning a namespace when multiple policies overlap.
//
// Entries with a zero CreatedAt are skipped (indeterminate age — no action can
// be safely prescribed).
//
// now must not be zero. If now is zero, Due panics (this is a wiring bug, not a
// runtime condition). Policies and inventory may each be nil or empty; the
// result is an empty slice.
func Due(policies []Policy, inventory []NamespaceEntry, now time.Time) []DueAction {
	if now.IsZero() {
		panic("retentionpolicy: Due: now must not be zero")
	}
	if len(policies) == 0 || len(inventory) == 0 {
		return nil
	}

	var out []DueAction
	for _, entry := range inventory {
		if entry.Namespace == "" || entry.CreatedAt.IsZero() {
			continue
		}
		age := now.Sub(entry.CreatedAt)
		for _, p := range policies {
			if !matchesPolicy(p, entry) {
				continue
			}
			if age > p.MaxAge {
				out = append(out, DueAction{
					Entry:  entry,
					Policy: p,
					Age:    age,
					Kind:   p.Action,
				})
				break // first matching overdue policy wins for this entry
			}
		}
	}
	return out
}

// Validate checks that every policy in the slice is valid, returning a combined
// error if any policy is invalid. The error message includes the policy index
// and name. A nil or empty slice is valid.
func Validate(policies []Policy) error {
	var errs []string
	for i, p := range policies {
		if err := p.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("[%d] %v", i, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("retentionpolicy: invalid policies: %s", strings.Join(errs, "; "))
	}
	return nil
}

// matchesPolicy reports whether policy p applies to entry e.
// Both filters (NamespacePrefix and Class) must be satisfied.
func matchesPolicy(p Policy, e NamespaceEntry) bool {
	if p.NamespacePrefix != "" && !strings.HasPrefix(e.Namespace, p.NamespacePrefix) {
		return false
	}
	if p.Class != "" && p.Class != e.Class {
		return false
	}
	return true
}

// errorf is a package-level error constructor that prefixes with the package name.
func errorf(format string, args ...any) error {
	return fmt.Errorf("retentionpolicy: "+format, args...)
}
