package hitl_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

// A tiny realistic domain: an account with a running balance + a note, mutated
// by a bounded op set. Mirrors the shape of a real app (gg-bi's finance.EditOp).

type acct struct {
	Balance int
	Note    string
}

type op struct {
	Kind   string
	Amount int
	Note   string
}

func apply(st *acct, o op) (string, error) {
	switch o.Kind {
	case "deposit":
		st.Balance += o.Amount
		return fmt.Sprintf("Deposit $%d", o.Amount), nil
	case "withdraw":
		if o.Amount > st.Balance {
			return "", errors.New("insufficient funds")
		}
		st.Balance -= o.Amount
		return fmt.Sprintf("Withdraw $%d", o.Amount), nil
	case "note":
		st.Note = o.Note
		return "Set note", nil
	}
	return "", fmt.Errorf("unknown op %q", o.Kind)
}

func cloneAcct(a acct) acct { return a } // value type: assignment is a full copy

func gauges(a acct) []hitl.Gauge {
	return []hitl.Gauge{{Key: "bal", Label: "Balance", Value: fmt.Sprintf("$%d", a.Balance)}}
}

func newEngine(allowed ...string) *hitl.Engine[acct, op] {
	set := hitl.NewOpSet(allowed...)
	return hitl.New(
		apply, cloneAcct,
		hitl.WithGauges[acct, op](gauges),
		hitl.WithGuard[acct, op](hitl.Guard(set, func(o op) string { return o.Kind })),
	)
}

func TestPreview_DoesNotMutate_AndDiffs(t *testing.T) {
	eng := newEngine("deposit", "withdraw")
	live := acct{Balance: 100}

	pv := eng.Preview(live, op{Kind: "deposit", Amount: 50})
	if !pv.OK || pv.Err != "" {
		t.Fatalf("preview not ok: %+v", pv)
	}
	if pv.Summary != "Deposit $50" {
		t.Errorf("summary = %q", pv.Summary)
	}
	want := []hitl.Change{{Key: "bal", Label: "Balance", From: "$100", To: "$150"}}
	if !reflect.DeepEqual(pv.Changes, want) {
		t.Errorf("changes = %+v, want %+v", pv.Changes, want)
	}
	if live.Balance != 100 {
		t.Errorf("preview mutated live state: balance = %d", live.Balance)
	}
}

func TestApply_CommitsInPlace(t *testing.T) {
	eng := newEngine("deposit", "withdraw")
	live := acct{Balance: 100}
	sum, err := eng.Apply(&live, op{Kind: "withdraw", Amount: 30})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if sum != "Withdraw $30" || live.Balance != 70 {
		t.Errorf("apply: sum=%q balance=%d", sum, live.Balance)
	}
}

// The load-bearing invariant: the summary a human approves in the preview is the
// exact summary the commit produces, because both run the same Applier.
func TestPreviewAndApply_SameSummary(t *testing.T) {
	eng := newEngine("deposit")
	live := acct{Balance: 10}
	o := op{Kind: "deposit", Amount: 5}
	pv := eng.Preview(live, o)
	sum, err := eng.Apply(&live, o)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Summary != sum {
		t.Errorf("preview summary %q != apply summary %q", pv.Summary, sum)
	}
}

func TestPreview_ApplierError_Surfaced_NoMutation(t *testing.T) {
	eng := newEngine("withdraw")
	live := acct{Balance: 20}
	pv := eng.Preview(live, op{Kind: "withdraw", Amount: 100})
	if pv.OK || pv.Err != "insufficient funds" {
		t.Errorf("expected surfaced error, got %+v", pv)
	}
	if live.Balance != 20 {
		t.Errorf("state mutated on failed preview: %d", live.Balance)
	}
}

func TestGuard_RejectsNonWhitelistedOp(t *testing.T) {
	eng := newEngine("deposit", "withdraw") // "note" NOT allowed
	live := acct{Balance: 100}

	pv := eng.Preview(live, op{Kind: "note", Note: "hi"})
	if pv.OK || pv.Err == "" {
		t.Errorf("preview should reject non-whitelisted op: %+v", pv)
	}
	if _, err := eng.Apply(&live, op{Kind: "note", Note: "hi"}); err == nil {
		t.Error("apply should reject non-whitelisted op")
	}
	if live.Note != "" {
		t.Error("rejected op still mutated state")
	}
}

func TestPreviewAll(t *testing.T) {
	eng := newEngine("deposit", "withdraw")
	live := acct{Balance: 100}
	pvs := eng.PreviewAll(live, []op{
		{Kind: "deposit", Amount: 10},
		{Kind: "withdraw", Amount: 999}, // fails
	})
	if len(pvs) != 2 || !pvs[0].OK || pvs[1].OK {
		t.Errorf("previewAll = %+v", pvs)
	}
	if live.Balance != 100 {
		t.Errorf("previewAll mutated live state: %d", live.Balance)
	}
}

func TestOpSet_KindsFeedSchemaAndWhitelist(t *testing.T) {
	set := hitl.NewOpSet("a", "b", "c")
	if got := set.Kinds(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("Kinds() = %v (must be declaration order for a deterministic enum)", got)
	}
	if !set.Allows("b") || set.Allows("z") {
		t.Error("Allows() disagrees with the whitelist")
	}
}

func TestNewOpSet_PanicsOnDuplicate(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate kind")
		}
	}()
	hitl.NewOpSet("x", "x")
}

func TestDiff_UnchangedGaugeOmittedNoGaugeFuncNoChanges(t *testing.T) {
	// An engine without gauges still previews (summary only, no deltas).
	eng := hitl.New(apply, cloneAcct)
	pv := eng.Preview(acct{Balance: 1}, op{Kind: "deposit", Amount: 0})
	if !pv.OK || len(pv.Changes) != 0 {
		t.Errorf("no-gauge / no-op preview should have no changes: %+v", pv)
	}
}
