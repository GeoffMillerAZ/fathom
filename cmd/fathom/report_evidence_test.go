package main

// report_evidence_test.go — CV-39 (ADR-044 §5): "fathom report evidence"
// gives, per service, typed share of call edges, certificate density,
// inventory item counts by kind, and whether receipts re-derive — and it
// ranks NO languages: neither the services themselves nor the languages
// listed within one service are ever reordered by how good their edge
// tier is.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedReportEvidenceStore writes two services with no corpus root recorded
// (ReceiptsRederive degrades honestly to "unproven" rather than shelling
// out to git) — one Go-only service with a fully resolved typed edge (the
// "best" tier), one Python-only service with no typed edge at all (the
// "worst" tier) — but names them so that ALPHABETICAL id order is the
// OPPOSITE of "best tier first": a ranking-by-merit renderer would put
// zzz-go-svc first; the id-ordered, never-ranked renderer must not.
func seedReportEvidenceStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	// aaa-py-svc: Python only, heuristic tier (worst), but alphabetically FIRST.
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "aaa-py-svc", Package: "pkg", Symbol: "handle", SourceKind: core.KindPyFunc,
		SourceRef: "pkg/handle.py", Claim: "def handle():",
	}}); err != nil {
		t.Fatalf("PutObservations aaa-py-svc: %v", err)
	}

	// zzz-go-svc: Go, with one fully resolved typed edge (best tier), but
	// alphabetically LAST.
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "zzz-go-svc", Package: "widgets", Symbol: "Caller", SourceKind: core.KindGoFunc,
			SourceRef: "widgets/w.go", Claim: "func Caller()"},
		{ServiceID: "zzz-go-svc", Package: "widgets", Symbol: "Callee", SourceKind: core.KindGoFunc,
			SourceRef: "widgets/w.go", Claim: "func Callee()"},
	}); err != nil {
		t.Fatalf("PutObservations zzz-go-svc: %v", err)
	}
	if err := st.PutTypedEdges(ctx, "zzz-go-svc", "widgets", []core.Edge{
		{FromService: "zzz-go-svc", FromFile: "widgets/w.go", FromSymbol: "zzz-go-svc::widgets.Caller", ToSymbol: "zzz-go-svc::widgets.Callee", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}
	return path
}

// TestCmdReportEvidence_NoRanking pins CV-39's own kill criterion: services
// are listed in the store's own (alphabetical) id order, and languages
// within one service are listed alphabetically — NEVER reordered so the
// service or language with the best edge tier sorts first.
func TestCmdReportEvidence_NoRanking(t *testing.T) {
	storePath := seedReportEvidenceStore(t)

	var out, errBuf bytes.Buffer
	code := run([]string{"report", "evidence", "--store", storePath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errBuf.String())
	}

	var rep struct {
		Schema   string `json:"schema"`
		Services []struct {
			Service   string `json:"service"`
			Languages []struct {
				Language string `json:"language"`
				Tier     string `json:"tier"`
			} `json:"languages"`
		} `json:"services"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\nstdout=%s", err, out.String())
	}
	if rep.Schema == "" {
		t.Errorf("expected a non-empty schema field")
	}
	if len(rep.Services) != 2 {
		t.Fatalf("services = %v, want exactly 2", rep.Services)
	}
	// Never ranked by merit: id order (aaa-py-svc, the WORSE heuristic
	// tier) comes first, zzz-go-svc (the BETTER typed tier) comes last —
	// the plain alphabetical order ListServiceIDs already returns.
	if rep.Services[0].Service != "aaa-py-svc" || rep.Services[1].Service != "zzz-go-svc" {
		t.Errorf("services = [%s, %s], want [aaa-py-svc, zzz-go-svc] (id order, never tier order)",
			rep.Services[0].Service, rep.Services[1].Service)
	}

	// Within zzz-go-svc, only one language is touched (Go) so ordering is
	// trivially alphabetical; assert the tier itself is reported honestly
	// (typed) without being used to reorder anything.
	var zzz = rep.Services[1]
	if len(zzz.Languages) != 1 || zzz.Languages[0].Language != "Go" || zzz.Languages[0].Tier != "typed" {
		t.Errorf("zzz-go-svc languages = %+v, want [{Go typed}]", zzz.Languages)
	}
	var aaa = rep.Services[0]
	if len(aaa.Languages) != 1 || aaa.Languages[0].Language != "Python" {
		t.Errorf("aaa-py-svc languages = %+v, want a single Python entry", aaa.Languages)
	}
}

// TestCmdReportEvidence_TableRenders is a light smoke test of the
// non-JSON table path (the JSON test above already pins the schema).
func TestCmdReportEvidence_TableRenders(t *testing.T) {
	storePath := seedReportEvidenceStore(t)

	var out, errBuf bytes.Buffer
	code := run([]string{"report", "evidence", "--service", "zzz-go-svc", "--store", storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if out.Len() == 0 {
		t.Fatalf("expected non-empty table output")
	}
}

// TestCmdReportEvidence_NoStoreRefuses: with no --store and no store at the
// resolved default, the verb used to OPEN (and so create) an empty SQLite
// file there and print a header of measured-looking zeros. It must refuse,
// naming the store it looked for, and leave nothing behind.
func TestCmdReportEvidence_NoStoreRefuses(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	code, stdout, stderr := runInDir(t, repo.root, []string{"report", "evidence", "--json"})
	if code == 0 {
		t.Fatalf("exit 0 with no store; stdout=%s", stdout)
	}
	want := filepath.Join("workspace", "db", "fathom.sqlite")
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr %q does not name the store it looked for (%s)", stderr, want)
	}
	if strings.Contains(stdout, `"services"`) {
		t.Errorf("a refused run still printed a report: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(repo.root, want)); !os.IsNotExist(err) {
		t.Errorf("the refused run created %s (stat err=%v) — an empty store the next run would read as real", want, err)
	}
}

// TestCmdReportEvidence_DefaultStoreFromDaemonConfig: omitted, --store
// resolves the way the other config-aware readers do ([daemon].store,
// then the workspace default), and the report names the store it read.
func TestCmdReportEvidence_DefaultStoreFromDaemonConfig(t *testing.T) {
	storePath := seedReportEvidenceStore(t)
	repo := newSelfSkipGitRepo(t)
	repo.writePlan(".fathom/config.toml", "[daemon]\nstore = \""+storePath+"\"\n")
	code, stdout, stderr := runInDir(t, repo.root, []string{"report", "evidence", "--json"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, stderr)
	}
	var rep struct {
		Store    string `json:"store"`
		Services []struct {
			Service string `json:"service"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\nstdout=%s", err, stdout)
	}
	if rep.Store != storePath {
		t.Errorf("store = %q, want the [daemon].store path %q", rep.Store, storePath)
	}
	if len(rep.Services) != 2 {
		t.Errorf("services = %+v, want the two seeded services", rep.Services)
	}
}

// TestCmdReportEvidence_UnknownServiceRefuses: a --service the store has
// never indexed used to get a row of zeros indistinguishable from a
// measured service with no typed edges.
func TestCmdReportEvidence_UnknownServiceRefuses(t *testing.T) {
	storePath := seedReportEvidenceStore(t)
	var out, errBuf bytes.Buffer
	code := run([]string{"report", "evidence", "--service", "nope", "--store", storePath, "--json"}, &out, &errBuf)
	if code == 0 {
		t.Fatalf("exit 0 for a service the store never indexed; stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "nope") || !strings.Contains(errBuf.String(), storePath) {
		t.Errorf("stderr %q must name the service and the store", errBuf.String())
	}
}

// TestCmdReportEvidence_EmptyStoreRefuses: a store that exists but indexes
// no service at all (a stray file the old default created) is refused, not
// reported as an empty table.
func TestCmdReportEvidence_EmptyStoreRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	st.Close()
	var out, errBuf bytes.Buffer
	if code := run([]string{"report", "evidence", "--store", path}, &out, &errBuf); code == 0 {
		t.Fatalf("exit 0 for a store with no indexed service; stdout=%s", out.String())
	}
	if !strings.Contains(errBuf.String(), path) {
		t.Errorf("stderr %q must name the store", errBuf.String())
	}
}
