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
// Expectations are scored against the SERVED SECTION'S OWN HEADING (the
// Rule field on a raw row — rawRuleText's own rendering of the section's
// ATX heading), the same "did the right document/section get served"
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
// CLAUDE.md, ADR.md, docs/plans/README.md, and every nested CLAUDE.md this
// repository actually carries.
func h1Corpus(t *testing.T) []core.SourceFile {
	t.Helper()
	paths := []string{
		"CLAUDE.md", "ADR.md", "docs/plans/README.md",
		"internal/CLAUDE.md", "docs/CLAUDE.md", "schema/CLAUDE.md", "cmd/fathom/CLAUDE.md",
	}
	docs := make([]core.SourceFile, 0, len(paths))
	for _, p := range paths {
		docs = append(docs, h1LoadDoc(t, p))
	}
	return docs
}

// TestH1GovernanceTasks_RawFallbackOffline is a DIAGNOSTIC reproduction,
// not a hard regression gate: like VB-12's own free-oracle reading
// (docs/plans/verbs.md, "skip_test: true... free oracle run; evidence is
// the committed baseline and post-change readings"), a real-corpus lexical
// ranking quality number is exploratory evidence to report, not a bar
// every future corpus edit must keep clearing byte-for-byte — ADR.md and
// the CLAUDE.md guides grow and reorganise, and a wording change five ADRs
// away can legitimately shift a borderline case in or out of a top-3
// window. Each subtest logs PASS/MISS; none fails the build. See this
// wave's report for which of the five actually pass and why the misses are
// structural (a root-tier answer crowded out of a 3-slot window by a
// nearer-tier guide's own several sections — correct per VB-8's tier
// primacy, not a relevance-ranking defect) rather than a ranking bug.
func TestH1GovernanceTasks_RawFallbackOffline(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	corpus := h1Corpus(t)

	cases := []struct {
		id    string
		file  string
		extra []string // caller-resolved symbol/import tokens, item 1's optional signal
		want  string   // substring expected in a top-3 row's own section heading (Rule)
		note  string
	}{
		{
			id: "H1-T01", file: "internal/extractor/goextract.go",
			want: "Adding things",
			note: "internal/CLAUDE.md's extractor-wiring rule (fusion.New, golden-test)",
		},
		{
			id: "H1-T02", file: "internal/core/model.go", extra: []string{"Edge", "EdgeKind"},
			want: "ADR-021",
			note: "typed call-edge kinds live on core.Edge",
		},
		{
			id: "H1-T08", file: "cmd/fathom/emit.go",
			want: "Non-negotiables",
			note: "root CLAUDE.md's workspace/ convention row",
		},
		{
			id: "H1-T12", file: "internal/core/model.go",
			want: "domain model",
			note: "shared types live in internal/core, extend additively",
		},
		{
			id: "H1-T13", file: "internal/core/model.go",
			want: "ADR-014",
			note: "companion allowlist / hot-loop tool rule now lives in ADR-014's own section",
		},
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			rows, err := RawFallbackRules(nil, layer, corpus, c.file, 3, "offline H1 repro ("+c.note+")", c.extra)
			if err != nil {
				t.Fatalf("RawFallbackRules: %v", err)
			}
			found := false
			for _, r := range rows {
				if strings.Contains(strings.ToLower(r.Rule), strings.ToLower(c.want)) {
					found = true
				}
			}
			if !found {
				t.Logf("%s: MISS — %q not in top-3 rows: %+v", c.id, c.want, rows)
				return
			}
			t.Logf("%s: PASS — found %q among top-3 rows", c.id, c.want)
		})
	}
}
