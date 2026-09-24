// Package integrity computes ADR-044 stage 1's "proof integrity" facts
// (docs/design/refactor-conservation.md §1): which Go tests are GUARDED
// (§1.2, CV-7), a rename-insensitive hash of a test body (§1.3, CV-8), what
// happened to every guarded test across a base..head change range (§1.3,
// CV-9), and whether a generated file was hand-edited instead of
// regenerated (§1.5, CV-11).
//
// Every exported function here is a pure function of its Input: no global
// state, no clock, no I/O of its own (internal/CLAUDE.md's "inject clocks…
// never call time.Now() in query/analysis logic" and the repo's leaf-
// analyzer convention — internal/analysis/review's own package doc comment
// is the model this package follows). A caller that has git access, an
// intent corpus, decoded test-kit manifests, or a call graph resolves those
// facts itself and hands this package flattened data — the same
// discipline internal/analysis/review.Input uses for ObligationRule/
// CertificationRecord, so this package never grows an internal/intent,
// internal/testevidence, or internal/analysis/callgraph import of its own
// (docs/design/refactor-conservation.md §0's "Run itself makes no git
// calls" carried down one more layer).
//
// Determinism (internal/CLAUDE.md): every returned slice is sorted before
// it leaves this package; two calls with byte-identical inputs always
// produce byte-identical output.
package integrity

import "sort"

// GuardReason names one of ADR-044 §1.2's five reasons a test is guarded.
type GuardReason string

const (
	// ReasonMarker: the test carries a "// certifies:" doc-comment marker
	// immediately above its func declaration.
	ReasonMarker GuardReason = "certifies_marker"
	// ReasonManifest: the test is named by a #Certifies entry in a
	// tests/*.cue suite manifest (test-kit's suite-level declaration).
	ReasonManifest GuardReason = "manifest_entry"
	// ReasonInvariant: the test is named in docs/intent/invariants.md's
	// enforcement column.
	ReasonInvariant GuardReason = "invariant_enforcement"
	// ReasonSurface: the test is named as Evidence on the compatibility
	// surface in docs/intent/scope.md.
	ReasonSurface GuardReason = "compatibility_surface"
	// ReasonReach: the test statically reaches a changed symbol, per a
	// caller-supplied call graph (nil/absent means "no index available" —
	// this reason then contributes nothing, never fabricated).
	ReasonReach GuardReason = "reaches_changed_symbol"
)

// Kind is one integrity finding's classification (design §1.3's table,
// plus KindHandEdit for GeneratedHandEdits, design §1.5).
type Kind string

const (
	KindDeleted       Kind = "deleted"
	KindRenamed       Kind = "renamed"
	KindWeakened      Kind = "weakened"
	KindSkipped       Kind = "skipped"
	KindGoldenChanged Kind = "golden_changed"
	KindMarkerMoved   Kind = "marker_moved"
	KindEdited        Kind = "edited"
	// KindHandEdit is GeneratedHandEdits' own finding kind (design §1.5's
	// "generated/hand_edit").
	KindHandEdit Kind = "hand_edit"
)

// GuardedTest is one guarded Test* function and every reason it qualifies —
// GuardedTests' own result shape (CV-7).
type GuardedTest struct {
	// Name is the bare Test* function's name (top-level Go funcs only, the
	// same "no receiver" convention internal/certify/compile.go's
	// testSymbolsByBareName already uses for this exact class of symbol).
	Name string
	File string
	Line int
	// Reasons is sorted and deduplicated — a test guarded by more than one
	// reason (e.g. both a certifies marker AND an invariant-enforcement
	// citation) carries every one of them, never just the first found.
	Reasons []GuardReason
	// CertifiesMarker is the marker's own text (design §1.3's marker_moved
	// check needs to know not just THAT a marker was present but what it
	// said, to recognize the SAME marker reappearing on a different
	// function) — "" when ReasonMarker is not among Reasons.
	CertifiesMarker string
}

// Finding is one integrity defect over base..head — Check's (CV-9) and
// GeneratedHandEdits' (CV-11) shared result shape. Field names are chosen
// to make internal/conserve/plane_integrity.go's mapping onto
// conserve.Finding a straight rename, never a re-derivation:
// Kind→conserve.Finding.Kind, Key→Key, Detail→Detail,
// "<BaseFile>:<BaseLine>"→Base, "<HeadFile>:<HeadLine>"→Head.
type Finding struct {
	Kind Kind
	// Key is the finding's stable identity within its kind: a guarded
	// test's bare name for every Check kind, or the changed file's
	// repo-relative path for KindHandEdit (GeneratedHandEdits names no
	// test).
	Key    string
	Detail string

	// BaseFile/BaseLine/HeadFile/HeadLine are "file:line" citations, empty
	// when the finding has nothing to point at on that side (KindDeleted
	// has no head location; a newly-added generated file is never checked
	// at all, so KindHandEdit always carries both).
	BaseFile string
	BaseLine int
	HeadFile string
	HeadLine int

	// Reasons is why the test was guarded (every Check kind; empty for
	// KindHandEdit) — carried through so a caller building a
	// conserve.Finding's Detail, or a human reading the raw result, sees
	// WHY this test mattered without a second GuardedTests lookup.
	Reasons []GuardReason

	// RenamedTo/RenamedToFile/RenamedToLine: KindRenamed only — the
	// head-side name/location the base test's normalized body matched.
	RenamedTo     string
	RenamedToFile string
	RenamedToLine int
	// StillNamedElsewhere: KindRenamed only — true when the OLD name was,
	// at base, cited by a reason that names it EXPLICITLY (marker,
	// invariant enforcement, or compatibility-surface evidence — never
	// mere reachability, which names no string), meaning the rename most
	// likely left a stale reference behind somewhere this package did not
	// itself update. Disclosed gap: a `go test -run <pattern>` reference to
	// the old name is NOT checked (design §1.3's own parenthetical) — no
	// reliable, repo-wide source for enumerating `-run` patterns exists
	// yet; StillNamedElsewhere is the best signal this package can prove
	// without one.
	StillNamedElsewhere bool
}

// sortedUniqueReasons returns reasons deduplicated and sorted — every
// Reasons field this package produces goes through this so two equal sets
// built in different orders compare equal and serialize identically.
func sortedUniqueReasons(reasons []GuardReason) []GuardReason {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[GuardReason]bool, len(reasons))
	out := make([]GuardReason, 0, len(reasons))
	for _, r := range reasons {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
