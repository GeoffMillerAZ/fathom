package langbench

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var updateBoard = flag.Bool("update", false, "rewrite the committed excellence board golden")

const boardGoldenPath = "testdata/excellence-board.golden.md"

func merge(ms ...map[string]*float64) map[string]*float64 {
	out := map[string]*float64{}
	for _, m := range ms {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func m(kv ...any) map[string]*float64 {
	out := map[string]*float64{}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = f(kv[i+1].(float64))
	}
	return out
}

// boardReading builds a finalized reading from metrics, every metric proven
// at oracle tier oracleTier.
func boardReading(id, kind, target, oracleTier string, metrics map[string]*float64) Reading {
	r := Reading{
		Dialect: id, Kind: kind, TargetLevel: target, MeasuredAt: "2026-10-02",
		Subjects:         []Subject{{Name: "o/r", Files: 10, Bytes: 1 << 20}},
		Oracles:          []OracleRun{{Tier: oracleTier, Tool: "oracle-" + id, Version: "1"}},
		RequiredKinds:    []string{"class"},
		KindIntersection: []string{"class"},
		Metrics:          metrics, Tenancy: okTenancy(),
	}
	r.MetricTiers = tiers(oracleTier, metrics)
	r.Finalize()
	return r
}

func langL1(v float64) map[string]*float64 { return l1Metrics(v) }
func langL2(v float64) map[string]*float64 { return m(MCallRecall, v, MCallPrecision, v) }
func langL3(v, edgeRecall float64) map[string]*float64 {
	return m(MImportResolution, v, MFileEdgePrecision, v, MFileEdgeRecall, edgeRecall)
}
func langL4(v float64) map[string]*float64 { return m(MDefMapping, v, MTypedRecall, v) }

// boardFixture extends trackerFixture with tiers that cover every bucket:
// L1..L4 met, none, refused, tolerated and unsupported, an excellent item, a
// good item under a value, a good item under a tier-C cap, and an untiered one.
func boardFixture() TrackerInput {
	in := trackerFixture()
	tierOf := map[string]string{"go": "T1", "swift": "T1", "python": "T1", "helm": "T3"}
	for i := range in.Queue {
		in.Queue[i].Tier = tierOf[in.Queue[i].ID]
	}
	extra, err := ParseQueue([]byte(`[
	 {"id":"alpha","name":"Alpha","kind":"language","tier":"T1","target_level":"L4","engine":"tags","plan_row":"LB-Q-alpha"},
	 {"id":"beta","name":"Beta","kind":"language","tier":"T1","target_level":"L3","engine":"tags","plan_row":"LB-Q-beta"},
	 {"id":"gamma","name":"Gamma","kind":"language","tier":"T1","target_level":"L2","engine":"tags","plan_row":"LB-Q-gamma"},
	 {"id":"delta","name":"Delta","kind":"language","tier":"T1","target_level":"L3","engine":"tags","plan_row":"LB-Q-delta"},
	 {"id":"epsilon","name":"Epsilon","kind":"language","tier":"T1","target_level":"L2","engine":"tags","plan_row":"LB-Q-epsilon"},
	 {"id":"zeta","name":"Zeta","kind":"language","tier":"T1","target_level":"L1","engine":"tags","plan_row":"LB-Q-zeta"},
	 {"id":"eta","name":"Eta","kind":"language","tier":"T4","target_level":"L3","engine":"tags","plan_row":"LB-Q-eta"},
	 {"id":"theta","name":"Theta","kind":"language","tier":"T4","target_level":"L2","engine":"tags","plan_row":"LB-Q-theta"},
	 {"id":"fmt-a","name":"Format A","kind":"tooling","tier":"T2","target_level":"L2","engine":"doc","plan_row":"LB-Q-fmt-a"},
	 {"id":"fmt-b","name":"Format B","kind":"tooling","tier":"T2","target_level":"L3","engine":"doc","plan_row":"LB-Q-fmt-b"},
	 {"id":"fmt-c","name":"Format C","kind":"tooling","tier":"T2","target_level":"L2","engine":"doc","plan_row":"LB-Q-fmt-c"},
	 {"id":"loose","name":"Loose","kind":"tooling","target_level":"L1","engine":"doc","plan_row":"LB-Q-loose"}]`))
	if err != nil {
		panic(err)
	}
	in.Queue = append(in.Queue, extra...)
	toolL1, toolL2 := m(MSymRecall, .99, MSymPrecision, .99), m(MRefRecall, .96, MRefPrecision, .96)
	in.Readings["alpha"] = boardReading("alpha", "language", "L4", "A", merge(langL1(.97), langL2(.95), langL3(.97, .6), langL4(.97)))
	in.Readings["beta"] = boardReading("beta", "language", "L3", "A", merge(langL1(.97), langL2(.95), langL3(.97, .4)))
	// L1 proven at tier B (excellent), L2 at tier C (capped at good).
	gamma := boardReading("gamma", "language", "L2", "B", merge(langL1(.97), langL2(.95)))
	for k := range langL2(0) {
		gamma.MetricTiers[k] = "C"
	}
	gamma.Finalize()
	in.Readings["gamma"] = gamma
	in.Readings["delta"] = boardReading("delta", "language", "L3", "A", merge(langL1(.97), langL2(.6)))
	in.Readings["epsilon"] = boardReading("epsilon", "language", "L2", "A", langL1(.5))
	in.Readings["zeta"] = boardReading("zeta", "language", "L1", "A", langL1(.97))
	in.Readings["eta"] = boardReading("eta", "language", "L3", "A", merge(langL1(.97), langL2(.95), langL3(.97, .6)))
	in.Readings["fmt-a"] = boardReading("fmt-a", "tooling", "L2", "A", merge(toolL1, toolL2))
	in.Readings["fmt-b"] = boardReading("fmt-b", "tooling", "L3", "A", merge(toolL1, toolL2))
	in.Registered["theta"] = Registration{Extractor: "dialects/theta", LangTag: "theta"}
	// eta has a reading but is refused: it claims nothing; theta is registered
	// with no reading (tolerated); fmt-c has neither (unsupported).
	in.Refused = map[string]string{"eta": "LB-Q-eta-rest: parse errors over the 5% line"}
	return in
}

// TestRenderBoardGolden (LB-F111): the board rendered from boardFixture is
// exactly the committed golden; -update rewrites it.
func TestRenderBoardGolden(t *testing.T) {
	got := RenderBoard(boardFixture())
	if *updateBoard {
		if err := os.WriteFile(boardGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(boardGoldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("RenderBoard differs from %s; regenerate with `go test ./internal/langbench -run TestRenderBoardGolden -update`\n--- got\n%s", boardGoldenPath, got)
	}
	if RenderBoard(boardFixture()) != got {
		t.Error("RenderBoard is not deterministic")
	}
}

// TestRenderBoardBuckets pins the next-gap rules and counts the golden
// shows, so a golden regenerated over a wrong rule is still caught.
func TestRenderBoardBuckets(t *testing.T) {
	got := RenderBoard(boardFixture())
	for _, want := range []string{
		"| Alpha | language | L4 | L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent) | excellent at target |",
		"| Beta | language | L3 | L3 (L1 excellent, L2 excellent, L3 good) | L3 good: file_edge_recall 0.4 < 0.5 |",
		"| Gamma | language | L2 | L2 (L1 excellent, L2 good) | L2 good: oracle tier C caps at good |",
		"| Delta | language | L3 | L1 (L1 excellent) | L2 not met: call_recall 0.6 < 0.75 |",
		"| Epsilon | language | L2 | none (measured) | L1 not met: sym_recall 0.5 < 0.85 |",
		"| Python | language | L4 | L1 (L1 excellent) | L2 not met: call_recall not measured |",
		"| Eta | language | L3 | refused | re-plan (LB-Q-eta-rest refused) |",
		"| Theta | language | L2 | tolerated | no reading (LB-Q-theta) |",
		"| Swift | language | L3 | unsupported | no dialect (LB-Q-swift) |",
		"Untiered entries (outside the excellence target set): 1 (loose).",
		"| **Total** | language | ",
		"| **Total** | tool format | ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("board lacks %q:\n%s", want, got)
		}
	}
	if !strings.HasPrefix(got, "## Excellence board (generated)\n") {
		t.Error("board does not open with its heading")
	}
	doc := RenderTracker(boardFixture())
	if i, j := strings.Index(doc, "## Excellence board (generated)"), strings.Index(doc, "## Support levels (generated)"); i < 0 || j < i {
		t.Errorf("the board must precede the support levels: %d, %d", i, j)
	}
}
