package proseheadless

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/hitl"
)

// ErrHold means a required condition is unmet, corrupt or unsupported: nothing is
// proposed, committed or sent. Holds carry reasons and never publish.
var ErrHold = errors.New("proseheadless: publication held")

// ErrPublish identifies a malformed or unauthorized publication request.
var ErrPublish = errors.New("proseheadless: publication refused")

// Principal is the host-authenticated caller. The host derives it from its identity
// provider and puts it in the context; nothing model-supplied is ever a Principal.
// Credential names the secret presented (a shared service credential is refused as
// an approver identity even when it carries a human actor name).
type Principal struct {
	Actor      string
	Credential string
	Tenant     string
	Roles      []string
}

type principalKey struct{}

// WithPrincipal attaches the host-authenticated principal to ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	p.Roles = append([]string(nil), p.Roles...)
	return context.WithValue(ctx, principalKey{}, p)
}

// Destination is a policy-approved publication target. Kind is "local" (committed
// atomically with host state) or "remote" (prepare/send/reconcile, never resent).
type Destination struct {
	Kind    string `json:"kind"`
	Account string `json:"account"`
}

// Policy is host-owned and versioned by Epoch. Changing it requires SetPolicy,
// which invalidates outstanding decisions.
type Policy struct {
	Epoch             string                 `json:"epoch"`
	Tenant            string                 `json:"tenant"`
	Purposes          []string               `json:"purposes"`
	Destinations      map[string]Destination `json:"destinations"`
	ApproverRole      string                 `json:"approver_role"`
	SharedCredentials []string               `json:"shared_credentials"`
}

// PublishOp is the exact effect a human approves. Every field is checked against
// the installed document and policy at preview, commit and prepare.
type PublishOp struct {
	DocumentDigest string    `json:"document_digest"`
	MarkdownDigest string    `json:"markdown_digest"`
	SessionDigest  string    `json:"session_digest"`
	ProfileDigest  string    `json:"profile_digest"`
	EvidenceDigest string    `json:"evidence_digest"`
	PolicyEpoch    string    `json:"policy_epoch"`
	Tenant         string    `json:"tenant"`
	Purpose        string    `json:"purpose"`
	Destination    string    `json:"destination"`
	Account        string    `json:"account"`
	NotAfter       time.Time `json:"not_after"`
}

// Published records one local publication effect.
type Published struct {
	DocumentDigest string `json:"document_digest"`
	Destination    string `json:"destination"`
}

// PubState is the authoritative HITL target state. Authors are actors who produced
// or edited the installed version and may not approve it.
type PubState struct {
	Doc          prose.DocumentVersion `json:"doc"`
	MarkdownHash string                `json:"markdown_hash"`
	Policy       Policy                `json:"policy"`
	Authors      []string              `json:"authors"`
	Published    []Published           `json:"published"`
	LocalEffects int                   `json:"local_effects"`
}

// HostAuthority authorizes mutation of installed documents and policy. It is
// separate from reader and approver rights and is required.
type HostAuthority interface {
	Authorize(ctx context.Context, action string) error
}

// RoleHostAuthority permits a non-shared credential holding Role in Tenant.
type RoleHostAuthority struct {
	Role, Tenant      string
	SharedCredentials []string
}

// Authorize implements HostAuthority.
func (r RoleHostAuthority) Authorize(ctx context.Context, action string) error {
	p, ok := principal(ctx)
	if !ok || r.Role == "" || !contains(p.Roles, r.Role) || p.Tenant != r.Tenant || p.Credential == "" || contains(r.SharedCredentials, p.Credential) {
		return fmt.Errorf("%w: %s requires trusted host authority", ErrPublish, action)
	}
	return ctx.Err()
}

// PublisherConfig supplies required host ports. Fence must be a supported
// qualification (FenceSynthetic): an absent, typed-nil or unqualified fence refuses
// construction. Host authorizes Install and SetPolicy.
type PublisherConfig struct {
	Fence    Fence
	Host     HostAuthority
	Clock    func() time.Time
	Random   func(int) ([]byte, error)
	MaxTTL   time.Duration
	Capacity int
}

// Publisher composes document readiness with the existing HITL committer. It is a
// thin policy layer: approval, atomic commit, revocation, consumption and remote
// delivery recovery are hitl's. Render is read-only and never consent. Safe for
// concurrent use when its ports are.
type Publisher struct {
	cfg   PublisherConfig
	store *hitl.MemoryCommitStore[PubState]
	hitl  *hitl.Committer[PubState, PublishOp]
}

func jsonDigest(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return prose.ContentDigest(string(b))
}

func resourceState(doc prose.DocumentVersion, st PubState) hitl.ResourceState[PubState] {
	return hitl.ResourceState[PubState]{
		Resource: "document:" + doc.Target.Spec.ID, Version: 1, EvidenceDigest: doc.Receipt.EvidenceDigest,
		ConfigDigest: jsonDigest([]string{doc.Receipt.SessionDigest, doc.Receipt.ProfileDigest, doc.Receipt.ConfigDigest, st.Policy.Epoch}), Value: st,
	}
}

func copyPolicy(p Policy) Policy {
	p.Purposes = append([]string(nil), p.Purposes...)
	p.SharedCredentials = append([]string(nil), p.SharedCredentials...)
	d := make(map[string]Destination, len(p.Destinations))
	for k, v := range p.Destinations {
		d[k] = v
	}
	p.Destinations = d
	return p
}

func validPolicy(p Policy) error {
	if p.Epoch == "" || p.Tenant == "" || len(p.Purposes) == 0 || len(p.Destinations) == 0 || p.ApproverRole == "" {
		return fmt.Errorf("%w: policy needs epoch, tenant, purposes, destinations and approver role", ErrPublish)
	}
	for name, d := range p.Destinations {
		if name == "" || d.Account == "" || (d.Kind != "local" && d.Kind != "remote") {
			return fmt.Errorf("%w: destination %q invalid", ErrPublish, name)
		}
	}
	return nil
}

// NewPublisher installs an initial document version under a policy. The first
// version may be installed only by non-approver authors; callers list them.
func NewPublisher(cfg PublisherConfig, doc prose.DocumentVersion, policy Policy, authors []string) (*Publisher, error) {
	fence, qualified := cfg.Fence.(*MemoryFence)
	if !qualified || fence == nil || nilPort(cfg.Host) || cfg.Clock == nil || cfg.MaxTTL <= 0 {
		return nil, fmt.Errorf("%w: a qualified fence, host authority, clock and TTL are required", ErrPublish)
	}
	switch host := cfg.Host.(type) {
	case RoleHostAuthority:
		host.SharedCredentials = append([]string(nil), host.SharedCredentials...)
		cfg.Host = host
	case *RoleHostAuthority:
		owned := *host
		owned.SharedCredentials = append([]string(nil), host.SharedCredentials...)
		cfg.Host = owned
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = 32
	}
	policy = copyPolicy(policy)
	if err := validPolicy(policy); err != nil {
		return nil, err
	}
	rd, err := prose.RenderMarkdown(doc)
	if err != nil {
		return nil, err
	}
	st := PubState{Doc: doc, MarkdownHash: rd.MarkdownDigest, Policy: policy, Authors: append([]string(nil), authors...)}
	p := &Publisher{cfg: cfg}
	if p.store, err = hitl.NewMemoryCommitStore(resourceState(doc, st), cfg.Capacity); err != nil {
		return nil, err
	}
	engine := hitl.New(p.apply, func(s PubState) PubState {
		b, _ := json.Marshal(s)
		var out PubState
		if err := json.Unmarshal(b, &out); err != nil {
			panic(err)
		}
		return out
	})
	random := cfg.Random
	if random == nil {
		random = func(n int) ([]byte, error) {
			b := make([]byte, n)
			_, err := io.ReadFull(rand.Reader, b)
			return b, err
		}
	}
	p.hitl, err = hitl.NewCommitter(engine, p.store, hitl.CommitConfig{Authenticate: p.authenticate, Approve: p.approve, Clock: cfg.Clock, Random: random, MaxTTL: cfg.MaxTTL})
	return p, err
}

func principal(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	p.Roles = append([]string(nil), p.Roles...)
	return p, ok && p.Actor != ""
}

// authenticate derives identity from the host principal and refuses shared
// credentials, so a service secret cannot act as a distinct human decision.
func (p *Publisher) authenticate(ctx context.Context) (string, error) {
	pr, ok := principal(ctx)
	if !ok {
		return "", errors.New("no authenticated principal")
	}
	s, err := p.store.View(ctx)
	if err != nil {
		return "", err
	}
	pol := s.Target.Value.Policy
	if contains(pol.SharedCredentials, pr.Credential) || pr.Credential == "" {
		return "", errors.New("shared or missing credential cannot act as a principal")
	}
	if pr.Tenant != pol.Tenant {
		return "", errors.New("principal tenant differs from policy tenant")
	}
	return pr.Actor, nil
}

// approve is the human decision gate: approver role, not an author of this
// version, and the exact operation already validated by the preview.
func (p *Publisher) approve(ctx context.Context, actor string, b hitl.Binding, pv hitl.Preview) (bool, error) {
	pr, _ := principal(ctx)
	s, err := p.store.View(ctx)
	if err != nil {
		return false, err
	}
	st := s.Target.Value
	if !pv.OK || b.Action != "publish" || !contains(pr.Roles, st.Policy.ApproverRole) || contains(st.Authors, actor) {
		return false, nil
	}
	return true, ctx.Err()
}

func validateOp(st PubState, op PublishOp, now time.Time) error {
	d := st.Doc.Receipt
	dest, ok := st.Policy.Destinations[op.Destination]
	switch {
	case op.DocumentDigest != d.DocumentDigest || op.SessionDigest != d.SessionDigest || op.ProfileDigest != d.ProfileDigest || op.EvidenceDigest != d.EvidenceDigest || op.MarkdownDigest != st.MarkdownHash:
		return fmt.Errorf("%w: operation does not bind the installed document", ErrPublish)
	case op.PolicyEpoch != st.Policy.Epoch || op.Tenant != st.Policy.Tenant:
		return fmt.Errorf("%w: policy epoch or tenant differs", ErrPublish)
	case !contains(st.Policy.Purposes, op.Purpose):
		return fmt.Errorf("%w: purpose not permitted", ErrPublish)
	case !ok || dest.Account != op.Account:
		return fmt.Errorf("%w: destination or account not permitted", ErrPublish)
	case op.NotAfter.IsZero() || !now.Before(op.NotAfter):
		return fmt.Errorf("%w: operation expired", ErrPublish)
	}
	return nil
}

func (p *Publisher) apply(st *PubState, op PublishOp) (string, error) {
	if err := validateOp(*st, op, p.cfg.Clock()); err != nil {
		return "", err
	}
	if st.Policy.Destinations[op.Destination].Kind == "local" {
		st.LocalEffects++
		st.Published = append(st.Published, Published{DocumentDigest: op.DocumentDigest, Destination: op.Destination})
		return "publish document " + op.DocumentDigest[:12] + " to " + op.Destination, nil
	}
	return "remote delivery of " + op.DocumentDigest[:12] + " to " + op.Destination + " prepared", nil
}

// State returns a copy of the authoritative state.
func (p *Publisher) State(ctx context.Context) (hitl.CommitSnapshot[PubState], error) {
	return p.store.View(ctx)
}

// Install replaces the current document with a successor. The successor must name
// the installed version as its parent, so two concurrent writers cannot both win.
// Authors accumulate. The target version advances, so every outstanding decision
// becomes stale. Installing is not approval.
func (p *Publisher) Install(ctx context.Context, doc prose.DocumentVersion, authors []string) error {
	if err := p.cfg.Host.Authorize(ctx, "install"); err != nil {
		return err
	}
	authors = append([]string(nil), authors...)
	rd, err := prose.RenderMarkdown(doc)
	if err != nil {
		return err
	}
	return p.store.Update(ctx, func(s *hitl.CommitSnapshot[PubState]) error {
		st := &s.Target.Value
		if doc.Parent != st.Doc.Receipt.DocumentDigest || doc.Target.Spec.ID != st.Doc.Target.Spec.ID {
			return fmt.Errorf("%w: successor does not extend the installed version", ErrPublish)
		}
		st.Doc, st.MarkdownHash = doc, rd.MarkdownDigest
		st.Authors = append(st.Authors, authors...)
		rs := resourceState(doc, *st)
		s.Target.EvidenceDigest, s.Target.ConfigDigest = rs.EvidenceDigest, rs.ConfigDigest
		s.Target.Version++
		return nil
	})
}

// SetPolicy installs a new policy epoch and invalidates outstanding decisions.
func (p *Publisher) SetPolicy(ctx context.Context, policy Policy) error {
	if err := p.cfg.Host.Authorize(ctx, "set-policy"); err != nil {
		return err
	}
	policy = copyPolicy(policy)
	if err := validPolicy(policy); err != nil {
		return err
	}
	return p.store.Update(ctx, func(s *hitl.CommitSnapshot[PubState]) error {
		st := &s.Target.Value
		if policy.Epoch == st.Policy.Epoch {
			return fmt.Errorf("%w: policy epoch must change", ErrPublish)
		}
		st.Policy = policy
		s.Target.ConfigDigest = resourceState(st.Doc, *st).ConfigDigest
		s.Target.Version++
		return nil
	})
}

func (p *Publisher) scope(ctx context.Context, op PublishOp) (Scope, prose.DocumentVersion, error) {
	pr, ok := principal(ctx)
	if !ok {
		return Scope{}, prose.DocumentVersion{}, &HoldError{Reasons: []string{"identity:unauthenticated"}}
	}
	s, err := p.store.View(ctx)
	if err != nil {
		return Scope{}, prose.DocumentVersion{}, err
	}
	if err := validateOp(s.Target.Value, op, p.cfg.Clock()); err != nil {
		return Scope{}, prose.DocumentVersion{}, &HoldError{Reasons: []string{err.Error()}}
	}
	return Scope{
		DocumentDigest: op.DocumentDigest, SessionDigest: op.SessionDigest, ProfileDigest: op.ProfileDigest, EvidenceDigest: op.EvidenceDigest, PolicyEpoch: op.PolicyEpoch,
		Tenant: op.Tenant, Purpose: op.Purpose, Destination: op.Destination, Account: op.Account, Actor: pr.Actor,
	}, s.Target.Value.Doc, nil
}

// Readiness evaluates every required condition for op under the fence and releases
// it at once. It is advisory: Commit, Prepare and Send enter the fence again and
// hold it across the effect. It never changes state and is not consent.
func (p *Publisher) Readiness(ctx context.Context, op PublishOp) ([]string, error) {
	sc, doc, err := p.scope(ctx, op)
	if err == nil {
		var rel func()
		if rel, err = p.cfg.Fence.Enter(ctx, sc, doc); err == nil {
			rel()
			return nil, nil
		}
	}
	var h *HoldError
	if errors.As(err, &h) {
		return h.Reasons, nil
	}
	return nil, err
}

// enter acquires the fence for op. The returned release must be called after the effect.
func (p *Publisher) enter(ctx context.Context, op PublishOp) (func(), error) {
	sc, doc, err := p.scope(ctx, op)
	if err != nil {
		return nil, err
	}
	return p.cfg.Fence.Enter(ctx, sc, doc)
}

// NewOp builds the exact operation for the installed version. It authorizes nothing.
func (p *Publisher) NewOp(ctx context.Context, purpose, destination, account string, notAfter time.Time) (PublishOp, error) {
	s, err := p.store.View(ctx)
	if err != nil {
		return PublishOp{}, err
	}
	st, d := s.Target.Value, s.Target.Value.Doc.Receipt
	return PublishOp{
		DocumentDigest: d.DocumentDigest, MarkdownDigest: st.MarkdownHash, SessionDigest: d.SessionDigest, ProfileDigest: d.ProfileDigest, EvidenceDigest: d.EvidenceDigest,
		PolicyEpoch: st.Policy.Epoch, Tenant: st.Policy.Tenant, Purpose: purpose, Destination: destination, Account: account, NotAfter: notAfter,
	}, nil
}

// Propose requires readiness, then asks the existing committer for a human
// decision bound to the exact operation and current target version. The caller
// context must carry the approver principal.
func (p *Publisher) Propose(ctx context.Context, op PublishOp, ttl time.Duration) (hitl.Decision, error) {
	rel, err := p.enter(ctx, op)
	if err != nil {
		return hitl.Decision{}, err
	}
	rel()
	d, _, err := p.hitl.Propose(ctx, "publish", op, ttl)
	return d, err
}

// Commit re-checks readiness at commit time, then commits locally through hitl.
// Remote destinations must use Prepare.
func (p *Publisher) Commit(ctx context.Context, d hitl.Decision, op PublishOp) (hitl.CommitReceipt, error) {
	s, err := p.store.View(ctx)
	if err != nil {
		return hitl.CommitReceipt{}, err
	}
	if s.Target.Value.Policy.Destinations[op.Destination].Kind != "local" {
		return hitl.CommitReceipt{}, fmt.Errorf("%w: remote destination requires Prepare/Send", ErrPublish)
	}
	rel, err := p.enter(ctx, op)
	if err != nil {
		return hitl.CommitReceipt{}, err
	}
	defer rel()
	return p.hitl.Commit(ctx, d.ID, d.Binding, op)
}

// Prepare re-checks readiness and the operation, then consumes the decision into
// one durable remote attempt through hitl.
func (p *Publisher) Prepare(ctx context.Context, d hitl.Decision, op PublishOp) (hitl.Attempt, error) {
	s, err := p.store.View(ctx)
	if err != nil {
		return hitl.Attempt{}, err
	}
	if s.Target.Value.Policy.Destinations[op.Destination].Kind != "remote" {
		return hitl.Attempt{}, fmt.Errorf("%w: local destination requires Commit", ErrPublish)
	}
	rel, err := p.enter(ctx, op)
	if err != nil {
		return hitl.Attempt{}, err
	}
	defer rel()
	return p.hitl.Prepare(ctx, d.ID, d.Binding, op)
}

// Send invokes the sender at most once for a prepared attempt, with the fence held
// across the dispatch (the sender must not call back into the fence). Any ambiguity
// requires Reconcile and is never resent.
func (p *Publisher) Send(ctx context.Context, id string, op PublishOp, send hitl.Sender[PublishOp]) (hitl.Attempt, error) {
	if send == nil {
		return hitl.Attempt{}, fmt.Errorf("%w: sender required", ErrPublish)
	}
	// The fence is held across dispatch: authority cannot change between the
	// check and the sender start. A held fence means the sender is never invoked.
	rel, err := p.enter(ctx, op)
	if err != nil {
		return hitl.Attempt{}, err
	}
	defer rel()
	return p.hitl.Send(ctx, id, op, send)
}

// Reconcile records destination knowledge. A not-applied outcome still requires a
// fresh decision for any further send.
func (p *Publisher) Reconcile(ctx context.Context, id string, r hitl.Reconciler) (hitl.Attempt, error) {
	return p.hitl.Reconcile(ctx, id, r)
}

// Revoke disables a pending decision for its approving actor.
func (p *Publisher) Revoke(ctx context.Context, d hitl.Decision) error {
	return p.hitl.Revoke(ctx, d.ID)
}

// PrincipalFrom returns the host-authenticated principal attached by WithPrincipal.
func PrincipalFrom(ctx context.Context) (Principal, bool) { return principal(ctx) }
