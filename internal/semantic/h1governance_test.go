package semantic

// h1governance_test.go — wave O2, item 5 (docs/bench/readings/
// vs-pilot-6-second-reading.md §5): reproduces the five "what governs this"
// scenariobench tasks (testdata/scenariobench/lookups-vs.json H1-T01/T02/
// T08/T12/T13) OFFLINE, against THIS repository's own governing docs at
// HEAD, with an EMPTY compiled layer (claims=nil) — the raw fallback is the
// only mechanism in play, exactly the shape vs-pilot-6's second reading
// found broken: the raw fallback fired on 8 of 10 trials but never once
// served the rule the oracle wanted (0 of 10 from the raw row itself).
//
// Expectations use the served section identity (Subject), because ADR-080 d2
// replaces heading text with instructions and withholds instruction-free sections.
// This retains the "did the right document/section get served"
// granularity indexbench's GoverningRuleOracle already uses — never the
// exact prompt-answer substring, which is the scorer's job, not this
// ranker's.
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// h1LoadDoc reads one governing doc from the real repository tree — this
// test's own working directory is internal/semantic, two levels below the
// repo root.
func h1LoadDoc(t *testing.T, rel string) core.SourceFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return core.SourceFile{Path: rel, Content: data}
}

// h1Corpus loads exactly the obligations layer's own real corpus include
// list (internal/semantic/layers/obligations/layer.cue) at HEAD: the root
// AGENTS.md, ADR.md, docs/plans/README.md, and every nested AGENTS.md this
// repository actually carries.
func h1Corpus(t *testing.T) []core.SourceFile {
	t.Helper()
	paths := []string{
		"AGENTS.md", "ADR.md", "docs/plans/README.md",
		"internal/AGENTS.md", "docs/AGENTS.md", "schema/AGENTS.md", "cmd/fathom/AGENTS.md",
	}
	docs := make([]core.SourceFile, 0, len(paths))
	for _, p := range paths {
		docs = append(docs, h1LoadDoc(t, p))
	}
	return docs
}

// TestH1GovernanceTasks_RawFallbackOffline is a MIXED-STATUS reproduction
// (wave O3, item 3): like VB-12's own free-oracle reading (docs/plans/
// verbs.md, "skip_test: true... free oracle run; evidence is the committed
// baseline and post-change readings"), a real-corpus lexical ranking
// quality number is still exploratory — ADR.md and the AGENTS.md guides
// grow and reorganise, and a wording change five ADRs away can legitimately
// shift a borderline case in or out of a top-3 window. But the two cases
// wave O2 already fixed (H1-T01, H1-T12) are no longer merely diagnostic:
// they pin a real regression surface (the relevance tie-break inside a
// governing-scope tier), and a future corpus edit or ranking change that
// breaks them must fail the build, not just log a quieter MISS. The other
// three (H1-T02, H1-T08, H1-T13) stay diagnostic-only, each carrying the
// wave O3 finding for WHY cross-tier promotion (item 1) and behaviour-token
// relevance (item 2) do not close them — never silently skipped, and never
// special-cased to force a PASS; see .fathom/observations.md O75 for the
// full account.
func TestH1GovernanceTasks_RawFallbackOffline(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	corpus := h1Corpus(t)

	cases := []struct {
		id       string
		file     string
		extra    []string // caller-resolved symbol/import tokens, item 1's optional signal
		want     string   // substring expected in a top-3 row's section identity
		note     string
		hardGate bool   // wave O3 item 3: a MISS here fails the build, not just logs
		missWhy  string // required when !hardGate: why this case still misses, so a MISS never reads as an unexplained skip
	}{
		{
			id: "H1-T01", file: "internal/extractor/goextract.go",
			want: "Adding things", hardGate: true,
			note: "internal/AGENTS.md's extractor-wiring rule (fusion.New, golden-test)",
		},
		{
			id: "H1-T02", file: "internal/core/model.go", extra: []string{"Edge", "EdgeKind"},
			want: "ADR-021",
			note: "typed call-edge kinds live on core.Edge",
			missWhy: "wave O4 item 1 (the narrowed cross-tier cap: \"the nearer candidate is " +
				"itself STRONG\", not \"the nearer candidate has ANY hit\") DOES fix the " +
				"originally-diagnosed block — internal/AGENTS.md's other same-tier sections carry " +
				"only a weak, ancestor-only hit on the bare token \"internal\" and no longer keep " +
				"their tier against a strong root candidate; proven in isolation, on a clean " +
				"synthetic corpus free of this repo's own self-reference " +
				"(TestCrossTierCloser_AncestorOnlyHitOnNearerDoesNotBlockPromotion, " +
				"TestRawFallbackRules_CrossTierPromotion_AncestorOnlyHitDoesNotBlockPromotion — " +
				"both now pass; TestRawFallbackRules_CrossTierPromotion_StrongHitOnNestedGuideBlocksPromotion " +
				"confirms a genuinely strong nearer hit still blocks it, as designed). But THIS " +
				"real-corpus case still misses top-3, for a newly-discovered, DIFFERENT reason " +
				"outside item 1's scope: several OTHER root-tier ADR.md sections (chiefly ADR-037 " +
				"itself, whose own multi-wave addendum narrates this exact test using its target " +
				"words \"model\"/\"core\"/\"Edge\"/\"EdgeKind\" verbatim) also cross the packageWeight " +
				"\"strong\" bar on ordinary domain vocabulary and, being longer and more " +
				"discursive, out-score ADR-021 on raw hit-weighted magnitude — a same-tier " +
				"(wave O2) relevance-ranking limitation this repo's own corpus growth keeps " +
				"widening every time a wave documents the gap using the gap's own tokens; not " +
				"something a cross-tier blocking fix can reach.",
		},
		{
			id: "H1-T08", file: "cmd/fathom/emit.go",
			want: "Non-negotiables",
			note: "root AGENTS.md's workspace/ convention row",
			missWhy: "wave O3 item 2 (behaviour-token relevance) needs the edited file's own " +
				"source text to scan for a signal (os.WriteFile, MkdirAll, …) — but " +
				"cmd/fathom/emit.go is a synthetic scenariobench path with no real file on disk " +
				"in this repo, so BehaviorTokens has nothing to read and the mechanism cannot " +
				"fire here. The mechanism itself works when a file exists: " +
				"TestRawFallbackRules_BehaviorTokenPromotesUntiedRootConvention and " +
				"TestClient_RawFallbackObligations_BehaviorTokenFromFileContent both prove it " +
				"promotes this exact convention given real source text.",
		},
		{
			id: "H1-T12", file: "internal/core/model.go",
			want: "domain model", hardGate: true,
			note: "shared types live in internal/core, extend additively",
		},
		{
			id: "H1-T13", file: "internal/core/model.go",
			want: "ADR-014",
			note: "companion allowlist / hot-loop tool rule now lives in ADR-014's own section",
			missWhy: "ADR-014's own section carries ZERO lexical relevance to model.go (no hit on " +
				"\"model\"/\"internal\"/\"core\", nor any caller-supplied symbol) — it is the " +
				"FARTHER-tier candidate here, and cross-tier promotion only ever promotes a " +
				"farther candidate that is itself STRONG. A rule that has drifted to name nothing " +
				"lexically about the file it governs needs a different mechanism (semantic, not " +
				"lexical) to be found at all — out of this wave's deterministic, no-LLM scope.",
		},
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if !c.hardGate && c.missWhy == "" {
				t.Fatalf("%s: test setup bug — a non-hard-gate case must carry missWhy so a MISS never reads as an unexplained skip", c.id)
			}
			rows, err := RawFallbackRules(nil, layer, corpus, c.file, 3, "offline H1 repro ("+c.note+")", c.extra, nil)
			if err != nil {
				t.Fatalf("RawFallbackRules: %v", err)
			}
			found := false
			for _, r := range rows {
				if strings.Contains(r.Subject, slugify(c.want)) {
					found = true
				}
			}
			if found {
				t.Logf("%s: PASS — found %q among top-3 rows", c.id, c.want)
				return
			}
			if c.hardGate {
				t.Errorf("%s: MISS (HARD GATE) — %q not in top-3 rows: %+v", c.id, c.want, rows)
				return
			}
			t.Logf("%s: MISS (diagnostic, expected) — %q not in top-3 rows: %+v\nwhy: %s", c.id, c.want, rows, c.missWhy)
		})
	}
}
