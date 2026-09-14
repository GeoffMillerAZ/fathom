package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── rankTestSelection (pure) ───────────────────────────────────────────────

func tsNode(name, file string) core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, EndLine: 5}
}

func TestRankTestSelection_ViaDispatchAlwaysLast(t *testing.T) {
	direct := tsNode("TestDirectUnit", "pkg/unit_test.go")
	dispatch := tsNode("TestDispatchUnit", "pkg/dispatch_test.go")
	changed := tsNode("Changed1", "pkg/a.go")

	affected := []impact.TestReach{
		{Test: dispatch.Key(), File: dispatch.FilePath, ViaDispatch: true, Reaches: []string{changed.Key()}},
		{Test: direct.Key(), File: direct.FilePath, ViaDispatch: false, Reaches: []string{changed.Key()}},
	}
	nodes := []core.SymbolNode{direct, dispatch, changed}

	ranked := rankTestSelection(nodes, affected, nil, false)
	if len(ranked) != 2 {
		t.Fatalf("len(ranked) = %d, want 2", len(ranked))
	}
	if ranked[0].Test != direct.Key() || ranked[1].Test != dispatch.Key() {
		t.Fatalf("order = %v, %v; want direct reach before a dispatch-mediated one", ranked[0].Test, ranked[1].Test)
	}
}

func TestRankTestSelection_CoverageOutranksLayer(t *testing.T) {
	unitTest := tsNode("TestUnit", "pkg/unit_test.go")
	intTest := tsNode("TestIntegration", "integration/int_test.go")
	changed1 := tsNode("Changed1", "pkg/a.go")
	changed2 := tsNode("Changed2", "pkg/b.go")

	affected := []impact.TestReach{
		{Test: unitTest.Key(), File: unitTest.FilePath, Reaches: []string{changed1.Key()}},
		{Test: intTest.Key(), File: intTest.FilePath, Reaches: []string{changed1.Key(), changed2.Key()}},
	}
	nodes := []core.SymbolNode{unitTest, intTest, changed1, changed2}
	covSyms := []coverage.SymbolLayerCoverage{
		{Symbol: changed1.Key(), Any: coverage.LayerStat{Percent: 20}},
		{Symbol: changed2.Key(), Any: coverage.LayerStat{Percent: 95}},
	}
	// unit reaches only changed1 (mean 20); integration reaches changed1+changed2
	// (mean 57.5) — with coverage data, the higher-corroborated integration
	// test must outrank the unit test despite its more expensive layer.
	ranked := rankTestSelection(nodes, affected, covSyms, true)
	if ranked[0].Test != intTest.Key() {
		t.Fatalf("with coverage supplied, want the higher-coverage integration test first, got order %v", []string{ranked[0].Test, ranked[1].Test})
	}

	// Without coverage data, layer order (unit before integration) governs.
	ranked = rankTestSelection(nodes, affected, nil, false)
	if ranked[0].Test != unitTest.Key() {
		t.Fatalf("with no coverage supplied, want unit before integration, got order %v", []string{ranked[0].Test, ranked[1].Test})
	}
}

func TestRankTestSelection_BreadthTiebreak(t *testing.T) {
	narrow := tsNode("TestNarrow", "pkg/a_test.go")
	broad := tsNode("TestBroad", "pkg/b_test.go")
	c1 := tsNode("C1", "pkg/a.go")
	c2 := tsNode("C2", "pkg/b.go")

	affected := []impact.TestReach{
		{Test: narrow.Key(), File: narrow.FilePath, Reaches: []string{c1.Key()}},
		{Test: broad.Key(), File: broad.FilePath, Reaches: []string{c1.Key(), c2.Key()}},
	}
	nodes := []core.SymbolNode{narrow, broad, c1, c2}
	ranked := rankTestSelection(nodes, affected, nil, false)
	if ranked[0].Test != broad.Key() {
		t.Fatalf("want the broader-reach test first when layer/coverage tie, got %v", []string{ranked[0].Test, ranked[1].Test})
	}
}

func TestRankTestSelection_NameTiebreak(t *testing.T) {
	a := tsNode("TestAlpha", "pkg/a_test.go")
	b := tsNode("TestBeta", "pkg/a_test.go")
	c1 := tsNode("C1", "pkg/a.go")

	affected := []impact.TestReach{
		{Test: b.Key(), File: b.FilePath, Reaches: []string{c1.Key()}},
		{Test: a.Key(), File: a.FilePath, Reaches: []string{c1.Key()}},
	}
	nodes := []core.SymbolNode{a, b, c1}
	ranked := rankTestSelection(nodes, affected, nil, false)
	if ranked[0].Test != a.Key() || ranked[1].Test != b.Key() {
		t.Fatalf("want alphabetical tiebreak, got %v", []string{ranked[0].Test, ranked[1].Test})
	}
}

// ─── groupTestInvocations ───────────────────────────────────────────────────

func TestGroupTestInvocations_GroupsAndSortsPerPackage(t *testing.T) {
	tb := tsNode("TestB", "pkg/b_test.go")
	ta := tsNode("TestA", "pkg/a_test.go")
	nodes := []core.SymbolNode{ta, tb}
	ranked := []rankedTest{
		{Test: tb.Key(), Package: "pkg"},
		{Test: ta.Key(), Package: "pkg"},
	}
	invs := groupTestInvocations(nodes, ranked)
	if len(invs) != 1 {
		t.Fatalf("len(invs) = %d, want 1 (one package)", len(invs))
	}
	if len(invs[0].Tests) != 2 || invs[0].Tests[0] != "TestA" || invs[0].Tests[1] != "TestB" {
		t.Fatalf("Tests = %v, want alphabetically sorted [TestA TestB]", invs[0].Tests)
	}
	want := "go test ./pkg -run '^(TestA|TestB)$'"
	if invs[0].Command != want {
		t.Errorf("Command = %q, want %q", invs[0].Command, want)
	}
}

// ─── loadTestSelectReplay ───────────────────────────────────────────────────

func TestLoadTestSelectReplay_MissingFile(t *testing.T) {
	stats := loadTestSelectReplay(filepath.Join(t.TempDir(), "nope.jsonl"))
	if stats.Total != 0 {
		t.Fatalf("expected Total=0 for a missing ledger, got %+v", stats)
	}
}

func TestLoadTestSelectReplay_SkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	body := `{"commit":"a","selected":3,"total":10,"caught":true}
not json at all
{"commit":"b","selected":2,"total":10,"caught":false}

`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stats := loadTestSelectReplay(path)
	if stats.Total != 2 || stats.Caught != 1 {
		t.Fatalf("stats = %+v, want {Caught:1 Total:2}", stats)
	}
	if got := stats.rate(); got != 0.5 {
		t.Errorf("rate() = %v, want 0.5", got)
	}
}

// ─── golden CLI test: fauxtown payment-service + a scripted diff ───────────

// buildTestsSelectFixture copies the real fauxtown/payment-service source
// (payment.go, errors.go) into a fresh temp dir and adds a payment_test.go
// exercising both Charge (which calls validateCharge) and validateCharge
// directly, so `fathom tests select` has real Test* functions with a real,
// syntax-derived call graph to rank — the fixture package itself carries no
// _test.go files (review's own golden test relies on that fact for its
// no-typed-edges assertion), so the test file is added here rather than
// mutating the checked-in fixture.
func buildTestsSelectFixture(t *testing.T) (storePath, service string) {
	t.Helper()
	src := fauxPaymentPathForTest(t)
	root := t.TempDir()
	for _, name := range []string{"payment.go", "errors.go"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	testSrc := `package payment

import (
	"context"
	"testing"
)

func TestChargeHappyPath(t *testing.T) {
	_, _ = Charge(context.Background(), ChargeRequest{AccountID: "a1", Amount: 5})
}

func TestValidateChargeDirect(t *testing.T) {
	_ = validateCharge(ChargeRequest{AccountID: "a1", Amount: 5})
}
`
	if err := os.WriteFile(filepath.Join(root, "payment_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write payment_test.go: %v", err)
	}

	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	service = "tests-select-svc"
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath, service
}

func TestTestsSelect_FauxtownGolden(t *testing.T) {
	storePath, service := buildTestsSelectFixture(t)

	runOnce := func() []byte {
		var out, errBuf bytes.Buffer
		code := runTestsSelect(
			[]string{"--since", "HEAD~1", "--service", service, "--store", storePath, "--json"},
			fakeGitRunner([]string{"payment.go"}), &out, &errBuf,
		)
		if code != 0 {
			t.Fatalf("tests select exit = %d, stderr: %s", code, errBuf.String())
		}
		return out.Bytes()
	}

	first := runOnce()
	second := runOnce()
	if !bytes.Equal(first, second) {
		t.Fatalf("tests select is not deterministic across identical runs:\n%s\nvs\n%s", first, second)
	}

	var payload map[string]any
	if err := json.Unmarshal(first, &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", first, err)
	}

	ranked, _ := payload["ranked_tests"].([]any)
	if len(ranked) != 2 {
		t.Fatalf("ranked_tests = %+v, want exactly 2 (TestChargeHappyPath, TestValidateChargeDirect)", payload["ranked_tests"])
	}
	top := ranked[0].(map[string]any)
	if got, _ := top["test"].(string); !strings.Contains(got, "TestChargeHappyPath") {
		t.Errorf("ranked_tests[0].test = %v, want TestChargeHappyPath first (it reaches both Charge and validateCharge, the broader reach)", top["test"])
	}
	topReaches, _ := top["reaches"].([]any)
	if len(topReaches) != 2 {
		t.Errorf("ranked_tests[0].reaches = %+v, want 2 (Charge, validateCharge)", top["reaches"])
	}
	second2 := ranked[1].(map[string]any)
	if got, _ := second2["test"].(string); !strings.Contains(got, "TestValidateChargeDirect") {
		t.Errorf("ranked_tests[1].test = %v, want TestValidateChargeDirect", second2["test"])
	}
	if layer, _ := top["layer"].(string); layer != "unit" {
		t.Errorf("layer = %q, want unit (bare _test.go, no integration/e2e marker)", layer)
	}
	if _, hasDispatch := top["via_dispatch"]; hasDispatch {
		t.Errorf("expected no via_dispatch (this fixture carries no typed edges): %+v", top)
	}

	invs, _ := payload["invocations"].([]any)
	if len(invs) != 1 {
		t.Fatalf("invocations = %+v, want exactly 1 (single package)", payload["invocations"])
	}
	inv := invs[0].(map[string]any)
	cmd, _ := inv["command"].(string)
	if !strings.Contains(cmd, "TestChargeHappyPath") || !strings.Contains(cmd, "TestValidateChargeDirect") || !strings.Contains(cmd, "-run") {
		t.Errorf("invocation command = %q, want both tests named in a -run regex", cmd)
	}

	if _, ok := payload["test_gap"].([]any); !ok {
		t.Errorf("expected a test_gap array in the payload: %+v", payload)
	}
	if _, hasReplay := payload["replay"]; hasReplay {
		t.Errorf("expected no replay block (no ledger file exists for this fixture): %+v", payload)
	}
}

func TestTestsSelect_RequiresSince(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := runTestsSelect([]string{"--service", "svc"}, fakeGitRunner(nil), &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --since is missing")
	}
}
