package conserve

// plane_integrity.go — the "integrity" conserve plane (ADR-044 §0, §1):
// wraps internal/analysis/integrity's Check (CV-9, over guarded tests),
// GeneratedHandEdits (CV-11), fed by GuardedTests' (CV-7) context — the
// one place in the package that resolves the intent corpus, decoded
// test-kit manifests, the index/call graph, and a source-path listing
// integrity's leaf functions need, then hands them flattened data, the
// same discipline internal/analysis/review's own callers use for
// ObligationRule/CertificationRecord.
//
// Go only (design §1.3: "Test integrity findings (Go; other languages
// report unproven)"): a changed file this plane cannot check because its
// language is not Go is named in Missing, never silently ignored — I44's
// "missing input means unproven" applies to a language gap exactly as it
// does to a stale index. Narrowed to files that plausibly HOLD tests (a
// non-Go, non-test source or doc change carries no guarded-test claim to
// prove or disprove, so it is not named as missing — Report generation
// would otherwise go permanently Unproven on this plane for any repo that
// touches a README alongside Go code, which is not what "Go only" means
// here).

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/integrity"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
)

func init() {
	Register(integrityPlane{})
}

const integrityPlaneName = "integrity"

// nonGoTestLanguages is every connector.LanguageForPath language this
// plane recognizes as "code that can hold a test" but cannot itself
// check — a changed file in one of these, whose basename plausibly names
// a test, is reported missing rather than silently skipped.
var nonGoTestLanguages = map[string]bool{
	"py": true, "js": true, "ts": true, "java": true, "cs": true,
	"c": true, "cpp": true, "rust": true, "scala": true, "php": true,
}

type integrityPlane struct{}

func (integrityPlane) Name() string { return integrityPlaneName }

func (integrityPlane) Check(ctx context.Context, in Input) (findings []Finding, missing []string) {
	var goFiles []string
	missingSet := map[string]bool{}
	for _, f := range in.ChangedFiles {
		lang := connector.LanguageForPath(f)
		switch {
		case lang == "go":
			goFiles = append(goFiles, f)
		case nonGoTestLanguages[lang] && looksLikeTestFile(f):
			missingSet[lang+" test file not supported for integrity checks: "+f] = true
		}
	}

	scopeEvidence := scopeEvidenceAt(in.BaseFile)
	manifestCertifies := manifestCertifiesAt(ctx, in)
	invariantTests := invariantTestsAt(in.BaseFile)
	changedSymbols, reaches := reachabilityFrom(ctx, in)

	guard := integrity.GuardedInput{
		ManifestCertifies:    manifestCertifies,
		InvariantTests:       invariantTests,
		SurfaceEvidenceTests: scopeEvidence,
		ChangedSymbols:       changedSymbols,
		Reaches:              reaches,
	}

	if len(goFiles) > 0 {
		checkFindings := integrity.Check(integrity.Input{
			ChangedFiles: goFiles,
			BaseFile:     in.BaseFile,
			HeadFile:     in.HeadFile,
			Guard:        guard,
		})
		for _, f := range checkFindings {
			findings = append(findings, toConserveFinding(f))
		}
	}

	genFindings, genMissing := integrity.GeneratedHandEdits(integrity.GeneratedInput{
		ChangedFiles: in.ChangedFiles,
		BaseFile:     in.BaseFile,
		HeadFile:     in.HeadFile,
		SiblingFiles: siblingFilesFromStore(ctx, in),
	})
	for _, f := range genFindings {
		findings = append(findings, toConserveFinding(f))
	}
	for _, m := range genMissing {
		missingSet[m] = true
	}

	for m := range missingSet {
		missing = append(missing, m)
	}
	sort.Strings(missing)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Key < findings[j].Key
	})
	return findings, missing
}

// looksLikeTestFile reports whether path's basename plausibly names a test
// file — the same permissive "contains test" signal used to decide
// whether a non-Go language gap is worth naming at all (see the package
// doc comment above).
func looksLikeTestFile(p string) bool {
	return strings.Contains(strings.ToLower(path.Base(p)), "test")
}

// toConserveFinding maps one integrity.Finding onto this package's own
// Finding shape — a straight rename by design (integrity.Finding's field
// names were chosen for exactly this, see that type's doc comment).
// Declared is always DeclaredNone here: the "integrity" plane has no
// declaration vocabulary of its own (unlike the ledger plane's retires:/
// surface_change:) — a lost test is never excused by a plan annotation,
// only by being genuinely absent from the guarded set to begin with.
//
// informational (never part of the wire contract — see Finding.
// informational's doc comment) is set per design §1.3's own loss/no-loss
// rule for each Kind: KindEdited is "informational; never a loss on its
// own"; KindRenamed is a loss only when the old name is StillNamedElsewhere
// (a marker, invariant-enforcement cell, or compatibility-surface entry
// that named it at base) — a clean rename with no such citation excuses
// nothing to excuse. Every other kind (deleted/weakened/skipped/
// golden_changed/marker_moved/hand_edit) is a genuine, undeclared loss by
// design and stays fully verdict-affecting.
func toConserveFinding(f integrity.Finding) Finding {
	detail := f.Detail
	if len(f.Reasons) > 0 {
		reasons := make([]string, len(f.Reasons))
		for i, r := range f.Reasons {
			reasons[i] = string(r)
		}
		detail = detail + " (guarded by: " + strings.Join(reasons, ", ") + ")"
	}
	remedy := ""
	switch f.Kind {
	case integrity.KindDeleted:
		remedy = "restore the test, or add `retires:`/a disposition to the plan row that removed it"
	case integrity.KindWeakened:
		remedy = "restore the removed assertion(s), or explain the reduction in the commit"
	case integrity.KindSkipped:
		remedy = "remove the skip guard, or justify it where the plan can see it"
	case integrity.KindMarkerMoved:
		remedy = "confirm the certifies marker's new location is intentional"
	case integrity.KindHandEdit:
		remedy = "regenerate instead of hand-editing, or update the generator input to match"
	}
	informational := f.Kind == integrity.KindEdited ||
		(f.Kind == integrity.KindRenamed && !f.StillNamedElsewhere)

	return Finding{
		Plane:         integrityPlaneName,
		Kind:          string(f.Kind),
		Key:           f.Key,
		Detail:        detail,
		Base:          citeOf(f.BaseFile, f.BaseLine),
		Head:          citeOf(f.HeadFile, f.HeadLine),
		Declared:      DeclaredNone,
		Remedy:        remedy,
		informational: informational,
	}
}

func citeOf(file string, line int) string {
	if file == "" {
		return ""
	}
	if line <= 0 {
		return file
	}
	return file + ":" + itoa(line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// scopeEvidenceAt reads docs/intent/scope.md through baseFile (base-side
// content, matching design §1.2's "at base") and extracts its
// Compatibility-surface Evidence test names. An absent file (a repo with
// no docs/intent/ at all, or ok=false for any other reason) is silently
// empty — the same honest-absence handling ExtractScopeEvidenceTests
// itself already gives a missing section.
func scopeEvidenceAt(baseFile func(string) ([]byte, bool, error)) []string {
	if baseFile == nil {
		return nil
	}
	b, ok, err := baseFile("docs/intent/scope.md")
	if err != nil || !ok {
		return nil
	}
	return integrity.ExtractScopeEvidenceTests(b)
}

// manifestCertifiesAt decodes tests/*.cue suite manifests AT in.Base — via
// `git ls-tree` for the path listing (gitLsTree, governance.go's own
// exception to "changed files only": a listing costs nothing like a
// content re-ingest) and in.BaseFile for each one's content — and flattens
// every #Certifies entry's test names. Design §1.2 says guardedness is
// judged "at base": reading root's CURRENT working tree instead (as this
// function used to) lets an agent launder a rename or deletion by dropping
// the SAME manifest's certifies entry in the same change — the guarded
// test then has zero reasons on EITHER side and Check reports nothing,
// not merely excuses an already-reported loss (the i28-laundering pattern
// ADR-044 exists to catch). Malformed or absent manifests, or a missing
// Root/BaseFile, degrade to an empty list, never an error —
// LoadManifestsFromContents already treats "no tests/*.cue at this ref"
// as (nil, nil, nil).
func manifestCertifiesAt(ctx context.Context, in Input) []string {
	if in.Root == "" || in.BaseFile == nil {
		return nil
	}
	paths, err := gitLsTree(ctx, in.Root, in.Base)
	if err != nil {
		return nil
	}
	files := map[string][]byte{}
	for _, p := range paths {
		if path.Dir(p) != "tests" || !strings.HasSuffix(p, ".cue") {
			continue
		}
		if b, ok, ferr := in.BaseFile(p); ferr == nil && ok {
			files[p] = b
		}
	}
	manifests, _, err := testevidence.LoadManifestsFromContents(files)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, m := range manifests {
		for _, su := range m.Suites {
			for _, c := range su.Certifies {
				for _, t := range c.Tests {
					seen[t] = true
				}
			}
		}
	}
	return setToSortedSlice(seen)
}

// invariantTestsAt reads docs/intent/invariants.md AT BASE (via baseFile,
// the same git-show discipline scopeEvidenceAt already applies to
// scope.md) and returns every enforcement-column test name
// (intent.ExtractInvariantEnforcementTests). Design §1.2 says guardedness
// is judged "at base": reading intent.Load(root) against root's CURRENT
// working tree instead (as this function used to) lets an agent launder a
// rename or deletion by dropping the SAME row's test citation in the same
// change, guarding the test on neither side — the invariants.md half of
// the i28-laundering pattern above. An absent file (no docs/intent/ at
// all, or ok=false for any other reason) is silently empty, the same
// honest-absence handling ExtractInvariantEnforcementTests itself already
// gives a missing table.
func invariantTestsAt(baseFile func(string) ([]byte, bool, error)) []string {
	if baseFile == nil {
		return nil
	}
	b, ok, err := baseFile("docs/intent/invariants.md")
	if err != nil || !ok {
		return nil
	}
	return intent.ExtractInvariantEnforcementTests(b)
}

// reachabilityFrom builds the "reaches a changed symbol" oracle (design
// §1.2's ReasonReach) from in.Store, when one is available — the whole
// service's symbol/typed-edge graph, the same composition
// internal/certify/compile.go's own reachedSymbols uses. A nil Store (or
// any read failure) degrades to (nil, nil): GuardedTests then simply never
// assigns ReasonReach, exactly as "no index available" already means.
// This never contributes to Missing — reach is one of five reasons a test
// can be guarded, not a precondition for checking the ones that already
// fired.
func reachabilityFrom(ctx context.Context, in Input) (changedSymbols []string, reaches func(test, symbol string) bool) {
	if in.Store == nil {
		return nil, nil
	}
	obs, err := in.Store.QueryObservations(ctx, in.Service, "")
	if err != nil || len(obs) == 0 {
		return nil, nil
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	edges, _ := in.Store.TypedEdges(ctx, in.Service)

	changedSet := make(map[string]bool, len(in.ChangedFiles))
	for _, f := range in.ChangedFiles {
		changedSet[f] = true
	}
	var changed []string
	for _, n := range nodes {
		if changedSet[n.FilePath] {
			changed = append(changed, n.Key())
		}
	}
	if len(changed) == 0 {
		return nil, nil
	}

	byBareName := map[string]string{} // "" once ambiguous, same discipline certify/compile.go's testSymbolsByBareName uses
	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Kind != core.KindGoFunc || !strings.HasPrefix(n.Name, "Test") {
			continue
		}
		if seen[n.Name] {
			byBareName[n.Name] = ""
			continue
		}
		seen[n.Name] = true
		byBareName[n.Name] = n.Key()
	}

	g := callgraph.BuildBoth(nodes, edges)
	reachFn := func(test, symbol string) bool {
		key := byBareName[test]
		if key == "" {
			return false
		}
		return callgraph.Reachable(g, key, symbol)
	}
	return changed, reachFn
}

// siblingFilesFromStore lists every indexed source path for the service
// (Store.ListSourceRefs — the closest thing to a directory listing
// conserve.Input's Store already carries) and groups them by directory,
// for GeneratedHandEdits' SiblingFiles. A nil Store, or a listing error,
// returns nil — GeneratedHandEdits then names every affected directory in
// its own returned Missing rather than either fabricating a finding or
// silently passing.
func siblingFilesFromStore(ctx context.Context, in Input) map[string][]string {
	if in.Store == nil {
		return nil
	}
	refs, err := in.Store.ListSourceRefs(ctx, in.Service)
	if err != nil || len(refs) == 0 {
		return nil
	}
	out := map[string][]string{}
	for _, p := range refs {
		dir := path.Dir(p)
		out[dir] = append(out[dir], p)
	}
	return out
}

func setToSortedSlice(seen map[string]bool) []string {
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
