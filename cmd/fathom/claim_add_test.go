package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestClaimAdd_MintsHumanAuthoredClaim is SU-17's end-to-end proof: the CLI
// mints one content-addressed, human-authored, sme-stated-capped claim with
// an external_findings row like an import, and the store's own served-
// obligations path (what the pre-edit hook calls) shows it — a
// human-authored claim rides the same path any other finding does.
func TestClaimAdd_MintsHumanAuthoredClaim(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "add",
		"--store", storePath, "--service", "svc",
		"--scope", "internal/foo/**",
		"--rule", "Do not log secrets in plaintext.",
		"--remedy", "Redact before logging.",
		"--json",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("claim add exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), `"claim_id"`) {
		t.Errorf("json output missing claim_id: %s", out.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	claims, err := st.ClaimsForService(ctx, "svc", "findings")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d", err, len(claims))
	}
	cl := claims[0]
	if cl.Tier != core.TierSMEStated {
		t.Errorf("Tier = %q, want sme-stated (the human cap)", cl.Tier)
	}
	if cl.PromptVersion != "human-authored/v1" {
		t.Errorf("PromptVersion = %q, want human-authored/v1", cl.PromptVersion)
	}
	if cl.Kind != "finding" || cl.Layer != "findings" {
		t.Errorf("Kind/Layer = %q/%q, want finding/findings", cl.Kind, cl.Layer)
	}
	if len(cl.Scope) != 1 || cl.Scope[0] != "internal/foo/**" {
		t.Errorf("Scope = %v, want [internal/foo/**]", cl.Scope)
	}

	ef, found, err := st.ExternalFinding(ctx, "svc", cl.ClaimID)
	if err != nil || !found {
		t.Fatalf("ExternalFinding: %v / found=%v", err, found)
	}
	if ef.Producer != "human.review" || ef.SourceVersion != 1 {
		t.Errorf("ExternalFinding producer/sourceVersion = %q/%d, want human.review/1", ef.Producer, ef.SourceVersion)
	}
	if !strings.HasPrefix(ef.SourceID, "human:") || len(ef.SourceID) != len("human:")+12 {
		t.Errorf("SourceID = %q, want \"human:\" + 12 hex chars", ef.SourceID)
	}

	// Served by the same obligations path the pre-edit hook calls
	// (edithook.go's fetchObligations -> semantic.ObligationsForLayer with
	// ObligationLayerAll) — a human-authored finding rides it exactly like
	// any other finding-kind claim.
	obl, err := semantic.ObligationsForLayer(ctx, st, "svc", "internal/foo/bar.go", 10, semantic.ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	served := false
	for _, r := range obl.Rules {
		if r.Rule == "Do not log secrets in plaintext." {
			served = true
			if r.Kind != "finding" {
				t.Errorf("served row Kind = %q, want finding", r.Kind)
			}
			if r.Tier != string(core.TierSMEStated) {
				t.Errorf("served row Tier = %q, want sme-stated", r.Tier)
			}
		}
	}
	if !served {
		t.Errorf("human-authored claim not served: %+v", obl.Rules)
	}
}

// TestClaimAdd_TierFlagRefused pins SU-17's cap: --tier is never settable,
// even to a value at or below sme-stated — the flag itself is refused.
func TestClaimAdd_TierFlagRefused(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "add",
		"--store", storePath, "--service", "svc",
		"--scope", "a.go", "--rule", "x", "--tier", "verified",
	}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected non-zero exit when --tier is given")
	}
	if !strings.Contains(errBuf.String(), "--tier is not settable") {
		t.Errorf("stderr = %q, want a named refusal of --tier", errBuf.String())
	}
}

// TestClaimAdd_ContentAddressed_SameInputSameClaimID proves two identical
// `claim add` calls mint the SAME claim id (content-addressed, like every
// other claim) — the second call is an idempotent PutClaims upsert, not a
// duplicate.
func TestClaimAdd_ContentAddressed_SameInputSameClaimID(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	args := []string{
		"claim", "add",
		"--store", storePath, "--service", "svc",
		"--scope", "a.go", "--rule", "Always do X.", "--json",
	}
	var out1, out2, errBuf bytes.Buffer
	if code := run(args, &out1, &errBuf); code != 0 {
		t.Fatalf("first add exit = %d: %s", code, errBuf.String())
	}
	errBuf.Reset()
	if code := run(args, &out2, &errBuf); code != 0 {
		t.Fatalf("second add exit = %d: %s", code, errBuf.String())
	}
	var r1, r2 struct {
		ClaimID string `json:"claim_id"`
	}
	if err := json.Unmarshal(out1.Bytes(), &r1); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if err := json.Unmarshal(out2.Bytes(), &r2); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if r1.ClaimID == "" || r1.ClaimID != r2.ClaimID {
		t.Errorf("ClaimID = %q vs %q, want identical (content-addressed)", r1.ClaimID, r2.ClaimID)
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer st.Close()
	claims, err := st.ClaimsForService(context.Background(), "svc", "findings")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d, want exactly 1 (upsert, not duplicate)", err, len(claims))
	}
}

// TestClaimAdd_MissingFlags_ErrorExit checks the required-flag refusals.
func TestClaimAdd_MissingFlags_ErrorExit(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	cases := []struct {
		name string
		args []string
	}{
		{"no scope", []string{"claim", "add", "--store", storePath, "--service", "svc", "--rule", "x"}},
		{"no rule", []string{"claim", "add", "--store", storePath, "--service", "svc", "--scope", "a.go"}},
		{"no service", []string{"claim", "add", "--store", storePath, "--scope", "a.go", "--rule", "x"}},
		{"no store", []string{"claim", "add", "--service", "svc", "--scope", "a.go", "--rule", "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			if code := run(tc.args, &out, &errBuf); code == 0 {
				t.Errorf("exit = 0, want non-zero; stderr: %s", errBuf.String())
			}
		})
	}
}
