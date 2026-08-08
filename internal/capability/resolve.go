package capability

import (
	"fmt"
	"sort"
	"strings"
)

// Scope is where a setting came from. Resolution is built-in → user →
// project, most specific winning, and the winning scope travels with the
// value so every surface can render provenance.
type Scope string

const (
	ScopeBuiltIn Scope = "built-in"
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

func (s Scope) describe() string {
	switch s {
	case ScopeUser:
		return "your user default (~/.fathom/capabilities.cue)"
	case ScopeProject:
		return "this repository (.fathom/capabilities.cue)"
	default:
		return "fathom's built-in default"
	}
}

// Setting is a resolved value plus the scope that supplied it.
type Setting struct {
	On    bool
	Scope Scope
}

// Settings is one scope's parsed file: capability id → on/off. A key
// absent from the map means that scope said nothing, which is different
// from saying "off".
type Settings map[string]bool

// Resolution holds the parsed scopes and resolves a capability on demand.
type Resolution struct {
	User    Settings
	Project Settings
}

// For resolves one capability: project wins over user, user over the
// built-in default. A user-scope setting for a project-only capability is
// ignored here because [Registry.ValidateSettings] already rejected it at load — this
// function is the fallback for callers that skipped validation, and it
// fails safe by deferring to the built-in rather than honouring a setting
// the scope was not entitled to make.
func (r Resolution) For(c Capability) Setting {
	if v, ok := r.Project[c.ID]; ok {
		return Setting{On: v, Scope: ScopeProject}
	}
	if v, ok := r.User[c.ID]; ok && c.Scopes != ScopeProjectOnly {
		return Setting{On: v, Scope: ScopeUser}
	}
	return Setting{On: c.Default, Scope: ScopeBuiltIn}
}

// ScopeViolationError names a setting a scope was not entitled to make.
// It names the key rather than ignoring it: silently dropping
// configuration is the defect class that produced O22, and a user who
// wrote a line that does nothing deserves to be told which line.
type ScopeViolationError struct {
	ID    string
	Scope Scope
}

func (e *ScopeViolationError) Error() string {
	return fmt.Sprintf("capability %q may only be set by a repository's own .fathom/capabilities.cue, "+
		"not at %s scope — it governs behaviour rather than expressing a preference (law 13)", e.ID, e.Scope)
}

// UnknownCapabilityError names an unrecognised id and, when one is close
// enough to be a plausible typo, suggests it. A typo that silently
// disables a safety feature is the worst failure this package can have,
// and it has already happened once in this program (the phase-N tier
// typo).
type UnknownCapabilityError struct {
	ID      string
	Suggest string
	Known   []string
}

func (e *UnknownCapabilityError) Error() string {
	if e.Suggest != "" {
		return fmt.Sprintf("unknown capability %q — did you mean %q?", e.ID, e.Suggest)
	}
	return fmt.Sprintf("unknown capability %q (known: %s)", e.ID, strings.Join(e.Known, ", "))
}

// ValidateSettings checks one scope's settings against the registry, returning
// every violation rather than the first — a config file with three
// mistakes should report three, not send its author round the loop twice.
func (r Registry) ValidateSettings(s Settings, scope Scope) []error {
	ids := make([]string, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var errs []error
	for _, id := range ids {
		c, ok := r.Get(id)
		if !ok {
			errs = append(errs, &UnknownCapabilityError{
				ID:      id,
				Suggest: nearest(id, r.IDs()),
				Known:   r.IDs(),
			})
			continue
		}
		if scope == ScopeUser && c.Scopes == ScopeProjectOnly {
			errs = append(errs, &ScopeViolationError{ID: id, Scope: scope})
		}
	}
	return errs
}

// nearest returns the known id within edit distance 2 of want, or "" when
// nothing is close. The threshold is deliberately tight: a wrong
// suggestion is more confusing than none.
func nearest(want string, known []string) string {
	best, bestD := "", 3
	for _, k := range known {
		if d := editDistance(want, k); d < bestD {
			best, bestD = k, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}
