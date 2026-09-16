package scenariobench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// lookupsVSPath is testdata/scenariobench/lookups-vs.json (SB-32), resolved
// relative to the repo root rather than this package directory since the
// corpus is committed under the repo-wide testdata/scenariobench/, not
// internal/scenariobench/testdata/.
func lookupsVSPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "scenariobench", "lookups-vs.json")
}

// TestLookupsVSCorpus_ParsesAgainstContract proves the SB-32 corpus is valid
// data against the Corpus/Task/Arm contract this file must not edit
// (types.go) — the anchor a future LoadScenarioCorpus (SB-10, wave 2) will
// build on.
func TestLookupsVSCorpus_ParsesAgainstContract(t *testing.T) {
	data, err := os.ReadFile(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("read lookups-vs.json: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal lookups-vs.json against Corpus: %v", err)
	}

	wantArms := []string{"full", "graphify", "serena", "gopls", "grep"}
	for _, a := range wantArms {
		if _, ok := c.Arms[a]; !ok {
			t.Errorf("arm %q missing", a)
		}
	}
	if len(c.Arms) != len(wantArms) {
		t.Errorf("got %d arms, want %d: %v", len(c.Arms), len(wantArms), c.Arms)
	}

	full := c.Arms["full"]
	if !full.Wired || full.Instructions != "stanza" || full.Surface != "fathom" {
		t.Errorf("full arm = %+v, want wired=true instructions=stanza surface=fathom", full)
	}
	// RC-8: grep has no MCP tool to route to and stays "plain"; the three
	// MCP-backed competitors each carry their own routing stanza.
	if c.Arms["grep"].Instructions != "plain" {
		t.Errorf("grep arm instructions = %q, want plain", c.Arms["grep"].Instructions)
	}
	for _, name := range []string{"graphify", "serena", "gopls"} {
		arm := c.Arms[name]
		if arm.Instructions != "text" {
			t.Errorf("arm %q instructions = %q, want text", name, arm.Instructions)
		}
		if arm.InstructionsText == "" {
			t.Errorf("arm %q InstructionsText is empty, want a routing stanza", name)
		}
	}
	for _, name := range []string{"graphify", "serena", "gopls", "grep"} {
		if c.Arms[name].Wired {
			t.Errorf("arm %q must not be wired", name)
		}
	}
	for _, name := range wantArms {
		got := c.Arms[name].AllowedTools
		if len(got) != 3 {
			t.Errorf("arm %q allowed_tools = %v, want [Read Grep Bash]", name, got)
		}
	}
	if c.Arms["graphify"].Surface != "graphify@v0.9.61" {
		t.Errorf("graphify surface = %q, want graphify@v0.9.61", c.Arms["graphify"].Surface)
	}
	if c.Arms["serena"].Surface != "serena@v1.7.0" {
		t.Errorf("serena surface = %q, want serena@v1.7.0", c.Arms["serena"].Surface)
	}
	if c.Arms["gopls"].Surface != "gopls@v0.20+" {
		t.Errorf("gopls surface = %q, want gopls@v0.20+", c.Arms["gopls"].Surface)
	}
	if c.Arms["grep"].Surface != "" {
		t.Errorf("grep surface = %q, want empty", c.Arms["grep"].Surface)
	}
	if len(c.Arms["graphify"].MCP) == 0 || len(c.Arms["serena"].MCP) == 0 || len(c.Arms["gopls"].MCP) == 0 {
		t.Error("graphify/serena/gopls arms must carry an mcp block")
	}
	if len(c.Arms["grep"].MCP) != 0 {
		t.Error("grep arm must carry no mcp block")
	}

	// RC-9: graphify's server command must be the one graphify.serve
	// actually starts under uv — `graphify --mcp` is not a real command
	// (vision 28 F2: failed 12/12 sessions in the second reading).
	graphifyMCP, _ := c.Arms["graphify"].MCP["graphify"].(map[string]any)
	args, _ := graphifyMCP["args"].([]any)
	var argsJoined string
	for _, a := range args {
		s, _ := a.(string)
		argsJoined += s + " "
	}
	if graphifyMCP["command"] != "uv" || !strings.Contains(argsJoined, "graphify.serve") {
		t.Errorf("graphify mcp block = %+v, want uv run ... -m graphify.serve <graph.json>", c.Arms["graphify"].MCP["graphify"])
	}

	wantIDs := []string{
		"H1-T01", "H1-T02", "H1-T03", "H1-T04", "H1-T05", "H1-T06", "H1-T07",
		"H1-T08", "H1-T09", "H1-T10", "H1-T11", "H1-T12", "H1-T13", "FRESH-M7-01",
	}
	if len(c.Tasks) != len(wantIDs) {
		t.Fatalf("got %d tasks, want %d", len(c.Tasks), len(wantIDs))
	}
	byID := map[string]Task{}
	for _, task := range c.Tasks {
		byID[task.ID] = task
	}
	for _, id := range wantIDs {
		task, ok := byID[id]
		if !ok {
			t.Errorf("task %q missing", id)
			continue
		}
		if task.Prompt == "" {
			t.Errorf("task %q has no prompt", id)
		}
		if task.Derivation == "" {
			t.Errorf("task %q has no derivation", id)
		}
		for _, arm := range wantArms {
			if task.Predictions[arm] == "" {
				t.Errorf("task %q missing a prediction for arm %q", id, arm)
			}
		}
		// O53: every task derived from the home corpus excludes the
		// corpus's own fixtures/docs, so no arm's session can read its way
		// to the gold answer instead of answering the prompt. Enforcement
		// is wave-n1's loader rule; this pins the DATA half today.
		wantExclude := []string{
			"testdata/contextbench/**",
			"testdata/scenariobench/**",
			"docs/bench/**",
		}
		if got := task.Subject.ExcludePaths; !reflect.DeepEqual(got, wantExclude) {
			t.Errorf("task %q subject.exclude_paths = %v, want %v", id, got, wantExclude)
		}
		// The 20-turn cap ended six competitor cells with the answer
		// already in hand but no turns left to state it — raised to 30 for
		// every task in this corpus (docs/bench/scenarios.md).
		if task.Sessions.MaxTurns != 30 {
			t.Errorf("task %q sessions.max_turns = %d, want 30", id, task.Sessions.MaxTurns)
		}
	}

	fresh, ok := byID["FRESH-M7-01"]
	if !ok {
		t.Fatal("FRESH-M7-01 missing")
	}
	if fresh.Category != CategoryFreshness {
		t.Errorf("FRESH-M7-01 category = %q, want %q", fresh.Category, CategoryFreshness)
	}
	// O45 item 4: the drift belongs in post_wire_setup (after the daemon's
	// first ingest), never subject.setup (before it) — moving it here is
	// exactly the fix, so subject.setup itself must now be EMPTY for this
	// task.
	if len(fresh.Subject.PostWireSetup) == 0 {
		t.Error("FRESH-M7-01 must carry a subject.post_wire_setup step that edits a file")
	}
	if len(fresh.Subject.Setup) != 0 {
		t.Errorf("FRESH-M7-01 must carry no subject.setup (the drift belongs in post_wire_setup), got %v", fresh.Subject.Setup)
	}

	if c.Criterion.ID != "M6-1" || c.Criterion.Treatment != "full" || c.Criterion.Control != "grep" {
		t.Errorf("criterion = %+v, want id=M6-1 treatment=full control=grep", c.Criterion)
	}
	if c.Criterion.MinTrials != 2 || !c.Criterion.RequireSuccessAtLeastEqual || c.Criterion.MaxTokensPerSuccessRatio != 1.0 {
		t.Errorf("criterion bar = %+v, want min_trials=2 require_success_at_least_equal=true max_tokens_per_success_ratio=1.0", c.Criterion)
	}
	if c.Criterion.Registered != "2026-09-13" {
		t.Errorf("criterion registered = %q, want 2026-09-13", c.Criterion.Registered)
	}

	if len(c.Sources) != 3 {
		t.Fatalf("got %d sources, want 3 (graphify, serena, gopls)", len(c.Sources))
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		seen[s.Name] = true
		if s.URL == "" || s.Revision == "" || s.License == "" {
			t.Errorf("source %+v missing a field", s)
		}
	}
	for _, name := range []string{"graphify", "serena", "gopls"} {
		if !seen[name] {
			t.Errorf("source pin %q missing", name)
		}
	}

	gold, ok := c.Notes["gold"].(map[string]any)
	if !ok {
		t.Fatal("notes.gold missing or not an object")
	}
	for _, id := range wantIDs {
		if _, ok := gold[id]; !ok {
			t.Errorf("notes.gold missing an entry for %q", id)
		}
	}
}

// TestLookupsVSCorpus_LoadScenarioCorpusAccepts proves LoadScenarioCorpus
// (SB-10's gate) no longer refuses lookups-vs.json's answer-only tasks:
// before oracle.answer_all_of existed, every task's oracle was `{}` and
// hasOracle refused it ("no oracle ... nothing would decide success").
func TestLookupsVSCorpus_LoadScenarioCorpusAccepts(t *testing.T) {
	c, err := LoadScenarioCorpus(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	for _, task := range c.Tasks {
		if len(task.Oracle.AnswerAllOf) == 0 {
			t.Errorf("task %q: Oracle.AnswerAllOf is empty", task.ID)
		}
		if !AnswerOnly(task) {
			t.Errorf("task %q: want AnswerOnly(task)=true", task.ID)
		}
	}
}

// TestLookupsVSCorpus_ContainsGraphifyLiteral pins the same lexical fact
// fathom verify's oracle checks for this file's plan row (SB-32: pattern
// "\"graphify\"").
func TestLookupsVSCorpus_ContainsGraphifyLiteral(t *testing.T) {
	data, err := os.ReadFile(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("read lookups-vs.json: %v", err)
	}
	if !bytes.Contains(data, []byte(`"graphify"`)) {
		t.Error(`lookups-vs.json must contain the literal string "graphify"`)
	}
}

// fathomStanzaRoutingSectionLen mirrors the byte length of cmd/fathom/init.go's
// own routing section: fathomStanzaTemplate's paragraph ("This repo is
// indexed by fathom...") through "Read files to *edit* them, not to discover
// them.", a blank line, then openingMovesTable — the portion BEFORE the
// "Refresh the index..." paragraph, which has no competitor equivalent
// (ingest, obligations hooks, the verify gate). internal/scenariobench
// cannot import cmd/fathom (package main), so this is pinned by length
// alone; recompute it if that template changes.
const fathomStanzaRoutingSectionLen = 1052

// stanzaRoutingSection strips a competitor arm's "## <tool> ... navigation"
// header (the first line, plus the blank line after it) from its
// instructions_text, leaving the same "routing section" shape
// fathomStanzaRoutingSectionLen measures for fathom's own stanza.
func stanzaRoutingSection(t *testing.T, instructionsText string) string {
	t.Helper()
	idx := strings.Index(instructionsText, "\n\n")
	if idx < 0 {
		t.Fatalf("instructions_text has no header/body separator: %q", instructionsText)
	}
	return instructionsText[idx+2:]
}

// withinFifteenPercent reports whether a is within +/-15% of b (SB-32's own
// parity bar, docs/bench/scenarios.md).
func withinFifteenPercent(a, b int) bool {
	lo, hi := float64(b)*0.85, float64(b)*1.15
	return float64(a) >= lo && float64(a) <= hi
}

// stanzaIntroParagraph returns a competitor's own opening capability claim —
// the first paragraph after the "## <tool> ... navigation" header, before
// the "Reach for these..." sentence and the "Opening moves" list — the one
// place review fix-first (2026-09-14) says an overclaim must not appear.
func stanzaIntroParagraph(t *testing.T, instructionsText string) string {
	t.Helper()
	section := stanzaRoutingSection(t, instructionsText)
	idx := strings.Index(section, "\n\n")
	if idx < 0 {
		t.Fatalf("instructions_text routing section has no intro/body separator: %q", section)
	}
	return section[:idx]
}

// TestLookupsVSCorpus_CompetitorStanzaParity is O53's own proof: a second
// reading of pilot vs-pilot-3 found gopls making 0 MCP calls in 28 trials,
// graphify 1, serena 1, because every competitor arm's own instructions_text
// named its tools and then added a disclaimer ("has no view onto documented
// conventions ... that's grep's job") that routed the model away from the
// tool before it ever read the prompt. This pins the fix: one shared
// template, instantiated per competitor, with (a) no disclaimer phrasing,
// (b) no overclaim in the intro sentence (a competitor's real capability is
// where-defined/who-calls/what-breaks only — never governance, provenance,
// or freshness, which fathom's own stanza routes to different tools
// entirely), (c) grep named second on every opening move whose category
// falls outside that real capability, (d) all eight of the corpus's own
// task-shape categories covered, and (e) length parity within +/-15% of
// the other two competitors and of fathom's own stanza's routing section.
// Review fix-first (2026-09-14) found the FIRST version of this test still
// passing an intro sentence that claimed "what governs its conventions"
// and "what changed since a prior citation" — capabilities none of these
// three tools has — and a dead, never-matching banned phrase
// ("not <tool>'s job", literally containing the placeholder). Both are
// fixed here.
func TestLookupsVSCorpus_CompetitorStanzaParity(t *testing.T) {
	c, err := LoadScenarioCorpus(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}

	competitors := []string{"graphify", "serena", "gopls"}
	// blastRadius is the one category a competitor's real capability
	// (who-calls/what-breaks) actually covers, so its line names the
	// closest chained query alone — no "grep second" hedge required.
	const blastRadius = "blast_radius"
	wantCategories := []string{
		"obligations", "claim_provenance", "concern_search", "contradiction",
		"freshness", "staleness", "verify", blastRadius,
	}
	introBannedPhrases := []string{
		"what governs", "what changed since", "governs its conventions",
		"decision records",
	}
	textBannedPhrases := []string{
		"no view onto", "has no view", "not its job", "is grep's job, not",
		"those are not its job",
	}

	lens := map[string]int{}
	for _, name := range competitors {
		arm, ok := c.Arms[name]
		if !ok || arm.InstructionsText == "" {
			t.Fatalf("arm %q missing instructions_text", name)
		}
		text := arm.InstructionsText
		lower := strings.ToLower(text)
		for _, banned := range textBannedPhrases {
			if strings.Contains(lower, banned) {
				t.Errorf("arm %q instructions_text contains disclaimer phrase %q, routing away from its own tool", name, banned)
			}
		}
		// The dead placeholder "not <tool>'s job" never matched real text;
		// substitute this arm's own name so the check can actually fire.
		if perTool := "not " + strings.ToLower(name) + "'s job"; strings.Contains(lower, perTool) {
			t.Errorf("arm %q instructions_text contains disclaimer phrase %q, routing away from its own tool", name, perTool)
		}
		intro := strings.ToLower(stanzaIntroParagraph(t, text))
		for _, banned := range introBannedPhrases {
			if strings.Contains(intro, banned) {
				t.Errorf("arm %q intro paragraph contains overclaim phrase %q — this tool has no documented-governance or citation-freshness capability", name, banned)
			}
		}
		for _, cat := range wantCategories {
			line := "- " + cat + ":"
			if !strings.Contains(text, line) {
				t.Errorf("arm %q instructions_text missing an opening move for category %q", name, cat)
				continue
			}
			for _, raw := range strings.Split(text, "\n") {
				if !strings.HasPrefix(raw, line) {
					continue
				}
				// Every category but blast_radius is hedged: the tool's
				// closest query is named first, but grep still appears in
				// the same line as the (non-asserted) fallback.
				if cat != blastRadius && !strings.Contains(raw, "grep") {
					t.Errorf("arm %q category %q line %q does not name grep as a fallback", name, cat, raw)
				}
			}
		}
		lens[name] = len(stanzaRoutingSection(t, text))
	}

	for _, name := range competitors {
		if !withinFifteenPercent(lens[name], fathomStanzaRoutingSectionLen) {
			t.Errorf("arm %q routing section = %d bytes, want within +/-15%% of fathom's %d bytes", name, lens[name], fathomStanzaRoutingSectionLen)
		}
		for _, other := range competitors {
			if other == name {
				continue
			}
			if !withinFifteenPercent(lens[name], lens[other]) {
				t.Errorf("arm %q routing section = %d bytes, not within +/-15%% of arm %q's %d bytes", name, lens[name], other, lens[other])
			}
		}
	}
}

// mechanismNamingPatterns are phrases that frame a lookup prompt in terms of
// fathom's OWN mechanism — a tool/verb/artifact only the wired "full" arm can
// reach — as the MEANS to answer, rather than naming the underlying question.
// O62 (2026-09-16): vs-pilot-4 aborted at 31/140 cells because 12 of the 14
// lookups-vs.json prompts did exactly this ("Per fathom's compiled
// obligations layer, which repository convention governs …", "evaluate it
// fresh against the current index", "Fathom's compiled obligations layer
// serves a rule …"), so every competitor arm (grep/gopls/graphify/serena)
// hunted for a layer/index/gate it had no tool for — H1-T04's serena-arm
// agent went as far as building fathom from source and running `fathom
// verify` (cell LOST by the baseline_ran_fathom detector). "the fathom repo"
// as a LOCATION is fine ("In the fathom repo, ..."); a fathom CLI verb named
// as the PRODUCT under test is fine too ("Add a `fathom debug-dump` CLI
// verb" — that names what's being built, not how to look up the answer).
// These patterns catch the MEANS framing only.
var mechanismNamingPatterns = []*regexp.Regexp{
	// "Per fathom's compiled obligations layer, ..." / "Fathom's compiled
	// obligations layer serves a rule ..." (H1-T01/T02/T07/T12/T13).
	regexp.MustCompile(`(?i)compiled obligations layer`),
	// "FAILS fathom's verify gate" / "fathom's gate gives" (H1-T04).
	regexp.MustCompile(`(?i)fathom's (verify )?gate`),
	// "evaluate it fresh against the current index" / "against the current
	// index" (H1-T04).
	regexp.MustCompile(`(?i)\bthe current index\b`),
	// "Concern-scoped search: ..." used as a prompt-opening directive
	// (H1-T09/T10) rather than the category label it also happens to share.
	regexp.MustCompile(`(?i)concern-scoped search`),
	// "state the claim's serving status" (H1-T12/T13) — fathom's own
	// staleness vocabulary, not a phrase any competitor tool emits.
	regexp.MustCompile(`(?i)claim'?s serving status`),
	// Defensive: fathom's own MCP tool names/verbs, not currently present in
	// any prompt but exactly the kind of phrase this defect reintroduces.
	regexp.MustCompile(`(?i)staleness_report`),
	regexp.MustCompile(`(?i)verify_plan`),
	regexp.MustCompile(`(?i)obligations tool`),
	regexp.MustCompile(`(?i)\bfathom's (verify|obligations|index)\b`),
}

// TestLookupsVSCorpus_PromptsAreToolNeutral is O62's regression pin: every
// prompt in lookups-vs.json must ask the QUESTION, never name fathom's own
// mechanism as the way to answer it (see mechanismNamingPatterns' doc for
// the full defect). Written against the pre-fix corpus this test failed on
// H1-T01, T02, T04, T07, T09, T10, T12, T13 — proving the defect — before
// the corpus was rewritten to fix it.
func TestLookupsVSCorpus_PromptsAreToolNeutral(t *testing.T) {
	c, err := LoadScenarioCorpus(lookupsVSPath(t))
	if err != nil {
		t.Fatalf("LoadScenarioCorpus: %v", err)
	}
	for _, task := range c.Tasks {
		for _, pat := range mechanismNamingPatterns {
			if pat.MatchString(task.Prompt) {
				t.Errorf("task %q prompt names fathom's own mechanism via pattern %s (a competitor arm has no tool to reach it): %q", task.ID, pat.String(), task.Prompt)
			}
		}
	}
}
