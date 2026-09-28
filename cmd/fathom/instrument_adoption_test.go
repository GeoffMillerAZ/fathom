package main

// instrument_adoption_test.go — FW-1 (docs/plans/pack-taxonomy-adoption.md).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// writeCallLogFixture writes entries as JSONL at path (creating parent dirs).
func writeCallLogFixture(t *testing.T, path string, entries []adoptionCallEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var buf bytes.Buffer
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// isolateVerifyLedger points FATHOM_VERIFY_LEDGER at a fresh, empty path for
// the duration of the test (IT-11: the verify-gate figure now reads
// ~/.fathom/verify-ledger.jsonl by default, and this repo's own is a live,
// growing file from real `fathom verify` runs — every test that exercises
// cmdInstrumentAdoption must not silently pick up ambient rows).
func isolateVerifyLedger(t *testing.T) {
	t.Helper()
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
}

// ─── buildAdoptionReport: the pure, deterministic core ─────────────────────

func TestBuildAdoptionReport_Golden(t *testing.T) {
	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
		{TS: "2026-07-23T10:01:00Z", Tool: "find_symbol", Surface: "agent"},
		{TS: "2026-07-23T10:02:00Z", Tool: "brief", Surface: "agent", ClaimsAttached: 3},
		{TS: "2026-07-23T10:03:00Z", Tool: "brief", Surface: "agent"}, // no claims attached
		{TS: "2026-07-23T09:00:00Z", Tool: "verify_plan", Surface: ""},
	}
	registered := []string{"brief", "find_symbol", "verify_plan", "capabilities"}
	openObs := 2

	r := buildAdoptionReport("workspace/logs/mcp-calls.jsonl", true, "", entries, 0, registered, "", &openObs, ".fathom/observations.md", nil)

	if r.TotalCalls != 5 {
		t.Errorf("TotalCalls = %d, want 5", r.TotalCalls)
	}

	// Surface traffic: sorted by surface name; "" (bare) sorts first.
	wantSurfaces := []SurfaceTrafficRow{
		{Surface: "", Calls: 1, LastTS: "2026-07-23T09:00:00Z"},
		{Surface: "agent", Calls: 4, LastTS: "2026-07-23T10:03:00Z"},
	}
	if len(r.SurfaceTraffic) != len(wantSurfaces) {
		t.Fatalf("SurfaceTraffic = %+v, want %+v", r.SurfaceTraffic, wantSurfaces)
	}
	for i, want := range wantSurfaces {
		if r.SurfaceTraffic[i] != want {
			t.Errorf("SurfaceTraffic[%d] = %+v, want %+v", i, r.SurfaceTraffic[i], want)
		}
	}

	// Top tools: find_symbol and brief tie at 2 calls each — alphabetical
	// tie-break puts brief first; verify_plan (1 call) last.
	wantTop := []ToolCallRow{
		{Tool: "brief", Calls: 2},
		{Tool: "find_symbol", Calls: 2},
		{Tool: "verify_plan", Calls: 1},
	}
	if len(r.TopTools) != len(wantTop) {
		t.Fatalf("TopTools = %+v, want %+v", r.TopTools, wantTop)
	}
	for i, want := range wantTop {
		if r.TopTools[i] != want {
			t.Errorf("TopTools[%d] = %+v, want %+v", i, r.TopTools[i], want)
		}
	}

	// Zero-call registered tools: "capabilities" was registered but never called.
	if len(r.ZeroCallTools) != 1 || r.ZeroCallTools[0] != "capabilities" {
		t.Errorf("ZeroCallTools = %v, want [capabilities]", r.ZeroCallTools)
	}
	if r.RegisteredToolCount != 4 {
		t.Errorf("RegisteredToolCount = %d, want 4", r.RegisteredToolCount)
	}

	// Claims-attach hit rate: 1 of 2 brief calls attached claims.
	if r.BriefCalls != 2 || r.BriefCallsWithClaims != 1 {
		t.Errorf("BriefCalls=%d BriefCallsWithClaims=%d, want 2 and 1", r.BriefCalls, r.BriefCallsWithClaims)
	}
	if r.ClaimsAttachHitRate == nil || *r.ClaimsAttachHitRate != 0.5 {
		t.Errorf("ClaimsAttachHitRate = %v, want 0.5", r.ClaimsAttachHitRate)
	}

	// Verify-gate (IT-11): sourced from the verify ledger, never the call
	// log — the verify_plan row above (with or without a verdict_outcome)
	// no longer has any bearing on this figure. No ledger rows given here,
	// so: honest "not instrumented" marker, never a fabricated zero.
	if r.VerifyGateRefusals != nil {
		t.Errorf("VerifyGateRefusals = %v, want nil (not instrumented)", r.VerifyGateRefusals)
	}
	if !strings.Contains(r.VerifyGateNote, "not instrumented") {
		t.Errorf("VerifyGateNote = %q, want it to say not instrumented", r.VerifyGateNote)
	}
	if r.VerifyGateInstrumentedCalls != 0 {
		t.Errorf("VerifyGateInstrumentedCalls = %d, want 0", r.VerifyGateInstrumentedCalls)
	}
	if r.VerifyGatePredatesInstrumentation != 0 {
		t.Errorf("VerifyGatePredatesInstrumentation = %d, want 0 (retired: ledger rows never predate instrumentation)", r.VerifyGatePredatesInstrumentation)
	}

	if r.OpenObservationCount == nil || *r.OpenObservationCount != 2 {
		t.Errorf("OpenObservationCount = %v, want 2", r.OpenObservationCount)
	}
}

// ─── IT-11: verify-gate figure, ledger-sourced ─────────────────────────────

// TestBuildAdoptionReport_GateCatchesRealCounts proves the verify-gate
// figure derives from the verify ledger, never the call log (IT-11, the
// 215/4940-family double-count this replaces): call-log verify_plan/review
// rows carrying a verdict_outcome are included here specifically to prove
// they no longer move this figure — if the retired call-log-derived path
// were still live, the refusal count below would be 2, not 1.
func TestBuildAdoptionReport_GateCatchesRealCounts(t *testing.T) {
	ledgerRows := []verifyLedgerRow{
		{Plan: "docs/plans/a.md", Total: 2, Claimed: 2, Verified: 1, Failing: 1},
		{Plan: "docs/plans/b.md", Total: 3, Claimed: 3, Verified: 3, Failing: 0},
	}
	entries := []adoptionCallEntry{
		{
			TS: "2026-07-24T10:00:00Z", Tool: "verify_plan", Surface: "agent",
			VerdictOutcome: map[string]int{"claimed": 2, "verified": 1, "failing": 1},
		},
		{
			TS: "2026-07-24T10:01:00Z", Tool: "review", Surface: "cli",
			VerdictOutcome: map[string]int{"impact": 0, "obligations": 1, "arch_conformance": 1, "spec_coverage": 0},
		},
	}
	r := buildAdoptionReport("x.jsonl", true, "", entries, 0, nil, "", nil, ".fathom/observations.md", ledgerRows)

	if r.VerifyGateRefusals == nil || *r.VerifyGateRefusals != 1 {
		t.Fatalf("VerifyGateRefusals = %v, want 1 refusal event (one ledger row had Failing>0; the call log's own 2 must not leak in)", r.VerifyGateRefusals)
	}
	if r.VerifyGateInstrumentedCalls != 2 {
		t.Errorf("VerifyGateInstrumentedCalls = %d, want 2 (the ledger ROW count, not the call-log entry count)", r.VerifyGateInstrumentedCalls)
	}
	if r.VerifyGatePredatesInstrumentation != 0 {
		t.Errorf("VerifyGatePredatesInstrumentation = %d, want 0", r.VerifyGatePredatesInstrumentation)
	}
	if strings.Contains(r.VerifyGateNote, "not instrumented") {
		t.Errorf("VerifyGateNote = %q, should not say not instrumented when the ledger has rows", r.VerifyGateNote)
	}
	if !strings.Contains(r.VerifyGateNote, "1 gate refusal") {
		t.Errorf("VerifyGateNote = %q, want it to state the 1 refusal", r.VerifyGateNote)
	}
	if !strings.Contains(r.VerifyGateNote, "2 verify-ledger run") {
		t.Errorf("VerifyGateNote = %q, want it to name the ledger-row denominator", r.VerifyGateNote)
	}
}

// TestBuildAdoptionReport_GateCatchesHonestSplit is the "never a fabricated
// zero" proof, re-pointed at the ledger source (IT-11): a ledger that has
// never run reports nil; a ledger that HAS run, entirely cleanly, reports a
// real, non-nil 0 — the two must never look the same.
func TestBuildAdoptionReport_GateCatchesHonestSplit(t *testing.T) {
	t.Run("no ledger rows: nil, never a fabricated zero", func(t *testing.T) {
		r := buildAdoptionReport("x.jsonl", true, "", nil, 0, nil, "", nil, ".fathom/observations.md", nil)
		if r.VerifyGateRefusals != nil {
			t.Errorf("VerifyGateRefusals = %v, want nil (the ledger has never run)", r.VerifyGateRefusals)
		}
		if !strings.Contains(r.VerifyGateNote, "not instrumented") {
			t.Errorf("VerifyGateNote = %q, want it to say not instrumented", r.VerifyGateNote)
		}
	})

	t.Run("ledger ran, entirely clean: a real 0", func(t *testing.T) {
		ledgerRows := []verifyLedgerRow{
			{Plan: "docs/plans/a.md", Total: 2, Claimed: 2, Verified: 2, Failing: 0},
			{Plan: "docs/plans/b.md", Total: 1, Claimed: 1, Verified: 1, Failing: 0},
		}
		r := buildAdoptionReport("x.jsonl", true, "", nil, 0, nil, "", nil, ".fathom/observations.md", ledgerRows)
		if r.VerifyGateRefusals == nil || *r.VerifyGateRefusals != 0 {
			t.Fatalf("VerifyGateRefusals = %v, want a real 0 (the ledger ran twice, both clean)", r.VerifyGateRefusals)
		}
		if r.VerifyGateInstrumentedCalls != 2 {
			t.Errorf("VerifyGateInstrumentedCalls = %d, want 2", r.VerifyGateInstrumentedCalls)
		}
		if strings.Contains(r.VerifyGateNote, "not instrumented") {
			t.Errorf("VerifyGateNote = %q, must not say not instrumented — the ledger genuinely ran", r.VerifyGateNote)
		}
	})
}

func TestBuildAdoptionReport_NoBriefCallsIsUndefinedNotZero(t *testing.T) {
	entries := []adoptionCallEntry{{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"}}
	r := buildAdoptionReport("x.jsonl", true, "", entries, 0, []string{"find_symbol"}, "", nil, ".fathom/observations.md", nil)
	if r.ClaimsAttachHitRate != nil {
		t.Errorf("ClaimsAttachHitRate = %v, want nil when no brief calls occurred", r.ClaimsAttachHitRate)
	}
	if r.OpenObservationCount != nil {
		t.Errorf("OpenObservationCount = %v, want nil when no observations file was read", r.OpenObservationCount)
	}
}

func TestBuildAdoptionReport_EmptyLogIsHonestZero(t *testing.T) {
	r := buildAdoptionReport("x.jsonl", true, "", nil, 0, []string{"find_symbol"}, "", nil, ".fathom/observations.md", nil)
	if r.TotalCalls != 0 {
		t.Errorf("TotalCalls = %d, want 0", r.TotalCalls)
	}
	if len(r.SurfaceTraffic) != 0 || len(r.TopTools) != 0 {
		t.Errorf("expected empty surface traffic and top tools, got %+v / %+v", r.SurfaceTraffic, r.TopTools)
	}
	if len(r.ZeroCallTools) != 1 || r.ZeroCallTools[0] != "find_symbol" {
		t.Errorf("ZeroCallTools = %v, want [find_symbol] (nothing was ever called)", r.ZeroCallTools)
	}
}

// ─── readCallLog ────────────────────────────────────────────────────────────

func TestReadCallLog_MissingFileIsNotFatal(t *testing.T) {
	entries, skipped, read := readCallLog(filepath.Join(t.TempDir(), "nope.jsonl"))
	if read {
		t.Error("read = true for a missing file, want false")
	}
	if entries != nil {
		t.Errorf("entries = %v, want nil", entries)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
}

func TestReadCallLog_SkipsMalformedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	content := `{"ts":"2026-07-23T10:00:00Z","tool":"find_symbol"}
{"ts":"2026-07-23T10:01:00Z","tool":"brief"}
not valid json at all
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, skipped, read := readCallLog(path)
	if !read {
		t.Error("read = false, want true (the file exists and opened fine)")
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want exactly the 2 well-formed lines", entries)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the garbage tail line)", skipped)
	}
}

// TestReadCallLog_SkipsMalformedMidFileLines is H2's regression test: a
// torn line in the MIDDLE of the log (daemon killed mid-write,
// hand-editing debris) must cost exactly that line — every entry after it
// still counts. The pre-fix json.Decoder-over-the-stream version broke at
// the first bad token and silently discarded the rest of the file.
func TestReadCallLog_SkipsMalformedMidFileLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	content := `{"ts":"2026-07-23T10:00:00Z","tool":"find_symbol"}
{"ts":"2026-07-23T10:01:00Z","tool":"bri
{"ts":"2026-07-23T10:02:00Z","tool":"brief","claims_attached":2}
{"ts":"2026-07-23T10:03:00Z","tool":"verify_plan"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, skipped, read := readCallLog(path)
	if !read {
		t.Fatal("read = false, want true")
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %+v, want the 3 well-formed lines INCLUDING those after the torn one", entries)
	}
	if entries[1].Tool != "brief" || entries[1].ClaimsAttached != 2 || entries[2].Tool != "verify_plan" {
		t.Errorf("post-torn-line entries wrong: %+v", entries)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
}

// ─── registeredToolNames: the tool-registry source ─────────────────────────

func TestRegisteredToolNames_NilBackendMatchesKnownTools(t *testing.T) {
	names, err := registeredToolNames("")
	if err != nil {
		t.Fatalf("registeredToolNames(\"\"): %v", err)
	}
	want := map[string]bool{"find_symbol": true, "status": true, "verify_plan": true, "read_source": true}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	for w := range want {
		if !seen[w] {
			t.Errorf("registeredToolNames(\"\") missing %q; got %v", w, names)
		}
	}
	if !sortedStrings(names) {
		t.Errorf("registeredToolNames(\"\") not sorted: %v", names)
	}
}

func TestRegisteredToolNames_WithStoreMatchesNilBackend(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	// store.Open creates the file fresh; registration doesn't depend on its
	// contents, only on which tools the binary defines.
	withStore, err := registeredToolNames(storePath)
	if err != nil {
		t.Fatalf("registeredToolNames(store): %v", err)
	}
	nilBackend, err := registeredToolNames("")
	if err != nil {
		t.Fatalf("registeredToolNames(\"\"): %v", err)
	}
	if len(withStore) != len(nilBackend) {
		t.Fatalf("store-backed registry has %d tools, nil-backend has %d — registration must not depend on the backend", len(withStore), len(nilBackend))
	}
	for i := range withStore {
		if withStore[i] != nilBackend[i] {
			t.Errorf("registry[%d] = %q (store) vs %q (nil) — must be identical", i, withStore[i], nilBackend[i])
		}
	}
}

func sortedStrings(ss []string) bool {
	for i := 1; i < len(ss); i++ {
		if ss[i-1] > ss[i] {
			return false
		}
	}
	return true
}

// ─── cmdInstrumentAdoption: end-to-end CLI ─────────────────────────────────

func TestCmdInstrumentAdoption_EndToEnd(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "workspace", "logs", "mcp-calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
		{TS: "2026-07-23T10:01:00Z", Tool: "brief", Surface: "agent", ClaimsAttached: 2},
		{TS: "2026-07-23T10:02:00Z", Tool: "brief", Surface: "agent"},
	})
	writeObservationsFixture(t, root, 2, 1) // 2 open, 1 resolved

	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"fathom instrument --adoption",
		logPath,
		"find_symbol",
		"brief",
		"claims-attach hit rate",
		"1/2",
		"not instrumented",
		"2 open",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestAdoptionVerifyFigureMatchesLedger pins IT-11: `fathom instrument
// --adoption`'s verify-gate figure derives from the verify ledger
// (~/.fathom/verify-ledger.jsonl, FATHOM_VERIFY_LEDGER-injectable —
// verifyLedgerPath), never the call log — proven by seeding a call log
// whose OWN verify_plan/review verdict_outcome entries would produce a
// DIFFERENT count under the retired call-log-derived computation (the
// 215/4940-family double-count, O6/DQ-5), and asserting the report
// reflects ONLY the ledger's numbers, with the ledger-row count named as
// the denominator in the rendered sentence.
func TestAdoptionVerifyFigureMatchesLedger(t *testing.T) {
	root := t.TempDir()
	ledgerPath := filepath.Join(root, "verify-ledger.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", ledgerPath)

	// Three ledger runs: two failing, one clean.
	appendVerifyLedger([]string{"docs/plans/a.md"}, "svc", []verify.Report{{Tasks: []verify.TaskVerdict{
		{TaskID: "T-1", Claimed: true, Status: verify.Fail},
	}}}, verifyProvenance{})
	appendVerifyLedger([]string{"docs/plans/b.md"}, "svc", []verify.Report{{Tasks: []verify.TaskVerdict{
		{TaskID: "T-2", Claimed: true, Status: verify.Pass},
	}}}, verifyProvenance{})
	appendVerifyLedger([]string{"docs/plans/c.md"}, "svc", []verify.Report{{Tasks: []verify.TaskVerdict{
		{TaskID: "T-3", Claimed: true, Status: verify.Fail},
	}}}, verifyProvenance{})

	logPath := filepath.Join(root, "calls.jsonl")
	// A call log carrying its OWN, LARGER verdict_outcome population — if
	// this still drove the figure, the report would show a different
	// count than the ledger's 2-of-3.
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: "2026-07-10T09:00:00Z", Tool: "verify_plan", Surface: "agent", VerdictOutcome: map[string]int{"claimed": 3, "verified": 0, "failing": 3}},
		{TS: "2026-07-11T09:00:00Z", Tool: "verify_plan", Surface: "agent", VerdictOutcome: map[string]int{"claimed": 3, "verified": 0, "failing": 3}},
		{TS: "2026-07-12T09:00:00Z", Tool: "review", Surface: "cli", VerdictOutcome: map[string]int{"obligations": 1}},
	})

	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"2 gate refusal", "3 verify-ledger run", "verify-ledger.jsonl"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--json exit = %d, stderr: %s", code, stderr.String())
	}
	var report AdoptionReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.VerifyGateRefusals == nil || *report.VerifyGateRefusals != 2 {
		t.Errorf("VerifyGateRefusals = %v, want 2 (the ledger's own count, not the call log's)", report.VerifyGateRefusals)
	}
	if report.VerifyGateInstrumentedCalls != 3 {
		t.Errorf("VerifyGateInstrumentedCalls = %d, want 3 (the ledger row count)", report.VerifyGateInstrumentedCalls)
	}
	if report.VerifyGatePredatesInstrumentation != 0 {
		t.Errorf("VerifyGatePredatesInstrumentation = %d, want 0 (retired: every ledger row has always carried verdict counts)", report.VerifyGatePredatesInstrumentation)
	}
}

func TestCmdInstrumentAdoption_JSONOutput(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
	})

	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	var report AdoptionReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal JSON output: %v\noutput: %s", err, stdout.String())
	}
	if report.TotalCalls != 1 {
		t.Errorf("TotalCalls = %d, want 1", report.TotalCalls)
	}
	if report.CallLogPath != logPath {
		t.Errorf("CallLogPath = %q, want %q", report.CallLogPath, logPath)
	}
	if report.VerifyGateRefusals != nil {
		t.Errorf("VerifyGateRefusals = %v, want nil in the JSON output too", report.VerifyGateRefusals)
	}
}

// TestCmdInstrumentAdoption_ReportsSkippedMalformedLines: when the log had
// undecodable lines, both output forms must SAY so (H2) — a truncated
// measurement rendered without a marker would launder a partial log into a
// confident number.
func TestCmdInstrumentAdoption_ReportsSkippedMalformedLines(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	content := `{"ts":"2026-07-23T10:00:00Z","tool":"find_symbol"}
torn line
{"ts":"2026-07-23T10:02:00Z","tool":"brief"}
`
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "skipped 1 malformed line") {
		t.Errorf("human output missing the skipped-lines note:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "(2 calls)") {
		t.Errorf("human output should still count both well-formed lines:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--json exit = %d, stderr: %s", code, stderr.String())
	}
	var report AdoptionReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report.SkippedMalformedLines != 1 {
		t.Errorf("SkippedMalformedLines = %d, want 1", report.SkippedMalformedLines)
	}
	if report.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2", report.TotalCalls)
	}
}

func TestCmdInstrumentAdoption_MissingCallLogIsHonestZero(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", filepath.Join(root, "nope.jsonl"), "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("missing call log must still exit 0 (advisory), got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "not found") {
		t.Errorf("output should say the log was not found: %s", stdout.String())
	}
}

func TestCmdInstrumentAdoption_ZeroCallToolsListsUncalledRegisteredTool(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
	})
	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	// "status" is always registered and was not in the fixture log.
	if !strings.Contains(stdout.String(), "status") {
		t.Errorf("output should list status among zero-call tools:\n%s", stdout.String())
	}
}

func TestCmdInstrumentAdoption_NoObservationsFileIsHonestlyUnknown(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	writeCallLogFixture(t, logPath, nil)
	var stdout, stderr bytes.Buffer
	code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no ") || !strings.Contains(stdout.String(), "observations.md") {
		t.Errorf("output should honestly say no observations file exists yet:\n%s", stdout.String())
	}
}

// TestInstrument_AdoptionFlagDispatchesViaTopLevelRun proves the --adoption
// peel-off in cmdInstrument (instrument.go) actually reaches
// cmdInstrumentAdoption through the real "fathom instrument" entrypoint,
// not just when called directly.
func TestInstrument_AdoptionFlagDispatchesViaTopLevelRun(t *testing.T) {
	isolateVerifyLedger(t)
	root := t.TempDir()
	logPath := filepath.Join(root, "calls.jsonl")
	writeCallLogFixture(t, logPath, []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
	})
	var stdout, stderr bytes.Buffer
	code := run([]string{"instrument", "--adoption", "--call-log", logPath, "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fathom instrument --adoption") {
		t.Errorf("output = %s, want the adoption report, not the hooks-beat report", stdout.String())
	}
}

// TestInstrument_WithoutAdoptionFlagStaysOnHooksReport is the regression
// guard for the peel-off: plain "instrument --log ..." (no --adoption) must
// still produce the original hooks-beat-log report untouched.
func TestInstrument_WithoutAdoptionFlagStaysOnHooksReport(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", logPath)
	emitBeat("edit-hook", "seen", nil)
	emitBeat("edit-hook", "answered", nil)

	var stdout, stderr bytes.Buffer
	if code := cmdInstrument([]string{"--log", logPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "fathom instrument --adoption") {
		t.Errorf("plain instrument must not switch to the adoption report: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "injection coverage") {
		t.Errorf("plain instrument should still print the hooks coverage report: %s", stdout.String())
	}
}

// ─── resolveCallLogPath: config-driven default ─────────────────────────────

func TestResolveCallLogPath_DefaultsFromDaemonConfig(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeProjectConfig(t, projectDir, `
[daemon]
call_log = "custom/logs/calls.jsonl"
`)
	got := resolveCallLogPath()
	if got != "custom/logs/calls.jsonl" {
		t.Errorf("resolveCallLogPath() = %q, want the configured [daemon].call_log", got)
	}
}

func TestResolveCallLogPath_FallsBackWithNoConfig(t *testing.T) {
	withIsolatedConfigHome(t)
	got := resolveCallLogPath()
	if got != defaultCallLogPath() {
		t.Errorf("resolveCallLogPath() = %q, want the resolved default %q", got, defaultCallLogPath())
	}
}

// TestAdoptionServiceScoped pins IO-10 (O6 + the fleet audit's cwd-coupling
// defect): the adoption gauge answers "is fathom helping in repo X" FROM
// repo X — --service filters the call log (excluded unattributed rows are
// counted, never silently dropped), and the zero-call list is scoped to the
// agent surface so "use more tools" and "expose more tools" stop looking
// like the same remedy.
func TestAdoptionServiceScoped(t *testing.T) {
	t.Run("filter_counts_unattributed", func(t *testing.T) {
		entries := []adoptionCallEntry{
			{Tool: "find_symbol", Service: "app-kit"},
			{Tool: "read_source", Service: "planning"},
			{Tool: "obligations", Service: ""},
		}
		kept, unattributed := filterEntriesByService(entries, "app-kit")
		if len(kept) != 1 || kept[0].Tool != "find_symbol" {
			t.Fatalf("kept = %+v, want the one app-kit row", kept)
		}
		if unattributed != 1 {
			t.Errorf("unattributed = %d, want 1 (counted, never silently dropped)", unattributed)
		}
	})

	t.Run("zero_call_list_partitions_by_surface", func(t *testing.T) {
		registered := []string{"find_symbol", "outline", "god_object", "license"}
		agent := []string{"find_symbol", "outline"}
		counts := map[string]int{"find_symbol": 5}
		zeroAgent, notExposed := partitionZeroCalls(registered, agent, counts)
		if len(zeroAgent) != 1 || zeroAgent[0] != "outline" {
			t.Fatalf("zeroAgent = %v, want [outline]", zeroAgent)
		}
		if notExposed != 2 {
			t.Errorf("notExposed = %d, want 2 (god_object, license are registry-only)", notExposed)
		}
	})

	t.Run("cmd_scopes_report_to_service", func(t *testing.T) {
		isolateVerifyLedger(t)
		dir := t.TempDir()
		logPath := filepath.Join(dir, "calls.jsonl")
		lines := `{"ts":"2026-08-07T01:02:03Z","tool":"find_symbol","service":"app-kit","surface":"agent"}
{"ts":"2026-08-07T01:02:04Z","tool":"read_source","service":"planning","surface":"agent"}
`
		if err := os.WriteFile(logPath, []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := cmdInstrumentAdoption([]string{"--call-log", logPath, "--service", "app-kit", "--root", dir}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "app-kit") {
			t.Errorf("report does not name the scoped service:\n%s", out)
		}
		if !strings.Contains(out, "1 call") && !strings.Contains(out, "calls observed: 1") && !strings.Contains(out, "(1 ") {
			t.Errorf("report should reflect the filtered single call:\n%s", out)
		}
	})
}

// TestAdoptionCatchesNotCumulative pins FR-2 (docs/plans/exploration-ledger.md),
// re-pointed at the ledger source (IT-11): the same standing failure
// re-verified by many runs is ONE refusal per run maximum, and the
// aggregate counts refusal EVENTS — never the per-row Failing sums that
// produced the 264,487-catches artifact.
func TestAdoptionCatchesNotCumulative(t *testing.T) {
	ledgerRows := []verifyLedgerRow{
		{Plan: "a.md", Total: 50, Claimed: 50, Verified: 10, Failing: 40},
		{Plan: "a.md", Total: 50, Claimed: 50, Verified: 10, Failing: 40}, // same plan, re-verified, still failing
		{Plan: "b.md", Total: 50, Claimed: 50, Verified: 50, Failing: 0},
		{Plan: "c.md", Total: 11, Claimed: 11, Verified: 3, Failing: 8},
	}
	count, instrumented, note := gateCatchStatsFromLedger(ledgerRows)
	if count == nil || *count != 3 {
		t.Fatalf("refusals = %v, want 3 (three runs each carried at least one failing row; NEVER 88 — a sum of the Failing counts)", count)
	}
	if instrumented != 4 {
		t.Errorf("instrumented = %d, want 4", instrumented)
	}
	if !strings.Contains(note, "refusal") {
		t.Errorf("note must speak in refusal events: %q", note)
	}
}

// TestAdoptionNamesReadErrorNeverUniversalZero pins the IT-1 read-error
// classification (observation O17, filed 2026-08-23, SP-B3 docs/plans/
// surface-prune.md): a call log that is present but genuinely unreadable
// (an EPERM-class error — the external-volume TCC wedge CLAUDE.md's own
// playbook names is the common real-world instance) must never render as
// "zero-call registered tools (N of N registered)" — the exact universal-
// zero fabrication a failed read used to produce by collapsing into the
// same all-tools-empty shape a clean-but-empty log renders. ENOENT stays
// the separate, honest "no log yet" case (classifyCallLogRead's own
// contract) and is not exercised here.
func TestAdoptionNamesReadErrorNeverUniversalZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — permission bits never block root's own reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	if err := os.WriteFile(path, []byte(`{"ts":"2026-01-01T00:00:00Z","tool":"find_symbol"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) }) // let t.TempDir()'s own cleanup remove it

	entries, skipped, err := classifyCallLogRead(path)
	if err == nil {
		t.Fatalf("expected a read error for an unreadable file, got a clean read (entries=%v)", entries)
	}
	if os.IsNotExist(err) {
		t.Fatalf("expected a real (non-ENOENT) read error, got ENOENT-classified: %v", err)
	}
	if entries != nil || skipped != 0 {
		t.Errorf("classifyCallLogRead on a real error must return nil entries and 0 skipped, got entries=%v skipped=%d", entries, skipped)
	}

	registered := []string{"find_symbol", "outline", "god_object"}
	report := buildAdoptionReport(path, false, err.Error(), nil, 0, registered, "", nil, ".fathom/observations.md", nil)

	if report.CallLogReadError == "" {
		t.Fatal("report.CallLogReadError must be set for a real read error")
	}
	if len(report.ZeroCallTools) != 0 {
		t.Errorf("ZeroCallTools must stay uncomputed on a real read error (never a fabricated universal zero), got %v", report.ZeroCallTools)
	}

	var buf bytes.Buffer
	renderAdoptionReport(&buf, report)
	out := buf.String()
	universalZero := "zero-call registered tools (" +
		strconv.Itoa(len(registered)) + " of " + strconv.Itoa(len(registered)) + " registered):"
	if strings.Contains(out, universalZero) {
		t.Errorf("rendered report must never claim universal zero-call from a read error; got:\n%s", out)
	}
	if !strings.Contains(out, "NOT COMPUTED") {
		t.Errorf("rendered report must honestly say the zero-call figure was not computed; got:\n%s", out)
	}
	if !strings.Contains(out, "READ ERROR") {
		t.Errorf("rendered report must surface the read error; got:\n%s", out)
	}
}
