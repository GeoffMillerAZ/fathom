package proseheadless_test

import (
	"context"
	"errors"
	"testing"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
	ph "github.com/geoffmilleraz/fathom/third_party/agentic-kit/scenarios/prose-headless"
)

type impersonatedFence struct{ noopFence }

func (impersonatedFence) Qualification() string { return ph.FenceSynthetic }

func TestFenceNameCannotImpersonateQualifiedImplementation(t *testing.T) {
	h := newHarness(t)
	_, err := ph.NewPublisher(ph.PublisherConfig{Fence: impersonatedFence{}, Host: h.host(), Clock: func() time.Time { return epoch }, MaxTTL: time.Minute}, h.doc, h.policy, nil)
	if !errors.Is(err, ph.ErrPublish) {
		t.Fatalf("matching-name no-op fence admitted: %v", err)
	}
}

func TestFenceOwnsAssessmentMethods(t *testing.T) {
	h := newHarness(t)
	methods := map[string][]fitness.EvidenceMethod{"exact-support": {fitness.EvidenceMethodHuman}}
	cfg := h.assessCfg()
	cfg.Methods = methods
	f := must(ph.NewMemoryFence(ph.MemoryFenceConfig{Engine: h.engine, Queue: h.queue, Assessment: cfg, ExpectedRequired: expectedRequired}))
	methods["exact-support"][0] = fitness.EvidenceMethodHeuristic
	methods["exact-support"] = []fitness.EvidenceMethod{fitness.EvidenceMethodHeuristic}
	f.SetTask([]string{"internal-brief"})
	for _, fact := range h.target.Input.Facts {
		f.SetTruth(fact.Span.ID, ph.TruthConfirmed)
	}
	obs := passAll(h.doc)
	for i := range obs {
		obs[i].Method = "heuristic"
	}
	f.SetObservations(obs)
	pub := must(ph.NewPublisher(ph.PublisherConfig{Fence: f, Host: h.host(), Clock: func() time.Time { return epoch }, MaxTTL: time.Minute}, h.doc, h.policy, nil))
	op := must(pub.NewOp(context.Background(), "internal-brief", "wiki", "acct-1", epoch.Add(time.Minute)))
	if _, err := pub.Propose(h.bob(), op, time.Minute); !errors.Is(err, ph.ErrHold) {
		t.Fatalf("caller changed eligible evidence policy: %v", err)
	}
}

func TestQueueOwnsForbiddenActors(t *testing.T) {
	h := newHarness(t)
	forbidden := []string{"generator"}
	q := must(ph.NewQueue(ph.QueueConfig{Authenticate: func(context.Context) (string, error) { return "generator", nil }, MayAnswer: func(string, ph.Question) bool { return true }, Forbidden: forbidden, CurrentVersion: func(context.Context, string) (string, error) { return "v", nil }, Clock: func() time.Time { return epoch }, MaxOpen: 4}))
	forbidden[0] = "other"
	must(q.Add(ph.Question{ID: "q", Kind: "ambiguity", Subject: "owner", SubjectVersion: "v", DocumentDigest: h.doc.Receipt.DocumentDigest, Evidence: []string{"fact-owner"}, Prompt: "tone?"}))
	if _, err := q.Answer(context.Background(), ph.Answer{QuestionID: "q", Outcome: ph.OutcomeAnnotate, Text: "plain", SeenVersion: "v"}); !errors.Is(err, ph.ErrActor) {
		t.Fatalf("caller removed forbidden actor: %v", err)
	}
}

func TestAnswerApplicationBindsExactDocumentAndCurrentSubject(t *testing.T) {
	h := newHarness(t)
	versions := map[string]string{"owner": "v"}
	q := newQueue(t, h, 4, 0, versions)
	must(q.Add(ph.Question{ID: "q", Kind: "ambiguity", Subject: "owner", SubjectVersion: "v", DocumentDigest: h.doc.Receipt.DocumentDigest, Evidence: []string{"fact-owner"}, Prompt: "tone?"}))
	must(q.Answer(h.alice(), ph.Answer{QuestionID: "q", Outcome: ph.OutcomeAnnotate, Text: "plain", SeenVersion: "v"}))
	next := h.target
	next.Spec.Nodes = append([]prose.NodeSpec(nil), h.target.Spec.Nodes...)
	next.Spec.Nodes[2].Instruction = "Describe the owner plainly"
	other := must(h.engine.Generate(context.Background(), next))
	if _, _, err := ph.ApplyAnswers(context.Background(), other, q, []string{"q"}, nil); !errors.Is(err, ph.ErrStale) {
		t.Fatalf("answer replayed against foreign document: %v", err)
	}
	if _, _, err := ph.ApplyAnswers(context.Background(), h.doc, q, []string{"q", "q"}, nil); !errors.Is(err, ph.ErrQueue) {
		t.Fatalf("duplicate application admitted: %v", err)
	}
	versions["owner"] = "changed"
	if _, _, err := ph.ApplyAnswers(context.Background(), h.doc, q, []string{"q"}, nil); !errors.Is(err, ph.ErrStale) {
		t.Fatalf("answer replayed after subject change: %v", err)
	}
}

func TestFenceExcludesDirectQueueWritersThroughEffect(t *testing.T) {
	h := newHarness(t)
	h.fence.AfterEnter = func() {
		if ph.QueueMutationAvailableForTest(h.queue) {
			t.Fatal("direct queue writer can interleave with effect")
		}
	}
	op := h.op("internal-brief", "wiki", "acct-1")
	d := must(h.pub.Propose(h.bob(), op, time.Minute))
	must(h.pub.Commit(h.bob(), d, op))
	if !ph.QueueMutationAvailableForTest(h.queue) {
		t.Fatal("queue mutation gate was not released")
	}
	must(h.queue.Add(ph.Question{ID: "new", Kind: "missing", Subject: "owner", SubjectVersion: "v", DocumentDigest: h.doc.Receipt.DocumentDigest, Evidence: []string{"fact-owner"}, Prompt: "needed", Required: true}))
	if ok, _ := h.queue.Ready(); ok {
		t.Fatal("required direct mutation disappeared")
	}
}

func TestAssessmentRejectsStalePolicyOutsideFence(t *testing.T) {
	h := newHarness(t)
	obs := passAll(h.doc)
	obs[0].PolicyEpoch = "foreign"
	if _, err := ph.AssessGoals(h.assessCfg(), h.doc.Target.Session, h.doc.Receipt.DocumentDigest, expectedRequired, obs); !errors.Is(err, ph.ErrAssessment) {
		t.Fatalf("stale policy evidence accepted: %v", err)
	}
}

func TestSuppliedHostAuthorityDenylistIsOwned(t *testing.T) {
	h := newHarness(t)
	host := h.host()
	policy := h.policy
	policy.SharedCredentials = nil
	pub := must(ph.NewPublisher(ph.PublisherConfig{Fence: h.fence, Host: host, Clock: func() time.Time { return epoch }, MaxTTL: time.Minute}, h.doc, policy, nil))
	host.SharedCredentials[0] = "other"
	policy.Epoch = "e2"
	ctx := as(context.Background(), "root", "svc-key", "host-admin")
	if err := pub.SetPolicy(ctx, policy); !errors.Is(err, ph.ErrPublish) {
		t.Fatalf("caller removed host credential denylist: %v", err)
	}
}

func TestAnswerTargetsDoNotAliasPriorVersion(t *testing.T) {
	h := newHarness(t)
	q := newQueue(t, h, 4, 0, map[string]string{"owner": "v"})
	must(q.Add(ph.Question{ID: "q", Kind: "ambiguity", Subject: "owner", SubjectVersion: "v", DocumentDigest: h.doc.Receipt.DocumentDigest, Evidence: []string{"fact-owner"}, Prompt: "tone?"}))
	must(q.Answer(h.alice(), ph.Answer{QuestionID: "q", Outcome: ph.OutcomeAnnotate, Text: "plain", SeenVersion: "v"}))
	next, _, err := ph.ApplyAnswers(context.Background(), h.doc, q, []string{"q"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Spec.Nodes[2].FactIDs[0] = "mutated"
	next.Profile.Goals[0].Statement = "mutated"
	if prose.VerifyDocument(h.doc) != nil {
		t.Fatal("next target aliased the prior immutable version")
	}
}
