package langbench

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func trackerFixture() TrackerInput {
	q, _ := ParseQueue([]byte(`[
	 {"id":"go","name":"Go (.go)","kind":"language","target_level":"L4","engine":"native","grammar_source":"native:go/parser","licence":"BSD-3-Clause","plan_row":"LB-Q-go"},
	 {"id":"swift","name":"Swift","kind":"language","target_level":"L3","engine":"tags","grammar_source":"alex-pinkus/tree-sitter-swift","licence":"MIT","plan_row":"LB-Q-swift"},
	 {"id":"python","name":"Python","kind":"language","target_level":"L4","engine":"tags","grammar_source":"tree-sitter/tree-sitter-python","licence":"MIT","plan_row":"LB-Q-python"},
	 {"id":"helm","name":"Helm | charts","kind":"tooling","target_level":"L3","engine":"doc","grammar_source":"pure-go:yaml","licence":"n/a","plan_row":"LB-Q-helm"}]`))
	r := Reading{Dialect: "python", Kind: "language", TargetLevel: "L4", MeasuredAt: "2026-10-01",
		Subjects:         []Subject{{Name: "o/r", Files: 10, Bytes: 1 << 20}},
		Oracles:          []OracleRun{{Tier: "B", Tool: "universal-ctags", Version: "6.2.1"}, {Tier: "A", Tool: "scip-python", Status: "unavailable: no build"}},
		RequiredKinds:    []string{"class"},
		KindIntersection: []string{"class"},
		Metrics:          l1Metrics(0.97), Tenancy: okTenancy()}
	r.MetricTiers = tiers("B", r.Metrics)
	r.Finalize()
	return TrackerInput{
		Template:   "# T\n\n" + TrackerMarker + "\n\ntail\n",
		Queue:      q,
		Readings:   map[string]Reading{"python": r},
		Registered: map[string]Registration{"go": {Extractor: "goextract.go", LangTag: "go"}, "python": {Extractor: "tsextract.go", LangTag: "py"}, "helm": {Extractor: "helmextract.go"}},
		TypedEdges: func(tag string) string {
			return map[string]string{"go": "native:go/types", "py": "scip:scip-python"}[tag]
		},
	}
}

func TestRenderTracker(t *testing.T) {
	doc := RenderTracker(trackerFixture())
	for _, want := range []string{
		"| Go (.go) | language | native | native:go/parser (BSD-3-Clause) | L4 | tolerated |",
		"| Swift | language | tags | alex-pinkus/tree-sitter-swift (MIT) | L3 | unsupported (disclosed) |",
		"| Python | language | tags | tree-sitter/tree-sitter-python (MIT) | L4 | L1 (L1 excellent) | B universal-ctags 6.2.1 | scip:scip-python | 2026-10-01 | LB-Q-python |",
		"| Helm \\| charts | tool format |",
		"\nMilestone M1 progress: languages at L1 excellent 1/20, at L2 0/12, at L3 0/8, at L4 0/6; tool formats at target 0/15\n",
		"\ntail\n",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("tracker lacks %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "scip-python ") && strings.Contains(doc, "A scip-python") {
		t.Error("an unavailable oracle is listed as a tier")
	}
	if errs := CheckTrackerAgainstReadings(doc, trackerFixture().Readings); len(errs) != 0 {
		t.Errorf("rendered tracker fails its own check: %v", errs)
	}
	// I54: a level typed above the reading is refused.
	bad := strings.Replace(doc, "| L4 | L1 (L1 excellent) |", "| L4 | L3 (L1 excellent) |", 1)
	if errs := CheckTrackerAgainstReadings(bad, trackerFixture().Readings); len(errs) == 0 {
		t.Error("a tracker row above its reading passed the check")
	}
	bad = strings.Replace(doc, "| L3 | unsupported (disclosed) |", "| L3 | L1 (L1 good) |", 1)
	if errs := CheckTrackerAgainstReadings(bad, trackerFixture().Readings); len(errs) == 0 {
		t.Error("a level with no reading passed the check")
	}
}

func TestMilestones(t *testing.T) {
	counts := MilestoneCounts{L1Excellent: 20, L1: 25, L2: 12, L3: 8, L4: 6, ToolsAtTarget: 15}
	if got := MilestoneLine(counts); !strings.HasPrefix(got, "Milestone M1 reached") {
		t.Errorf("MilestoneLine = %q", got)
	}
	counts.L4 = 5
	if got := MilestoneLine(counts); !strings.HasPrefix(got, "Milestone M1 progress:") {
		t.Errorf("MilestoneLine = %q", got)
	}
}

// TestLanguagesTrackerIsGenerated (LB-F17, design §12, I54): the committed
// docs/languages.md is exactly the rendering of the queue, the registered
// dialects and the committed readings, and no row claims a level above
// its reading.
func TestLanguagesTrackerIsGenerated(t *testing.T) {
	in, err := TrackerInputFromRepo("../..")
	if err != nil {
		t.Fatal(err)
	}
	want := RenderTracker(in)
	got, err := os.ReadFile("../../docs/languages.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("docs/languages.md is not the generated rendering; run `go run ./cmd/indexbench lang-tracker > docs/languages.md` (never hand-edit the excellence board or the support-level table)")
	}
	if errs := CheckTrackerAgainstReadings(string(got), in.Readings); len(errs) > 0 {
		t.Fatalf("tracker vs readings: %v", errs)
	}
}

// TestRenderTrackerRefusedItem (design §14.2, integration review
// 2026-09-29): a queue item whose row or -rest sibling is refused claims no
// level, whatever its reading measured — the tracker names the refusal and
// the measured level, gives no typed-edge level (nothing is dispatched), and
// the milestones do not count it; its dialect is not dispatched, so the
// unsupported table still discloses its extensions.
func TestRenderTrackerRefusedItem(t *testing.T) {
	in := trackerFixture()
	r := in.Readings["python"]
	in.Readings = map[string]Reading{"python": r}
	in.Refused = map[string]string{"python": "LB-Q-python-rest: parse errors over §14.2's 5% line"}
	doc := RenderTracker(in)
	for _, want := range []string{
		"| Python | language | tags | tree-sitter/tree-sitter-python (MIT) | L4 | refused; measured L1 (L1 excellent) | B universal-ctags 6.2.1 | — | 2026-10-01 | re-plan (LB-Q-python-rest refused) |",
		"\nMilestone M1 progress: languages at L1 excellent 0/20, at L2 0/12, at L3 0/8, at L4 0/6; tool formats at target 0/15\n",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("tracker lacks %q:\n%s", want, doc)
		}
	}
	if errs := CheckTrackerAgainstReadings(doc, in.Readings); len(errs) != 0 {
		t.Errorf("rendered tracker fails its own check: %v", errs)
	}
	if _, ok := in.Dispatched()["python"]; ok {
		t.Error("a refused dialect counts as dispatched")
	}
	if _, ok := in.Dispatched()["go"]; !ok {
		t.Error("a registered, unrefused extractor is not dispatched")
	}
	q, _ := ParseQueue([]byte(`[{"id":"swift","name":"Swift (.swift)","kind":"language","target_level":"L3","engine":"tags","plan_row":"LB-Q-swift"}]`))
	reg := map[string]Registration{"swift": {Extractor: "dialects/swift", LangTag: "swift"}}
	if got := DisclosureTable(q, reg, nil); got[".swift"] != "" {
		t.Errorf("a registered dialect is disclosed: %v", got)
	}
	in2 := TrackerInput{Queue: q, Registered: reg, Refused: map[string]string{"swift": "refused"}}
	if got := DisclosureTable(q, in2.Dispatched(), nil); got[".swift"] != "swift" {
		t.Errorf("a refused dialect is not disclosed: %v", got)
	}
}

// TestRefusedItems: a queue item is refused when its own row or its -rest
// sibling is [-] with disposition refused; a deferred sibling is not.
func TestRefusedItems(t *testing.T) {
	q, _ := ParseQueue([]byte(`[
	 {"id":"a","name":"A","kind":"language","target_level":"L3","engine":"tags","plan_row":"LB-Q-a"},
	 {"id":"b","name":"B","kind":"language","target_level":"L3","engine":"tags","plan_row":"LB-Q-b"},
	 {"id":"c","name":"C","kind":"language","target_level":"L3","engine":"tags","plan_row":"LB-Q-c"}]`))
	plan := verify.ParseMarkdownPlan("s", []byte(`- [ ] a {id: LB-Q-a, oracle: lexical, path: x, pattern: y}
- [-] a rest {id: LB-Q-a-rest, disposition: refused, reason: "parse errors 5.6%"}
- [ ] b {id: LB-Q-b, oracle: lexical, path: x, pattern: y}
- [-] b rest {id: LB-Q-b-rest, disposition: deferred, reason: "later", trigger: "t"}
- [-] c {id: LB-Q-c, disposition: refused, reason: "licence GPL-3.0"}
`))
	got := RefusedItems(q, plan)
	want := map[string]string{"a": "LB-Q-a-rest: parse errors 5.6%", "c": "LB-Q-c: licence GPL-3.0"}
	if len(got) != len(want) || got["a"] != want["a"] || got["c"] != want["c"] {
		t.Fatalf("RefusedItems = %v, want %v", got, want)
	}
}

// TestTrackerDispatchColumnMatchesRegistry (LB-F357): the template's "Engine /
// dispatch" cells name no extractor by hand. Every cell is a placeholder over
// keys fusion's registry serves, and the rendered cell is exactly what the
// registry answers for those keys (the TestMcpToolsDocMatchesRegistry
// principle: the doc holds no copy of a registry fact).
func TestTrackerDispatchColumnMatchesRegistry(t *testing.T) {
	in, err := TrackerInputFromRepo("../..")
	if err != nil {
		t.Fatal(err)
	}
	table := fusion.DispatchTable()
	if errs := CheckDispatchColumn(in.Template, table); len(errs) > 0 {
		t.Fatalf("dispatch column: %s", strings.Join(errs, "\n"))
	}
	doc := RenderTracker(in)
	rendered := 0
	for _, line := range strings.Split(in.Template, "\n") {
		m := dispatchRE.FindStringSubmatch(line)
		if m == nil || !strings.HasPrefix(line, "| ") {
			continue
		}
		keys := strings.Fields(m[1])
		want := DispatchCell(table, keys)
		cells := strings.Split(line, " | ")
		row := strings.Replace(line, m[0], want, 1)
		if !strings.Contains(doc, row) {
			t.Errorf("row %q: rendered doc lacks the registry's cell %q", cells[0], want)
		}
		for _, k := range keys {
			if !strings.Contains(want, table[k]) {
				t.Errorf("row %q: key %q is served by %q, the cell reads %q", cells[0], k, table[k], want)
			}
		}
		rendered++
	}
	if rendered < 30 {
		t.Fatalf("only %d dispatch placeholders in the template: the column is not generated", rendered)
	}
	// The rows the stale prose misstated (lf1 moved Java, C++ and PHP to the tags dialects).
	for _, want := range []string{"| Java | tags dialect `java` |", "| PHP | tags dialect `php` |", "| Rust | Go walker `RustExtractor`;"} {
		if !strings.Contains(doc, want) {
			t.Errorf("rendered tracker lacks %q", want)
		}
	}
}

// TestCheckDispatchColumnCatchesHandTypedCell: the check fails on the exact
// stale cell this item removed, on prose that restates a dispatch fact, and
// on a key no extractor serves.
func TestCheckDispatchColumnCatchesHandTypedCell(t *testing.T) {
	table := map[string]string{".java": "tags dialect `java`"}
	head := dispatchHeader + " Grammar |\n|---|---|---|\n"
	for name, tmpl := range map[string]string{
		"hand-typed cell":     head + "| Java | heuristic (`javaextract.go`) dispatched | yes |\n",
		"prose restates":      head + "| Java | {{dispatch .java}}; dispatched by javaextract.go | yes |\n",
		"key served by none":  head + "| Java | {{dispatch .nosuch}} | yes |\n",
		"placeholder not 1st": head + "| Java | pure Go {{dispatch .java}} | yes |\n",
	} {
		if errs := CheckDispatchColumn(tmpl, table); len(errs) == 0 {
			t.Errorf("%s passed the dispatch check", name)
		}
	}
	if errs := CheckDispatchColumn(head+"| Java | {{dispatch .java}}; grammar pin v1 | yes |\n", table); len(errs) != 0 {
		t.Errorf("a clean cell failed: %v", errs)
	}
}

// TestStaleDispatchProseIsGone (LB-F358): no tracker text calls a dialect-served
// language "heuristic ... dispatched".
func TestStaleDispatchProseIsGone(t *testing.T) {
	re := regexp.MustCompile("heuristic \\(.(javaextract|cppextract|phpextract)\\.go.\\) dispatched")
	for _, p := range []string{"languages.tmpl.md", "../../docs/languages.md"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if re.Match(b) {
			t.Errorf("%s still types a stale dispatch fact", p)
		}
	}
}
