package intent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// U12 — surface_rows_uncovered (EP-38, ADR-026).
//
// The router names sensitive surfaces and, for each, the evidence a change
// there needs ("Done needs"). Plan rows carry their own proof.
//
// NARROWED FROM THE SPEC, and the narrowing is the finding. EP-38 as written
// asked for rows whose `reachable_from` is not one of the surface's Done-needs
// tests. Run against this repo that fired 50+ times, because a surface's
// Done needs is what a change there needs IN ADDITION to the row's own proof,
// not INSTEAD of it: `TestWalkCppWiredDirect` proving a C++ extractor row is
// correct, not a gap. A finding that fires on every healthy row is not a
// finding.
//
// What is left is real and much smaller: a row that CLAIMS DONE on a
// sensitive surface while pinning no named test. `fathom verify` still
// proves it — some `Test*` reaches the symbol — but nothing says WHICH, so
// the surface's guard test can be deleted or renamed and the row stays
// green. On `internal/store/**` that is the difference between "a test
// touches this" and "store purity is still enforced".
//
// This LISTS. It never gates: a `skip_test` row with a written reason is an
// authored decision, and a row may be proven a way the router never named.

// PlanRow is one gated plan row's contract, read statically from the plan
// markdown. No index, no daemon — U12 must be computable on a laptop with a
// cold store.
type PlanRow struct {
	Plan          string // repo-relative plan path
	ID            string
	Package       string // the row's `package:` annotation; "" makes the row unmatchable
	Symbol        string
	ReachableFrom []string
	Claimed       bool
	SkipTest      bool
	SkipReason    string
}

// SurfaceRowsUncovered returns one claim per claimed plan row that sits
// inside a sensitive surface and pins no named test. The second return names
// every surface that could not be evaluated, with the reason — a surface
// silently skipped is worse than one reported unchecked.
//
// Deterministic: claims sorted by ref, notes sorted and deduplicated.
func SurfaceRowsUncovered(c Corpus, rows []PlanRow) ([]Claim, []string) {
	var claims []Claim
	var untested, fileOnly, partial []string

	for _, s := range c.Surfaces {
		dirs, fileScoped := surfaceScopes(s.Globs)
		label := strings.Join(s.Globs, ", ")

		if len(s.Tests) == 0 {
			untested = append(untested, label)
		}
		if len(dirs) == 0 {
			fileOnly = append(fileOnly, label)
			continue
		}
		if len(fileScoped) > 0 {
			partial = append(partial, label)
		}

		for _, r := range rows {
			if !r.Claimed || r.Package == "" || !insideAny(r.Package, dirs) {
				continue
			}
			if len(r.ReachableFrom) > 0 {
				continue // the guard is named; drift renames it and verify fails loudly
			}
			if r.SkipTest && r.SkipReason != "" {
				continue // an authored, lint-checked decision, not an omission
			}
			why := "pins no named test (`reachable_from` absent)"
			if r.SkipTest {
				why = "skips its test with no reason given"
			}
			claims = append(claims, Claim{
				Kind:   "U12",
				Ref:    r.Plan + " " + r.ID,
				Detail: fmt.Sprintf("claims done on sensitive surface %s but %s — the surface's guard can be renamed or deleted and this row stays green", label, why),
				Remedy: surfaceRemedy(s),
			})
		}
	}

	// RC-9: ONE line per reason, never one per surface. Eleven near-identical
	// notes drowned the section a reader most needs — the section that says
	// what the audit could NOT compute.
	var notes []string
	if n := len(fileOnly); n > 0 {
		notes = append(notes, fmt.Sprintf(
			"could not match %d surface(s) whose globs are all file-scoped against package-scoped plan rows (%s) — index-backed file resolution would close this",
			n, briefList(fileOnly, 3),
		))
	}
	if n := len(partial); n > 0 {
		notes = append(notes, fmt.Sprintf(
			"matched only the directory globs of %d surface(s) with mixed globs (%s)", n, briefList(partial, 3),
		))
	}
	if n := len(untested); n > 0 {
		notes = append(notes, fmt.Sprintf(
			"%d surface(s) name no Test* in `Done needs`, so U12 can say a row is unpinned but not what to pin it to (%s)",
			n, briefList(untested, 3),
		))
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].Ref < claims[j].Ref })
	sort.Strings(notes)
	return claims, notes
}

// surfaceRemedy names the tests the router says this surface needs, or says
// plainly that it names none.
func surfaceRemedy(s Surface) string {
	if len(s.Tests) == 0 {
		return "add `reachable_from: <the test that guards this row>`; the router's `Done needs` for this surface names no Test* to point at, which is itself worth fixing"
	}
	return fmt.Sprintf("add `reachable_from: <the test that guards this row>` — this surface's `Done needs` names %s", strings.Join(s.Tests, ", "))
}

// surfaceScopes splits a surface's globs into directory scopes (`dir/**`, or
// a bare directory path) and file-scoped globs, which a package-level plan
// row cannot be matched against without the index.
func surfaceScopes(globs []string) (dirs, fileScoped []string) {
	for _, g := range globs {
		switch {
		case strings.HasSuffix(g, "/**"):
			dirs = append(dirs, strings.TrimSuffix(g, "/**"))
		case !strings.ContainsAny(g, "*?[") && !strings.Contains(g, "."):
			dirs = append(dirs, strings.TrimSuffix(g, "/"))
		default:
			fileScoped = append(fileScoped, g)
		}
	}
	return dirs, fileScoped
}

// insideAny reports whether the repo-relative package path lies at or under
// any of the directories.
func insideAny(pkg string, dirs []string) bool {
	pkg = strings.TrimSuffix(strings.TrimPrefix(pkg, "./"), "/")
	for _, d := range dirs {
		if pkg == d || strings.HasPrefix(pkg, d+"/") {
			return true
		}
	}
	return false
}

func anyIn(names []string, want map[string]bool) bool {
	for _, n := range names {
		if want[n] {
			return true
		}
	}
	return false
}

// SurfaceStamp is the provenance a proposed guard entry carries so staleness
// is computable later: the first 12 hex of a SHA-256 over the surface row's
// normalized content (globs, why, done-needs).
//
// DEVIATION FROM EP-21, deliberate: the spec said "@ <router blob sha>". A
// whole-file sha marks EVERY entry stale on any router edit — a typo fix in
// an unrelated row would fire the whole set. That is the false-alarm channel
// the attention line's staleness rule already refuses; readers learn to
// ignore a signal that cries wolf. Hashing the ROW marks stale exactly the
// entries whose source actually changed.
func SurfaceStamp(s Surface) string {
	norm := strings.Join(s.Globs, "|") + "\x00" +
		strings.Join(strings.Fields(s.Why), " ") + "\x00" +
		strings.Join(strings.Fields(s.DoneNeeds), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])[:12]
}

// stampRe pulls the provenance stamp back out of a pasted guard note.
var stampRe = regexp.MustCompile(`@ ([0-9a-f]{12})\b`)

// GuardEntryStale returns U13 claims: guard entries carrying an intent
// provenance stamp whose source surface row has changed or vanished, and the
// router surfaces no guard entry covers at all.
//
// Entries with no stamp are NOT reported. A human-authored guard that happens
// to overlap a sensitive surface is theirs, not fathom's to police (law 15).
func GuardEntryStale(c Corpus, notes []string, scopesOf func(note string) []string) []Claim {
	live := map[string]Surface{}
	for _, s := range c.Surfaces {
		live[SurfaceStamp(s)] = s
	}

	var claims []Claim
	covered := map[string]bool{}
	for _, note := range notes {
		m := stampRe.FindStringSubmatch(note)
		if m == nil {
			continue
		}
		stamp := m[1]
		scopes := strings.Join(scopesOf(note), ", ")
		if s, ok := live[stamp]; ok {
			covered[SurfaceStamp(s)] = true
			continue
		}
		claims = append(claims, Claim{
			Kind:   "U13",
			Ref:    ".fathom/guard.cue [" + scopes + "]",
			Detail: "carries intent provenance @ " + stamp + ", but no sensitive-surface row in the router matches it any more — the row changed or was removed",
			Remedy: "re-run `fathom intent export --guard` and replace this entry, or drop the `intent: … @ …` stamp if the note is now yours to maintain",
		})
	}

	for _, s := range c.Surfaces {
		if covered[SurfaceStamp(s)] {
			continue
		}
		claims = append(claims, Claim{
			Kind:   "U13",
			Ref:    "docs/intent/README.md [" + strings.Join(s.Globs, ", ") + "]",
			Detail: "the router calls this surface sensitive, but no guard entry carries its provenance — an edit here gets no pre-edit guidance from the intent corpus",
			Remedy: "`fathom intent export --guard` proposes the entry; paste it into .fathom/guard.cue if you want it (fathom never writes that file)",
		})
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].Ref < claims[j].Ref })
	return claims
}

// briefList renders at most max names plus an honest "and N more" — a
// truncation that names its own size, never a silent one.
func briefList(names []string, max int) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	if len(sorted) <= max {
		return strings.Join(sorted, "; ")
	}
	return fmt.Sprintf("%s; and %d more", strings.Join(sorted[:max], "; "), len(sorted)-max)
}
