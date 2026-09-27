package scope

// apply.go — the filter itself (SP-2, SP-3). Set compiles suppression
// matchers once per file load; ApplySuppressions is the single pure
// function every filtered read funnels through, and it always reports what
// it hid. Expiry is judged per CALL against the caller's clock — the Set
// deliberately does not bake "active" in at compile time, so a suppression
// that lapses between two queries stops filtering on the second with no
// reload and no sweep (SP-1).

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

// Set is the compiled suppression set for one loaded file generation. It
// keeps every non-superseded record with its expiry/removal fields intact;
// Match judges each record's State at the caller's now, which is what
// makes "evaluated at read time, no sweep" concrete.
type Set struct {
	entries []setEntry
}

type setEntry struct {
	sup     Suppression
	matcher *filter.Matcher
}

// NewSet compiles matchers for all. It refuses, naming the offender, any
// pattern a Suppression should never have carried — the same loud stance
// connector.LoadIgnoreScoped takes for a user-scope re-include.
func NewSet(all []Suppression) (*Set, error) {
	set := &Set{}
	for _, s := range all {
		if err := s.validate(); err != nil {
			return nil, fmt.Errorf("scope: suppression %q: %w", s.ID, err)
		}
		set.entries = append(set.entries, setEntry{sup: s, matcher: filter.New([]string{s.Pattern})})
	}
	sort.Slice(set.entries, func(i, j int) bool { return set.entries[i].sup.ID < set.entries[j].sup.ID })
	return set, nil
}

// Empty reports whether the set holds no records at all. A nil *Set is
// empty, so the no-suppressions case needs no special-casing at any call
// site. Note an all-expired set is NOT Empty — expiry is Match's per-call
// judgement, not a compile-time one.
func (s *Set) Empty() bool { return s == nil || len(s.entries) == 0 }

// Suppressions returns the set's records in ID order, regardless of state.
func (s *Set) Suppressions() []Suppression {
	if s.Empty() {
		return nil
	}
	out := make([]Suppression, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e.sup)
	}
	return out
}

// lineSuffix strips a trailing ":123" locator from a source_ref. Observation
// refs are documented as "file[:line]", and a suppression pattern is written
// against files, so the line must not defeat the match.
var lineSuffix = regexp.MustCompile(`:\d+$`)

// NormalizeRef reduces a stored locator to the plain repo-relative file path
// a gitignore-syntax pattern is written against. Purge resolution uses the
// SAME function, so "what preview counts", "what the filter hides" and
// "what purge deletes" can never disagree about what a ref means.
func NormalizeRef(ref string) string {
	ref = strings.TrimPrefix(ref, "./")
	return lineSuffix.ReplaceAllString(ref, "")
}

// Match returns the first (ID-ordered, therefore deterministic) suppression
// that is ACTIVE at now and hides ref for service, and whether one did. A
// suppression with an empty Service is fleet-wide.
func (s *Set) Match(service, ref string, now time.Time) (Suppression, bool) {
	if s.Empty() || ref == "" {
		return Suppression{}, false
	}
	path := NormalizeRef(ref)
	for _, e := range s.entries {
		if SuppressionState(e.sup, now) != StateActive {
			continue
		}
		if e.sup.Service != "" && e.sup.Service != service {
			continue
		}
		if e.matcher.Match(path) {
			return e.sup, true
		}
	}
	return Suppression{}, false
}

// AffectsService reports whether any suppression ACTIVE at now applies to
// service (its own or a fleet-wide one) — a cheap gate for reads that
// serve a precomputed aggregate and must recompute it only when something
// could be hidden (RT-79's service_summary).
func (s *Set) AffectsService(service string, now time.Time) bool {
	if s.Empty() {
		return false
	}
	for _, e := range s.entries {
		if SuppressionState(e.sup, now) != StateActive {
			continue
		}
		if e.sup.Service == "" || e.sup.Service == service {
			return true
		}
	}
	return false
}

// Disclosure is what a filtered read owes its caller: how many rows were
// hidden and the governing reason(s). §2.2 — "a silent filter is
// indistinguishable from a recall bug, and this program has spent real time
// chasing recall bugs."
type Disclosure struct {
	// Count is the number of items dropped.
	Count int `json:"count"`
	// Reason is the human sentence: every governing reason, joined, each
	// naming its pattern and (when time-boxed) its expiry.
	Reason string `json:"reason"`
	// Reasons is the same content as discrete, sorted entries, for a UI
	// that wants to render them as a list rather than a sentence.
	Reasons []string `json:"reasons,omitempty"`
	// IDs are the suppressions that did the hiding, sorted — so `fathom
	// scope unsuppress <id>` is one copy-paste away from any disclosure.
	IDs []string `json:"ids,omitempty"`
}

// Merge folds other into d, keeping counts additive and reasons unique and
// sorted. Used to accumulate one disclosure per service across several
// reads.
func (d *Disclosure) Merge(other *Disclosure) {
	if other == nil {
		return
	}
	d.Count += other.Count
	d.Reasons = mergeSorted(d.Reasons, other.Reasons)
	d.IDs = mergeSorted(d.IDs, other.IDs)
	d.Reason = strings.Join(d.Reasons, "; ")
}

func mergeSorted(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// describe renders one suppression's reason for a disclosure, naming the
// pattern and disclosing an expiry when the hiding is temporary — so a
// reader can tell "hidden forever" from "hidden until Tuesday" without
// looking anything up.
func describe(s Suppression) string {
	if s.ExpiresAt != nil {
		return fmt.Sprintf("%s (%s, expires %s)", s.Reason, s.Pattern, s.ExpiresAt.UTC().Format("2006-01-02"))
	}
	return fmt.Sprintf("%s (%s)", s.Reason, s.Pattern)
}

// ApplySuppressions is the ONE place content is dropped from a read (SP-2).
// It returns the surviving items in their original order and — whenever it
// dropped anything at all — a Disclosure naming the count and the reasons
// (SP-3). It returns a nil Disclosure when nothing was dropped, so a caller
// can distinguish "nothing was hidden" from "something was hidden and not
// explained"; the latter must be impossible.
//
// refOf extracts the path column the suppression pattern is matched
// against. Each location supplies its OWN column — observations'
// source_ref, knowledge_cards' file, typed_edges' from_file — because a
// single ref list misses every row anchored by another key (the same
// mistake §3.3 caught in purge resolution).
func ApplySuppressions[T any](set *Set, service string, now time.Time, items []T, refOf func(T) string) ([]T, *Disclosure) {
	if set.Empty() || len(items) == 0 {
		return items, nil
	}
	kept := make([]T, 0, len(items))
	var reasons, ids []string
	dropped := 0
	for _, it := range items {
		sup, hidden := set.Match(service, refOf(it), now)
		if !hidden {
			kept = append(kept, it)
			continue
		}
		dropped++
		reasons = append(reasons, describe(sup))
		ids = append(ids, sup.ID)
	}
	if dropped == 0 {
		return items, nil
	}
	d := &Disclosure{Count: dropped, Reasons: mergeSorted(nil, reasons), IDs: mergeSorted(nil, ids)}
	d.Reason = strings.Join(d.Reasons, "; ")
	return kept, d
}
