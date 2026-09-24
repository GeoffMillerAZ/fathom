package integrity

// guarded.go — CV-7 (ADR-044 §1.2): GuardedTests, the set of Test*
// functions the refactor-conservation checks protect, with the reason(s)
// each one qualifies.

import "sort"

// Source is one Go source file's content, keyed by its repo-relative path
// — GuardedTests' (and Check's) shared input shape for "the Go text to
// scan."
type Source struct {
	Path    string
	Content []byte
}

// GuardedInput bundles everything GuardedTests needs to establish
// guardedness at one point in time. Design §1.2 says guardedness is judged
// "at base" — Check (CV-9) passes base-side Sources; a caller wanting the
// current set (e.g. to report "what does this repo guard today") passes
// head-side Sources instead. Every field beyond Sources is flattened data
// a caller already resolved elsewhere (an intent.Corpus, a decoded
// test-kit manifest, a call graph) — this package does no I/O and imports
// none of those packages itself (see the package doc comment).
type GuardedInput struct {
	// Sources is the Go source to scan for Test* functions and their
	// "// certifies:" markers.
	Sources []Source
	// ManifestCertifies is every #Certifies entry's test name, decoded
	// from tests/*.cue suite manifests (testevidence.LoadManifests'
	// Suite.Certifies[].Tests, flattened by the caller).
	ManifestCertifies []string
	// InvariantTests is every test name docs/intent/invariants.md's
	// enforcement column names (intent.Corpus.Invariants[].Tests,
	// flattened by the caller).
	InvariantTests []string
	// SurfaceEvidenceTests is every test name docs/intent/scope.md's
	// "Compatibility surface" section names as Evidence — the caller
	// builds this with ExtractScopeEvidenceTests over that section's raw
	// bytes (this package's own helper, since intent.Corpus does not
	// parse that section into a structured list — see its doc comment).
	SurfaceEvidenceTests []string
	// ChangedSymbols/Reaches: optional reachability oracle for
	// ReasonReach. Reaches(test, symbol) reports whether test statically
	// reaches symbol; nil means "no index available" (design §1.2's own
	// parenthetical), so ReasonReach then contributes nothing — never
	// fabricated from an absent graph.
	ChangedSymbols []string
	Reaches        func(test, symbol string) bool
}

// GuardedTests returns every guarded Test* function found across
// in.Sources, each with every reason it qualifies (design §1.2). A test
// matching none of the five reasons is not guarded and is omitted — a
// clean, ordinary test the conservation checks have no opinion about.
//
// Sorted by (File, Line, Name) for determinism; a name that appears more
// than once across in.Sources (a duplicate top-level Test* func name in
// two different files — a build-tag-split file pair, most commonly) is
// reported once per (File, Name) occurrence, never merged: the caller
// asked "what is guarded, and where," and a merge would silently discard
// one of two real locations.
func GuardedTests(in GuardedInput) []GuardedTest {
	manifestSet := toStringSet(in.ManifestCertifies)
	invariantSet := toStringSet(in.InvariantTests)
	surfaceSet := toStringSet(in.SurfaceEvidenceTests)

	var out []GuardedTest
	for _, src := range in.Sources {
		tests, _, ok := scanGoFile(src.Path, src.Content)
		if !ok {
			continue // unparseable source: never a fatal error to a multi-file scan
		}
		for _, t := range tests {
			var reasons []GuardReason
			if t.CertifiesMarker != "" {
				reasons = append(reasons, ReasonMarker)
			}
			if manifestSet[t.Name] {
				reasons = append(reasons, ReasonManifest)
			}
			if invariantSet[t.Name] {
				reasons = append(reasons, ReasonInvariant)
			}
			if surfaceSet[t.Name] {
				reasons = append(reasons, ReasonSurface)
			}
			if in.Reaches != nil {
				for _, sym := range in.ChangedSymbols {
					if in.Reaches(t.Name, sym) {
						reasons = append(reasons, ReasonReach)
						break
					}
				}
			}
			if len(reasons) == 0 {
				continue
			}
			out = append(out, GuardedTest{
				Name:            t.Name,
				File:            src.Path,
				Line:            t.Line,
				Reasons:         sortedUniqueReasons(reasons),
				CertifiesMarker: t.CertifiesMarker,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func toStringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
