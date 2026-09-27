package serviceref

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// fauxtownRoot resolves the fixture fleet root the same way
// internal/indexbench.FauxtownRoot does, without importing indexbench (to
// keep this package's test dependency graph shallow).
func fauxtownRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "connector", "testdata", "fauxtown")
}

func readFixture(t *testing.T, root, svc, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, svc, name))
	if err != nil {
		t.Fatalf("read fixture %s/%s: %v", svc, name, err)
	}
	return data
}

// ---------------------------------------------------------------------
// NormalizeTemplate
// ---------------------------------------------------------------------

func TestNormalizeTemplate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"colon param", "/items/:id", "/items/{}"},
		{"brace param", "/items/{id}", "/items/{}"},
		{"brace param with regex constraint", "/items/{id:[0-9]+}", "/items/{}"},
		{"trailing star wildcard", "/items/*", "/items/{}"},
		{"named trailing wildcard", "/items/*rest", "/items/{}"},
		{"printf %s verb", "/items/%s", "/items/{}"},
		{"printf %d verb", "/items/%d", "/items/{}"},
		{"printf width verb", "/items/%02d", "/items/{}"},
		{"scheme and host stripped", "http://catalog-service/items/%s", "/items/{}"},
		{"https scheme and host stripped", "https://catalog-service:8080/items/:id", "/items/{}"},
		{"no params unchanged", "/items", "/items"},
		{"missing leading slash gains one", "items/:id", "/items/{}"},
		{"multi-segment with two params", "/accounts/:acct/items/{id}", "/accounts/{}/items/{}"},

		// Host-strip must be ANCHORED at the string's start — a scheme+host
		// appearing later (e.g. inside a query value) is not a leading
		// host and must not be stripped.
		{"unanchored scheme in query value is not a host strip", "/redirect?next=http://evil.com/path", "/redirect"},

		// Query string and fragment stripping.
		{"query string stripped", "/items?limit=10", "/items"},
		{"fragment stripped", "/items#frag", "/items"},
		{"query then fragment stripped", "/items?limit=10#frag", "/items"},
		{"colon param with trailing query", "/items/:id?limit=10", "/items/{}"},

		// Trailing slash normalisation.
		{"trailing slash normalised", "/items/", "/items"},
		{"root stays root", "/", "/"},
		{"trailing slash after colon param", "/items/:id/", "/items/{}"},

		// Colon params are segment-anchored: the colon must be the FIRST
		// character of its segment, or it stays completely literal.
		{"colon not segment-anchored stays literal", "/events/at:noon", "/events/at:noon"},
		{"colon mid-word in last segment stays literal", "/foo/bar:baz", "/foo/bar:baz"},

		// Wildcards normalise wherever they form a WHOLE segment, not only
		// at the end of the path.
		{"bare wildcard mid-path", "/foo/*/bar", "/foo/{}/bar"},
		{"named wildcard mid-path", "/foo/*rest/bar", "/foo/{}/bar"},
		{"bare wildcard at end", "/foo/*", "/foo/{}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeTemplate(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeTemplate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// ExtractClientCalls
// ---------------------------------------------------------------------

func TestExtractClientCalls_InlineSnippets(t *testing.T) {
	src := []byte(`package demo

import (
	"context"
	"fmt"
	"net/http"
)

func FetchWidget(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://widgets/widgets/%s", id), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

func Ping() error {
	_, err := http.Get("http://widgets/health")
	return err
}

func CreateWidget(name string) error {
	_, err := http.Post("http://widgets/widgets", "application/json", nil)
	return err
}

func Unresolvable(u string) error {
	// Not resolvable: URL is a bare identifier, not a literal or Sprintf.
	req, err := http.NewRequestWithContext(context.Background(), "GET", u, nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)

	calls := ExtractClientCalls("go", "demo.go", src)

	want := []ClientCall{
		{Method: "GET", Template: "/widgets/{}", RawURL: `fmt.Sprintf("http://widgets/widgets/%s", id)`, Symbol: "FetchWidget", File: "demo.go", Line: 10},
		{Method: "GET", Template: "/health", RawURL: `"http://widgets/health"`, Symbol: "Ping", File: "demo.go", Line: 19},
		{Method: "POST", Template: "/widgets", RawURL: `"http://widgets/widgets"`, Symbol: "CreateWidget", File: "demo.go", Line: 24},
	}
	if len(calls) != len(want) {
		t.Fatalf("ExtractClientCalls: got %d calls, want %d: %+v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %+v, want %+v", i, calls[i], w)
		}
	}
}

func TestExtractClientCalls_CommentOnlyExampleYieldsNoCalls(t *testing.T) {
	// A regex over raw bytes would find this; the AST walk must not — the
	// example lives inside a doc comment, never becoming an ast.CallExpr.
	src := []byte(`package demo

// FetchExample shows typical usage:
//
//	resp, err := http.Get("http://example.com/should-not-count")
//	_ = resp
//	_ = err
func FetchExample() {}
`)
	if calls := ExtractClientCalls("go", "demo.go", src); len(calls) != 0 {
		t.Fatalf("ExtractClientCalls found calls inside a doc comment: %+v", calls)
	}
}

func TestExtractClientCalls_HTTPMethodConstant(t *testing.T) {
	src := []byte(`package demo

import (
	"context"
	"net/http"
)

func Fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://svc/widgets", nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	calls := ExtractClientCalls("go", "demo.go", src)
	if len(calls) != 1 {
		t.Fatalf("ExtractClientCalls = %+v, want exactly one call", calls)
	}
	if calls[0].Method != "GET" || calls[0].Template != "/widgets" {
		t.Errorf("call = %+v, want Method GET, Template /widgets (from http.MethodGet)", calls[0])
	}
}

func TestExtractClientCalls_UnparsableSourceYieldsNoCalls(t *testing.T) {
	if calls := ExtractClientCalls("go", "broken.go", []byte(`package demo(((not valid go`)); calls != nil {
		t.Fatalf("ExtractClientCalls(unparsable source) = %+v, want nil", calls)
	}
}

func TestExtractClientCalls_NonGoLangIsEmpty(t *testing.T) {
	if got := ExtractClientCalls("python", "app.py", []byte(`requests.get("http://x/y")`)); got != nil {
		t.Errorf("ExtractClientCalls(python, ...) = %+v, want nil (Go-only for VB-30)", got)
	}
}

// ---------------------------------------------------------------------
// ExtractClientCalls — bounded intra-function dataflow (VB-31 M8)
// ---------------------------------------------------------------------

func TestExtractClientCalls_SprintfAssignedToVariable(t *testing.T) {
	// The fmt.Sprintf call is NOT inline at the argument position — it's
	// assigned to a local variable first, then that variable is used
	// later. Also exercises the leading-verb-is-a-base heuristic: the
	// format's first %s pairs with a `.String()` call, so it's treated as
	// an interpolated host (stripped to "/"), not an ordinary "{}"
	// placeholder.
	src := []byte(`package demo

import (
	"fmt"
	"net/http"
	"net/url"
)

func Scale(base url.URL, name string) error {
	urlPath := fmt.Sprintf("%ssystem/scale-function/%s?namespace=%s", base.String(), name, "default")
	req, err := http.NewRequest(http.MethodPost, urlPath, nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	calls := ExtractClientCalls("go", "demo.go", src)
	if len(calls) != 1 {
		t.Fatalf("ExtractClientCalls = %+v, want exactly one call", calls)
	}
	if calls[0].Method != "POST" || calls[0].Template != "/system/scale-function/{}" {
		t.Errorf("call = %+v, want Method POST, Template /system/scale-function/{}", calls[0])
	}
}

func TestExtractClientCalls_PathJoinOnURLStringer(t *testing.T) {
	// endpointURL.Path is extended via path.Join BEFORE .String() is
	// called on it — the self-referential path.Join(endpointURL.Path, …)
	// argument is the pre-existing base, not a substituted value, and is
	// dropped rather than turned into a "{}" wildcard.
	src := []byte(`package demo

import (
	"net/http"
	"net/url"
	"path"
)

func Get(endpointURL url.URL) error {
	endpointURL.Path = path.Join(endpointURL.Path, "/system/functions")
	req, err := http.NewRequest(http.MethodGet, endpointURL.String(), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	calls := ExtractClientCalls("go", "demo.go", src)
	if len(calls) != 1 {
		t.Fatalf("ExtractClientCalls = %+v, want exactly one call", calls)
	}
	if calls[0].Method != "GET" || calls[0].Template != "/system/functions" {
		t.Errorf("call = %+v, want Method GET, Template /system/functions", calls[0])
	}
}

func TestExtractClientCalls_URLStringerWithNoVisiblePathIsUnresolved(t *testing.T) {
	// No `.Path = …` assignment and no composite-literal initializer
	// anywhere in the function — the base is not "visible", so this
	// resolves nothing rather than guessing (precision over recall).
	src := []byte(`package demo

import (
	"net/http"
	"net/url"
)

func Get(endpointURL url.URL) error {
	req, err := http.NewRequest(http.MethodGet, endpointURL.String(), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	if calls := ExtractClientCalls("go", "demo.go", src); len(calls) != 0 {
		t.Fatalf("ExtractClientCalls = %+v, want none (base URL not visible)", calls)
	}
}

func TestExtractClientCalls_URLCompositeLiteralPath(t *testing.T) {
	src := []byte(`package demo

import (
	"net/http"
	"net/url"
)

func Get() error {
	u := url.URL{Scheme: "http", Host: "catalog-service", Path: "/items"}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	calls := ExtractClientCalls("go", "demo.go", src)
	if len(calls) != 1 {
		t.Fatalf("ExtractClientCalls = %+v, want exactly one call", calls)
	}
	if calls[0].Method != "GET" || calls[0].Template != "/items" {
		t.Errorf("call = %+v, want Method GET, Template /items", calls[0])
	}
}

func TestExtractClientCalls_ConstConcatenation(t *testing.T) {
	src := []byte(`package demo

import (
	"context"
	"net/http"
)

const basePath = "/system/function/"

func Fetch(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://svc"+basePath+id, nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}
`)
	calls := ExtractClientCalls("go", "demo.go", src)
	if len(calls) != 1 {
		t.Fatalf("ExtractClientCalls = %+v, want exactly one call", calls)
	}
	if calls[0].Method != "GET" || calls[0].Template != "/system/function/{}" {
		t.Errorf("call = %+v, want Method GET, Template /system/function/{}", calls[0])
	}
}

func TestExtractClientCalls_BareConcatenationOfTwoUnknownsIsUnresolved(t *testing.T) {
	// Neither side of the concatenation resolves to genuine literal
	// content (both a plain variable) — indistinguishable from arbitrary
	// string arithmetic, so this must NOT manufacture a call (regression
	// guard: an httptest-style `ts.URL + path` pattern, as found in
	// internal/fusion/testdata/reachfix/server_test.go, must stay silent).
	src := []byte(`package demo

import "net/http"

func Get(base, path string) error {
	resp, err := http.Get(base + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
`)
	if calls := ExtractClientCalls("go", "demo.go", src); len(calls) != 0 {
		t.Fatalf("ExtractClientCalls = %+v, want none (two unresolved fragments)", calls)
	}
}

func TestExtractClientCalls_Fixture(t *testing.T) {
	root := fauxtownRoot(t)
	src := readFixture(t, root, "checkout-service", "checkout.go")
	calls := ExtractClientCalls("go", "checkout-service/checkout.go", src)

	var got []struct{ Method, Template, Symbol string }
	for _, c := range calls {
		got = append(got, struct{ Method, Template, Symbol string }{c.Method, c.Template, c.Symbol})
	}
	want := []struct{ Method, Template, Symbol string }{
		{"GET", "/items/{}", "FetchItem"},
		{"DELETE", "/items/{}/hold", "HoldItem"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractClientCalls(checkout.go) = %+v, want %+v", got, want)
	}
}

// ---------------------------------------------------------------------
// Cross-repo join (fixture golden test)
// ---------------------------------------------------------------------

func fixtureRoutesAndCalls(t *testing.T) (map[string][]routes.Route, map[string][]ClientCall) {
	t.Helper()
	root := fauxtownRoot(t)
	catalogSrc := readFixture(t, root, "catalog-service", "catalog.go")
	checkoutSrc := readFixture(t, root, "checkout-service", "checkout.go")

	routesByService := map[string][]routes.Route{
		"catalog-service": routes.Extract("go", "catalog-service/catalog.go", catalogSrc),
	}
	callsByService := map[string][]ClientCall{
		"checkout-service": ExtractClientCalls("go", "checkout-service/checkout.go", checkoutSrc),
	}
	return routesByService, callsByService
}

func TestServiceRefEdges_FixtureJoin(t *testing.T) {
	routesByService, callsByService := fixtureRoutesAndCalls(t)

	hops := ServiceRefEdges(routesByService, callsByService)
	if len(hops) != 1 {
		t.Fatalf("ServiceRefEdges: got %d hops, want 1 (the decoy DELETE call must not match): %+v", len(hops), hops)
	}
	got := hops[0]
	want := Hop{
		FromService: "checkout-service",
		FromFile:    "checkout-service/checkout.go",
		FromLine:    got.FromLine, // asserted != 0 below; exact line is fixture-coupled and checked via FetchItem symbol instead
		FromSymbol:  "FetchItem",
		ToService:   "catalog-service",
		ToFile:      "catalog-service/catalog.go",
		ToLine:      got.ToLine,
		ToHandler:   "getItemHandler",
		Method:      "GET",
		Template:    "/items/{}",
		Ambiguous:   false,
		Candidates:  []string{"catalog-service"},
	}
	if got.FromLine == 0 || got.ToLine == 0 {
		t.Fatalf("hop line numbers must be nonzero: %+v", got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hop = %+v, want %+v", got, want)
	}
}

func TestServiceRefEdges_NoIntraServiceHop(t *testing.T) {
	// A client call and a matching route in the SAME service must never
	// join — that is an ordinary intra-service call, not a cross-repo
	// service_ref edge.
	routesByService := map[string][]routes.Route{
		"solo-service": {{Method: "GET", Path: "/items/:id", Handler: "h", File: "solo.go", Line: 1}},
	}
	callsByService := map[string][]ClientCall{
		"solo-service": {{Method: "GET", Template: "/items/{}", File: "solo.go", Line: 5}},
	}
	if hops := ServiceRefEdges(routesByService, callsByService); len(hops) != 0 {
		t.Errorf("ServiceRefEdges: same-service call+route produced %d hops, want 0: %+v", len(hops), hops)
	}
}

func TestServiceRefEdges_FanOutToMultipleServices(t *testing.T) {
	// A client call matching routes in TWO different services fans out to
	// both — the true blast radius, mirroring platformlink's own
	// documented fan-out behavior for ordinary cross-repo call edges.
	routesByService := map[string][]routes.Route{
		"svc-a": {{Method: "GET", Path: "/items/:id", Handler: "ha", File: "a.go", Line: 1}},
		"svc-b": {{Method: "GET", Path: "/items/:id", Handler: "hb", File: "b.go", Line: 2}},
	}
	callsByService := map[string][]ClientCall{
		"client-svc": {{Method: "GET", Template: "/items/{}", File: "c.go", Line: 3}},
	}
	hops := ServiceRefEdges(routesByService, callsByService)
	if len(hops) != 2 {
		t.Fatalf("ServiceRefEdges: got %d hops, want 2: %+v", len(hops), hops)
	}
	if hops[0].ToService != "svc-a" || hops[1].ToService != "svc-b" {
		t.Errorf("hops not sorted by ToService: %+v", hops)
	}
	// A key matched by more than one service is fan-out, not dropped —
	// AND each resulting hop is marked ambiguous with the full candidate
	// set, so a consumer can tell this apart from a unique join.
	wantCandidates := []string{"svc-a", "svc-b"}
	for _, h := range hops {
		if !h.Ambiguous {
			t.Errorf("hop %+v: Ambiguous = false, want true (two services register the same route)", h)
		}
		if !reflect.DeepEqual(h.Candidates, wantCandidates) {
			t.Errorf("hop %+v: Candidates = %v, want %v", h, h.Candidates, wantCandidates)
		}
	}
}

func TestServiceRefEdges_UniqueJoinIsNotAmbiguous(t *testing.T) {
	routesByService := map[string][]routes.Route{
		"svc-a": {{Method: "GET", Path: "/items/:id", Handler: "ha", File: "a.go", Line: 1}},
	}
	callsByService := map[string][]ClientCall{
		"client-svc": {{Method: "GET", Template: "/items/{}", File: "c.go", Line: 3}},
	}
	hops := ServiceRefEdges(routesByService, callsByService)
	if len(hops) != 1 {
		t.Fatalf("got %d hops, want 1: %+v", len(hops), hops)
	}
	if hops[0].Ambiguous {
		t.Errorf("hop = %+v, want Ambiguous false (only one service registers this route)", hops[0])
	}
	if want := []string{"svc-a"}; !reflect.DeepEqual(hops[0].Candidates, want) {
		t.Errorf("Candidates = %v, want %v", hops[0].Candidates, want)
	}
}

func TestServiceRefEdges_Deterministic(t *testing.T) {
	routesByService, callsByService := fixtureRoutesAndCalls(t)
	first := ServiceRefEdges(routesByService, callsByService)
	for i := 0; i < 20; i++ {
		again := ServiceRefEdges(routesByService, callsByService)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("ServiceRefEdges is not deterministic across repeated calls (iteration %d):\nfirst=%+v\nagain=%+v", i, first, again)
		}
	}
}

// ---------------------------------------------------------------------
// core.CrossRepoEdge / Payload rendering
// ---------------------------------------------------------------------

func TestHop_CrossRepoEdge(t *testing.T) {
	h := Hop{
		FromService: "checkout-service", FromFile: "checkout.go",
		ToService: "catalog-service", ToFile: "catalog.go",
		Method: "GET", Template: "/items/{}",
	}
	got := h.CrossRepoEdge()
	want := core.CrossRepoEdge{
		FromService: "checkout-service", FromFile: "checkout.go",
		ToService: "catalog-service", ToKey: "GET /items/{}", ToFile: "catalog.go",
		Kind: core.CrossRepoEdgeServiceRef,
	}
	if got != want {
		t.Errorf("Hop.CrossRepoEdge() = %+v, want %+v", got, want)
	}
	if core.CrossRepoEdgeConfidenceOf(got) != core.EdgeConfidenceHeuristic {
		t.Errorf("CrossRepoEdgeConfidenceOf(service_ref edge) = %q, want %q",
			core.CrossRepoEdgeConfidenceOf(got), core.EdgeConfidenceHeuristic)
	}
}

func TestHop_Payload(t *testing.T) {
	h := Hop{
		FromService: "checkout-service", ToService: "catalog-service", Method: "GET", Template: "/items/{}",
		Ambiguous: true, Candidates: []string{"catalog-service", "other-service"},
	}
	p := h.Payload()
	if p["kind"] != "service_ref" {
		t.Errorf("Payload()[kind] = %v, want service_ref", p["kind"])
	}
	if p["tier"] != "heuristic" {
		t.Errorf("Payload()[tier] = %v, want heuristic", p["tier"])
	}
	if p["ambiguous"] != true {
		t.Errorf("Payload()[ambiguous] = %v, want true", p["ambiguous"])
	}
	if !reflect.DeepEqual(p["candidates"], []string{"catalog-service", "other-service"}) {
		t.Errorf("Payload()[candidates] = %v, want [catalog-service other-service]", p["candidates"])
	}
}

func TestKindAndTierConstants(t *testing.T) {
	if Kind != "service_ref" {
		t.Errorf("Kind = %q, want service_ref", Kind)
	}
	if Tier != "heuristic" {
		t.Errorf("Tier = %q, want heuristic", Tier)
	}
}

// ---------------------------------------------------------------------
// BuildFromStore
// ---------------------------------------------------------------------

type fakeSourceLister map[string][]core.SourceFile

func (f fakeSourceLister) ScanSources(ctx context.Context, service string, b corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	return corpus.ScanSourceFiles(ctx, f[service], b, fn)
}

func TestBuildFromStore(t *testing.T) {
	root := fauxtownRoot(t)
	catalogSrc := readFixture(t, root, "catalog-service", "catalog.go")
	checkoutSrc := readFixture(t, root, "checkout-service", "checkout.go")

	backend := fakeSourceLister{
		"catalog-service":  {{Path: "catalog.go", Content: catalogSrc, Lang: "go"}},
		"checkout-service": {{Path: "checkout.go", Content: checkoutSrc, Lang: "go"}},
	}

	hops, err := BuildFromStore(context.Background(), backend, []string{"checkout-service", "catalog-service"})
	if err != nil {
		t.Fatalf("BuildFromStore: %v", err)
	}
	if len(hops) != 1 {
		t.Fatalf("BuildFromStore: got %d hops, want 1: %+v", len(hops), hops)
	}
	if hops[0].FromService != "checkout-service" || hops[0].ToService != "catalog-service" {
		t.Errorf("hop = %+v, want checkout-service -> catalog-service", hops[0])
	}
}

func TestBuildFromStore_UnknownServiceIsEmptyNotError(t *testing.T) {
	backend := fakeSourceLister{}
	hops, err := BuildFromStore(context.Background(), backend, []string{"nowhere"})
	if err != nil {
		t.Fatalf("BuildFromStore: %v", err)
	}
	if len(hops) != 0 {
		t.Errorf("BuildFromStore(unknown service) = %+v, want empty", hops)
	}
}
