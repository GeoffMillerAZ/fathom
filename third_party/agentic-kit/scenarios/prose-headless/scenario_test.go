package proseheadless_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

var epoch = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

// store is a synthetic authorized source provider with one revision per source.
type store struct {
	mu      sync.Mutex
	items   map[prose.SourceRef]prose.SourceSnapshot
	current map[string]prose.SourceRef
}

func (s *store) Resolve(ctx context.Context, principal string, ref prose.SourceRef) (prose.SourceSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[ref]
	if !ok || principal != "analyst" {
		return prose.SourceSnapshot{}, prose.ErrEvidence
	}
	return v, ctx.Err()
}

func (s *store) Current(ctx context.Context, principal, id string) (prose.SourceRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.current[id]
	if !ok || principal != "analyst" {
		return prose.SourceRef{}, prose.ErrEvidence
	}
	return v, ctx.Err()
}

// publish stores content as the current revision of source id and returns its fact.
func (s *store) publish(id, version, content string) prose.Fact {
	ref := prose.SourceRef{ID: id, Version: version, Digest: prose.ContentDigest(content)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[ref] = prose.SourceSnapshot{Ref: ref, Content: content, MediaType: "text/plain", AcquiredBy: "fixture", AcquiredAt: epoch.Add(-time.Hour), Access: "public", RetainUntil: epoch.Add(time.Hour)}
	s.current[id] = ref
	span := evidencespan.Span{
		ID: "fact-" + id, EntityID: id, Field: "value", Value: content, Confidence: 1, SourceID: id, RecordID: version, RecordHash: ref.Digest,
		OccurredAt: epoch.Add(-2 * time.Hour), ObservedAt: epoch.Add(-time.Hour), Locator: evidencespan.Locator{Kind: "byte-range", Value: id, Start: 0, End: len(content) - 1},
		Author: aiprovenance.NewHuman("fixture"), Grade: &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 1},
	}
	return prose.Fact{Span: span, Source: ref, Unit: "text"}
}

// quoter answers each node by quoting its first exposed fact (or its instruction for a heading).
type quoter struct {
	mu    sync.Mutex
	calls []string
}

func (q *quoter) Complete(_ context.Context, req llmclient.Request) (llmclient.Response, error) {
	var p struct {
		Node  struct{ ID, Instruction string } `json:"node"`
		Facts []struct{ ID, Value string }     `json:"facts"`
	}
	if err := json.Unmarshal([]byte(req.Prompt), &p); err != nil {
		return llmclient.Response{}, err
	}
	q.mu.Lock()
	q.calls = append(q.calls, p.Node.ID)
	q.mu.Unlock()
	seg := `{"kind":"label","text":` + strconv.Quote(p.Node.Instruction) + `,"fact_ids":[],"contradictions":[],"reason":"label"}`
	if len(p.Facts) > 0 {
		seg = `{"kind":"assertion","text":` + strconv.Quote(p.Facts[0].Value) + `,"fact_ids":[` + strconv.Quote(p.Facts[0].ID) + `],"contradictions":[],"reason":""}`
	}
	out := `{"segments":[` + seg + `]}`
	return llmclient.Response{Text: out, StopReason: llmclient.StopEndTurn}, nil
}

func (q *quoter) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, context.Canceled
}

// TestFreeformBriefThroughExportedAPI exercises the frozen API from outside the
// package: a freeform brief with a task-local goal, a source correction and exact
// reuse. It is an API-shape proof with fakes, not a live or gate result.
func TestFreeformBriefThroughExportedAPI(t *testing.T) {
	src := &store{items: map[prose.SourceRef]prose.SourceSnapshot{}, current: map[string]prose.SourceRef{}}
	facts := []prose.Fact{src.publish("owner", "v1", "Alex owns the rollout"), src.publish("date", "v1", "Launch is 3 Nov")}
	profile, err := prose.NewDocumentTypeProfile(prose.DocumentTypeProfile{
		Key: "freeform-brief", Version: "1", Freeform: true, Origin: prose.ProfileOrigin{Kind: "supplied"},
		Sections: []prose.SectionSpec{{ID: "body", Title: "Body", Required: true}},
		Goals:    []prose.Goal{{ID: "g-support", Class: prose.GoalRequired, Statement: "Only supported facts", Scope: prose.ScopeDocument, Method: "exact-support", Acceptance: "all claims supported", Origin: prose.OriginProfile}},
		Defaults: prose.PresentationDefaults{Audience: "team", Voice: "plain", TechnicalLevel: "low"},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := prose.NewDocumentSession(profile, prose.SessionInput{
		ID: "brief-1", Purpose: "Tell the team who owns launch",
		Goals:           []prose.Goal{{ID: "g-owner", Class: prose.GoalRequired, Statement: "Name the owner", Scope: "body", Method: "exact-support", Acceptance: "owner named", Origin: prose.OriginTask}},
		ExpectedGoalIDs: []string{"g-owner"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	spec := prose.DocumentSpec{ID: "brief", Nodes: []prose.NodeSpec{
		{ID: "doc", Kind: prose.KindDocument},
		{ID: "body", Kind: prose.KindSection, ParentID: "doc", Order: 1},
		{ID: "owner", Kind: prose.KindParagraph, ParentID: "body", Order: 1, Instruction: "Name the owner", FactIDs: []string{"fact-owner"}},
		{ID: "date", Kind: prose.KindParagraph, ParentID: "body", Order: 2, Instruction: "State the date", FactIDs: []string{"fact-date"}},
	}}
	target := prose.DocumentTarget{Session: session, Profile: profile, Spec: spec, Input: prose.DocumentInput{Principal: "analyst", Facts: facts}}
	client := &quoter{}
	engine, err := prose.NewDocumentEngine(client, src, prose.ExactSupportReviewer{}, prose.DocumentConfig{
		Sourced: prose.SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "configured-model", Limits: prose.DefaultProseLimits()},
		Limits:  prose.DefaultDocumentLimits(),
	}, func() time.Time { return epoch })
	if err != nil {
		t.Fatal(err)
	}
	v1, err := engine.Generate(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := engine.Render(context.Background(), v1)
	if err != nil || rendered.Markdown != "Alex owns the rollout\n\nLaunch is 3 Nov" {
		t.Fatalf("render %q %v", rendered.Markdown, err)
	}
	client.calls = nil
	next := target
	next.Input.Facts = []prose.Fact{facts[0], src.publish("date", "v2", "Launch is 10 Nov")}
	res, err := engine.Rebuild(context.Background(), v1, next, nil, prose.RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(client.calls, ",") != "date" || res.Version.Nodes[2].Text != "Alex owns the rollout" || !res.Version.Nodes[2].Reused {
		t.Fatalf("only the corrected node regenerates; calls=%v", client.calls)
	}
	if _, err := engine.Rebuild(context.Background(), v1, next, nil, prose.RebuildOptions{Requested: []string{"owner"}}); !errors.Is(err, prose.ErrImpact) {
		t.Fatalf("a request that omits the floor must be refused: %v", err)
	}
	if m, err := engine.Render(context.Background(), res.Version); err != nil || !strings.HasSuffix(m.Markdown, "Launch is 10 Nov") {
		t.Fatalf("rebuilt render: %v", err)
	}
}
