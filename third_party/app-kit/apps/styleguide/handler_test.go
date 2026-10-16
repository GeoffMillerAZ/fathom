package styleguide_test

// Real HTTP and artifact checks only. These do not establish browser CSP,
// layout, font fidelity, keyboard behavior or adoption in a consumer app.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/styleguide"
	"golang.org/x/net/html"
)

const guideBase = "/dev/theme"
const guideSource = "../../cue/tokens/kit.cue"
const syntheticLabel = `<img src=x onerror="alert(1)"> & synthetic`

type guideFixture struct {
	server  *httptest.Server
	handler *styleguide.Handler
	engine  *styleguide.Engine
	options styleguide.Options
	source  []byte
	appCSS  string
}

func newGuideFixture(t *testing.T, configure func(*styleguide.Options), secure bool) *guideFixture {
	t.Helper()
	source, err := os.ReadFile(guideSource)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{
		Source: source, SourceLabel: "Synthetic <source> & fixture", KitRevision: "synthetic-kit-test-revision",
		Fonts:  map[string]string{"sans-test": "system-ui, sans-serif", "mono-test": "ui-monospace, monospace"},
		Checks: []styleguide.ContrastCheck{{ID: "body-light", Foreground: "ink", Background: "surface", Scheme: "light", Minimum: 4.5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	t.Cleanup(server.Close)
	scheme := "http"
	if secure {
		scheme = "https"
	}
	options := styleguide.Options{
		Engine: engine, BasePath: guideBase, Origin: scheme + "://" + server.Listener.Addr().String(),
		Title: "Synthetic <guide> & app", Stylesheets: []string{"/app.css"},
		Specimens: []styleguide.Specimen{{ID: "work-queue", Name: "Queue <specimen> & labels", Render: func(_ context.Context, w io.Writer) error {
			_, err := io.WriteString(w, stat.RenderBand([]stat.Stat{{Label: syntheticLabel, Value: "42", Sub: "synthetic tickets", Delta: "+2"}}, stat.BandOptions{Title: "Synthetic work queue", AriaLabel: "Synthetic queue summary"}))
			return err
		}}},
	}
	if configure != nil {
		configure(&options)
	}
	handler, err := styleguide.New(options)
	if err != nil {
		t.Fatal(err)
	}
	appCSS := tokens.ThemeCSS() + "\n.text-ink{color:var(--ak-color-ink)}.font-sans{font-family:var(--ak-font-sans)}"
	mux := http.NewServeMux()
	mux.Handle(guideBase, handler)
	mux.Handle(guideBase+"/", handler)
	mux.HandleFunc("/app", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-App-Sentinel", "unchanged")
		_, _ = io.WriteString(w, "normal app, unchanged")
	})
	mux.HandleFunc("/app.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = io.WriteString(w, appCSS)
	})
	server.Config.Handler = mux
	if secure {
		server.StartTLS()
	} else {
		server.Start()
	}
	if server.URL != options.Origin {
		t.Fatal("test did not bind the actual listener origin")
	}
	return &guideFixture{server, handler, engine, options, source, appCSS}
}

func (f *guideFixture) client() *http.Client {
	return &http.Client{Transport: f.server.Client().Transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

type guideResponse struct {
	status int
	header http.Header
	body   []byte
}

func guideRequest(t *testing.T, f *guideFixture, client *http.Client, method, target, body string, headers http.Header, host string) guideResponse {
	t.Helper()
	r, err := http.NewRequest(method, f.server.URL+target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header = headers.Clone()
	if host != "" {
		r.Host = host
	}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil || len(data) == 2<<20 {
		t.Fatal("response unreadable or exceeds test bound", err)
	}
	if len(response.Header.Values("Set-Cookie")) != 0 {
		t.Fatal("guide unexpectedly saved cookie state")
	}
	return guideResponse{response.StatusCode, response.Header.Clone(), data}
}

func postHeaders(f *guideFixture) http.Header {
	return http.Header{"Origin": {f.server.URL}, "Content-Type": {"application/x-www-form-urlencoded"}}
}

func wantStatus(t *testing.T, response guideResponse, status int) {
	t.Helper()
	if response.status != status {
		t.Fatalf("status %d, want %d (body %d bytes)", response.status, status, len(response.body))
	}
	policy := "no-referrer"
	if bytes.Contains(response.body, []byte(`<form action="`+guideBase+`"`)) {
		policy = "same-origin"
	}
	if response.header.Get("Cache-Control") != "no-store" || response.header.Get("Referrer-Policy") != policy || response.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("guide privacy headers missing")
	}
}

func guideDOM(t *testing.T, body []byte) *html.Node {
	t.Helper()
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func attribute(n *html.Node, name string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

func matching(root *html.Node, tag, key, value string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			v, ok := attribute(n, key)
			if key == "" || (ok && v == value) {
				found = append(found, n)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func only(t *testing.T, root *html.Node, tag, key, value string) *html.Node {
	t.Helper()
	found := matching(root, tag, key, value)
	if len(found) != 1 {
		t.Fatalf("%s[%s=%q]: got %d nodes, want 1", tag, key, value, len(found))
	}
	return found[0]
}

func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var out strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(nodeText(child))
	}
	return out.String()
}

// Read the actual HTML form, including browser-decoded attributes and selects.
func guideForm(t *testing.T, body []byte) url.Values {
	t.Helper()
	form := only(t, guideDOM(t, body), "form", "action", guideBase)
	values := url.Values{}
	for _, input := range matching(form, "input", "", "") {
		name, ok := attribute(input, "name")
		if ok {
			value, _ := attribute(input, "value")
			values.Add(name, value)
		}
	}
	for _, selectNode := range matching(form, "select", "", "") {
		name, _ := attribute(selectNode, "name")
		options := matching(selectNode, "option", "", "")
		if len(options) == 0 {
			t.Fatal("select lacks options")
		}
		choice := options[0]
		for _, option := range options {
			if _, selected := attribute(option, "selected"); selected {
				choice = option
			}
		}
		value, _ := attribute(choice, "value")
		values.Add(name, value)
	}
	return values
}

func previewURL(t *testing.T, body []byte) string {
	t.Helper()
	frame := only(t, guideDOM(t, body), "iframe", "", "")
	if value, ok := attribute(frame, "sandbox"); !ok || value != "" {
		t.Fatal("preview frame sandbox widened")
	}
	if value, _ := attribute(frame, "referrerpolicy"); value != "no-referrer" {
		t.Fatal("frame referrer policy missing")
	}
	target, _ := attribute(frame, "src")
	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path != guideBase+"/preview" {
		t.Fatal("preview escaped mounted guide")
	}
	return target
}

func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func expectedSnapshot(t *testing.T, value tokens.Tokens) []byte {
	t.Helper()
	body, err := json.MarshalIndent(struct {
		Tokens tokens.Tokens `json:"tokens"`
	}{value}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(body, '\n')
}

func TestMountedStyleGuideHTTPPreviewExportResetAndClientIsolation(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	clientA, clientB := f.client(), f.client()
	appBefore := guideRequest(t, f, clientA, "GET", "/app", "", nil, "")
	cssBefore := guideRequest(t, f, clientA, "GET", guideBase+"/style.css", "", nil, "")
	baseline, err := tokens.Load(f.source)
	if err != nil {
		t.Fatal(err)
	}
	initial := guideRequest(t, f, clientA, "GET", guideBase, "", nil, "")
	wantStatus(t, initial, 200)
	assertParent(t, initial)
	form := guideForm(t, initial.body)
	if form.Get("review_digest") != "" || form.Get("colors.ink.light") != "" || form.Get("fonts.sans") != "" {
		t.Fatal("initial form inherited a candidate")
	}
	if !bytes.Contains(initial.body, []byte(bodyHash(f.source))) || !bytes.Contains(initial.body, []byte(bodyHash(expectedSnapshot(t, baseline)))) {
		t.Fatal("initial source or concrete candidate identity missing")
	}
	doc := guideDOM(t, initial.body)
	if nodeText(only(t, doc, "h1", "", "")) != f.options.Title || len(matching(doc, "guide", "", "")) != 0 || len(matching(doc, "source", "", "")) != 0 {
		t.Fatal("app metadata was not escaped")
	}
	basePreview := guideRequest(t, f, clientA, "GET", previewURL(t, initial.body), "", nil, "")
	wantStatus(t, basePreview, 200)
	if basePreview.header.Get("X-Theme-Candidate-SHA256") != bodyHash(expectedSnapshot(t, baseline)) {
		t.Fatal("initial iframe not bound to baseline snapshot")
	}
	edits := map[string]string{"colors.ink.light": "#000000", "colors.surface.light": "#ffffff", "fonts.sans": "sans-test", "radius.md": "1.5rem"}
	for key, value := range edits {
		form.Set(key, value)
	}
	form.Set("action", "preview")
	preview := guideRequest(t, f, clientA, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, preview, 200)
	assertParent(t, preview)
	acknowledged := guideForm(t, preview.body)
	digest := acknowledged.Get("review_digest")
	if decoded, err := hex.DecodeString(digest); err != nil || len(decoded) != sha256.Size {
		t.Fatal("preview lacks bounded review digest")
	}
	wantTokens := baseline
	wantTokens.Colors = maps.Clone(baseline.Colors)
	ink, surface := wantTokens.Colors["ink"], wantTokens.Colors["surface"]
	ink.Light, surface.Light = "#000000", "#ffffff"
	wantTokens.Colors["ink"], wantTokens.Colors["surface"] = ink, surface
	wantTokens.Fonts.Sans, wantTokens.Radius.MD = "system-ui, sans-serif", "1.5rem"
	wantCUE := expectedSnapshot(t, wantTokens)
	wantCSS := []byte(tokens.EmitThemeCSSOpts(wantTokens, tokens.EmitOptions{DarkVariant: tokens.DarkVariantNone}))
	target := previewURL(t, preview.body)
	u, _ := url.Parse(target)
	rawDraft, err := base64.RawURLEncoding.DecodeString(u.Query().Get("draft"))
	var gotEdits map[string]string
	if err != nil || json.Unmarshal(rawDraft, &gotEdits) != nil || !reflect.DeepEqual(gotEdits, edits) {
		t.Fatal("iframe draft is not the exact submitted bounded edit set")
	}
	child := guideRequest(t, f, clientA, "GET", target, "", nil, "")
	wantStatus(t, child, 200)
	if child.header.Get("X-Theme-Candidate-SHA256") != bodyHash(wantCUE) || !bytes.Contains(preview.body, []byte(bodyHash(wantCUE))) {
		t.Fatal("page and child candidate hashes disagree with independent snapshot")
	}
	assertChild(t, child, wantCSS)
	stale := maps.Clone(acknowledged)
	stale.Set("radius.md", "2rem")
	stale.Set("action", "export")
	refused := guideRequest(t, f, clientA, "POST", guideBase, stale.Encode(), postHeaders(f), "")
	wantStatus(t, refused, 409)
	if strings.Contains(refused.header.Get("Content-Type"), "zip") || guideForm(t, refused.body).Get("review_digest") != "" {
		t.Fatal("stale review exported or silently acquired a new acknowledgement")
	}
	acknowledged.Set("action", "export")
	export := guideRequest(t, f, clientA, "POST", guideBase, acknowledged.Encode(), postHeaders(f), "")
	wantStatus(t, export, 200)
	if export.header.Get("Content-Type") != "application/zip" || export.header.Get("X-Content-SHA256") != bodyHash(export.body) || !strings.Contains(export.header.Get("Content-Disposition"), `filename="app-theme-candidate.zip"`) {
		t.Fatal("download type, name or content identity missing")
	}
	bundle := guideBundle(t, export.body)
	if !bytes.Equal(bundle["original.cue"], f.source) || !bytes.Equal(bundle["candidate.cue"], wantCUE) || !bytes.Equal(bundle["candidate.css"], wantCSS) || bodyHash(bundle["receipt.json"]) != digest {
		t.Fatal("export lost original bytes or differs from acknowledged candidate")
	}
	var changes []styleguide.Change
	if json.Unmarshal(bundle["diff.json"], &changes) != nil || !reflect.DeepEqual(changes, []styleguide.Change{
		{Path: "colors.ink.light", Before: baseline.Colors["ink"].Light, After: "#000000"},
		{Path: "colors.surface.light", Before: baseline.Colors["surface"].Light, After: "#ffffff"},
		{Path: "fonts.sans", Before: baseline.Fonts.Sans, After: "system-ui, sans-serif"},
		{Path: "radius.md", Before: baseline.Radius.MD, After: "1.5rem"},
	}) {
		t.Fatal("semantic diff lost, invented or misordered changes")
	}
	assertReceipt(t, bundle)
	other := guideRequest(t, f, clientB, "GET", guideBase, "", nil, "")
	wantStatus(t, other, 200)
	otherForm := guideForm(t, other.body)
	if otherForm.Get("review_digest") != "" || otherForm.Get("radius.md") != "" || previewURL(t, other.body) != previewURL(t, initial.body) {
		t.Fatal("client B inherited client A's draft")
	}
	otherForm.Set("radius.md", "3rem")
	otherForm.Set("action", "preview")
	otherPreview := guideRequest(t, f, clientB, "POST", guideBase, otherForm.Encode(), postHeaders(f), "")
	wantStatus(t, otherPreview, 200)
	if guideForm(t, otherPreview.body).Get("review_digest") == digest {
		t.Fatal("distinct client candidate reused acknowledgement")
	}
	repeated := guideRequest(t, f, clientA, "POST", guideBase, acknowledged.Encode(), postHeaders(f), "")
	wantStatus(t, repeated, 200)
	if !bytes.Equal(export.body, repeated.body) {
		t.Fatal("same acknowledged export changed after another client's preview")
	}
	reset := guideRequest(t, f, clientA, "GET", guideBase, "", nil, "")
	wantStatus(t, reset, 200)
	if !bytes.Equal(initial.body, reset.body) {
		t.Fatal("reset failed to restore exact source view")
	}
	appAfter := guideRequest(t, f, clientA, "GET", "/app", "", nil, "")
	appStyleAfter := guideRequest(t, f, clientA, "GET", "/app.css", "", nil, "")
	cssAfter := guideRequest(t, f, clientA, "GET", guideBase+"/style.css", "", nil, "")
	sourceAfter, err := os.ReadFile(guideSource)
	if err != nil || !bytes.Equal(sourceAfter, f.source) || f.engine.SourceSHA256() != bodyHash(f.source) || !bytes.Equal(appBefore.body, appAfter.body) || appAfter.header.Get("X-App-Sentinel") != "unchanged" || string(appStyleAfter.body) != f.appCSS || !bytes.Equal(cssBefore.body, cssAfter.body) {
		t.Fatal("guide changed source, normal app or stable editor styles")
	}
}

func TestStyleGuideShowingSourceDoesNotAcknowledgeNewEdits(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	initial := guideRequest(t, f, client, "GET", guideBase, "", nil, "")
	form := guideForm(t, initial.body)
	form.Set("radius.md", "1rem")
	form.Set("action", "baseline")
	sourceView := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, sourceView, 200)
	if guideForm(t, sourceView.body).Get("review_digest") != "" || strings.Contains(previewURL(t, sourceView.body), "draft=") {
		t.Fatal("showing source acknowledged or rendered a fresh candidate")
	}
	form.Set("action", "preview")
	preview := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, preview, 200)
	form = guideForm(t, preview.body)
	digest := form.Get("review_digest")
	if digest == "" {
		t.Fatal("candidate preview did not acknowledge its exact draft")
	}
	form.Set("action", "baseline")
	sourceView = guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, sourceView, 200)
	if guideForm(t, sourceView.body).Get("review_digest") != digest || strings.Contains(previewURL(t, sourceView.body), "draft=") {
		t.Fatal("unchanged acknowledged candidate was lost when viewing source")
	}
	form.Set("radius.md", "2rem")
	sourceView = guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, sourceView, 200)
	if guideForm(t, sourceView.body).Get("review_digest") != "" || guideForm(t, sourceView.body).Get("radius.md") != "2rem" {
		t.Fatal("edited candidate retained stale acknowledgement or lost edits")
	}
}

func TestStyleGuideOversizedPreviewRetainsValidEditsWithoutAcknowledgement(t *testing.T) {
	source, err := os.ReadFile(guideSource)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := tokens.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{}
	for i := 0; len(baseline.Colors) < 128; i++ {
		role := fmt.Sprintf("extra-%03d-%s", i, strings.Repeat("x", 54))
		baseline.Colors[role] = tokens.ColorPair{Light: "#112233", Dark: "#112233"}
		edits["colors."+role+".light"], edits["colors."+role+".dark"] = "#223344", "#223344"
	}
	engine, err := styleguide.NewEngine(styleguide.EngineOptions{
		Source: expectedSnapshot(t, baseline), SourceLabel: "Synthetic large token set", KitRevision: "synthetic-large-revision",
		Checks: []styleguide.ContrastCheck{{ID: "body-light", Foreground: "ink", Background: "surface", Scheme: "light", Minimum: 4.5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidate, err := engine.Prepare(edits); err != nil || !candidate.Exportable || len(candidate.Changes) != len(edits) {
		t.Fatal("overflow fixture is not otherwise a valid passing candidate", err)
	}
	raw, _ := json.Marshal(edits)
	if len(raw) <= 12<<10 || len(edits) < 200 {
		t.Fatal("fixture does not exercise the smaller preview capsule bound")
	}
	f := newGuideFixture(t, func(o *styleguide.Options) { o.Engine = engine }, false)
	client := f.client()
	initial := guideRequest(t, f, client, "GET", guideBase, "", nil, "")
	wantStatus(t, initial, 200)
	form := guideForm(t, initial.body)
	for key, value := range edits {
		form.Set(key, value)
	}
	form.Set("action", "preview")
	if len(form.Encode()) > 64<<10 {
		t.Fatal("fixture exceeds the form limit instead of only the preview limit")
	}
	response := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, response, 422)
	preserved := guideForm(t, response.body)
	for key, value := range edits {
		if preserved.Get(key) != value {
			t.Fatal("preview overflow discarded a valid edit", key)
		}
	}
	doc := guideDOM(t, response.body)
	if preserved.Get("review_digest") != "" || !strings.Contains(nodeText(doc), "12KiB preview budget") || strings.Contains(previewURL(t, response.body), "draft=") {
		t.Fatal("preview overflow was not an explicit unacknowledged baseline recovery")
	}
	if _, disabled := attribute(only(t, doc, "button", "value", "export"), "disabled"); !disabled {
		t.Fatal("oversized unpreviewed draft enabled export")
	}
	diff := only(t, doc, "h2", "id", "sg-diff").Parent
	if len(matching(only(t, diff, "tbody", "", ""), "tr", "", "")) != len(edits) {
		t.Fatal("candidate diff lost changes after preview refusal")
	}
}

func assertParent(t *testing.T, response guideResponse) {
	t.Helper()
	doc := guideDOM(t, response.body)
	if len(matching(doc, "style", "", "")) != 0 || len(matching(doc, "script", "", "")) != 0 || len(matching(doc, "main", "", "")) != 1 || len(matching(doc, "link", "", "")) != 1 {
		t.Fatal("parent gained candidate styles, scripts or duplicate landmarks")
	}
	only(t, doc, "link", "href", guideBase+"/style.css")
	policy := response.header.Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'none'", "script-src 'none'", "style-src 'self'", "frame-src 'self'", "form-action 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(policy, directive) {
			t.Error("parent CSP missing", directive)
		}
	}
	if strings.Contains(policy, "unsafe-") || response.header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("candidate preview widened parent permissions")
	}
}

func assertChild(t *testing.T, response guideResponse, css []byte) {
	t.Helper()
	doc := guideDOM(t, response.body)
	style := only(t, doc, "style", "", "")
	if !strings.HasPrefix(nodeText(style), string(css)) || len(matching(doc, "main", "", "")) != 1 || len(matching(doc, "script", "", "")) != 0 || len(matching(doc, "img", "", "")) != 0 || !strings.Contains(nodeText(doc), syntheticLabel) || !strings.Contains(nodeText(doc), "42") {
		t.Fatal("real escaped stat specimen or exact candidate CSS missing")
	}
	link := only(t, doc, "link", "href", "/app.css")
	if link.Parent != style.Parent {
		t.Fatal("app CSS and candidate CSS are not in the same document head")
	}
	before := false
	for node := link.NextSibling; node != nil; node = node.NextSibling {
		before = before || node == style
	}
	if !before {
		t.Fatal("app CSS loaded after candidate CSS")
	}
	hash := sha256.Sum256([]byte(nodeText(style)))
	for _, want := range []string{"script-src 'none'", "font-src 'none'", "img-src 'none'", "form-action 'none'", "frame-ancestors 'self'", "; sandbox", "style-src-attr 'unsafe-inline'", "'sha256-" + base64.StdEncoding.EncodeToString(hash[:]) + "'"} {
		if !strings.Contains(response.header.Get("Content-Security-Policy"), want) {
			t.Error("child CSP missing", want)
		}
	}
	if response.header.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal("child frame policy missing")
	}
}

func guideBundle(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"original.cue", "candidate.cue", "candidate.css", "diff.json", "receipt.json"}
	files := map[string][]byte{}
	for _, file := range archive.File {
		if !slices.Contains(want, file.Name) || files[file.Name] != nil || file.Mode().Perm() != 0600 {
			t.Fatal("unexpected, duplicate or nonprivate archive member", file.Name)
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(io.LimitReader(r, 1<<20))
		_ = r.Close()
		if err != nil || len(data) == 1<<20 {
			t.Fatal("invalid archive member", file.Name)
		}
		files[file.Name] = data
	}
	if len(files) != len(want) {
		t.Fatal("incomplete review bundle")
	}
	return files
}

func assertReceipt(t *testing.T, bundle map[string][]byte) {
	t.Helper()
	var receipt struct {
		SchemaVersion int    `json:"schema_version"`
		Scope         string `json:"scope"`
		KitRevision   string `json:"kit_revision"`
		Exportable    bool   `json:"exportable"`
		Definitions   []struct {
			ID, Foreground, Background, Scheme string
			Minimum                            float64
		} `json:"check_definitions"`
		Files map[string]struct {
			SHA256 string `json:"sha256"`
			Size   int    `json:"size"`
		} `json:"files"`
		Checks []struct {
			ID, Scheme string
			Ratio      float64
			Minimum    float64
			Passed     bool
			Error      string
		} `json:"checks"`
	}
	if err := json.Unmarshal(bundle["receipt.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != 1 || receipt.Scope != "experimental-app-owned-styleguide" || receipt.KitRevision != "synthetic-kit-test-revision" || !receipt.Exportable || len(receipt.Files) != 4 || len(receipt.Checks) != 1 || len(receipt.Definitions) != 1 {
		t.Fatal("receipt changed declared scope, source or check denominator")
	}
	definition := receipt.Definitions[0]
	if definition.ID != "body-light" || definition.Foreground != "ink" || definition.Background != "surface" || definition.Scheme != "light" || definition.Minimum != 4.5 {
		t.Fatal("receipt changed app-selected check definition")
	}
	for name, identity := range receipt.Files {
		if name == "receipt.json" || bundle[name] == nil || identity.SHA256 != bodyHash(bundle[name]) || identity.Size != len(bundle[name]) {
			t.Fatal("receipt identity does not match downloaded member", name)
		}
	}
	check := receipt.Checks[0]
	// Independent black/white oracle: exact 21:1, not the engine's own verdict.
	if check.ID != "body-light" || check.Scheme != "light" || check.Ratio != 21 || check.Minimum != 4.5 || !check.Passed || check.Error != "" {
		t.Fatal("selected contrast result disagrees with black/white oracle")
	}
}

func TestStyleGuideFailedContrastAndInvalidEditsNeverExport(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	initial := guideRequest(t, f, client, "GET", guideBase, "", nil, "")
	form := guideForm(t, initial.body)
	form.Set("colors.ink.light", "#ffffff")
	form.Set("colors.surface.light", "#ffffff")
	form.Set("action", "preview")
	preview := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, preview, 200)
	doc := guideDOM(t, preview.body)
	button := only(t, doc, "button", "value", "export")
	if _, disabled := attribute(button, "disabled"); !disabled || !strings.Contains(nodeText(doc), "1.00 / 4.50") || !strings.Contains(nodeText(doc), "Needs review") {
		t.Fatal("equal-color failure did not remain visible and disable export")
	}
	form = guideForm(t, preview.body)
	form.Set("action", "export")
	refused := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, refused, 409)
	if strings.Contains(refused.header.Get("Content-Type"), "zip") {
		t.Fatal("failed contrast exported despite current acknowledgement")
	}
	for key, value := range map[string]string{"colors.ink.light": `#fff; background:url(https://example.com)`, "fonts.sans": "unapproved", "radius.md": "-1px"} {
		t.Run(key, func(t *testing.T) {
			bad := guideForm(t, initial.body)
			bad.Set(key, value)
			bad.Set("action", "preview")
			response := guideRequest(t, f, client, "POST", guideBase, bad.Encode(), postHeaders(f), "")
			wantStatus(t, response, 422)
			if guideForm(t, response.body).Get("review_digest") != "" || strings.Contains(previewURL(t, response.body), "draft=") {
				t.Fatal("invalid edit acquired acknowledgement or active draft")
			}
		})
	}
}

func TestStyleGuideHTTPFormAndRouteRefusals(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	valid := url.Values{"action": {"preview"}, "specimen": {"work-queue"}, "scheme": {"light"}, "viewport": {"wide"}}
	for _, body := range []string{valid.Encode() + "&unknown=x", valid.Encode() + "&action=export", valid.Encode() + "&radius.md=1px&radius.md=2px", valid.Encode() + "&radius.md=%zz", valid.Encode() + "&radius.md=" + strings.Repeat("1", 513), valid.Encode() + "&unknown=" + strings.Repeat("x", 64<<10)} {
		response := guideRequest(t, f, client, "POST", guideBase, body, postHeaders(f), "")
		wantStatus(t, response, 400)
		assertGuideDenied(t, response)
	}
	for key, value := range map[string]string{"action": "save", "specimen": "missing", "scheme": "system", "viewport": "1440"} {
		bad := maps.Clone(valid)
		bad.Set(key, value)
		response := guideRequest(t, f, client, "POST", guideBase, bad.Encode(), postHeaders(f), "")
		wantStatus(t, response, 400)
		bad.Del(key)
		wantStatus(t, guideRequest(t, f, client, "POST", guideBase, bad.Encode(), postHeaders(f), ""), 400)
	}
	for _, media := range []string{"", "application/json", "multipart/form-data; boundary=x", "application/x-www-form-urlencoded; charset=utf-16", "application/x-www-form-urlencoded; other=x"} {
		headers := postHeaders(f)
		headers.Set("Content-Type", media)
		wantStatus(t, guideRequest(t, f, client, "POST", guideBase, valid.Encode(), headers, ""), 415)
	}
	utf8 := postHeaders(f)
	utf8.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	wantStatus(t, guideRequest(t, f, client, "POST", guideBase, valid.Encode(), utf8, ""), 200)
	for _, target := range []string{guideBase + "?x=1", guideBase + "?x=%zz", guideBase + "/style.css?x=1", "/dev/%74heme", guideBase + "/%73tyle.css"} {
		response := guideRequest(t, f, client, "GET", target, "", nil, "")
		wantStatus(t, response, 400)
		assertGuideDenied(t, response)
	}
	for _, target := range []string{guideBase + "/", guideBase + "/missing", guideBase + "/preview/"} {
		wantStatus(t, guideRequest(t, f, client, "GET", target, "", nil, ""), 404)
	}
	for _, target := range []string{guideBase, guideBase + "/style.css", guideBase + "/preview"} {
		for _, method := range []string{"HEAD", "PUT", "DELETE", "OPTIONS"} {
			response := guideRequest(t, f, client, method, target, "", nil, "")
			wantStatus(t, response, 405)
			assertGuideDenied(t, response)
			if response.header.Get("Allow") == "" {
				t.Fatal("method refusal lacks Allow")
			}
		}
	}
	for _, target := range []string{guideBase + "/style.css", guideBase + "/preview"} {
		wantStatus(t, guideRequest(t, f, client, "POST", target, valid.Encode(), postHeaders(f), ""), 405)
	}
	asset := guideRequest(t, f, client, "GET", guideBase+"/style.css", "", nil, "")
	wantStatus(t, asset, 200)
	if !strings.HasPrefix(asset.header.Get("Content-Type"), "text/css") || !bytes.HasPrefix(asset.body, []byte(tokens.ThemeCSS())) || !bytes.Contains(asset.body, []byte(".sg")) {
		t.Fatal("fixed editor asset is not the kit theme plus guide chrome")
	}
}

func TestStyleGuidePreviewDraftRefusals(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	baseQuery := "specimen=work-queue&scheme=light"
	for _, query := range []string{"", "specimen=missing&scheme=light", "specimen=work-queue&scheme=system", baseQuery + "&scheme=dark", baseQuery + "&unknown=x", baseQuery + "&draft=%zz", baseQuery + "&draft=%%%", baseQuery + "&draft=" + strings.Repeat("x", 21<<10)} {
		response := guideRequest(t, f, client, "GET", guideBase+"/preview?"+query, "", nil, "")
		wantStatus(t, response, 400)
		assertGuideDenied(t, response)
	}
	for _, tc := range []struct {
		name, raw string
		status    int
	}{
		{"null", "null", 400}, {"array", "[]", 400}, {"wrong-type", `{"radius.md":1}`, 400},
		{"duplicate", `{"radius.md":"1px","radius.md":"2px"}`, 400},
		{"whitespace", `{ "radius.md": "1px" }`, 400},
		{"noncanonical-order", `{"radius.md":"1px","fonts.sans":"sans-test"}`, 400},
		{"oversized-decoded", `{"radius.md":"` + strings.Repeat("1", 13<<10) + `"}`, 400},
		{"unknown", `{"tokens":"replacement CUE"}`, 422}, {"unsafe-color", `{"colors.ink.light":"url(https://example.com)"}`, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded := base64.RawURLEncoding.EncodeToString([]byte(tc.raw))
			response := guideRequest(t, f, client, "GET", guideBase+"/preview?"+baseQuery+"&draft="+encoded, "", nil, "")
			wantStatus(t, response, tc.status)
			assertGuideDenied(t, response)
		})
	}
	for _, suffix := range []string{"=", "%0A"} {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + suffix
		wantStatus(t, guideRequest(t, f, client, "GET", guideBase+"/preview?"+baseQuery+"&draft="+encoded, "", nil, ""), 400)
	}
}

func TestStyleGuideNativeSectionsAndErrorRecoveryRetainVisibleEdits(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	initial := guideRequest(t, f, client, "GET", guideBase, "", nil, "")
	doc := guideDOM(t, initial.body)
	for _, id := range []string{"sg-editor", "sg-review", "sg-export"} {
		only(t, doc, "a", "href", "#"+id)
		var target *html.Node
		for _, tag := range []string{"aside", "div", "section"} {
			if found := matching(doc, tag, "id", id); len(found) == 1 {
				target = found[0]
			}
		}
		if target == nil {
			t.Fatal("section shortcut target missing", id)
		}
		if value, _ := attribute(target, "tabindex"); value != "-1" {
			t.Fatal("section shortcut cannot transfer native keyboard focus", id)
		}
	}
	editor := only(t, doc, "aside", "id", "sg-editor")
	for _, group := range matching(editor, "details", "", "") {
		if _, open := attribute(group, "open"); open {
			t.Fatal("initial long token list hides the narrow-screen preview")
		}
	}
	form := guideForm(t, initial.body)
	form.Set("action", "preview")
	form.Set("radius.md", "-1px")
	response := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
	wantStatus(t, response, 422)
	doc = guideDOM(t, response.body)
	alert := only(t, doc, "section", "role", "alert")
	for key, want := range map[string]string{"tabindex": "-1", "autofocus": "", "aria-labelledby": "sg-error-title"} {
		if value, present := attribute(alert, key); !present || value != want {
			t.Fatal("error summary lost focus/name contract", key)
		}
	}
	only(t, alert, "h2", "id", "sg-error-title")
	only(t, alert, "a", "href", "#sg-field-radius.md")
	input := only(t, doc, "input", "name", "radius.md")
	for key, want := range map[string]string{"aria-invalid": "true", "aria-describedby": "sg-field-radius.md-error", "id": "sg-field-radius.md"} {
		if value, _ := attribute(input, key); value != want {
			t.Fatal("invalid field lost error association", key)
		}
	}
	only(t, doc, "p", "id", "sg-field-radius.md-error")
	corrected := guideForm(t, response.body)
	corrected.Set("radius.md", ".75rem")
	corrected.Set("action", "preview")
	good := guideRequest(t, f, client, "POST", guideBase, corrected.Encode(), postHeaders(f), "")
	wantStatus(t, good, 200)
	goodDoc := guideDOM(t, good.body)
	if len(matching(goodDoc, "input", "aria-invalid", "true")) != 0 || len(matching(goodDoc, "section", "role", "alert")) != 0 {
		t.Fatal("corrected candidate retained stale validation errors")
	}
	if value, _ := attribute(input, "value"); value != "-1px" {
		t.Fatal("invalid edit discarded")
	}
	for group := input.Parent; group != nil; group = group.Parent {
		if group.Data == "details" {
			if _, open := attribute(group, "open"); !open {
				t.Fatal("invalid edit hidden in a closed group")
			}
			return
		}
	}
	t.Fatal("radius field lost its editor group")
}

func TestStyleGuideNativeFormPolicyPreservesStrictOriginAndOpaquePreview(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	initial := guideRequest(t, f, client, "GET", guideBase, "", nil, "")
	wantStatus(t, initial, 200)
	if initial.header.Get("Referrer-Policy") != "same-origin" {
		t.Fatal("form page can produce Origin:null on a native Chrome POST")
	}
	for _, action := range []string{"preview", "baseline", "export"} {
		form := guideForm(t, initial.body)
		form.Set("action", action)
		status := 200
		if action == "export" {
			status = 409 // No preview acknowledgement: retain the repairable form.
		}
		response := guideRequest(t, f, client, "POST", guideBase, form.Encode(), postHeaders(f), "")
		wantStatus(t, response, status)
		if response.header.Get("Referrer-Policy") != "same-origin" {
			t.Fatalf("%s response broke subsequent native submission", action)
		}
	}
	invalid := guideForm(t, initial.body)
	invalid.Set("radius.md", "-1px")
	invalid.Set("action", "preview")
	wantStatus(t, guideRequest(t, f, client, "POST", guideBase, invalid.Encode(), postHeaders(f), ""), 422)
	for _, path := range []string{guideBase + "/style.css", previewURL(t, initial.body)} {
		response := guideRequest(t, f, client, "GET", path, "", nil, "")
		wantStatus(t, response, 200)
		if response.header.Get("Referrer-Policy") != "no-referrer" {
			t.Fatal("read-only resource lost its narrower policy", path)
		}
	}
	// The fix changes form identity generation, never the accepted origins.
	for _, origin := range []string{"", "null", "https://example.com"} {
		headers := postHeaders(f)
		headers.Set("Origin", origin)
		response := guideRequest(t, f, client, "POST", guideBase, "action=preview", headers, "")
		wantStatus(t, response, 403)
		assertGuideDenied(t, response)
	}
}

func TestStyleGuideOriginHostPeerAndTLSRefusals(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	client := f.client()
	body := "action=preview&specimen=work-queue&scheme=light&viewport=wide"
	for _, origins := range [][]string{nil, {""}, {"null"}, {"https://example.com"}, {f.server.URL, f.server.URL}, {f.server.URL, "null"}} {
		headers := postHeaders(f)
		delete(headers, "Origin")
		if origins != nil {
			headers["Origin"] = origins
		}
		response := guideRequest(t, f, client, "POST", guideBase, body, headers, "")
		wantStatus(t, response, 403)
		assertGuideDenied(t, response)
	}
	for _, headers := range []http.Header{{"Origin": {"null"}}, {"Origin": {"https://example.com"}}, {"Sec-Fetch-Site": {"cross-site"}}, {"Sec-Fetch-Site": {"same-site"}}, {"Sec-Fetch-Site": {"same-origin", "cross-site"}}} {
		wantStatus(t, guideRequest(t, f, client, "GET", guideBase, "", headers, ""), 403)
	}
	for _, site := range []string{"same-origin", "none"} {
		wantStatus(t, guideRequest(t, f, client, "GET", guideBase, "", http.Header{"Sec-Fetch-Site": {site}}, ""), 200)
	}
	for _, target := range []string{guideBase, guideBase + "/style.css", guideBase + "/preview?specimen=work-queue&scheme=light"} {
		response := guideRequest(t, f, client, "GET", target, "", http.Header{"X-Forwarded-Host": {strings.TrimPrefix(f.server.URL, "http://")}}, "example.com")
		wantStatus(t, response, 403)
		assertGuideDenied(t, response)
	}
	for _, remote := range []string{"192.0.2.1:1234", "not-an-address", "127.0.0.1"} {
		r := httptest.NewRequest("GET", f.server.URL+guideBase, nil)
		r.RemoteAddr = remote
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("non-loopback or malformed actual peer accepted", remote)
		}
	}
	r := httptest.NewRequest("GET", f.server.URL+guideBase, nil)
	r.RemoteAddr, r.TLS = "127.0.0.1:1234", &tls.ConnectionState{}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("TLS state disagreed with configured HTTP origin but was accepted")
	}
	tlsFixture := newGuideFixture(t, nil, true)
	wantStatus(t, guideRequest(t, tlsFixture, tlsFixture.client(), "GET", guideBase, "", nil, ""), 200)
	r = httptest.NewRequest("GET", tlsFixture.server.URL+guideBase, nil)
	r.RemoteAddr, r.TLS = "127.0.0.1:1234", nil
	w = httptest.NewRecorder()
	tlsFixture.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("HTTPS origin trusted an unencrypted request")
	}
}

func assertGuideDenied(t *testing.T, response guideResponse) {
	t.Helper()
	policy := response.header.Get("Content-Security-Policy")
	if response.header.Get("X-Frame-Options") != "DENY" || !strings.Contains(policy, "frame-ancestors 'none'") || strings.Contains(policy, "frame-src 'self'") || response.header.Get("X-Theme-Candidate-SHA256") != "" {
		t.Fatal("refusal retained success-only preview permissions or identity")
	}
}

func TestStyleGuideSpecimenOutputBudgetAndErrorAreAtomic(t *testing.T) {
	const limit = 512 << 10
	for _, tc := range []struct {
		name   string
		render func(context.Context, io.Writer) error
	}{
		{"error-after-prefix", func(_ context.Context, w io.Writer) error {
			_, _ = io.WriteString(w, "PRIVATE-FRAGMENT-PREFIX")
			return errors.New("INTERNAL-CALLBACK-DETAIL")
		}},
		{"oversized-write-ignored", func(_ context.Context, w io.Writer) error {
			_, _ = w.Write(bytes.Repeat([]byte("X"), limit+1))
			return nil
		}},
		{"overflow-after-exact-budget", func(_ context.Context, w io.Writer) error {
			_, _ = w.Write(bytes.Repeat([]byte("X"), limit))
			_, _ = w.Write([]byte("!"))
			return nil
		}},
		{"copy-readerfrom-bypass", func(_ context.Context, w io.Writer) error {
			// Hide strings.Reader.WriteTo, so io.Copy exercises destination
			// ReaderFrom if an implementation accidentally embeds bytes.Buffer.
			_, _ = io.Copy(w, struct{ io.Reader }{strings.NewReader(strings.Repeat("X", limit+1))})
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGuideFixture(t, func(o *styleguide.Options) { o.Specimens[0].Render = tc.render }, false)
			response := guideRequest(t, f, f.client(), "GET", guideBase+"/preview?specimen=work-queue&scheme=light", "", nil, "")
			wantStatus(t, response, 500)
			assertGuideDenied(t, response)
			if len(response.body) > 256 || bytes.Contains(response.body, []byte("PRIVATE-FRAGMENT")) || bytes.Contains(response.body, []byte("INTERNAL-CALLBACK")) || bytes.Contains(response.body, []byte("<style>")) {
				t.Fatal("callback failure leaked partial markup or internal details")
			}
		})
	}
	t.Run("exact-budget-is-valid", func(t *testing.T) {
		f := newGuideFixture(t, func(o *styleguide.Options) {
			o.Specimens[0].Render = func(_ context.Context, w io.Writer) error {
				_, err := w.Write(bytes.Repeat([]byte("X"), limit))
				return err
			}
		}, false)
		response := guideRequest(t, f, f.client(), "GET", guideBase+"/preview?specimen=work-queue&scheme=dark", "", nil, "")
		wantStatus(t, response, 200)
		if len(response.body) <= limit || response.header.Get("X-Theme-Candidate-SHA256") == "" {
			t.Fatal("exact-budget successful fragment was truncated or unidentified")
		}
	})
}

func TestStyleGuideConfigurationRefusals(t *testing.T) {
	f := newGuideFixture(t, nil, false)
	for _, tc := range []struct {
		name   string
		change func(*styleguide.Options)
	}{
		{"missing-engine", func(o *styleguide.Options) { o.Engine = nil }},
		{"missing-specimens", func(o *styleguide.Options) { o.Specimens = nil }},
		{"missing-callback", func(o *styleguide.Options) { o.Specimens[0].Render = nil }},
		{"duplicate-specimen", func(o *styleguide.Options) { o.Specimens = append(o.Specimens, o.Specimens[0]) }},
		{"unsafe-specimen-id", func(o *styleguide.Options) { o.Specimens[0].ID = "../app" }},
		{"empty-title", func(o *styleguide.Options) { o.Title = "" }},
		{"oversized-title", func(o *styleguide.Options) { o.Title = strings.Repeat("x", 161) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := f.options
			o.Specimens = slices.Clone(o.Specimens)
			tc.change(&o)
			if _, err := styleguide.New(o); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	for _, origin := range []string{"", "https://example.com:443", "http://localhost:8765", "http://127.0.0.1", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1:08765", "http://user@127.0.0.1:8765", f.server.URL + "/", f.server.URL + "?x=1", f.server.URL + "#x"} {
		o := f.options
		o.Origin = origin
		if _, err := styleguide.New(o); err == nil {
			t.Error("invalid origin accepted", origin)
		}
	}
	for _, path := range []string{"", "/", "dev/theme", "/dev/theme/", "//dev/theme", "/dev/../theme", "/dev/%74heme", "/dev/theme?x=1"} {
		o := f.options
		o.BasePath = path
		if _, err := styleguide.New(o); err == nil {
			t.Error("noncanonical base accepted", path)
		}
	}
	for _, path := range []string{"https://example.com/app.css", "//example.com/app.css", `/\example.com/app.css`, "/a/../app.css", "/%61pp.css", "/app.css?x=1", "/app.css#x", "/app.js"} {
		o := f.options
		o.Stylesheets = []string{path}
		if _, err := styleguide.New(o); err == nil {
			t.Error("unsafe stylesheet accepted", path)
		}
	}
	for _, origin := range []string{"http://127.0.0.1:8765", "https://127.0.0.1:8765", "http://[::1]:8765"} {
		o := f.options
		o.Origin = origin
		if _, err := styleguide.New(o); err != nil {
			t.Error("valid exact loopback origin refused", origin, err)
		}
	}
}
