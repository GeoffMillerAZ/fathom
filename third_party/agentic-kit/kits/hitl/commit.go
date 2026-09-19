package hitl

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"
)

// ErrApproval denies a missing, stale, expired, revoked or actor-mismatched decision.
var ErrApproval = errors.New("hitl: approval denied")

// ErrConsumed denies reuse of a consumed decision.
var ErrConsumed = errors.New("hitl: decision already consumed")

// ErrCommitConflict identifies a duplicate ID or conflicting durable record.
var ErrCommitConflict = errors.New("hitl: commit record conflict")

// ErrCommitCapacity means the bounded store cannot retain another decision.
var ErrCommitCapacity = errors.New("hitl: commit store capacity reached")

// ErrReconcileRequired means an external outcome cannot safely be inferred or resent.
var ErrReconcileRequired = errors.New("hitl: remote outcome requires reconciliation")

// Binding identifies the exact approved operation and authoritative target inputs.
// Digests are lowercase SHA-256 hex. OperationDigest uses OperationDigest's v1
// typed-Go-JSON encoding; use the same Go operation type after reconstruction.
type Binding struct {
	Action          string
	Resource        string
	OperationDigest string
	ExpectedVersion uint64
	EvidenceDigest  string
	ConfigDigest    string
}

// Validate checks a binding's complete shape; hashes alone do not grant authority.
func (b Binding) Validate() error {
	if b.Action == "" || b.Resource == "" || b.ExpectedVersion == 0 || !validDigest(b.OperationDigest) || !validDigest(b.EvidenceDigest) || !validDigest(b.ConfigDigest) {
		return fmt.Errorf("hitl: malformed binding")
	}
	return nil
}

// Decision records the authenticated actor's approval. It is looked up from the
// trusted store at commit; supplying a fabricated Decision does not grant access.
type Decision struct {
	ID        string
	Actor     string
	Binding   Binding
	IssuedAt  time.Time
	ExpiresAt time.Time
	Revoked   bool
	Consumed  bool
}

// CommitReceipt binds a successful local commit to its consumed decision.
type CommitReceipt struct {
	DecisionID       string
	Actor            string
	Binding          Binding
	CommittedVersion uint64
	Summary          string
	At               time.Time
}

// CommitConfig supplies trusted host callbacks. Authenticate derives identity
// from authenticated context, never model arguments. Approve presents the exact
// binding/preview to that actor; false or error denies. MaxTTL must be positive.
// Clock is required. Random supplies 16 fresh bytes; nil defaults to crypto/rand.
// Callbacks must be concurrent-safe; production randomness must be unpredictable.
type CommitConfig struct {
	Authenticate func(context.Context) (string, error)
	Approve      func(context.Context, string, Binding, Preview) (bool, error)
	Clock        func() time.Time
	Random       func(int) ([]byte, error)
	MaxTTL       time.Duration
}

// Committer composes Engine with durable approval, atomic local commit and remote
// recovery. State and Op must round-trip through encoding/json without loss.
// Concurrency: safe for concurrent use when its engine, callbacks and store are.
type Committer[State any, Op any] struct {
	engine *Engine[State, Op]
	store  TransactionStore[State]
	cfg    CommitConfig
}

// NewCommitter requires all safety dependencies; it never changes Engine defaults.
func NewCommitter[State any, Op any](engine *Engine[State, Op], store TransactionStore[State], cfg CommitConfig) (*Committer[State, Op], error) {
	if engine == nil || engine.apply == nil || engine.clone == nil || nilTransactionStore(store) || cfg.Authenticate == nil || cfg.Approve == nil || cfg.Clock == nil || cfg.MaxTTL <= 0 {
		return nil, fmt.Errorf("hitl: missing commit dependency or invalid TTL bound")
	}
	if cfg.Random == nil {
		cfg.Random = func(n int) ([]byte, error) {
			b := make([]byte, n)
			_, err := io.ReadFull(rand.Reader, b)
			return b, err
		}
	}
	return &Committer[State, Op]{engine: engine, store: store, cfg: cfg}, nil
}

func nilTransactionStore(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return r.IsNil()
	}
	return false
}

// OperationDigest hashes the compact JSON encoding of a typed operation with a
// v1 domain separator. Map keys follow encoding/json ordering. Custom JSON codecs
// must be deterministic; a lossy round trip is rejected. This is not RFC 8785.
func OperationDigest[Op any](op Op) (string, error) {
	b, _, err := ownJSON(op)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte("hitl-operation-v1\x00"), b...))
	return hex.EncodeToString(h[:]), nil
}

// Propose previews an owned operation/state, requests an authenticated decision,
// and persists approval only if target/evidence/config remain current. Failed or
// canceled approval creates no decision. ID collisions fail without overwriting.
func (c *Committer[State, Op]) Propose(ctx context.Context, action string, op Op, ttl time.Duration) (Decision, Preview, error) {
	actor, err := c.actor(ctx)
	if err != nil {
		return Decision{}, Preview{}, err
	}
	if ttl <= 0 || ttl > c.cfg.MaxTTL {
		return Decision{}, Preview{}, fmt.Errorf("hitl: approval TTL exceeds bound")
	}
	_, owned, err := ownJSON(op)
	if err != nil {
		return Decision{}, Preview{}, err
	}
	digest, err := OperationDigest(owned)
	if err != nil {
		return Decision{}, Preview{}, err
	}
	snapshot, err := c.store.View(ctx)
	if err != nil {
		return Decision{}, Preview{}, err
	}
	_, state, err := ownJSON(snapshot.Target.Value)
	if err != nil {
		return Decision{}, Preview{}, err
	}
	b := Binding{Action: action, Resource: snapshot.Target.Resource, OperationDigest: digest, ExpectedVersion: snapshot.Target.Version, EvidenceDigest: snapshot.Target.EvidenceDigest, ConfigDigest: snapshot.Target.ConfigDigest}
	if err := b.Validate(); err != nil {
		return Decision{}, Preview{}, err
	}
	pv := c.engine.Preview(state, owned)
	if !pv.OK {
		return Decision{}, pv, fmt.Errorf("%w: preview failed: %s", ErrApproval, pv.Err)
	}
	_, approvalPreview, err := ownJSON(pv)
	if err != nil {
		return Decision{}, pv, err
	}
	allowed, err := c.cfg.Approve(ctx, actor, b, approvalPreview)
	if err != nil {
		return Decision{}, pv, fmt.Errorf("%w: approver: %w", ErrApproval, err)
	}
	if !allowed {
		return Decision{}, pv, ErrApproval
	}
	if err := ctx.Err(); err != nil {
		return Decision{}, pv, err
	}
	raw, err := c.cfg.Random(16)
	if err != nil {
		return Decision{}, pv, err
	}
	if len(raw) != 16 {
		return Decision{}, pv, fmt.Errorf("hitl: random source must return 16 bytes")
	}
	now := c.cfg.Clock()
	d := Decision{ID: hex.EncodeToString(raw), Actor: actor, Binding: b, IssuedAt: now, ExpiresAt: now.Add(ttl)}
	err = c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		if !matchesTarget(b, s.Target) {
			return ErrApproval
		}
		if decisionIndex(s.Decisions, d.ID) >= 0 {
			return ErrCommitConflict
		}
		if !c.cfg.Clock().Before(d.ExpiresAt) {
			return ErrApproval
		}
		s.Decisions = append(s.Decisions, d)
		return nil
	})
	if err != nil {
		return Decision{}, pv, err
	}
	return d, pv, nil
}

// Commit verifies the stored decision and exact operation, then consumes it and
// applies on an owned clone within the caller's atomic transaction. Apply failure,
// cancellation, stale inputs or persistence failure cannot publish partial state.
func (c *Committer[State, Op]) Commit(ctx context.Context, id string, binding Binding, op Op) (CommitReceipt, error) {
	actor, err := c.actor(ctx)
	if err != nil {
		return CommitReceipt{}, err
	}
	_, owned, err := ownJSON(op)
	if err != nil {
		return CommitReceipt{}, err
	}
	digest, err := OperationDigest(owned)
	if err != nil {
		return CommitReceipt{}, err
	}
	var receipt CommitReceipt
	err = c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		i, err := c.verify(s, id, actor, binding, digest, false)
		if err != nil {
			return err
		}
		if unresolvedAttempt(s.Attempts, "") {
			return ErrReconcileRequired
		}
		if s.Target.Version == math.MaxUint64 {
			return fmt.Errorf("hitl: target version exhausted")
		}
		_, next, err := ownJSON(s.Target.Value)
		if err != nil {
			return err
		}
		s.Decisions[i].Consumed = true
		summary, err := c.engine.Apply(&next, owned)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Recheck expiry after application, before publishing the transaction.
		if _, err := c.verify(s, id, actor, binding, digest, true); err != nil {
			return err
		}
		s.Target.Value = next
		s.Target.Version++
		receipt = CommitReceipt{DecisionID: id, Actor: actor, Binding: binding, CommittedVersion: s.Target.Version, Summary: summary, At: c.cfg.Clock()}
		s.Receipts = append(s.Receipts, receipt)
		return nil
	})
	if err != nil {
		return CommitReceipt{}, err
	}
	return receipt, nil
}

// Revoke disables future commits/sends by the approving actor. It cannot undo an
// already committed effect, nor cancel a remote request already in flight.
func (c *Committer[State, Op]) Revoke(ctx context.Context, id string) error {
	actor, err := c.actor(ctx)
	if err != nil {
		return err
	}
	return c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		i := decisionIndex(s.Decisions, id)
		if i < 0 || s.Decisions[i].Actor != actor {
			return ErrApproval
		}
		s.Decisions[i].Revoked = true
		return nil
	})
}

func (c *Committer[State, Op]) actor(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	actor, err := c.cfg.Authenticate(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: identity: %w", ErrApproval, err)
	}
	if actor == "" {
		return "", ErrApproval
	}
	return actor, nil
}

func (c *Committer[State, Op]) verify(s *CommitSnapshot[State], id, actor string, b Binding, digest string, consumed bool) (int, error) {
	i := decisionIndex(s.Decisions, id)
	if i < 0 {
		return -1, ErrApproval
	}
	d := s.Decisions[i]
	now := c.cfg.Clock()
	if d.Actor != actor || d.Binding != b || b.OperationDigest != digest || !matchesTarget(b, s.Target) || d.Revoked || now.Before(d.IssuedAt) || !now.Before(d.ExpiresAt) {
		return -1, ErrApproval
	}
	if d.Consumed != consumed {
		return -1, ErrConsumed
	}
	return i, nil
}

func matchesTarget[State any](b Binding, s ResourceState[State]) bool {
	return b.Resource == s.Resource && b.ExpectedVersion == s.Version && b.EvidenceDigest == s.EvidenceDigest && b.ConfigDigest == s.ConfigDigest
}
func decisionIndex(ds []Decision, id string) int {
	for i, d := range ds {
		if d.ID == id {
			return i
		}
	}
	return -1
}
func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func validID(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 16 && hex.EncodeToString(b) == s
}

func ownJSON[T any](value T) ([]byte, T, error) {
	var out T
	b, err := json.Marshal(value)
	if err != nil {
		return nil, out, fmt.Errorf("hitl: encode owned value: %w", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, out, fmt.Errorf("hitl: decode owned value: %w", err)
	}
	roundtrip, err := json.Marshal(out)
	if err != nil || !bytes.Equal(b, roundtrip) {
		return nil, out, fmt.Errorf("hitl: JSON value cannot round-trip without loss")
	}
	return b, out, nil
}
