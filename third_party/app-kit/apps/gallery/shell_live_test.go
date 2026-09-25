package gallery

import (
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func shellLiveTestID(doc *html.Node, id string) *html.Node {
	var found *html.Node
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && shellDemoTestAttr(n, "id") == id {
			found = n
		}
	})
	return found
}

func TestShellLiveOnlySignedInActivityLoadsObserver(t *testing.T) {
	mux := http.NewServeMux()
	registerShellDemo(mux)
	registerShellDemoAssets(mux)
	browser := shellDemoTestNewBrowser(mux)
	var previousNonce string
	for _, page := range []string{"home", "profile", "settings", "help", "about", "activity", "activity"} {
		w := browser.request(http.MethodGet, shellDemoURL(page), "", nil)
		shellDemoTestStatus(t, w, http.StatusOK)
		doc := shellDemoTestDOM(t, w.Body.String())
		var nonce, dataNonce string
		var entry int
		shellDemoTestWalk(doc, func(n *html.Node) {
			if n.Type != html.ElementNode {
				return
			}
			if n.Data == "html" {
				dataNonce = shellDemoTestAttr(n, "data-nonce")
			}
			if n.Data == "script" {
				if value := shellDemoTestAttr(n, "nonce"); value != "" {
					nonce = value
				}
				if shellDemoTestAttr(n, "src") == "/static/shell-live-entry.js" {
					entry++
					if shellDemoTestAttr(n, "type") != "module" {
						t.Error("observer entry must be an ES module")
					}
				}
			}
		})
		csp := w.Header().Get("Content-Security-Policy")
		if nonce == "" || nonce == previousNonce || !strings.Contains(csp, "'nonce-"+nonce+"'") {
			t.Fatalf("%s lacks a fresh header-bound nonce", page)
		}
		previousNonce = nonce
		for _, forbidden := range []string{"unsafe-eval", "unsafe-inline", "https:", "http:", "*"} {
			if strings.Contains(csp, forbidden) {
				t.Errorf("%s weakens CSP with %q: %s", page, forbidden, csp)
			}
		}
		if page == "activity" {
			if entry != 1 || dataNonce != nonce || !strings.Contains(csp, "; connect-src 'self'") {
				t.Fatalf("activity observer module, nonce or connect policy absent: entry=%d csp=%s", entry, csp)
			}
			for _, id := range []string{"shell-live", "shell-live-start", "shell-live-stop", "shell-live-status", "shell-live-content", "shell-live-summary", "shell-live-activity"} {
				if shellLiveTestID(doc, id) == nil {
					t.Errorf("missing observer node %s", id)
				}
			}
			form := shellLiveTestID(doc, "shell-live-form")
			if form == nil || form.Data != "form" || shellDemoTestAttr(form, "method") != "get" || shellDemoTestAttr(form, "action") != "/shell/activity" {
				t.Fatal("observer requires its own harmless empty form")
			}
			shellDemoTestWalk(form, func(n *html.Node) {
				if shellDemoTestHasAttr(n, "name") {
					t.Error("observer form must send no named controls/signals/CSRF/edit versions")
				}
			})
			start, stop := shellLiveTestID(doc, "shell-live-start"), shellLiveTestID(doc, "shell-live-stop")
			for _, control := range []*html.Node{start, stop} {
				if !shellDemoTestHasAttr(control, "disabled") || shellDemoTestAttr(control, "type") != "button" {
					t.Error("observer controls must be inert until the client mounts")
				}
			}
			const expression = "window.AppKitShellLive.connect(() => @get('/shell/events', {contentType: 'form', selector: '#shell-live-form', headers: {Accept: 'text/event-stream'}, openWhenHidden: true, retry: 'never', retryMaxCount: 0, requestCancellation: window.AppKitShellLive.controller}))"
			if got := shellDemoTestAttr(start, "data-on:click"); got != expression {
				t.Fatalf("wire/lifetime options changed: %q", got)
			}
			if shellDemoTestAttr(shellLiveTestID(doc, "shell-live-status"), "role") != "status" {
				t.Error("connection state lacks status semantics")
			}
			if !strings.Contains(w.Body.String(), "Live updates require scripts") || !strings.Contains(w.Body.String(), `href="/shell/activity"`) {
				t.Error("ordinary reload/no-script fallback missing")
			}
		} else if entry != 0 || dataNonce != "" || strings.Contains(csp, "connect-src") || strings.Contains(w.Body.String(), "data-on:") {
			t.Fatalf("%s unexpectedly enables streaming", page)
		}
	}
	// The normal Gallery asset registration reaches every import without a CDN.
	for _, path := range []string{"/static/shell-live-entry.js", "/static/shell-live.js", "/static/datastar-v1.0.3.js"} {
		shellDemoTestStatus(t, browser.request(http.MethodGet, path, "", nil), http.StatusOK)
	}
	page := browser.request(http.MethodGet, "/shell/activity", "", nil)
	form := shellDemoTestForm(t, page.Body.String(), "signout")
	shellDemoTestStatus(t, browser.request(http.MethodPost, "/shell/action", form.Encode(), nil), http.StatusSeeOther)
	signedOut := browser.request(http.MethodGet, "/shell/activity", "", nil)
	if strings.Contains(signedOut.Body.String(), "shell-live") || strings.Contains(signedOut.Header().Get("Content-Security-Policy"), "connect-src") {
		t.Fatal("signed-out page must not offer/load the observer")
	}
}

func TestShellLiveUntrustedStateCannotChangeExpressions(t *testing.T) {
	v := shellDemoView{Page: "activity", State: defaultShellDemoState(1), CSRF: strings.Repeat("a", 64), Nonce: strings.Repeat("b", 64), Status: 200}
	v.State.DisplayName = `"><script>alert(1)</script>`
	v.State.Activity = []string{`<img src=x onerror="alert(1)">`}
	for _, availability := range []string{"ready", "loading", "error"} {
		v.State.Availability = availability
		v.State.Access = "viewer" // Read-only access may observe but cannot save.
		body, err := renderShellDemo(v)
		if err != nil {
			t.Fatal(err)
		}
		doc := shellDemoTestDOM(t, string(body))
		if !strings.Contains(shellDemoTestText(shellLiveTestID(doc, "shell-live-summary")), v.State.DisplayName) || !strings.Contains(shellDemoTestText(shellLiveTestID(doc, "shell-live-activity")), v.State.Activity[0]) {
			t.Error("synthetic labels must remain text")
		}
		shellDemoTestWalk(doc, func(n *html.Node) {
			if n.Type == html.ElementNode && (n.Data == "img" || n.Data == "script" && strings.Contains(shellDemoTestText(n), "alert(1)")) {
				t.Error("untrusted state became active markup")
			}
		})
	}
}
