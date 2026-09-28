package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// writeDebtRows seeds the intent ledger with debt rows for root.
func writeDebtRows(t *testing.T, rows ...intent.LedgerRow) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "intent-ledger.jsonl")
	t.Setenv("FATHOM_INTENT_LEDGER", path)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range rows {
		r.SchemaVersion = intent.LedgerSchemaVersion
		r.Kind = intent.KindDebt
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
}

func debtRow(root, at string, byKind map[string]int) intent.LedgerRow {
	return intent.LedgerRow{RepoRoot: root, At: at, ByKind: byKind}
}

// EP-19: the intent half EXTENDS the custodian's line rather than adding a
// second one, and its two counts are disjoint populations that add up.
func TestAttentionLineExtendedNotDoubled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	root := t.TempDir()
	writeConcernFixture(t, root) // 1 violated concern
	writeDebtRows(
		t,
		debtRow(root, now.Add(-72*time.Hour).Format(time.RFC3339), map[string]int{"U2": 4}),
		debtRow(root, now.Add(-time.Minute).Format(time.RFC3339), map[string]int{"U2": 6, "U11": 2, "U5": 1}),
	)

	line := attentionLine(root, now)
	if strings.Count(line, "\n") != 0 {
		t.Fatalf("the attention surface is ONE line, got %q", line)
	}
	// config 1 · unproven 8 (+4) · fired 1 — U5 is not counted in `unproven`.
	for _, want := range []string{"config 1", "unproven 8", "(+4)", "fired 1", "fathom doctor"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
	if len(line) > attentionLineBudget {
		t.Fatalf("line is %d B, over the %d B budget: %q", len(line), attentionLineBudget, line)
	}
}

// TestLearningAttentionFragment is GL-14: a promotion/retirement recorded
// within learningAttentionWindow renders as `learning N`, joins the SAME
// shared line as config/intent (extended, not doubled — mirrors
// TestAttentionLineExtendedNotDoubled above), and a row outside the window
// or for a different repo root is silent.
func TestLearningAttentionFragment(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	other := t.TempDir()

	t.Run("nothing recorded: silent", func(t *testing.T) {
		t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))
		if got := learningAttentionFragment(root, now); got != "" {
			t.Fatalf("want silence, got %q", got)
		}
	})

	t.Run("recorded within the window: counted", func(t *testing.T) {
		t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(t.TempDir(), "l.jsonl"))
		_ = intent.AppendLedger(intent.LedgerRow{
			Kind: intent.KindLearning, RepoRoot: root, ID: "cl-1", Verdict: "active",
			Evidence: "3 corroborations from 2 sessions", At: now.Add(-time.Hour).Format(time.RFC3339),
		})
		_ = intent.AppendLedger(intent.LedgerRow{
			Kind: intent.KindLearning, RepoRoot: root, ID: "cl-2", Verdict: "retired",
			Evidence: "never triggered for 180d", At: now.Add(-2 * time.Hour).Format(time.RFC3339),
		})
		// Outside the window and for a different repo root — neither counts.
		_ = intent.AppendLedger(intent.LedgerRow{
			Kind: intent.KindLearning, RepoRoot: root, ID: "cl-3", Verdict: "retired",
			At: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		})
		_ = intent.AppendLedger(intent.LedgerRow{
			Kind: intent.KindLearning, RepoRoot: other, ID: "cl-4", Verdict: "active",
			At: now.Add(-time.Hour).Format(time.RFC3339),
		})

		if got := learningAttentionFragment(root, now); got != "learning 2" {
			t.Fatalf("fragment = %q, want %q", got, "learning 2")
		}

		writeConcernFixture(t, root) // 1 violated concern, so the line isn't empty either way
		line := attentionLine(root, now)
		if !strings.Contains(line, "learning 2") {
			t.Fatalf("attentionLine = %q, missing the learning fragment", line)
		}
		if !strings.Contains(line, "config 1") {
			t.Fatalf("attentionLine = %q, the learning fragment must EXTEND the line, not replace it", line)
		}
		if strings.Count(line, "\n") != 0 {
			t.Fatalf("the attention surface is ONE line, got %q", line)
		}
	})
}

// A recorded audit that found nothing, a repo with no recorded audit, and an
// audit too old to be evidence all print NOTHING — the line exists only when
// there is something true to say.
func TestIntentAttentionFragmentSilentWhenNothingToSay(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	other := t.TempDir()

	cases := []struct {
		name string
		rows []intent.LedgerRow
	}{
		{"no ledger at all", nil},
		{"no row for this repo", []intent.LedgerRow{debtRow(other, now.Format(time.RFC3339), map[string]int{"U2": 9})}},
		{"clean audit", []intent.LedgerRow{debtRow(root, now.Format(time.RFC3339), map[string]int{})}},
		{"audit older than the max age", []intent.LedgerRow{debtRow(root, now.Add(-30*24*time.Hour).Format(time.RFC3339), map[string]int{"U2": 9})}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeDebtRows(t, tc.rows...)
			if got := intentAttentionFragment(root, now); got != "" {
				t.Fatalf("want silence, got %q", got)
			}
		})
	}
}

// The delta is only spoken when debt ROSE, and only against a row from
// before the current session. Two back-to-back audits must not congratulate
// themselves with a "+0", and a fall must not render as a negative.
func TestDebtDeltaOnlyOnARiseAgainstAPriorSession(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()

	writeDebtRows(
		t,
		debtRow(root, now.Add(-48*time.Hour).Format(time.RFC3339), map[string]int{"U2": 9}),
		debtRow(root, now.Add(-time.Minute).Format(time.RFC3339), map[string]int{"U2": 3}),
	)
	if got := intentAttentionFragment(root, now); got != "unproven 3" {
		t.Fatalf("a fall must print no delta, got %q", got)
	}

	writeDebtRows(
		t,
		debtRow(root, now.Add(-2*time.Hour).Format(time.RFC3339), map[string]int{"U2": 1}),
		debtRow(root, now.Add(-time.Minute).Format(time.RFC3339), map[string]int{"U2": 5}),
	)
	if got := intentAttentionFragment(root, now); got != "unproven 5" {
		t.Fatalf("a baseline inside the session window is not a baseline, got %q", got)
	}
}

// EP-20: the whole primer is capped, the cut is NAMED, and the byte count
// returned is what was actually injected — not what was composed.
func TestWholePrimerCap(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	// A thread brief far over the whole-primer budget.
	threads := filepath.Join(t.TempDir(), "threads.jsonl")
	t.Setenv("FATHOM_THREADS", threads)
	f, err := os.Create(threads)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for i := 0; i < 40; i++ {
		_ = enc.Encode(map[string]any{
			"schema_version": 1,
			"id":             fmt.Sprintf("TH-%02d", i),
			"thread":         "handoff",
			"kind":           "note",
			"text":           strings.Repeat("a long handoff note that eats budget ", 4),
			"filed_at":       now.Format(time.RFC3339),
			"pointers":       map[string]string{"repo_root": root},
		})
	}
	f.Close()

	// A realistic long identity line: the daemon-down message carries a URL
	// and a launchctl remedy. Identity + a full 2,400 B brief + the
	// attention line is what pushes the composition over the cap.
	identity := "⚠️ fathom code-intelligence daemon (http://localhost:7581) is not responding — " + strings.Repeat("its MCP tools are unavailable this session, so Grep/Read discovery is expected. ", 12)
	primer, briefBytes, _ := composeContinuityAndCustodian(identity, root, now)

	if len(primer) > sessionPrimerBudget {
		t.Fatalf("primer is %d B, over the %d B cap", len(primer), sessionPrimerBudget)
	}
	if !strings.HasPrefix(primer, identity) {
		t.Fatalf("the identity line is never dropped, got %q", primer[:60])
	}
	if !strings.Contains(primer, "cut to fit") {
		t.Fatalf("a silent cut is a lie (law 3); primer tail: %q", primer[len(primer)-200:])
	}
	if briefBytes > len(primer) {
		t.Fatalf("briefBytes %d exceeds the whole primer %d — post-cut bytes are what was injected", briefBytes, len(primer))
	}
}

// A primer that fits is returned untouched: no cut notice, no lost lines.
func TestPrimerUnderBudgetIsUntouched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_THREADS", filepath.Join(t.TempDir(), "threads.jsonl"))
	root := t.TempDir()
	identity := "✅ fathom is live."
	primer, briefBytes, attention := composeContinuityAndCustodian(identity, root, time.Now())
	if primer != identity {
		t.Fatalf("a clean repo adds nothing to the identity line, got %q", primer)
	}
	if briefBytes != 0 || attention != "" {
		t.Fatalf("nothing to report must measure as nothing: bytes=%d attention=%q", briefBytes, attention)
	}
}

// The cap is a BACKSTOP, and this test says so out loud: today the parts'
// own budgets sum below it, so the cap binds only when a part overruns or a
// fourth part appears. If someone raises the brief's budget or adds a
// section, this arithmetic fails and tells them the cap is now load-bearing
// — which is the moment to re-read EP-20 rather than bump a number.
func TestPrimerPartBudgetsSumUnderCap(t *testing.T) {
	const typicalIdentityBytes = 400                                                                        // the healthy-daemon line, measured
	sum := typicalIdentityBytes + threadBriefBudgetBytes + attentionLineBudget + tenancyAttentionBudget + 3 // + the joining newlines
	if sum > sessionPrimerBudget {
		t.Fatalf("the parts now sum to %d B against a %d B cap — the cap is load-bearing, not a backstop; re-read EP-20 before changing it", sum, sessionPrimerBudget)
	}
}

// EP-24: the attention line's bytes are a COMPONENT of the primer's
// payload-cost beat — inside the total, never a second row added to it, or
// the rent model prices the same bytes twice.
func TestAttentionBytesRideAsAComponentNotASecondChannel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(dir, "beats.jsonl"))

	payload := "identity line\n⚠️ attention: config 1 — fathom doctor"
	emitPayloadCost("session-brief", payload, 0, "", "sess-1", map[string]string{"attention_bytes": "41"})

	raw, err := os.ReadFile(filepath.Join(dir, "beats.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var beat struct {
		Channel string            `json:"channel"`
		Kind    string            `json:"kind"`
		Meta    map[string]string `json:"meta"`
	}
	line := strings.TrimSpace(strings.Split(strings.TrimSpace(string(raw)), "\n")[0])
	if err := json.Unmarshal([]byte(line), &beat); err != nil {
		t.Fatalf("%v: %s", err, line)
	}
	if beat.Channel != "payload-cost" || beat.Kind != "session-brief" {
		t.Fatalf("the component must NOT become its own channel: %s/%s", beat.Channel, beat.Kind)
	}
	if beat.Meta["attention_bytes"] != "41" {
		t.Fatalf("component bytes missing from meta: %v", beat.Meta)
	}
	total, convErr := strconv.Atoi(beat.Meta["bytes"])
	if convErr != nil || total != len(payload) {
		t.Fatalf("the channel total must still be the WHOLE payload: %q vs %d", beat.Meta["bytes"], len(payload))
	}
	if 41 > total {
		t.Fatal("a component larger than its container is a double-count, not a breakdown")
	}
}
