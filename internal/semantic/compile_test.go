package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
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
			// T5-1/O13: compile must persist the literal cited text
			// alongside the hash, so a later drifted citation can be
			// re-anchored by the sweep, not just flagged stale.
			text, ok := core.LinesText([]byte(govDoc), s, e)
			if !ok || text != l.AnchorText {
				t.Errorf("AnchorText = %q, want the literal cited text %q", l.AnchorText, text)
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

	// Q-2 (docs/plans/expert-suite-q.md, O19's fix): this run trips three
	// DISTINCT causes in one pass — anchor-missing (bad cite), scope-
	// unverifiable (dead scope), malformed (over-budget rule) — the
	// deterministic grouped breakdown must record all three.
	if len(run.RejectionReasons) != 3 {
		t.Fatalf("RejectionReasons = %+v, want 3 distinct causes", run.RejectionReasons)
	}
	wantCauses := map[string]int{"anchor-missing": 1, "malformed": 1, "scope-unverifiable": 1}
	for _, rb := range run.RejectionReasons {
		if wantCauses[rb.Cause] != rb.Count {
			t.Errorf("cause %q count = %d, want %d", rb.Cause, rb.Count, wantCauses[rb.Cause])
		}
		if len(rb.Samples) == 0 {
			t.Errorf("cause %q has no samples", rb.Cause)
		}
	}
	// Alphabetical order: anchor-missing, malformed, scope-unverifiable.
	if run.RejectionReasons[0].Cause != "anchor-missing" || run.RejectionReasons[1].Cause != "malformed" || run.RejectionReasons[2].Cause != "scope-unverifiable" {
		t.Errorf("RejectionReasons not in deterministic alphabetical order: %+v", run.RejectionReasons)
	}
}

// TestCompile_RejectionReasons_SamplesCapped (Q-2): a run with many
// same-cause rejections must cap the persisted example messages at
// maxRejectionSamplesPerCause, never persist one string per rejection.
func TestCompile_RejectionReasons_SamplesCapped(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	var claims []string
	for i := 0; i < maxRejectionSamplesPerCause+3; i++ {
		claims = append(claims, fmt.Sprintf(`{"rule":"Scope matches nothing %d.","scope":["nonexistent/dir/%d/**"],"cite_start":3,"cite_end":3}`, i, i))
	}
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[` + strings.Join(claims, ",") + `]}`,
	}})

	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsRejected != maxRejectionSamplesPerCause+3 {
		t.Fatalf("ClaimsRejected = %d, want %d", run.ClaimsRejected, maxRejectionSamplesPerCause+3)
	}
	if len(run.RejectionReasons) != 1 {
		t.Fatalf("RejectionReasons = %+v, want exactly 1 cause bucket", run.RejectionReasons)
	}
	rb := run.RejectionReasons[0]
	if rb.Cause != "scope-unverifiable" || rb.Count != maxRejectionSamplesPerCause+3 {
		t.Fatalf("bucket = %+v, want cause=scope-unverifiable count=%d", rb, maxRejectionSamplesPerCause+3)
	}
	if len(rb.Samples) != maxRejectionSamplesPerCause {
		t.Fatalf("samples = %d, want capped at %d", len(rb.Samples), maxRejectionSamplesPerCause)
	}
}

// TestCompile_DryRun_NeverPopulatesRejectionReasons (Q-2): verifyDraft never
// runs during a dry run, so RejectionReasons must stay nil, not a
// zero-valued breakdown.
func TestCompile_DryRun_NeverPopulatesRejectionReasons(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st)

	run, err := c.Compile(ctx, compileInput(true))
	if err != nil {
		t.Fatalf("Compile (dry run): %v", err)
	}
	if run.RejectionReasons != nil {
		t.Errorf("RejectionReasons = %+v, want nil on a dry run", run.RejectionReasons)
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

// certifies: invariant I6 (asserts)
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
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 200000, MaxUSDPerRun: 0.0001}
	c := &Compiler{
		St:            st,
		LLM:           mock,
		Model:         "mock-model",
		Layer:         layer,
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

// TestCompile_NoPriorRun_SkipsPreflightRefusal_AndBudgetStopsInstead
// (refactor-conservation gap-closing wave 3, item 3, CV-41): the SAME
// whole-corpus-exceeds-budget shape TestCompile_RefusesToStartWhenEstimated
// CostExceedsBudget refuses outright, but with CompileInput.NoPriorRun set
// (this service's very first compile of this layer) must instead proceed,
// compile what fits, and stop as BudgetStopped once the REAL usage crosses
// the token budget — never Refused, and never zero claims for a layer
// nothing has ever tried to compile before.
func TestCompile_NoPriorRun_SkipsPreflightRefusal_AndBudgetStopsInstead(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	// A single section whose real usage (scripted below) crosses the small
	// token budget — the pre-flight bytes/4 estimate (promptOverheadTokens
	// alone is 260) already exceeds MaxTokensPerRun=100 on ANY non-empty
	// section, which is exactly the shape that refuses outright today.
	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[{"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":3,"cite_end":3}]}`,
		Usage: llmclient.Usage{InputTokens: 150, OutputTokens: 50},
	}})
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 100, MaxUSDPerRun: 1000}
	c := &Compiler{
		St:            st,
		LLM:           mock,
		Model:         "mock-model",
		Layer:         layer,
		InRatePerKTok: 0.001,
		Now:           fixedNow,
	}

	in := compileInput(false)
	in.NoPriorRun = true
	run, err := c.Compile(ctx, in)
	if err != nil {
		t.Fatalf("a NoPriorRun compile must not error on a mid-run budget stop: %v", err)
	}
	if run.Refused {
		t.Errorf("run.Refused = true, want false — NoPriorRun must skip the pre-flight refusal")
	}
	if !run.BudgetStopped {
		t.Errorf("run.BudgetStopped = false, want true — the real usage still crosses the budget mid-run")
	}
	if run.SectionsCompiled != 1 || run.ClaimsEmitted != 1 {
		t.Errorf("compiled/emitted = %d/%d, want 1/1 — a first compile must make real progress, not zero", run.SectionsCompiled, run.ClaimsEmitted)
	}
	if mock.CallCount() != 1 {
		t.Errorf("CallCount = %d, want 1 — NoPriorRun must actually call the LLM, unlike a pre-flight refusal", mock.CallCount())
	}
	if run.ResumedFrom != "" {
		t.Errorf("ResumedFrom = %q, want empty — NoPriorRun names no resume point (nothing was being resumed)", run.ResumedFrom)
	}

	runs, err := st.CompileRuns(ctx, "svc", "")
	if err != nil || len(runs) != 1 {
		t.Fatalf("CompileRuns = %d, %v; want 1 recorded budget-stopped run", len(runs), err)
	}
	if runs[0].Refused {
		t.Errorf("recorded run.Refused = true, want false")
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	if len(claims) != 1 {
		t.Errorf("claims = %d, want 1 — a first compile must persist what it compiled before stopping", len(claims))
	}
}

// TestEnforceBudgets_MaxUSDPerRunZeroOrNegativeMeansUnlimited (wave O4,
// item 2) pins the fix: MaxUSDPerRun<=0 now means "no cost cap", mirroring
// MaxTokensPerRun's own ">0" convention just above it in enforceBudgets.
// Before this fix, a caller (a BudgetOverride to 0, or a hand-authored
// layer.cue) that set MaxUSDPerRun to 0 "for no cost cap" instead refused
// EVERY non-free run outright, since estCost > 0 for any real spend — the
// zero-value sentinel meant the opposite of every other budget field in
// this same function.
func TestEnforceBudgets_MaxUSDPerRunZeroOrNegativeMeansUnlimited(t *testing.T) {
	for _, max := range []float64{0, -1, -0.0001} {
		layer := LayerDef{Name: "obligations", Budgets: LayerBudgets{MaxUSDPerRun: max}}
		// A large, real token total that would price well above any normal
		// max_usd_per_run — the point is that it refuses NOTHING when the
		// cap itself is <=0.
		refuse, reason := enforceBudgets(layer, 1_000_000, 1_000_000, 100.0)
		if refuse {
			t.Errorf("MaxUSDPerRun=%v: enforceBudgets refused (%q), want unlimited (no cost cap)", max, reason)
		}
	}
}

// TestEnforceBudgets_MaxUSDPerRunPositiveStillEnforces: the fix above must
// not have quietly disabled the cost cap for its ordinary, positive case —
// a positive MaxUSDPerRun still refuses an estimate that exceeds it, exactly
// as TestCompile_RefusesToStartWhenEstimatedCostExceedsBudget already proves
// end-to-end; this pins enforceBudgets itself, directly.
func TestEnforceBudgets_MaxUSDPerRunPositiveStillEnforces(t *testing.T) {
	layer := LayerDef{Name: "obligations", Budgets: LayerBudgets{MaxUSDPerRun: 0.0001}}
	refuse, reason := enforceBudgets(layer, 1_000_000, 0, 100.0)
	if !refuse {
		t.Fatal("enforceBudgets did not refuse an estimate far over a positive max_usd_per_run")
	}
	if reason == "" {
		t.Error("refusal reason is empty")
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
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 600, MaxUSDPerRun: 1000}
	c := &Compiler{
		St:            st,
		LLM:           mock,
		Model:         "mock-model",
		Layer:         layer,
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
			Text: `{"claims":[{"rule":"Add retry to the query engine's outbound calls.","scope":["internal/query/**"],"cite_start":3,"cite_end":4,"class":"error-handling"}]}`,
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

// ─── findings-as-evidence: the closed class vocabulary (§3.2, §6 item 2) ────

// findingsCompilerWithClass builds a findings-layer Compiler whose mock LLM
// emits the given class string verbatim (or omits the field entirely when
// class == "" via omitClassField) — used to exercise verifyDraft's
// class-validation path directly through a real Compile call.
func findingsCompilerWithClass(t *testing.T, st store.Store, class string, omitClassField bool) *Compiler {
	t.Helper()
	layer, err := LoadLayer(LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayer(findings): %v", err)
	}
	classField := fmt.Sprintf(`,"class":%q`, class)
	if omitClassField {
		classField = ""
	}
	text := fmt.Sprintf(`{"claims":[{"rule":"Add retry to the query engine's outbound calls.","scope":["internal/query/**"],"cite_start":3,"cite_end":4%s}]}`, classField)
	return &Compiler{
		St:    st,
		LLM:   llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: text}}),
		Model: "mock-model",
		Layer: layer,
		Now:   fixedNow,
	}
}

// TestCompile_FindingClass_Valid_Persisted proves a valid class round-trips
// onto the persisted claim.
func TestCompile_FindingClass_Valid_Persisted(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	c := findingsCompilerWithClass(t, st, "error-handling", false)
	run, err := c.Compile(ctx, findingsInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted=%d rejected=%d, want 1/0", run.ClaimsEmitted, run.ClaimsRejected)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerFindings)
	if len(claims) != 1 {
		t.Fatalf("claims = %d, want 1", len(claims))
	}
	if claims[0].Class != core.FindingClassErrorHandling {
		t.Errorf("Class = %q, want %q", claims[0].Class, core.FindingClassErrorHandling)
	}
}

// TestCompile_FindingClass_Unknown_Rejected proves an unrecognized class
// (a reviewer-invented value, or a typo) is rejected at compile time —
// never silently accepted or bucketed as "other" (docs/design/findings-as-
// evidence.md §3.2).
func TestCompile_FindingClass_Unknown_Rejected(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	c := findingsCompilerWithClass(t, st, "other", false)
	run, err := c.Compile(ctx, findingsInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 0 || run.ClaimsRejected != 1 {
		t.Fatalf("emitted=%d rejected=%d, want 0/1", run.ClaimsEmitted, run.ClaimsRejected)
	}
	if len(run.RejectionReasons) != 1 || run.RejectionReasons[0].Cause != string(causeInvalidClass) {
		t.Fatalf("RejectionReasons = %+v, want one entry with cause %q", run.RejectionReasons, causeInvalidClass)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerFindings)
	if len(claims) != 0 {
		t.Fatalf("claims = %d, want 0 (rejected draft must not persist)", len(claims))
	}
}

// TestCompile_FindingClass_Empty_Rejected proves an omitted/empty class is
// rejected exactly like an unknown one — the vocabulary is mandatory for
// kind==KindFinding, not merely validated-if-present.
func TestCompile_FindingClass_Empty_Rejected(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	c := findingsCompilerWithClass(t, st, "", true)
	run, err := c.Compile(ctx, findingsInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 0 || run.ClaimsRejected != 1 {
		t.Fatalf("emitted=%d rejected=%d, want 0/1", run.ClaimsEmitted, run.ClaimsRejected)
	}
}

// TestCompile_FindingClass_NotEnforcedForNonFindingKind proves the
// vocabulary is scoped to kind==KindFinding only: an obligations-layer
// draft (which never emits "class" at all) is unaffected.
func TestCompile_FindingClass_NotEnforcedForNonFindingKind(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}]}`,
	}})
	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted=%d rejected=%d, want 1/0 (class must not be enforced on a non-finding-kind draft)", run.ClaimsEmitted, run.ClaimsRejected)
	}
}

// ─── LC-2: serving.drift_tracked replaces the kind=="finding" hardcode ──────

// TestCompile_ScopeFPGatedByDriftTrackedNotKind proves the LC-2 decoupling
// directly at the compile-time gate site: scope-fp anchors are written when
// the RESOLVED LAYER's serving.drift_tracked is true, regardless of the
// claim's Kind string — and withheld when it is false, even for a claim
// whose Kind is literally "finding". Kind is claim taxonomy; drift_tracked
// is a manifest-authored serving decision, and this test is the proof they
// are no longer the same axis.
func TestCompile_ScopeFPGatedByDriftTrackedNotKind(t *testing.T) {
	ctx := context.Background()

	// A layer named "finding"-kind but drift_tracked FALSE must NOT write
	// scope-fp anchors, even though the pre-LC-2 code gated on Kind alone.
	st1 := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st1.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	layer1, err := LoadLayer(LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayer(findings): %v", err)
	}
	layer1.Serving.DriftTracked = false // override: same kind, tracking OFF
	c1 := &Compiler{
		St: st1,
		LLM: llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Add retry to the query engine's outbound calls.","scope":["internal/query/**"],"cite_start":3,"cite_end":4,"class":"error-handling"}]}`,
		}}),
		Model: "mock-model",
		Layer: layer1,
		Now:   fixedNow,
	}
	if _, err := c1.Compile(ctx, findingsInput()); err != nil {
		t.Fatalf("Compile (drift_tracked=false): %v", err)
	}
	claims1, _ := st1.ClaimsForService(ctx, "svc", LayerFindings)
	if len(claims1) != 1 {
		t.Fatalf("claims = %d, want 1", len(claims1))
	}
	links1, err := st1.ClaimLinks(ctx, "svc", []string{claims1[0].ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	for _, l := range links1 {
		if l.Rel == core.RelScopeFP {
			t.Errorf("layer with drift_tracked=false wrote a scope-fp link (kind=%q): %+v", claims1[0].Kind, l)
		}
	}

	// The inverse: a layer whose Kind is NOT "finding" but whose
	// drift_tracked IS true must write scope-fp anchors.
	st2 := store.NewMemoryStore()
	for _, f := range indexedFiles {
		if err := st2.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	layer2, err := LoadLayer(LayerFindings)
	if err != nil {
		t.Fatalf("LoadLayer(findings): %v", err)
	}
	layer2.Name = "custom-layer"
	layer2.Kind = "custom-kind" // deliberately not "finding"
	layer2.Serving.DriftTracked = true
	c2 := &Compiler{
		St: st2,
		LLM: llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Add retry to the query engine's outbound calls.","scope":["internal/query/**"],"cite_start":3,"cite_end":4,"class":"error-handling"}]}`,
		}}),
		Model: "mock-model",
		Layer: layer2,
		Now:   fixedNow,
	}
	if _, err := c2.Compile(ctx, findingsInput()); err != nil {
		t.Fatalf("Compile (drift_tracked=true, kind=custom-kind): %v", err)
	}
	claims2, _ := st2.ClaimsForService(ctx, "svc", "custom-layer")
	if len(claims2) != 1 || claims2[0].Kind != "custom-kind" {
		t.Fatalf("claims = %+v, want 1 claim of kind custom-kind", claims2)
	}
	links2, err := st2.ClaimLinks(ctx, "svc", []string{claims2[0].ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	var scopeFP2 []core.ClaimLink
	for _, l := range links2 {
		if l.Rel == core.RelScopeFP {
			scopeFP2 = append(scopeFP2, l)
		}
	}
	if len(scopeFP2) != 1 {
		t.Errorf("layer with drift_tracked=true, kind=custom-kind: scope-fp links = %+v, want 1", scopeFP2)
	}
}

// TestDryRunEstimateRange pins O8 (plan IO-8): a dry run reports output
// tokens and cost as an honest RANGE, never input-only tokens with a
// confident zero cost — the shape that once preceded a $0.64, 21-minute
// "effectively free" compile.
func TestDryRunEstimateRange(t *testing.T) {
	st := store.NewMemoryStore()
	c := &Compiler{St: st, Now: fixedNow, InRatePerKTok: 0.001, OutRatePerKTok: 0.005} // no LLM: a dry run must not need one
	run, err := c.Compile(context.Background(), compileInput(true))
	if err != nil {
		t.Fatalf("dry-run compile: %v", err)
	}
	if run.TokensIn <= 0 {
		t.Fatalf("dry run estimated no input tokens; corpus empty? TokensIn=%d", run.TokensIn)
	}
	if run.EstTokensOutLow != run.TokensIn*2 || run.EstTokensOutHigh != run.TokensIn*8 {
		t.Errorf("output band = [%d, %d], want [%d, %d]", run.EstTokensOutLow, run.EstTokensOutHigh, run.TokensIn*2, run.TokensIn*8)
	}
	if !(run.EstCostLowUSD > 0 && run.EstCostHighUSD > run.EstCostLowUSD) {
		t.Errorf("cost band = [%f, %f], want a positive ascending range — never a confident zero", run.EstCostLowUSD, run.EstCostHighUSD)
	}
}

// ─── BP-32: the directory scope floor (O23's root cause) ───────────────────

// nestedGovDoc is a nested instruction file's section, shaped like
// internal/CLAUDE.md's real "adding an extractor" rule.
// Line numbers (1-based):
//
//	1: # Adding an extractor
//	2: (blank)
//	3: - Register the extractor in fusion.New and add a golden test.
//	4: - Never widen a rule beyond this package tree.
const nestedGovDoc = `# Adding an extractor

- Register the extractor in fusion.New and add a golden test.
- Never widen a rule beyond this package tree.
`

func nestedCompileInput() CompileInput {
	in := compileInput(false)
	in.Docs = []core.SourceFile{{Path: "internal/CLAUDE.md", Content: []byte(nestedGovDoc)}}
	in.IndexedFiles = []string{
		"internal/extractor/goextract.go",
		"internal/fusion/fusion.go",
		"cmd/fathom/main.go",
		"docs/x.md",
	}
	return in
}

// TestCompile_NestedInstructionFileFloorsScopeToItsSubtree (BP-32, O23's root
// cause): a claim compiled from internal/CLAUDE.md may only bind files under
// internal/. A file-class glob the model proposed repo-wide ("**/*.go") is
// floored to "internal/**/*.go" — deterministically, at compile time, with no
// second LLM call — so the edit hook serves the precise rule for
// internal/extractor/goextract.go instead of six broad root-corpus rules.
func TestCompile_NestedInstructionFileFloorsScopeToItsSubtree(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Register a new extractor in fusion.New and add a golden test.","scope":["**/*.go"],"cite_start":3,"cite_end":3},
  {"rule":"Keep extractor rules inside the package tree.","scope":["internal/extractor/**"],"cite_start":4,"cite_end":4}
]}`,
	}})

	run, err := c.Compile(ctx, nestedCompileInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 2 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted=%d rejected=%d, want 2/0", run.ClaimsEmitted, run.ClaimsRejected)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	got := map[string][]string{}
	for _, cl := range claims {
		got[cl.Rule] = cl.Scope
	}
	if s := got["Register a new extractor in fusion.New and add a golden test."]; len(s) != 1 || s[0] != "internal/**/*.go" {
		t.Errorf("repo-wide glob scope = %v, want [internal/**/*.go] (floored to the document's own subtree)", s)
	}
	if s := got["Keep extractor rules inside the package tree."]; len(s) != 1 || s[0] != "internal/extractor/**" {
		t.Errorf("already-inside glob scope = %v, want it unchanged", s)
	}
}

// TestCompile_NestedInstructionFileRejectsOutOfSubtreeScope (BP-32): a glob
// naming a DIFFERENT literal directory is a compile error named per claim —
// never silently widened, never silently re-homed under the document's own
// directory. A rule a human wrote in internal/CLAUDE.md cannot bind cmd/.
func TestCompile_NestedInstructionFileRejectsOutOfSubtreeScope(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Wire every new extractor into the CLI.","scope":["cmd/**"],"cite_start":3,"cite_end":3},
  {"rule":"Register a new extractor in fusion.New and add a golden test.","scope":["internal/fusion/**"],"cite_start":3,"cite_end":3}
]}`,
	}})

	run, err := c.Compile(ctx, nestedCompileInput())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 1 {
		t.Fatalf("emitted=%d rejected=%d, want 1/1 (cmd/** rejected, internal/fusion/** passes)", run.ClaimsEmitted, run.ClaimsRejected)
	}
	if len(run.RejectionReasons) != 1 || run.RejectionReasons[0].Cause != string(causeScopeOutOfSubtree) {
		t.Fatalf("RejectionReasons = %+v, want a single %q cause", run.RejectionReasons, causeScopeOutOfSubtree)
	}
	sample := strings.Join(run.RejectionReasons[0].Samples, " ")
	if !strings.Contains(sample, "cmd/**") || !strings.Contains(sample, "internal") {
		t.Errorf("rejection sample %q does not NAME the offending glob and the governing subtree", sample)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 1 || claims[0].Scope[0] != "internal/fusion/**" {
		t.Fatalf("stored claims = %+v, want only the in-subtree claim", claims)
	}
}

// TestScopeFloor_RootDocumentsAreUnchanged (BP-32): the floor is a NESTED
// instruction-file rule. Root CLAUDE.md, ADR.md and every other governing doc
// keep exactly the scope the model proposed — this change must not narrow the
// fleet-wide corpus that already works.
func TestScopeFloor_RootDocumentsAreUnchanged(t *testing.T) {
	for _, doc := range []string{"CLAUDE.md", "AGENTS.md", "ADR.md", "docs/plans/README.md", "docs/conventions.md"} {
		in := []string{"**/*.go", "cmd/**", "docs/**"}
		got, err := scopeFloor(doc, in)
		if err != nil {
			t.Fatalf("scopeFloor(%q): unexpected error %v", doc, err)
		}
		if strings.Join(got, ",") != strings.Join(in, ",") {
			t.Errorf("scopeFloor(%q) = %v, want %v unchanged", doc, got, in)
		}
	}
}

// TestScopeFloor_Table pins the per-glob decision table deterministically.
func TestScopeFloor_Table(t *testing.T) {
	const doc = "internal/CLAUDE.md"
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "internal/**", want: "internal/**"},
		{in: "internal", want: "internal"},
		{in: "internal/extractor/*.go", want: "internal/extractor/*.go"},
		{in: "**/*.go", want: "internal/**/*.go"},
		{in: "*_test.go", want: "internal/*_test.go"},
		{in: "goextract.go", want: "internal/goextract.go"},
		{in: "./**/*.go", want: "internal/**/*.go"},
		{in: "cmd/**", wantErr: true},
		{in: "docs/plans/*.md", wantErr: true},
		{in: "internalish/**", wantErr: true},
	} {
		got, err := scopeFloor(doc, []string{tc.in})
		if tc.wantErr {
			if err == nil {
				t.Errorf("scopeFloor(%q, %q) = %v, want an out-of-subtree error", doc, tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("scopeFloor(%q, %q): unexpected error %v", doc, tc.in, err)
			continue
		}
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("scopeFloor(%q, %q) = %v, want [%s]", doc, tc.in, got, tc.want)
		}
	}
}

// TestObligationsCorpusIncludesNestedInstructionFiles (BP-32): the corpus that
// was missing internal/CLAUDE.md — O23's root cause — must keep covering every
// nested instruction file, with the build/vendor trees excluded.
func TestObligationsCorpusIncludesNestedInstructionFiles(t *testing.T) {
	def, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	includes := map[string]bool{}
	for _, g := range def.CorpusInclude {
		includes[g] = true
	}
	if !includes["**/CLAUDE.md"] {
		t.Errorf("obligations corpus include = %v, want it to cover **/CLAUDE.md (O23: internal/CLAUDE.md's rules were never compiled)", def.CorpusInclude)
	}
	// ADR-059 (AG-13): AGENTS.md is the one instruction file — root,
	// .claude/ and nested — so a converted repo keeps its governing corpus.
	for _, want := range []string{"AGENTS.md", "**/AGENTS.md", ".claude/AGENTS.md", ".claude/CLAUDE.md"} {
		if !includes[want] {
			t.Errorf("obligations corpus include = %v, want it to carry %q (ADR-059)", def.CorpusInclude, want)
		}
	}
	for _, want := range []string{"third_party/**", ".claude/worktrees/**", "workspace/**", "node_modules/**"} {
		var found bool
		for _, g := range def.CorpusExclude {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("obligations corpus exclude = %v, want it to carry %q", def.CorpusExclude, want)
		}
	}

	// The floor and the corpus are one change: widening the corpus without
	// it would let a nested file's rules compile repo-wide scopes.
	files := []core.SourceFile{
		{Path: "CLAUDE.md"},
		{Path: "AGENTS.md"},
		{Path: "internal/CLAUDE.md"},
		{Path: "internal/AGENTS.md"},
		{Path: "third_party/x/AGENTS.md"},
		{Path: "cmd/fathom/CLAUDE.md"},
		{Path: "third_party/x/CLAUDE.md"},
		{Path: ".claude/worktrees/w/internal/CLAUDE.md"},
		{Path: "workspace/tmp/CLAUDE.md"},
		{Path: "node_modules/p/CLAUDE.md"},
		{Path: "ADR.md"},
		{Path: "docs/design/essay.md"},
	}
	var got []string
	for _, f := range SelectCorpus(files, def.CorpusInclude, def.CorpusExclude) {
		got = append(got, f.Path)
	}
	want := "ADR.md,AGENTS.md,CLAUDE.md,cmd/fathom/CLAUDE.md,internal/AGENTS.md,internal/CLAUDE.md"
	if strings.Join(got, ",") != want {
		t.Errorf("SelectCorpus = %v, want %s", got, want)
	}
}

// ─── O54: nearest-scope-first compile order, layer coverage, resume ────────

// TestCompile_BudgetStopLeavesNearestGuidesCompiledRootTailUncompiled is
// O54 deliverable 1's integration proof: a fixture corpus shaped like the
// obligations layer's real one (nested package guides, the root's own
// CLAUDE.md, and ADR.md) compiled under a small budget leaves the NESTED
// GUIDES and the ROOT CLAUDE.md's own Non-negotiables section compiled,
// and ADR.md the uncompiled tail — never the reverse, which is what plain
// alphabetical doc order produced (ADR.md sorts before every CLAUDE.md).
// Second review round: a two-tier split alone still starved the root
// CLAUDE.md's own governing section by sorting it into the same
// size-ordered bucket as ADR.md — this corpus's root Non-negotiables
// section is the LARGEST section in the whole corpus, and the assertion
// below is exactly "it compiles before any ADR.md section regardless".
// Also pins deliverable 2's UncompiledScopes source data.
func TestCompile_BudgetStopLeavesNearestGuidesCompiledRootTailUncompiled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	docs := CompileInput{
		ServiceID: "svc",
		Docs: []core.SourceFile{
			// Alphabetically FIRST (would compile first under the old,
			// plain doc-sort order) but ROOT-OTHER tier — must compile LAST.
			{Path: "ADR.md", Content: []byte("# Decisions\n\n- Some root decision.\n")},
			// The root's OWN guide: a single large Non-negotiables section
			// (the largest section in this whole corpus by byte count) —
			// ROOT-GUIDE tier, ahead of ADR.md regardless of size.
			{Path: "CLAUDE.md", Content: []byte("## Non-negotiables\n\n- Deploy = task deploy, never a raw cp; an unsigned binary loses the daemon's TCC grant.\n")},
			// Nested guides — NEAREST-SCOPE tier, must compile before both
			// root tiers. Document order (path) is the tier's own
			// tie-break now, not size: cmd/fathom sorts before internal.
			{Path: "cmd/fathom/CLAUDE.md", Content: []byte("# cmd fathom\n\n- Wire every new verb into verbs.go.\n")},
			{Path: "internal/CLAUDE.md", Content: []byte("# internal\n\n- Register every extractor in fusion.New.\n")},
		},
		IndexedFiles: []string{
			"internal/extractor/goextract.go",
			"cmd/fathom/compile.go",
			"pkg/client/client.go",
			"docs/x.md",
			"schema/semantic/schema.cue",
		},
		Watermark: "manifest:aaa",
	}

	mock := llmclient.NewMockClient(
		// cmd/fathom/CLAUDE.md's section: nested tier, first by path.
		llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"claims":[{"rule":"Wire every new verb into verbs.go.","scope":["cmd/fathom/**"],"cite_start":3,"cite_end":3}]}`,
			Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 50},
		}},
		// internal/CLAUDE.md's section: nested tier, second by path.
		llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"claims":[{"rule":"Register every extractor in fusion.New.","scope":["internal/**"],"cite_start":3,"cite_end":3}]}`,
			Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 50},
		}},
		// CLAUDE.md's Non-negotiables section: root-GUIDE tier, ahead of
		// ADR.md regardless of being the largest section in the corpus —
		// its real usage is what crosses the budget.
		llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"claims":[{"rule":"Deploy = task deploy, never a raw cp.","scope":["docs/x.md"],"cite_start":3,"cite_end":3}]}`,
			Usage: llmclient.Usage{InputTokens: 5000, OutputTokens: 100},
		}},
		// ADR.md must NEVER be reached — no entry scripted for it; a call
		// here would still return the mock's harmless fallback text, so
		// mock.CallCount() is what actually proves it was never asked.
	)
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	// A generous budget so the PRE-FLIGHT estimate (a cheap bytes/4
	// heuristic over all 4 sections' text, including the deliberately
	// verbose Non-negotiables section) never refuses the run outright —
	// only the REAL usage scripted above crosses it, mid-run.
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 5000, MaxUSDPerRun: 100000}
	c := &Compiler{
		St:            st,
		LLM:           mock,
		Model:         "mock-model",
		Layer:         layer,
		InRatePerKTok: 0.001,
		Now:           fixedNow,
	}
	run, err := c.Compile(ctx, docs)
	if err != nil {
		t.Fatalf("a clean mid-run budget stop must not return an error: %v", err)
	}
	if !run.BudgetStopped {
		t.Fatalf("run.BudgetStopped = false, want true")
	}
	if run.SectionsTotal != 4 {
		t.Fatalf("run.SectionsTotal = %d, want 4", run.SectionsTotal)
	}
	if run.SectionsCompiled != 3 {
		t.Fatalf("run.SectionsCompiled = %d, want 3 (both nested guides plus the root's own Non-negotiables — ADR.md never reached)", run.SectionsCompiled)
	}
	if mock.CallCount() != 3 {
		t.Fatalf("mock.CallCount() = %d, want 3 — ADR.md must never be reached", mock.CallCount())
	}
	if want := []string{"ADR.md"}; len(run.UncompiledScopes) != 1 || run.UncompiledScopes[0] != want[0] {
		t.Fatalf("run.UncompiledScopes = %v, want %v", run.UncompiledScopes, want)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 3 {
		t.Fatalf("len(claims) = %d, want 3", len(claims))
	}
	var sawNonNegotiables bool
	for _, cl := range claims {
		if strings.HasPrefix(cl.Subject, "ADR.md#") {
			t.Fatalf("claim %s compiled from ADR.md — the root-other tail must never compile before a budget stop that starves the guides", cl.ClaimID)
		}
		if cl.Subject == "CLAUDE.md#non-negotiables" {
			sawNonNegotiables = true
		}
	}
	if !sawNonNegotiables {
		t.Fatalf("the root CLAUDE.md's Non-negotiables section never compiled: claims=%+v", claims)
	}

	// The recorded compile_runs row is what layer_coverage (O54 deliverable
	// 2) reads at serve time.
	cov, found, cerr := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if cerr != nil || !found {
		t.Fatalf("ObligationsLayerCoverage: found=%v err=%v", found, cerr)
	}
	if cov.SectionsTotal != 4 || cov.SectionsCompiled != 3 || !cov.BudgetStopped {
		t.Fatalf("coverage = %+v, want {4 3 true ...}", cov)
	}
	// The uncompiled tail is ADR.md alone — root-OTHER tier, never a
	// "nearest guide" for any file. NearestUncompiledGuide must say so.
	if guide, ok := NearestUncompiledGuide(cov, "internal/extractor/goextract.go"); ok {
		t.Fatalf("NearestUncompiledGuide = (%q, true), want ok=false — internal/CLAUDE.md and the root CLAUDE.md both DID compile; only the root-other ADR.md tail is uncompiled", guide)
	}
}

// TestCompile_RepoShapedCorpus_EighteenSectionBudget_CoversEveryGuide is
// the second review round's own reproduction target: a corpus shaped like
// THIS repository's own obligations corpus (4 nested guides, a root
// CLAUDE.md with 7 sections, ADR.md with 37) under an 18-section budget.
// Measured on this repository (see this round's review), the root
// CLAUDE.md's Non-negotiables section alone is 1,742 bytes — larger than
// every one of ADR.md's individual sections — and a size-only sort over
// "everything root-scoped" ranked it 65th of 65, well outside an 18-section
// budget. With the three-tier fix: every nested guide section AND every
// root CLAUDE.md section (11 total) compile before a single ADR.md
// section, and the remaining 7 sections of budget land on ADR.md's own
// first 7 sections (document order, since they are byte-length ties) —
// ADR.md is the part that stops, never a guide.
func TestCompile_RepoShapedCorpus_EighteenSectionBudget_CoversEveryGuide(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	const (
		nestedGuides  = 4
		rootSections  = 7
		adrSections   = 37
		wantCompiled  = 18
		wantFromGuide = nestedGuides + rootSections // 11
	)

	var docs []core.SourceFile
	nestedDirs := []string{"aaa", "bbb", "ccc", "ddd"}
	for _, dir := range nestedDirs {
		docs = append(docs, core.SourceFile{
			Path:    dir + "/CLAUDE.md",
			Content: []byte(fmt.Sprintf("# guide\n\n- A rule governing %s.\n\n", dir)),
		})
	}
	var rootBuf strings.Builder
	for i := 1; i <= rootSections; i++ {
		fmt.Fprintf(&rootBuf, "## Root section %d\n\n- Root rule %d.\n\n", i, i)
	}
	docs = append(docs, core.SourceFile{Path: "CLAUDE.md", Content: []byte(rootBuf.String())})
	var adrBuf strings.Builder
	for i := 1; i <= adrSections; i++ {
		fmt.Fprintf(&adrBuf, "## Decision %d\n\n- Decision rule %d.\n\n", i, i)
	}
	docs = append(docs, core.SourceFile{Path: "ADR.md", Content: []byte(adrBuf.String())})

	indexedFiles := []string{"aaa/x.go", "bbb/x.go", "ccc/x.go", "ddd/x.go", "pkg/y.go", "docs/z.md"}

	// Reconstruct the EXACT order Compile() will use, so the scripted mock
	// lines up 1:1 with the real compile order regardless of any future
	// ordering-detail change — this test asserts on the PRODUCTION
	// function's own answer, never a hand-predicted guess.
	var all []docSections
	for _, d := range docs {
		all = append(all, docSections{doc: d, sections: SplitMarkdownSections(d.Path, d.Content)})
	}
	ordered := orderSectionsByGoverningScope(all)
	if len(ordered) != nestedGuides+rootSections+adrSections {
		t.Fatalf("total sections = %d, want %d", len(ordered), nestedGuides+rootSections+adrSections)
	}
	for i := 0; i < wantFromGuide; i++ {
		if docCompileTier(ordered[i].sec.Path) == docTierRootOther {
			t.Fatalf("ordered[%d] = %s is root-OTHER tier, want every one of the first %d sections to be a guide (nested or root)", i, ordered[i].sec.Path, wantFromGuide)
		}
	}
	for i := wantFromGuide; i < wantCompiled; i++ {
		if ordered[i].sec.Path != "ADR.md" {
			t.Fatalf("ordered[%d] = %s, want ADR.md filling the remaining budget after every guide section", i, ordered[i].sec.Path)
		}
	}

	scopeFor := func(docPath string) []string {
		switch docPath {
		case "CLAUDE.md":
			return []string{"pkg/y.go"}
		case "ADR.md":
			return []string{"docs/z.md"}
		default:
			return []string{strings.TrimSuffix(docPath, "/CLAUDE.md") + "/**"}
		}
	}
	entries := make([]llmclient.MockEntry, len(ordered))
	for i, item := range ordered {
		usage := llmclient.Usage{InputTokens: 10, OutputTokens: 0}
		if i == wantCompiled-1 {
			// The 18th section's real usage is what crosses the budget.
			usage = llmclient.Usage{InputTokens: 100000, OutputTokens: 0}
		}
		scope, _ := json.Marshal(scopeFor(item.sec.Path))
		claim := fmt.Sprintf(`{"claims":[{"rule":"Rule for section %d.","scope":%s,"cite_start":%d,"cite_end":%d}]}`,
			i, scope, item.sec.StartLine+2, item.sec.StartLine+2)
		entries[i] = llmclient.MockEntry{Response: llmclient.Response{Text: claim, Usage: usage}}
	}
	mock := llmclient.NewMockClient(entries...)

	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 50000, MaxUSDPerRun: 1e9}
	c := &Compiler{St: st, LLM: mock, Model: "mock-model", Layer: layer, InRatePerKTok: 0, Now: fixedNow}
	run, err := c.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: docs, IndexedFiles: indexedFiles, Watermark: "manifest:aaa",
	})
	if err != nil {
		t.Fatalf("a clean mid-run budget stop must not return an error: %v", err)
	}
	if !run.BudgetStopped {
		t.Fatalf("run.BudgetStopped = false, want true")
	}
	if run.SectionsTotal != nestedGuides+rootSections+adrSections {
		t.Fatalf("run.SectionsTotal = %d, want %d", run.SectionsTotal, nestedGuides+rootSections+adrSections)
	}
	if run.SectionsCompiled != wantCompiled {
		t.Fatalf("run.SectionsCompiled = %d, want %d", run.SectionsCompiled, wantCompiled)
	}
	if mock.CallCount() != wantCompiled {
		t.Fatalf("mock.CallCount() = %d, want %d — ADR.md's remaining sections must never be reached", mock.CallCount(), wantCompiled)
	}
	if want := []string{"ADR.md"}; len(run.UncompiledScopes) != 1 || run.UncompiledScopes[0] != want[0] {
		t.Fatalf("run.UncompiledScopes = %v, want %v — ADR.md is the part that stops, never a guide", run.UncompiledScopes, want)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != wantCompiled {
		t.Fatalf("len(claims) = %d, want %d", len(claims), wantCompiled)
	}
	wantSubjects := map[string]bool{}
	for _, item := range ordered[:wantCompiled] {
		wantSubjects[item.sec.Subject] = true
	}
	for _, cl := range claims {
		if !wantSubjects[cl.Subject] {
			t.Errorf("unexpected claim compiled: %s (subject %s)", cl.ClaimID, cl.Subject)
		}
		delete(wantSubjects, cl.Subject)
	}
	if len(wantSubjects) != 0 {
		t.Errorf("sections that should have compiled but produced no claim: %v", wantSubjects)
	}
	// Every nested guide and every root CLAUDE.md section — all 11 — must
	// be among the compiled claims.
	var guideClaims int
	for _, cl := range claims {
		if docCompileTier(strings.SplitN(cl.Subject, "#", 2)[0]) != docTierRootOther {
			guideClaims++
		}
	}
	if guideClaims != wantFromGuide {
		t.Fatalf("guide-tier claims compiled = %d, want %d (every nested guide + every root CLAUDE.md section)", guideClaims, wantFromGuide)
	}
}

// TestCompile_Resume_SkipsPreflightRefusalAndMakesProgress is O54
// deliverable 3: a layer budgeted BELOW promptOverheadTokens (260) refuses
// EVERY plain compile at the pre-flight gate before a single section is
// ever attempted — the layer is permanently wedged, not merely stopped
// mid-run, since even ONE section's fixed overhead alone exceeds the
// budget. Resume:true skips that refusal so real progress happens where a
// plain call makes none, and records which section it started from.
func TestCompile_Resume_SkipsPreflightRefusalAndMakesProgress(t *testing.T) {
	ctx := context.Background()

	oneDoc := CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{{Path: "internal/CLAUDE.md", Content: []byte(govDoc)}},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
	}
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	// Below promptOverheadTokens (260): no section, however small, can
	// ever pass the pre-flight estimate — a plain compile of this layer is
	// wedged from the very first invocation, not just a resume.
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 50, MaxUSDPerRun: 1000}

	t.Run("plain compile refuses and makes zero progress", func(t *testing.T) {
		st := store.NewMemoryStore()
		mock := llmclient.NewMockClient() // no entries: a call would prove the gate leaked
		c := &Compiler{St: st, LLM: mock, Model: "mock-model", Layer: layer, InRatePerKTok: 0.001, Now: fixedNow}
		run, err := c.Compile(ctx, oneDoc)
		if err == nil {
			t.Fatal("want a pre-flight refusal, got nil error")
		}
		if !run.Refused || run.SectionsCompiled != 0 {
			t.Fatalf("run = %+v, want Refused=true, SectionsCompiled=0", run)
		}
		if mock.CallCount() != 0 {
			t.Errorf("CallCount = %d, want 0 — refused before any LLM call", mock.CallCount())
		}
	})

	t.Run("resume skips the refusal and compiles", func(t *testing.T) {
		st := store.NewMemoryStore()
		mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text:  `{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`,
			Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 50},
		}})
		c := &Compiler{St: st, LLM: mock, Model: "mock-model", Layer: layer, InRatePerKTok: 0.001, Now: fixedNow}
		run, err := c.Compile(ctx, CompileInput{
			ServiceID: oneDoc.ServiceID, Docs: oneDoc.Docs, IndexedFiles: oneDoc.IndexedFiles,
			Watermark: oneDoc.Watermark, Resume: true,
		})
		if err != nil {
			t.Fatalf("resume Compile: %v", err)
		}
		if run.Refused {
			t.Fatalf("run.Refused = true, want false — Resume must skip the pre-flight refusal")
		}
		if run.SectionsCompiled != 1 || run.ClaimsEmitted != 1 {
			t.Fatalf("compiled/emitted = %d/%d, want 1/1", run.SectionsCompiled, run.ClaimsEmitted)
		}
		if mock.CallCount() != 1 {
			t.Errorf("CallCount = %d, want 1", mock.CallCount())
		}
		wantSubject := "internal/CLAUDE.md#conventions"
		if run.ResumedFrom != wantSubject {
			t.Errorf("run.ResumedFrom = %q, want %q", run.ResumedFrom, wantSubject)
		}
		// The real usage (150) also exceeds the tiny 50-token budget, so
		// the mid-run gate stops the run cleanly right after — a
		// DIFFERENT, already-existing mechanism (BudgetStopped, not
		// Refused) that Resume never needs to touch.
		if !run.BudgetStopped {
			t.Errorf("run.BudgetStopped = false, want true (mid-run stop after the one section it compiled)")
		}

		runs, err := st.CompileRuns(ctx, "svc", LayerObligations)
		if err != nil || len(runs) != 1 {
			t.Fatalf("CompileRuns: %d, %v", len(runs), err)
		}
		if runs[0].ResumedFrom != wantSubject {
			t.Errorf("recorded run.ResumedFrom = %q, want %q", runs[0].ResumedFrom, wantSubject)
		}
	})
}

// TestCompile_Resume_NamesFirstNotYetCompiledSection_DryRun is O54
// deliverable 3's "test with the dry-run seam": ResumedFrom names the
// first section NOT already covered by a prior compile — never the
// already-digest-gated one — with no LLM call at all (DryRun structurally
// never calls the LLM, independent of Resume).
func TestCompile_Resume_NamesFirstNotYetCompiledSection_DryRun(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	twoDocs := CompileInput{
		ServiceID: "svc",
		Docs: []core.SourceFile{
			{Path: "internal/CLAUDE.md", Content: []byte("# internal\n\n- Register every extractor in fusion.New.\n")},
			{Path: "cmd/fathom/CLAUDE.md", Content: []byte("# cmd fathom\n\n- Wire every new verb into verbs.go.\n")},
		},
		IndexedFiles: indexedFiles,
		Watermark:    "manifest:aaa",
	}

	// Seed internal/CLAUDE.md's section as already compiled (an unbudgeted
	// compiler, mirroring how a real prior run would have written it).
	seed := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Register every extractor in fusion.New.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	cSeed := &Compiler{St: st, LLM: seed, Model: "mock-model", Now: fixedNow}
	seedRun, err := cSeed.Compile(ctx, CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{twoDocs.Docs[0]},
		IndexedFiles: twoDocs.IndexedFiles, Watermark: twoDocs.Watermark,
	})
	if err != nil || seedRun.SectionsCompiled != 1 {
		t.Fatalf("seed compile: run=%+v err=%v", seedRun, err)
	}

	// Now a DryRun+Resume over BOTH docs: internal/CLAUDE.md is
	// digest-gated (already compiled), so ResumedFrom must name
	// cmd/fathom/CLAUDE.md's section — never restate the first one.
	dry := llmclient.NewMockClient() // a call here would fail CallCount below
	cDry := &Compiler{St: st, LLM: dry, Model: "mock-model", Now: fixedNow}
	run, err := cDry.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: twoDocs.Docs, IndexedFiles: twoDocs.IndexedFiles,
		Watermark: twoDocs.Watermark, DryRun: true, Resume: true,
	})
	if err != nil {
		t.Fatalf("dry-run resume Compile: %v", err)
	}
	if dry.CallCount() != 0 {
		t.Fatalf("CallCount = %d, want 0 — a dry run never calls the LLM", dry.CallCount())
	}
	wantSubject := "cmd/fathom/CLAUDE.md#cmd-fathom"
	if run.ResumedFrom != wantSubject {
		t.Fatalf("run.ResumedFrom = %q, want %q (internal/CLAUDE.md is already compiled)", run.ResumedFrom, wantSubject)
	}
	if run.SectionsCompiled != 1 {
		t.Fatalf("run.SectionsCompiled = %d, want 1 (only the not-yet-compiled section counts, even in estimate mode)", run.SectionsCompiled)
	}
}

// TestCompile_UncompiledScopes_ExcludesAlreadyCompiledSectionsInTheTail is
// the second review round's fix for deliverable 2/3's own bug: labelling
// every doc in ordered[stoppedAt+1:] uncompiled, with no digest-gate
// check, fabricated a claim in the OTHER direction — an already-compiled
// doc later in the compile order got reported uncompiled just because
// THIS run's stop point happened to land before it.
//
// Run 1 compiles A, B, C (three nested guides) in full. A small guide D is
// then added whose path sorts BETWEEN B and C — so the compile order
// becomes A, B, D, C. Run 2's budget stops right after D (A and B are
// digest-gated, spending nothing; D is the only section that actually
// compiles and its real usage crosses the budget) — the ordered TAIL is
// exactly [C], but C already has a valid, digest-matching claim from run
// 1: UncompiledScopes must not name it.
func TestCompile_UncompiledScopes_ExcludesAlreadyCompiledSectionsInTheTail(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	docA := core.SourceFile{Path: "aaa/CLAUDE.md", Content: []byte("# guide\n\n- A rule for aaa.\n")}
	docB := core.SourceFile{Path: "bbb/CLAUDE.md", Content: []byte("# guide\n\n- A rule for bbb.\n")}
	docC := core.SourceFile{Path: "zzz/CLAUDE.md", Content: []byte("# guide\n\n- A rule for zzz.\n")}
	// D sorts between B and C alphabetically (aaa < bbb < ccc < zzz).
	docD := core.SourceFile{Path: "ccc/CLAUDE.md", Content: []byte("# guide\n\n- A rule for ccc.\n")}

	indexed := []string{"aaa/x.go", "bbb/x.go", "ccc/x.go", "zzz/x.go", "pkg/y.go", "docs/z.md"}

	// Run 1: A, B, C compile in full (no budget).
	mock1 := llmclient.NewMockClient(
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Rule A.","scope":["aaa/**"],"cite_start":3,"cite_end":3}]}`}},
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Rule B.","scope":["bbb/**"],"cite_start":3,"cite_end":3}]}`}},
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Rule C.","scope":["zzz/**"],"cite_start":3,"cite_end":3}]}`}},
	)
	c1 := &Compiler{St: st, LLM: mock1, Model: "mock-model", Now: fixedNow}
	run1, err := c1.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: []core.SourceFile{docA, docB, docC}, IndexedFiles: indexed, Watermark: "manifest:aaa",
	})
	if err != nil || run1.SectionsCompiled != 3 {
		t.Fatalf("run1: %+v, %v", run1, err)
	}

	// Run 2: A, B, C, D — A and B are digest-gated (free), D compiles and
	// its real usage crosses a small budget, stopping before C is reached.
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	layer.Budgets = LayerBudgets{MaxTokensPerRun: 300, MaxUSDPerRun: 1000}
	mock2 := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[{"rule":"Rule D.","scope":["ccc/**"],"cite_start":3,"cite_end":3}]}`,
		Usage: llmclient.Usage{InputTokens: 500, OutputTokens: 100},
	}})
	c2 := &Compiler{St: st, LLM: mock2, Model: "mock-model", Layer: layer, InRatePerKTok: 0.001, Now: fixedNow}
	run2, err := c2.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: []core.SourceFile{docA, docB, docC, docD}, IndexedFiles: indexed, Watermark: "manifest:aaa",
	})
	if err != nil {
		t.Fatalf("run2: a clean mid-run budget stop must not return an error: %v", err)
	}
	if !run2.BudgetStopped {
		t.Fatalf("run2.BudgetStopped = false, want true")
	}
	if run2.SectionsCompiled != 1 {
		t.Fatalf("run2.SectionsCompiled = %d, want 1 (only D — A/B are digest-gated, C never reached)", run2.SectionsCompiled)
	}
	if mock2.CallCount() != 1 {
		t.Fatalf("mock2.CallCount() = %d, want 1 (only D)", mock2.CallCount())
	}
	// The bug: C (zzz/CLAUDE.md), the ordered tail after D, already has a
	// valid claim from run 1 — it must NOT be reported uncompiled.
	for _, doc := range run2.UncompiledScopes {
		if doc == "zzz/CLAUDE.md" {
			t.Fatalf("run2.UncompiledScopes = %v, must not name zzz/CLAUDE.md — it already compiled in run 1 and its digest is unchanged", run2.UncompiledScopes)
		}
	}
	if len(run2.UncompiledScopes) != 0 {
		t.Fatalf("run2.UncompiledScopes = %v, want empty — D compiled this run and C was already compiled in run 1; nothing is actually left uncompiled", run2.UncompiledScopes)
	}

	all, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(all) != 4 {
		t.Fatalf("total claims = %d, %v; want 4 (A, B, C from run 1 plus D from run 2)", len(all), err)
	}
}

// twoRuleDoc has two sections, one clearly a binding rule (Rule A) and one
// this test's mock decision port scores below threshold (Rule B) — the
// compile pre-filter's own fixture (ADR-042, vision 32 §2 item 1).
// Line numbers (1-based):
//
//	1: # Rule A
//	2: (blank)
//	3: - Always do the thing.
//	4: (blank)
//	5: # Rule B
//	6: (blank)
//	7: - Never do the other thing.
const twoRuleDoc = `# Rule A

- Always do the thing.

# Rule B

- Never do the other thing.
`

// TestCompile_PrefilterSkipsAndDiscloses is DP-5/DP-6 (ADR-042, vision 32
// §2 item 1): a decision.MockDecider answers noul 0.9 for "Rule A" and 0.2
// for "Rule B" at threshold 0.5 — only Rule A reaches the compile model
// (the mock LLM client has exactly one scripted response and CallCount
// proves it was used exactly once). Rule B is never compiled and never
// failed: it is counted in SectionsPrefiltered/PrefilteredSections, folded
// into UncompiledScopes exactly like a budget-stopped section, round-trips
// through the store's sections_prefiltered column, surfaces as
// layer_coverage's prefilter_skipped, and ADR-037's raw fallback can still
// serve it.
func TestCompile_PrefilterSkipsAndDiscloses(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	llmMock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[{"rule":"Always do the thing.","scope":["internal/query/**"],"cite_start":3,"cite_end":3}]}`,
		Usage: llmclient.Usage{InputTokens: 60, OutputTokens: 20},
	}})
	decider := decision.NewMockDecider("mock-decide", decision.MockEntry{Response: decision.Response{
		Provider: "mock-decide",
		Answers: map[string]decision.Answer{
			"docs/twosections.md#rule-a": {ID: "docs/twosections.md#rule-a", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
			"docs/twosections.md#rule-b": {ID: "docs/twosections.md#rule-b", Kind: decision.KindPredicate, Noul: 0.2, Confidence: 0.8},
		},
	}})

	c := &Compiler{St: st, LLM: llmMock, Model: "mock-model", Now: fixedNow, Decider: decider, PrefilterThreshold: 0.5}
	docs := []core.SourceFile{{Path: "docs/twosections.md", Content: []byte(twoRuleDoc)}}
	run, err := c.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: docs, IndexedFiles: indexedFiles, Watermark: "manifest:aaa",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if llmMock.CallCount() != 1 {
		t.Fatalf("llmMock.CallCount() = %d, want 1 — Rule B must never reach the compile model", llmMock.CallCount())
	}
	if run.SectionsCompiled != 1 || run.SectionsFailed != 0 {
		t.Fatalf("SectionsCompiled/Failed = %d/%d, want 1/0", run.SectionsCompiled, run.SectionsFailed)
	}
	if run.SectionsPrefiltered != 1 {
		t.Fatalf("SectionsPrefiltered = %d, want 1", run.SectionsPrefiltered)
	}
	if want := []string{"docs/twosections.md#rule-b"}; !reflect.DeepEqual(run.PrefilteredSections, want) {
		t.Fatalf("PrefilteredSections = %v, want %v", run.PrefilteredSections, want)
	}
	if want := []string{"docs/twosections.md"}; !reflect.DeepEqual(run.UncompiledScopes, want) {
		t.Fatalf("UncompiledScopes = %v, want %v (Rule B's doc, exactly like a budget-stopped tail)", run.UncompiledScopes, want)
	}
	if len(run.Decisions) == 0 {
		t.Fatalf("Decisions is empty, want the compile_prefilter records for both questions")
	}

	// Store round-trip: the additive sections_prefiltered column.
	runs, err := st.CompileRuns(ctx, "svc", LayerObligations)
	if err != nil || len(runs) != 1 {
		t.Fatalf("CompileRuns = %d, %v; want 1 recorded run", len(runs), err)
	}
	if runs[0].SectionsPrefiltered != 1 {
		t.Errorf("stored run SectionsPrefiltered = %d, want 1 (round-tripped)", runs[0].SectionsPrefiltered)
	}

	// layer_coverage discloses prefilter_skipped.
	cov, found, cerr := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if cerr != nil || !found {
		t.Fatalf("ObligationsLayerCoverage: found=%v err=%v", found, cerr)
	}
	if cov.PrefilterSkipped != 1 {
		t.Errorf("layer_coverage.PrefilterSkipped = %d, want 1", cov.PrefilterSkipped)
	}

	// ADR-037 raw fallback can still serve the prefiltered section.
	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	layerDef, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer: %v", err)
	}
	rawRows, err := RawFallbackRules(claims, layerDef, docs, "internal/query/engine.go", 2, "prefiltered", nil)
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	var sawRuleB bool
	for _, row := range rawRows {
		if row.Tier == "raw" && row.Rule == "Rule B" {
			sawRuleB = true
		}
	}
	if !sawRuleB {
		t.Errorf("RawFallbackRules = %+v, want a raw row for the prefiltered Rule B section", rawRows)
	}
}
