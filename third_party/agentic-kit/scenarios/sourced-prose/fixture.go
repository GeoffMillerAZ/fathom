// Package sourcedprose is a synthetic, offline composition of strict sourced
// prose and durable HITL approval. Run owns all mutable state; concurrent Run
// calls are independent. No model, network, disk persistence or real identity
// provider is contacted. This is an integration proof, not consumer adoption.
package sourcedprose

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	grade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	span "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	author "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
	prose "github.com/geoffmilleraz/agentic-kit/kits/governed-prose-kit"
	"github.com/geoffmilleraz/agentic-kit/kits/hitl"
)

const comparison = "Count increased from 10 to 12, a difference of 2."
const statusText = "Status: verified."

type actorKey struct{}
type state struct {
	Sources      []prose.SourceSnapshot
	Current      map[string]prose.SourceRef
	Facts        []prose.Fact
	Artifact     prose.SourcedArtifact
	Prepared     prose.ProseExport
	History      []prose.ProseExport
	LocalEffects int
}
type publishOp struct {
	Export       prose.ProseExport
	ConfigDigest string
}
type fixture struct {
	store    *hitl.MemoryCommitStore[state]
	config   prose.SourcedConfig
	sequence atomic.Uint64
	epoch    time.Time
}

func newFixture() (*fixture, error) {
	f := &fixture{epoch: time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC), config: prose.SourcedConfig{
		PolicyVersion: "comparison-policy/v1", PromptVersion: "facts-to-prose/v1", Model: "synthetic-generator", Limits: prose.ProseLimits{MaxFacts: 8, MaxUnits: 4, MaxClaims: 4, MaxBytes: 65536, MaxTokens: 512},
	}}
	current, curFact := f.source("count", "v2", "12", "current", false)
	historical, oldFact := f.source("count", "v1", "10", "historical", true)
	status, statusFact := f.source("status", "v1", "verified", "status", false)
	delta := prose.Fact{Span: span.Span{ID: "delta", EntityID: "record", Field: "change", Value: "2", Confidence: 1,
		OccurredAt: f.epoch.Add(-2 * time.Hour), ObservedAt: f.epoch.Add(-time.Hour), Author: author.NewHuman("fixture-calculator"),
		Grade: &grade.Grade{Tier: grade.TierDocumentary, Confidence: 1}}, Unit: "items",
		Derivation: &prose.Derivation{Version: "arithmetic/v1", Operation: "difference", Inputs: []string{"current", "historical"}}}
	s := state{Sources: []prose.SourceSnapshot{current, historical, status}, Current: map[string]prose.SourceRef{"count": current.Ref, "status": status.Ref}, Facts: []prose.Fact{curFact, oldFact, delta, statusFact}}
	var err error
	f.store, err = hitl.NewMemoryCommitStore(hitl.ResourceState[state]{Resource: "synthetic-publication", Version: 1, EvidenceDigest: digest(s.Facts), ConfigDigest: digest(f.config), Value: s}, 16)
	return f, err
}

func (f *fixture) source(id, version, value, factID string, historical bool) (prose.SourceSnapshot, prose.Fact) {
	r := prose.SourceRef{ID: id, Version: version, Digest: prose.ContentDigest(value)}
	s := prose.SourceSnapshot{Ref: r, Content: value, MediaType: "text/plain", AcquiredBy: "synthetic-fixture", AcquiredAt: f.epoch.Add(-time.Hour), Access: "public", RetainUntil: f.epoch.Add(time.Hour)}
	v := span.Span{ID: factID, EntityID: "record", Field: id, Value: value, Confidence: 1, SourceID: id, RecordID: version, RecordHash: r.Digest,
		OccurredAt: f.epoch.Add(-2 * time.Hour), ObservedAt: f.epoch.Add(-time.Hour), Locator: span.Locator{Kind: "byte-range", Value: id, Start: 0, End: len(value) - 1},
		Author: author.NewHuman("fixture-extractor"), Grade: &grade.Grade{Tier: grade.TierDocumentary, Confidence: 1}}
	return s, prose.Fact{Span: v, Source: r, Unit: "items", Historical: historical}
}

// storedSources deliberately has no separate source map or current-pointer cache.
type storedSources struct {
	store hitl.TransactionStore[state]
	now   func() time.Time
}

func (p storedSources) Resolve(ctx context.Context, principal string, ref prose.SourceRef) (prose.SourceSnapshot, error) {
	if !authenticated(ctx, principal) {
		return prose.SourceSnapshot{}, prose.ErrEvidence
	}
	s, err := p.store.View(ctx)
	if err != nil {
		return prose.SourceSnapshot{}, err
	}
	for _, source := range s.Target.Value.Sources {
		if source.Ref == ref && !source.Revoked && source.RetainUntil.After(p.now()) && source.Access == "public" {
			return source, nil
		}
	}
	return prose.SourceSnapshot{}, prose.ErrEvidence
}
func (p storedSources) Current(ctx context.Context, principal, id string) (prose.SourceRef, error) {
	if !authenticated(ctx, principal) {
		return prose.SourceRef{}, prose.ErrEvidence
	}
	s, err := p.store.View(ctx)
	if err != nil {
		return prose.SourceRef{}, err
	}
	r, ok := s.Target.Value.Current[id]
	if !ok {
		return prose.SourceRef{}, prose.ErrEvidence
	}
	for _, source := range s.Target.Value.Sources {
		if source.Ref == r && !source.Revoked && source.RetainUntil.After(p.now()) && source.Access == "public" {
			return r, nil
		}
	}
	return prose.SourceRef{}, prose.ErrEvidence
}
func authenticated(ctx context.Context, principal string) bool {
	actor, _ := ctx.Value(actorKey{}).(string)
	return actor == "editor" && actor == principal
}

// fixtureReviewer is an independent, narrowly specified semantic oracle. It is
// not ExactSupportReviewer with a historical bypass, nor a general entailment model.
type fixtureReviewer struct{}

func (fixtureReviewer) Review(ctx context.Context, r prose.SupportReview) (prose.SupportDecision, error) {
	d := prose.SupportDecision{Status: "unsupported", Reviewer: "fixture-reviewer", Method: "independent-comparison-fixture/v1", Reason: "wording not supported by this fixture"}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	if r.Claim == nil {
		return d, nil
	}
	facts := map[string]prose.Fact{}
	for _, f := range r.Facts {
		facts[f.Span.ID] = f
	}
	c := r.Claim
	// Expected labels intentionally do not reuse the generator's text constants.
	if c.Assertion == "Count increased from 10 to 12, a difference of 2." && len(c.FactIDs) == 1 && c.FactIDs[0] == "delta" && len(c.Contradictions) == 0 &&
		facts["current"].Span.Value == "12" && !facts["current"].Historical && facts["historical"].Span.Value == "10" && facts["historical"].Historical && facts["delta"].Span.Value == "2" {
		d.Status = "supported"
		d.Reason = "explicit historical comparison and exact difference"
	}
	if c.Assertion == "Status: verified." && len(c.FactIDs) == 1 && c.FactIDs[0] == "status" && facts["status"].Span.Value == "verified" && len(c.Contradictions) == 0 {
		d.Status = "supported"
		d.Reason = "exact fixture status wording"
	}
	return d, nil
}

func draft(unsupported bool) prose.ProseDraft {
	text, ids := comparison, []string{"delta"}
	if unsupported {
		text = "14"
		ids = []string{"current", "historical"}
	}
	return prose.ProseDraft{Claims: []prose.ProseClaim{
		{ID: "comparison", Assertion: text, FactIDs: ids, Contradictions: []string{}}, {ID: "status", Assertion: statusText, FactIDs: []string{"status"}, Contradictions: []string{}},
	}, Units: []prose.ProseUnit{
		{ID: "comparison-unit", Text: text, Claims: []prose.ClaimRange{{ClaimID: "comparison", End: len(text)}}},
		{ID: "status-unit", Text: statusText, Claims: []prose.ClaimRange{{ClaimID: "status", End: len(statusText)}}},
	}}
}
func (f *fixture) pipeline(d prose.ProseDraft) (*prose.SourcedPipeline, *llmclient.MockClient, error) {
	b, _ := json.Marshal(d)
	m := llmclient.NewMockClient(llmclient.MockEntryOK(string(b)))
	p, err := prose.NewSourcedPipeline(m, storedSources{f.store, func() time.Time { return f.epoch }}, fixtureReviewer{}, f.config, func() time.Time { return f.epoch })
	return p, m, err
}
func (f *fixture) committer() (*hitl.Committer[state, publishOp], error) {
	e := hitl.New(func(s *state, op publishOp) (string, error) {
		if digest(op.Export) != digest(s.Prepared) || op.Export.ArtifactDigest == "" || op.Export.EvidenceDigest != digest(s.Facts) || op.ConfigDigest != digest(f.config) {
			return "", prose.ErrArtifact
		}
		if err := prose.VerifyProseExport(op.Export); err != nil {
			return "", err
		}
		s.LocalEffects++
		s.History = append(s.History, op.Export)
		return "publish checked prose and its citation sidecar", nil
	}, func(s state) state {
		b, _ := json.Marshal(s)
		var out state
		if err := json.Unmarshal(b, &out); err != nil {
			panic(err)
		}
		return out
	})
	return hitl.NewCommitter(e, f.store, hitl.CommitConfig{
		Authenticate: func(ctx context.Context) (string, error) {
			if !authenticated(ctx, "editor") {
				return "", hitl.ErrApproval
			}
			return "editor", nil
		},
		Approve: func(ctx context.Context, actor string, b hitl.Binding, p hitl.Preview) (bool, error) {
			return actor == "editor" && b.Action == "publish" && p.OK, ctx.Err()
		},
		Clock: func() time.Time { return f.epoch }, MaxTTL: time.Minute,
		Random: func(n int) ([]byte, error) {
			b := make([]byte, n)
			binary.BigEndian.PutUint64(b[n-8:], f.sequence.Add(1))
			return b, nil
		},
	})
}

func digest(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return prose.ContentDigest(string(b))
}

func (f *fixture) restart(ctx context.Context) error {
	s, err := f.store.View(ctx)
	if err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var recovered hitl.CommitSnapshot[state]
	if err := json.Unmarshal(b, &recovered); err != nil {
		return err
	}
	f.store, err = hitl.RestoreMemoryCommitStore(recovered, 16)
	return err
}

func authorizeExport(ctx context.Context, principal string, r prose.ProseReceipt, audience string) error {
	if !authenticated(ctx, principal) || audience != "public" || r.Integrity != "pass" || r.ArtifactDigest == "" || r.EvidenceDigest == "" || r.ConfigDigest == "" {
		return errors.New("export preparation not authorized")
	}
	return ctx.Err() // Deliberately no mutation, approval consumption or side effect.
}

func request(s hitl.CommitSnapshot[state]) prose.SourcedRequest {
	return prose.SourcedRequest{Principal: "editor", Topic: "synthetic count comparison and independent status", Facts: s.Target.Value.Facts}
}

func (f *fixture) prepare(ctx context.Context) (publishOp, int, error) {
	s, err := f.store.View(ctx)
	if err != nil {
		return publishOp{}, 0, err
	}
	p, m, err := f.pipeline(draft(false))
	if err != nil {
		return publishOp{}, 0, err
	}
	a, err := p.Generate(ctx, request(s))
	if err != nil {
		return publishOp{}, m.CallCount(), err
	}
	a, err = p.CheckDraft(ctx, a.Request, a.Draft)
	if err != nil {
		return publishOp{}, m.CallCount(), err
	}
	x, err := p.Export(ctx, a, "public", authorizeExport)
	if err != nil {
		return publishOp{}, m.CallCount(), err
	}
	// Bind observations to authoritative state before requesting approval. This
	// CAS closes correction races across generation, review and export callbacks.
	err = f.install(ctx, s, a, x)
	return publishOp{Export: x, ConfigDigest: a.Receipt.ConfigDigest}, m.CallCount(), err
}

func (f *fixture) install(ctx context.Context, s hitl.CommitSnapshot[state], a prose.SourcedArtifact, x prose.ProseExport) error {
	return f.store.Update(ctx, func(current *hitl.CommitSnapshot[state]) error {
		if current.Target.Version != s.Target.Version || current.Target.EvidenceDigest != a.Receipt.EvidenceDigest || current.Target.ConfigDigest != a.Receipt.ConfigDigest {
			return hitl.ErrApproval
		}
		current.Target.Value.Artifact = a
		current.Target.Value.Prepared = x
		current.Target.Version++
		return nil
	})
}

func (f *fixture) correct(ctx context.Context) error {
	source, fact := f.source("count", "v3", "13", "current", false)
	return f.store.Update(ctx, func(s *hitl.CommitSnapshot[state]) error {
		invalid, err := prose.InvalidateSource(s.Target.Value.Artifact, "count")
		if err != nil {
			return err
		}
		s.Target.Value.Sources = append(s.Target.Value.Sources, source)
		s.Target.Value.Current["count"] = source.Ref
		s.Target.Value.Facts[0] = fact
		s.Target.Value.Facts[2].Span.Value = "3"
		s.Target.Value.Artifact = invalid
		s.Target.Value.Prepared = prose.ProseExport{}
		s.Target.EvidenceDigest = digest(s.Target.Value.Facts)
		s.Target.Version++
		return nil
	})
}

func expect(err, target error, step string) error {
	if !errors.Is(err, target) {
		return fmt.Errorf("%s: wanted %v, got %v", step, target, err)
	}
	return nil
}
