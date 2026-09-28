package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// supportTestRoot is this checkout: the page projects the real committed
// inputs, the same files `fathom support` reads.
func supportTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// supportRequest builds one request to /support with the checkout pinned.
func supportRequest(t *testing.T, target, root string) *consoleRequest {
	t.Helper()
	prev := supportRootFor
	supportRootFor = func(*consoleRequest) (string, string, string) { return "fathom", root, "" }
	t.Cleanup(func() { supportRootFor = prev })
	supportMemo.reset()
	t.Cleanup(supportMemo.reset)
	p, ok := defaultConsoleRegistry().page("support")
	if !ok {
		t.Fatal("/support is not registered")
	}
	return &consoleRequest{w: httptest.NewRecorder(), r: httptest.NewRequest("GET", target, nil), deps: &consoleDeps{}, page: p, now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)}
}

// jsonOf round-trips v through JSON so it compares with a verb's output.
func jsonOf(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestConsoleSupportPageMatchesJSON pins SM-20: /support is declared in the
// System group, its JSON section is exactly `fathom support --root R
// --json` (and `--kind K` when the kind filter is set), and every panel's
// footer names its verb, sources, denominator and freshness.
func TestConsoleSupportPageMatchesJSON(t *testing.T) {
	p, ok := defaultConsoleRegistry().page("support")
	if !ok || p.Route != "/support" || p.Group != "System" || !p.Nav {
		t.Fatalf("support page = %+v, want a navigable /support in System", p)
	}
	root := supportTestRoot(t)
	for _, tc := range []struct{ target, kind string }{{"/support", ""}, {"/support?kind=client", "client"}} {
		cr := supportRequest(t, tc.target, root)
		out, err := buildSupportPage(cr)
		if err != nil {
			t.Fatal(err)
		}
		want := fillVerb(verbSupport, map[string]string{"root": root})
		args := []string{"support", "--root", root, "--json"}
		if tc.kind != "" {
			want = fillVerb(verbSupportKind, map[string]string{"root": root, "kind": tc.kind})
			args = []string{"support", "--root", root, "--kind", tc.kind, "--json"}
		}
		if len(out.Verbs) != 1 || out.Verbs[0] != want {
			t.Fatalf("%s: verbs = %v, want [%s]", tc.target, out.Verbs, want)
		}
		if !verbsFillTemplates(out.Verbs, p.Verbs) {
			t.Errorf("%s: verbs %v do not fill the registry's %v", tc.target, out.Verbs, p.Verbs)
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("fathom %s: exit %d: %s", strings.Join(args, " "), code, stderr.String())
		}
		var fromVerb map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &fromVerb); err != nil {
			t.Fatal(err)
		}
		fromPage := jsonOf(t, out.Sections[want])
		delete(fromVerb, "generated_at") // each run's own clock
		delete(fromPage, "generated_at")
		if !reflect.DeepEqual(fromVerb, fromPage) {
			t.Errorf("%s: page section disagrees with `%s`", tc.target, want)
		}
		if entries, _ := fromPage["entries"].([]any); len(entries) == 0 {
			t.Errorf("%s: the matrix is empty — the parity proved nothing", tc.target)
		}

		body := out.Body()
		panels := regexp.MustCompile(`(?s)<div class="[^"]*" data-console-panel.*?data-panel-footer>([^<]*)</p>`).FindAllStringSubmatch(body, -1)
		if n := strings.Count(body, `data-live-source="index:fathom plans:fathom"`); n != 4 {
			t.Errorf("%s: %d panels follow the checkout's ingest and plans, want 4", tc.target, n)
		}
		// A live panel's freshness leads its status line (console_live.go).
		if n := strings.Count(body, `data-live-asof class="col-start-1 row-start-1">freshness: as of `); n != 4 {
			t.Errorf("%s: %d panels name their freshness, want 4", tc.target, n)
		}
		if len(panels) != 4 {
			t.Fatalf("%s: %d panels with a footer, want the heatmap, dialects, progress and next", tc.target, len(panels))
		}
		for _, m := range panels {
			for _, part := range []string{"source: ", "denominator: "} {
				if !strings.Contains(m[1], part) {
					t.Errorf("%s: a panel footer %q lacks %q", tc.target, m[1], part)
				}
			}
			if !strings.Contains(m[0], `data-console-verb="`+htmlAttr(want)+`"`) {
				t.Errorf("%s: a panel does not name its verb %s", tc.target, want)
			}
		}
	}

	// No registered checkout: absent with its reason, never an empty matrix.
	cr := supportRequest(t, "/support", "")
	supportRootFor = func(*consoleRequest) (string, string, string) {
		return "", "", "no registered project's root carries it"
	}
	out, err := buildSupportPage(cr)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Verbs) != 0 || len(out.Sections) != 0 || !strings.Contains(out.Body(), `data-truth-state="absent"`) {
		t.Errorf("no checkout: verbs %v, body must be an absent panel", out.Verbs)
	}
}

func htmlAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "'", "&#39;", `"`, "&#34;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// TestSupportFiltersInURL pins SM-25: kind, wave, engine and state are held
// in the URL — parsed, validated, written back in a fixed order, selected in
// the form and applied to the panels — so a filtered answer is a link.
func TestSupportFiltersInURL(t *testing.T) {
	q, _ := url.ParseQuery("state=below-target&wave=1&kind=language&engine=tags")
	f, err := supportFilters(q)
	if err != nil {
		t.Fatal(err)
	}
	if f != (supportFilter{Kind: "language", Wave: 1, Engine: "tags", State: "below-target"}) {
		t.Errorf("filters = %+v", f)
	}
	if f.href() != "/support?kind=language&wave=1&engine=tags&state=below-target" {
		t.Errorf("href = %s, want the fixed key order", f.href())
	}
	back, _ := url.Parse(f.href())
	if g, err := supportFilters(back.Query()); err != nil || g != f {
		t.Errorf("round trip = %+v, %v", g, err)
	}
	for _, bad := range []string{"kind=nope", "state=great", "wave=0", "wave=x"} {
		q, _ := url.ParseQuery(bad)
		if _, err := supportFilters(q); err == nil {
			t.Errorf("%s: accepted, want refused", bad)
		}
	}
	if (supportFilter{}).href() != "/support" {
		t.Error("no filter is the bare page")
	}

	root := supportTestRoot(t)
	cr := supportRequest(t, "/support?kind=language&wave=1&state=tolerated", root)
	out, err := buildSupportPage(cr)
	if err != nil {
		t.Fatal(err)
	}
	m := out.Model.(*supportPageModel)
	if m.Filters.Kind != "language" || m.Filters.Wave != 1 {
		t.Fatalf("model filters = %+v", m.Filters)
	}
	for _, g := range m.Heatmap {
		if g.Kind != "language" {
			t.Errorf("kind filter leaked %s", g.Kind)
		}
		for _, r := range g.Rows {
			if r.Wave != 1 {
				t.Errorf("wave filter leaked %s (wave %d)", r.ID, r.Wave)
			}
			has := false
			for _, c := range r.Cells {
				has = has || c.State == "tolerated"
			}
			if !has {
				t.Errorf("state filter leaked %s", r.ID)
			}
		}
	}
	for _, w := range m.Next.Waves {
		if w.Wave != 1 {
			t.Errorf("what's next shows wave %d under a wave 1 filter", w.Wave)
		}
	}
	body := out.Body()
	for _, want := range []string{
		`name="kind"`, `<option value="language" selected`, `<option value="1" selected`, `<option value="tolerated" selected`,
		`href="/support?kind=language&amp;wave=1&amp;state=tolerated" data-support-permalink`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("filtered body lacks %q", want)
		}
	}

	bad := supportRequest(t, "/support?wave=-3", root)
	_, err = buildSupportPage(bad)
	var ce *consoleError
	if !errors.As(err, &ce) || ce.Status != 400 {
		t.Errorf("a bad filter = %v, want a 400 naming it", err)
	}
}
