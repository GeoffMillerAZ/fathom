package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// typedEdgesDaemon serves just enough MCP for checkTypedEdges: a
// staleness_report whose typed_edges block is whatever the test hands it
// (raw JSON, so "block absent" is expressible too).
func typedEdgesDaemon(t *testing.T, typedBlock string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name != "staleness_report" {
			http.Error(w, "unknown tool", 400)
			return
		}
		inner := `{"service":"svc","source_count":1`
		if typedBlock != "" {
			inner += `,"typed_edges":` + typedBlock
		}
		inner += `}`
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	return httptest.NewServer(mux)
}

func typedEdgesRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// BP-36: a daemon whose typed pass failed must turn this row RED and name
// the reason. This is the 2026-09-12 defect's only visible symptom outside
// a log file nobody tails — a broken daemon Go environment served
// syntax-only edges for an unknown number of days while every surface,
// doctor's 35 checks included, reported a healthy index.
func TestCheckTypedEdges_FailsWithReasonAndRemedy(t *testing.T) {
	srv := typedEdgesDaemon(t, `{"known":true,"packages":0,"failed":true,"reason":"fusion: typed-edge load: GOPATH entry is relative"}`)
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, typedEdgesRepo(t), "svc", true)
	if got.Name != "typed_edges" {
		t.Fatalf("row name = %q", got.Name)
	}
	if got.Status != doctorFail {
		t.Fatalf("status = %v, want FAIL — a typed pass that failed must not read healthy (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "GOPATH entry is relative") {
		t.Fatalf("the reason must be NAMED, not summarised (I24): %q", got.Reason)
	}
	if !strings.Contains(got.Remedy, "fathom daemon install") {
		t.Fatalf("remedy must point at the environment fix: %q", got.Remedy)
	}
}

func TestCheckTypedEdges_PassesWithPackageCount(t *testing.T) {
	srv := typedEdgesDaemon(t, `{"known":true,"packages":142,"failed":false}`)
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, typedEdgesRepo(t), "svc", true)
	if got.Status != doctorPass {
		t.Fatalf("status = %v, want PASS (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "142") {
		t.Fatalf("a passing row must carry the package count: %q", got.Reason)
	}
}

// `known:false` means this daemon process has not ingested the service —
// not that the pass succeeded. Reporting it as a pass would be the exact
// degrade-toward-healthy ADR-027 forbids.
func TestCheckTypedEdges_UnknownIsSkippedNotPassed(t *testing.T) {
	srv := typedEdgesDaemon(t, `{"known":false}`)
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, typedEdgesRepo(t), "svc", true)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP for an unknown outcome (%s)", got.Status, got.Reason)
	}
}

// A daemon too old to carry the block is also skipped-with-reason, never a
// silent pass.
func TestCheckTypedEdges_AbsentBlockIsSkipped(t *testing.T) {
	srv := typedEdgesDaemon(t, "")
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, typedEdgesRepo(t), "svc", true)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP when the daemon predates the block (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "predates") {
		t.Fatalf("reason must say why: %q", got.Reason)
	}
}

func TestCheckTypedEdges_NonGoRepoSkips(t *testing.T) {
	srv := typedEdgesDaemon(t, `{"known":true,"packages":1,"failed":false}`)
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, t.TempDir(), "svc", true)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP for a non-Go repo (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "go.mod") {
		t.Fatalf("reason must name the gate: %q", got.Reason)
	}
}

func TestCheckTypedEdges_DaemonDownSkips(t *testing.T) {
	got := checkTypedEdges(http.DefaultClient, "http://127.0.0.1:1", typedEdgesRepo(t), "svc", false)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP when the daemon is unreachable (%s)", got.Status, got.Reason)
	}
	_ = fmt.Sprint(got)
}

// typedEdgesDaemonMultiService serves list_services (each row optionally
// carrying its recorded "path") and staleness_report keyed by the ACTUAL
// requested service — svcBlocks maps service name to a typed_edges block
// (absent/"" means the daemon has not ingested that name).
func typedEdgesDaemonMultiService(t *testing.T, services []string, servicePaths, svcBlocks map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var inner string
		switch req.Params.Name {
		case "list_services":
			rows := make([]string, len(services))
			for i, s := range services {
				pathField := ""
				if p := servicePaths[s]; p != "" {
					pathField = fmt.Sprintf(`,"path":%q`, p)
				}
				rows[i] = fmt.Sprintf(`{"service":%q,"symbol_count":10,"languages":["go"]%s}`, s, pathField)
			}
			inner = `{"services":[` + strings.Join(rows, ",") + `]}`
		case "staleness_report":
			svc, _ := req.Params.Arguments["service"].(string)
			block := svcBlocks[svc]
			body := fmt.Sprintf(`{"service":%q,"source_count":1`, svc)
			if block != "" {
				body += `,"typed_edges":` + block
			}
			body += `}`
			inner = body
		default:
			http.Error(w, "unknown tool", 400)
			return
		}
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	return httptest.NewServer(mux)
}

// O30: doctor's typed_edges row must resolve the service the same way
// obligations_served does — via the daemon's recorded corpus path
// (candidatesWithDaemon), not the project directory's basename. Here the
// repo directory is named "sextant" but the daemon indexes it as "fathom"
// (the sextant-dir/fathom-service split, live on this very repo); the OLD
// code asked staleness_report("sextant") and always got known:false.
func TestCheckTypedEdges_ResolvesServiceLikeObligationsServed(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "sextant") // dir basename != indexed service name
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := typedEdgesDaemonMultiService(
		t,
		[]string{"fathom"},
		map[string]string{"fathom": repo},
		map[string]string{"fathom": `{"known":true,"packages":211,"failed":false}`},
	)
	defer srv.Close()

	got := checkTypedEdges(srv.Client(), srv.URL, repo, "", true)
	if got.Status != doctorPass {
		t.Fatalf("status = %v, want PASS resolved via the daemon-recorded path (like obligations_served), not the \"sextant\" basename (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "211") {
		t.Fatalf("reason must carry the package count from the correctly-resolved service: %q", got.Reason)
	}
	if !strings.Contains(got.Reason, "fathom") {
		t.Fatalf("reason should name the resolved service \"fathom\", not the directory basename \"sextant\": %q", got.Reason)
	}
}
