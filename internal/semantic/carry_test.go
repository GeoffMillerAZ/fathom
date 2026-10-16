package semantic

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// movedGovDoc is govDoc two lines lower (a blank preamble is dropped by the
// splitter), so the "Conventions" section keeps its exact bytes and digest
// while every line number shifts by two: lines 3 and 4 of govDoc are lines
// 5 and 6 here.
const movedGovDoc = "\n\n" + govDoc

// twoClaimResponse answers govDoc's section with two claims citing lines 3
// and 4 (document coordinates of govDoc at line 1).
const twoClaimResponse = `{"claims":[
  {"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":3,"cite_end":3},
  {"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":4,"cite_end":4}
]}`

// movedResponse is the same answer in movedGovDoc's coordinates.
const movedResponse = `{"claims":[
  {"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":5,"cite_end":5},
  {"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":6,"cite_end":6}
]}`

func carryInput(dryRun bool, docs ...core.SourceFile) CompileInput {
	in := compileInput(dryRun)
	in.Docs = docs
	return in
}

// compileWith runs one compile against st with a mock answering entries in
// order, and returns the run and the mock (for its call count).
func compileWith(t *testing.T, st store.Store, in CompileInput, responses ...string) (core.CompileRun, *llmclient.MockClient) {
	t.Helper()
	entries := make([]llmclient.MockEntry, 0, len(responses))
	for _, r := range responses {
		entries = append(entries, llmclient.MockEntry{Response: llmclient.Response{Text: r}})
	}
	mock := llmclient.NewMockClient(entries...)
	c := &Compiler{St: st, LLM: mock, Model: "mock-model", Now: fixedNow}
	run, err := c.Compile(context.Background(), in)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return run, mock
}

func claimsBySubject(t *testing.T, st store.Store) map[string][]core.SemanticClaim {
	t.Helper()
	all, err := st.ClaimsForService(context.Background(), "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	out := map[string][]core.SemanticClaim{}
	for _, cl := range all {
		out[cl.Subject] = append(out[cl.Subject], cl)
	}
	return out
}

// TestCompileCarriesClaimsAcrossAMovedSection is AG-26 (docs/plans/
// agents-md.md): a section whose content digest and prompt identity match a
// section compiled at another path keeps its claims, re-cited to the new
// path and lines, with no model call. The carry is one-to-one: the digest
// must name exactly one departed subject and exactly one section of the
// current corpus, or nothing carries and the section compiles.
func TestCompileCarriesClaimsAcrossAMovedSection(t *testing.T) {
	ctx := context.Background()

	t.Run("a section moved CLAUDE.md to AGENTS.md keeps its claims with new cites and no model call", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		before := claimsBySubject(t, st)["CLAUDE.md#conventions"]
		if len(before) != 2 {
			t.Fatalf("setup: %d claims under CLAUDE.md#conventions, want 2", len(before))
		}

		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}))
		if mock.CallCount() != 0 {
			t.Fatalf("moved section made %d model calls, want 0", mock.CallCount())
		}
		if run.SectionsCarried != 1 || run.ClaimsCarried != 2 || run.SectionsCompiled != 0 || run.TokensIn != 0 {
			t.Errorf("carried sections/claims = %d/%d, compiled = %d, tokens_in = %d; want 1/2, 0, 0",
				run.SectionsCarried, run.ClaimsCarried, run.SectionsCompiled, run.TokensIn)
		}

		by := claimsBySubject(t, st)
		moved := by["AGENTS.md#conventions"]
		if len(moved) != 2 {
			t.Fatalf("%d claims under AGENTS.md#conventions, want 2", len(moved))
		}
		for _, cl := range by["CLAUDE.md#conventions"] {
			if cl.Status != core.ClaimSuperseded || cl.ValidTo.IsZero() {
				t.Errorf("departed claim %q status=%s valid_to=%v, want superseded with a closed window", cl.Rule, cl.Status, cl.ValidTo)
			}
		}
		rules := map[string]bool{}
		ids := make([]string, 0, len(moved))
		for _, cl := range moved {
			rules[cl.Rule] = true
			ids = append(ids, cl.ClaimID)
			if cl.Status != core.ClaimFresh || cl.PromptVersion != obligationsPromptVersion() || cl.Model != "mock-model" {
				t.Errorf("carried claim %q status/prompt/model = %s/%s/%s", cl.Rule, cl.Status, cl.PromptVersion, cl.Model)
			}
			if !strings.HasPrefix(cl.AssertedBy, "compile:") || cl.AssertedBy != core.AssertedBy("compile", run.RunID) {
				t.Errorf("carried claim %q asserted_by = %q, want this run", cl.Rule, cl.AssertedBy)
			}
		}
		for _, cl := range before {
			if !rules[cl.Rule] {
				t.Errorf("rule %q did not carry", cl.Rule)
			}
		}

		links, err := st.ClaimLinks(ctx, "svc", ids)
		if err != nil {
			t.Fatalf("ClaimLinks: %v", err)
		}
		var cites []string
		var supersedes int
		for _, l := range links {
			switch l.Rel {
			case core.RelCites:
				cites = append(cites, l.TargetRef)
				p, s, e, ok := core.ParseCiteRef(l.TargetRef)
				if !ok || p != "AGENTS.md" {
					t.Fatalf("carried cite %q does not name AGENTS.md", l.TargetRef)
				}
				h, _ := core.HashLines([]byte(movedGovDoc), s, e)
				text, _ := core.LinesText([]byte(movedGovDoc), s, e)
				if h != l.TargetHash || text != l.AnchorText {
					t.Errorf("carried cite %q does not hash-match the new document", l.TargetRef)
				}
			case core.RelSupersedes:
				supersedes++
			}
		}
		sort.Strings(cites)
		if want := []string{"AGENTS.md#L5-L5", "AGENTS.md#L6-L6"}; strings.Join(cites, ",") != strings.Join(want, ",") {
			t.Errorf("carried cites = %v, want %v", cites, want)
		}
		if supersedes != 2 {
			t.Errorf("supersedes links = %d, want one per carried claim naming its predecessor", supersedes)
		}

		// The carried claims pass the ordinary digest gate from now on.
		again, mock3 := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}))
		if mock3.CallCount() != 0 || again.SectionsCarried != 0 || again.SectionsCompiled != 0 {
			t.Errorf("recompile after carry: calls=%d carried=%d compiled=%d, want 0/0/0", mock3.CallCount(), again.SectionsCarried, again.SectionsCompiled)
		}
	})

	t.Run("a dry run counts the carry, spends nothing and writes nothing", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(true, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}))
		if mock.CallCount() != 0 || run.SectionsCarried != 1 || run.ClaimsCarried != 2 || run.SectionsCompiled != 0 || run.TokensIn != 0 {
			t.Errorf("dry run: calls=%d carried=%d/%d compiled=%d tokens_in=%d, want 0, 1/2, 0, 0",
				mock.CallCount(), run.SectionsCarried, run.ClaimsCarried, run.SectionsCompiled, run.TokensIn)
		}
		by := claimsBySubject(t, st)
		if len(by["AGENTS.md#conventions"]) != 0 {
			t.Errorf("dry run wrote %d carried claims", len(by["AGENTS.md#conventions"]))
		}
		for _, cl := range by["CLAUDE.md#conventions"] {
			if cl.Status != core.ClaimFresh {
				t.Errorf("dry run retired %q (status %s)", cl.Rule, cl.Status)
			}
		}
	})

	t.Run("a CLAUDE.md twin of AGENTS.md hands its claims to the AGENTS.md copy", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)},
			core.SourceFile{Path: "AGENTS.md", Content: []byte(govDoc)}))
		if mock.CallCount() != 0 || run.SectionsCarried != 1 {
			t.Errorf("twin: calls=%d carried=%d, want 0/1", mock.CallCount(), run.SectionsCarried)
		}
	})

	t.Run("a section moved and edited still needs a model call", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		edited := strings.Replace(movedGovDoc, "All outputs must be sorted.", "All outputs must be sorted by key.", 1)
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(edited)}), movedResponse)
		if mock.CallCount() != 1 || run.SectionsCompiled != 1 || run.SectionsCarried != 0 {
			t.Errorf("moved+edited: calls=%d compiled=%d carried=%d, want 1/1/0", mock.CallCount(), run.SectionsCompiled, run.SectionsCarried)
		}
	})

	t.Run("a move across a scope floor compiles", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "internal/AGENTS.md", Content: []byte(movedGovDoc)}),
			`{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":6,"cite_end":6}]}`)
		if mock.CallCount() != 1 || run.SectionsCarried != 0 {
			t.Errorf("root -> internal/: calls=%d carried=%d, want 1/0 (the floor changes what the claims may govern)", mock.CallCount(), run.SectionsCarried)
		}
	})

	// The one-to-one rule, current side: one departed subject, but two
	// sections of the current corpus carry its digest. Which copy inherits
	// is a guess, so neither does.
	t.Run("nothing carries when two current sections share the digest", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)},
			core.SourceFile{Path: "GEMINI.md", Content: []byte(movedGovDoc)}), movedResponse, movedResponse)
		if mock.CallCount() != 2 || run.SectionsCarried != 0 || run.SectionsCompiled != 2 {
			t.Errorf("two current copies: calls=%d carried=%d compiled=%d, want 2/0/2", mock.CallCount(), run.SectionsCarried, run.SectionsCompiled)
		}
	})

	// The one-to-one rule, departed side: two departed subjects compiled
	// the same digest (their claim sets need not agree), so the section
	// has no single predecessor and compiles.
	t.Run("nothing carries when two departed subjects share the digest", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)},
			core.SourceFile{Path: "docs/old.md", Content: []byte(govDoc)}), twoClaimResponse, twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}), movedResponse)
		if mock.CallCount() != 1 || run.SectionsCarried != 0 || run.SectionsCompiled != 1 {
			t.Errorf("two departed subjects: calls=%d carried=%d compiled=%d, want 1/0/1", mock.CallCount(), run.SectionsCarried, run.SectionsCompiled)
		}
	})

	t.Run("a prompt change defeats the carry like it defeats the digest gate", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		all, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
		for _, cl := range all {
			cl.PromptVersion = "obligations/older"
			if err := st.PutClaims(ctx, []core.SemanticClaim{cl}, nil); err != nil {
				t.Fatalf("PutClaims: %v", err)
			}
		}
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}), movedResponse)
		if mock.CallCount() != 1 || run.SectionsCarried != 0 {
			t.Errorf("older prompt: calls=%d carried=%d, want 1/0", mock.CallCount(), run.SectionsCarried)
		}
	})

	// Rule 1: a section whose own subject already holds claims from its
	// digest keeps them; a departed copy with the same digest hands it
	// nothing (its claims simply retire with their subject). docs/old.md,
	// not CLAUDE.md: a CLAUDE.md twin of AGENTS.md never enters the corpus.
	t.Run("a section that holds its own claims takes none from a departed copy", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "AGENTS.md", Content: []byte(govDoc)},
			core.SourceFile{Path: "docs/old.md", Content: []byte(govDoc)}), twoClaimResponse, twoClaimResponse)
		by := claimsBySubject(t, st)
		own := map[string]bool{}
		for _, cl := range by["AGENTS.md#conventions"] {
			own[cl.ClaimID] = true
		}
		if len(own) != 2 || len(by["docs/old.md#conventions"]) != 2 {
			t.Fatalf("setup: %d claims under AGENTS.md, %d under docs/old.md, want 2 and 2", len(own), len(by["docs/old.md#conventions"]))
		}
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(govDoc)}))
		if mock.CallCount() != 0 || run.SectionsCarried != 0 || run.SectionsCompiled != 0 {
			t.Errorf("copy removed: calls=%d carried=%d compiled=%d, want 0/0/0", mock.CallCount(), run.SectionsCarried, run.SectionsCompiled)
		}
		for _, cl := range claimsBySubject(t, st)["AGENTS.md#conventions"] {
			if !own[cl.ClaimID] || cl.Status != core.ClaimFresh {
				t.Errorf("AGENTS.md claim %q (id %s, status %s): want only its own fresh claims", cl.Rule, cl.ClaimID, cl.Status)
			}
		}
	})

	// A carried claim whose id the subject already held (a subject left
	// with a mix of digests, so rule 1 lets it carry) is re-emitted, not
	// retired by the supersede sweep that retires the mix's stale claim.
	t.Run("a carried claim the subject already held stays fresh", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "AGENTS.md", Content: []byte(govDoc)},
			core.SourceFile{Path: "docs/old.md", Content: []byte(govDoc)}), twoClaimResponse, twoClaimResponse)
		own := claimsBySubject(t, st)["AGENTS.md#conventions"]
		if len(own) != 2 {
			t.Fatalf("setup: %d claims under AGENTS.md, want 2", len(own))
		}
		stale := own[0]
		stale.Rule = "A rule from an older digest."
		stale.InputDigest = "older-digest"
		stale.ReproKey += "-older"
		stale.ClaimID = core.ClaimID(stale.Layer, stale.Kind, stale.Subject, stale.ReproKey)
		if err := st.PutClaims(ctx, []core.SemanticClaim{stale}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(govDoc)}))
		if mock.CallCount() != 0 || run.SectionsCarried != 1 {
			t.Fatalf("setup: calls=%d carried=%d, want 0/1", mock.CallCount(), run.SectionsCarried)
		}
		fresh := map[string]bool{}
		for _, cl := range claimsBySubject(t, st)["AGENTS.md#conventions"] {
			if cl.Status == core.ClaimFresh {
				fresh[cl.ClaimID] = true
			}
		}
		if fresh[stale.ClaimID] || !fresh[own[0].ClaimID] || !fresh[own[1].ClaimID] || len(fresh) != 2 {
			t.Errorf("fresh claims %v: want exactly the two carried ids %s and %s, the older-digest claim retired", fresh, own[0].ClaimID, own[1].ClaimID)
		}
	})

	// A carried subject is retired like a recompiled one: its own claims
	// from an older digest do not stay live beside the carried set.
	t.Run("a carried subject's claims from an older digest retire", func(t *testing.T) {
		st := store.NewMemoryStore()
		oldDoc := "# Conventions\n\n- Use tabs for indentation.\n"
		oldResponse := `{"claims":[{"rule":"Use tabs for indentation.","scope":["internal/query/**"],"cite_start":3,"cite_end":3}]}`
		compileWith(t, st, carryInput(false,
			core.SourceFile{Path: "AGENTS.md", Content: []byte(oldDoc)},
			core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), oldResponse, twoClaimResponse)
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}))
		if mock.CallCount() != 0 || run.SectionsCarried != 1 {
			t.Fatalf("setup: calls=%d carried=%d, want 0/1", mock.CallCount(), run.SectionsCarried)
		}
		fresh := 0
		for _, cl := range claimsBySubject(t, st)["AGENTS.md#conventions"] {
			if cl.Rule == "Use tabs for indentation." && cl.Status == core.ClaimFresh {
				t.Errorf("the older-digest claim %q is still fresh after the carry", cl.Rule)
			}
			if cl.Status == core.ClaimFresh {
				fresh++
			}
		}
		if fresh != 2 {
			t.Errorf("%d fresh claims under AGENTS.md#conventions, want the 2 carried", fresh)
		}
	})

	// Rule 4, scope side: a rules file that declares its own scope (a Cursor
	// .mdc rule, whatever its frontmatter) recorded claims under that
	// declaration, so neither a move out of one nor into one carries.
	t.Run("a move out of or into a rules file that declares its scope compiles", func(t *testing.T) {
		for _, tc := range []struct{ from, to string }{
			{".cursor/rules/conventions.mdc", "AGENTS.md"},
			{"CLAUDE.md", ".cursor/rules/conventions.mdc"},
		} {
			st := store.NewMemoryStore()
			compileWith(t, st, carryInput(false, core.SourceFile{Path: tc.from, Content: []byte(govDoc)}), twoClaimResponse)
			if n := len(claimsBySubject(t, st)[tc.from+"#conventions"]); n != 2 {
				t.Fatalf("%s setup: %d claims, want 2", tc.from, n)
			}
			run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: tc.to, Content: []byte(movedGovDoc)}), movedResponse)
			if mock.CallCount() != 1 || run.SectionsCarried != 0 {
				t.Errorf("%s -> %s: calls=%d carried=%d, want 1/0", tc.from, tc.to, mock.CallCount(), run.SectionsCarried)
			}
		}
	})

	// Rule 5, ambiguity: the cited line appears twice in the section, so
	// two shifts re-anchor it and the cites cannot say which. It compiles.
	t.Run("a cite that lands on a repeated line compiles", func(t *testing.T) {
		st := store.NewMemoryStore()
		repeated := "# Conventions\n\n- All outputs must be sorted.\n- All outputs must be sorted.\n"
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(repeated)}),
			`{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":3,"cite_end":3}]}`)
		if n := len(claimsBySubject(t, st)["CLAUDE.md#conventions"]); n != 1 {
			t.Fatalf("setup: %d claims, want 1", n)
		}
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte("\n\n" + repeated)}),
			`{"claims":[{"rule":"Sort all outputs.","scope":["internal/mcp/**"],"cite_start":5,"cite_end":5}]}`)
		if mock.CallCount() != 1 || run.SectionsCarried != 0 {
			t.Errorf("ambiguous shift: calls=%d carried=%d, want 1/0", mock.CallCount(), run.SectionsCarried)
		}
	})

	// Rule 5, coverage: a departed claim with no citation has nothing to
	// re-cite, so the section compiles rather than carry it uncited.
	t.Run("a departed claim with no citation compiles", func(t *testing.T) {
		st := store.NewMemoryStore()
		compileWith(t, st, carryInput(false, core.SourceFile{Path: "CLAUDE.md", Content: []byte(govDoc)}), twoClaimResponse)
		before := claimsBySubject(t, st)["CLAUDE.md#conventions"]
		if len(before) != 2 {
			t.Fatalf("setup: %d claims, want 2", len(before))
		}
		uncited := before[0]
		uncited.Rule = "An uncited rule."
		uncited.ReproKey += "-uncited"
		uncited.ClaimID = core.ClaimID(uncited.Layer, uncited.Kind, uncited.Subject, uncited.ReproKey)
		if err := st.PutClaims(ctx, []core.SemanticClaim{uncited}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		run, mock := compileWith(t, st, carryInput(false, core.SourceFile{Path: "AGENTS.md", Content: []byte(movedGovDoc)}), movedResponse)
		if mock.CallCount() != 1 || run.SectionsCarried != 0 {
			t.Errorf("uncited departed claim: calls=%d carried=%d, want 1/0", mock.CallCount(), run.SectionsCarried)
		}
	})
}

// TestNeedsCompileHonorsCarry pins the lifted digest gate: a section needs
// the model when its own subject has no claims compiled from its digest
// under this prompt, unless AG-26 carried claims to it.
func TestNeedsCompileHonorsCarry(t *testing.T) {
	sec := Section{Subject: "AGENTS.md#x", Digest: "d1"}
	prior := map[string][]core.SemanticClaim{"AGENTS.md#x": {{InputDigest: "d1", PromptVersion: "p"}}}
	if needsCompile(prior, nil, sec, "p") {
		t.Error("matching own claims: needsCompile = true, want false")
	}
	if !needsCompile(prior, nil, sec, "q") {
		t.Error("prompt changed: needsCompile = false, want true")
	}
	if !needsCompile(nil, nil, sec, "p") {
		t.Error("no claims: needsCompile = false, want true")
	}
	if needsCompile(nil, map[string]carriedSection{"AGENTS.md#x": {}}, sec, "p") {
		t.Error("carried: needsCompile = true, want false")
	}
}
