package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// preSeamStanzaTemplate is cmd/fathom/init.go's fathomStanzaTemplate as
// it stood before the harness seam, with the plain obligations paragraph
// in place — the Claude Code stanza golden every wired repository carries
// (HP-11: the Claude Code stanza stays byte-identical ACROSS RENDERING
// PATHS — this golden and fathomStanzaTemplate move together on any
// deliberate content change, most recently ADR-035 decision 1's
// verb-first opening moves, docs/vision/29-verbs-not-lookups.md §3 wave A).
const preSeamStanzaTemplate = "## fathom code intelligence\n" +
	"\n" +
	"This repo is indexed by fathom as service \"svc\". Route by\n" +
	"question shape (SB-7, docs/vision/27-strengths-bench.md §8 — the home\n" +
	"bench found \"index before any grep\" over-broad), on this session's\n" +
	"default eight-tool MCP registration (surface reckoning F-86): where a\n" +
	"symbol is defined is `find_symbol`; who-calls it, briefly (definition,\n" +
	"a capped caller/callee list, tests) is `symbol_context`; what-breaks\n" +
	"is `blast_radius`; a composed review verdict over a diff is\n" +
	"`review_diff`; what-governs this repository is `obligations` with no\n" +
	"path (the root conventions); is a plan's box proven is `verify_plan`;\n" +
	"the current text at a cited line is `read_source`. A literal string, a\n" +
	"config value, a log line, or a document-versus-document contradiction\n" +
	"is `search`'s job (grep with a receipt, RC-24), not the other tools'.\n" +
	"Read files to *edit* them, not to discover them. The wider verb set\n" +
	"— the full resolved caller graph, staleness and what-was-known, and\n" +
	"more — lives one hop away at `/rpc/extended` (or its `/rpc/companion`\n" +
	"alias); register that surface instead of `/rpc/agent` for a session\n" +
	"that needs it.\n" +
	"\n" +
	"Opening moves by task shape (first 2-3 calls, then edit):\n" +
	"- bug report / stack trace / failing test: search -> symbol_context -> read_source\n" +
	"- feature/change: search -> obligations -> symbol_context\n" +
	"- review/verify: review_diff -> obligations -> verify_plan\n" +
	"- refactor/delete: blast_radius -> review_diff -> verify_plan\n" +
	"\n" +
	"The index refreshes itself: a read of a drifted service answers from\n" +
	"what it has and queues a re-ingest in the background (RT-99) —\n" +
	"nothing to call.\n" +
	"\n" +
	"Obligations — governing rules for the file you are about to touch — are\n" +
	"injected automatically on every edit via the PostToolUse hook. Treat them\n" +
	"as binding, not advisory.\n" +
	"\n" +
	"Continuity: run `fathom prime` at session start if your harness has no\n" +
	"SessionStart hook (Codex does not) — it prints the same identity,\n" +
	"freshness, live-example, thread-brief, and custodian primer this hook\n" +
	"injects on Claude Code. File what the next session must know:\n" +
	"`fathom thread note \"…\"`.\n" +
	"\n" +
	"Done means proven: a plan task is only \"done\" when the index proves it, not\n" +
	"when it is claimed. Before checking off a `docs/plans/*.md` item, run:\n" +
	"\n" +
	"    fathom verify --service svc --plan docs/plans/<plan>.md\n" +
	"\n" +
	"Exit 0 = every claimed task verifies. Exit 1 = a claimed task is unproven —\n" +
	"fix the code or uncheck the box. Exit 2 = operational (daemon down / index\n" +
	"stale) — fail-open, not a verification failure."

func TestStanzaHookSentencesClaudeByteIdentical(t *testing.T) {
	if got := fathomStanzaBody("svc", false); got != preSeamStanzaTemplate {
		t.Fatalf("the Claude Code stanza changed bytes — the seam kill (vision 25 §5):\n--- got ---\n%s\n--- want ---\n%s", got, preSeamStanzaTemplate)
	}
	if fathomStanzaBodyFor(harness.ClaudeCode, "svc", false) != fathomStanzaBody("svc", false) {
		t.Error("fathomStanzaBodyFor(claude-code) must be fathomStanzaBody")
	}
	if fathomStanzaBodyFor("", "svc", true) != fathomStanzaBody("svc", true) {
		t.Error("an empty id must render the Claude Code stanza")
	}
	o, c := stanzaHookSentences(harness.ClaudeCode, true)
	if o != obligationsWithIntent || c != continuityClaude {
		t.Error("claude-code with intent must render the pre-seam literals")
	}
}

// TestStanzaHookSentencesNeverPromiseAnAbsentChannel pins HP-11's rule:
// a harness whose hooks are not wired gets a stanza that names the pull
// tier and the harness, and never a hook it lacks.
func TestStanzaHookSentencesNeverPromiseAnAbsentChannel(t *testing.T) {
	for _, id := range []string{harness.Codex, harness.GeminiCLI, harness.Agents} {
		d, _ := harness.Lookup(id)
		for _, intentOn := range []bool{false, true} {
			body := fathomStanzaBodyFor(id, "svc", intentOn)
			for _, forbidden := range []string{"PostToolUse hook", "(Codex does not)", "injects on Claude Code"} {
				if strings.Contains(body, forbidden) {
					t.Errorf("%s (intent=%v) promises a channel it lacks: %q", id, intentOn, forbidden)
				}
			}
			for _, want := range []string{d.Display, "`obligations` MCP tool", "fathom prime", "Treat them as binding, not advisory."} {
				if !strings.Contains(body, want) {
					t.Errorf("%s (intent=%v) lacks %q:\n%s", id, intentOn, want, body)
				}
			}
			if intentOn && !strings.Contains(body, "docs/intent/") {
				t.Errorf("%s with intent must keep the intent wording", id)
			}
			if strings.Contains(body, "{{") {
				t.Errorf("%s: unexpanded placeholder:\n%s", id, body)
			}
		}
	}
	// Both wordings share the one template: everything outside the two
	// hook paragraphs is identical between Claude Code and Codex.
	claude := strings.Split(fathomStanzaBody("svc", false), "\n\n")
	codex := strings.Split(fathomStanzaBodyFor(harness.Codex, "svc", false), "\n\n")
	if len(claude) != len(codex) {
		t.Fatalf("paragraph counts differ: %d vs %d", len(claude), len(codex))
	}
	differ := 0
	for i := range claude {
		if claude[i] != codex[i] {
			differ++
		}
	}
	if differ != 2 {
		t.Errorf("exactly the two hook paragraphs may differ, got %d differing paragraphs", differ)
	}
}
