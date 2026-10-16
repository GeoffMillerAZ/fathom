package hitl_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/hitl"
)

func prepareRemote(t *testing.T, f *commitFixture) (*hitl.Committer[commitState, commitOp], hitl.Decision, commitOp) {
	t.Helper()
	c := f.committer(t, f.store)
	op := commitOp{Delta: 1, Values: []string{"approved"}}
	d := approveCommit(t, c, op)
	if a, err := c.Prepare(reviewer(), d.ID, d.Binding, op); err != nil || a.Status != hitl.AttemptPrepared {
		t.Fatalf("prepare: %+v %v", a, err)
	}
	return c, d, op
}

func TestRemoteRestartBeforeSendAndBoundedConcurrency(t *testing.T) {
	f := newCommitFixture(t)
	_, d, op := prepareRemote(t, f)
	restored := restoreCommit(t, snapshotBytes(t, f.store))
	c := f.committer(t, restored)
	var calls atomic.Int64
	send := func(ctx context.Context, a hitl.Attempt, owned commitOp) (string, error) {
		calls.Add(1)
		if a.IdempotencyKey != "hitl-"+d.ID || a.Binding != d.Binding || owned.Values[0] != "approved" {
			t.Error("send binding changed")
		}
		owned.Values[0] = "sender-owned"
		return "destination-receipt", nil
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = c.Send(reviewer(), d.ID, op, send) }()
	}
	wg.Wait()
	if calls.Load() != 1 || op.Values[0] != "approved" {
		t.Fatalf("send count=%d or input mutated", calls.Load())
	}
	s, _ := restored.View(reviewer())
	if len(s.Attempts) != 1 || s.Attempts[0].Status != hitl.AttemptSucceeded || s.Attempts[0].Sends != 1 || s.Target.Value.Count != 0 {
		t.Fatalf("remote state: %+v", s)
	}
	for range 10 {
		if _, err := c.Prepare(reviewer(), d.ID, d.Binding, op); err == nil {
			t.Fatal("duplicate prepare succeeded")
		}
	}
	s, _ = restored.View(reviewer())
	if len(s.Attempts) != 1 {
		t.Fatal("duplicate approval grew attempts")
	}
}

type failOutcomeStore struct {
	hitl.TransactionStore[commitState]
	calls atomic.Int64
}

func (s *failOutcomeStore) Update(ctx context.Context, fn func(*hitl.CommitSnapshot[commitState]) error) error {
	if s.calls.Add(1) == 2 {
		return errors.New("receipt persistence failed")
	}
	return s.TransactionStore.Update(ctx, fn)
}

func TestRemoteCrashAfterSendBeforeReceipt(t *testing.T) {
	f := newCommitFixture(t)
	_, d, op := prepareRemote(t, f)
	c := f.committer(t, &failOutcomeStore{TransactionStore: f.store})
	var calls atomic.Int64
	send := func(context.Context, hitl.Attempt, commitOp) (string, error) {
		calls.Add(1)
		return "external-committed", nil
	}
	if _, err := c.Send(reviewer(), d.ID, op, send); !errors.Is(err, hitl.ErrReconcileRequired) {
		t.Fatalf("lost receipt: %v", err)
	}
	restored := restoreCommit(t, snapshotBytes(t, f.store))
	restarted := f.committer(t, restored)
	if _, err := restarted.Send(reviewer(), d.ID, op, send); !errors.Is(err, hitl.ErrReconcileRequired) {
		t.Fatalf("restart blindly sent: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate external effect")
	}
	f.now.Store(1000) // Expiry must not prevent observing the historical outcome.
	a, err := restarted.Reconcile(reviewer(), d.ID, func(_ context.Context, a hitl.Attempt) (hitl.AttemptStatus, string, error) {
		if a.Status != hitl.AttemptSending {
			t.Error("lost crash window")
		}
		return hitl.AttemptSucceeded, "external-committed", nil
	})
	if err != nil || a.Status != hitl.AttemptSucceeded || a.Receipt != "external-committed" {
		t.Fatalf("reconcile: %+v %v", a, err)
	}
	if _, err := restarted.Send(reviewer(), d.ID, op, send); err == nil || calls.Load() != 1 {
		t.Fatal("reconciled decision resent")
	}
}

func TestUnknownRemoteBlocksFurtherEffectsUntilReconciled(t *testing.T) {
	f := newCommitFixture(t)
	c, d, op := prepareRemote(t, f)
	if _, err := c.Send(reviewer(), d.ID, op, func(context.Context, hitl.Attempt, commitOp) (string, error) {
		return "", errors.New("timeout after possible send")
	}); !errors.Is(err, hitl.ErrReconcileRequired) {
		t.Fatalf("unknown not surfaced: %v", err)
	}
	second := approveCommit(t, c, op)
	if _, err := c.Prepare(reviewer(), second.ID, second.Binding, op); !errors.Is(err, hitl.ErrReconcileRequired) {
		t.Fatalf("prepared past unknown: %v", err)
	}
	if _, err := c.Commit(reviewer(), second.ID, second.Binding, op); !errors.Is(err, hitl.ErrReconcileRequired) {
		t.Fatalf("local commit past unknown: %v", err)
	}
	for _, reconcile := range []hitl.Reconciler{
		func(context.Context, hitl.Attempt) (hitl.AttemptStatus, string, error) {
			return hitl.AttemptUnknown, "", nil
		},
		func(context.Context, hitl.Attempt) (hitl.AttemptStatus, string, error) {
			return hitl.AttemptNotApplied, "", errors.New("lookup failed")
		},
		func(context.Context, hitl.Attempt) (hitl.AttemptStatus, string, error) {
			return hitl.AttemptPrepared, "", nil
		},
		func(context.Context, hitl.Attempt) (hitl.AttemptStatus, string, error) {
			return hitl.AttemptSucceeded, "", nil
		},
	} {
		if _, err := c.Reconcile(reviewer(), d.ID, reconcile); err == nil {
			t.Fatal("ambiguous reconciliation succeeded")
		}
	}
	if a, err := c.Reconcile(reviewer(), d.ID, func(context.Context, hitl.Attempt) (hitl.AttemptStatus, string, error) {
		return hitl.AttemptNotApplied, "", nil
	}); err != nil || a.Status != hitl.AttemptNotApplied {
		t.Fatalf("settled non-application: %+v %v", a, err)
	}
	if _, err := c.Prepare(reviewer(), second.ID, second.Binding, op); err != nil {
		t.Fatalf("fresh decision blocked after reconciliation: %v", err)
	}
	if _, err := c.Send(reviewer(), d.ID, op, func(context.Context, hitl.Attempt, commitOp) (string, error) {
		t.Error("old decision was sent again")
		return "", nil
	}); err == nil {
		t.Fatal("old attempt reopened")
	}
}

func TestRemoteCancellationExpiryRevocationAndMissingDependencies(t *testing.T) {
	for _, name := range []string{"canceled prepare", "canceled send", "expired recovery", "revoked", "wrong actor", "wrong args", "missing sender", "missing reconciler", "canceled during send", "empty receipt"} {
		t.Run(name, func(t *testing.T) {
			f := newCommitFixture(t)
			c := f.committer(t, f.store)
			op := commitOp{Delta: 1}
			d := approveCommit(t, c, op)
			ctx := reviewer()
			if name == "canceled prepare" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				before := snapshotBytes(t, f.store)
				if _, err := c.Prepare(canceled, d.ID, d.Binding, op); err == nil {
					t.Fatal("canceled prepare succeeded")
				}
				if string(before) != string(snapshotBytes(t, f.store)) {
					t.Fatal("canceled prepare consumed decision")
				}
				return
			}
			if _, err := c.Prepare(ctx, d.ID, d.Binding, op); err != nil {
				t.Fatal(err)
			}
			c = f.committer(t, restoreCommit(t, snapshotBytes(t, f.store)))
			var calls int
			var cancel context.CancelFunc
			send := hitl.Sender[commitOp](func(context.Context, hitl.Attempt, commitOp) (string, error) {
				calls++
				if cancel != nil {
					cancel()
				}
				if name == "empty receipt" {
					return "", nil
				}
				return "receipt", nil
			})
			switch name {
			case "canceled send":
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "expired recovery":
				f.now.Store(160)
			case "revoked":
				if err := c.Revoke(ctx, d.ID); err != nil {
					t.Fatal(err)
				}
			case "wrong actor":
				ctx = context.WithValue(ctx, actorKey{}, "intruder")
			case "wrong args":
				op.Delta++
			case "missing sender":
				send = nil
			case "missing reconciler":
				if _, err := c.Reconcile(ctx, d.ID, nil); err == nil {
					t.Fatal("nil reconciler accepted")
				}
				return
			case "canceled during send":
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
			}
			if _, err := c.Send(ctx, d.ID, op, send); err == nil {
				t.Fatal("invalid send succeeded")
			}
			expected := 0
			if name == "canceled during send" || name == "empty receipt" {
				expected = 1
			}
			if calls != expected {
				t.Fatalf("effect count %d, want %d", calls, expected)
			}
		})
	}
}

func TestRemoteRestoreRejectsMalformedAttempts(t *testing.T) {
	f := newCommitFixture(t)
	_, d, _ := prepareRemote(t, f)
	for _, name := range []string{"duplicate", "unknown decision", "key", "binding", "actor", "status", "send count", "receipt"} {
		t.Run(name, func(t *testing.T) {
			s := restoreCommit(t, snapshotBytes(t, f.store))
			snapshot, _ := s.View(reviewer())
			a := &snapshot.Attempts[0]
			switch name {
			case "duplicate":
				snapshot.Attempts = append(snapshot.Attempts, *a)
			case "unknown decision":
				a.DecisionID = "missing"
			case "key":
				a.IdempotencyKey = "different"
			case "binding":
				a.Binding.Action = "delete"
			case "actor":
				a.Actor = "intruder"
			case "status":
				a.Status = "invalid"
			case "send count":
				a.Sends = 2
			case "receipt":
				a.Receipt = "not sent"
			}
			if _, err := hitl.RestoreMemoryCommitStore(snapshot, 32); err == nil {
				t.Fatalf("malformed attempt %s accepted for %s", name, d.ID)
			}
		})
	}
}
