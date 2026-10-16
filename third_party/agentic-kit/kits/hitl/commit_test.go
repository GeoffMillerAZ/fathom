package hitl_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/hitl"
)

type commitState struct {
	Count  int
	Values []string
}
type commitOp struct {
	Delta  int
	Values []string
}
type actorKey struct{}

type commitFixture struct {
	store  *hitl.MemoryCommitStore[commitState]
	engine *hitl.Engine[commitState, commitOp]
	cfg    hitl.CommitConfig
	now    atomic.Int64
	seq    atomic.Uint64
	fail   atomic.Bool
}

func newCommitFixture(t *testing.T) *commitFixture {
	t.Helper()
	f := &commitFixture{}
	f.now.Store(100)
	f.engine = hitl.New(func(s *commitState, op commitOp) (string, error) {
		s.Count += op.Delta
		s.Values = append(s.Values, op.Values...)
		if f.fail.Load() {
			return "", errors.New("apply failed after mutation")
		}
		return "updated", nil
	}, func(s commitState) commitState { s.Values = append([]string(nil), s.Values...); return s })
	var err error
	f.store, err = hitl.NewMemoryCommitStore(hitl.ResourceState[commitState]{Resource: "draft", Version: 1, EvidenceDigest: strings.Repeat("a", 64), ConfigDigest: strings.Repeat("b", 64), Value: commitState{Values: []string{"original"}}}, 32)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg = hitl.CommitConfig{
		Authenticate: func(ctx context.Context) (string, error) {
			actor, _ := ctx.Value(actorKey{}).(string)
			return actor, nil
		},
		Approve: func(context.Context, string, hitl.Binding, hitl.Preview) (bool, error) { return true, nil },
		Clock:   func() time.Time { return time.Unix(f.now.Load(), 0).UTC() },
		Random: func(n int) ([]byte, error) {
			b := make([]byte, n)
			binary.BigEndian.PutUint64(b[n-8:], f.seq.Add(1))
			return b, nil
		},
		MaxTTL: time.Minute,
	}
	return f
}

func reviewer() context.Context {
	return context.WithValue(context.Background(), actorKey{}, "reviewer")
}
func (f *commitFixture) committer(t *testing.T, store hitl.TransactionStore[commitState]) *hitl.Committer[commitState, commitOp] {
	t.Helper()
	c, err := hitl.NewCommitter(f.engine, store, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func approveCommit(t *testing.T, c *hitl.Committer[commitState, commitOp], op commitOp) hitl.Decision {
	t.Helper()
	d, pv, err := c.Propose(reviewer(), "publish", op, time.Minute)
	if err != nil || !pv.OK {
		t.Fatalf("proposal: %+v %v", pv, err)
	}
	return d
}

func snapshotBytes(t *testing.T, s *hitl.MemoryCommitStore[commitState]) []byte {
	t.Helper()
	snapshot, err := s.View(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func restoreCommit(t *testing.T, b []byte) *hitl.MemoryCommitStore[commitState] {
	t.Helper()
	var snapshot hitl.CommitSnapshot[commitState]
	if err := json.Unmarshal(b, &snapshot); err != nil {
		t.Fatal(err)
	}
	s, err := hitl.RestoreMemoryCommitStore(snapshot, 32)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDurableCommitReconstructionAndReplay(t *testing.T) {
	f := newCommitFixture(t)
	c := f.committer(t, f.store)
	op := commitOp{Delta: 2, Values: []string{"approved"}}
	d := approveCommit(t, c, op)
	restored := restoreCommit(t, snapshotBytes(t, f.store))
	restarted := f.committer(t, restored)
	r, err := restarted.Commit(reviewer(), d.ID, d.Binding, op)
	if err != nil || r.Actor != "reviewer" || r.Binding != d.Binding || r.CommittedVersion != 2 {
		t.Fatalf("commit: %+v %v", r, err)
	}
	snapshot, _ := restored.View(reviewer())
	if snapshot.Target.Value.Count != 2 || !snapshot.Decisions[0].Consumed || len(snapshot.Receipts) != 1 {
		t.Fatalf("bad committed state: %+v", snapshot)
	}
	before := snapshotBytes(t, restored)
	if _, err := restarted.Commit(reviewer(), d.ID, d.Binding, op); err == nil {
		t.Fatal("replayed approval committed")
	}
	if !bytes.Equal(before, snapshotBytes(t, restored)) {
		t.Fatal("replay mutated state")
	}
	again := restoreCommit(t, before)
	if _, err := f.committer(t, again).Commit(reviewer(), d.ID, d.Binding, op); err == nil {
		t.Fatal("restart lost consumption")
	}
}

func TestDurableCommitRefusalsDoNotMutate(t *testing.T) {
	for _, name := range []string{"actor", "args", "action", "resource", "expected version", "evidence", "config", "stale state", "stale evidence", "stale config", "expired", "revoked", "unknown", "canceled", "failed apply"} {
		t.Run(name, func(t *testing.T) {
			f := newCommitFixture(t)
			c := f.committer(t, f.store)
			op := commitOp{Delta: 2}
			d := approveCommit(t, c, op)
			ctx := reviewer()
			binding := d.Binding
			id := d.ID
			switch name {
			case "actor":
				ctx = context.WithValue(ctx, actorKey{}, "intruder")
			case "args":
				op.Delta++
			case "action":
				binding.Action = "delete"
			case "resource":
				binding.Resource = "elsewhere"
			case "expected version":
				binding.ExpectedVersion++
			case "evidence":
				binding.EvidenceDigest = strings.Repeat("c", 64)
			case "config":
				binding.ConfigDigest = strings.Repeat("c", 64)
			case "stale state", "stale evidence", "stale config":
				if err := f.store.Update(ctx, func(s *hitl.CommitSnapshot[commitState]) error {
					switch name {
					case "stale state":
						s.Target.Version++
						s.Target.Value.Count = 9
					case "stale evidence":
						s.Target.EvidenceDigest = strings.Repeat("c", 64)
					case "stale config":
						s.Target.ConfigDigest = strings.Repeat("c", 64)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			case "expired":
				f.now.Store(160)
			case "revoked":
				if err := c.Revoke(ctx, d.ID); err != nil {
					t.Fatal(err)
				}
			case "unknown":
				id = strings.Repeat("f", 32)
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "failed apply":
				f.fail.Store(true)
			}
			before := snapshotBytes(t, f.store)
			if _, err := c.Commit(ctx, id, binding, op); err == nil {
				t.Fatal("invalid commit succeeded")
			}
			if !bytes.Equal(before, snapshotBytes(t, f.store)) {
				t.Fatal("refused commit changed durable state")
			}
		})
	}
}

func TestApprovalDeniedErroredMissingAndStale(t *testing.T) {
	for _, name := range []string{"denied", "error", "unauthenticated", "auth error", "canceled", "mutated state", "invalid ttl", "entropy error", "entropy length"} {
		t.Run(name, func(t *testing.T) {
			f := newCommitFixture(t)
			ctx := reviewer()
			ttl := time.Minute
			switch name {
			case "denied":
				f.cfg.Approve = func(context.Context, string, hitl.Binding, hitl.Preview) (bool, error) { return false, nil }
			case "error":
				f.cfg.Approve = func(context.Context, string, hitl.Binding, hitl.Preview) (bool, error) {
					return true, errors.New("approval unavailable")
				}
			case "unauthenticated":
				ctx = context.Background()
			case "auth error":
				f.cfg.Authenticate = func(context.Context) (string, error) { return "reviewer", errors.New("identity unavailable") }
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				f.cfg.Approve = func(context.Context, string, hitl.Binding, hitl.Preview) (bool, error) { cancel(); return true, nil }
			case "mutated state":
				f.cfg.Approve = func(context.Context, string, hitl.Binding, hitl.Preview) (bool, error) {
					err := f.store.Update(reviewer(), func(s *hitl.CommitSnapshot[commitState]) error { s.Target.Version++; return nil })
					return true, err
				}
			case "invalid ttl":
				ttl = time.Hour
			case "entropy error":
				f.cfg.Random = func(int) ([]byte, error) { return nil, errors.New("no entropy") }
			case "entropy length":
				f.cfg.Random = func(int) ([]byte, error) { return nil, nil }
			}
			c := f.committer(t, f.store)
			if _, _, err := c.Propose(ctx, "publish", commitOp{Delta: 1}, ttl); err == nil {
				t.Fatal("invalid approval persisted")
			}
			snapshot, _ := f.store.View(context.Background())
			if len(snapshot.Decisions) != 0 || snapshot.Target.Value.Count != 0 {
				t.Fatal("approval failure produced effects")
			}
		})
	}
	for _, missing := range []string{"engine", "store", "identity", "approver", "clock", "max ttl"} {
		f := newCommitFixture(t)
		engine := f.engine
		var store hitl.TransactionStore[commitState] = f.store
		cfg := f.cfg
		switch missing {
		case "engine":
			engine = nil
		case "store":
			store = nil
		case "identity":
			cfg.Authenticate = nil
		case "approver":
			cfg.Approve = nil
		case "clock":
			cfg.Clock = nil
		case "max ttl":
			cfg.MaxTTL = 0
		}
		if _, err := hitl.NewCommitter(engine, store, cfg); err == nil {
			t.Fatalf("accepted missing %s", missing)
		}
	}
}

func TestOwnedApprovalOperationStateAndConcurrentCommit(t *testing.T) {
	f := newCommitFixture(t)
	f.engine = hitl.New(func(s *commitState, op commitOp) (string, error) {
		op.Values[0] = "internal"
		s.Values[0] = "changed"
		s.Count += op.Delta
		return "updated", nil
	}, func(s commitState) commitState { return s })
	c := f.committer(t, f.store)
	op := commitOp{Delta: 1, Values: []string{"original"}}
	d := approveCommit(t, c, op)
	snapshot, _ := f.store.View(reviewer())
	if op.Values[0] != "original" || snapshot.Target.Value.Values[0] != "original" {
		t.Fatal("preview mutated caller operation or persisted state")
	}
	var wg sync.WaitGroup
	var commits atomic.Int64
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Commit(reviewer(), d.ID, d.Binding, op); err == nil {
				commits.Add(1)
			}
		}()
	}
	wg.Wait()
	if commits.Load() != 1 {
		t.Fatalf("committed %d times", commits.Load())
	}
	if op.Values[0] != "original" {
		t.Fatal("apply mutated caller operation")
	}
	if err := f.store.Update(reviewer(), func(s *hitl.CommitSnapshot[commitState]) error {
		s.Target.Value.Values[0] = "failed mutation"
		return errors.New("abort")
	}); err == nil {
		t.Fatal("transaction should fail")
	}
	snapshot, _ = f.store.View(reviewer())
	if snapshot.Target.Value.Values[0] != "changed" {
		t.Fatal("aborted transaction leaked changes")
	}
	snapshot.Target.Value.Values[0] = "outside"
	fresh, _ := f.store.View(reviewer())
	if fresh.Target.Value.Values[0] != "changed" {
		t.Fatal("View did not own state")
	}
}

func TestSnapshotValidationCapacityAndOperationParse(t *testing.T) {
	f := newCommitFixture(t)
	c := f.committer(t, f.store)
	d := approveCommit(t, c, commitOp{Delta: 1})
	snapshot, _ := f.store.View(reviewer())
	for _, name := range []string{"duplicate", "format", "binding", "time", "consumed", "attempt", "capacity"} {
		t.Run(name, func(t *testing.T) {
			copy := restoreCommit(t, snapshotBytes(t, f.store))
			s, _ := copy.View(reviewer())
			capacity := 32
			switch name {
			case "duplicate":
				s.Decisions = append(s.Decisions, d)
			case "format":
				s.FormatVersion++
			case "binding":
				s.Decisions[0].Binding.OperationDigest = "invalid"
			case "time":
				s.Decisions[0].ExpiresAt = s.Decisions[0].IssuedAt
			case "consumed":
				s.Decisions[0].Consumed = true
			case "attempt":
				s.Attempts = []hitl.Attempt{{ID: d.ID, DecisionID: d.ID, Status: hitl.AttemptUnknown}}
			case "capacity":
				capacity = 0
			}
			if _, err := hitl.RestoreMemoryCommitStore(s, capacity); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
	bounded, err := hitl.RestoreMemoryCommitStore(snapshot, 1)
	if err != nil {
		t.Fatal(err)
	}
	boundedCommit := f.committer(t, bounded)
	if _, _, err := boundedCommit.Propose(reviewer(), "publish", commitOp{}, time.Minute); !errors.Is(err, hitl.ErrCommitCapacity) {
		t.Fatalf("capacity: %v", err)
	}
	f.seq.Store(0)
	if _, _, err := c.Propose(reviewer(), "publish", commitOp{}, time.Minute); !errors.Is(err, hitl.ErrCommitConflict) {
		t.Fatalf("ID collision: %v", err)
	}
	if _, err := hitl.OperationDigest(func() {}); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	if _, err := hitl.OperationDigest(json.RawMessage("{")); err == nil {
		t.Fatal("malformed operation accepted")
	}
	var parsed hitl.CommitSnapshot[commitState]
	if err := json.Unmarshal([]byte("{"), &parsed); err == nil {
		t.Fatal("malformed snapshot parsed")
	}
	a, _ := hitl.OperationDigest(map[string]int{"b": 2, "a": 1})
	b, _ := hitl.OperationDigest(map[string]int{"a": 1, "b": 2})
	if a != b {
		t.Fatal("map order changed operation digest")
	}
}

func TestCommitCanceledAfterApplyAndSnapshotByteBound(t *testing.T) {
	f := newCommitFixture(t)
	var cancel context.CancelFunc
	f.engine = hitl.New(func(s *commitState, op commitOp) (string, error) {
		s.Count += op.Delta
		if cancel != nil {
			cancel()
		}
		return "updated", nil
	}, func(s commitState) commitState { return s })
	c := f.committer(t, f.store)
	op := commitOp{Delta: 1}
	d := approveCommit(t, c, op)
	before := snapshotBytes(t, f.store)
	ctx, stop := context.WithCancel(reviewer())
	defer stop()
	cancel = stop
	if _, err := c.Commit(ctx, d.ID, d.Binding, op); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled apply: %v", err)
	}
	if !bytes.Equal(before, snapshotBytes(t, f.store)) {
		t.Fatal("cancellation after apply published state")
	}
	snapshot, _ := f.store.View(reviewer())
	snapshot.Target.Value.Values = []string{strings.Repeat("x", hitl.MaxCommitSnapshotBytes)}
	if _, err := hitl.RestoreMemoryCommitStore(snapshot, 32); !errors.Is(err, hitl.ErrCommitCapacity) {
		t.Fatalf("snapshot byte bound: %v", err)
	}
}
