package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const guideTestOrigin = "http://127.0.0.1:18767"

func guideTestRequest(t *testing.T, h http.Handler, method, route, body, origin string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, guideTestOrigin+route, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Host = "127.0.0.1:18767"
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func guideTestForm(t *testing.T, body []byte) (url.Values, string) {
	t.Helper()
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{}
	previewURL := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "iframe" {
				previewURL = attrs["src"]
			}
			if n.Data == "input" && attrs["name"] != "" {
				values.Set(attrs["name"], attrs["value"])
			}
			if n.Data == "select" && attrs["name"] != "" {
				name := attrs["name"]
				selected := ""
				for option := n.FirstChild; option != nil; option = option.NextSibling {
					if option.Type != html.ElementNode || option.Data != "option" {
						continue
					}
					for _, a := range option.Attr {
						if a.Key == "value" && selected == "" {
							selected = a.Val
						}
						if a.Key == "selected" {
							for _, value := range option.Attr {
								if value.Key == "value" {
									selected = value.Val
								}
							}
						}
					}
				}
				values.Set(name, selected)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return values, previewURL
}

func TestObservatoryThemeGuideSyntheticStatesAndOrigin(t *testing.T) {
	h, err := observatoryGuideMux(guideTestOrigin)
	if err != nil {
		t.Fatal(err)
	}
	page := guideTestRequest(t, h, http.MethodGet, "/dev/theme", "", "")
	if page.Code != http.StatusOK {
		t.Fatalf("guide GET = %d: %s", page.Code, page.Body.String())
	}
	for _, label := range []string{"Complete observation coverage", "Partial observation coverage", "Missing registry", "Registry and call-log error"} {
		if !strings.Contains(page.Body.String(), label) {
			t.Errorf("guide omitted %s specimen", label)
		}
	}
	if strings.Contains(page.Body.String(), "real service registry") {
		t.Fatal("guide leaked live-model language")
	}
	for _, state := range []string{"complete", "partial", "missing", "error"} {
		model := observatoryGuideModel(state)
		if model.FathomVersion != "synthetic-guide" || model.GeneratedAt != "2026-09-21T00:00:00Z" {
			t.Fatal("guide model is not fixed synthetic input")
		}
		if state == "complete" && (len(model.Services) != 1 || model.Services[0].OpenObservations == nil) {
			t.Fatal("complete specimen lost known value")
		}
		if state == "partial" && (len(model.Services) != 1 || model.Services[0].OpenObservations != nil) {
			t.Fatal("partial specimen fabricated observation total")
		}
		if state == "missing" && (len(model.Services) != 0 || !strings.Contains(model.DegradedNote, "unknown")) {
			t.Fatal("missing specimen became zero")
		}
		if state == "error" && (len(model.Services) != 0 || !strings.Contains(model.CallLogNote, "failed")) {
			t.Fatal("error specimen hid read failure")
		}
	}
	wrongHost := httptest.NewRequest(http.MethodGet, guideTestOrigin+"/dev/theme", nil)
	wrongHost.RemoteAddr = "127.0.0.1:12345"
	wrongHost.Host = "localhost:18767"
	refused := httptest.NewRecorder()
	h.ServeHTTP(refused, wrongHost)
	if refused.Code != http.StatusForbidden {
		t.Fatalf("wrong Host = %d, want 403", refused.Code)
	}
	wrongAsset := httptest.NewRecorder()
	wrongHost.URL.Path = "/app.css"
	h.ServeHTTP(wrongAsset, wrongHost)
	if wrongAsset.Code != http.StatusForbidden {
		t.Fatalf("wrong Host asset = %d, want 403", wrongAsset.Code)
	}
	if got := guideTestRequest(t, h, http.MethodPost, "/dev/theme", "action=preview", "null"); got.Code != http.StatusForbidden {
		t.Fatalf("null Origin = %d, want 403", got.Code)
	}
	if got := guideTestRequest(t, h, http.MethodGet, "/observatory", "", ""); got.Code != http.StatusNotFound {
		t.Fatalf("opt-in guide unexpectedly mounted Observatory: %d", got.Code)
	}
}

func TestObservatoryThemeGuideSourceValidityCanaries(t *testing.T) {
	for _, tc := range []struct {
		state, services, freshness, obligations, sessions string
	}{
		{"complete", "1", "all fresh", "3", "2"},
		{"partial", "≥\u00a01", "—", "—", "—"},
		{"missing", "—", "—", "—", "—"},
		{"error", "—", "—", "—", "—"},
	} {
		t.Run(tc.state, func(t *testing.T) {
			model := observatoryGuideModel(tc.state)
			fleet := renderObsFleet(model)
			impact := renderObsImpact(model)
			for _, check := range []struct{ body, label, want string }{
				{fleet, "services indexed", tc.services},
				{fleet, "index freshness", tc.freshness},
				{impact, "obligations served (calls)", tc.obligations},
				{impact, "ledger sessions", tc.sessions},
				{impact, "obligations served", tc.obligations},
			} {
				if got := tileValue(t, check.body, check.label); got != check.want {
					t.Errorf("%s = %q, want %q", check.label, got, check.want)
				}
			}
			if tc.state == "partial" && !strings.Contains(fleet, "listed services; fleet coverage unavailable") {
				t.Error("partial fleet count lacks listed-only lower-bound disclosure")
			}
		})
	}

	measuredZero := observatoryGuideModel("complete")
	measuredZero.Services = nil // successful empty registry, not a failed read
	measuredZero.Impact.ObligationCalls = 0
	measuredZero.Impact.LedgerSessions = 0
	if got := tileValue(t, renderObsFleet(measuredZero), "services indexed"); got != "0" {
		t.Errorf("readable empty registry = %q, want measured 0", got)
	}
	for _, check := range []struct{ label, want string }{
		{"obligations served (calls)", "0"}, {"ledger sessions", "0"},
	} {
		if got := tileValue(t, renderObsImpact(measuredZero), check.label); got != check.want {
			t.Errorf("measured %s = %q, want %q", check.label, got, check.want)
		}
	}

	callLogOnly := observatoryGuideModel("complete")
	callLogOnly.CallLogNote = "Synthetic call-log read failed."
	if got := tileValue(t, renderObsImpact(callLogOnly), "obligations served (calls)"); got != "—" {
		t.Errorf("unreadable call log produced obligations %q", got)
	}
	if got := tileValue(t, renderObsImpact(callLogOnly), "ledger sessions"); got != "2" {
		t.Errorf("independent readable ledger lost its two sessions: %q", got)
	}
	ledgerOnly := observatoryGuideModel("complete")
	ledgerOnly.Impact.LedgerNote = "Synthetic session ledger read failed."
	if got := tileValue(t, renderObsImpact(ledgerOnly), "ledger sessions"); got != "—" {
		t.Errorf("unreadable ledger produced sessions %q", got)
	}
	if got := tileValue(t, renderObsImpact(ledgerOnly), "obligations served (calls)"); got != "3" {
		t.Errorf("independent readable call log lost its three obligation calls: %q", got)
	}
	for _, state := range []string{"partial", "missing", "error"} {
		model := observatoryGuideModel(state)
		impact := renderObsImpact(model)
		for _, arm := range []string{"ON arm", "OFF arm"} {
			value := kvValue(t, impact, arm)
			if strings.Contains(value, "0 sessions") || !strings.Contains(value, model.Impact.LedgerNote) {
				t.Errorf("%s %s must name unreadable ledger, got %q", state, arm, value)
			}
		}
	}
	for _, model := range []observatoryModel{measuredZero, callLogOnly} {
		impact := renderObsImpact(model)
		for _, arm := range []string{"ON arm", "OFF arm"} {
			if value := kvValue(t, impact, arm); !strings.Contains(value, "0 sessions") {
				t.Errorf("readable ledger %s lost measured zero due to unrelated source failure: %q", arm, value)
			}
		}
	}
}

func TestObservatoryThemeGuidePreviewResetExportAndRefusal(t *testing.T) {
	h, err := observatoryGuideMux(guideTestOrigin)
	if err != nil {
		t.Fatal(err)
	}
	page := guideTestRequest(t, h, http.MethodGet, "/dev/theme", "", "")
	form, _ := guideTestForm(t, page.Body.Bytes())
	form.Set("action", "preview")
	preview := guideTestRequest(t, h, http.MethodPost, "/dev/theme", form.Encode(), guideTestOrigin)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview = %d: %s", preview.Code, preview.Body.String())
	}
	review, target := guideTestForm(t, preview.Body.Bytes())
	if review.Get("review_digest") == "" || target == "" {
		t.Fatal("preview did not produce digest and frame")
	}
	if u, err := url.Parse(target); err != nil || u.Path != "/dev/theme/preview" || u.IsAbs() {
		t.Fatalf("unsafe preview target %q: %v", target, err)
	}
	frame := guideTestRequest(t, h, http.MethodGet, target, "", "")
	if frame.Code != http.StatusOK || !strings.Contains(frame.Body.String(), "fixture") ||
		!strings.Contains(frame.Body.String(), "open observations") {
		t.Fatalf("synthetic preview = %d", frame.Code)
	}
	if strings.Contains(frame.Body.String(), `href="/observatory`) ||
		!strings.Contains(frame.Body.String(), "operational links are disabled") ||
		!strings.Contains(frame.Body.String(), `aria-disabled="true"`) {
		t.Fatal("synthetic preview exposes live-route navigation or hides its disabled-link boundary")
	}
	stale := url.Values{}
	for k, v := range review {
		stale[k] = append([]string(nil), v...)
	}
	stale.Set("action", "export")
	stale.Set("review_digest", strings.Repeat("0", 64))
	refused := guideTestRequest(t, h, http.MethodPost, "/dev/theme", stale.Encode(), guideTestOrigin)
	if strings.Contains(refused.Header().Get("Content-Type"), "zip") {
		t.Fatal("stale digest exported")
	}
	review.Set("action", "export")
	export := guideTestRequest(t, h, http.MethodPost, "/dev/theme", review.Encode(), guideTestOrigin)
	if export.Code != http.StatusOK || !strings.Contains(export.Header().Get("Content-Type"), "zip") {
		t.Fatalf("export = %d, type %q", export.Code, export.Header().Get("Content-Type"))
	}
	z, err := zip.NewReader(bytes.NewReader(export.Body.Bytes()), int64(export.Body.Len()))
	if err != nil || len(z.File) != 5 {
		t.Fatalf("export archive invalid: %v, entries=%d", err, len(z.File))
	}
	for _, file := range z.File {
		if file.Name == "original.cue" {
			r, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			r.Close()
			if err != nil || !bytes.Equal(data, observatoryGuideSource) {
				t.Fatal("export original CUE differs from exact guide baseline")
			}
		}
	}
	// A fresh GET is the stateless reset; "baseline" instead compares the
	// source while preserving the candidate form and acknowledgement.
	reset := guideTestRequest(t, h, http.MethodGet, "/dev/theme", "", "")
	if reset.Code != http.StatusOK {
		t.Fatalf("reset = %d", reset.Code)
	}
	resetForm, _ := guideTestForm(t, reset.Body.Bytes())
	if resetForm.Get("review_digest") != "" {
		t.Fatal("reset retained candidate acknowledgement")
	}
}

func TestObservatoryThemeGuideDisablesUnmountedRendererLinks(t *testing.T) {
	for _, state := range []string{"complete", "partial", "missing", "error"} {
		t.Run(state, func(t *testing.T) {
			model := observatoryGuideModel(state)
			fragment, err := disableObservatoryGuideLinks(renderObsFleet(model) + renderObsImpact(model))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(fragment, `href="/observatory`) || !strings.Contains(fragment, `aria-disabled="true"`) {
				t.Fatal("guide fragment retained an operational link")
			}
			if state == "complete" && (!strings.Contains(fragment, "detail →") || !strings.Contains(fragment, "Inspect session evidence")) {
				t.Fatal("disabled links lost the renderer's visible task labels")
			}
		})
	}
}

func TestThemeGuideRejectsNonLoopbackBeforeListening(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := cmdThemeGuide([]string{"-addr", "0.0.0.0:18767"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "loopback") {
		t.Fatalf("nonloopback code=%d, stderr=%q", code, errOut.String())
	}
}

// The guide's "selected kit identity" must name the mirror it was built
// from. A pin bump that forgets this constant would export review bundles
// claiming a kit revision the binary does not contain.
func TestThemeGuideKitRevisionMatchesVendoredMirror(t *testing.T) {
	stamp, err := os.ReadFile(filepath.Join("..", "..", "third_party", "app-kit", ".vendor-commit"))
	if err != nil {
		t.Fatalf("read app-kit mirror stamp: %v", err)
	}
	if got, want := observatoryGuideRevision, strings.TrimSpace(string(stamp)); got != want {
		t.Fatalf("observatoryGuideRevision = %s, want the vendored mirror stamp %s", got, want)
	}
}
