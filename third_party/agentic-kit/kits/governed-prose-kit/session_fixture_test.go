package governedprosekit

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// fakeDocClient answers per-node prompts deterministically. text picks the unit
// text and cited fact IDs from the decoded prompt; default quotes the first fact.
type fakeDocClient struct {
	mu      sync.Mutex
	calls   []string
	prompts map[string]string
	text    func(node string, facts []promptFact) (string, []string)
	raw     func(node string) (string, bool)
	stop    llmclient.StopReason
	only    string
	after   func(n int)
}

func (c *fakeDocClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	var p struct {
		Node struct {
			ID          string `json:"id"`
			Kind        string `json:"kind"`
			Instruction string `json:"instruction"`
		} `json:"node"`
		Facts []promptFact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(req.Prompt), &p); err != nil {
		return llmclient.Response{}, err
	}
	c.mu.Lock()
	c.calls = append(c.calls, p.Node.ID)
	if c.prompts == nil {
		c.prompts = map[string]string{}
	}
	c.prompts[p.Node.ID] = req.Prompt
	n := len(c.calls)
	c.mu.Unlock()
	if c.after != nil {
		c.after(n)
	}
	stop := c.stop
	if stop == "" {
		stop = llmclient.StopEndTurn
	}
	if c.raw != nil {
		if out, ok := c.raw(p.Node.ID); ok {
			return llmclient.Response{Text: out, StopReason: stop}, nil
		}
	}
	text, cited := "", []string(nil)
	if c.text != nil && (c.only == "" || c.only == p.Node.ID) {
		text, cited = c.text(p.Node.ID, p.Facts)
	} else if len(p.Facts) > 0 {
		text, cited = p.Facts[0].Value, []string{p.Facts[0].ID}
	} else {
		text = p.Node.Instruction
	}
	seg := documentSegment{Kind: "assertion", Text: text, FactIDs: cited, Contradictions: []string{}}
	if len(cited) == 0 {
		seg = documentSegment{Kind: "label", Text: text, FactIDs: []string{}, Contradictions: []string{}, Reason: "structural label"}
	}
	out, _ := json.Marshal(documentResponse{Segments: []documentSegment{seg}})
	r := llmclient.Response{Text: string(out), StopReason: stop}
	r.InputTokens, r.OutputTokens = 100, 20
	return r, nil
}

func (c *fakeDocClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, context.Canceled
}

func (c *fakeDocClient) called() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func (c *fakeDocClient) reset() {
	c.mu.Lock()
	c.calls = nil
	c.mu.Unlock()
}

type docEnv struct {
	t       *testing.T
	sources *testSources
	client  *fakeDocClient
	engine  *DocumentEngine
	profile DocumentTypeProfile
	session DocumentSession
	target  DocumentTarget
}

func testProfile(t *testing.T) DocumentTypeProfile {
	t.Helper()
	p, err := NewDocumentTypeProfile(DocumentTypeProfile{
		Key: "test-brief", Version: "1", Origin: ProfileOrigin{Kind: "supplied"},
		Sections: []SectionSpec{{ID: "findings", Title: "Findings", Required: true, FactFamilies: []string{"incident"}}, {ID: "summary", Title: "Summary", Required: true}},
		Goals: []Goal{
			{ID: "g-support", Class: GoalRequired, Statement: "State only supported facts", Scope: ScopeDocument, Method: "exact-support", Acceptance: "every claim supported", Origin: OriginProfile},
			{ID: "g-voice", Class: GoalAdvisory, Statement: "Plain voice", Scope: ScopeDocument, Method: "style-review", Acceptance: "reader finds it plain", Origin: OriginProfile},
		},
		Defaults: PresentationDefaults{Audience: "executive", Voice: "plain", TechnicalLevel: "low"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func testSpec() DocumentSpec {
	return DocumentSpec{ID: "doc-1", Nodes: []NodeSpec{
		{ID: "doc", Kind: KindDocument},
		{ID: "findings", Kind: KindSection, ParentID: "doc", Order: 1},
		{ID: "h1", Kind: KindHeading, ParentID: "findings", Order: 1, Instruction: "Findings"},
		{ID: "p1", Kind: KindParagraph, ParentID: "findings", Order: 2, Instruction: "Report the count", FactIDs: []string{"fact-a"}},
		{ID: "p2", Kind: KindParagraph, ParentID: "findings", Order: 3, Instruction: "Report the duration", FactIDs: []string{"fact-b"}},
		{ID: "summary", Kind: KindSection, ParentID: "doc", Order: 2},
		{ID: "hs", Kind: KindHeading, ParentID: "summary", Order: 1, Instruction: "Summary"},
		{ID: "sum", Kind: KindSummary, ParentID: "summary", Order: 2, Instruction: "Summarize outcome", FactIDs: []string{"fact-c"}, Summarizes: []string{"p1", "p2"}},
	}, ExpectedEdges: []Edge{{From: "sum", To: "p1", Kind: EdgeSummary}, {From: "sum", To: "p2", Kind: EdgeSummary}}}
}

func newDocEnv(t *testing.T) *docEnv {
	t.Helper()
	s, req, _ := sourcedFixture(t, "12 incidents", "30 minutes", "Resolved")
	client := &fakeDocClient{}
	reviewer := ExactSupportReviewer{NonFactual: []string{"Findings", "Summary", "structural label"}}
	engine, err := NewDocumentEngine(client, s, reviewer, DocumentConfig{Sourced: SourcedConfig{PolicyVersion: "policy/v1", PromptVersion: "prompt/v1", Model: "fixture-generator", Limits: DefaultProseLimits()}, Limits: DefaultDocumentLimits()}, func() time.Time { return sourcedEpoch })
	if err != nil {
		t.Fatal(err)
	}
	profile := testProfile(t)
	session, err := NewDocumentSession(profile, SessionInput{ID: "sess-1", Purpose: "Brief the executive team", Intent: "initial draft"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := &docEnv{t: t, sources: s, client: client, engine: engine, profile: profile, session: session}
	e.target = DocumentTarget{Session: session, Profile: profile, Spec: testSpec(), Input: DocumentInput{Principal: "reader", Facts: req.Facts}}
	return e
}

func (e *docEnv) generate() DocumentVersion {
	e.t.Helper()
	v, err := e.engine.Generate(context.Background(), e.target)
	if err != nil {
		e.t.Fatal(err)
	}
	return v
}

// revise returns a target whose fact id has a new source revision and value.
func (e *docEnv) revise(id, value string) DocumentTarget {
	e.t.Helper()
	next := e.target
	next.Input.Facts = append([]Fact(nil), e.target.Input.Facts...)
	for i, f := range next.Input.Facts {
		if f.Span.ID != id {
			continue
		}
		ref := SourceRef{ID: f.Source.ID, Version: "v2-" + ContentDigest(value)[:6], Digest: ContentDigest(value)}
		e.sources.mu.Lock()
		e.sources.items[ref] = SourceSnapshot{Ref: ref, Content: value, MediaType: "text/plain", AcquiredBy: "fixture", AcquiredAt: sourcedEpoch.Add(-time.Minute), Access: "public", RetainUntil: sourcedEpoch.Add(time.Hour)}
		e.sources.current[ref.ID] = ref
		e.sources.mu.Unlock()
		f.Source, f.Span.Value, f.Span.RecordID, f.Span.RecordHash = ref, value, ref.Version, ref.Digest
		f.Span.Locator.Value, f.Span.Locator.End = ref.ID, len(value)-1
		next.Input.Facts[i] = f
	}
	return next
}

func ids(in []NodeImpact) string {
	var out []string
	for _, n := range in {
		out = append(out, n.NodeID)
	}
	return strings.Join(out, ",")
}

func nodeByID(v DocumentVersion, id string) DocNode {
	for _, n := range v.Nodes {
		if n.ID == id {
			return n
		}
	}
	return DocNode{}
}
