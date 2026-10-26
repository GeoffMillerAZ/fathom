package proseheadless_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	catalogkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
	ph "github.com/geoffmilleraz/fathom/third_party/agentic-kit/scenarios/prose-headless"
)

var expectedRequired = []string{"g-owner", "g-support"}

// corrector is the host source-truth port: only the editor may publish a new revision.
type corrector struct{ src *store }

func (c corrector) Correct(_ context.Context, actor, factID, value string) (prose.Fact, error) {
	if actor != "alice" {
		return prose.Fact{}, errors.New("actor may not revise sources")
	}
	return c.src.publish(strings.TrimPrefix(factID, "fact-"), "v2", value), nil
}

type harness struct {
	t      *testing.T
	src    *store
	client *quoter
	engine *prose.DocumentEngine
	fence  *ph.MemoryFence
	queue  *ph.Queue
	source ph.ProfileSource
	bundle catalogkit.Bundle
	target prose.DocumentTarget
	doc    prose.DocumentVersion
	pub    *ph.Publisher
	obs    func(prose.DocumentVersion) []ph.CriterionObservation
	policy ph.Policy
	mu     sync.Mutex
	remote int
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func as(ctx context.Context, actor, cred string, roles ...string) context.Context {
	return ph.WithPrincipal(ctx, ph.Principal{Actor: actor, Credential: cred, Tenant: "t1", Roles: roles})
}

func (h *harness) alice() context.Context {
	return as(context.Background(), "alice", "alice-key", "editor")
}

func (h *harness) bob() context.Context {
	return as(context.Background(), "bob", "bob-key", "approver")
}

func passAll(doc prose.DocumentVersion) []ph.CriterionObservation {
	var out []ph.CriterionObservation
	for _, g := range doc.Target.Session.Goals {
		out = append(out, ph.CriterionObservation{
			GoalID: g.ID, SubjectDigest: doc.Receipt.DocumentDigest, PolicyEpoch: "e1", Outcome: "pass", Method: "human", Reason: "reviewed by named human", ArtifactDigests: []string{doc.Receipt.DocumentDigest},
			ObservedAt: epoch.Add(-time.Minute), ValidUntil: epoch.Add(time.Hour),
		})
	}
	return out
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	def, err := os.ReadFile("testdata/freeform-brief.json")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t}
	h.src = &store{items: map[prose.SourceRef]prose.SourceSnapshot{}, current: map[string]prose.SourceRef{}}
	facts := []prose.Fact{h.src.publish("owner", "v1", "Alex owns the rollout"), h.src.publish("date", "v1", "Launch is 3 Nov")}
	spec := catalogkit.Spec{Version: 1, Layers: []catalogkit.Layer{{Name: "general", Nodes: []catalog.AuthoredNode{{
		Slug: "freeform-brief", Title: "Freeform brief", StableKey: "freeform-brief", Source: "profiles.json",
		Facets: map[string][]string{ph.DefinitionFacet: {prose.ContentDigest(string(def))}},
	}}}}}
	h.bundle = must(mustBundle(spec))
	profile := must(ph.CatalogProfile(h.bundle, "freeform-brief", def))
	h.source = ph.ProfileSource{Profile: profile, Bundle: &h.bundle, Definition: def}
	session := must(ph.NewSession(h.source, prose.SessionInput{
		ID: "brief-1", Purpose: "Tell the team who owns launch",
		Goals: []prose.Goal{
			{ID: "g-owner", Class: prose.GoalRequired, Statement: "Name the owner", Scope: "body", Method: "exact-support", Acceptance: "owner named", Origin: prose.OriginTask},
			{ID: "g-tone", Class: prose.GoalAdvisory, Statement: "Friendly tone", Scope: prose.ScopeDocument, Method: "exact-support", Acceptance: "reader finds it friendly", Origin: prose.OriginSession},
		},
		ExpectedGoalIDs: []string{"g-owner"},
	}, nil))
	h.target = prose.DocumentTarget{Session: session, Profile: profile, Spec: prose.DocumentSpec{ID: "brief", Nodes: []prose.NodeSpec{
		{ID: "doc", Kind: prose.KindDocument},
		{ID: "body", Kind: prose.KindSection, ParentID: "doc", Order: 1},
		{ID: "owner", Kind: prose.KindParagraph, ParentID: "body", Order: 1, Instruction: "Name the owner", FactIDs: []string{"fact-owner"}},
		{ID: "date", Kind: prose.KindParagraph, ParentID: "body", Order: 2, Instruction: "State the date", FactIDs: []string{"fact-date"}},
	}}, Input: prose.DocumentInput{Principal: "analyst", Facts: facts}}
	h.client = &quoter{}
	h.engine = h.newEngine(h.client)
	h.doc = must(h.engine.Generate(context.Background(), h.target))
	h.policy = ph.Policy{
		Epoch: "e1", Tenant: "t1", Purposes: []string{"internal-brief"}, ApproverRole: "approver", SharedCredentials: []string{"svc-key"},
		Destinations: map[string]ph.Destination{"wiki": {Kind: "local", Account: "acct-1"}, "mail": {Kind: "remote", Account: "acct-2"}},
	}
	h.queue = newQueue(t, h, 8, 4, map[string]string{"fact-date": "v", "owner": "v"})
	h.fence = must(ph.NewMemoryFence(ph.MemoryFenceConfig{Engine: h.engine, Queue: h.queue, Assessment: h.assessCfg(), ExpectedRequired: expectedRequired}))
	h.fence.SetTask([]string{"internal-brief"})
	for _, f := range facts {
		h.fence.SetTruth(f.Span.ID, ph.TruthConfirmed)
	}
	h.fence.SetObservations(passAll(h.doc))
	h.pub = must(ph.NewPublisher(ph.PublisherConfig{Fence: h.fence, Host: h.host(), Clock: func() time.Time { return epoch }, MaxTTL: 10 * time.Minute}, h.doc, h.policy, []string{"alice"}))
	return h
}

func mustBundle(spec catalogkit.Spec) (catalogkit.Bundle, error) {
	return catalogkit.Resolve(spec, catalogkit.Policy{})
}

func (h *harness) newEngine(c llmclient.LLMClient) *prose.DocumentEngine {
	return must(prose.NewDocumentEngine(c, h.src, prose.ExactSupportReviewer{}, prose.DocumentConfig{
		Sourced: prose.SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "configured-model", Limits: prose.DefaultProseLimits()},
		Limits:  prose.DefaultDocumentLimits(),
	}, func() time.Time { return epoch }))
}

func (h *harness) host() ph.RoleHostAuthority {
	return ph.RoleHostAuthority{Role: "host-admin", Tenant: "t1", SharedCredentials: []string{"svc-key"}}
}

func (h *harness) admin() context.Context {
	return as(context.Background(), "root", "root-key", "host-admin")
}

func (h *harness) assessCfg() ph.AssessmentConfig {
	return ph.AssessmentConfig{
		PolicyEpoch: "e1", Now: func() time.Time { return epoch },
		Methods: map[string][]fitness.EvidenceMethod{"exact-support": {fitness.EvidenceMethodDeterministic, fitness.EvidenceMethodHuman}},
	}
}

// op builds the approved operation for the installed version.
func (h *harness) op(purpose, dest, account string) ph.PublishOp {
	return must(h.pub.NewOp(context.Background(), purpose, dest, account, epoch.Add(5*time.Minute)))
}

// successor regenerates date under a new source revision and returns the new version.
func (h *harness) successor(value string) prose.DocumentVersion {
	next := h.target
	next.Input.Facts = []prose.Fact{h.target.Input.Facts[0], h.src.publish("date", "v2", value)}
	res := must(h.engine.Rebuild(context.Background(), h.doc, next, nil, prose.RebuildOptions{}))
	return res.Version
}
