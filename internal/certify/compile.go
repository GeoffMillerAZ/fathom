package certify

// compile.go — CompileCertifications (TC-6, design doc §3): declarations
// (per-test `// certifies:` markers, TC-5; per-suite `#Certifies` when a
// suite names one, design doc §1) times the joins (exists, reaches, last
// attested run) become one core.SemanticClaim per (target, test), exactly
// like the findings layer's own deterministic bridge — content-addressed,
// no model, PutClaims. Re-running it against unchanged declarations and
// runs reproduces the identical claim_ids (idempotent).
//
// Coverage/mutation joins (design doc §3's "covered… when a profile is
// joined", "mutation… when a report is joined") are NOT wired into this
// signature: CompileCertifications runs at the end of a plain `ingest`/
// `tests ingest`, which carries no coverage profile or mutation report of
// its own. Meta.CoveragePct/MutationScore stay unset — a documented stage
// 1 simplification, not a silent gap (they are reserved fields, see
// meta's own doc comment).
import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
)

// declaredEntry is one (test, target) pairing this compile considers,
// gathered from either a per-test marker (TestSymbolKey already known) or
// a suite-level #Certifies entry naming explicit tests (resolved to a
// symbol key by bare-name lookup, same ambiguity discipline
// testevidence.JoinCasesToSymbols uses).
type declaredEntry struct {
	TestSymbolKey string
	Kind          string
	TargetID      string
	Strength      string
	// Suite/Class/Component are best-effort provenance (design doc §3's
	// meta.suite/meta.class) — set only when the entry came from a
	// suite-level declaration, or (for a marker-based entry) recovered
	// from the test's own most recent attested run when one exists.
	Suite     string
	Class     string
	Component string
}

// Meta is CompileCertifications'/SweepCertifications' JSON provenance
// block (design doc §3's {test, suite, class, strength, state, run_digest,
// attempts, coverage_pct?, mutation_score?}), stored in
// core.SemanticClaim.Meta. TestSymbolKey/TargetKind/TargetID/TestFile are
// carried too — internal bookkeeping SweepCertifications needs to
// re-derive a claim's current state without re-scanning declarations from
// scratch (TestFile is what lets it detect a same-key, moved-file test —
// design doc §3's "stale when the test… moved").
type Meta struct {
	Test          string   `json:"test"`
	Suite         string   `json:"suite,omitempty"`
	Class         string   `json:"class,omitempty"`
	Strength      string   `json:"strength"`
	State         string   `json:"state"`
	RunDigest     string   `json:"run_digest,omitempty"`
	Attempts      int      `json:"attempts,omitempty"`
	CoveragePct   *float64 `json:"coverage_pct,omitempty"`
	MutationScore *float64 `json:"mutation_score,omitempty"`

	TestSymbolKey string `json:"test_symbol_key"`
	TargetKind    string `json:"target_kind"`
	TargetID      string `json:"target_id"`
	TestFile      string `json:"test_file"`
}

// CompileCertifications compiles service's declared certifications
// (certify_declarations markers plus suite-level #Certifies entries) into
// the certifications layer: one core.SemanticClaim per (target, test),
// tiered and stated per design doc §3. watermark stamps CompiledAtWatermark;
// now is the injected clock (repo convention: never time.Now() in
// compile/analysis logic).
//
// repoDir and git are CV-5's own addition (design doc §1.1, O114): the
// working tree evaluateCertificate's reach check (CertificateStateAt,
// reachUnchangedSince) runs against, and the injected GitRunner it runs
// with. Both are injected rather than resolved from the service's own
// recorded ServiceMeta.Path deliberately — a store's recorded path is not
// always a live git checkout on the machine compiling against it (a
// restored store, a different clone), and a caller that does not have a
// real repository to check reach against (most tests) passes a stub
// GitRunner rather than CompileCertifications silently trusting a path it
// was never told to trust.
func CompileCertifications(ctx context.Context, st store.Store, service, watermark string, now time.Time, repoDir string, git GitRunner) (Report, error) {
	var report Report

	decls, err := st.CertifyDeclarationsForService(ctx, service)
	if err != nil {
		return report, fmt.Errorf("CompileCertifications: CertifyDeclarationsForService: %w", err)
	}
	suites, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		return report, fmt.Errorf("CompileCertifications: TestSuiteStatuses: %w", err)
	}
	nodes, err := serviceSymbols(ctx, st, service)
	if err != nil {
		return report, fmt.Errorf("CompileCertifications: serviceSymbols: %w", err)
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		return report, fmt.Errorf("CompileCertifications: TypedEdges: %w", err)
	}

	byName := testSymbolsByBareName(nodes)
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
	}

	var entries []declaredEntry
	for _, d := range decls {
		entries = append(entries, declaredEntry{
			TestSymbolKey: d.TestSymbolKey, Kind: d.Kind, TargetID: d.TargetID, Strength: d.Strength,
		})
	}
	for _, su := range suites {
		var refs []testevidence.CertifyRef
		raw := su.DeclaredCertifiesJSON
		if raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(raw), &refs); err != nil {
			continue // malformed suite-level JSON: skip, never abort the whole compile
		}
		for _, ref := range refs {
			if len(ref.Tests) == 0 {
				// "absent = every Test* case of the suite" (design doc
				// §1) needs a suite→test-symbol membership fathom does
				// not yet have a store read for (TC-17 landed the
				// `certifies` field itself; this expansion is a separate,
				// still-open gap) — recorded as a documented stage-1 gap,
				// never silently guessed.
				report.Skipped = append(report.Skipped,
					fmt.Sprintf("suite %s/%s: certifies %s %s declares no explicit tests (every-case expansion not yet available)",
						su.Component, su.Suite, ref.Kind, ref.ID))
				continue
			}
			for _, testName := range ref.Tests {
				key, ok := byName[testName]
				if !ok {
					report.Skipped = append(report.Skipped,
						fmt.Sprintf("suite %s/%s: certifies %s %s: test %q not found or ambiguous in the index",
							su.Component, su.Suite, ref.Kind, ref.ID, testName))
					continue
				}
				entries = append(entries, declaredEntry{
					TestSymbolKey: key, Kind: ref.Kind, TargetID: ref.ID, Strength: ref.Strength,
					Suite: su.Suite, Class: su.Class, Component: su.Component,
				})
			}
		}
	}
	report.Declared = len(entries)
	if len(entries) == 0 {
		return report, nil
	}

	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, edges)

	targetCache := make(map[string]Target)
	var claims []core.SemanticClaim
	for _, e := range entries {
		targetKey := e.Kind + ":" + e.TargetID
		target, ok := targetCache[targetKey]
		if !ok {
			target, err = ResolveTarget(ctx, st, service, e.Kind, e.TargetID)
			if err != nil {
				return report, fmt.Errorf("CompileCertifications: ResolveTarget(%s, %s): %w", e.Kind, e.TargetID, err)
			}
			targetCache[targetKey] = target
		}

		claim, cerr := evaluateCertificate(ctx, st, service, watermark, now, e, target, g, byKey, repoDir, git)
		if cerr != nil {
			return report, cerr
		}
		if claim == nil {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: test symbol %q not found in the index", targetKey, e.TestSymbolKey))
			continue
		}
		claims = append(claims, *claim)
	}

	if len(claims) > 0 {
		if err := st.PutClaims(ctx, claims, nil); err != nil {
			return report, fmt.Errorf("CompileCertifications: PutClaims: %w", err)
		}
	}
	report.ClaimsWritten = len(claims)
	sort.Strings(report.Skipped)
	return report, nil
}

// evaluateCertificate builds the compiled claim for one declared (test,
// target) pairing — the shared core CompileCertifications and
// SweepCertifications both use, so a claim's state is derived by exactly
// one code path. Returns (nil, nil) when the test symbol itself is not in
// the index — never a fabricated claim for a symbol that does not exist.
//
// repoDir/git are CV-5's addition: threaded straight through to
// CertificateStateAt's reachUnchangedSince closure (CV-3/CV-4) so a
// changed reach both stales an already-compiled claim's Status and drops
// its State to unproven (design doc §1.1, O114).
func evaluateCertificate(ctx context.Context, st store.Store, service, watermark string, now time.Time, e declaredEntry, target Target, g *callgraph.Graph, byKey map[string]core.SymbolNode, repoDir string, git GitRunner) (*core.SemanticClaim, error) {
	testNode, testExists := byKey[e.TestSymbolKey]
	if !testExists {
		return nil, nil
	}

	strength := e.Strength
	if strength == "" {
		strength = string(extractor.DefaultCertifyStrength)
	}

	// reachedSyms is the subset of target.Symbols the call graph actually
	// confirms e.TestSymbolKey reaches — NOT just whether any one of them
	// is reached. A multi-symbol target (kind "rule"/"port"/"spec" can
	// all resolve to symbols spanning several files, resolve.go) must
	// only certify the files backing what was actually proven reachable
	// (I43); reachedSyms is empty and unused for the vacuous-reach kinds
	// (invariant/incident), whose target.Symbols is intentionally empty.
	var reachedSyms []string
	if target.Resolved && len(target.Symbols) > 0 {
		reachedSyms = reachedSymbols(g, e.TestSymbolKey, target.Symbols)
	}
	reached := target.Resolved && (len(target.Symbols) == 0 || len(reachedSyms) > 0)

	runs, err := st.TestCaseRunsForSymbol(ctx, service, e.TestSymbolKey, 3)
	if err != nil {
		return nil, fmt.Errorf("evaluateCertificate: TestCaseRunsForSymbol: %w", err)
	}

	subject := e.Kind + ":" + e.TargetID
	// Scope is the test file plus the files of the symbols the test
	// REACHES that the target names (Certification.Scope's own doc
	// comment, internal/verify/verify.go) — never target.Files wholesale.
	// A target with Symbols (rule/port/spec) narrows to reachedSyms'
	// files; the vacuous-reach kinds (invariant/incident, Symbols always
	// empty) keep target.Files as their whole Scope, same as before.
	// Computed BEFORE the state ladder (rather than after, as before
	// CV-5): CertificateStateAt's reachUnchangedSince closure needs
	// exactly this file set — the certificate's reach — to check against.
	scopeFiles := target.Files
	if len(target.Symbols) > 0 {
		scopeFiles = filesForSymbols(reachedSyms, byKey)
	}
	scope := sortedUnique(append(append([]string(nil), testNode.FilePath), scopeFiles...))

	reachUnchanged := func(commit string) (bool, error) {
		return reachUnchangedSince(git, repoDir, commit, scope)
	}
	state, last, _, reason, stale, serr := CertificateStateAt(reached, runs, reachUnchanged)
	if serr != nil {
		// A git failure (no repository at repoDir, an unknown revision in
		// a shallow clone, git not installed) is a MISSING INPUT (I44),
		// never a reason to abort every other certificate this compile
		// was about to write: CertificateStateAt itself stays strict and
		// propagates the error honestly (never silently guessing
		// "unchanged"), but this integration point degrades to unproven
		// with the failure named in the reason, the same "missing input
		// -> unproven, never conserved" rule design doc §1.1/I44 states
		// for every other stage-1 input.
		state, reason, stale = StateUnproven, "reach could not be checked: "+serr.Error(), false
	}
	var runDigest string
	var attempts int
	var suite, class, component = e.Suite, e.Class, e.Component
	if reached && len(runs) > 0 {
		runDigest = last.RunDigest
		attempts = last.Attempts
		if suite == "" {
			suite, component = last.Suite, last.Component
		}
	}

	// Tier follows State directly now (CV-5): Verified only when the
	// ladder actually says Live — the same outcome the old inline
	// pass/flaky check produced, but derived from the ONE ladder
	// computation above rather than a second, parallel one that could
	// drift from it (a run that is unbound, or whose reach changed, must
	// never earn Verified just because its last run happened to pass).
	tier := core.TierDocClaimed
	if extractor.CertifyKind(e.Kind) != extractor.CertifyKindBehaviour && reached {
		tier = core.TierPreScan
		if state == StateLive {
			tier = core.TierVerified
		}
	}

	status := core.ClaimFresh
	switch {
	case !target.Resolved:
		status = core.ClaimContradicted
	case stale:
		// CV-5/I43: a changed reach stales the CLAIM's own status,
		// distinct from the certificate ladder's State (already dropped
		// to unproven by CertificateStateAt above) — design doc §1.1's
		// "the claim's status stale."
		status = core.ClaimStale
	}

	rule := fmt.Sprintf("%s certifies %s %s (%s) — %s", testBareName(testNode.Name), e.Kind, e.TargetID, strength, state)
	if reason != "" {
		rule += " (" + reason + ")"
	}

	m := Meta{
		Test: testBareName(testNode.Name), Suite: suite, Class: class, Strength: strength, State: string(state),
		RunDigest: runDigest, Attempts: attempts,
		TestSymbolKey: e.TestSymbolKey, TargetKind: e.Kind, TargetID: e.TargetID, TestFile: testNode.FilePath,
	}
	_ = component // reserved for a future per-component gap report; not yet rendered
	metaJSON, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("evaluateCertificate: marshal meta: %w", err)
	}

	claim := core.SemanticClaim{
		ClaimID:             core.ClaimID(Layer, Kind, subject, e.TestSymbolKey),
		ServiceID:           service,
		Layer:               Layer,
		LayerVersion:        LayerVersion,
		Kind:                Kind,
		Subject:             subject,
		Rule:                rule,
		Scope:               scope,
		Tier:                tier,
		TierProfile:         TierProfile,
		Status:              status,
		PromptVersion:       PromptVersion,
		ReproKey:            e.TestSymbolKey,
		CompiledAtWatermark: watermark,
		CompiledAt:          now,
		Meta:                metaJSON,
		AssertedBy:          core.AssertedBy("compile", "certify"),
	}
	return &claim, nil
}

// reachedSymbols returns the subset of symbols the call graph confirms
// test reaches (typed edges included) — the same walk test_impact uses
// (internal/analysis/callgraph.Reachable), just checked from the test
// forward rather than from a symbol backward, and collecting EVERY match
// rather than stopping at the first: evaluateCertificate's Scope must be
// built from exactly what was proven reachable (I43), never from the
// target's full symbol/file set on the strength of any single match.
func reachedSymbols(g *callgraph.Graph, test string, symbols []string) []string {
	var out []string
	for _, s := range symbols {
		if callgraph.Reachable(g, test, s) {
			out = append(out, s)
		}
	}
	return out
}

// filesForSymbols maps each of symbols (canonical SymbolKeys, as found in
// byKey) to its defining file, deduplicated and sorted — the files
// backing exactly the reached subset a multi-symbol target names, never
// that target's full Files set wholesale (I43).
func filesForSymbols(symbols []string, byKey map[string]core.SymbolNode) []string {
	var files []string
	for _, s := range symbols {
		if n, ok := byKey[s]; ok {
			files = append(files, n.FilePath)
		}
	}
	return sortedUnique(files)
}

// testSymbolsByBareName indexes every Test* Go function by its bare name
// — "" once a name is ambiguous (shared by two Test* funcs in different
// packages), same discipline testevidence.JoinCasesToSymbols uses, so a
// suite-level #Certifies `tests:` entry can be resolved to a symbol key
// without silently guessing.
func testSymbolsByBareName(nodes []core.SymbolNode) map[string]string {
	byName := make(map[string]string, len(nodes))
	seen := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n.Kind != core.KindGoFunc || !strings.HasPrefix(n.Name, "Test") {
			continue
		}
		if seen[n.Name] {
			byName[n.Name] = ""
			continue
		}
		seen[n.Name] = true
		byName[n.Name] = n.Key()
	}
	out := make(map[string]string, len(byName))
	for name, key := range byName {
		if key != "" {
			out[name] = key
		}
	}
	return out
}

// testBareName returns a Test* symbol's own bare function name — n.Name
// already IS the bare name for a top-level Go function (core.SymbolNode
// carries no receiver qualification for a plain func), kept as a named
// helper so the claim-building call sites read self-documented.
func testBareName(name string) string { return name }
