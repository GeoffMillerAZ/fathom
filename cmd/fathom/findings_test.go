package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

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
		{ClaimID: "near", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
			Subject: "arch_conformance#pkg.B", Rule: "near finding", Scope: []string{"pkg/b.go"}, Status: core.ClaimFresh, CompiledAt: time.Now()},
		{ClaimID: "far", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassTestGap,
			Subject: "spec_coverage#pkg.C", Rule: "far finding", Scope: []string{"pkg/c.go"}, Status: core.ClaimFresh, CompiledAt: time.Now()},
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
		{ClaimID: "fixed", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassConformance,
			Subject: "arch_conformance#pkg.A", Rule: "old finding", Scope: []string{"pkg/a.go"}, Status: core.ClaimSuperseded, CompiledAt: time.Now()},
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
		{ClaimID: "r1", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity,
			Subject: "site#1", Rule: "shared", Status: core.ClaimSuperseded, CompiledAt: t0},
		{ClaimID: "r2", ServiceID: "svc", Layer: "findings", Kind: "finding", Class: core.FindingClassSecurity,
			Subject: "site#2", Rule: "shared", Status: core.ClaimSuperseded, CompiledAt: t0},
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
