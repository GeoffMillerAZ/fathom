package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/specdialect"
)

// TestVerifyForeignPlanInPlace (SI-9, ADR-049, spec-interop design §3):
// `fathom verify --plan <foreign file>` dispatches to the dialect that
// claims it, `--dialect` overrides, the exit codes stay 0/1/2, verdicts
// point at lines in the foreign file, and the foreign file is
// byte-identical after the run.
func TestVerifyForeignPlanInPlace(t *testing.T) {
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "payment-service", "--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux"}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// dialectForPlan: detection, the override, and refusals.
	reg := specdialect.Default()
	if d, err := dialectForPlan(reg, "specs/001-pay/tasks.md", []byte("- [ ] T001 x\n"), ""); err != nil || d.ID() != "speckit" {
		t.Fatalf("dialectForPlan(spec kit) = %v, %v", d, err)
	}
	if d, err := dialectForPlan(reg, "docs/plans/p.md", []byte("- [ ] x\n"), ""); err != nil || d.ID() != specdialect.NativeID {
		t.Fatalf("dialectForPlan(native) = %v, %v", d, err)
	}
	if d, err := dialectForPlan(reg, "notes.md", []byte("- [ ] T001 x\n"), "speckit"); err != nil || d.ID() != "speckit" {
		t.Fatalf("--dialect override = %v, %v", d, err)
	}
	if _, err := dialectForPlan(reg, "notes.md", nil, "nope"); err == nil || !strings.Contains(err.Error(), "speckit") {
		t.Fatalf("an unknown --dialect must be refused naming the known ids, got %v", err)
	}
	if _, err := dialectForPlan(reg, "tasks.json", []byte("{}"), ""); err == nil {
		t.Fatal("a file no dialect claims must be refused")
	}

	dir := filepath.Join(t.TempDir(), "specs", "001-pay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pf := filepath.Join(dir, "tasks.md")
	passing := "# Tasks\n\n" +
		"- [x] T001 Charge handler {symbol: Charge, skip_test: true, skip: \"fixture\"}\n" +
		"- [X] T002 [P] The verify layer lives in internal/verify/verify.go\n" +
		"- [ ] T003 Write the runbook\n"
	if err := os.WriteFile(pf, []byte(passing), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--service", "payment-service", "--plan", pf, "--store", store}, &out, &errb); code != 0 {
		t.Fatalf("passing Spec Kit plan: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	for _, want := range []string{"[T001]", "[T002]", "[T003]", "tasks.md:3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if after, _ := os.ReadFile(pf); string(after) != passing {
		t.Fatal("the foreign file changed during verify")
	}

	// A claimed task whose contract fails: exit 1, located in the foreign file.
	failing := passing + "- [x] T004 Ghost {symbol: GhostSymbol, skip_test: true, skip: \"fixture\"}\n"
	if err := os.WriteFile(pf, []byte(failing), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"verify", "--service", "payment-service", "--plan", pf, "--store", store, "--json"}, &out, &errb); code != 1 {
		t.Fatalf("failing Spec Kit plan: want exit 1, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	var doc struct {
		Reports []struct {
			Tasks []struct {
				TaskID string `json:"id"`
				Line   int    `json:"line"`
				Status string `json:"status"`
			} `json:"tasks"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	found := false
	for _, r := range doc.Reports {
		for _, tk := range r.Tasks {
			if tk.TaskID == "T004" {
				found = true
				if tk.Line != 6 || tk.Status != "fail" {
					t.Errorf("T004 = line %d status %s, want line 6 fail", tk.Line, tk.Status)
				}
			}
		}
	}
	if !found {
		t.Fatalf("T004 missing from the JSON report:\n%s", out.String())
	}
	if after, _ := os.ReadFile(pf); string(after) != failing {
		t.Fatal("the foreign file changed during verify")
	}

	// --dialect overrides detection: the same rows in a file no dialect
	// would call Spec Kit's.
	other := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(other, []byte(passing), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"verify", "--service", "payment-service", "--plan", other, "--store", store, "--dialect", "speckit"}, &out, &errb); code != 0 {
		t.Fatalf("--dialect speckit: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "[T001]") {
		t.Errorf("--dialect speckit did not read T### ids:\n%s", out.String())
	}

	// An unknown dialect is operational: exit 2.
	if code := run([]string{"verify", "--service", "payment-service", "--plan", other, "--store", store, "--dialect", "nope"}, &out, &errb); code != 2 {
		t.Fatalf("--dialect nope: want exit 2, got %d", code)
	}
}

// TestVerifyForeignPlanOverHTTP (SI-9): in daemon mode a foreign plan is
// sent synthesized into the native grammar (the daemon parses
// plan_markdown natively), never as the foreign bytes, and the verdict's
// line is the foreign file's.
func TestVerifyForeignPlanOverHTTP(t *testing.T) {
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	var sent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"list_services"`) {
			writeRPCText(w, `{"services":[{"service":"svc","symbol_count":42}]}`)
			return
		}
		var req struct {
			Params struct {
				Arguments struct {
					PlanMarkdown string `json:"plan_markdown"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Arguments.PlanMarkdown != "" {
			sent = req.Params.Arguments.PlanMarkdown
		}
		writeRPCText(w, `{"service":"svc","summary":{"total":1,"pass":0,"fail":1,"unknown":0},"tasks":[{"id":"T001","title":"Ghost","status":"fail","label":"claimed-but-absent","criteria":[{"name":"symbol_present","status":"fail","detail":"Ghost not found"}]}]}`)
	}))
	defer ts.Close()

	dir := filepath.Join(t.TempDir(), "specs", "001-pay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pf := filepath.Join(dir, "tasks.md")
	raw := "# Tasks\n\n- [x] T001 Ghost {symbol: Ghost, skip_test: true, skip: \"fixture\"}\n"
	if err := os.WriteFile(pf, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--service", "svc", "--url", ts.URL, "--plan", pf}, &out, &errb); code != 1 {
		t.Fatalf("want exit 1, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if !strings.Contains(sent, "{id: T001, symbol: Ghost") || strings.Contains(sent, "# Tasks") {
		t.Fatalf("the daemon must be sent the synthesized native plan, got:\n%s", sent)
	}
	if !strings.Contains(out.String(), "tasks.md:3") {
		t.Fatalf("the verdict must point at the foreign line:\n%s", out.String())
	}
	if after, _ := os.ReadFile(pf); string(after) != raw {
		t.Fatal("the foreign file changed during verify")
	}
}
