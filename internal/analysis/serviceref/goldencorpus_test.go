package serviceref

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

// This file is VB-31's golden corpus test (docs/plans/verbs.md,
// docs/bench/readings/vb31-service-ref-m8.md): it loads the hand-verified
// route table written BEFORE ServiceRefEdges was ever run in that session
// (testdata/serviceref/route-table.json, written_before_extractor_run:true)
// and checks the extractor's actual output against it for the two corpora
// that live in THIS repo (fauxtown, and the reachfix false-positive
// control). The third corpus in that table, "real_pair" (openfaas/faas <->
// openfaas/faas-provider, cloned into a scratch directory, never
// committed), cannot be gated here — see the report for its numbers,
// produced by a throwaway workspace/ program over the same extractor
// functions.
//
// Per VB-31 step 3: this test reports precision/recall (t.Logf) rather
// than failing the gate on a threshold — the plan row's checkbox is the
// place that carries the verdict, not this test. It DOES hard-fail on
// structural problems (route table missing/malformed, or the
// false-positive control producing a spurious hop), which are correctness
// bugs regardless of any threshold.

// routeTableRow mirrors one row of testdata/serviceref/route-table.json.
// See that file's $schema_note for the "kind" vocabulary.
type routeTableRow struct {
	Corpus        string  `json:"corpus"`
	Kind          string  `json:"kind"`
	ClientRepo    *string `json:"client_repo"`
	Method        *string `json:"method"`
	RouteTemplate *string `json:"route_template"`
	ServerRepo    *string `json:"server_repo"`
}

type routeTable struct {
	WrittenBeforeExtractorRun bool            `json:"written_before_extractor_run"`
	Rows                      []routeTableRow `json:"rows"`
}

func loadRouteTable(t *testing.T) routeTable {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// The hand-verified table lives at the REPO ROOT's testdata/serviceref/
	// (VB-31 task instruction), not this package's own testdata/ — three
	// levels up from internal/analysis/serviceref.
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "serviceref", "route-table.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read route table %s: %v", path, err)
	}
	var rt routeTable
	if err := json.Unmarshal(data, &rt); err != nil {
		t.Fatalf("parse route table %s: %v", path, err)
	}
	if !rt.WrittenBeforeExtractorRun {
		t.Fatalf("route table %s does not declare written_before_extractor_run:true (VB-31 ordering requirement)", path)
	}
	if len(rt.Rows) == 0 {
		t.Fatalf("route table %s has no rows", path)
	}
	return rt
}

// hopKey is the identity used to compare a Hop against a hand-verified
// "match" row: (fromService, method, template, toService). Ambiguity
// metadata is deliberately excluded, same as sortHops's own rationale.
func hopKey(fromService, method, template, toService string) string {
	return fromService + " " + method + " " + template + " -> " + toService
}

// precisionRecall compares hops against the "match" rows of truth rows
// filtered to corpus. Returns TP/FP/FN counts plus the FP and FN key lists
// (sorted, for deterministic t.Logf output).
func precisionRecall(hops []Hop, rows []routeTableRow, corpus string) (tp, fp, fn int, fpKeys, fnKeys []string) {
	truth := map[string]bool{}
	for _, r := range rows {
		if r.Corpus != corpus || r.Kind != "match" {
			continue
		}
		truth[hopKey(*r.ClientRepo, *r.Method, *r.RouteTemplate, *r.ServerRepo)] = true
	}

	found := map[string]bool{}
	for _, h := range hops {
		k := hopKey(h.FromService, h.Method, h.Template, h.ToService)
		found[k] = true
		if truth[k] {
			tp++
		} else {
			fp++
			fpKeys = append(fpKeys, k)
		}
	}
	for k := range truth {
		if !found[k] {
			fn++
			fnKeys = append(fnKeys, k)
		}
	}
	sort.Strings(fpKeys)
	sort.Strings(fnKeys)
	return tp, fp, fn, fpKeys, fnKeys
}

func logPR(t *testing.T, corpus string, tp, fp, fn int, fpKeys, fnKeys []string) {
	t.Helper()
	precision := 1.0
	if tp+fp > 0 {
		precision = float64(tp) / float64(tp+fp)
	}
	recall := 1.0
	if tp+fn > 0 {
		recall = float64(tp) / float64(tp+fn)
	}
	t.Logf("corpus=%s tp=%d fp=%d fn=%d precision=%.4f recall=%.4f", corpus, tp, fp, fn, precision, recall)
	for _, k := range fpKeys {
		t.Logf("corpus=%s FP: %s", corpus, k)
	}
	for _, k := range fnKeys {
		t.Logf("corpus=%s FN: %s", corpus, k)
	}
}

// walkGoSources returns (routes, calls) extracted from every *.go file
// under dir (recursive), skipping dot-directories the way a real connector
// would (internal/connector.skipDir's ".claude" etc. rationale — kept
// local here to avoid importing internal/connector into this package's
// test dependency graph, matching fauxtownRoot's own stated reason above
// it in serviceref_test.go).
func walkGoSources(t *testing.T, dir, service string) ([]routes.Route, []ClientCall) {
	t.Helper()
	var rts []routes.Route
	var calls []ClientCall
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rts = append(rts, routes.ExtractFile("go", rel, src).Routes...)
		calls = append(calls, ExtractClientCalls("go", rel, src)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	_ = service
	return rts, calls
}

// TestGoldenCorpus_Fauxtown joins EVERY fauxtown service (not just
// catalog-service/checkout-service, which is all TestServiceRefEdges_
// FixtureJoin above exercises) against the hand-verified route table's
// "fauxtown" corpus rows, and reports precision/recall. It also folds in
// reachfix (a single-service, client-call-free fixture from a different
// plan's testdata — see route-table.json's reachfix-fp-control corpus
// note) in the SAME join, as a false-positive control: reachfix must
// contribute zero hops whether joined alone or alongside fauxtown.
func TestGoldenCorpus_Fauxtown(t *testing.T) {
	rt := loadRouteTable(t)

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	fauxRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "connector", "testdata", "fauxtown")
	reachfixDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "fusion", "testdata", "reachfix")

	fauxServices := []string{"catalog-service", "checkout-service", "ledger-service", "payment-service", "polyglot-service", "worktree-service"}

	routesByService := map[string][]routes.Route{}
	callsByService := map[string][]ClientCall{}
	for _, svc := range fauxServices {
		rts, calls := walkGoSources(t, filepath.Join(fauxRoot, svc), svc)
		if len(rts) > 0 {
			routesByService[svc] = rts
		}
		if len(calls) > 0 {
			callsByService[svc] = calls
		}
	}

	// Fold in reachfix as its own "service" in the SAME join universe, to
	// prove it cannot contaminate fauxtown's result (it has no client
	// calls of its own — see reachfixRoutes below — so it can only ever
	// appear as ToService, never FromService).
	reachfixRoutes, reachfixCalls := walkGoSources(t, reachfixDir, "reachfix")
	if len(reachfixCalls) != 0 {
		t.Fatalf("reachfix fixture: expected zero client calls (it is a route-registration-only fixture), got %d: %+v", len(reachfixCalls), reachfixCalls)
	}
	if len(reachfixRoutes) > 0 {
		routesByService["reachfix"] = reachfixRoutes
	}

	hops := ServiceRefEdges(routesByService, callsByService)

	var reachfixHops []Hop
	var fauxtownHops []Hop
	for _, h := range hops {
		if h.FromService == "reachfix" || h.ToService == "reachfix" {
			reachfixHops = append(reachfixHops, h)
			continue
		}
		fauxtownHops = append(fauxtownHops, h)
	}
	if len(reachfixHops) != 0 {
		t.Fatalf("reachfix-fp-control: expected zero hops touching reachfix, got %d: %+v", len(reachfixHops), reachfixHops)
	}

	tp, fp, fn, fpKeys, fnKeys := precisionRecall(fauxtownHops, rt.Rows, "fauxtown")
	logPR(t, "fauxtown", tp, fp, fn, fpKeys, fnKeys)

	// fauxtown is fully in-repo and deterministic (no network, no external
	// clone) — a regression here is a real bug, not a threshold call, so
	// this corpus alone is hard-asserted zero-FP/zero-FN. This does NOT
	// gate on the plan row's 0.90 threshold (see file doc comment); it
	// gates on "this specific, always-reproducible corpus stayed
	// perfect", which existing tests (TestServiceRefEdges_FixtureJoin)
	// already imply.
	if fp != 0 || fn != 0 {
		t.Errorf("fauxtown corpus: want 0 FP/0 FN (this corpus is fully in-repo and deterministic), got fp=%d fn=%d", fp, fn)
	}
}
