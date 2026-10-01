package decision_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

func mockRequest(state string) decision.Request {
	return decision.Request{State: state, Questions: []decision.Question{validNoulQuestion("q1")}}
}

func TestMockDecider_ScriptedEntriesInOrder(t *testing.T) {
	boom := errors.New("scripted failure")
	m := decision.NewMockDecider("mock",
		decision.MockEntry{Response: decision.Response{Provider: "first", Answers: map[string]decision.Answer{"q1": {ID: "q1", Kind: decision.KindPredicate, Noul: 0.9}}}},
		decision.MockEntry{Err: boom, Response: decision.Response{Provider: "ignored when Err is set"}},
	)

	if got := m.Name(); got != "mock" {
		t.Errorf("Name() = %q, want %q", got, "mock")
	}

	first, err := m.Decide(context.Background(), mockRequest("one"))
	if err != nil {
		t.Fatalf("first Decide() error = %v", err)
	}
	if first.Provider != "first" || first.Answers["q1"].Noul != 0.9 {
		t.Errorf("first Decide() = %+v, want the first scripted response", first)
	}

	second, err := m.Decide(context.Background(), mockRequest("two"))
	if !errors.Is(err, boom) {
		t.Fatalf("second Decide() error = %v, want the scripted error", err)
	}
	if second.Provider != "ignored when Err is set" {
		t.Errorf("second Decide() Response.Provider = %q, want the scripted response returned beside the error", second.Provider)
	}
}

func TestMockDecider_ExhaustedEntriesReturnEmptyValidBatch(t *testing.T) {
	m := decision.NewMockDecider("mock")

	resp, err := m.Decide(context.Background(), mockRequest("s"))
	if err != nil {
		t.Fatalf("Decide() with no entries error = %v, want nil", err)
	}
	if resp.Provider != "mock" {
		t.Errorf("Response.Provider = %q, want the mock's name", resp.Provider)
	}
	if len(resp.Answers) != 0 || len(resp.Invalid) != 0 {
		t.Errorf("exhausted mock answered something: %+v", resp)
	}
}

func TestMockDecider_AddEntryContinuesTheScript(t *testing.T) {
	m := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{Provider: "one"}})
	if _, err := m.Decide(context.Background(), mockRequest("a")); err != nil {
		t.Fatal(err)
	}
	m.AddEntry(decision.MockEntry{Response: decision.Response{Provider: "two"}})

	resp, err := m.Decide(context.Background(), mockRequest("b"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "two" {
		t.Errorf("Response.Provider = %q, want the entry added after construction", resp.Provider)
	}
}

func TestMockDecider_CallsRecordsEveryRequestInOrderAsASnapshot(t *testing.T) {
	m := decision.NewMockDecider("mock")
	for _, state := range []string{"a", "b", "c"} {
		if _, err := m.Decide(context.Background(), mockRequest(state)); err != nil {
			t.Fatal(err)
		}
	}

	calls := m.Calls()
	if len(calls) != 3 || calls[0].State != "a" || calls[1].State != "b" || calls[2].State != "c" {
		t.Fatalf("Calls() = %+v, want states a, b, c in call order", calls)
	}

	// Mutating the returned slice must not rewrite the mock's own record.
	calls[0].State = "tampered"
	if got := m.Calls()[0].State; got != "a" {
		t.Errorf("Calls() exposes internal storage: first state became %q", got)
	}

	// A later call does not alter the snapshot already returned.
	if _, err := m.Decide(context.Background(), mockRequest("d")); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Errorf("earlier snapshot grew to %d entries", len(calls))
	}
}

func TestMockDecider_ConstructorCopiesEntries(t *testing.T) {
	entries := []decision.MockEntry{{Response: decision.Response{Provider: "original"}}}
	m := decision.NewMockDecider("mock", entries...)
	entries[0] = decision.MockEntry{Response: decision.Response{Provider: "rewritten by caller"}}

	resp, err := m.Decide(context.Background(), mockRequest("s"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "original" {
		t.Errorf("Response.Provider = %q: the mock aliases the caller's slice", resp.Provider)
	}
}

func TestMockDecider_SafeForConcurrentUse(t *testing.T) {
	const workers, perWorker = 8, 25
	m := decision.NewMockDecider("mock")

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				m.AddEntry(decision.MockEntry{})
				if _, err := m.Decide(context.Background(), mockRequest("s")); err != nil {
					t.Errorf("Decide() error = %v", err)
				}
				_ = m.Calls()
			}
		}()
	}
	wg.Wait()

	if got := len(m.Calls()); got != workers*perWorker {
		t.Errorf("len(Calls()) = %d, want %d", got, workers*perWorker)
	}
}

// MockDecider satisfies Decider, the point of the double.
var _ decision.Decider = (*decision.MockDecider)(nil)
