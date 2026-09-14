package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ─── flag validation ──────────────────────────────────────────────────────────

func TestCmdVerify_MissingPlan(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"verify", "--service", "svc"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2 when --plan missing, got %d; stderr=%q", code, errBuf.String())
	}
}

func TestCmdVerify_MissingService(t *testing.T) {
	pf := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(pf, []byte("- [x] task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := run([]string{"verify", "--plan", pf}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2 when --service missing, got %d; stderr=%q", code, errBuf.String())
	}
}

func TestCmdVerify_UnreadablePlan(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", "/no/such/plan.md",
		"--store", filepath.Join(t.TempDir(), "test.sqlite"),
	}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2 for unreadable plan, got %d; stderr=%q", code, errBuf.String())
	}
}

// ─── store mode ───────────────────────────────────────────────────────────────

// TestCmdVerify_Store_Pass: ingest payment-service, verify a symbol that
// exists (Charge, skip_test) → gate passes, exit 0.
func TestCmdVerify_Store_Pass(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	// Ingest the faux payment-service fixture.
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// Plan: one claimed task for a symbol that exists.
	plan := "- [x] Charge handler {symbol: Charge, skip_test: true}\n"
	pf := writeTempPlan(t, plan)

	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("verify store pass: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
}

// TestCmdVerify_Store_GateFail: a claimed task for a missing symbol → exit 1.
func TestCmdVerify_Store_GateFail(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// Claimed task for a non-existent symbol.
	plan := "- [x] Missing ghost {symbol: GhostSymbol, skip_test: true}\n"
	pf := writeTempPlan(t, plan)

	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store,
	}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("verify store gate fail: want exit 1, got %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
	// Output should mention the task.
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "GhostSymbol") && !strings.Contains(combined, "md-001") {
		t.Errorf("output should mention the failing task; got:\n%s", combined)
	}
}

// TestCmdVerify_Store_OpenTask_NoGate: open [ ] task that fails verification
// never gates (exit 0 from gate perspective).
func TestCmdVerify_Store_OpenTask_NoGate(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed: %s", ing.String())
	}

	// Open task for a missing symbol — should not gate.
	plan := "- [ ] Missing ghost {symbol: GhostSymbol, skip_test: true}\n"
	pf := writeTempPlan(t, plan)

	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("open task should not gate; exit %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
}

// TestCmdVerify_Store_Strict: --strict gates on Unknown (no-contract) tasks.
func TestCmdVerify_Store_Strict(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed: %s", ing.String())
	}

	// Claimed task with no contract → Unknown, which --strict should gate on.
	plan := "- [x] Vibe-based task with no contract\n"
	pf := writeTempPlan(t, plan)

	// Non-strict: exit 0 (unknown doesn't gate).
	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("non-strict: unknown should not gate; exit %d; stderr=%q", code, errBuf.String())
	}

	// Strict: exit 1 (unknown gates).
	var out2, errBuf2 bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store, "--strict",
	}, &out2, &errBuf2)
	if code != 1 {
		t.Fatalf("strict: unknown claimed task should gate; exit %d\nstdout=%s\nstderr=%s",
			code, out2.String(), errBuf2.String())
	}
}

// TestVerifyQuietPrintsFailingOnly: --quiet suppresses ✓ rows — a passing plan
// prints only the summary; a failing plan prints the ✗ row(s) + their detail
// lines + summary, and no ✓ rows (VS-8 / O19 / G14).
func TestVerifyQuietPrintsFailingOnly(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// Passing plan: one claimed task for a symbol that exists.
	passPlan := writeTempPlan(t, "- [x] Charge handler {symbol: Charge, skip_test: true}\n")
	var passOut, passErr bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", passPlan, "--store", store, "--quiet",
	}, &passOut, &passErr)
	if code != 0 {
		t.Fatalf("passing plan under --quiet: want exit 0, got %d; stdout=%q stderr=%q",
			code, passOut.String(), passErr.String())
	}
	if strings.Contains(passOut.String(), "✓") {
		t.Errorf("--quiet on a passing plan must suppress ✓ rows; stdout=%q", passOut.String())
	}
	if !strings.Contains(passOut.String(), "tasks total") {
		t.Errorf("--quiet on a passing plan must still print the summary; stdout=%q", passOut.String())
	}
	// Failing plan: one passing task + one claimed task for a missing symbol.
	failPlan := writeTempPlan(t, ""+
		"- [x] Charge handler {symbol: Charge, skip_test: true}\n"+
		"- [x] Missing ghost {symbol: GhostSymbol, skip_test: true}\n")
	var failOut, failErr bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", failPlan, "--store", store, "--quiet",
	}, &failOut, &failErr)
	if code != 1 {
		t.Fatalf("failing plan under --quiet: want exit 1, got %d; stdout=%q stderr=%q",
			code, failOut.String(), failErr.String())
	}
	out := failOut.String()
	if !strings.Contains(out, "✗") {
		t.Errorf("--quiet on a failing plan must print the ✗ row; stdout=%q", out)
	}
	if !strings.Contains(out, "GhostSymbol") && !strings.Contains(out, "Missing ghost") {
		t.Errorf("--quiet failing row must name the failing task; stdout=%q", out)
	}
	if strings.Contains(out, "✓") {
		t.Errorf("--quiet on a failing plan must still suppress ✓ rows; stdout=%q", out)
	}
	if strings.Contains(out, "Charge handler") {
		t.Errorf("--quiet must not print the passing task's row; stdout=%q", out)
	}
	if !strings.Contains(out, "tasks total") {
		t.Errorf("--quiet on a failing plan must still print the summary; stdout=%q", out)
	}

	// --quiet composes with --quiet-pass: both set + passing plan → no output at all.
	var bothOut, bothErr bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", passPlan, "--store", store, "--quiet", "--quiet-pass",
	}, &bothOut, &bothErr)
	if code != 0 {
		t.Fatalf("quiet+quiet-pass passing plan: want exit 0, got %d", code)
	}
	if bothOut.String() != "" {
		t.Errorf("--quiet + --quiet-pass on a passing plan must print nothing; stdout=%q", bothOut.String())
	}
}

// TestCmdVerify_Store_JSON: --json emits a parseable JSON report with gate key.
func TestCmdVerify_Store_JSON(t *testing.T) {
	store := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", store, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed: %s", ing.String())
	}

	plan := "- [x] Charge {symbol: Charge, skip_test: true}\n"
	pf := writeTempPlan(t, plan)

	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "payment-service",
		"--plan", pf, "--store", store, "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("json mode: exit %d; stderr=%q", code, errBuf.String())
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("json output not parseable: %v\nraw=%s", err, out.String())
	}
	gate, ok := m["gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing 'gate' key; output=%s", out.String())
	}
	if gate["pass"] != true {
		t.Errorf("gate.pass should be true; got %v", gate["pass"])
	}
}

// TestCmdVerify_OperationalError_BadStore: a non-existent store parent dir
// causes client open to fail → exit 2.
func TestCmdVerify_OperationalError_BadStore(t *testing.T) {
	pf := writeTempPlan(t, "- [x] task {symbol: Foo, skip_test: true}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", pf,
		"--store", "/no-such-dir/deep/path/verify.sqlite",
	}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2 for bad store, got %d; stderr=%q stdout=%q",
			code, errBuf.String(), out.String())
	}
}

// ─── HTTP mode ────────────────────────────────────────────────────────────────

// cannedVerifyServer returns an httptest.Server that answers list_services
// (the CLI's indexed-service probe) with a populated "svc" and verify_plan
// with a result where task md-001 has the given status/label.
func cannedVerifyServer(t *testing.T, taskStatus, taskLabel string, taskClaimed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"list_services"`) {
			writeRPCText(w, `{"services":[{"service":"svc","symbol_count":42}]}`)
			return
		}
		task := map[string]any{
			"id":     "md-001",
			"title":  "test task",
			"status": taskStatus,
			"label":  taskLabel,
			"criteria": []any{
				map[string]any{"name": "runtime_pass", "status": "unknown", "detail": "no CI evidence"},
			},
		}
		if taskClaimed {
			task["claimed"] = true
		}
		if taskStatus == "fail" {
			task["criteria"] = []any{
				map[string]any{"name": "symbol_present", "status": "fail", "detail": `"Ghost" not found`},
			}
		}
		inner := map[string]any{
			"service": "svc",
			"summary": map[string]any{"total": 1, "pass": 0, "fail": 0, "unknown": 1},
			"tasks":   []any{task},
		}
		if taskStatus == "pass" {
			inner["summary"] = map[string]any{"total": 1, "pass": 1, "fail": 0, "unknown": 0}
		} else if taskStatus == "fail" {
			inner["summary"] = map[string]any{"total": 1, "pass": 0, "fail": 1, "unknown": 0}
		}
		innerJSON, _ := json.Marshal(inner)
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": string(innerJSON)},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// writeRPCText writes a JSON-RPC MCP text-content envelope around inner.
func writeRPCText(w http.ResponseWriter, inner string) {
	resp := map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"result": map[string]any{
			"content": []any{map[string]any{"type": "text", "text": inner}},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// TestCmdVerify_HTTP_ServiceNotIndexed: the daemon answers but has no symbols
// for the service — every claim would read "not found" (evidence-shaped output
// for an operational problem). Must be exit 2 (fail-open), never exit 1.
func TestCmdVerify_HTTP_ServiceNotIndexed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCText(w, `{"services":[{"service":"other","symbol_count":9}]}`)
	}))
	defer ts.Close()

	plan := writeTempPlan(t, "- [x] claimed work {symbol: Ghost}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{"verify", "--service", "svc", "--url", ts.URL, "--plan", plan}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (operational fail-open); stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "not indexed") {
		t.Errorf("stderr %q does not explain the unindexed service", errBuf.String())
	}
}

// TestCmdVerify_Store_EmptyIndex: same principle in store mode.
func TestCmdVerify_Store_EmptyIndex(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "empty.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	plan := writeTempPlan(t, "- [x] claimed work {symbol: Ghost}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{"verify", "--service", "svc", "--store", storePath, "--plan", plan}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (operational fail-open); stderr: %s", code, errBuf.String())
	}
}

// TestCmdVerify_HTTP_Pass: daemon returns pass for a claimed task → exit 0.
func TestCmdVerify_HTTP_Pass(t *testing.T) {
	ts := cannedVerifyServer(t, "pass", "verified", true)
	defer ts.Close()

	// Local plan: [x] claimed → Claimed=true locally
	pf := writeTempPlan(t, "- [x] test task {symbol: Foo, skip_test: true}\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", pf, "--url", ts.URL, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("HTTP pass: exit %d; stderr=%q stdout=%q", code, errBuf.String(), out.String())
	}
}

// TestCmdVerify_HTTP_GateFail: daemon returns fail for a claimed task → exit 1.
func TestCmdVerify_HTTP_GateFail(t *testing.T) {
	ts := cannedVerifyServer(t, "fail", "claimed-but-absent", true)
	defer ts.Close()

	pf := writeTempPlan(t, "- [x] missing ghost {symbol: Ghost, skip_test: true}\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", pf, "--url", ts.URL,
	}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("HTTP gate fail: want exit 1, got %d; stderr=%q stdout=%q",
			code, errBuf.String(), out.String())
	}
}

// TestCmdVerify_HTTP_FailNotGated_OpenTask: daemon returns fail but task is
// open [ ] → gate passes (exit 0).
func TestCmdVerify_HTTP_FailNotGated_OpenTask(t *testing.T) {
	ts := cannedVerifyServer(t, "fail", "claimed-but-absent", false)
	defer ts.Close()

	pf := writeTempPlan(t, "- [ ] missing ghost {symbol: Ghost, skip_test: true}\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", pf, "--url", ts.URL, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("open task fail should not gate; exit %d; stderr=%q", code, errBuf.String())
	}
}

// TestCmdVerify_HTTP_OperationalError: daemon unreachable → exit 2 (fail-open).
func TestCmdVerify_HTTP_OperationalError(t *testing.T) {
	pf := writeTempPlan(t, "- [x] task {symbol: Foo, skip_test: true}\n")
	var out, errBuf bytes.Buffer
	// Port 1 is unprivileged and almost certainly not listening.
	code := run([]string{
		"verify", "--service", "svc",
		"--plan", pf, "--url", "http://127.0.0.1:1",
	}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("want exit 2 when daemon unreachable, got %d; stderr=%q stdout=%q",
			code, errBuf.String(), out.String())
	}
}

// ─── dispatch ─────────────────────────────────────────────────────────────────

// TestCmdVerify_InMainSwitch: "verify" is registered in the subcommand switch.
func TestCmdVerify_InMainSwitch(t *testing.T) {
	var out, errBuf bytes.Buffer
	// Missing --plan and --service → exit 2 (operational), not 1 (unknown).
	code := run([]string{"verify"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("verify with no args: want exit 2, got %d; stderr=%q", code, errBuf.String())
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeTempPlan(t *testing.T, content string) string {
	t.Helper()
	pf := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(pf, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return pf
}

// ─── package: grammar e2e suite ────────────────────────────────────────────

// seedPackageGrammarStore seeds a SQLite store with a bare symbol name ("New")
// that is genuinely ambiguous across two packages, plus a test that reaches
// exactly one of them via its canonical key. This is the shape the `package:`
// plan annotation exists to disambiguate: without it, the bare name resolves
// to nothing in the call graph (ambiguity stays honest, never first-match-wins).
func seedPackageGrammarStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.sqlite")

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()

	obs := []core.Observation{
		{
			ServiceID:    "svc",
			SourceRef:    "foundations/pkga/new.go",
			Symbol:       "New",
			Package:      "pkga",
			EvidenceTier: core.TierVerified,
			SourceKind:   core.KindGoFunc,
		},
		{
			ServiceID:    "svc",
			SourceRef:    "foundations/pkga/new_test.go",
			Symbol:       "TestNew",
			Package:      "pkga",
			EvidenceTier: core.TierVerified,
			SourceKind:   core.KindGoFunc,
			// TestNew calls pkga's New by canonical key — the test_reaches edge.
			Calls: []string{"svc::pkga.New"},
		},
		{
			// Same bare name in another package: makes "New" ambiguous.
			ServiceID:    "svc",
			SourceRef:    "foundations/pkgb/new.go",
			Symbol:       "New",
			Package:      "pkgb",
			EvidenceTier: core.TierVerified,
			SourceKind:   core.KindGoFunc,
		},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
	return storePath
}

// verifyMarkdownPlan runs gen-status-cue over a markdown plan checklist against
// the seeded store and returns the rendered STATUS.cue content.
func verifyMarkdownPlan(t *testing.T, storePath, planMD string) string {
	t.Helper()
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte(planMD), 0o644); err != nil {
		t.Fatalf("writing plan file: %v", err)
	}
	outPath := filepath.Join(dir, "STATUS.cue")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-status-cue",
		"--plan", planPath,
		"--service", "svc",
		"--store", storePath,
		"--output", outPath,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("gen-status-cue exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	return string(content)
}

// TestVerifyMarkdownPlan_PackageKeyHappyPath: a `package:` annotation
// disambiguates a bare symbol name that exists in two packages; the claim
// verifies via the canonical key of the node under that directory.
func TestVerifyMarkdownPlan_PackageKeyHappyPath(t *testing.T) {
	storePath := seedPackageGrammarStore(t)
	got := verifyMarkdownPlan(t, storePath,
		"- [x] constructor {symbol: New, package: foundations/pkga}\n")

	if !strings.Contains(got, `label:  "verified"`) {
		t.Errorf("expected verified verdict via package: disambiguation; got:\n%s", got)
	}
	if !strings.Contains(got, `status: "pass"`) {
		t.Errorf("expected pass status; got:\n%s", got)
	}
}

// TestVerifyMarkdownPlan_NonexistentSymbol: a claimed symbol that is not in
// the index must fail with claimed-but-absent — the gate still bites.
// (Permanent negative proof, not a scratch file.)
func TestVerifyMarkdownPlan_NonexistentSymbol(t *testing.T) {
	storePath := seedPackageGrammarStore(t)
	got := verifyMarkdownPlan(t, storePath,
		"- [x] ghost feature {symbol: Ghost}\n")

	if !strings.Contains(got, `label:  "claimed-but-absent"`) {
		t.Errorf("expected claimed-but-absent for a nonexistent symbol; got:\n%s", got)
	}
	if !strings.Contains(got, `status: "fail"`) {
		t.Errorf("expected fail status; got:\n%s", got)
	}
}

// TestVerifyMarkdownPlan_WrongPackage: a real symbol claimed under a directory
// it does not live in must fail with claimed-but-absent — a `package:` filter
// that empties the match set is a failed claim, never silently ignored.
// (Permanent negative proof, not a scratch file.)
func TestVerifyMarkdownPlan_WrongPackage(t *testing.T) {
	storePath := seedPackageGrammarStore(t)
	got := verifyMarkdownPlan(t, storePath,
		"- [x] constructor {symbol: New, package: foundations/pkgc}\n")

	if !strings.Contains(got, `label:  "claimed-but-absent"`) {
		t.Errorf("expected claimed-but-absent for a wrong package: filter; got:\n%s", got)
	}
	if !strings.Contains(got, `status: "fail"`) {
		t.Errorf("expected fail status; got:\n%s", got)
	}
}

// ─── mutation (VB-34) ───────────────────────────────────────────────────────

// mutationFixture ingests a tiny package with a Test* function that reaches
// Widget via a direct syntax call edge, so test_reaches Passes and
// OverlayMutation has something to act on.
func mutationFixture(t *testing.T) (storePath, service string, widgetLine int) {
	t.Helper()
	root := t.TempDir()
	src := "package widget\n\nfunc Widget() int {\n\treturn 1\n}\n"
	testSrc := "package widget\n\nimport \"testing\"\n\nfunc TestWidget(t *testing.T) {\n\t_ = Widget()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "widget.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "widget_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write fixture test: %v", err)
	}
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	service = "mutation-svc"
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", service, "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath, service, 3 // `func Widget() int {` is line 3
}

func writeVerifyMutationReport(t *testing.T, line int, killed bool) string {
	t.Helper()
	body := fmt.Sprintf(`{"mutants":[{"file":"widget.go","line":%d,"killed":%t}]}`, line, killed)
	path := filepath.Join(t.TempDir(), "mutation.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write mutation report: %v", err)
	}
	return path
}

func TestCmdVerify_Mutation_Advisory_NotesCoveredNotAsserted(t *testing.T) {
	storePath, service, line := mutationFixture(t)
	pf := writeTempPlan(t, "- [x] widget works {symbol: Widget}\n")
	mutationPath := writeVerifyMutationReport(t, line, false)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", service, "--plan", pf, "--store", storePath,
		"--mutation", mutationPath, "--format", "json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("advisory mutation must not fail the gate: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "covered-not-asserted") {
		t.Errorf("expected a covered-not-asserted note in the JSON report:\n%s", out.String())
	}
}

func TestCmdVerify_Mutation_Strict_FailsGate(t *testing.T) {
	storePath, service, line := mutationFixture(t)
	pf := writeTempPlan(t, "- [x] widget works {symbol: Widget}\n")
	mutationPath := writeVerifyMutationReport(t, line, false)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", service, "--plan", pf, "--store", storePath,
		"--mutation", mutationPath, "--strict-mutation", "--format", "json",
	}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("--strict-mutation must fail the gate on a score-0 finding: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "covered-not-asserted") {
		t.Errorf("expected a covered-not-asserted label in the JSON report:\n%s", out.String())
	}
}

func TestCmdVerify_Mutation_KilledScore_NeverFails(t *testing.T) {
	storePath, service, line := mutationFixture(t)
	pf := writeTempPlan(t, "- [x] widget works {symbol: Widget}\n")
	mutationPath := writeVerifyMutationReport(t, line, true)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", service, "--plan", pf, "--store", storePath,
		"--mutation", mutationPath, "--strict-mutation",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("a killed mutant (score 1.0) must never fail the gate: exit %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
}

func TestCmdVerify_Mutation_RequiresStore(t *testing.T) {
	pf := writeTempPlan(t, "- [x] task {symbol: Widget, skip_test: true}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "svc", "--plan", pf,
		"--mutation", filepath.Join(t.TempDir(), "mutation.json"),
	}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("--mutation without --store must be an operational error (exit 2), got %d; stderr=%q", code, errBuf.String())
	}
}
