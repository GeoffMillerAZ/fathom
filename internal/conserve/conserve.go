// Package conserve computes ADR-044's conservation report: given a change
// range (base commit → head), it asks a set of Planes what a refactor was
// supposed to conserve and reports what was lost, what excuses the loss,
// and what could not be checked at all.
//
// Three rules are load-bearing (I44, I45, vision 34 §3):
//
//   - A plane with a missing input is "unproven" — never "conserved," no
//     matter what it managed to check. A stale index, a failed typed pass,
//     an unbound test run, an unsupported language: any of these makes a
//     plane's verdict unproven, and the missing inputs are named.
//   - A finding is excused ONLY when it was declared at BASE ("prior"). A
//     declaration added inside the change range ("self") excuses nothing —
//     that is the whole point of Declarations.Declared and I45.
//   - fathom asserts facts; it never scores, ranks, or grades. Verdict is
//     one of three words, never a number.
//
// This package is pure Go and does no I/O of its own beyond what Input's
// closures (BaseFile/HeadFile) and Store already provide — Run itself makes
// no git calls, no store queries, no network calls. NewInput (input.go) is
// the one place that touches git, for callers that want the ordinary
// "compare the working tree against a resolved base commit" shape; a
// caller with its own way to resolve those facts builds an Input directly.
//
// Planes register themselves from their own file's init(), via Register —
// so each plane (integrity, surface, certificates, governance, absence,
// resurrection, generated; ADR-044 §0) is a self-contained addition that
// never requires editing a shared list, and two planes landing in the same
// wave from different workers can never conflict on this file.
package conserve

import (
	"context"
	"sort"
)

// Verdict is a plane's conservation verdict — never a score (vision 34 §3).
type Verdict = string

// The three verdicts a plane may report. Conserved means "checked, no
// undeclared finding." Losses means "checked, at least one finding is not
// excused by a prior declaration." Unproven means "at least one input this
// plane needed was missing" — it beats every other consideration (I44):
// a plane never reports conserved from a missing input, however clean its
// findings look otherwise.
const (
	Conserved Verdict = "conserved"
	Losses    Verdict = "losses"
	Unproven  Verdict = "unproven"
)

// The three ways a Finding's excusing declaration may stand, per I45.
// "Prior" is the only one that excuses a loss: it means the declaration
// (a plan row's retires:/surface_change: entry, or whatever a later plane
// treats as its own declaration convention) was already present at Base.
// "Self" means it was added somewhere inside base..head — the same change
// that produced the finding excusing itself, which I45 says counts for
// nothing. "None" means no declaration was found at either point.
const (
	DeclaredPrior Verdict = "prior"
	DeclaredSelf  Verdict = "self"
	DeclaredNone  Verdict = "none"
)

// SchemaV1 is the wire-contract schema tag every Report carries (ADR-044
// §0's "The JSON report is fathom.conserve/v1 (compatibility surface)").
const SchemaV1 = "fathom.conserve/v1"

// Finding is one conservation defect a Plane reports: something the change
// range appears to have lost, weakened, orphaned, or otherwise broken the
// conservation law for. Plane is carried on the Finding itself (not only
// implied by which PlaneReport it sits under) so a Finding remains
// self-describing wherever it travels alone — a cross-repo consumer
// listing, a findings-bridge claim, a SARIF result.
//
// Base and Head are "file:line" citations (empty when a finding has no
// single line to point at — an absence proof, for instance, points at
// nothing because there is nothing left to point at). Declared is always
// one of DeclaredPrior/DeclaredSelf/DeclaredNone. Remedy is a short,
// human-actionable suggestion ("add `retires: api:foo.Bar` to the row that
// claims this," "re-add the export or declare the change"); empty when a
// plane has nothing more specific to say than the finding itself.
type Finding struct {
	Plane    string `json:"plane"`
	Kind     string `json:"kind"`
	Key      string `json:"key"`
	Detail   string `json:"detail"`
	Base     string `json:"base,omitempty"`
	Head     string `json:"head,omitempty"`
	Declared string `json:"declared"`
	Remedy   string `json:"remedy,omitempty"`

	// informational marks a finding a plane reports for visibility but
	// that must never, by itself, tip planeVerdict to Losses — design doc
	// §1.3's per-kind rule that `edited` is "informational; never a loss
	// on its own" and `renamed` is a loss only when the old name is still
	// named elsewhere. Unexported: a purely internal verdict-computation
	// signal, deliberately never part of the fathom.conserve/v1 wire
	// contract (encoding/json never marshals an unexported field
	// regardless of tag) — the finding itself still appears in
	// PlaneReport.Findings exactly as before, only its effect on the
	// rolled-up Verdict changes. The alternative certificates.go uses
	// (never emit a "live" finding at all) does not fit here: an edited or
	// cleanly-renamed guarded test is still something a caller inspecting
	// findings directly should see.
	informational bool
}

// PlaneReport is one Plane's contribution to a Report: its computed
// Verdict, the Findings it based that verdict on (sorted by kind, then
// key), and Missing — the inputs it needed and did not have, which is what
// forces Verdict to Unproven regardless of Findings.
type PlaneReport struct {
	Plane    string    `json:"plane"`
	Verdict  Verdict   `json:"verdict"`
	Missing  []string  `json:"missing"`
	Findings []Finding `json:"findings"`
}

// TierInfo names the edge tier (ADR-044 §5) a report's facts for one
// language stood on — "typed" (a real type-checked pass), "file-resolved"
// (SU-11's tier), or a plane-specific "unproven"-adjacent label a later
// row defines. Populated by a caller (CV-38/CV-39, a later row) that joins
// per-language tier data onto a Report Run already produced; Run itself
// never touches this field.
type TierInfo struct {
	Language string `json:"language"`
	Tier     string `json:"tier"`
}

// Report is the full conservation result for one Run — the fathom.conserve
// /v1 wire document (ADR-044 §0). Field names and every Verdict/Declared
// string above are the compatibility surface: never rename one without
// bumping Schema.
type Report struct {
	Schema  string        `json:"schema"`
	Service string        `json:"service"`
	Base    string        `json:"base"`
	Head    string        `json:"head"`
	Planes  []PlaneReport `json:"planes"`
	// Tiers is left nil by Run — see TierInfo's doc comment. A caller that
	// wants it set assigns Report.Tiers after Run returns.
	Tiers []TierInfo `json:"tiers,omitempty"`
}

// Plane is one family of conservation checks (ADR-044 §0: integrity,
// surface, certificates, governance, absence, resurrection, generated).
// Check runs the plane's own analysis over in and returns its findings
// (unsorted; Run sorts) plus the named reasons any required input was
// missing (also unsorted). A Plane returning a non-empty missing MAY still
// return findings computed from whatever it did have — Run's verdict rule
// disregards them (Unproven beats everything), but a caller inspecting a
// PlaneReport directly still sees what partial evidence was gathered.
//
// Check must be a pure function of in (and whatever ctx cancels): no
// global state, no clock, no writes. Determinism here is what lets Run's
// own output be schema-stable (docs/design/refactor-conservation.md,
// internal/CLAUDE.md).
type Plane interface {
	Name() string
	Check(ctx context.Context, in Input) (findings []Finding, missing []string)
}

// registry holds every Plane that has called Register, in registration
// order (never load-bearing — Planes() always returns them sorted by
// name). Registration happens from each plane's own init(), so two planes
// landing from different workers in the same wave touch different files
// and never conflict on this one.
var registry []Plane

// Register adds p to the package-level plane registry. Meant to be called
// from a plane's own file's init() — see the package doc comment. Never
// deduplicates: registering the same Name() twice is a caller bug Planes()
// makes visible (two entries with the same name) rather than silently
// resolving.
func Register(p Plane) {
	registry = append(registry, p)
}

// Planes returns every registered Plane, sorted by Name() — the set
// `fathom conserve` (CV-19, a later row) runs by default. A fresh slice
// each call; the registry itself is never exposed for mutation.
func Planes() []Plane {
	out := make([]Plane, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Run checks in against every plane in planes and returns the composed
// Report: each PlaneReport's Verdict follows the rule in the package doc
// comment (missing beats everything; an undeclared-or-self-declared
// finding is losses; otherwise conserved), and both PlaneReports and each
// one's Findings come out sorted — plane name, then (within a plane) kind,
// then key, exactly the order the wire contract promises
// (docs/design/refactor-conservation.md §0: "sort order is plane, then
// kind, then key").
//
// Pure and deterministic given planes' own Check implementations are: Run
// itself does no I/O, and never mutates in or the planes slice.
func Run(ctx context.Context, in Input, planes []Plane) Report {
	rep := Report{
		Schema:  SchemaV1,
		Service: in.Service,
		Base:    in.Base,
		Head:    in.Head,
		Planes:  make([]PlaneReport, 0, len(planes)),
	}
	for _, p := range planes {
		findings, missing := p.Check(ctx, in)

		// fs/ms start as non-nil, zero-length slices (never append's
		// nil-preserving shorthand) so a plane with nothing to report
		// marshals "missing":[]/"findings":[] — the documented
		// fathom.conserve/v1 wire example (design doc §0) — rather than
		// null, which a JSON Schema validator or a non-Go client would
		// refuse where an array is declared.
		fs := make([]Finding, 0, len(findings))
		fs = append(fs, findings...)
		sort.Slice(fs, func(i, j int) bool {
			if fs[i].Kind != fs[j].Kind {
				return fs[i].Kind < fs[j].Kind
			}
			return fs[i].Key < fs[j].Key
		})

		ms := make([]string, 0, len(missing))
		ms = append(ms, missing...)
		sort.Strings(ms)

		rep.Planes = append(rep.Planes, PlaneReport{
			Plane:    p.Name(),
			Verdict:  planeVerdict(ms, fs),
			Missing:  ms,
			Findings: fs,
		})
	}
	sort.Slice(rep.Planes, func(i, j int) bool { return rep.Planes[i].Plane < rep.Planes[j].Plane })
	return rep
}

// planeVerdict applies the rule the package doc comment states: any
// missing input makes the plane Unproven outright (I44); otherwise any
// finding not declared prior makes it Losses; otherwise Conserved. A
// finding marked informational (Finding.informational) never contributes
// to this decision either way — it is reported, never a loss on its own.
func planeVerdict(missing []string, findings []Finding) Verdict {
	if len(missing) > 0 {
		return Unproven
	}
	for _, f := range findings {
		if f.informational {
			continue
		}
		if f.Declared != DeclaredPrior {
			return Losses
		}
	}
	return Conserved
}
