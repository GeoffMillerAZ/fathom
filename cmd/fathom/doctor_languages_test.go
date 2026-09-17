package main

// doctor_languages_test.go — O69 (wave P1): the `unsupported_languages`
// doctor row. The Long Code Arena bench run found bumble-tech/appyx, a real
// Kotlin repository, indexed near-empty with no signal anywhere — not
// list_services, not find_symbol, not outline — that Kotlin was the reason.
// This row reads list_services (the same tool list_servicesTest already
// pins the wire shape for) and warns by name and file count when this
// repo's own service carries any, so the gap surfaces at `fathom doctor`
// time instead of only after a suspiciously low symbol count is noticed.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// languagesDaemon serves list_services with one row for "svc" carrying
// unsupportedBlock verbatim as its "unsupported_languages" value (raw JSON,
// so "field absent" is expressible by passing "").
func languagesDaemon(t *testing.T, unsupportedBlock string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name != "list_services" {
			http.Error(w, "unknown tool", 400)
			return
		}
		row := `{"service":"svc","symbol_count":165,"languages":["javascript"]`
		if unsupportedBlock != "" {
			row += `,"unsupported_languages":` + unsupportedBlock
		}
		row += `}`
		inner := `{"services":[` + row + `],"count":1}`
		env := map[string]any{"result": map[string]any{"content": []map[string]any{{"type": "text", "text": inner}}}}
		_ = json.NewEncoder(w).Encode(env)
	})
	return httptest.NewServer(mux)
}

func languagesRepo(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestCheckUnsupportedLanguages_WarnsWithNameAndCount(t *testing.T) {
	srv := languagesDaemon(t, `[{"language":"kotlin","files":412}]`)
	defer srv.Close()

	got := checkUnsupportedLanguages(srv.Client(), srv.URL, languagesRepo(t), "svc", true)
	if got.Name != "unsupported_languages" {
		t.Fatalf("row name = %q", got.Name)
	}
	if got.Status != doctorWarn {
		t.Fatalf("status = %v, want WARN — a known-unsupported language must be visible, not FAIL or silently PASS (%s)", got.Status, got.Reason)
	}
	if !strings.Contains(got.Reason, "kotlin") || !strings.Contains(got.Reason, "412") {
		t.Fatalf("reason must name the language and its file count: %q", got.Reason)
	}
}

func TestCheckUnsupportedLanguages_PassesWhenNone(t *testing.T) {
	srv := languagesDaemon(t, "")
	defer srv.Close()

	got := checkUnsupportedLanguages(srv.Client(), srv.URL, languagesRepo(t), "svc", true)
	if got.Status != doctorPass {
		t.Fatalf("status = %v, want PASS when nothing is unsupported (%s)", got.Status, got.Reason)
	}
}

func TestCheckUnsupportedLanguages_DaemonDownSkips(t *testing.T) {
	got := checkUnsupportedLanguages(http.DefaultClient, "http://127.0.0.1:1", languagesRepo(t), "svc", false)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP when the daemon is unreachable (%s)", got.Status, got.Reason)
	}
}

func TestCheckUnsupportedLanguages_ServiceNotIngestedSkips(t *testing.T) {
	srv := languagesDaemon(t, "")
	defer srv.Close()

	got := checkUnsupportedLanguages(srv.Client(), srv.URL, languagesRepo(t), "not-svc", true)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP when the service is not in list_services (%s)", got.Status, got.Reason)
	}
}

// A directory doctor cannot resolve to any indexed service (no explicit
// --service, no daemon-recorded identity match) is skipped, never failed.
func TestCheckUnsupportedLanguages_NoResolvableServiceSkips(t *testing.T) {
	srv := languagesDaemon(t, "")
	defer srv.Close()

	root := t.TempDir()
	unresolvable := filepath.Join(root, "not-recorded-anywhere")
	if err := os.MkdirAll(unresolvable, 0o755); err != nil {
		t.Fatal(err)
	}
	got := checkUnsupportedLanguages(srv.Client(), srv.URL, unresolvable, "", true)
	if got.Status != doctorSkipped {
		t.Fatalf("status = %v, want SKIP (%s)", got.Status, got.Reason)
	}
}
