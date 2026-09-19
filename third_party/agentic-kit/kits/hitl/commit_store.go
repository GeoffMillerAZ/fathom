package hitl

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MaxCommitSnapshotBytes bounds serialized state plus retained decisions, receipts
// and attempts in MemoryCommitStore. Larger workloads need a caller-owned adapter
// with an explicit retention/capacity policy; replay tombstones must not disappear.
const MaxCommitSnapshotBytes = 16 << 20

// ResourceState is the caller's authoritative target and current evidence/config
// versions. Every external target change must increment Version in the same store.
type ResourceState[State any] struct {
	Resource       string
	Version        uint64
	EvidenceDigest string
	ConfigDigest   string
	Value          State
}

// CommitSnapshot is the versioned persistence shape for one resource. It contains
// no execution authority outside its trusted host/store. Slices retain duplicate
// identities so malformed restoration can reject them instead of overwriting.
type CommitSnapshot[State any] struct {
	FormatVersion int
	Target        ResourceState[State]
	Decisions     []Decision
	Receipts      []CommitReceipt
	Attempts      []Attempt
}

// TransactionStore owns atomic verify/consume/apply persistence. Update must call
// fn once on an isolated snapshot under its transaction/lock, roll back all changes
// on error/cancellation, and publish durably only on success. Callbacks must not
// reenter this store. View returns a deep copy. Implementations must enforce the
// snapshot invariants and their capacity bound, and be safe for concurrent use.
// Stores and authentication endpoints are trusted host dependencies, not model tools.
type TransactionStore[State any] interface {
	View(context.Context) (CommitSnapshot[State], error)
	Update(context.Context, func(*CommitSnapshot[State]) error) error
}

// MemoryCommitStore is a bounded, concurrent-safe reference transaction store.
// It owns snapshots via JSON copies and loses them on process exit. The capacity
// bounds decisions; each decision can own at most one receipt or remote attempt.
// This reference retains replay tombstones rather than evicting them silently.
type MemoryCommitStore[State any] struct {
	mu       sync.Mutex
	snapshot CommitSnapshot[State]
	capacity int
}

// NewMemoryCommitStore initializes an empty approval ledger around an owned target.
func NewMemoryCommitStore[State any](target ResourceState[State], capacity int) (*MemoryCommitStore[State], error) {
	return RestoreMemoryCommitStore(CommitSnapshot[State]{FormatVersion: 1, Target: target}, capacity)
}

// RestoreMemoryCommitStore validates and owns a caller-deserialized snapshot.
// Durable adapters must protect snapshots from tampering and preserve tombstones.
func RestoreMemoryCommitStore[State any](snapshot CommitSnapshot[State], capacity int) (*MemoryCommitStore[State], error) {
	if err := snapshot.Validate(capacity); err != nil {
		return nil, err
	}
	_, owned, err := ownJSON(snapshot)
	if err != nil {
		return nil, err
	}
	return &MemoryCommitStore[State]{snapshot: owned, capacity: capacity}, nil
}

// View returns an owned snapshot suitable for caller-managed serialization.
func (s *MemoryCommitStore[State]) View(ctx context.Context) (CommitSnapshot[State], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return CommitSnapshot[State]{}, err
	}
	_, out, err := ownJSON(s.snapshot)
	return out, err
}

// Update commits an isolated transaction only after validation and cancellation
// checks. Failed appliers cannot mutate persisted state or consume the decision.
func (s *MemoryCommitStore[State]) Update(ctx context.Context, fn func(*CommitSnapshot[State]) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return fmt.Errorf("hitl: nil transaction")
	}
	_, work, err := ownJSON(s.snapshot)
	if err != nil {
		return err
	}
	if err := fn(&work); err != nil {
		return err
	}
	if err := work.Validate(s.capacity); err != nil {
		return err
	}
	_, owned, err := ownJSON(work)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.snapshot = owned
	return nil
}

// Validate rejects unsupported, inconsistent, duplicate or over-capacity records.
func (s CommitSnapshot[State]) Validate(capacity int) error {
	if capacity <= 0 || len(s.Decisions) > capacity {
		return ErrCommitCapacity
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("hitl: snapshot encoding: %w", err)
	}
	if len(encoded) > MaxCommitSnapshotBytes {
		return ErrCommitCapacity
	}
	if s.FormatVersion != 1 || s.Target.Resource == "" || s.Target.Version == 0 || !validDigest(s.Target.EvidenceDigest) || !validDigest(s.Target.ConfigDigest) {
		return fmt.Errorf("hitl: invalid commit snapshot")
	}
	ids := make(map[string]Decision, len(s.Decisions))
	for _, d := range s.Decisions {
		if _, exists := ids[d.ID]; exists {
			return ErrCommitConflict
		}
		if !validID(d.ID) || d.Actor == "" || d.Binding.Validate() != nil || d.Binding.Resource != s.Target.Resource || d.Binding.ExpectedVersion > s.Target.Version || d.IssuedAt.IsZero() || !d.ExpiresAt.After(d.IssuedAt) {
			return fmt.Errorf("hitl: invalid stored decision")
		}
		ids[d.ID] = d
	}
	used := make(map[string]bool, len(s.Decisions))
	for _, r := range s.Receipts {
		d, ok := ids[r.DecisionID]
		if !ok || used[r.DecisionID] || !d.Consumed || d.Actor != r.Actor || d.Binding != r.Binding || r.CommittedVersion < 2 || r.CommittedVersion != d.Binding.ExpectedVersion+1 || r.CommittedVersion > s.Target.Version || r.At.Before(d.IssuedAt) || !r.At.Before(d.ExpiresAt) {
			return fmt.Errorf("hitl: invalid commit receipt")
		}
		used[r.DecisionID] = true
	}
	for _, a := range s.Attempts {
		d, ok := ids[a.DecisionID]
		if !ok || used[a.DecisionID] || !d.Consumed || a.ID != d.ID || a.IdempotencyKey != "hitl-"+d.ID || a.Binding != d.Binding || a.Actor != d.Actor || a.PreparedAt.Before(d.IssuedAt) || !a.PreparedAt.Before(d.ExpiresAt) || a.UpdatedAt.Before(a.PreparedAt) {
			return fmt.Errorf("hitl: invalid remote attempt")
		}
		if a.Status != AttemptPrepared && a.Status != AttemptSending && a.Status != AttemptUnknown && a.Status != AttemptSucceeded && a.Status != AttemptNotApplied {
			return fmt.Errorf("hitl: invalid remote status")
		}
		if (a.Status == AttemptPrepared && a.Sends != 0) || (a.Status != AttemptPrepared && a.Sends != 1) || (a.Status == AttemptSucceeded) != (a.Receipt != "") {
			return fmt.Errorf("hitl: inconsistent remote attempt")
		}
		if a.Status == AttemptSending && !a.UpdatedAt.Before(d.ExpiresAt) {
			return fmt.Errorf("hitl: send reservation expired")
		}
		used[a.DecisionID] = true
	}
	for _, d := range s.Decisions {
		if d.Consumed != used[d.ID] {
			return fmt.Errorf("hitl: missing consumption record")
		}
	}
	return nil
}
