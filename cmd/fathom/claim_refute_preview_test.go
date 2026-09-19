package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedRefuteFixture writes one served claim scoped to "mod/**" plus three
// indexed observations under mod/ (two matching the scope, one outside it)
// to a fresh store, and returns its path.
func seedRefuteFixture(t *testing.T) (storePath, service, claimID string) {
	t.Helper()
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	service, claimID = "svc", "cl-preview"
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: service,
		Layer: "findings", LayerVersion: "v1", Kind: "finding",
		Subject: "findings/wa-001.md#remediation", Rule: "Add retry with backoff.",
		Scope: []string{"mod/**"}, Tier: core.TierSMEStated, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	obs := []core.Observation{
		{ServiceID: service, Package: "mod", Symbol: "A", SourceKind: core.KindGoFunc, SourceRef: "mod/a.go"},
		{ServiceID: service, Package: "mod", Symbol: "B", SourceKind: core.KindGoFunc, SourceRef: "mod/b.go"},
		{ServiceID: service, Package: "other", Symbol: "C", SourceKind: core.KindGoFunc, SourceRef: "other/c.go"},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	return storePath, service, claimID
}

func TestClaimRefute_DryRun_CountsScopedFiles_JSON(t *testing.T) {
	storePath, service, claimID := seedRefuteFixture(t)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute", "--store", storePath, "--service", service, "--claim", claimID, "--dry-run", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var got refuteDryRunResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out.String())
	}
	if got.ClaimID != claimID {
		t.Errorf("ClaimID = %q, want %q", got.ClaimID, claimID)
	}
	if got.WouldWithhold != 2 {
		t.Errorf("WouldWithhold = %d, want 2 (mod/a.go, mod/b.go — other/c.go is out of scope)", got.WouldWithhold)
	}
	if got.Truncated {
		t.Error("Truncated = true, want false (only 2 files, cap is 5)")
	}
	want := []string{"mod/a.go", "mod/b.go"}
	if !reflect.DeepEqual(got.Files, want) {
		t.Errorf("Files = %v, want %v", got.Files, want)
	}
}

func TestClaimRefute_DryRun_TextRender(t *testing.T) {
	storePath, service, claimID := seedRefuteFixture(t)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute", "--store", storePath, "--service", service, "--claim", claimID, "--dry-run",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{"would withhold 2 served row(s) across 2 file(s)", "mod/a.go", "mod/b.go"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q, got %q", want, got)
		}
	}
}

func TestClaimRefute_DryRun_Truncation(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	service, claimID := "svc", "cl-many"
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: service, Layer: "findings", Kind: "finding",
		Subject: "s", Rule: "r", Scope: []string{"mod/**"}, Tier: core.TierSMEStated,
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	var obs []core.Observation
	for i := 0; i < 8; i++ {
		obs = append(obs, core.Observation{
			ServiceID: service, Package: "mod", Symbol: string(rune('A' + i)),
			SourceKind: core.KindGoFunc, SourceRef: "mod/" + string(rune('a'+i)) + ".go",
		})
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute", "--store", storePath, "--service", service, "--claim", claimID, "--dry-run", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var got refuteDryRunResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.WouldWithhold != 8 {
		t.Errorf("WouldWithhold = %d, want 8 (full count, never truncated)", got.WouldWithhold)
	}
	if !got.Truncated || len(got.Files) != 5 {
		t.Errorf("Truncated/len(Files) = %v/%d, want true/5", got.Truncated, len(got.Files))
	}
}

// TestClaimRefute_DryRun_AlreadyNotServed_WithholdsNothing proves a claim
// that is not currently served (already refuted) previews as withholding
// zero — refuting it again changes nothing further.
func TestClaimRefute_DryRun_AlreadyNotServed_WithholdsNothing(t *testing.T) {
	storePath, service, claimID := seedRefuteFixture(t)
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := st.SetClaimStatus(context.Background(), service, claimID, core.ClaimRefuted); err != nil {
		t.Fatalf("SetClaimStatus: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute", "--store", storePath, "--service", service, "--claim", claimID, "--dry-run", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}
	var got refuteDryRunResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.WouldWithhold != 0 {
		t.Errorf("WouldWithhold = %d, want 0 (already refuted, nothing left to withhold)", got.WouldWithhold)
	}
}

// TestClaimRefute_DryRun_CorroborateRefused proves --dry-run is refused for
// corroborate (nothing to preview).
func TestClaimRefute_DryRun_CorroborateRefused(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, "wa-review", "cl-x")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "corroborate", "--store", storePath, "--service", "wa-review", "--claim", "cl-x", "--dry-run",
	}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit: --dry-run is refute-only")
	}
	if !strings.Contains(errBuf.String(), "only supported for claim refute") {
		t.Errorf("stderr = %q, want a named refusal", errBuf.String())
	}
}

// snapshotClaimState reads back everything a dry-run touches (the claim
// rows for service, and this claim's own events) — the store-state
// equality proof SU-18 requires: nothing written by --dry-run.
func snapshotClaimState(t *testing.T, storePath, service, claimID string) ([]core.SemanticClaim, []core.ClaimEvent) {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	claims, err := st.ClaimsForService(ctx, service, "")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	events, err := st.ClaimEvents(ctx, claimID)
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	return claims, events
}

// TestClaimRefute_DryRun_WritesNothing is SU-18's store-state equality
// proof: the store's claim rows and this claim's events are byte-for-byte
// identical before and after a --dry-run call.
func TestClaimRefute_DryRun_WritesNothing(t *testing.T) {
	storePath, service, claimID := seedRefuteFixture(t)

	beforeClaims, beforeEvents := snapshotClaimState(t, storePath, service, claimID)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute", "--store", storePath, "--service", service, "--claim", claimID, "--dry-run", "--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errBuf.String())
	}

	afterClaims, afterEvents := snapshotClaimState(t, storePath, service, claimID)

	if !reflect.DeepEqual(beforeClaims, afterClaims) {
		t.Errorf("claims changed by --dry-run:\nbefore: %+v\nafter:  %+v", beforeClaims, afterClaims)
	}
	if !reflect.DeepEqual(beforeEvents, afterEvents) {
		t.Errorf("claim_events changed by --dry-run:\nbefore: %+v\nafter:  %+v", beforeEvents, afterEvents)
	}
	if len(afterEvents) != 0 {
		t.Errorf("len(afterEvents) = %d, want 0 (no real refute ever ran)", len(afterEvents))
	}
}
