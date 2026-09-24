package integrity

// check.go — CV-9 (ADR-044 §1.3): Check, the base..head integrity report
// over every guarded test.

import (
	"fmt"
	"sort"
	"strings"
)

// Input bundles base..head file access — the same BaseFile/HeadFile/
// ChangedFiles shape conserve.Input already exposes (a deliberate subset
// mirror, so a caller building a conserve.Input for the "integrity" plane
// has everything Check needs without a second git-access abstraction) —
// plus the guardedness context (design §1.2) Check evaluates AT BASE, per
// its own doc comment below.
type Input struct {
	// ChangedFiles is the changed-file set (ADR-044 §0); only the entries
	// scanGoFile can parse as Go contribute guarded tests. A non-.go
	// changed file contributes nothing to Check's own result — see
	// design §1.3's "other languages report unproven": a caller wraps
	// Check as one input to the "integrity" conserve plane, which is
	// responsible for reporting a changed non-Go file as a missing input
	// rather than silently skipping it (internal/conserve/plane_integrity.go).
	ChangedFiles []string
	BaseFile     func(path string) ([]byte, bool, error)
	HeadFile     func(path string) ([]byte, bool, error)

	// Guard supplies GuardedInput's non-Sources fields (manifest/
	// invariant/surface/reach data) — Sources is filled internally by
	// Check from BaseFile over the Go files in ChangedFiles, since
	// guardedness is judged strictly AT BASE (design §1.2).
	Guard GuardedInput
}

// Check reports the integrity of every test guarded AT BASE, across
// base..head, per design §1.3's kind table:
//
//   - The guarded set is computed ONCE, from base-side content only
//     (design §1.2's "at base") — a test only a head-side marker newly
//     guards is out of scope for THIS run; a later run, comparing this
//     head as its own new base, will see it.
//   - For each guarded base test, the SAME repo-relative file is checked
//     at head first. A same-named Test* function still there runs every
//     applicable finer-grained check (weakened/skipped/golden_changed/
//     marker_moved/edited).
//   - No same-named function remains in that file at head: every OTHER
//     changed file's head-side Test* functions are searched for one whose
//     NormalizedBodyHash matches the base test's — found, `renamed`; not
//     found anywhere in the changed-file set, `deleted`.
//
// Scoping note (disclosed): the "found under another name" search is
// bounded to in.ChangedFiles, never a whole-package listing — Check has
// no directory-listing primitive (conserve.Input doesn't carry one
// either). Sufficient for the ordinary case: a rename that moves a test
// necessarily changes BOTH the file it left and the file it landed in, so
// both are already in the changed-file set git handed over; the only case
// this misses is a rename onto a pre-existing, otherwise-untouched file —
// a case design §8 already refuses to chase with anything heavier than
// what the diff itself hands over ("costs scale with the change").
//
// Sorted by (Key, Kind) for determinism; a test that trips more than one
// kind (e.g. both weakened AND skipped) contributes one Finding per kind.
func Check(in Input) []Finding {
	baseSources := goSources(in.ChangedFiles, in.BaseFile)
	headSources := goSources(in.ChangedFiles, in.HeadFile)

	guardIn := in.Guard
	guardIn.Sources = baseSources
	guarded := GuardedTests(guardIn)
	if len(guarded) == 0 {
		return nil
	}

	headTestsByFile := map[string][]scannedTest{}
	var headTestsFlat []scannedTest
	headBuildTag := map[string]bool{}
	for _, src := range headSources {
		tests, hasBuildTag, ok := scanGoFile(src.Path, src.Content)
		if !ok {
			continue
		}
		headTestsByFile[src.Path] = tests
		headTestsFlat = append(headTestsFlat, tests...)
		headBuildTag[src.Path] = hasBuildTag
	}

	baseByFileAndName := map[string]scannedTest{}
	baseBuildTag := map[string]bool{}
	for _, src := range baseSources {
		tests, hasBuildTag, ok := scanGoFile(src.Path, src.Content)
		if !ok {
			continue
		}
		baseBuildTag[src.Path] = hasBuildTag
		for _, t := range tests {
			baseByFileAndName[t.File+"\x00"+t.Name] = t
		}
	}

	var out []Finding
	for _, g := range guarded {
		bt, ok := baseByFileAndName[g.File+"\x00"+g.Name]
		if !ok {
			continue // defensive: GuardedTests scanned these exact sources, so this always hits
		}
		baseHash := normalizedBodyHashOfDecl(sourceOf(baseSources, g.File), bt.Decl)

		var ht *scannedTest
		for i, t := range headTestsByFile[g.File] {
			if t.Name == g.Name {
				ht = &headTestsByFile[g.File][i]
				break
			}
		}
		if ht != nil {
			out = append(out, checkStillPresent(in, g, bt, *ht, baseHash, headTestsFlat, sourceOf(headSources, g.File),
				baseBuildTag[g.File], headBuildTag[g.File])...)
			continue
		}

		if matched, matchOK := findByNormalizedHash(headTestsFlat, headSources, baseHash); matchOK {
			out = append(out, Finding{
				Kind:                KindRenamed,
				Key:                 g.Name,
				Detail:              "gone under its base name; a head test with the same normalized body was found under a different name",
				BaseFile:            g.File,
				BaseLine:            g.Line,
				HeadFile:            matched.File,
				HeadLine:            matched.Line,
				Reasons:             g.Reasons,
				RenamedTo:           matched.Name,
				RenamedToFile:       matched.File,
				RenamedToLine:       matched.Line,
				StillNamedElsewhere: namesExplicitly(g.Reasons),
			})
			continue
		}

		out = append(out, Finding{
			Kind:     KindDeleted,
			Key:      g.Name,
			Detail:   "gone at head; no test in the changed-file set has a matching normalized body",
			BaseFile: g.File,
			BaseLine: g.Line,
			Reasons:  g.Reasons,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// checkStillPresent evaluates every finer-grained kind (design §1.3) for a
// guarded test that still exists, under the same name, in the same file,
// at head. baseHasBuildTag/headHasBuildTag are the enclosing FILE's own
// //go:build (or // +build) state at each point — the file-level half of
// the skipped check (the test-body half is ht.HasSkip below).
func checkStillPresent(in Input, g GuardedTest, bt, ht scannedTest, baseHash string, headTestsFlat []scannedTest, headSrc []byte, baseHasBuildTag, headHasBuildTag bool) []Finding {
	var out []Finding
	found := map[Kind]bool{}
	add := func(f Finding) {
		f.Key = g.Name
		f.BaseFile, f.BaseLine = bt.File, bt.Line
		f.HeadFile, f.HeadLine = ht.File, ht.Line
		f.Reasons = g.Reasons
		out = append(out, f)
		found[f.Kind] = true
	}

	if ht.AssertCount < bt.AssertCount {
		add(Finding{Kind: KindWeakened, Detail: fmt.Sprintf("assertion call count fell from %d to %d", bt.AssertCount, ht.AssertCount)})
	}
	if ht.HasSkip && !bt.HasSkip {
		add(Finding{Kind: KindSkipped, Detail: "a t.Skip*/testing.Short() guard was added"})
	}
	if headHasBuildTag && !baseHasBuildTag {
		add(Finding{Kind: KindSkipped, Detail: "a //go:build (or // +build) constraint line was added to the file"})
	}
	if changed := changedGoldenPaths(in, bt.GoldenPaths); len(changed) > 0 {
		add(Finding{Kind: KindGoldenChanged, Detail: "testdata referenced by this test changed: " + strings.Join(changed, ", ")})
	}
	if bt.CertifiesMarker != "" && ht.CertifiesMarker != bt.CertifiesMarker {
		detail := "the \"// certifies:\" marker was removed"
		if movedTo, movedOK := findMarker(headTestsFlat, bt.CertifiesMarker, ht.Name); movedOK {
			detail = fmt.Sprintf("the \"// certifies:\" marker now sits on %s instead", movedTo.Name)
		}
		add(Finding{Kind: KindMarkerMoved, Detail: detail})
	}

	headHash := normalizedBodyHashOfDecl(headSrc, ht.Decl)
	if headHash != baseHash && !anyFound(found) {
		add(Finding{Kind: KindEdited, Detail: "the normalized body changed"})
	}
	return out
}

func anyFound(found map[Kind]bool) bool { return len(found) > 0 }

// goSources reads every path in files through read, keeping only the ones
// read successfully AND that end in ".go" — the language scanGoFile can
// parse.
func goSources(files []string, read func(string) ([]byte, bool, error)) []Source {
	var out []Source
	for _, path := range files {
		if !hasGoSuffix(path) {
			continue
		}
		content, ok, err := read(path)
		if err != nil || !ok {
			continue // absent on that side (e.g. a newly added file has no base content) — not an error
		}
		out = append(out, Source{Path: path, Content: content})
	}
	return out
}

func hasGoSuffix(path string) bool {
	return len(path) > 3 && path[len(path)-3:] == ".go"
}

func sourceOf(sources []Source, path string) []byte {
	for _, s := range sources {
		if s.Path == path {
			return s.Content
		}
	}
	return nil
}

// findByNormalizedHash searches candidates for the first (by File, Line,
// Name — the same deterministic order GuardedTests sorts by) whose body,
// hashed against its own file's source in sources, equals want.
func findByNormalizedHash(candidates []scannedTest, sources []Source, want string) (scannedTest, bool) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].File != candidates[j].File {
			return candidates[i].File < candidates[j].File
		}
		return candidates[i].Line < candidates[j].Line
	})
	for _, c := range candidates {
		if normalizedBodyHashOfDecl(sourceOf(sources, c.File), c.Decl) == want {
			return c, true
		}
	}
	return scannedTest{}, false
}

// findMarker searches candidates (excluding excludeName) for a test
// carrying marker verbatim — marker_moved's "now sits on another
// function" half.
func findMarker(candidates []scannedTest, marker, excludeName string) (scannedTest, bool) {
	for _, c := range candidates {
		if c.Name == excludeName {
			continue
		}
		if c.CertifiesMarker == marker {
			return c, true
		}
	}
	return scannedTest{}, false
}

// namesExplicitly reports whether reasons includes one that names a test
// BY NAME in a durable place (a marker, a manifest entry, an invariant
// enforcement cell, or compatibility-surface evidence) rather than merely
// proving it reaches a changed symbol — Check's own signal for
// Finding.StillNamedElsewhere (see that field's doc comment for the
// disclosed `-run`-pattern gap).
func namesExplicitly(reasons []GuardReason) bool {
	for _, r := range reasons {
		switch r {
		case ReasonMarker, ReasonManifest, ReasonInvariant, ReasonSurface:
			return true
		}
	}
	return false
}

// changedGoldenPaths returns the subset of paths whose BaseFile/HeadFile
// content differs (including existence differing) — design §1.3's
// "a testdata/ path named in the test body changed in the range."
func changedGoldenPaths(in Input, paths []string) []string {
	var out []string
	for _, p := range paths {
		b, bok, _ := in.BaseFile(p)
		h, hok, _ := in.HeadFile(p)
		if bok != hok || string(b) != string(h) {
			out = append(out, p)
		}
	}
	return out
}
