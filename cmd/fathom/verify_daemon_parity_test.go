package main

// verify_daemon_parity_test.go — O1068: `fathom verify` through the daemon
// (the default --url) read `?` on every `gone:` row of
// docs/plans/retire-unused-analyzers.md while `--store <the same index>`
// proved all of them, so `fathom plans status` over the daemon counted 46
// claimed-unproven rows on fathom's own plans. The daemon's evidence source
// had no GoneEvidence at all, and its store-backed capabilities (runtime_pass
// reads attested runs through the store) were stripped by the auto-refresh
// wrapper every real daemon serves through. This test is the parity
// contract: one fixture plan carrying every evidence kind, verified through
// both paths over ONE index, must read the same per-row verdicts.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// parityPlan carries every evidence kind the verify engine evaluates: symbol
// presence (found and not), package filters, tier, test reach, typed
// reachable_from, product wiring, skip_test, spec coverage, certified_by,
// runtime_pass (an attested run is ingested below), the lexical oracle, every
// gone: form (pass, fail and the build-tag-excluded Unknown), and a refused
// contract.
const parityPlan = `# Parity

- [x] Kept is present and tested {id: P-1, symbol: Kept, package: keptpkg}
- [x] Kept is reachable from its caller {id: P-2, symbol: Kept, package: keptpkg, reachable_from: CallsKept}
- [x] A symbol nobody wrote {id: P-3, symbol: NeverWritten}
- [x] Drift is a waived fixture {id: P-4, symbol: Drift, package: driftpkg, skip_test: true, skip: "fixture package, kept unused on purpose"}
- [x] Kept is at the verified tier {id: P-5, symbol: Kept, package: keptpkg, min_tier: verified}
- [x] The README names the window {id: P-6, oracle: lexical, path: README.md, pattern: "^## Window$"}
- [x] driftpkg is imported nowhere {id: P-7, gone: import parityfixture/driftpkg}
- [x] keptpkg is imported nowhere {id: P-8, gone: import parityfixture/keptpkg}
- [x] No module requires the retired one {id: P-9, gone: module example.com/retired}
- [x] LegacyThing is spelled nowhere in code {id: P-10, gone: pattern "LegacyThing", paths: "keptpkg/,caller/,cmd/"}
- [x] Kept is declared at most once under keptpkg {id: P-11, gone: pattern "func Kept[(]", paths: "keptpkg/", max: 1}
- [x] Drift has no typed callers {id: P-12, gone: callers Drift, package: driftpkg}
- [x] Kept has no typed callers {id: P-13, gone: callers Kept, package: keptpkg}
- [x] Kept implements [FR-7] {id: P-14, symbol: Kept, package: keptpkg}
- [x] Kept is certified {id: P-15, symbol: Kept, package: keptpkg, certified_by: TestKept}
- [x] legacypkg is imported nowhere {id: P-16, gone: import parityfixture/legacypkg}
- [x] One contract per row {id: P-17, symbol: Kept, gone: import parityfixture/driftpkg}
`

func newParityFixtureRepo(t *testing.T) *selfSkipGitRepo {
	t.Helper()
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module parityfixture\n\ngo 1.21\n")
	repo.writePlan("README.md", "# Fixture\n\n## Window\n")
	repo.writePlan("keptpkg/kept.go", "package keptpkg\n\n// Kept implements FR-7.\nfunc Kept() int { return 1 }\n")
	repo.writePlan("keptpkg/kept_test.go", "package keptpkg\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {\n\tif Kept() != 1 {\n\t\tt.Fatal(\"Kept\")\n\t}\n}\n")
	// A build-tag-excluded importer: the typed pass never sees it, so a
	// gone: import of legacypkg is honestly Unknown on both paths.
	repo.writePlan("keptpkg/kept_plan9.go", "//go:build plan9\n\npackage keptpkg\n\nimport _ \"parityfixture/legacypkg\"\n")
	repo.writePlan("legacypkg/legacy.go", "package legacypkg\n\nfunc Legacy() {}\n")
	repo.writePlan("driftpkg/drift.go", "package driftpkg\n\nfunc Drift() {}\n")
	repo.writePlan("caller/user.go", "package caller\n\nimport \"parityfixture/keptpkg\"\n\nfunc CallsKept() int {\n\treturn keptpkg.Kept()\n}\n")
	repo.writePlan("cmd/app/main.go", "package main\n\nimport \"parityfixture/caller\"\n\nfunc main() {\n\t_ = caller.CallsKept()\n}\n")
	return repo
}

// parityRow is one row's verdict as both paths render it: the status, the
// label and every criterion's name and status. Details may legitimately
// differ in wording that is not a verdict (a run's age read against two
// clocks); the verdicts may not.
type parityRow struct {
	Status   string
	Label    string
	Criteria []string
}

func parityRows(t *testing.T, stdout string) map[string]parityRow {
	t.Helper()
	var doc struct {
		Reports []struct {
			Tasks []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Label    string `json:"label"`
				Criteria []struct {
					Name   string `json:"name"`
					Status string `json:"status"`
					Detail string `json:"detail"`
				} `json:"criteria"`
			} `json:"tasks"`
		} `json:"reports"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode verify --json: %v\n%s", err, stdout)
	}
	if len(doc.Reports) != 1 {
		t.Fatalf("want one report, got %d\n%s", len(doc.Reports), stdout)
	}
	out := map[string]parityRow{}
	for _, tk := range doc.Reports[0].Tasks {
		r := parityRow{Status: tk.Status, Label: tk.Label}
		for _, c := range tk.Criteria {
			r.Criteria = append(r.Criteria, c.Name+"="+c.Status)
		}
		sort.Strings(r.Criteria)
		out[tk.ID] = r
	}
	return out
}

// startParityDaemon serves the store the way `fathom serve-http` does: the
// client behind the auto-refresh wrapper (the daemon default) behind the
// real HTTP handler.
func startParityDaemon(t *testing.T, storePath string) string {
	t.Helper()
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	logger := testDiscardLogger()
	ctx, cancel := context.WithCancel(context.Background())
	q := mcp.NewRefreshQueueLogged(c, refresh.Config{}, logger)
	q.Start(ctx)
	t.Cleanup(func() { cancel(); q.Wait() })
	backend := mcp.NewAutoRefreshQueued(c, logger, q)
	srv := mcp.NewServerWithFleetAndLogger(backend, newFleetAdapter(c, logger), nil)
	ts := httptest.NewServer(newHTTPHandler(srv, c, "127.0.0.1:0", nil))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestVerifyDaemonParity_EveryEvidenceKind(t *testing.T) {
	repo := newParityFixtureRepo(t)
	pf := repo.writePlan("docs/plans/parity.md", parityPlan)
	repo.commitAll("parity fixture")
	storePath := filepath.Join(t.TempDir(), "parity.sqlite")
	if code, out, errOut := runInDir(t, repo.root, []string{"ingest", "--service", "parityfixture", "--path", repo.root, "--store", storePath}); code != 0 {
		t.Fatalf("ingest: exit %d\n%s%s", code, out, errOut)
	}
	// An attested run of TestKept, so runtime_pass has a run to read.
	started := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	run := `{"suite": {"component": "keptpkg", "name": "unit", "class": "unit"}, "key": "k-pass", "verdict": "pass",
  "cases": [{"id": "1", "name": "TestKept", "status": "pass", "duration": "1ms", "attempts": 1}],
  "totals": {"run": 1, "passed": 1, "failed": 0, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "` + started + `", "wallClock": "1ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "arm64", "os": "darwin", "commit": "0123456789abcdef0123456789abcdef01234567"},
  "artifacts": []}`
	runPath := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(runPath, []byte(run), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := runInDir(t, repo.root, []string{"tests", "ingest", "--service", "parityfixture", "--store", storePath, "--manifest", t.TempDir(), "--results", runPath}); code != 0 {
		t.Fatalf("tests ingest: exit %d\n%s%s", code, out, errOut)
	}

	verifyArgs := func(extra ...string) []string {
		return append([]string{"verify", "--service", "parityfixture", "--plan", pf, "--json", "--strict", "--ledger", filepath.Join(t.TempDir(), "l.jsonl")}, extra...)
	}
	storeCode, storeOut, storeErr := runInDir(t, repo.root, verifyArgs("--store", storePath))
	url := startParityDaemon(t, storePath)
	daemonCode, daemonOut, daemonErr := runInDir(t, repo.root, verifyArgs("--url", url))

	direct, daemon := parityRows(t, storeOut), parityRows(t, daemonOut)
	if len(direct) != 17 {
		t.Fatalf("direct path verified %d rows, want 17\nstdout=%s\nstderr=%s", len(direct), storeOut, storeErr)
	}
	ids := make([]string, 0, len(direct))
	for id := range direct {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d, ok := daemon[id]
		if !ok {
			t.Errorf("%s: the daemon path returned no verdict", id)
			continue
		}
		if d.Status != direct[id].Status || d.Label != direct[id].Label || strings.Join(d.Criteria, ",") != strings.Join(direct[id].Criteria, ",") {
			t.Errorf("%s: daemon %+v, direct store %+v", id, d, direct[id])
		}
	}
	if daemonCode != storeCode {
		t.Errorf("exit codes differ: daemon %d, direct store %d\ndaemon stderr=%s\nstore stderr=%s", daemonCode, storeCode, daemonErr, storeErr)
	}

	// Parity of two wrong answers proves nothing: the gone: rows must be
	// the verdicts the fixture makes true.
	want := map[string]string{"P-7": "pass", "P-8": "fail", "P-9": "pass", "P-10": "pass", "P-11": "pass", "P-12": "pass", "P-13": "fail", "P-16": "unknown", "P-17": "fail"}
	for id, status := range want {
		if got := daemon[id].Status; got != status {
			t.Errorf("%s through the daemon: %s (%+v), want %s", id, got, daemon[id], status)
		}
	}
	if c := strings.Join(daemon["P-1"].Criteria, ","); !strings.Contains(c, "runtime_pass=pass") {
		t.Errorf("P-1 through the daemon: runtime_pass did not read the attested run: %s", c)
	}

	// An uncommitted file in the plan's tree: the index no longer is that
	// tree, and both paths must say so rather than prove a gone: row.
	repo.writePlan("scratch.txt", "uncommitted\n")
	_, dirtyStore, _ := runInDir(t, repo.root, verifyArgs("--store", storePath))
	_, dirtyDaemon, _ := runInDir(t, repo.root, verifyArgs("--url", url))
	for _, id := range []string{"P-7", "P-9", "P-12"} {
		ds, dd := parityRows(t, dirtyStore)[id], parityRows(t, dirtyDaemon)[id]
		if ds.Status != "unknown" || dd.Status != "unknown" || ds.Label != dd.Label {
			t.Errorf("%s over a dirty tree: direct %+v, daemon %+v, want both unknown", id, ds, dd)
		}
	}
}
