package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// govDoc has one heading section with two citable rule lines.
// Line numbers (1-based):
//
//	1: # Conventions
//	2: (blank)
//	3: - Never call time.Now() in query logic; inject clocks.
//	4: - All outputs must be sorted.
const govDoc = `# Conventions

- Never call time.Now() in query logic; inject clocks.
- All outputs must be sorted.
`

var indexedFiles = []string{"internal/query/engine.go", "internal/mcp/tools.go", "docs/x.md"}

func fixedNow() time.Time { return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC) }

func newCompiler(t *testing.T, st store.Store, entries ...llmclient.MockEntry) *Compiler {
	t.Helper()
	return &Compiler{
		St:    st,
		LLM:   llmclient.NewMockClient(entries...),
		Model: "mock-model",
		Now:   fixedNow,
	}
}

func compileInput(dryRun bool) CompileInput {
	return CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{{Path: "docs/conventions.md", Content: []byte(govDoc)}},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
		DryRun:       dryRun,
	}
}

func TestCompile_HappyPath(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":3,"cite_end":3},
  {"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}
]}`,
		Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 50},
	}})

	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 2 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted=%d rejected=%d, want 2/0", run.ClaimsEmitted, run.ClaimsRejected)
	}
	if run.SectionsCompiled != 1 || run.SectionsTotal != 1 {
		t.Errorf("sections compiled/total = %d/%d, want 1/1", run.SectionsCompiled, run.SectionsTotal)
	}
	if run.TokensIn != 100 || run.TokensOut != 50 {
		t.Errorf("usage %d/%d, want the mock's real usage 100/50", run.TokensIn, run.TokensOut)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("stored %d claims, want 2", len(claims))
	}
	for _, cl := range claims {
		if cl.Status != core.ClaimFresh || cl.Tier != core.TierDocClaimed {
			t.Errorf("claim %s status/tier = %s/%s", cl.ClaimID, cl.Status, cl.Tier)
		}
		if cl.PromptVersion != obligationsPromptVersion() || cl.Model != "mock-model" || cl.InputDigest == "" || cl.ReproKey == "" {
			t.Errorf("claim %s provenance incomplete: %+v", cl.ClaimID, cl)
		}
		if cl.CompiledAtWatermark != "manifest:aaa" {
			t.Errorf("claim %s watermark = %q", cl.ClaimID, cl.CompiledAtWatermark)
		}
		if cl.Subject != "docs/conventions.md#conventions" {
			t.Errorf("claim subject = %q", cl.Subject)
		}
	}

	// Citation link carries a hash that re-verifies against the doc.
	ids := []string{claims[0].ClaimID, claims[1].ClaimID}
	links, err := st.ClaimLinks(ctx, "svc", ids)
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	var cites, scopes int
	for _, l := range links {
		switch l.Rel {
		case core.RelCites:
			cites++
			path, s, e, ok := core.ParseCiteRef(l.TargetRef)
			if !ok || path != "docs/conventions.md" {
				t.Fatalf("bad cite ref %q", l.TargetRef)
			}
			h, ok := core.HashLines([]byte(govDoc), s, e)
			if !ok || h != l.TargetHash {
				t.Errorf("cite hash mismatch for %q", l.TargetRef)
			}
		case core.RelScopes:
			scopes++
		}
	}
	if cites != 2 || scopes != 2 {
		t.Errorf("links cites/scopes = %d/%d, want 2/2", cites, scopes)
	}

	runs, err := st.CompileRuns(ctx, "svc", LayerObligations)
	if err != nil || len(runs) != 1 {
		t.Fatalf("CompileRuns = %d, %v; want 1 recorded run", len(runs), err)
	}
}

func TestCompile_RejectsBadAnchorsAndScopes(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	longRule := strings.Repeat("word ", defaultMaxRuleWords+1)
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Cite outside the section.","scope":["**"],"cite_start":99,"cite_end":120},
  {"rule":"Scope matches nothing.","scope":["nonexistent/dir/**"],"cite_start":3,"cite_end":3},
  {"rule":"` + strings.TrimSpace(longRule) + `","scope":["**"],"cite_start":3,"cite_end":3},
  {"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}
]}`,
	}})

	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 3 {
		t.Fatalf("emitted=%d rejected=%d, want 1 emitted / 3 rejected (bad cite, dead scope, over-budget rule)", run.ClaimsEmitted, run.ClaimsRejected)
	}
}

// TestCompile_RejectsOverBroadScope (O8/OP-2): a scope glob matching more
// than half of IndexedFiles is a catch-all in disguise — typically "**" —
// and must be rejected exactly like a failed citation, counted in
// ClaimsRejected, while a narrow glob in the same batch still passes.
func TestCompile_RejectsOverBroadScope(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Everything must be globally consistent.","scope":["**"],"cite_start":3,"cite_end":3},
  {"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}
]}`,
	}})

	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 1 {
		t.Fatalf("emitted=%d rejected=%d, want 1/1 (\"**\" over-broad rejected, narrow glob passes)", run.ClaimsEmitted, run.ClaimsRejected)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 1 || claims[0].Rule != "Sort all outputs." {
		t.Fatalf("stored claims = %+v, want only the narrow-scope claim", claims)
	}
}

// TestCompile_OverBroadGuardExemptsFileClassGlobs (O8/OP-2 refinement): the
// >50% breadth check and prompt v3 must not contradict each other. Prompt v3
// mandates single-file-class extension globs (e.g. "**/*.go") as the ONLY
// legitimate shape for repo-wide rules — in a go-majority corpus such a glob
// necessarily matches >50% of indexed files, so the guard exempts globs whose
// final segment is "*.<ext>" (class-scoped by construction). Bare "**" and
// multi-class directory globs matching >50% stay rejected; zero-match
// rejection still applies to everything.
func TestCompile_OverBroadGuardExemptsFileClassGlobs(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	// Go-majority corpus (3 .go + 1 .md) — the fathom-like shape where the
	// original guard rejected every prompt-compliant fleet-wide code rule.
	goMajority := []string{"internal/a/a.go", "internal/b/b.go", "internal/c/c.go", "docs/x.md"}
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Inject clocks in all Go code.","scope":["**/*.go"],"cite_start":3,"cite_end":3},
  {"rule":"Everything must be globally consistent.","scope":["**"],"cite_start":3,"cite_end":3},
  {"rule":"Follow internal conventions.","scope":["internal/**"],"cite_start":3,"cite_end":3},
  {"rule":"Sort all outputs.","scope":["**/*.md"],"cite_start":4,"cite_end":4}
]}`,
	}})

	in := compileInput(false)
	in.IndexedFiles = goMajority
	run, err := c.Compile(ctx, in)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// "**/*.go" matches 3/4 (>50%) but is a single-class extension glob →
	// ACCEPTED. "**" matches 4/4 → REJECTED. "internal/**" matches 3/4 and
	// is a multi-class directory glob → REJECTED. "**/*.md" matches 1/4 →
	// accepted normally.
	if run.ClaimsEmitted != 2 || run.ClaimsRejected != 2 {
		t.Fatalf("emitted=%d rejected=%d, want 2/2 (class globs exempt; ** and broad dir globs rejected)", run.ClaimsEmitted, run.ClaimsRejected)
	}
	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	gotRules := map[string]bool{}
	for _, cl := range claims {
		gotRules[cl.Rule] = true
	}
	if !gotRules["Inject clocks in all Go code."] {
		t.Errorf("the prompt-compliant fleet-wide code rule (scope **/*.go) must be accepted; stored: %+v", gotRules)
	}
	if !gotRules["Sort all outputs."] {
		t.Errorf("the narrow markdown-class rule must be accepted; stored: %+v", gotRules)
	}
	if gotRules["Everything must be globally consistent."] || gotRules["Follow internal conventions."] {
		t.Errorf("catch-all and broad directory scopes must be rejected; stored: %+v", gotRules)
	}
}

func TestCompile_DigestGateSkipsUnchangedSections(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c.Compile(ctx, compileInput(false)); err != nil {
		t.Fatalf("first Compile: %v", err)
	}

	// Second compile: same doc, a mock with NO entries — it would answer
	// "mock: no more entries" (parse failure) if the gate leaked an LLM call.
	c2 := newCompiler(t, st)
	run, err := c2.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	if run.SectionsCompiled != 0 {
		t.Errorf("SectionsCompiled = %d, want 0 (digest-gated)", run.SectionsCompiled)
	}
	if run.ClaimsEmitted != 0 {
		t.Errorf("ClaimsEmitted = %d, want 0", run.ClaimsEmitted)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	if len(claims) != 1 || claims[0].Status != core.ClaimFresh {
		t.Errorf("prior claim disturbed by gated recompile: %+v", claims)
	}
}

func TestCompile_SupersedesReplacedClaims(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c.Compile(ctx, compileInput(false)); err != nil {
		t.Fatalf("first Compile: %v", err)
	}

	// The doc changes (rule rewritten) — the section digest changes, and the
	// recompile emits a different claim.
	changed := strings.Replace(govDoc, "All outputs must be sorted.", "All outputs must be sorted deterministically.", 1)
	in := compileInput(false)
	in.Docs = []core.SourceFile{{Path: "docs/conventions.md", Content: []byte(changed)}}
	c2 := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs deterministically.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c2.Compile(ctx, in); err != nil {
		t.Fatalf("second Compile: %v", err)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	var fresh, superseded int
	for _, cl := range claims {
		switch cl.Status {
		case core.ClaimFresh:
			fresh++
			if cl.Rule != "Sort all outputs deterministically." {
				t.Errorf("fresh claim rule = %q", cl.Rule)
			}
		case core.ClaimSuperseded:
			superseded++
		}
	}
	if fresh != 1 || superseded != 1 {
		t.Errorf("fresh=%d superseded=%d, want 1/1 (audit history kept, never deleted)", fresh, superseded)
	}
}

func TestCompile_DryRunSpendsNothing(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := &Compiler{St: st, Now: fixedNow} // no LLM at all: dry run must not need one

	run, err := c.Compile(ctx, compileInput(true))
	if err != nil {
		t.Fatalf("dry-run Compile: %v", err)
	}
	if !run.DryRun || run.SectionsCompiled != 1 || run.TokensIn == 0 {
		t.Errorf("dry run = %+v; want DryRun, 1 section, non-zero token estimate", run)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", "")
	if len(claims) != 0 {
		t.Errorf("dry run wrote %d claims", len(claims))
	}
	runs, _ := st.CompileRuns(ctx, "svc", "")
	if len(runs) != 0 {
		t.Errorf("dry run recorded %d compile runs, want 0", len(runs))
	}
}

// TestCompile_BadResponseSkipsSectionOnly: a malformed (or truncated) model
// response poisons only its own section — the corpus compile continues and
// the failure is counted. Genuine transport errors still abort.
func TestCompile_BadResponseSkipsSectionOnly(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// Two docs → two sections. First response is garbage, second is valid.
	twoDocs := CompileInput{
		ServiceID: "svc",
		Docs: []core.SourceFile{
			{Path: "docs/a.md", Content: []byte("# A\n\n- Rule in A.\n")},
			{Path: "docs/b.md", Content: []byte("# B\n\n- All outputs must be sorted.\n")},
		},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
	}
	c := newCompiler(
		t, st,
		llmclient.MockEntry{Response: llmclient.Response{Text: `[{"rule":"truncated mid`}},
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`}},
	)
	run, err := c.Compile(ctx, twoDocs)
	if err != nil {
		t.Fatalf("Compile aborted on a per-section bad response: %v", err)
	}
	if run.SectionsFailed != 1 || run.SectionsCompiled != 1 || run.ClaimsEmitted != 1 {
		t.Errorf("failed/compiled/emitted = %d/%d/%d, want 1/1/1", run.SectionsFailed, run.SectionsCompiled, run.ClaimsEmitted)
	}

	// A truncation (max_tokens stop) is the same skippable class.
	st2 := store.NewMemoryStore()
	c2 := newCompiler(
		t, st2,
		llmclient.MockEntry{Response: llmclient.Response{Text: `[{"rule":"cut`, StopReason: llmclient.StopMaxTokens}},
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`}},
	)
	run2, err := c2.Compile(ctx, twoDocs)
	if err != nil {
		t.Fatalf("Compile aborted on truncation: %v", err)
	}
	if run2.SectionsFailed != 1 || run2.ClaimsEmitted != 1 {
		t.Errorf("truncation: failed/emitted = %d/%d, want 1/1", run2.SectionsFailed, run2.ClaimsEmitted)
	}

	// A transport-level error is NOT skippable — it aborts.
	st3 := store.NewMemoryStore()
	c3 := newCompiler(t, st3, llmclient.MockEntry{Err: context.DeadlineExceeded})
	if _, err := c3.Compile(ctx, twoDocs); err == nil {
		t.Fatal("transport error must abort the compile, not be skipped")
	}
}

// TestCompile_RetiresClaimsWhenDocLeavesCorpus: a compile owns its layer — a
// doc removed from the corpus selector retires its claims (the sweep cannot:
// the doc still exists on disk and its citations still hash-match).
func TestCompile_RetiresClaimsWhenDocLeavesCorpus(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c.Compile(ctx, compileInput(false)); err != nil {
		t.Fatalf("first Compile: %v", err)
	}

	// Second compile with an EMPTY corpus (the doc left the selector).
	c2 := newCompiler(t, st)
	in := compileInput(false)
	in.Docs = nil
	if _, err := c2.Compile(ctx, in); err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims = %d, %v", len(claims), err)
	}
	if claims[0].Status != core.ClaimSuperseded {
		t.Errorf("claim from departed doc = %s, want superseded (audit kept, serving stopped)", claims[0].Status)
	}
}

// TestCompile_PromptBumpDefeatsDigestGate: claims compiled under an older
// prompt version must be recompiled even when the section content is
// unchanged (observed live: a v1→v2 bump left 27/32 sections on v1 claims).
func TestCompile_PromptBumpDefeatsDigestGate(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c.Compile(ctx, compileInput(false)); err != nil {
		t.Fatalf("first Compile: %v", err)
	}
	// Simulate the prior claims having been compiled under an older prompt.
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	for i := range claims {
		claims[i].PromptVersion = "obligations/v0-old"
	}
	if err := st.PutClaims(ctx, claims, nil); err != nil {
		t.Fatalf("rewrite prompt version: %v", err)
	}

	c2 := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	run, err := c2.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	if run.SectionsCompiled != 1 {
		t.Errorf("SectionsCompiled = %d, want 1 (prompt bump must defeat the digest gate)", run.SectionsCompiled)
	}
}

// ─── M1-6: structured.Complete adoption ─────────────────────────────────────

// schemaAssertingClient fails the test if a request arrives without
// OutputSchema set — the direct proof that compileSection routes through
// agentic-kit's structured.Complete (which sets it) rather than a bare
// c.LLM.Complete call.
type schemaAssertingClient struct {
	t    *testing.T
	resp llmclient.Response
}

func (s schemaAssertingClient) Complete(_ context.Context, req llmclient.Request) (llmclient.Response, error) {
	s.t.Helper()
	if len(req.OutputSchema) == 0 {
		s.t.Fatal("compileSection: request has no OutputSchema — expected structured.Complete to set it")
	}
	return s.resp, nil
}

func (s schemaAssertingClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk)
	close(ch)
	return ch, nil
}

func TestCompile_UsesStructuredOutputSchema(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := &Compiler{
		St: st,
		LLM: schemaAssertingClient{t: t, resp: llmclient.Response{
			Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
		}},
		Model: "mock-model",
		Now:   fixedNow,
	}
	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 {
		t.Errorf("ClaimsEmitted = %d, want 1", run.ClaimsEmitted)
	}
}

// ─── M1-7: enforceBudgets ────────────────────────────────────────────────────

// TestCompile_RefusesToStartWhenEstimatedCostExceedsBudget: a real compile's
// pre-flight cost estimate — computed before any LLM call — above the
// layer's max_usd_per_run refuses the run outright. The refusal is still
// ledgered (ADR-016 §4.1: refusals are recorded, not silent).
func TestCompile_RefusesToStartWhenEstimatedCostExceedsBudget(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	mock := llmclient.NewMockClient() // no entries: a real call would prove the gate leaked
	c := &Compiler{
		St:    st,
		LLM:   mock,
		Model: "mock-model",
		Layer: LayerDef{
			Name:    "obligations",
			Budgets: LayerBudgets{MaxTokensPerRun: 200000, MaxUSDPerRun: 0.0001},
		},
		InRatePerKTok: 100.0, // deliberately extreme so any non-trivial estimate trips the gate
		Now:           fixedNow,
	}

	run, err := c.Compile(ctx, compileInput(false))
	if err == nil {
		t.Fatal("Compile should refuse when the pre-flight cost estimate exceeds max_usd_per_run")
	}
	if !run.Refused {
		t.Errorf("run.Refused = false, want true")
	}
	if run.RefusalReason == "" {
		t.Errorf("run.RefusalReason is empty, want a reason")
	}
	if run.BudgetStopped {
		t.Errorf("run.BudgetStopped = true, want false (this is a pre-flight refusal, not a mid-run stop)")
	}
	if run.ClaimsEmitted != 0 {
		t.Errorf("ClaimsEmitted = %d, want 0 (refused before any LLM call)", run.ClaimsEmitted)
	}
	if mock.CallCount() != 0 {
		t.Errorf("CallCount = %d, want 0 — a refused run must never call the LLM", mock.CallCount())
	}

	// The refusal is still ledgered.
	runs, err := st.CompileRuns(ctx, "svc", "")
	if err != nil || len(runs) != 1 {
		t.Fatalf("CompileRuns = %d, %v; want 1 recorded refusal", len(runs), err)
	}
	if !runs[0].Refused || runs[0].RefusalReason == "" {
		t.Errorf("recorded run = %+v, want Refused with a reason", runs[0])
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	if len(claims) != 0 {
		t.Errorf("refused run wrote %d claims, want 0", len(claims))
	}
}

// TestCompile_StopsMidRunWhenTokenBudgetExceeded_AndResumesViaDigestGate:
// the running token total crossing max_tokens_per_run after the first of two
// sections stops the run cleanly — it keeps what it already compiled and
// never reaches the second section. A follow-up compile against the same
// corpus (a fresh, unbudgeted Compiler) then resumes via the ordinary digest
// gate: it skips the already-compiled section and finishes the corpus.
func TestCompile_StopsMidRunWhenTokenBudgetExceeded_AndResumesViaDigestGate(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	twoDocs := CompileInput{
		ServiceID: "svc",
		Docs: []core.SourceFile{
			{Path: "docs/a.md", Content: []byte("# A\n\n- Rule in A.\n")},
			{Path: "docs/b.md", Content: []byte("# B\n\n- All outputs must be sorted.\n")},
		},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
	}

	mock := llmclient.NewMockClient(
		// docs/a.md's only section: a large real usage — big enough on its
		// own to cross the token budget, so the run stops BEFORE docs/b.md.
		llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"claims":[{"rule":"Rule in A.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`,
			Usage: llmclient.Usage{InputTokens: 500, OutputTokens: 200},
		}},
	)
	c := &Compiler{
		St:    st,
		LLM:   mock,
		Model: "mock-model",
		Layer: LayerDef{
			Name:    "obligations",
			Budgets: LayerBudgets{MaxTokensPerRun: 600, MaxUSDPerRun: 1000},
		},
		InRatePerKTok: 0.001,
		Now:           fixedNow,
	}
	run, err := c.Compile(ctx, twoDocs)
	if err != nil {
		t.Fatalf("a clean mid-run budget stop must not return an error: %v", err)
	}
	if !run.BudgetStopped {
		t.Errorf("run.BudgetStopped = false, want true")
	}
	if run.Refused {
		t.Errorf("run.Refused = true, want false (this is a mid-run stop, not a pre-flight refusal)")
	}
	if run.RefusalReason == "" {
		t.Errorf("run.RefusalReason is empty, want a reason")
	}
	if run.SectionsCompiled != 1 || run.ClaimsEmitted != 1 {
		t.Fatalf("compiled/emitted = %d/%d, want 1/1 (only docs/a.md; docs/b.md was never reached)", run.SectionsCompiled, run.ClaimsEmitted)
	}
	if mock.CallCount() != 1 {
		t.Errorf("CallCount = %d, want 1 — the run must stop before calling the LLM for docs/b.md", mock.CallCount())
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims = %d, %v; want 1 (docs/a.md only)", len(claims), err)
	}
	if claims[0].Status != core.ClaimFresh {
		t.Errorf("claims[0].Status = %s, want fresh (a clean stop must not touch what it did compile)", claims[0].Status)
	}

	// Resume: a fresh, unbudgeted compile against the SAME corpus. The
	// digest gate skips docs/a.md's unchanged section (its claim already
	// matches content + prompt identity) and compiles only docs/b.md.
	mock2 := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c2 := &Compiler{St: st, LLM: mock2, Model: "mock-model", Now: fixedNow}
	run2, err := c2.Compile(ctx, twoDocs)
	if err != nil {
		t.Fatalf("resume Compile: %v", err)
	}
	if run2.SectionsCompiled != 1 || run2.ClaimsEmitted != 1 {
		t.Fatalf("resume compiled/emitted = %d/%d, want 1/1 (docs/b.md only — docs/a.md is digest-gated)", run2.SectionsCompiled, run2.ClaimsEmitted)
	}
	if mock2.CallCount() != 1 {
		t.Errorf("resume CallCount = %d, want 1", mock2.CallCount())
	}

	all, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(all) != 2 {
		t.Fatalf("total claims after resume = %d, %v; want 2 (a.md + b.md)", len(all), err)
	}
	for _, cl := range all {
		if cl.Status != core.ClaimFresh {
			t.Errorf("claim %s status = %s, want fresh (nothing should be superseded across a stop+resume)", cl.ClaimID, cl.Status)
		}
	}
}

// ─── F1: schema must satisfy Anthropic structured-outputs constraints ───────

// TestClaimDraftSchema_AnthropicStructuredOutputCompatible pins the two wire
// constraints of Anthropic structured outputs (a real compile 400s otherwise,
// and a 400 is a transport-class abort — not skippable): the schema root must
// be an OBJECT (array roots are not a documented shape), and every object
// level must set additionalProperties:false.
func TestClaimDraftSchema_AnthropicStructuredOutputCompatible(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(claimDraftSchema, &schema); err != nil {
		t.Fatalf("claimDraftSchema is not valid JSON: %v", err)
	}
	if typ, _ := schema["type"].(string); typ != "object" {
		t.Fatalf("schema root type = %q, want object (array roots 400 on Anthropic structured outputs)", typ)
	}

	// Walk every object-typed schema node; each must set additionalProperties:false.
	var walk func(path string, node map[string]any)
	walk = func(path string, node map[string]any) {
		if typ, _ := node["type"].(string); typ == "object" {
			ap, present := node["additionalProperties"]
			if !present {
				t.Errorf("%s: object schema missing additionalProperties:false", path)
			} else if b, ok := ap.(bool); !ok || b {
				t.Errorf("%s: additionalProperties = %v, want false", path, ap)
			}
			if props, ok := node["properties"].(map[string]any); ok {
				for name, sub := range props {
					if subMap, ok := sub.(map[string]any); ok {
						walk(path+".properties."+name, subMap)
					}
				}
			}
		}
		if items, ok := node["items"].(map[string]any); ok {
			walk(path+".items", items)
		}
	}
	walk("$", schema)

	// The envelope must expose the claims array the decode target expects.
	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["claims"]; !ok {
		t.Errorf("schema root must define a claims property (the {\"claims\":[...]} envelope)")
	}
}

// ─── F2: superseded claims must not defeat the digest gate ──────────────────

// TestCompile_SupersededClaimsDoNotDefeatDigestGate: a superseded row keeps
// its OLD InputDigest/PromptVersion forever — if the digest gate consults it,
// its subject re-LLMs on every compile for the rest of time. Sequence:
// compile → edit doc → recompile (creates the superseded twin) → third
// compile with ZERO changes must be fully digest-gated (0 sections compiled,
// 0 LLM calls).
func TestCompile_SupersededClaimsDoNotDefeatDigestGate(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// First compile against govDoc v1.
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c.Compile(ctx, compileInput(false)); err != nil {
		t.Fatalf("first Compile: %v", err)
	}

	// Doc edited → second compile supersedes the v1 claim and emits a fresh one.
	changed := strings.Replace(govDoc, "All outputs must be sorted.", "All outputs must be sorted deterministically.", 1)
	in := compileInput(false)
	in.Docs = []core.SourceFile{{Path: "docs/conventions.md", Content: []byte(changed)}}
	c2 := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs deterministically.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	if _, err := c2.Compile(ctx, in); err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	// Sanity: the superseded twin exists beside the fresh claim.
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	var superseded int
	for _, cl := range claims {
		if cl.Status == core.ClaimSuperseded {
			superseded++
		}
	}
	if superseded != 1 {
		t.Fatalf("superseded = %d, want 1 (the setup requires the twin)", superseded)
	}

	// Third compile: SAME doc, ZERO changes — must be entirely digest-gated.
	mock3 := llmclient.NewMockClient() // no entries: a call would poison the run deterministically
	c3 := &Compiler{St: st, LLM: mock3, Model: "mock-model", Now: fixedNow}
	run3, err := c3.Compile(ctx, in)
	if err != nil {
		t.Fatalf("third Compile: %v", err)
	}
	if run3.SectionsCompiled != 0 || run3.SectionsFailed != 0 {
		t.Errorf("third compile compiled/failed = %d/%d, want 0/0 (superseded twin defeated the digest gate)", run3.SectionsCompiled, run3.SectionsFailed)
	}
	if mock3.CallCount() != 0 {
		t.Errorf("third compile made %d LLM calls, want 0", mock3.CallCount())
	}
	// The fresh claim is untouched.
	after, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	var fresh int
	for _, cl := range after {
		if cl.Status == core.ClaimFresh {
			fresh++
		}
	}
	if fresh != 1 {
		t.Errorf("fresh = %d after gated recompile, want 1", fresh)
	}
}

// ─── FT-6 drift anchors: scope-fp persistence is loud, not best-effort ───────

// findingsDoc line numbers (1-based):
//
//	1: # WA-001: query engine lacks retry
//	2: (blank)
//	3: Severity: high. Area: reliability.
//	4: Add retry to the query engine's outbound calls.
const findingsDoc = `# WA-001: query engine lacks retry

Severity: high. Area: reliability.
Add retry to the query engine's outbound calls.
`

func newFindingsCompiler(t *testing.T, st store.Store) *Compiler {
	t.Helper()
	layer, err := LoadLayer(LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayer(findings): %v", err)
	}
	return &Compiler{
		St: st,
		LLM: llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Add retry to the query engine's outbound calls.","scope":["internal/query/**"],"cite_start":3,"cite_end":4}]}`,
		}}),
		Model: "mock-model",
		Layer: layer,
		Now:   fixedNow,
	}
}

func findingsInput() CompileInput {
	return CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{{Path: "findings/wa-001.md", Content: []byte(findingsDoc)}},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
	}
}

// failingFPStore fails every Fingerprint read — a transient store error at
// compile time.
type failingFPStore struct {
	*store.MemoryStore
}

func (f *failingFPStore) Fingerprint(_ context.Context, _, _ string) (string, bool, error) {
	return "", false, fmt.Errorf("boom: transient store failure")
}

// TestCompile_FindingDriftAnchor_StoreErrorAborts: a Fingerprint read error
// while persisting a finding claim's scope-fp drift anchors must ABORT the
// compile loudly, never silently skip the link — the digest gate would
// otherwise never recompile the unchanged section, permanently blinding
// remediation drift for that file.
func TestCompile_FindingDriftAnchor_StoreErrorAborts(t *testing.T) {
	ctx := context.Background()
	st := &failingFPStore{MemoryStore: store.NewMemoryStore()}
	c := newFindingsCompiler(t, st)

	run, err := c.Compile(ctx, findingsInput())
	if err == nil {
		t.Fatalf("Compile succeeded (emitted=%d rejected=%d), want abort on drift-anchor store error", run.ClaimsEmitted, run.ClaimsRejected)
	}
	if !errors.Is(err, errDriftAnchor) {
		t.Errorf("error = %v, want errors.Is errDriftAnchor", err)
	}
	if run.ClaimsRejected != 0 {
		t.Errorf("ClaimsRejected = %d — a store error is an abort, not a draft rejection", run.ClaimsRejected)
	}
}

// TestCompile_FindingDriftAnchor_MissingFingerprintAborts: an indexed file
// with no stored fingerprint violates the pipeline contract (SetFingerprint
// runs for every processed file) — fail loudly rather than emit a finding
// claim with an incomplete drift-anchor set.
func TestCompile_FindingDriftAnchor_MissingFingerprintAborts(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore() // no fingerprints seeded
	c := newFindingsCompiler(t, st)

	_, err := c.Compile(ctx, findingsInput())
	if err == nil {
		t.Fatal("Compile succeeded, want abort on missing fingerprint")
	}
	if !errors.Is(err, errDriftAnchor) {
		t.Errorf("error = %v, want errors.Is errDriftAnchor", err)
	}
}

// TestCompile_FindingDriftAnchor_Written: with fingerprints present, every
// finding claim carries one scope-fp link per scope-matched file (the FT-6
// invariant DriftedFindings relies on).
func TestCompile_FindingDriftAnchor_Written(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	c := newFindingsCompiler(t, st)

	run, err := c.Compile(ctx, findingsInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 {
		t.Fatalf("emitted = %d, want 1", run.ClaimsEmitted)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerFindings)
	if len(claims) != 1 {
		t.Fatalf("claims = %d, want 1", len(claims))
	}
	links, err := st.ClaimLinks(ctx, "svc", []string{claims[0].ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	var scopeFP []core.ClaimLink
	for _, l := range links {
		if l.Rel == core.RelScopeFP {
			scopeFP = append(scopeFP, l)
		}
	}
	// scope internal/query/** matches exactly one indexed file.
	if len(scopeFP) != 1 || scopeFP[0].TargetRef != "internal/query/engine.go" || scopeFP[0].TargetHash != "fp-internal/query/engine.go" {
		t.Errorf("scope-fp links = %+v, want one anchor for internal/query/engine.go", scopeFP)
	}
}
