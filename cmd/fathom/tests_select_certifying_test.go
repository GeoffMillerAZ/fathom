package main

// tests_select_certifying_test.go — TQ-15w, TQ-16w, SR-50 (docs/design/
// test-intelligence.md §3 T4): `fathom tests select` selects the minimal set
// of tests that re-proves every touched certificate, `--safe` adds every test
// that statically reaches a changed symbol, and the CI time the selection
// saves is read from attested run durations, reported and recorded.

import (
	"bytes"
	"context"
	"encoding/json"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// csNode is a function node whose Calls name other nodes by key.
func csNode(file, name string, calls ...string) core.SymbolNode {
	return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, EndLine: 5, Calls: calls}
}

// csClaim is one live certificate: test (by key and bare name) certifies
// subject, backed by scope's files.
func csClaim(t *testing.T, id, subject string, test core.SymbolNode, state string, scope ...string) core.SemanticClaim {
	t.Helper()
	meta, err := json.Marshal(certify.Meta{Test: test.Name, TestSymbolKey: test.Key(), Strength: "asserts", State: state})
	if err != nil {
		t.Fatal(err)
	}
	return core.SemanticClaim{ClaimID: id, ServiceID: "svc", Layer: certify.Layer, Subject: subject, Scope: scope, Meta: meta}
}

// oracleMinCover is the coverage oracle: the smallest number of candidate
// tests whose certified targets cover every target in want, found by trying
// every subset (the fixtures below are small), or -1 when no subset covers.
func oracleMinCover(certifies map[string][]string, want []string) int {
	tests := make([]string, 0, len(certifies))
	for t := range certifies {
		tests = append(tests, t)
	}
	sort.Strings(tests)
	best := -1
	for mask := 0; mask < 1<<len(tests); mask++ {
		covered := map[string]bool{}
		for i, t := range tests {
			if mask&(1<<i) != 0 {
				for _, tgt := range certifies[t] {
					covered[tgt] = true
				}
			}
		}
		ok := true
		for _, w := range want {
			if !covered[w] {
				ok = false
				break
			}
		}
		if n := bits.OnesCount(uint(mask)); ok && (best < 0 || n < best) {
			best = n
		}
	}
	return best
}

// TestTestsSelectCertifyingFirst is SR-50: over a fixture with certificates,
// the selection re-proves every touched certificate with as few tests as the
// oracle's minimum, names what it skipped, and leaves untouched certificates
// (and the tests that only certify them) out.
func TestTestsSelectCertifyingFirst(t *testing.T) {
	a := csNode("pkg/a.go", "A")
	b := csNode("pkg/b.go", "B")
	c := csNode("pkg/c.go", "C")
	testAB := csNode("pkg/x_test.go", "TestAB", a.Key(), b.Key())
	testA := csNode("pkg/x_test.go", "TestA", a.Key())
	testC := csNode("pkg/x_test.go", "TestC", c.Key())
	testNone := csNode("pkg/x_test.go", "TestNone")
	nodes := []core.SymbolNode{a, b, c, testAB, testA, testC, testNone}
	claims := []core.SemanticClaim{
		csClaim(t, "1", "spec:ab1", testAB, "live", "pkg/a.go"),
		csClaim(t, "2", "spec:ab2", testAB, "live", "pkg/b.go"),
		csClaim(t, "3", "spec:ab1", testA, "live", "pkg/a.go"),
		csClaim(t, "4", "spec:c", testC, "live", "pkg/c.go"),
		csClaim(t, "5", "spec:ab2", testNone, "unproven", "pkg/b.go"), // not live: never a cover candidate
	}
	changed := []string{"pkg/a.go", "pkg/b.go"}

	sel := selectCertifyingFirst(nodes, nil, claims, changed, nil, false)
	if want := []string{"spec:ab1", "spec:ab2"}; !reflect.DeepEqual(sel.TouchedTargets, want) {
		t.Fatalf("TouchedTargets = %v, want %v (spec:c is not touched by a diff to a.go and b.go)", sel.TouchedTargets, want)
	}
	if want := []string{testAB.Key()}; !reflect.DeepEqual(sel.Tests, want) {
		t.Fatalf("Tests = %v, want only %v", sel.Tests, want)
	}
	// Skipped is every certifying test the diff does not need: TestA (its
	// certificate is already re-proved by TestAB) and TestC (its certificate
	// is untouched).
	if want := []string{testA.Key(), testC.Key()}; !reflect.DeepEqual(sel.Skipped, want) {
		t.Fatalf("Skipped = %v, want %v", sel.Skipped, want)
	}
	if len(sel.Uncovered) != 0 {
		t.Fatalf("Uncovered = %v, want none", sel.Uncovered)
	}

	// The coverage oracle: every touched target is certified by a selected
	// test, and the selection is as small as the best possible cover.
	certifies := map[string][]string{
		testAB.Key(): {"spec:ab1", "spec:ab2"},
		testA.Key():  {"spec:ab1"},
		testC.Key():  {"spec:c"},
	}
	covered := map[string]bool{}
	for _, tst := range sel.Tests {
		for _, tgt := range certifies[tst] {
			covered[tgt] = true
		}
	}
	for _, tgt := range sel.TouchedTargets {
		if !covered[tgt] {
			t.Errorf("touched target %s is re-proved by no selected test", tgt)
		}
	}
	if min := oracleMinCover(certifies, sel.TouchedTargets); len(sel.Tests) != min {
		t.Errorf("selected %d tests, the oracle's minimum cover is %d", len(sel.Tests), min)
	}

	// A change that touches nothing certified selects nothing (without --safe).
	if none := selectCertifyingFirst(nodes, nil, claims, []string{"docs/readme.md"}, nil, false); len(none.Tests) != 0 || len(none.TouchedTargets) != 0 {
		t.Errorf("a diff touching no certificate selected %+v", none)
	}
}

// TestTestsSelectCertifyingFirstSafeFlag is TQ-15w: `tests select` prints the
// certifying-first minimal set, and --safe adds every test that statically
// reaches a changed symbol, whether or not it certifies anything.
func TestTestsSelectCertifyingFirstSafeFlag(t *testing.T) {
	storePath, service, happy, direct := buildCertifyingFixture(t, false)
	setEconomicsLedger(t, filepath.Join(t.TempDir(), "econ.jsonl"))
	runSel := func(extra ...string) map[string]any {
		var out, errBuf bytes.Buffer
		args := append([]string{"--since", "HEAD~1", "--service", service, "--store", storePath, "--json"}, extra...)
		if code := runTestsSelect(args, fakeGitRunner([]string{"payment.go"}), &out, &errBuf); code != 0 {
			t.Fatalf("tests select exit = %d: %s", code, errBuf.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out.String())
		}
		return payload["selection"].(map[string]any)
	}
	keysOf := func(v any) []string {
		var out []string
		for _, x := range v.([]any) {
			out = append(out, x.(string))
		}
		return out
	}

	plain := runSel()
	if got, want := keysOf(plain["tests"]), []string{happy}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection.tests = %v, want the minimal certifying set %v", got, want)
	}
	if got, want := keysOf(plain["skipped"]), []string{direct}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection.skipped = %v, want %v", got, want)
	}
	if plain["safe"] != false {
		t.Errorf("selection.safe = %v, want false", plain["safe"])
	}
	invs := plain["invocations"].([]any)
	if len(invs) != 1 || !strings.Contains(invs[0].(map[string]any)["command"].(string), "TestChargeHappyPath") ||
		strings.Contains(invs[0].(map[string]any)["command"].(string), "TestValidateChargeDirect") {
		t.Errorf("selection.invocations = %v, want one go test command naming only the selected test", invs)
	}

	safe := runSel("--safe")
	if got := keysOf(safe["tests"]); len(got) != 2 {
		t.Fatalf("--safe selection.tests = %v, want both reaching tests", got)
	}
	if safe["safe"] != true {
		t.Errorf("--safe selection.safe = %v, want true", safe["safe"])
	}

	// The text form leads with the certifying-first set.
	var out, errBuf bytes.Buffer
	if code := runTestsSelect([]string{"--since", "HEAD~1", "--service", service, "--store", storePath},
		fakeGitRunner([]string{"payment.go"}), &out, &errBuf); code != 0 {
		t.Fatalf("text exit = %d: %s", code, errBuf.String())
	}
	text := out.String()
	if i, j := strings.Index(text, "certifying-first"), strings.Index(text, "ranked"); i < 0 || j < 0 || i > j {
		t.Errorf("text output must lead with the certifying-first selection, before the reach ranking:\n%s", text)
	}
}

// TestTestsSelectRecordsTimeSaved is TQ-16w: the selection's CI time saved is
// summed from each test's last attested run duration, printed, and appended
// to the economics ledger; a selection with no attested duration on record
// says so instead of reporting zero.
func TestTestsSelectRecordsTimeSaved(t *testing.T) {
	storePath, service, _, _ := buildCertifyingFixture(t, true)
	ledger := filepath.Join(t.TempDir(), "econ.jsonl")
	setEconomicsLedger(t, ledger)

	var out, errBuf bytes.Buffer
	if code := runTestsSelect([]string{"--since", "HEAD~1", "--service", service, "--store", storePath, "--json"},
		fakeGitRunner([]string{"payment.go"}), &out, &errBuf); code != 0 {
		t.Fatalf("tests select exit = %d: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	saved := payload["selection"].(map[string]any)["time_saved"].(map[string]any)
	if saved["known"] != true || saved["selected_ms"] != float64(200) || saved["skipped_ms"] != float64(100) || saved["total_ms"] != float64(300) {
		t.Fatalf("time_saved = %v, want 200 selected + 100 skipped ms from the attested durations", saved)
	}
	if pct, _ := saved["percent"].(float64); pct < 33.3 || pct > 33.4 {
		t.Errorf("time_saved.percent = %v, want 33.3", saved["percent"])
	}

	raw, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("the economics ledger was not written: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("ledger has %d lines, want one row for one run: %s", len(lines), raw)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("ledger row is not JSON: %v: %s", err, lines[0])
	}
	if row["service"] != service || row["since"] != "HEAD~1" || row["skipped_ms"] != float64(100) || row["selected_ms"] != float64(200) || row["safe"] != false {
		t.Errorf("ledger row = %v", row)
	}

	// The text form reports it.
	out.Reset()
	if code := runTestsSelect([]string{"--since", "HEAD~1", "--service", service, "--store", storePath},
		fakeGitRunner([]string{"payment.go"}), &out, &errBuf); code != 0 {
		t.Fatalf("text exit = %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "time saved: 33%") {
		t.Errorf("text output lacks the time saved line:\n%s", out.String())
	}

	// No attested durations: unknown, named, and nothing recorded.
	storePath2, service2, _, _ := buildCertifyingFixture(t, false)
	ledger2 := filepath.Join(t.TempDir(), "econ2.jsonl")
	setEconomicsLedger(t, ledger2)
	out.Reset()
	if code := runTestsSelect([]string{"--since", "HEAD~1", "--service", service2, "--store", storePath2},
		fakeGitRunner([]string{"payment.go"}), &out, &errBuf); code != 0 {
		t.Fatalf("no-duration exit = %d: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "time saved: unknown") {
		t.Errorf("a selection with no attested duration must say the time saved is unknown:\n%s", out.String())
	}
	if _, err := os.Stat(ledger2); err == nil {
		t.Error("nothing is known, so nothing may be recorded for economics")
	}
}

// setEconomicsLedger points the economics ledger at path for one test.
func setEconomicsLedger(t *testing.T, path string) {
	t.Helper()
	prev := testsSelectEconomicsPath
	testsSelectEconomicsPath = func() string { return path }
	t.Cleanup(func() { testsSelectEconomicsPath = prev })
}

// buildCertifyingFixture ingests the payment fixture and seeds live
// certificates: TestChargeHappyPath certifies spec:charge and invariant:I2,
// TestValidateChargeDirect certifies spec:charge too, all backed by
// payment.go. With durations, both tests also have an attested run: 200 ms
// and 100 ms.
func buildCertifyingFixture(t *testing.T, durations bool) (storePath, service, happyKey, directKey string) {
	t.Helper()
	storePath, service = buildTestsSelectFixture(t)
	ctx := context.Background()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	defer st.Close()
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		switch n.Name {
		case "TestChargeHappyPath":
			happyKey = n.Key()
		case "TestValidateChargeDirect":
			directKey = n.Key()
		}
	}
	if happyKey == "" || directKey == "" {
		t.Fatalf("fixture tests not indexed: happy=%q direct=%q", happyKey, directKey)
	}
	mk := func(id, subject, testName, testKey string) core.SemanticClaim {
		meta, _ := json.Marshal(certify.Meta{Test: testName, TestSymbolKey: testKey, Strength: "asserts", State: "live"})
		return core.SemanticClaim{ClaimID: id, ServiceID: service, Layer: certify.Layer, Kind: certify.Kind, Subject: subject,
			Rule: "r", Scope: []string{"payment.go"}, Status: core.ClaimFresh, Meta: meta}
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{
		mk("c1", "spec:charge", "TestChargeHappyPath", happyKey),
		mk("c2", "invariant:I2", "TestChargeHappyPath", happyKey),
		mk("c3", "spec:charge", "TestValidateChargeDirect", directKey),
	}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	if durations {
		if err := st.PutTestRuns(ctx, service, []store.TestRun{{
			RunDigest: "run-1", Component: "payment", Suite: "unit", StartedAt: "2026-09-28T00:00:00Z", Verdict: "pass",
			Cases: []store.TestCase{
				{CaseID: "1", Name: "TestChargeHappyPath", Status: "pass", Attempts: 1, DurationMS: 200, TestSymbolKey: happyKey},
				{CaseID: "2", Name: "TestValidateChargeDirect", Status: "pass", Attempts: 1, DurationMS: 100, TestSymbolKey: directKey},
			},
		}}); err != nil {
			t.Fatalf("PutTestRuns: %v", err)
		}
	}
	return storePath, service, happyKey, directKey
}
