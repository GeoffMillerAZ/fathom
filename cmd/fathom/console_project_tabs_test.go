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

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/scope"
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
	obsCache.model, obsCache.at = m, f.now
	obsCache.Unlock()
}

// runVerbJSON runs a rendered command exactly as shown and decodes its JSON.
func runVerbJSON(t *testing.T, verb string) any {
	t.Helper()
	var out, errb bytes.Buffer
	// doctor's exit 1 is its verdict (a check failed), not a failure to
	// run; only its exit 2 is.
	code := run(shellFields(verb)[1:], &out, &errb)
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
func assertSectionsEqualVerbs(t *testing.T, env consolePageJSONWire, want int) {
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
		fromVerb := runVerbJSON(t, verb)
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

func resetIntentAuditMemo() {
	intentAuditMemo.Lock()
	intentAuditMemo.entries = nil
	intentAuditMemo.Unlock()
}

// The wiring and index tabs' JSON sections equal their verbs run exactly as
// rendered, on a repository that is on this machine (OC-8 for the tabs the
// shared fixture cannot exercise: its services have no local root).
func TestProjectTabs_VerbParityOnALocalRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# Project\n")
	f := newConsoleFixture(t)
	seedProjectRoot(t, f, "payment-service", root)

	env := f.pageJSON(t, "/projects/payment-service/wiring")
	assertSectionsEqualVerbs(t, env, 2)
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
	assertSectionsEqualVerbs(t, env, 2)
	env = f.pageJSON(t, "/projects/payment-service/activity")
	assertSectionsEqualVerbs(t, env, 1)
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
			if !reflect.DeepEqual(runVerbJSON(t, verb), fromPage) {
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
