package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func seedClaim(t *testing.T, storePath, service, claimID string) {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: service,
		Layer: "findings", LayerVersion: "v1", Kind: "finding",
		Subject: "findings/wa-001.md#remediation", Rule: "Add retry with backoff.",
		Scope: []string{"mod/**"}, Tier: core.TierSMEStated, TierProfile: "engineering",
		Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
}

func TestClaim_Refute_FlipsStatusAndAppendsEvent(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, "wa-review", "cl-1")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "refute",
		"--store", storePath, "--service", "wa-review", "--claim", "cl-1",
		"--note", "already fixed upstream", "--author", "alice",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("claim refute exit = %d, stderr: %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "refuted") {
		t.Errorf("output %q missing verdict", out.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	claims, err := st.ClaimsForService(ctx, "wa-review", "")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d", err, len(claims))
	}
	if claims[0].Status != core.ClaimRefuted {
		t.Errorf("status = %q, want refuted", claims[0].Status)
	}

	events, err := st.ClaimEvents(ctx, "cl-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("ClaimEvents: %v / %d", err, len(events))
	}
	if events[0].Verdict != "refuted" || events[0].Author != "alice" || events[0].Note != "already fixed upstream" {
		t.Errorf("event = %+v", events[0])
	}
}

func TestClaim_Corroborate_NeverPromotesStatus(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, "wa-review", "cl-2")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"claim", "corroborate",
		"--store", storePath, "--service", "wa-review", "--claim", "cl-2",
		"--note", "confirmed in review",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("claim corroborate exit = %d, stderr: %s", code, errBuf.String())
	}

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	claims, err := st.ClaimsForService(ctx, "wa-review", "")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d", err, len(claims))
	}
	// corroboration is provenance, not evidence-laundering: status (and tier)
	// must be untouched.
	if claims[0].Status != core.ClaimFresh {
		t.Errorf("status = %q, want unchanged (fresh) — corroborate must never flip status", claims[0].Status)
	}
	if claims[0].Tier != core.TierSMEStated {
		t.Errorf("tier = %q, want unchanged", claims[0].Tier)
	}

	events, err := st.ClaimEvents(ctx, "cl-2")
	if err != nil || len(events) != 1 {
		t.Fatalf("ClaimEvents: %v / %d", err, len(events))
	}
	if events[0].Verdict != "corroborated" {
		t.Errorf("verdict = %q, want corroborated", events[0].Verdict)
	}
}

// TestClaim_RefuteAndCorroborate_RecordRuleOutcome is GL-10's wiring proof:
// design doc §5 names the existing refuted/corroborated claim verdicts as
// two of the six event kinds the signal source records, so `fathom claim
// refute`/`corroborate` must feed recordRuleOutcome exactly like the edit
// and PreToolUse hooks do — not a second, disconnected verdict channel.
func TestClaim_RefuteAndCorroborate_RecordRuleOutcome(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, "wa-review", "cl-3")
	seedClaim(t, storePath, "wa-review", "cl-4")

	logPath := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", logPath)

	var out, errBuf bytes.Buffer
	if code := run([]string{
		"claim", "refute",
		"--store", storePath, "--service", "wa-review", "--claim", "cl-3",
		"--note", "wrong", "--author", "alice",
	}, &out, &errBuf); code != 0 {
		t.Fatalf("claim refute exit = %d, stderr: %s", code, errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{
		"claim", "corroborate",
		"--store", storePath, "--service", "wa-review", "--claim", "cl-4",
		"--note", "confirmed",
	}, &out, &errBuf); code != 0 {
		t.Fatalf("claim corroborate exit = %d, stderr: %s", code, errBuf.String())
	}

	rows := readRuleOutcomeRows(t, logPath)
	if len(rows) != 2 {
		t.Fatalf("want 2 recorded rule-outcome rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].RuleID != "cl-3" || rows[0].Event != ruleOutcomeRefuted {
		t.Errorf("row1 = %+v, want rule_id=cl-3 event=refuted", rows[0])
	}
	if rows[1].RuleID != "cl-4" || rows[1].Event != ruleOutcomeCorroborated {
		t.Errorf("row2 = %+v, want rule_id=cl-4 event=corroborated", rows[1])
	}
}

func TestClaim_Errors(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedClaim(t, storePath, "wa-review", "cl-3")

	cases := []struct {
		name string
		args []string
	}{
		{"no verb", []string{"claim"}},
		{"unknown verb", []string{"claim", "delete", "--store", storePath, "--service", "wa-review", "--claim", "cl-3", "--note", "x"}},
		{"missing note", []string{"claim", "refute", "--store", storePath, "--service", "wa-review", "--claim", "cl-3"}},
		{"missing store", []string{"claim", "refute", "--service", "wa-review", "--claim", "cl-3", "--note", "x"}},
		{"unknown claim", []string{"claim", "refute", "--store", storePath, "--service", "wa-review", "--claim", "no-such-claim", "--note", "x"}},
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
