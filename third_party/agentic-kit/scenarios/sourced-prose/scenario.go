package sourcedprose

import (
	"context"
	"errors"
	"fmt"
	"time"

	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit"
	"github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

// Report exposes the inspectable artifact, invalidation and actual effect counts.
// Its synthetic reviewer result establishes only this fixture's expected support;
// transport hashes do not establish semantic truth or production reliability.
type Report struct {
	Export         prose.ProseExport
	Invalidated    prose.SourcedArtifact
	Receipt        hitl.CommitReceipt
	Remote         hitl.Attempt
	LocalEffects   int
	RemoteEffects  int
	GeneratorCalls int
	Denied         []string
}

// Run executes Generate→CheckDraft→Export→Propose→serialize/reconstruct→Commit,
// then correction and remote-recovery failure cases. All inputs, authenticated
// actor, independent review, time and randomness are synthetic and deterministic.
// Memory snapshots demonstrate reconstruction, not a production durable adapter.
func Run(ctx context.Context) (Report, error) {
	var out Report
	if err := ctx.Err(); err != nil {
		return out, err
	}
	ctx = context.WithValue(ctx, actorKey{}, "editor")
	f, err := newFixture()
	if err != nil {
		return out, err
	}
	s, err := f.store.View(ctx)
	if err != nil {
		return out, err
	}
	bad, m, err := f.pipeline(draft(true))
	if err != nil {
		return out, err
	}
	_, err = bad.Generate(ctx, request(s))
	out.GeneratorCalls += m.CallCount()
	if err := expect(err, prose.ErrUnsupported, "14 from 12/10"); err != nil {
		return out, err
	}
	out.Denied = append(out.Denied, "unsupported-14")
	op, calls, err := f.prepare(ctx)
	out.GeneratorCalls += calls
	if err != nil {
		return out, err
	}
	out.Export = op.Export
	c, err := f.committer()
	if err != nil {
		return out, err
	}
	d, _, err := c.Propose(ctx, "publish", op, time.Minute)
	if err != nil {
		return out, err
	}
	if err := f.restart(ctx); err != nil {
		return out, err
	}
	c, err = f.committer()
	if err != nil {
		return out, err
	}
	out.Receipt, err = c.Commit(ctx, d.ID, d.Binding, op)
	if err != nil {
		return out, err
	}
	_, err = c.Commit(ctx, d.ID, d.Binding, op)
	if err := expect(err, hitl.ErrApproval, "local replay"); err != nil {
		return out, err
	}
	out.Denied = append(out.Denied, "local-replay")
	// A second decision is pending when correction arrives; it must not execute.
	stale, _, err := c.Propose(ctx, "publish", op, time.Minute)
	if err != nil {
		return out, err
	}
	if err := f.correct(ctx); err != nil {
		return out, err
	}
	if err := f.restart(ctx); err != nil {
		return out, err
	}
	c, err = f.committer()
	if err != nil {
		return out, err
	}
	_, err = c.Commit(ctx, stale.ID, stale.Binding, op)
	if err := expect(err, hitl.ErrApproval, "corrected approval"); err != nil {
		return out, err
	}
	out.Denied = append(out.Denied, "stale-approval")
	s, err = f.store.View(ctx)
	if err != nil {
		return out, err
	}
	out.Invalidated = s.Target.Value.Artifact
	out.LocalEffects = s.Target.Value.LocalEffects
	p, _, err := f.pipeline(draft(false))
	if err != nil {
		return out, err
	}
	_, err = p.Export(ctx, out.Invalidated, "public", authorizeExport)
	if err := expect(err, prose.ErrArtifact, "invalidated export"); err != nil {
		return out, err
	}
	out.Denied = append(out.Denied, "invalidated-export")
	// Even the retained pre-invalidation request cannot silently resolve to v3.
	_, err = p.CheckDraft(ctx, out.Invalidated.Request, out.Invalidated.Draft)
	if err := expect(err, prose.ErrEvidence, "stale source revision"); err != nil {
		return out, err
	}
	out.Denied = append(out.Denied, "stale-source")
	remote, remoteEffects, remoteCalls, err := runRemote(ctx)
	if err != nil {
		return out, err
	}
	out.Remote = remote
	out.RemoteEffects = remoteEffects
	out.GeneratorCalls += remoteCalls
	out.Denied = append(out.Denied, "unknown-resend", "duplicate-prepare", "reconciled-resend")
	if out.LocalEffects != 1 || out.RemoteEffects != 1 {
		return out, fmt.Errorf("incorrect effect counts: local=%d remote=%d", out.LocalEffects, out.RemoteEffects)
	}
	return out, nil
}

func runRemote(ctx context.Context) (hitl.Attempt, int, int, error) {
	f, err := newFixture()
	if err != nil {
		return hitl.Attempt{}, 0, 0, err
	}
	op, calls, err := f.prepare(ctx)
	if err != nil {
		return hitl.Attempt{}, 0, calls, err
	}
	c, err := f.committer()
	if err != nil {
		return hitl.Attempt{}, 0, calls, err
	}
	d, _, err := c.Propose(ctx, "publish", op, time.Minute)
	if err != nil {
		return hitl.Attempt{}, 0, calls, err
	}
	a, err := c.Prepare(ctx, d.ID, d.Binding, op)
	if err != nil {
		return a, 0, calls, err
	}
	if err := f.restart(ctx); err != nil {
		return a, 0, calls, err
	}
	c, err = f.committer()
	if err != nil {
		return a, 0, calls, err
	}
	effects := 0
	key, operation := "", ""
	sender := func(ctx context.Context, attempt hitl.Attempt, owned publishOp) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		s, err := f.store.View(ctx)
		if err != nil {
			return "", err
		}
		if len(s.Attempts) != 1 || s.Attempts[0].Status != hitl.AttemptSending {
			return "", errors.New("send boundary was not persisted")
		}
		effects++
		key = attempt.IdempotencyKey
		operation = digest(owned)
		return "", errors.New("synthetic destination applied; receipt transport lost")
	}
	a, err = c.Send(ctx, a.ID, op, sender)
	if err := expect(err, hitl.ErrReconcileRequired, "ambiguous send"); err != nil {
		return a, effects, calls, err
	}
	if a.Status != hitl.AttemptUnknown {
		return a, effects, calls, errors.New("ambiguous outcome not retained")
	}
	if err := f.restart(ctx); err != nil {
		return a, effects, calls, err
	}
	c, err = f.committer()
	if err != nil {
		return a, effects, calls, err
	}
	_, err = c.Send(ctx, a.ID, op, sender)
	if err := expect(err, hitl.ErrReconcileRequired, "unknown resend"); err != nil {
		return a, effects, calls, err
	}
	_, err = c.Prepare(ctx, d.ID, d.Binding, op)
	if err == nil {
		return a, effects, calls, errors.New("duplicate prepare succeeded")
	}
	a, err = c.Reconcile(ctx, a.ID, func(ctx context.Context, attempt hitl.Attempt) (hitl.AttemptStatus, string, error) {
		if attempt.IdempotencyKey != key || operation != digest(op) {
			return hitl.AttemptUnknown, "", errors.New("destination binding mismatch")
		}
		return hitl.AttemptSucceeded, "synthetic-destination-receipt", ctx.Err()
	})
	if err != nil {
		return a, effects, calls, err
	}
	_, err = c.Send(ctx, a.ID, op, sender)
	if err := expect(err, hitl.ErrReconcileRequired, "reconciled resend"); err != nil {
		return a, effects, calls, err
	}
	s, err := f.store.View(ctx)
	if err != nil {
		return a, effects, calls, err
	}
	if len(s.Attempts) != 1 || s.Target.Value.LocalEffects != 0 {
		return a, effects, calls, errors.New("remote flow grew attempts or applied locally")
	}
	return a, effects, calls, nil
}
