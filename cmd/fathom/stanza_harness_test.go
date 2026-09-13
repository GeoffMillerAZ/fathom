package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// preSeamStanzaTemplate is cmd/fathom/init.go's fathomStanzaTemplate as
// it stood before the harness seam, with the plain obligations paragraph
// in place — the Claude Code stanza golden every wired repository carries
// (HP-11: the Claude Code stanza stays byte-identical).
const preSeamStanzaTemplate = "## fathom code intelligence\n" +
	"\n" +
	"This repo is indexed by fathom as service \"svc\". Route by\n" +
	"question shape (SB-7, docs/vision/27-strengths-bench.md §8 — the home\n" +
	"bench found \"index before any grep\" over-broad): where / who-calls /\n" +
	"what-breaks resolve via the fathom MCP tools (`find_symbol`,\n" +
	"`references`, `symbol_context`, `outline`, `repo_map`, `pack_context`,\n" +
	"`verify_plan`); what-governs this repository is `obligations` with no\n" +
	"path (the root conventions); is this still true / what was known is\n" +
	"`staleness_report` (RC-23, docs/vision/28-receipts.md §3.5). A literal\n" +
	"string, a config value, a log line, or a document-versus-document\n" +
	"contradiction is grep's job, not the index's.\n" +
	"Read files to *edit* them, not to discover them.\n" +
	"\n" +
	"Opening moves by task shape (first 2-3 calls, then edit):\n" +
	"- debug/trace: find_symbol -> find_callers/references -> symbol_context\n" +
	"- feature/change: repo_map -> pack_context(concern) -> references\n" +
	"- review/verify: verify_plan -> obligations -> references on changed symbols\n" +
	"- refactor: references -> outline -> symbol_context\n" +
	"\n" +
	"Refresh the index via the `ingest` MCP tool (`{\"service\":\"svc\"}`) at\n" +
	"session start and after each commit; it is hash-gated, so a clean repo\n" +
	"refreshes in well under a second.\n" +
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
