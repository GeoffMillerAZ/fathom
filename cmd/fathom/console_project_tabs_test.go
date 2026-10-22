package main

// console_project_tabs_test.go — the workspace's tabs on the fixture fleet
// (plan section E): the intent tab's read-only audit, JSON parity for the
// tabs whose verbs need a repository on this machine, the index tab's
// profile and neighbourhood, and the read-only Scope panel.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// seedProjectRoot points the cached model's row for svc at root — the
// fixture's services are faux-ingested with no local root, and a tab that
// reads the repository needs one on disk.
func seedProjectRoot(t *testing.T, f *consoleFixture, svc, root string) {
	t.Helper()
	m := cachedObservatoryModel(f.ts.URL, f.now)
	found := false
	for i := range m.Services {
		if m.Services[i].Service == svc {
			m.Services[i].Path = root
			found = true
		}
	}
	if !found {
		t.Fatalf("no fixture service %s", svc)
	}
	obsCache.Lock()
	// A rebuild flight the read above started (the cached model was stale)
	// would store its own model over this one when it lands — late, on a
	// loaded machine (CS-8). A new generation makes it drop that result.
	obsCache.gen++
	obsCache.model, obsCache.at = m, f.now
	obsCache.Unlock()
}

// runVerbJSON runs a rendered command exactly as shown and decodes its JSON.
func runVerbAsShownJSON(t *testing.T, verb string, clock ...time.Time) any {
	t.Helper()
	var out, errb bytes.Buffer
	// doctor's exit 1 is its verdict (a check failed), not a failure to
	// run; only its exit 2 is.
	var code int
	if strings.HasPrefix(verb, "fathom doctor ") && len(clock) > 0 {
		code = cmdDoctorWithClock(shellFields(verb)[2:], &out, &errb, func() time.Time { return clock[0] })
	} else {
		code = run(shellFields(verb)[1:], &out, &errb)
	}
	if code != 0 && !(code == 1 && strings.HasPrefix(verb, "fathom doctor ")) {
		t.Fatalf("%s exited %d: %s", verb, code, errb.String())
	}
	var v any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("%s: %v\n%s", verb, err, out.String())
	}
	return v
}

// assertSectionsEqualVerbs holds a page's JSON to the verbs it rendered.
func assertSectionsEqualVerbs(t *testing.T, env consolePageJSONWire, want int, clock ...time.Time) {
	t.Helper()
	if len(env.Verbs) != want {
		t.Fatalf("page %s rendered %d verbs, want %d: %v", env.Page, len(env.Verbs), want, env.Verbs)
	}
	for _, verb := range env.Verbs {
		raw, ok := env.Sections[verb]
		if !ok {
			t.Errorf("page %s: no section for %q", env.Page, verb)
			continue
		}
		var fromPage any
		if err := json.Unmarshal(raw, &fromPage); err != nil {
			t.Fatal(err)
		}
		fromVerb := runVerbAsShownJSON(t, verb, clock...)
		dropVolatileVerbFields(verb, fromVerb, fromPage)
		if !reflect.DeepEqual(fromVerb, fromPage) {
			a, _ := json.MarshalIndent(fromVerb, "", " ")
			b, _ := json.MarshalIndent(fromPage, "", " ")
			t.Errorf("page %s disagrees with `%s`:\nverb: %.2000s\npage: %.2000s", env.Page, verb, a, b)
		}
	}
}

func fileBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// TestIntentView_ReadOnlyAuditWritesNothing pins OC-38: `intent audit
// --read-only` renders the same audit and appends NOTHING — no clock,
// baseline or debt row to the intent ledger, no enforcement-probe cache row,
// no beat — while the plain verb still records its run (so the check can
// see a write). The intent tab renders IntentView over a read-only run:
// invariants with their rung and audit state, decisions and futures with
// their conditions, the unproven claims, and the ledger's debt over time;
// its JSON section equals the read-only verb run exactly as rendered, and
// looking at the tab, in HTML or JSON, leaves the ledger byte-identical.
func TestIntentView_ReadOnlyAuditWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	state := t.TempDir()
	ledger := filepath.Join(state, "intent-ledger.jsonl")
	hooks := filepath.Join(state, "hooks.jsonl")
	t.Setenv("FATHOM_INTENT_LEDGER", ledger)
	t.Setenv("FATHOM_HOOKS_LOG", hooks)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(state, "verify-ledger.jsonl"))
	// One recorded audit of this root already exists (a CLI run last week).
	mustWrite(t, ledger, `{"schema_version":1,"kind":"debt","at":"2026-09-18T10:00:00Z","repo_root":"`+root+`","by_kind":{"U4":3}}`+"\n")
	before := fileBytes(t, ledger)

	args := []string{"intent", "audit", "--read-only", "--json", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}
	var out, errb bytes.Buffer
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("read-only audit exited %d: %s", code, errb.String())
	}
	if got := fileBytes(t, ledger); got != before {
		t.Fatalf("--read-only appended to the intent ledger:\n%s", strings.TrimPrefix(got, before))
	}
	if b := fileBytes(t, hooks); b != "" {
		t.Errorf("--read-only emitted a beat: %s", b)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || doc["invariants"] != float64(2) {
		t.Fatalf("read-only JSON = %v (%v)", doc, err)
	}
	// The plain verb still records its run: the check above can see a write.
	out.Reset()
	if code := run([]string{"intent", "audit", "--json", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errb); code != 0 {
		t.Fatalf("audit exited %d", code)
	}
	if got := fileBytes(t, ledger); !strings.Contains(strings.TrimPrefix(got, before), `"kind":"debt"`) {
		t.Fatalf("the plain audit must append its debt row (the read-only check would otherwise prove nothing):\n%s", got)
	}

	// The projection over a read-only run.
	afterPlain := fileBytes(t, ledger)
	r, err := runIntentAudit(intentAuditOptions{Root: root, Service: "svc", URL: "http://127.0.0.1:1", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	v := console.IntentView(r.Corpus, r.Audit, r.Ledger, time.Now())
	if !v.ReadOnly || len(v.Invariants) != 2 || fileBytes(t, ledger) != afterPlain {
		t.Fatalf("view = %+v", v)
	}
	states := map[string]string{}
	for _, inv := range v.Invariants {
		states[inv.ID] = inv.State
	}
	if states["I2"] != "unenforced" || states["I1"] != "not computed" {
		t.Errorf("invariant states = %v (I2 is an honest stated rung; I1's test could not be checked with no index)", states)
	}
	if len(v.Decisions) != 1 || v.Decisions[0].Label != "Revisit when" || v.Decisions[0].State != "human-judged" {
		t.Errorf("decisions = %+v", v.Decisions)
	}
	var futureLabels []string
	for _, c := range v.Futures {
		futureLabels = append(futureLabels, c.Label+"/"+c.State)
	}
	if strings.Join(futureLabels, ",") != "Promote when/human-judged,Drop if/terminal" {
		t.Errorf("futures = %v", futureLabels)
	}
	if v.Fired.State != console.StateAbsent || !strings.Contains(v.Fired.Reason, "human-judged") {
		t.Errorf("with no annotations nothing can be shown to fire: %+v", v.Fired)
	}
	if v.UnprovenCount.State != console.StatePartial {
		t.Errorf("an audit with an uncomputable source is a lower bound: %+v", v.UnprovenCount)
	}
	if len(v.Debt) != 2 || v.Debt[0].Total != 3 || v.DebtState.V != 2 {
		t.Errorf("debt over time = %+v (%+v)", v.Debt, v.DebtState)
	}

	// The tab: rendering it, in HTML and JSON, leaves the ledger untouched,
	// and its section is the read-only verb run exactly as rendered.
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{LogCalls: true})
	t.Setenv("FATHOM_INTENT_LEDGER", ledger)
	seedProjectRoot(t, f, "payment-service", root)
	resetIntentAuditMemo()
	logBefore := fileBytes(t, f.callLog)
	code, _, page := f.get(t, "/projects/payment-service/intent")
	if code != http.StatusOK {
		t.Fatalf("intent tab = %d: %.400s", code, page)
	}
	// The audit's index oracle asked this daemon about I1's test — as the
	// console, so looking at the tab is never counted as usage.
	added := strings.TrimSpace(strings.TrimPrefix(fileBytes(t, f.callLog), logBefore))
	if added == "" {
		t.Error("the tab's audit must consult the index for I1's named test")
	}
	for _, ln := range strings.Split(added, "\n") {
		if ln != "" && !strings.Contains(ln, `"origin":"console"`) {
			t.Errorf("a call the intent tab made is not stamped as the console's own read: %s", ln)
		}
	}
	for _, want := range []string{`id="intent-invariants"`, "I2", "unenforced", `data-intent-group="decisions"`, "Revisit when", `id="intent-debt"`, "--read-only", `data-console-copy="fathom intent audit --read-only --json --root `} {
		if !strings.Contains(page, want) {
			t.Errorf("intent tab missing %q", want)
		}
	}
	env := f.pageJSON(t, "/projects/payment-service/intent")
	assertSectionsEqualVerbs(t, env, 1)
	if !strings.Contains(env.Verbs[0], "--url "+f.ts.URL) || !strings.Contains(env.Verbs[0], "--service payment-service") {
		t.Errorf("the rendered verb names this daemon and the project: %s", env.Verbs[0])
	}
	if got := fileBytes(t, ledger); got != afterPlain {
		t.Errorf("rendering the intent tab appended to the ledger:\n%s", strings.TrimPrefix(got, afterPlain))
	}
	if b := fileBytes(t, hooks); strings.Count(b, "\n") != 1 {
		t.Errorf("only the one plain CLI run may have beaten: %q", b)
	}
	// A project whose root is not on this machine offers no audit verb.
	resetObsCache()
	env = f.pageJSON(t, "/projects/payment-service/intent")
	var m projectIntentModel
	if err := json.Unmarshal(env.Model, &m); err != nil || m.RootNote == "" || len(env.Verbs) != 0 {
		t.Errorf("no local root = %+v, verbs %v", m, env.Verbs)
	}
}

func resetIntentAuditMemo() { intentAuditMemo.reset() }

// The wiring and index tabs' JSON sections equal their verbs run exactly as
// rendered, on a repository that is on this machine (OC-8 for the tabs the
// shared fixture cannot exercise: its services have no local root).
func TestProjectTabs_VerbParityOnALocalRoot(t *testing.T) {
	for _, history := range []bool{false, true} {
		name := "empty history"
		if history {
			name = "retained history"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# Project\n")
			f := newConsoleFixture(t)
			// Use the fixture's injected clock for both surfaces. Empty history must
			// remain comparable before the doctor's own invocation is recorded.
			if history {
				mustWrite(t, beatLogPath(), `{"at":"`+f.now.Add(-8*24*time.Hour).UTC().Format(time.RFC3339)+`","channel":"doctor","kind":"run"}`+"\n")
			}
			seedProjectRoot(t, f, "payment-service", root)

			env := f.pageJSON(t, "/projects/payment-service/wiring")
			assertSectionsEqualVerbs(t, env, 2, f.now)
			var wv console.WiringViewModel
			if err := json.Unmarshal(env.Model, &wv); err != nil {
				t.Fatal(err)
			}
			if len(wv.Checks) == 0 || !wv.Fail.Known() || !wv.ReceiptsTotal.Known() || wv.ReceiptsTotal.V != 0 || !wv.DriftCount.Known() {
				t.Errorf("wiring view on a local root = %+v", wv)
			}
			code, _, page := f.get(t, "/projects/payment-service/wiring")
			if code != http.StatusOK {
				t.Fatalf("wiring tab = %d", code)
			}
			for _, want := range []string{`id="wiring-doctor"`, `id="wiring-drift"`, `id="wiring-receipts"`, `action="/observatory/wiring/preview"`, `name="service" value="payment-service"`, "--repo-only"} {
				if !strings.Contains(page, want) {
					t.Errorf("wiring tab missing %q", want)
				}
			}
			// The tab only reads: nothing was written into the repository.
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Errorf("rendering the wiring tab wrote into the repository: %v", entries)
			}

			env = f.pageJSON(t, "/projects/payment-service/index")
			assertSectionsEqualVerbs(t, env, 2, f.now)
			env = f.pageJSON(t, "/projects/payment-service/activity")
			assertSectionsEqualVerbs(t, env, 1, f.now)
		})
	}
}

// The index tab: the profile (languages, typed share, edge kinds,
// inventory) from IndexProfile, and the neighbourhood through the kit's
// layered graph with its table alternative; an unknown symbol is said in
// words.
func TestProjectIndexTab_ProfileAndNeighborhood(t *testing.T) {
	f := newConsoleFixture(t)
	env := f.pageJSON(t, "/projects/payment-service/index?symbol=Charge")
	var m projectIndexModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Profile.Languages) == 0 || m.Profile.Languages[0].Language != "Go" || m.Profile.Languages[0].Engine != "built-in" {
		t.Errorf("languages = %+v", m.Profile.Languages)
	}
	if m.Profile.Inventory.Known() {
		t.Errorf("the fixture's corpus is not on this machine: the inventory is not measured, never 0 — %+v", m.Profile.Inventory)
	}
	if m.Neighborhood.Focus == "" || !strings.HasSuffix(m.Neighborhood.Focus, ".Charge") {
		t.Fatalf("Charge must resolve: %+v", m.Neighborhood)
	}
	_, _, page := f.get(t, "/projects/payment-service/index?symbol=Charge")
	for _, want := range []string{`id="neighborhood"`, `data-neighborhood-graph`, `role="img" aria-label="call neighborhood of`, "The neighborhood as a table", `id="index-languages"`, `id="index-edges"`, `id="index-inventory"`, `id="scope"`} {
		if !strings.Contains(page, want) {
			t.Errorf("index tab missing %q", want)
		}
	}
	_, _, miss := f.get(t, "/projects/payment-service/index?symbol=NoSuchSymbolAnywhere")
	if !strings.Contains(miss, `data-neighborhood-unresolved>no symbol named &#34;NoSuchSymbolAnywhere&#34;`) {
		t.Errorf("an unknown symbol must be said in words:\n%.300s", miss[strings.Index(miss, `id="neighborhood"`):])
	}
	assertRoleUtilitiesOnly(t, page)
}

// The Scope panel is read-only (SP-10 stays open): it lists the active
// suppressions for the project, previews what a glob would hide from the
// store's own path columns, and shows the purge's dry run and exact command
// only after the file list — with no form that posts anything.
func TestProjectScopePanel_ReadOnlyPreviewAndPurgePlan(t *testing.T) {
	f := newConsoleFixture(t)
	sup := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv("FATHOM_SUPPRESSIONS", sup)
	if _, err := scope.AppendSuppression(sup, scope.Suppression{Service: "payment-service", Pattern: "vendor/**", Reason: "third-party", CreatedAt: f.now.Add(-time.Hour), CreatedBy: scope.OriginCLI}); err != nil {
		t.Fatal(err)
	}
	_, _, page := f.get(t, "/projects/payment-service/index")
	panel := page[strings.Index(page, `id="scope"`):]
	for _, want := range []string{"vendor/**", "third-party", "fathom scope unsuppress ", `name="scope"`, `method="get"`} {
		if !strings.Contains(panel, want) {
			t.Errorf("scope panel missing %q", want)
		}
	}
	if strings.Contains(panel, "fathom purge ") || strings.Contains(page, `method="post" action="/observatory/scope`) {
		t.Error("the purge command appears only after a preview's file list; nothing posts")
	}
	_, _, preview := f.get(t, "/projects/payment-service/index?scope="+"*.go")
	panel = preview[strings.Index(preview, `id="scope"`):]
	for _, want := range []string{"data-scope-preview", "payment.go", "Danger zone", "data-purge-command", "fathom purge ", "--reason &#39;&lt;why&gt;&#39; --yes", "data-purge-not-here"} {
		if !strings.Contains(panel, want) {
			t.Errorf("preview missing %q", want)
		}
	}
	// JSON: the scope list section equals the verb.
	env := f.pageJSON(t, "/projects/payment-service/index")
	for _, verb := range env.Verbs {
		if strings.Contains(verb, "scope list") {
			var fromPage any
			_ = json.Unmarshal(env.Sections[verb], &fromPage)
			if !reflect.DeepEqual(runVerbAsShownJSON(t, verb), fromPage) {
				t.Errorf("scope list section disagrees with the verb")
			}
		}
	}
	// A preview writes nothing: the suppressions file is unchanged.
	b, _ := os.ReadFile(sup)
	if strings.Count(string(b), "\n") != 1 {
		t.Errorf("previewing wrote a suppression: %s", b)
	}
	resp, err := http.Post(f.ts.URL+"/projects/payment-service/index", "application/x-www-form-urlencoded", strings.NewReader("scope=*.go"))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST to the index tab = %d, want 405", resp.StatusCode)
	}
}

// The code neighbourhood's adapter classifies each edge through
// core.EdgeConfidenceOf, the one confidence rule (OC-37): a file-resolved
// edge (per-file import-graph matching, ADR-041 decision 5) carries a kind
// and is still heuristic — drawn dashed, counted heuristic, never "resolved
// (type-checked)" — while a direct typed edge is resolved.
func TestProjectNeighborhood_FileResolvedEdgesAreHeuristic(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "web", Name: "reveal", Kind: core.KindGoFunc, Calls: []string{"svc::web.announcer"}},
		{Service: "svc", Package: "web", Name: "boot", Kind: core.KindGoFunc, Calls: []string{"svc::web.announcer"}},
		{Service: "svc", Package: "web", Name: "announcer", Kind: core.KindGoFunc},
	}
	edges := []core.Edge{
		{FromService: "svc", FromSymbol: "svc::web.reveal", ToSymbol: "svc::web.announcer", Kind: core.EdgeFileResolved},
		{FromService: "svc", FromSymbol: "svc::web.boot", ToSymbol: "svc::web.announcer", Kind: core.EdgeDirect},
	}
	pg := projectGraph{nodes: nodes, g: callgraph.BuildBoth(nodes, edges), isTest: map[string]bool{}}
	nb := projectNeighborhood("svc", pg, "svc::web.announcer")
	if nb.Focus != "svc::web.announcer" {
		t.Fatalf("focus = %q (%s)", nb.Focus, nb.Unresolved)
	}
	if nb.CallersResolved != 1 || nb.CallersHeuristic != 1 {
		t.Errorf("callers = %d resolved, %d heuristic; want the direct edge resolved and the file-resolved one heuristic", nb.CallersResolved, nb.CallersHeuristic)
	}
	byKind := map[string]bool{}
	for _, l := range nb.Links {
		byKind[l.Kind] = l.Resolved
	}
	if resolved, ok := byKind[string(core.EdgeFileResolved)]; !ok || resolved {
		t.Errorf("the file-resolved link must be drawn heuristic: %+v", nb.Links)
	}
	if resolved, ok := byKind[string(core.EdgeDirect)]; !ok || !resolved {
		t.Errorf("the direct link must be drawn resolved: %+v", nb.Links)
	}
	svg, _, err := layoutNeighborhood("svc", nb)
	if err != nil {
		t.Fatal(err)
	}
	// One dashed edge (the file-resolved one) plus the legend's dashed
	// swatch, which the kit draws only when a dashed edge exists.
	if n := strings.Count(svg, `stroke-dasharray="5 4"`); n != 2 {
		t.Errorf("exactly one edge (the file-resolved one) is dashed: %d dashed strokes, want the edge and its legend swatch", n)
	}
}

// panelOpenTag is the opening tag of the panel whose id is id ("" when the
// page has no such panel): its attributes carry the panel's truth state,
// its live sources and its verb (data-console-verb).
func panelOpenTag(page, id string) string {
	i := strings.Index(page, `id="`+id+`"`)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(page[:i], "<div")
	end := strings.Index(page[i:], ">")
	if start < 0 || end < 0 {
		return ""
	}
	return page[start : i+end+1]
}

// A panel's verb chip names the command that produces its figures, or
// nothing: with no daemon record the evidence verb is not offered, and the
// languages panel must not borrow the scope list's command in its place.
func TestProjectIndexTab_LanguagesPanelNeverBorrowsAVerb(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, page := f.get(t, "/projects/payment-service/index")
	if tag := panelOpenTag(page, "index-languages"); !strings.Contains(tag, `data-console-verb="fathom report evidence --service payment-service --store `) {
		t.Fatalf("with a daemon record the languages panel names the evidence verb: %s", tag)
	}
	recPath, ok := daemonRecordPathFor(f.ts.URL)
	if !ok {
		t.Fatal("no daemon record path")
	}
	if err := os.Remove(recPath); err != nil {
		t.Fatal(err)
	}
	_, _, page = f.get(t, "/projects/payment-service/index")
	tag := panelOpenTag(page, "index-languages")
	if tag == "" {
		t.Fatal("no languages panel")
	}
	if strings.Contains(tag, "data-console-verb") {
		t.Errorf("with no daemon record the languages panel offers no verb, never another panel's: %s", tag)
	}
	if !strings.Contains(page, "the evidence verb is not offered") {
		t.Error("the panel says why no verb is offered")
	}
}

// A tab that reads the repository's files and has nothing to read on this
// machine says why in the kit's callout (doctrine rule 1: never a
// hand-written box the kit already has), with its ARIA role and icon: an
// info note for a remote corpus with no local root, a caution for a
// registered root this machine cannot read.
func TestProjectWiringTab_RootNoteIsAKitCallout(t *testing.T) {
	f := newConsoleFixture(t)
	for _, tc := range []struct {
		root, text, open string
	}{
		{"", "no local root is recorded for payment-service", `<div role="note" class="rounded-md border border-info bg-info-soft`},
		{filepath.Join(t.TempDir(), "gone"), "is not readable on this machine", `<div role="alert" class="rounded-md border border-caution bg-caution-soft`},
	} {
		seedProjectRoot(t, f, "payment-service", tc.root)
		_, _, page := f.get(t, "/projects/payment-service/wiring")
		if strings.Contains(page, "data-project-root-note") {
			t.Error("the root note is a hand-written box, not the kit's callout")
		}
		i := strings.Index(page, tc.text)
		if i < 0 {
			t.Fatalf("root %q: the wiring tab must say why there is nothing to read", tc.root)
		}
		open := strings.LastIndex(page[:i], `<div role="`)
		if open < 0 || !strings.HasPrefix(page[open:], tc.open) {
			t.Errorf("root %q: the note must be callout.Render's callout %s:\n%.300s", tc.root, tc.open, page[max(open, 0):])
		}
		assertRoleUtilitiesOnly(t, page)
	}
}

// The intent tab never reports an unreadable corpus as "no corpus" (I38,
// law 3): with the router and the decision records present and unreadable
// (the EPERM wedge's shape), the tab renders an error naming the files and
// the errno — never the clean "a choice, not a defect" absence — and its
// JSON says the same, while its section still equals the verb run exactly
// as rendered.
func TestIntentTab_UnreadableCorpusIsNeverNoCorpus(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads every file")
	}
	root := t.TempDir()
	writeCorpusFixture(t, root)
	state := t.TempDir()
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(state, "intent-ledger.jsonl"))
	f := newConsoleFixture(t)
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(state, "intent-ledger.jsonl"))
	seedProjectRoot(t, f, "payment-service", root)
	resetIntentAuditMemo()
	t.Cleanup(resetIntentAuditMemo)
	for _, p := range []string{filepath.Join(root, "docs", "intent", "README.md"), filepath.Join(root, "ADR.md")} {
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
	code, _, page := f.get(t, "/projects/payment-service/intent")
	if code != http.StatusOK {
		t.Fatalf("intent tab = %d", code)
	}
	if strings.Contains(page, "a choice, not a defect") || strings.Contains(page, "no intent corpus") {
		t.Error("an unreadable corpus rendered as no corpus")
	}
	tag := panelOpenTag(page, "intent")
	if !strings.Contains(tag, `data-truth-state="error"`) {
		t.Errorf("the corpus panel must be an error: %s", tag)
	}
	for _, want := range []string{"docs/intent/README.md", "ADR.md", "permission denied", `data-truth-line="error"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the tab must name %q", want)
		}
	}
	env := f.pageJSON(t, "/projects/payment-service/intent")
	var m projectIntentModel
	if err := json.Unmarshal(env.Model, &m); err != nil || m.View == nil {
		t.Fatalf("model = %s (%v)", env.Model, err)
	}
	if m.View.Corpus.State != console.StateError || !strings.Contains(m.View.Corpus.Reason, "permission denied") {
		t.Errorf("view.corpus = %+v, want an error naming the unreadable files", m.View.Corpus)
	}
	assertSectionsEqualVerbs(t, env, 1)
}

// The intent tab keeps an audit per (root, HEAD) only while nothing its
// panels follow has moved (design §1 "Caching", §2 "Live freshness"): an
// event naming plans:{svc} — the moment every intent panel shows "updated —
// refresh this panel" — makes the refresh re-run the audit instead of
// re-serving the one the affordance just called out of date; an event for
// another project leaves it kept.
func TestIntentTab_RefreshAfterAnUpdateReaudits(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	ledger := filepath.Join(t.TempDir(), "intent-ledger.jsonl")
	f := newConsoleFixture(t)
	t.Setenv("FATHOM_INTENT_LEDGER", ledger)
	resetIntentAuditMemo()
	t.Cleanup(resetIntentAuditMemo)
	auditAt := func() string {
		t.Helper()
		seedProjectRoot(t, f, "payment-service", root) // the model is rebuilt as the clock moves
		var m projectIntentModel
		if err := json.Unmarshal(f.pageJSON(t, "/projects/payment-service/intent").Model, &m); err != nil || m.AuditAt == "" {
			t.Fatalf("intent model = %+v (%v)", m, err)
		}
		return m.AuditAt
	}
	first := auditAt()
	f.advance(time.Minute)
	if got := auditAt(); got != first {
		t.Fatalf("with nothing moved the audit is kept: %s, then %s", first, got)
	}
	if _, err := f.deps.bus.Publish(console.Event{Kind: console.EventVerify, Service: "checkout-service", Name: "other.md", Ref: "fail"}); err != nil {
		t.Fatal(err)
	}
	if got := auditAt(); got != first {
		t.Errorf("another project's verify event re-ran this project's audit: %s, then %s", first, got)
	}
	if _, err := f.deps.bus.Publish(console.Event{Kind: console.EventVerify, Service: "payment-service", Name: "plan.md", Ref: "fail"}); err != nil {
		t.Fatal(err)
	}
	if got, want := auditAt(), f.now.UTC().Format(time.RFC3339); got != want {
		t.Errorf("after a verify event for the project the refresh must re-audit: audit_at %s, want %s", got, want)
	}
	// A recorded CLI audit (the intent ledger grew) is new data for the debt
	// panel: it is re-read too.
	f.advance(time.Minute)
	mustWrite(t, ledger, `{"schema_version":1,"kind":"debt","at":"2026-09-25T12:00:30Z","repo_root":"`+root+`","by_kind":{"U4":1}}`+"\n")
	if got, want := auditAt(), f.now.UTC().Format(time.RFC3339); got != want {
		t.Errorf("after the intent ledger grew the audit must re-read it: audit_at %s, want %s", got, want)
	}
}

// The wiring tab keeps a read of the repository for a minute only while
// nothing its panels follow has moved: an ingest event for the project
// (index:{svc}, the source its panels follow — the moment they show
// "updated — refresh this panel") makes the refresh re-read the repository,
// and so does a new receipt (an apply or undo); an event for another
// project does not.
func TestWiringTab_RefreshAfterAnUpdateRereads(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# Project\n")
	f := newConsoleFixture(t)
	projectWiringMemo.reset()
	t.Cleanup(projectWiringMemo.reset)
	failing := func() int {
		t.Helper()
		seedProjectRoot(t, f, "payment-service", root)
		var wv console.WiringViewModel
		if err := json.Unmarshal(f.pageJSON(t, "/projects/payment-service/wiring").Model, &wv); err != nil || !wv.Fail.Known() {
			t.Fatalf("wiring model = %+v (%v)", wv, err)
		}
		return wv.Fail.V
	}
	before := failing()
	// config_valid now fails — on disk, not yet on the page.
	mustWrite(t, filepath.Join(root, ".fathom", "config.toml"), "this is = [not toml\n")
	f.advance(10 * time.Second)
	if got := failing(); got != before {
		t.Fatalf("with no event the read is kept: %d failing, then %d", before, got)
	}
	if _, err := f.deps.bus.Publish(console.Event{Kind: console.EventIngest, Service: "checkout-service", Ref: "c0ffee"}); err != nil {
		t.Fatal(err)
	}
	if got := failing(); got != before {
		t.Errorf("another project's ingest re-read this repository: %d, then %d", before, got)
	}
	if _, err := f.deps.bus.Publish(console.Event{Kind: console.EventIngest, Service: "payment-service", Ref: "c0ffee"}); err != nil {
		t.Fatal(err)
	}
	if got := failing(); got != before+1 {
		t.Errorf("after an ingest event for the project the refresh must re-read it: %d failing, want %d", got, before+1)
	}
	// A receipt (an apply through the console or the CLI) is new data too.
	if err := os.Remove(filepath.Join(root, ".fathom", "config.toml")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, wiring.ReceiptsPath(root), `{"id":"r1","time":"2026-09-25T12:00:00Z","files":[]}`+"\n")
	if got := failing(); got != before {
		t.Errorf("after a new receipt the tab must re-read the repository: %d failing, want %d", got, before)
	}
}

// At 390 px nine tabs overflow their strip, which scrolls sideways in its
// own region: the console's script brings the current tab into that
// region's view on load (never scrolling the page itself, so an anchored
// refresh stays put), and without JavaScript the h1 and the trail still
// name the tab. The doctor table leads with each check's Status, so a phone
// sees PASS/FAIL before it scrolls sideways.
func TestProjectWorkspace_PhoneKeepsTheCurrentTabAndStatusInView(t *testing.T) {
	js := string(consoleJS)
	for _, want := range []string{"[data-project-tabs]", `[aria-current="page"]`, "scrollLeft"} {
		if !strings.Contains(js, want) {
			t.Errorf("console.js must bring the current tab into its strip's view: missing %q", want)
		}
	}
	if strings.Contains(js, "scrollIntoView") {
		t.Error("scrollIntoView scrolls the page too; the strip is scrolled by itself")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# Project\n")
	f := newConsoleFixture(t)
	seedProjectRoot(t, f, "payment-service", root)
	_, _, page := f.get(t, "/projects/payment-service/wiring")
	if !strings.Contains(page, `data-project-tabs`) || !strings.Contains(page, `aria-current="page" href="/projects/payment-service/wiring"`) {
		t.Fatal("the wiring tab is the marked tab in the strip")
	}
	i := strings.Index(page, `id="wiring-doctor"`)
	if i < 0 {
		t.Fatal("no doctor panel")
	}
	tbl := page[i:]
	first := strings.Index(tbl, "<th")
	end := strings.Index(tbl, "</th>")
	if first < 0 || end < first || !strings.HasPrefix(stripTags(tbl[first:end]), "Status") {
		t.Errorf("the doctor table's first column must be Status:\n%.600s", tbl[max(first, 0):])
	}
}

// stripTags drops markup from an HTML fragment, leaving its text.
func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
