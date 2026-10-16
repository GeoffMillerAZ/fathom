package semantic

// rawfallback_test.go — wave O1 (ADR-037), failing-first. Pins
// RawFallbackRules directly (unit level) ahead of the fuller
// pkg/client/MCP-level proofs in pkg/client/semantic_test.go and
// internal/mcp/rawfallback_tool_test.go.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// rawFallbackTestLayer loads the real embedded obligations layer — the same
// one production compiles against — so promptForLayer's identity and
// allDigestsMatch behave exactly as they do at serve time.
func rawFallbackTestLayer(t *testing.T) LayerDef {
	t.Helper()
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	return layer
}

// freshClaimFor builds a core.SemanticClaim that allDigestsMatch will accept
// as covering sec — the "already compiled, fresh" case a raw candidate must
// never duplicate.
func freshClaimFor(t *testing.T, layer LayerDef, sec Section) core.SemanticClaim {
	t.Helper()
	_, promptVersion, err := promptForLayer(layer)
	if err != nil {
		t.Fatalf("promptForLayer: %v", err)
	}
	return core.SemanticClaim{
		ClaimID: "cl-" + sec.Subject, ServiceID: "svc",
		Layer: layer.Name, LayerVersion: "v1", Kind: KindObligation,
		Subject: sec.Subject, Rule: "compiled rule text",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, InputDigest: sec.Digest, PromptVersion: promptVersion,
	}
}

func TestRawFallbackRules_NestedGuideRankedAheadOfRootOther(t *testing.T) {
	layer := rawFallbackTestLayer(t)

	rootDoc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- Root rule, compiled already.")}
	nestedDoc := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte("# internal guide\n\n- Register every extractor in fusion.New.")}
	adrDoc := core.SourceFile{Path: "ADR.md", Content: []byte("## ADR-001: Some decision\n\nSome unrelated decision text.")}

	rootSec := SplitMarkdownSections(rootDoc.Path, rootDoc.Content)[0]
	claim := freshClaimFor(t, layer, rootSec) // the ROOT section is already compiled and fresh

	corpus := []core.SourceFile{rootDoc, nestedDoc, adrDoc}
	rows, err := RawFallbackRules([]core.SemanticClaim{claim}, layer, corpus, "internal/extractor/goextract.go", 2, "compiled layer incomplete: budget_stopped; raw nearest guide served", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected raw rows for the uncompiled nested guide, got none")
	}
	if rows[0].Tier != "raw" {
		t.Errorf("rows[0].Tier = %q, want %q", rows[0].Tier, "raw")
	}
	if rows[0].Layer != LayerObligations {
		t.Errorf("rows[0].Layer = %q, want %q", rows[0].Layer, LayerObligations)
	}
	if rows[0].Src != "internal/CLAUDE.md#L1-L3" {
		t.Errorf("rows[0].Src = %q, want the nested guide's own citation", rows[0].Src)
	}
	if rows[0].Why == "" {
		t.Error("rows[0].Why should be set on a raw row")
	}
	// The ALREADY-COMPILED root section must never re-appear as a raw
	// candidate — the whole point of the digest/prompt-version gate.
	for _, r := range rows {
		if r.Src == "CLAUDE.md#L1-L3" {
			t.Errorf("compiled root section re-appeared as a raw row: %+v", r)
		}
	}
	// ADR.md (root-other, tier 3) must never rank AHEAD of the nested guide
	// (tier 1/2) — nearest first, regardless of source order.
	if len(rows) > 1 {
		for i := 1; i < len(rows); i++ {
			if rows[i].Src == "internal/CLAUDE.md#L1-L3" {
				t.Errorf("nested guide row not ranked first: rows=%+v", rows)
			}
		}
	}
}

func TestRawFallbackRules_NoGapReturnsNil(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- Only rule, already compiled.")}
	sec := SplitMarkdownSections(doc.Path, doc.Content)[0]
	claim := freshClaimFor(t, layer, sec)

	rows, err := RawFallbackRules([]core.SemanticClaim{claim}, layer, []core.SourceFile{doc}, "pkg/foo.go", 2, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows for a fully-compiled corpus, got %+v", rows)
	}
}

func TestRawFallbackRules_RespectsMax(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	docA := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte("# internal\n\n- Rule A.")}
	docB := core.SourceFile{Path: "internal/store/CLAUDE.md", Content: []byte("# store\n\n- Rule B, deeper and nearer to the file.")}
	corpus := []core.SourceFile{docA, docB}

	rows, err := RawFallbackRules(nil, layer, corpus, "internal/store/backend.go", 1, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (max respected)", len(rows))
	}
	if rows[0].Src != "internal/store/CLAUDE.md#L1-L3" {
		t.Errorf("rows[0].Src = %q, want the DEEPER (nearer) guide to win the single slot", rows[0].Src)
	}
}

func TestRawFallbackRules_GuardClauses(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- A rule.")}

	cases := []struct {
		name string
		file string
		max  int
		docs []core.SourceFile
	}{
		{"empty file (root scope)", "", 2, []core.SourceFile{doc}},
		{"zero max", "pkg/foo.go", 0, []core.SourceFile{doc}},
		{"negative max", "pkg/foo.go", -1, []core.SourceFile{doc}},
		{"empty corpus", "pkg/foo.go", 2, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := RawFallbackRules(nil, layer, c.docs, c.file, c.max, "why", nil, nil)
			if err != nil {
				t.Fatalf("RawFallbackRules: %v", err)
			}
			if rows != nil {
				t.Errorf("expected nil rows, got %+v", rows)
			}
		})
	}
}

func TestRawFallbackRules_UnrelatedGuideNeverMatches(t *testing.T) {
	// A nested guide that governs a DIFFERENT subtree entirely must never
	// show up as a raw candidate for file — impliedGuideScope/
	// rankByGoverningScope must classify it as no-match (tier < 0), not
	// root-tier-by-accident.
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "cmd/fathom/CLAUDE.md", Content: []byte("# cmd/fathom\n\n- UI doctrine.")}
	rows, err := RawFallbackRules(nil, layer, []core.SourceFile{doc}, "internal/store/backend.go", 2, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows — cmd/fathom/CLAUDE.md does not govern internal/store, got %+v", rows)
	}
}

// TestRawFallbackRules_RelevanceRanksExtractorSectionFirst is wave O2's
// flagship regression (vs-pilot-6 second reading §5): a nested guide with
// SIX same-tier, same-depth sections — including a "Store purity" decoy
// that mentions "internal" and "extractor" only in passing, and an
// "Adding things" section that is the ONLY one naming how to wire up a new
// extractor (fusion.New, a golden test) — must serve "Adding things"
// FIRST for internal/extractor/goextract.go. Before wave O2, document
// order (StartLine) decided this tie, and "Store purity" (line 3 of this
// fixture) always won — the exact defect the second reading documents.
func TestRawFallbackRules_RelevanceRanksExtractorSectionFirst(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	nested := "# internal/ — engine rules\n\n" +
		"## Determinism is a contract\n\n" +
		"- Sort every output slice; never rely on map iteration order.\n" +
		"- Inject clocks; never call time.Now() in query logic.\n\n" +
		"## Store purity (ADR-020 / ADR-025)\n\n" +
		"internal/store is pure Go — no CGO anywhere in the store or query\n" +
		"layers. CGO in this repo comes only from the tree-sitter extractor\n" +
		"bindings for a different, unrelated reason.\n\n" +
		"## The domain model is the keystone\n\n" +
		"Shared types live in internal/core; extend it additively.\n\n" +
		"## Working conventions\n\n" +
		"- Prefer small commits.\n" +
		"- Keep the changelog current.\n\n" +
		"## Testing discipline\n\n" +
		"- Always run the race detector before claiming green.\n\n" +
		"## Adding things\n\n" +
		"- Extractor: implement extractor.Extractor, register it by extension\n" +
		"  in fusion.New(), add its ext-to-kind mapping in connector, then\n" +
		"  write a golden-test for it against the fixture fleet.\n" +
		"- Analyzer: a pure internal/analysis/<name> package; deterministic.\n" +
		"- MCP tool: registerTool in internal/mcp with a group; cover with a\n" +
		"  fake-backend test.\n" +
		"- Custodian check kind: a CheckKind in internal/custodian, loaded\n" +
		"  from .fathom/concerns.cue.\n"
	doc := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte(nested)}

	rows, err := RawFallbackRules(nil, layer, []core.SourceFile{doc}, "internal/extractor/goextract.go", 6, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected raw rows, got none")
	}
	if rows[0].Rule != "Adding things" {
		t.Fatalf("rows[0].Rule = %q, want %q (the extractor-wiring section) — got rows=%+v", rows[0].Rule, "Adding things", rows)
	}

	// The served citation must resolve to lines that actually contain
	// fusion.New — the bullet an agent needs, not merely the section's
	// opening line (item 4: a long section's citation narrows to the first
	// relevance hit, not its first bytes).
	path, start, end, ok := parseCite(rows[0].Src)
	if !ok {
		t.Fatalf("could not parse Src %q", rows[0].Src)
	}
	if path != "internal/CLAUDE.md" {
		t.Fatalf("Src path = %q, want internal/CLAUDE.md", path)
	}
	lines := strings.Split(nested, "\n")
	if start < 1 || end > len(lines) {
		t.Fatalf("Src range %d-%d out of bounds (doc has %d lines)", start, end, len(lines))
	}
	window := strings.Join(lines[start-1:end], "\n")
	if !strings.Contains(window, "fusion.New") {
		t.Errorf("cited window %q (lines %d-%d) does not contain fusion.New", window, start, end)
	}
}

// TestCiteWindow_NarrowsLongSectionToFirstHit is item 4's direct unit test:
// a section long enough to trigger narrowing (more than rawSnippetWindowLines
// lines) with its one relevant bullet buried past the opening lines must
// cite a window AROUND that bullet, not the section's own first line.
func TestCiteWindow_NarrowsLongSectionToFirstHit(t *testing.T) {
	var b strings.Builder
	b.WriteString("## A long section\n\n")
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&b, "- Filler bullet number %d, about nothing in particular.\n", i)
	}
	b.WriteString("- The extractor recipe: register it in fusion.New().\n")
	for i := 0; i < 4; i++ {
		fmt.Fprintf(&b, "- More filler bullet number %d.\n", i)
	}
	text := b.String()
	sections := SplitMarkdownSections("internal/CLAUDE.md", []byte(text))
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}
	sec := sections[0]
	if sec.EndLine-sec.StartLine < rawSnippetWindowLines {
		t.Fatalf("fixture section too short to exercise narrowing (got %d lines)", sec.EndLine-sec.StartLine)
	}

	tokens := pathRelevanceTokens("internal/extractor/goextract.go")
	start, end := citeWindow(sec, tokens)
	if end-start >= sec.EndLine-sec.StartLine {
		t.Fatalf("citeWindow did not narrow: got %d-%d, full section is %d-%d", start, end, sec.StartLine, sec.EndLine)
	}
	lines := strings.Split(text, "\n")
	window := strings.Join(lines[start-1:end], "\n")
	if !strings.Contains(window, "fusion.New") {
		t.Errorf("narrowed window %q (lines %d-%d) does not contain the extractor bullet", window, start, end)
	}
}

// TestRawFallbackRules_CrossTierPromotion_StrongRootBeatsBlankNestedGuide
// (wave O3, item 1; ADR-037 addendum) is design (1)'s flagship regression:
// a NESTED guide's own section (package tier — nearer) that names NOTHING
// about the edited file at all must lose its proximity advantage to a
// ROOT-tier section (farther) that strongly names the file's own stem —
// the shape H1-T02 approximates in the real corpus (a package guide with
// no lexical tie loses to a root ADR that names the file's own symbol),
// pinned here against a synthetic, corpus-growth-proof fixture.
func TestRawFallbackRules_CrossTierPromotion_StrongRootBeatsBlankNestedGuide(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	nestedDoc := core.SourceFile{Path: "internal/foo/CLAUDE.md", Content: []byte(
		"# Unrelated policy\n\nWidgets must be logged and reviewed before merging. " +
			"Nothing in this sentence names the edited file at all.\n")}
	rootDoc := core.SourceFile{Path: "ADR.md", Content: []byte(
		"## ADR-099: The bar type\n\nEverything in this subsystem revolves around bar. " +
			"See bar's own tests for details.\n")}
	corpus := []core.SourceFile{nestedDoc, rootDoc}

	rows, err := RawFallbackRules(nil, layer, corpus, "internal/foo/bar.go", 1, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Src != "ADR.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want the root ADR section (%q) — the blank nested guide must not win on proximity alone: rows=%+v", rows[0].Src, "ADR.md#L1-L4", rows)
	}
}

// TestRawFallbackRules_CrossTierPromotion_AncestorOnlyHitDoesNotBlockPromotion
// (wave O4, item 1; ADR-037 addendum) replaces this test's own former
// pinning of the OPPOSITE result: the same fixture as above, except the
// nested guide's section now mentions the file's own ancestor directory
// once ("internal") — a weak, ancestor-only hit, never classified strong.
// H1-T02 (docs/bench readings) found this exact shape in the real repo
// corpus: internal/CLAUDE.md's other sections each carry only a bare
// "internal" mention, which used to be enough to block ADR-021 (strong on
// the file's own symbol) from ever being promoted past them, even though
// none of them actually names the file's own package or symbol. An
// ancestor-only hit is not "the nearer rule names the file" — it is
// evidence the nearer rule names the whole subtree the file merely lives
// in — so it no longer blocks; see crossTierCloser's own doc comment.
func TestRawFallbackRules_CrossTierPromotion_AncestorOnlyHitDoesNotBlockPromotion(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	nestedDoc := core.SourceFile{Path: "internal/foo/CLAUDE.md", Content: []byte(
		"# Unrelated policy\n\nEvery internal package must log before merging widgets.\n")}
	rootDoc := core.SourceFile{Path: "ADR.md", Content: []byte(
		"## ADR-099: The bar type\n\nEverything in this subsystem revolves around bar. " +
			"See bar's own tests for details.\n")}
	corpus := []core.SourceFile{nestedDoc, rootDoc}

	rows, err := RawFallbackRules(nil, layer, corpus, "internal/foo/bar.go", 1, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Src != "ADR.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want the strong root ADR (%q) — a bare ancestor-directory hit on the nested guide must not keep its proximity advantage: rows=%+v", rows[0].Src, "ADR.md#L1-L4", rows)
	}
}

// TestRawFallbackRules_CrossTierPromotion_StrongHitOnNestedGuideBlocksPromotion
// (wave O4, item 1) is the narrowed cap's own regression: the same fixture,
// except the nested guide's section now names the file's own PACKAGE
// directory as its immediate parent ("foo", not merely the shared ancestor
// "internal") — a hit at packageWeight, which IS strong — and must still
// keep the nested guide's proximity advantage; the root ADR must NOT
// leapfrog a nearer candidate that genuinely names the file.
func TestRawFallbackRules_CrossTierPromotion_StrongHitOnNestedGuideBlocksPromotion(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	nestedDoc := core.SourceFile{Path: "internal/foo/CLAUDE.md", Content: []byte(
		"# Foo package policy\n\nEvery file in foo must log before merging widgets.\n")}
	rootDoc := core.SourceFile{Path: "ADR.md", Content: []byte(
		"## ADR-099: The bar type\n\nEverything in this subsystem revolves around bar. " +
			"See bar's own tests for details.\n")}
	corpus := []core.SourceFile{nestedDoc, rootDoc}

	rows, err := RawFallbackRules(nil, layer, corpus, "internal/foo/bar.go", 1, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Src != "internal/foo/CLAUDE.md#L1-L4" {
		t.Errorf("rows[0].Src = %q, want the nested guide (its own package-name hit is strong and must block promotion): rows=%+v", rows[0].Src, rows)
	}
}

// TestRawFallbackRules_BehaviorTokenPromotesUntiedRootConvention (wave O3,
// item 2) is design (2)'s end-to-end proof at the RawFallbackRules level:
// a root-wide convention that names no file/package/symbol of its own
// (H1-T08's own shape) is crowded out of a narrow window by unrelated,
// lexically-louder root sections — UNTIL the caller resolves a behaviour
// token from the edited file's own source (BehaviorTokens) and passes it
// through extraTokens, exactly as pkg/client.RawFallbackObligations now
// does. Confirms the mechanism actually moves the served row, independent
// of any one real file existing on disk (H1-T08 itself can't prove this —
// cmd/fathom/emit.go isn't a real file in this repo).
func TestRawFallbackRules_BehaviorTokenPromotesUntiedRootConvention(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	workspaceDoc := core.SourceFile{Path: "CLAUDE.md", Content: []byte(
		"# Rules\n\n## Non-negotiables — the workspace/ convention\n\nAll build artifacts " +
			"belong under the gitignored workspace/ directory, never elsewhere in the tree. " +
			"Run go tools from cmd as usual.\n")}
	// A decoy that outscores the workspace section on raw lexical noise
	// alone (repeats the file's own generic path tokens once each) but is
	// otherwise unrelated to output-writing behaviour.
	decoyDoc := core.SourceFile{Path: "ADR.md", Content: []byte(
		"## ADR-050: cmd tool\n\nThe cmd package wraps go binaries.\n")}
	corpus := []core.SourceFile{workspaceDoc, decoyDoc}
	file := "cmd/emit.go"

	without, err := RawFallbackRules(nil, layer, corpus, file, 1, "why", nil, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules (no behaviour token): %v", err)
	}
	if len(without) != 1 || without[0].Rule != "ADR-050: cmd tool" {
		t.Fatalf("test setup: without a behaviour token the decoy should win the single slot, got %+v", without)
	}

	behavior := BehaviorTokens(`func emit(dir string) error { return os.WriteFile(filepath.Join(dir, "out"), data, 0o644) }`)
	if len(behavior) == 0 {
		t.Fatalf("test setup: expected BehaviorTokens to detect the os.WriteFile signal")
	}
	with, err := RawFallbackRules(nil, layer, corpus, file, 1, "why", behavior, nil)
	if err != nil {
		t.Fatalf("RawFallbackRules (with behaviour token): %v", err)
	}
	if len(with) != 1 || with[0].Rule != "Non-negotiables — the workspace/ convention" {
		t.Errorf("with the workspace behaviour token, the untied convention must win the slot instead of the decoy: got %+v", with)
	}
}

// TestRawFallbackRules_RestrictsToSubjects: the stale-withheld path puts
// back only the named sections, not every uncompiled one.
func TestRawFallbackRules_RestrictsToSubjects(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "ADR.md", Content: []byte("## ADR-001: One\n\nFirst text.\n\n## ADR-002: Two\n\nSecond text.")}
	secs := SplitMarkdownSections(doc.Path, doc.Content)
	if len(secs) != 2 {
		t.Fatalf("want 2 sections, got %d", len(secs))
	}
	rows, err := RawFallbackRules(nil, layer, []core.SourceFile{doc}, "go.mod", 5, "w", nil, map[string]bool{secs[1].Subject: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !strings.Contains(rows[0].Rule, "ADR-002") {
		t.Fatalf("want only the second section, got %+v", rows)
	}
}
