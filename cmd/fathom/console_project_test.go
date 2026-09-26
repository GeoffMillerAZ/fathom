package main

// console_project_test.go — the project workspace (vision 35 §3, plan
// section E): tabs that are URLs, and the helpers the shared JSON-parity
// test uses for pages whose verbs name their project.

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// verbsFillTemplates reports whether rendered are the page's declared verb
// templates filled for one request: with no placeholder in any template,
// exactly the templates; otherwise each rendered verb fills the next
// matching template, in order (a verb whose input the request lacks — a
// root not on this machine — is left out, never rendered half-filled).
func verbsFillTemplates(rendered, templates []string) bool {
	templated := false
	for _, t := range templates {
		templated = templated || strings.Contains(t, "{")
	}
	if !templated {
		if len(rendered) != len(templates) {
			return false
		}
		for i := range rendered {
			if rendered[i] != templates[i] {
				return false
			}
		}
		return true
	}
	j := 0
	for _, v := range rendered {
		if strings.Contains(v, "{") && regexp.MustCompile(`\{[a-z]+\}`).MatchString(v) {
			return false // an unfilled placeholder
		}
		for j < len(templates) && !templateRe(templates[j]).MatchString(v) {
			j++
		}
		if j == len(templates) {
			return false
		}
		j++
	}
	return true
}

func templateRe(t string) *regexp.Regexp {
	q := regexp.QuoteMeta(t)
	q = regexp.MustCompile(`\\\{[a-z]+\\\}`).ReplaceAllString(q, `.+?`)
	return regexp.MustCompile("^" + q + "$")
}

// shellFields splits a rendered command the way a POSIX shell would for
// the quoting fillVerb emits (single quotes, '\” for a quote).
func shellFields(cmd string) []string {
	var out []string
	var cur strings.Builder
	inWord, quoted := false, false
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quoted && c == '\'':
			quoted = false
		case quoted:
			cur.WriteByte(c)
		case c == '\'':
			quoted, inWord = true, true
		case c == '\\' && i+1 < len(cmd):
			i++
			cur.WriteByte(cmd[i])
			inWord = true
		case c == ' ':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}

// dropVolatileVerbFields removes the fields two runs of one verb cannot
// share: `intent audit`'s "ms" is that run's own elapsed time.
func dropVolatileVerbFields(verb string, docs ...any) {
	if !strings.Contains(verb, "intent audit") {
		return
	}
	for _, d := range docs {
		if m, ok := d.(map[string]any); ok {
			delete(m, "ms")
		}
	}
}

func TestShellFieldsRoundTripsFillVerb(t *testing.T) {
	cmd := fillVerb("fathom x --root {root} --service {svc}", map[string]string{"root": "/tmp/it's here", "svc": "payment-service"})
	got := shellFields(cmd)
	want := []string{"fathom", "x", "--root", "/tmp/it's here", "--service", "payment-service"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("shellFields(%q) = %q", cmd, got)
	}
}

// TestProjectWorkspace_TabsAreURLs pins OC-35: the workspace has nine tabs —
// overview, index, governance, plans, proof, refactor, intent, activity,
// wiring — each a plain link (no script needed); the five the workspace
// renders are registered pages at /projects/{svc}/<tab> that answer on the
// real handler with the tab marked current, the project in the trail and a
// one-h1 shell; the other four open the Knowledge and Proof pages filtered
// to the project. An unknown project on any tab is OC-5's page.
func TestProjectWorkspace_TabsAreURLs(t *testing.T) {
	tabs := projectTabs("payment-service", "index")
	var ids []string
	for _, tb := range projectTabList {
		ids = append(ids, tb.ID)
	}
	if strings.Join(ids, ",") != "overview,index,governance,plans,proof,refactor,intent,activity,wiring" {
		t.Fatalf("tabs = %v", ids)
	}
	wantHref := map[string]string{
		"Overview": "/projects/payment-service", "Index": "/projects/payment-service/index",
		"Governance": "/layers?svc=payment-service", "Plans": "/plans?svc=payment-service",
		"Proof": "/certifications?svc=payment-service", "Refactor": "/refactors?svc=payment-service",
		"Intent": "/projects/payment-service/intent", "Activity": "/projects/payment-service/activity", "Wiring": "/projects/payment-service/wiring",
	}
	active := 0
	for _, tb := range tabs {
		if wantHref[tb.Label] != tb.Href {
			t.Errorf("tab %s → %s, want %s", tb.Label, tb.Href, wantHref[tb.Label])
		}
		if tb.Active {
			active++
			if tb.Label != "Index" {
				t.Errorf("active tab = %s", tb.Label)
			}
		}
	}
	if active != 1 {
		t.Errorf("%d active tabs, want exactly one", active)
	}
	// Every tab the workspace renders is a registered page at its URL.
	reg := defaultConsoleRegistry()
	for _, tb := range projectTabList {
		if tb.Page == "" {
			continue
		}
		p, ok := reg.page(tb.Page)
		if !ok || p.Route != "/projects/{svc}"+tb.Suffix || p.NavParent != "fleet" || p.Group != "Fleet" {
			t.Errorf("tab %s: page %q not registered at /projects/{svc}%s under Fleet", tb.ID, tb.Page, tb.Suffix)
		}
	}

	f := newConsoleFixture(t)
	for _, tb := range projectTabList {
		if tb.Page == "" {
			continue
		}
		path := projectTabHref(tb, "payment-service")
		code, _, body := f.get(t, path)
		if code != http.StatusOK {
			t.Fatalf("%s = %d: %.300s", path, code, body)
		}
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s: %d h1", path, strings.Count(body, "<h1"))
		}
		bar := body[strings.Index(body, "data-project-tabs"):]
		bar = bar[:strings.Index(bar, "</nav>")]
		if strings.Count(bar, `role="tab"`) != 9 || strings.Count(bar, `aria-current="page"`) != 1 {
			t.Errorf("%s: the tab bar must carry nine link tabs, one current:\n%s", path, bar)
		}
		cur := regexp.MustCompile(`aria-current="page" href="([^"]+)"`).FindStringSubmatch(bar)
		if cur == nil || cur[1] != path {
			t.Errorf("%s: current tab = %v", path, cur)
		}
		if strings.Contains(bar, "<script") || strings.Contains(bar, "onclick") {
			t.Errorf("%s: a tab is a plain link", path)
		}
		trail := body[strings.Index(body, `aria-label="Breadcrumb"`):]
		trail = trail[:strings.Index(trail, "</nav>")]
		if !strings.Contains(trail, `href="/fleet"`) || !strings.Contains(trail, "payment-service") {
			t.Errorf("%s: the trail runs Observatory / Fleet / payment-service …:\n%s", path, trail)
		}
		if tb.ID != "overview" && (!strings.Contains(trail, `href="/projects/payment-service"`) || !strings.Contains(trail, `aria-current="page">`+tb.Label+`</li>`)) {
			t.Errorf("%s: the trail links the project and ends at the tab:\n%s", path, trail)
		}
		assertRoleUtilitiesOnly(t, body)
		// JSON: the console envelope, the tab's own page id.
		env := f.pageJSON(t, path)
		if env.Page != tb.Page {
			t.Errorf("%s JSON page = %s, want %s", path, env.Page, tb.Page)
		}
		// An unknown project on the tab is OC-5's page, not a bare 404.
		code, _, miss := f.get(t, strings.Replace(path, "payment-service", "nope-service", 1))
		if code != http.StatusNotFound || !strings.Contains(miss, "Unknown project") {
			t.Errorf("%s for an unknown project = %d", path, code)
		}
	}
	// The overview's JSON still carries the project's row, as F0 served it.
	var row map[string]any
	if err := json.Unmarshal(f.pageJSON(t, "/projects/payment-service").Model, &row); err != nil || row["service"] != "payment-service" {
		t.Errorf("overview model = %v (%v)", row, err)
	}
	// The workspace's shared metrics are the project's.
	env := f.pageJSON(t, "/projects/payment-service/activity")
	var calls console.Value[int]
	if err := json.Unmarshal(env.Metrics[string(console.MetricProjectCalls)], &calls); err != nil || calls.V != 2 {
		t.Errorf("project.calls = %+v (%v), want payment-service's 2 fixture rows", calls, err)
	}
}
