package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedFindingsStore writes obs (as symbol observations, via
// Observation.ToSymbolNode's inverse shape) and claims to a fresh on-disk
// SQLite store, closes it, and returns its path — the seed both
// cmdFindingsNear and cmdFindingsStatus tests mount read-only, mirroring
// metrics_spend_test.go's seedCompileRunsStore.
func seedFindingsStore(t *testing.T, obs []core.Observation, claims []core.SemanticClaim, events []core.ClaimEvent) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if len(obs) > 0 {
		if err := st.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
	}
	if len(claims) > 0 {
		if err := st.PutClaims(ctx, claims, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
	}
	for _, ev := range events {
		if err := st.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

// findingsFixtureObs: A (pkg/a.go) calls B (pkg/b.go) calls C (pkg/c.go).
func findingsFixtureObs() []core.Observation {
	return []core.Observation{
		{ServiceID: "svc", Package: "pkg", Symbol: "A", SourceKind: core.KindGoFunc, SourceRef: "pkg/a.go", Calls: []string{"svc::pkg.B"}},
		{ServiceID: "svc", Package: "pkg", Symbol: "B", SourceKind: core.KindGoFunc, SourceRef: "pkg/b.go", Calls: []string{"svc::pkg.C"}},
		{ServiceID: "svc", Package: "pkg", Symbol: "C", SourceKind: core.KindGoFunc, SourceRef: "pkg/c.go"},
	}
}

func TestCmdFindingsNear_JSON_RanksByProximity(t *testing.T) {
	claims := []core.SemanticClaim{
		{
			ClaimID: "near", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
			Subject: "arch_conformance#pkg.B", Rule: "near finding", Scope: []string{"pkg/b.go"}, Status: core.ClaimFresh, CompiledAt: time.Now(),
		},
		{
			ClaimID: "far", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassTestGap,
			Subject: "spec_coverage#pkg.C", Rule: "far finding", Scope: []string{"pkg/c.go"}, Status: core.ClaimFresh, CompiledAt: time.Now(),
		},
	}
	storePath := seedFindingsStore(t, findingsFixtureObs(), claims, nil)

	var out, errBuf bytes.Buffer
	code := cmdFindingsNear([]string{"--store", storePath, "--service", "svc", "--hops", "2", "--json", "svc::pkg.A"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var report findingsNearReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if !report.Found {
		t.Fatalf("Found = false, want true: %+v", report)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("Findings = %+v, want 2", report.Findings)
	}
	if report.Findings[0].ClaimID != "near" || report.Findings[1].ClaimID != "far" {
		t.Errorf("order = [%s, %s], want [near, far] (proximity ranking)", report.Findings[0].ClaimID, report.Findings[1].ClaimID)
	}
	if report.Findings[0].Hops != 1 || report.Findings[1].Hops != 2 {
		t.Errorf("hops = [%d, %d], want [1, 2]", report.Findings[0].Hops, report.Findings[1].Hops)
	}
}

func TestCmdFindingsNear_SupersededExcludedUnlessAsked(t *testing.T) {
	claims := []core.SemanticClaim{
		{
			ClaimID: "fixed", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
			Subject: "arch_conformance#pkg.A", Rule: "old finding", Scope: []string{"pkg/a.go"}, Status: core.ClaimSuperseded, CompiledAt: time.Now(),
		},
	}
	storePath := seedFindingsStore(t, findingsFixtureObs(), claims, nil)

	var out1, errBuf1 bytes.Buffer
	if code := cmdFindingsNear([]string{"--store", storePath, "--service", "svc", "--json", "svc::pkg.A"}, &out1, &errBuf1); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf1.String())
	}
	var report1 findingsNearReport
	json.Unmarshal(out1.Bytes(), &report1)
	if len(report1.Findings) != 0 {
		t.Errorf("default findings = %+v, want empty (superseded excluded)", report1.Findings)
	}

	var out2, errBuf2 bytes.Buffer
	if code := cmdFindingsNear([]string{"--store", storePath, "--service", "svc", "--include-superseded", "--json", "svc::pkg.A"}, &out2, &errBuf2); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf2.String())
	}
	var report2 findingsNearReport
	json.Unmarshal(out2.Bytes(), &report2)
	if len(report2.Findings) != 1 || !report2.Findings[0].Superseded {
		t.Errorf("--include-superseded findings = %+v, want 1 labeled superseded", report2.Findings)
	}
}

func TestCmdFindingsNear_UnresolvableSubject_HonestNote(t *testing.T) {
	storePath := seedFindingsStore(t, findingsFixtureObs(), nil, nil)
	var out, errBuf bytes.Buffer
	code := cmdFindingsNear([]string{"--store", storePath, "--service", "svc", "--json", "svc::pkg.NoSuch"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var report findingsNearReport
	json.Unmarshal(out.Bytes(), &report)
	if report.Found {
		t.Error("Found = true, want false for an unresolvable subject")
	}
	if report.Note == "" {
		t.Error("Note is empty, want an honest not-found explanation")
	}
}

func TestCmdFindingsNear_MissingFlags_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := cmdFindingsNear([]string{"x"}, &out, &errBuf); code == 0 {
		t.Error("expected non-zero exit with no --store/--service")
	}
}

func TestCmdFindingsStatus_JSON(t *testing.T) {
	t0 := time.Now()
	claims := []core.SemanticClaim{
		{
			ClaimID: "r1", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity,
			Subject: "site#1", Rule: "shared", Status: core.ClaimSuperseded, CompiledAt: t0,
		},
		{
			ClaimID: "r2", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity,
			Subject: "site#2", Rule: "shared", Status: core.ClaimSuperseded, CompiledAt: t0,
		},
	}
	events := []core.ClaimEvent{
		{ClaimID: "r1", Verdict: "superseded", At: t0, ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "c1"},
		{ClaimID: "r2", Verdict: "superseded", At: t0, ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "c2"},
	}
	storePath := seedFindingsStore(t, nil, claims, events)

	var out, errBuf bytes.Buffer
	code := cmdFindingsStatus([]string{"--store", storePath, "--service", "svc", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["resolved_finding_count"].(float64) != 2 {
		t.Errorf("resolved_finding_count = %v, want 2", payload["resolved_finding_count"])
	}
	if payload["entry_condition_met"].(bool) {
		t.Error("entry_condition_met = true, want false (2 < 15)")
	}
}

// TestCmdFindingsStatus_CandidatesAndDisagreements pins FL-23's two new
// StatusReport fields at the CLI/JSON boundary.
func TestCmdFindingsStatus_CandidatesAndDisagreements(t *testing.T) {
	t0 := time.Now()
	claims := []core.SemanticClaim{
		{ClaimID: "s1", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassCorrectness, Subject: "a", Rule: "Do a thing.", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "s2", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassCorrectness, Subject: "b", Rule: "Do a thing.", Scope: []string{"b.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "s3", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassCorrectness, Subject: "c", Rule: "Do a thing.", Scope: []string{"c.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "fresh1", ServiceID: "svc", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "arch_conformance#api.A", Scope: []string{"x.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "closed1", ServiceID: "svc", Layer: "findings", Kind: "finding", Rule: "Do X.", Subject: "ext:reviewkit.security-review:f1", Scope: []string{"x.go"}, Status: core.ClaimSuperseded, CompiledAt: t0},
	}
	events := []core.ClaimEvent{
		{ClaimID: "closed1", Verdict: "superseded", At: t0, ResolutionKind: "external-closure"},
	}
	storePath := seedFindingsStore(t, nil, claims, events)

	var out, errBuf bytes.Buffer
	code := cmdFindingsStatus([]string{"--store", storePath, "--service", "svc", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["candidates"].(float64) != 1 {
		t.Errorf("candidates = %v, want 1", payload["candidates"])
	}
	if payload["disagreements_by_rule"].(float64) != 1 {
		t.Errorf("disagreements_by_rule = %v, want 1", payload["disagreements_by_rule"])
	}
}

func TestCmdFindingsCandidates_JSON(t *testing.T) {
	t0 := time.Now()
	claims := []core.SemanticClaim{
		{ClaimID: "c1", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "ext:reviewkit.security-review:f1", Rule: "Do not log API keys in plaintext.", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "c2", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "arch_conformance#api.B", Rule: "Do not log API keys in plaintext.", Scope: []string{"b.go"}, Status: core.ClaimFresh, CompiledAt: t0},
	}
	storePath := seedFindingsStore(t, nil, claims, nil)

	var out, errBuf bytes.Buffer
	code := cmdFindingsCandidates([]string{"--store", storePath, "--service", "svc", "--min-sites", "2", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var cands []map[string]any
	if err := json.Unmarshal(out.Bytes(), &cands); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if len(cands) != 1 {
		t.Fatalf("len(cands) = %d, want 1", len(cands))
	}
	if cands[0]["class"] != "security" {
		t.Errorf("class = %v, want security", cands[0]["class"])
	}
	if cands[0]["check_family"] != "secret_literal" {
		t.Errorf("check_family = %v, want secret_literal", cands[0]["check_family"])
	}
	if cands[0]["sites"].(float64) != 2 {
		t.Errorf("sites = %v, want 2", cands[0]["sites"])
	}
}

func TestCmdFindingsCandidates_Text(t *testing.T) {
	t0 := time.Now()
	claims := []core.SemanticClaim{
		{ClaimID: "c1", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "ext:reviewkit.security-review:f1", Rule: "Do not log API keys in plaintext.", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "c2", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "arch_conformance#api.B", Rule: "Do not log API keys in plaintext.", Scope: []string{"b.go"}, Status: core.ClaimFresh, CompiledAt: t0},
	}
	storePath := seedFindingsStore(t, nil, claims, nil)

	var out, errBuf bytes.Buffer
	code := cmdFindingsCandidates([]string{"--store", storePath, "--service", "svc", "--min-sites", "2"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{"[security]", "draft: Never", "check family: secret_literal", "sites: 2"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got %s", want, got)
		}
	}
}

func TestCmdFindingsCandidates_MissingFlags_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := cmdFindingsCandidates(nil, &out, &errBuf); code == 0 {
		t.Error("expected non-zero exit with no --store/--service")
	}
}

func TestCmdFindingsStatus_MissingFlags_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := cmdFindingsStatus(nil, &out, &errBuf); code == 0 {
		t.Error("expected non-zero exit with no --store/--service")
	}
}

func TestCmdFindings_UnknownVerb_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := cmdFindings([]string{"bogus"}, &out, &errBuf); code == 0 {
		t.Error("expected non-zero exit for an unknown verb")
	}
}

func TestCmdFindings_NoVerb_ErrorExit(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := cmdFindings(nil, &out, &errBuf); code == 0 {
		t.Error("expected non-zero exit with no verb")
	}
}

// TestCmdFindingsStatus_TelemetryFromSyntheticBeatLog is SU-16's CLI-wiring
// proof: `findings status` reads the hooks beat log through the SAME
// FATHOM_HOOKS_LOG/beatLogPath resolver every other instrument uses, and
// folds RuleTelemetry into rules_never_fired_30d/top_fired.
func TestCmdFindingsStatus_TelemetryFromSyntheticBeatLog(t *testing.T) {
	t0 := time.Now()
	claims := []core.SemanticClaim{
		{ClaimID: "fired", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "a", Rule: "shared", Scope: []string{"a.go"}, Status: core.ClaimFresh, CompiledAt: t0},
		{ClaimID: "silent", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity, Subject: "b", Rule: "shared", Scope: []string{"b.go"}, Status: core.ClaimFresh, CompiledAt: t0},
	}
	storePath := seedFindingsStore(t, nil, claims, nil)
	logPath := writeBeatsFixture(t, []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "finding_touched", At: time.Now(), Meta: map[string]string{"file": "a.go", "shown": "1"}},
	})
	t.Setenv("FATHOM_HOOKS_LOG", logPath)

	var out, errBuf bytes.Buffer
	code := cmdFindingsStatus([]string{"--store", storePath, "--service", "svc", "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if payload["rules_never_fired_30d"].(float64) != 1 {
		t.Errorf("rules_never_fired_30d = %v, want 1 (only 'silent' never fired)", payload["rules_never_fired_30d"])
	}
	topFired, ok := payload["top_fired"].([]any)
	if !ok || len(topFired) != 1 {
		t.Fatalf("top_fired = %v, want one entry", payload["top_fired"])
	}
	row := topFired[0].(map[string]any)
	if row["claim_id"] != "fired" {
		t.Errorf("top_fired[0].claim_id = %v, want %q", row["claim_id"], "fired")
	}
}
