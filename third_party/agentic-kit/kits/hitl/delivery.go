package hitl

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// AttemptStatus describes a durable remote delivery outcome. Sending recovered
// after interruption is unresolved and must be reconciled, never sent again.
type AttemptStatus string

const (
	// AttemptPrepared has not crossed the durable send boundary.
	AttemptPrepared AttemptStatus = "prepared"
	// AttemptSending crossed the send boundary; completion is not yet durable.
	AttemptSending AttemptStatus = "sending"
	// AttemptSucceeded carries a destination receipt.
	AttemptSucceeded AttemptStatus = "succeeded"
	// AttemptUnknown requires destination reconciliation.
	AttemptUnknown AttemptStatus = "unknown"
	// AttemptNotApplied records authoritative reconciliation proving non-application.
	AttemptNotApplied AttemptStatus = "not-applied"
)

// Attempt binds at most one remote send to a consumed decision. The idempotency
// key remains stable across restart; destination deduplication remains its job.
type Attempt struct {
	ID             string
	DecisionID     string
	Actor          string
	Binding        Binding
	IdempotencyKey string
	Status         AttemptStatus
	Sends          int
	Receipt        string
	PreparedAt     time.Time
	UpdatedAt      time.Time
}

// Sender performs the exact owned operation and passes IdempotencyKey to the
// destination. A nonempty receipt confirms success; any error or empty receipt is
// ambiguous. It must honor context and must not silently retry the remote request.
type Sender[Op any] func(context.Context, Attempt, Op) (receipt string, err error)

// Reconciler queries the destination without sending again. It returns succeeded
// with a receipt, not-applied only for authoritative settled non-application, or
// unknown. It cannot infer non-application from a timeout or eventually consistent
// absence. The host owns destination semantics and fencing of in-flight work.
type Reconciler func(context.Context, Attempt) (AttemptStatus, string, error)

// Prepare atomically consumes a decision and persists one remote attempt. It does
// not apply local state or send anything. Duplicate preparation cannot grow records.
func (c *Committer[State, Op]) Prepare(ctx context.Context, id string, binding Binding, op Op) (Attempt, error) {
	actor, err := c.actor(ctx)
	if err != nil {
		return Attempt{}, err
	}
	digest, err := OperationDigest(op)
	if err != nil {
		return Attempt{}, err
	}
	var attempt Attempt
	err = c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		i, err := c.verify(s, id, actor, binding, digest, false)
		if err != nil {
			return err
		}
		if unresolvedAttempt(s.Attempts, "") {
			return ErrReconcileRequired
		}
		now := c.cfg.Clock()
		attempt = Attempt{ID: id, DecisionID: id, Actor: actor, Binding: binding, IdempotencyKey: "hitl-" + id, Status: AttemptPrepared, PreparedAt: now, UpdatedAt: now}
		s.Decisions[i].Consumed = true
		s.Attempts = append(s.Attempts, attempt)
		return nil
	})
	if err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

// Send durably marks a prepared attempt sending BEFORE invoking the sender once.
// Errors/cancellation after that boundary leave unknown, or sending if recording
// the outcome fails. Neither status can send again. Recovered prepared attempts
// still verify actor, exact operation, current versions, expiry and revocation.
func (c *Committer[State, Op]) Send(ctx context.Context, id string, op Op, send Sender[Op]) (Attempt, error) {
	if send == nil {
		return Attempt{}, fmt.Errorf("hitl: missing sender")
	}
	actor, err := c.actor(ctx)
	if err != nil {
		return Attempt{}, err
	}
	_, owned, err := ownJSON(op)
	if err != nil {
		return Attempt{}, err
	}
	digest, err := OperationDigest(owned)
	if err != nil {
		return Attempt{}, err
	}
	var attempt Attempt
	err = c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		i := attemptIndex(s.Attempts, id)
		if i < 0 {
			return ErrApproval
		}
		a := s.Attempts[i]
		if unresolvedAttempt(s.Attempts, id) {
			return ErrReconcileRequired
		}
		if a.Actor != actor {
			return ErrApproval
		}
		if a.Status != AttemptPrepared {
			return ErrReconcileRequired
		}
		if _, err := c.verify(s, a.DecisionID, actor, a.Binding, digest, true); err != nil {
			return err
		}
		a.Status = AttemptSending
		a.Sends = 1
		a.UpdatedAt = c.cfg.Clock()
		s.Attempts[i] = a
		attempt = a
		return nil
	})
	if err != nil {
		return Attempt{}, err
	}
	var receipt string
	if err = ctx.Err(); err == nil {
		receipt, err = send(ctx, attempt, owned)
	}
	status := AttemptSucceeded
	if err != nil || receipt == "" {
		status = AttemptUnknown
		receipt = ""
		err = errors.Join(ErrReconcileRequired, err)
	}
	updated, persistErr := c.recordOutcome(ctx, attempt, status, receipt)
	if persistErr != nil {
		return attempt, errors.Join(ErrReconcileRequired, err, persistErr)
	}
	return updated, err
}

// Reconcile records destination knowledge after an unknown outcome or restart
// with sending persisted. It may run after expiry/revocation because observing a
// past effect grants no new authority. No result reopens this decision for sending;
// another send requires a fresh preview and decision after reconciliation.
func (c *Committer[State, Op]) Reconcile(ctx context.Context, id string, reconcile Reconciler) (Attempt, error) {
	if reconcile == nil {
		return Attempt{}, fmt.Errorf("hitl: missing reconciler")
	}
	actor, err := c.actor(ctx)
	if err != nil {
		return Attempt{}, err
	}
	snapshot, err := c.store.View(ctx)
	if err != nil {
		return Attempt{}, err
	}
	i := attemptIndex(snapshot.Attempts, id)
	if i < 0 || snapshot.Attempts[i].Actor != actor {
		return Attempt{}, ErrApproval
	}
	a := snapshot.Attempts[i]
	if a.Status != AttemptSending && a.Status != AttemptUnknown {
		return a, ErrCommitConflict
	}
	status, receipt, err := reconcile(ctx, a)
	if err != nil {
		return a, errors.Join(ErrReconcileRequired, err)
	}
	if (status != AttemptSucceeded && status != AttemptNotApplied && status != AttemptUnknown) || ((status == AttemptSucceeded) != (receipt != "")) {
		return a, fmt.Errorf("hitl: invalid reconciliation result")
	}
	updated, err := c.recordOutcome(ctx, a, status, receipt)
	if err != nil {
		return a, err
	}
	if status == AttemptUnknown {
		return updated, ErrReconcileRequired
	}
	return updated, nil
}

func (c *Committer[State, Op]) recordOutcome(ctx context.Context, previous Attempt, status AttemptStatus, receipt string) (Attempt, error) {
	updated := previous
	updated.Status = status
	updated.Receipt = receipt
	updated.UpdatedAt = c.cfg.Clock()
	if updated.UpdatedAt.Before(previous.UpdatedAt) {
		return Attempt{}, ErrCommitConflict
	}
	err := c.store.Update(ctx, func(s *CommitSnapshot[State]) error {
		i := attemptIndex(s.Attempts, previous.ID)
		if i < 0 || s.Attempts[i] != previous {
			return ErrCommitConflict
		}
		s.Attempts[i] = updated
		return nil
	})
	if err != nil {
		return Attempt{}, err
	}
	return updated, nil
}

func attemptIndex(as []Attempt, id string) int {
	for i, a := range as {
		if a.ID == id {
			return i
		}
	}
	return -1
}

func unresolvedAttempt(as []Attempt, except string) bool {
	for _, a := range as {
		if a.ID != except && (a.Status == AttemptSending || a.Status == AttemptUnknown) {
			return true
		}
	}
	return false
}
