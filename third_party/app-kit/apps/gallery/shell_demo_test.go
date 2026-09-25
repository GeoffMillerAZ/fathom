package gallery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// These tests exercise the HTTP/HTML contract, not the session store or renderer
// structs. Every mutation uses a synthetic browser's real issued CSRF token.
type shellDemoTestBrowser struct {
	h       http.Handler
	cookies map[string]*http.Cookie
}

func shellDemoTestNewBrowser(h http.Handler) *shellDemoTestBrowser {
	return &shellDemoTestBrowser{h: h, cookies: map[string]*http.Cookie{}}
}

func shellDemoTestHandler() http.Handler {
	mux := http.NewServeMux()
	registerShellDemo(mux)
	return mux
}

func (b *shellDemoTestBrowser) request(method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host, r.RemoteAddr = "127.0.0.1:8765", "127.0.0.1:32100"
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "http://127.0.0.1:8765")
	}
	for _, cookie := range b.cookies {
		r.AddCookie(cookie)
	}
	for key, value := range headers {
		switch key {
		case "Host":
			r.Host = value
		case "RemoteAddr":
			r.RemoteAddr = value
		default:
			r.Header.Set(key, value)
		}
	}
	w := httptest.NewRecorder()
	b.h.ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(b.cookies, cookie.Name)
		} else {
			b.cookies[cookie.Name] = cookie
		}
	}
	return w
}

func shellDemoTestStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d, want %d; response %.1000s", w.Code, want, w.Body.String())
	}
}

func shellDemoTestAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func shellDemoTestHasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func shellDemoTestWalk(n *html.Node, visit func(*html.Node)) {
	visit(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		shellDemoTestWalk(child, visit)
	}
}

func shellDemoTestText(n *html.Node) string {
	var text strings.Builder
	shellDemoTestWalk(n, func(child *html.Node) {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	})
	return text.String()
}

func shellDemoTestDOM(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	ids, labels := map[string]bool{}, map[string]bool{}
	mainCount := 0
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if n.Data == "main" {
			mainCount++
		}
		if id := shellDemoTestAttr(n, "id"); id != "" {
			if ids[id] {
				t.Errorf("duplicate HTML id %q", id)
			}
			ids[id] = true
		}
		if n.Data == "label" {
			labels[shellDemoTestAttr(n, "for")] = true
		}
		for _, attr := range n.Attr {
			if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
				t.Errorf("unexpected inline event handler %s", attr.Key)
			}
		}
	})
	if mainCount != 1 {
		t.Errorf("main landmarks=%d, want exactly one", mainCount)
	}
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		for _, attr := range []string{"aria-describedby", "aria-labelledby", "aria-errormessage"} {
			for _, id := range strings.Fields(shellDemoTestAttr(n, attr)) {
				if !ids[id] {
					t.Errorf("%s references missing id %q", attr, id)
				}
			}
		}
		if n.Data == "a" {
			if href := shellDemoTestAttr(n, "href"); strings.HasPrefix(href, "#") && !ids[strings.TrimPrefix(href, "#")] {
				t.Errorf("fragment link has no target: %q", href)
			}
		}
		if n.Data != "input" && n.Data != "select" && n.Data != "textarea" {
			return
		}
		typ := shellDemoTestAttr(n, "type")
		if typ == "hidden" || typ == "submit" || typ == "button" {
			return
		}
		labelled := labels[shellDemoTestAttr(n, "id")] && shellDemoTestAttr(n, "id") != ""
		for p := n.Parent; p != nil; p = p.Parent {
			labelled = labelled || p.Type == html.ElementNode && p.Data == "label"
		}
		if !labelled && shellDemoTestAttr(n, "aria-label") == "" && shellDemoTestAttr(n, "aria-labelledby") == "" {
			t.Errorf("unlabelled %s name=%q", n.Data, shellDemoTestAttr(n, "name"))
		}
	})
	return doc
}

func shellDemoTestForm(t *testing.T, body, action string) url.Values {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var result url.Values
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "form" {
			return
		}
		values := url.Values{}
		shellDemoTestWalk(n, func(field *html.Node) {
			if field.Type != html.ElementNode {
				return
			}
			name := shellDemoTestAttr(field, "name")
			if name == "" {
				return
			}
			switch field.Data {
			case "button":
				// A single-action form may put its action on the submitter.
				if name == "action" && shellDemoTestAttr(field, "value") == action {
					values.Add(name, action)
				}
			case "input":
				typ := shellDemoTestAttr(field, "type")
				if (typ == "checkbox" || typ == "radio") && !shellDemoTestHasAttr(field, "checked") {
					return
				}
				values.Add(name, shellDemoTestAttr(field, "value"))
			case "textarea":
				values.Add(name, shellDemoTestText(field))
			case "select":
				first, selected, hasFirst, hasSelected := "", "", false, false
				shellDemoTestWalk(field, func(option *html.Node) {
					if option.Type != html.ElementNode || option.Data != "option" {
						return
					}
					value := shellDemoTestAttr(option, "value")
					if !hasFirst {
						first, hasFirst = value, true
					}
					if shellDemoTestHasAttr(option, "selected") {
						selected, hasSelected = value, true
					}
				})
				if hasSelected {
					values.Add(name, selected)
				} else {
					values.Add(name, first)
				}
			}
		})
		if values.Get("action") == action {
			if result != nil && result.Encode() != values.Encode() {
				t.Fatalf("inconsistent forms for action %q", action)
			}
			if shellDemoTestAttr(n, "action") != "/shell/action" || strings.ToLower(shellDemoTestAttr(n, "method")) != "post" {
				t.Fatalf("%s form is not POST /shell/action", action)
			}
			result = values
		}
	})
	if result == nil || result.Get("csrf") == "" {
		t.Fatalf("missing %s form/CSRF in %.1000s", action, body)
	}
	return result
}

func (b *shellDemoTestBrowser) get(t *testing.T, path string, status int) *httptest.ResponseRecorder {
	t.Helper()
	w := b.request("GET", path, "", nil)
	shellDemoTestStatus(t, w, status)
	return w
}

func (b *shellDemoTestBrowser) post(t *testing.T, values url.Values, status int) *httptest.ResponseRecorder {
	t.Helper()
	w := b.request("POST", "/shell/action", values.Encode(), nil)
	shellDemoTestStatus(t, w, status)
	return w
}

func shellDemoTestRedirect(t *testing.T, w *httptest.ResponseRecorder, path string) {
	t.Helper()
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil || u.IsAbs() || u.Host != "" || u.Path != path {
		t.Fatalf("unsafe/wrong redirect %q, want %s", w.Header().Get("Location"), path)
	}
}

func (b *shellDemoTestBrowser) fixture(t *testing.T, access, availability string, fail bool) {
	t.Helper()
	page := b.get(t, "/shell/help", 200)
	values := shellDemoTestForm(t, page.Body.String(), "fixture")
	values.Set("access", access)
	values.Set("availability", availability)
	values.Del("fail_next_save")
	if fail {
		values.Set("fail_next_save", "on")
	}
	b.post(t, values, 303)
}

func TestShellDemoRoutesSessionsAndSemantics(t *testing.T) {
	h := shellDemoTestHandler()
	a, b := shellDemoTestNewBrowser(h), shellDemoTestNewBrowser(h)
	first := a.get(t, "/shell/profile", 200)
	profile := shellDemoTestForm(t, first.Body.String(), "save-profile")
	if profile.Get("display_name") != "Alex Morgan" || profile.Get("version") == "" {
		t.Fatal("profile fixture/default version missing")
	}
	cookie := a.cookies["appkit_shell_demo"]
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/shell" {
		t.Fatal("demo session must be HttpOnly, SameSiteStrict and scoped to /shell")
	}
	other := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	if other.Get("csrf") == profile.Get("csrf") || b.cookies[cookie.Name].Value == cookie.Value {
		t.Fatal("independent browsers share a session or CSRF token")
	}
	b.post(t, profile, 403)
	visited := map[string]bool{}
	for _, page := range []string{"", "/home", "/activity", "/profile", "/settings", "/help", "/about"} {
		w := a.get(t, "/shell"+page, 200)
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("stateful demo response missing private/no-sniff headers")
		}
		doc := shellDemoTestDOM(t, w.Body.String())
		shellDemoTestWalk(doc, func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "script" && shellDemoTestAttr(n, "src") == "" {
				nonce := shellDemoTestAttr(n, "nonce")
				if nonce == "" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "'nonce-"+nonce+"'") {
					t.Error("inline appearance bootstrap lacks the response CSP nonce")
				}
			}
			if n.Type != html.ElementNode || n.Data != "a" {
				return
			}
			u, err := url.Parse(shellDemoTestAttr(n, "href"))
			if err != nil || !strings.HasPrefix(u.Path, "/shell") || visited[u.RequestURI()] {
				return
			}
			visited[u.RequestURI()] = true
			if u.IsAbs() || u.Host != "" {
				t.Fatalf("demo navigation leaves local application: %s", u)
			}
			a.get(t, u.RequestURI(), 200)
		})
	}
	for _, path := range []string{"/shell/profile", "/shell/settings", "/shell/help", "/shell/about"} {
		if !visited[path] {
			t.Errorf("no working navigation destination for %s", path)
		}
	}
	profile.Set("display_name", "Synthetic Avery")
	a.post(t, profile, 303)
	if got := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Alex Morgan" {
		t.Fatal("one browser's save changed another browser's profile")
	}
}

func TestShellDemoProfileSaveValidationConflictAndRetry(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	page := b.get(t, "/shell/profile", 200)
	first, stale := shellDemoTestForm(t, page.Body.String(), "save-profile"), shellDemoTestForm(t, page.Body.String(), "save-profile")
	first.Set("display_name", "Synthetic Avery")
	shellDemoTestRedirect(t, b.post(t, first, 303), "/shell/profile")
	stale.Set("display_name", "Reviewed Casey")
	conflict := b.post(t, stale, 409)
	shellDemoTestDOM(t, conflict.Body.String())
	draft := shellDemoTestForm(t, conflict.Body.String(), "save-profile")
	if draft.Get("display_name") != "Reviewed Casey" || draft.Get("version") == stale.Get("version") || !strings.Contains(conflict.Body.String(), "Apply reviewed draft") {
		t.Fatal("conflict must retain draft and require explicit review at the current version")
	}
	if got := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Synthetic Avery" {
		t.Fatal("stale save overwrote the current profile")
	}
	b.post(t, draft, 303)
	valid := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	if valid.Get("display_name") != "Reviewed Casey" {
		t.Fatal("explicit reviewed retry did not save")
	}
	valid.Set("display_name", strings.Repeat("x", 300))
	invalid := b.post(t, valid, 422)
	shellDemoTestDOM(t, invalid.Body.String())
	if shellDemoTestForm(t, invalid.Body.String(), "save-profile").Get("display_name") != valid.Get("display_name") || !strings.Contains(invalid.Body.String(), `aria-invalid="true"`) {
		t.Fatal("validation failure lost draft or associated field error")
	}
	b.fixture(t, "editor", "ready", true)
	values := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	values.Set("display_name", "Retry Morgan")
	failed := b.post(t, values, 503)
	retry := shellDemoTestForm(t, failed.Body.String(), "save-profile")
	if retry.Get("display_name") != "Retry Morgan" {
		t.Fatal("temporary save failure lost draft")
	}
	if got := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Reviewed Casey" {
		t.Fatal("failed save mutated saved profile")
	}
	b.post(t, retry, 303)
	if got := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Retry Morgan" {
		t.Fatal("one-shot failure did not permit exact retry")
	}
}

func TestShellDemoSettingsSaveAndValidation(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	values := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if values.Get("timezone") != "America/Phoenix" || values.Get("start_page") != "home" || values.Get("digest") != "on" {
		t.Fatal("settings defaults disagree with documented synthetic fixture")
	}
	values.Set("timezone", "Europe/London")
	values.Set("start_page", "activity")
	values.Del("digest")
	shellDemoTestRedirect(t, b.post(t, values, 303), "/shell/settings")
	saved := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if saved.Get("timezone") != "Europe/London" || saved.Get("start_page") != "activity" || saved.Has("digest") {
		t.Fatal("settings save failed to retain exact selections and unchecked checkbox")
	}
	for _, field := range []string{"timezone", "start_page"} {
		invalid := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
		invalid.Set(field, "invalid-choice")
		w := b.post(t, invalid, 422)
		shellDemoTestDOM(t, w.Body.String())
		if shellDemoTestForm(t, w.Body.String(), "save-settings").Get(field) != "invalid-choice" || !strings.Contains(w.Body.String(), `aria-invalid="true"`) {
			t.Fatalf("invalid %s selection was silently replaced or lacked field error", field)
		}
		unchanged := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
		if unchanged.Get("timezone") != "Europe/London" || unchanged.Get("start_page") != "activity" || unchanged.Has("digest") {
			t.Fatal("invalid settings mutation changed saved values")
		}
	}
}

func TestShellDemoSettingsConflictAndSaveFailure(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	page := b.get(t, "/shell/settings", 200)
	first := shellDemoTestForm(t, page.Body.String(), "save-settings")
	stale := shellDemoTestForm(t, page.Body.String(), "save-settings")
	first.Set("timezone", "UTC")
	b.post(t, first, 303)
	stale.Set("timezone", "Europe/London")
	stale.Set("start_page", "activity")
	stale.Del("digest")
	conflict := b.post(t, stale, 409)
	reviewed := shellDemoTestForm(t, conflict.Body.String(), "save-settings")
	if reviewed.Get("timezone") != "Europe/London" || reviewed.Get("start_page") != "activity" || reviewed.Has("digest") || reviewed.Get("version") == stale.Get("version") {
		t.Fatal("settings conflict lost draft fields or latest review version")
	}
	current := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if current.Get("timezone") != "UTC" || current.Get("start_page") != "home" || current.Get("digest") != "on" {
		t.Fatal("conflicting settings changed saved state")
	}
	b.post(t, reviewed, 303)
	b.fixture(t, "editor", "ready", true)
	values := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	values.Set("timezone", "America/Phoenix")
	values.Set("digest", "on")
	failed := b.post(t, values, 503)
	retry := shellDemoTestForm(t, failed.Body.String(), "save-settings")
	if retry.Get("timezone") != "America/Phoenix" || retry.Get("digest") != "on" {
		t.Fatal("failed settings save lost draft")
	}
	current = shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if current.Get("timezone") != "Europe/London" || current.Has("digest") {
		t.Fatal("failed settings save changed saved state")
	}
	b.post(t, retry, 303)
	current = shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if current.Get("timezone") != "America/Phoenix" || current.Get("start_page") != "activity" || current.Get("digest") != "on" {
		t.Fatal("settings retry did not save the exact retained draft")
	}
}

func TestShellDemoPermissionsAvailabilityAndSessionActions(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	old := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	oldSettings := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	b.fixture(t, "viewer", "ready", false)
	for page, field := range map[string]string{"profile": "display_name", "settings": "timezone"} {
		doc := shellDemoTestDOM(t, b.get(t, "/shell/"+page, 200).Body.String())
		found := false
		shellDemoTestWalk(doc, func(n *html.Node) {
			if n.Type != html.ElementNode || shellDemoTestAttr(n, "name") != field {
				return
			}
			found = true
			if !shellDemoTestHasAttr(n, "disabled") && !shellDemoTestHasAttr(n, "readonly") {
				t.Errorf("viewer %s control remains editable", field)
			}
		})
		if !found {
			t.Errorf("viewer cannot inspect saved %s control", field)
		}
	}
	old.Set("display_name", "Must not save")
	b.post(t, old, 403)
	b.post(t, oldSettings, 403)
	b.fixture(t, "editor", "loading", false)
	for _, page := range []string{"profile", "settings"} {
		w := b.get(t, "/shell/"+page, 200)
		if !strings.Contains(strings.ToLower(w.Body.String()), "loading") {
			t.Fatal("loading fixture has no explicit loading state")
		}
	}
	b.post(t, old, 503)
	b.post(t, oldSettings, 503)
	b.fixture(t, "editor", "error", false)
	for _, page := range []string{"profile", "settings"} {
		b.get(t, "/shell/"+page, 503)
	}
	failed := b.get(t, "/shell/profile", 503)
	b.post(t, shellDemoTestForm(t, failed.Body.String(), "retry-load"), 303)
	ready := b.get(t, "/shell/profile", 200)
	if shellDemoTestForm(t, ready.Body.String(), "save-profile").Get("display_name") != "Alex Morgan" {
		t.Fatal("permission/recovery fixtures changed saved profile")
	}
	b.post(t, shellDemoTestForm(t, ready.Body.String(), "signout"), 303)
	signedOut := b.get(t, "/shell/profile", 401)
	b.get(t, "/shell/settings", 401)
	b.post(t, old, 401)
	b.post(t, oldSettings, 401)
	b.post(t, shellDemoTestForm(t, signedOut.Body.String(), "signin"), 303)
	ready = b.get(t, "/shell/profile", 200)
	values := shellDemoTestForm(t, ready.Body.String(), "save-profile")
	values.Set("display_name", "Reset this fixture")
	b.post(t, values, 303)
	b.post(t, shellDemoTestForm(t, b.get(t, "/shell/help", 200).Body.String(), "reset-demo"), 303)
	if got := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Alex Morgan" {
		t.Fatal("reset did not restore only this browser's defaults")
	}
}

func TestShellDemoRejectsMalformedRequestsAndForeignAuthority(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	values := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	for _, query := range []string{"theme=unknown", "theme=light&theme=dark", "theme=%ff", "theme=%00", "theme=%zz", "unknown=value", "theme=a;b=c"} {
		shellDemoTestStatus(t, b.request("GET", "/shell/profile?"+query, "", nil), 400)
	}
	for _, theme := range []string{"light", "dark"} {
		w := b.get(t, "/shell/profile?theme="+theme, 200)
		if !strings.Contains(w.Body.String(), `data-theme="`+theme+`"`) {
			t.Fatal("accepted theme is not reflected in document")
		}
	}
	for _, suffix := range []string{"&unknown=value", "&display_name=duplicate", "&csrf=duplicate", "&display_name=%ff", "&display_name=%00", "&bad=%zz"} {
		shellDemoTestStatus(t, b.request("POST", "/shell/action", values.Encode()+suffix, nil), 400)
	}
	for _, value := range []string{"\xff", "name\x00suffix"} {
		bad := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
		bad.Set("display_name", value)
		b.post(t, bad, 400)
	}
	for _, version := range []string{"-1", "01", "not-a-number"} {
		bad := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
		bad.Set("version", version)
		b.post(t, bad, 400)
	}
	unknown := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	unknown.Set("action", "unknown-action")
	b.post(t, unknown, 400)
	shellDemoTestStatus(t, b.request("POST", "/shell/action?theme=light", values.Encode(), nil), 400)
	shellDemoTestStatus(t, b.request("POST", "/shell/action", values.Encode()+"&padding="+strings.Repeat("x", 8192), nil), 413)
	for _, csrf := range []string{"", "wrong-token"} {
		bad := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
		bad.Set("csrf", csrf)
		b.post(t, bad, 403)
	}
	missing := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	missing.Del("csrf")
	b.post(t, missing, 403)
	for _, target := range []string{"https://evil.example", "//evil.example", "../profile", "/shell/profile", "settings?theme=dark"} {
		bad := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
		bad.Set("return_to", target)
		b.post(t, bad, 400)
	}
	for _, headers := range []map[string]string{
		{"Host": "evil.example"}, {"Origin": "http://evil.example"}, {"Origin": "null"}, {"Origin": "http://127.0.0.1:8766"},
		{"RemoteAddr": "192.0.2.1:1234", "X-Forwarded-For": "127.0.0.1"}, {"Sec-Fetch-Site": "cross-site"},
	} {
		for _, method := range []string{"GET", "POST"} {
			path, body := "/shell/profile", ""
			if method == "POST" {
				path, body = "/shell/action", values.Encode()
			}
			shellDemoTestStatus(t, b.request(method, path, body, headers), 403)
		}
	}
	for _, method := range []string{"PUT", "DELETE", "PATCH", "OPTIONS"} {
		shellDemoTestStatus(t, b.request(method, "/shell/profile", "", nil), 405)
	}
	shellDemoTestStatus(t, b.request("GET", "/shell/action", "", nil), 405)
	shellDemoTestStatus(t, b.request("POST", "/shell/profile", values.Encode(), nil), 405)
	for _, path := range []string{"/shell/unknown", "/shell/profile/extra", "/shell/%70rofile", "/shell/profile%2fextra"} {
		shellDemoTestStatus(t, b.request("GET", path, "", nil), 404)
	}
}

func TestShellDemoSameOriginReferrerPolicyPreservesStrictOriginGuard(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	for _, path := range []string{"/shell/profile", "/shell/settings", "/shell/activity"} {
		w := b.get(t, path, http.StatusOK)
		if got := w.Header().Get("Referrer-Policy"); got != "same-origin" {
			t.Errorf("%s Referrer-Policy = %q, want same-origin", path, got)
		}
	}
	values := shellDemoTestForm(t, b.get(t, "/shell/profile", http.StatusOK).Body.String(), "save-profile")
	for _, origin := range []string{"null", "http://evil.example"} {
		shellDemoTestStatus(t, b.request(http.MethodPost, "/shell/action", values.Encode(), map[string]string{"Origin": origin}), http.StatusForbidden)
	}
	shellDemoTestStatus(t, b.request(http.MethodPost, "/shell/action", values.Encode(), map[string]string{"Origin": "http://127.0.0.1:8765"}), http.StatusSeeOther)
}

func TestShellDemoHostileDraftStaysText(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	values := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	hostile := `<img src=x onerror=fixture()>`
	values.Set("display_name", hostile)
	b.post(t, values, 303)
	w := b.get(t, "/shell/profile", 200)
	doc := shellDemoTestDOM(t, w.Body.String())
	if shellDemoTestForm(t, w.Body.String(), "save-profile").Get("display_name") != hostile {
		t.Fatal("saved text was silently rewritten")
	}
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" && shellDemoTestAttr(n, "src") == "x" {
			t.Fatal("hostile profile text became an active element")
		}
	})
	for name, cookie := range b.cookies {
		if strings.Contains(cookie.Value, hostile) {
			t.Fatalf("session cookie %s embeds user profile data", name)
		}
	}
}

func TestShellDemoAboutDiagnosticsExcludeSessionData(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	values := shellDemoTestForm(t, b.get(t, "/shell/profile", 200).Body.String(), "save-profile")
	values.Set("display_name", "Synthetic Secret Name")
	b.post(t, values, 303)
	doc := shellDemoTestDOM(t, b.get(t, "/shell/about", 200).Body.String())
	var diagnostics string
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "textarea" && shellDemoTestHasAttr(n, "readonly") {
			diagnostics = shellDemoTestText(n)
		}
	})
	if diagnostics == "" || !strings.Contains(diagnostics, "Build:") || !strings.Contains(diagnostics, "Runtime:") {
		t.Fatal("about page lacks selectable, read-only build/runtime diagnostics")
	}
	for _, secret := range []string{"Synthetic Secret Name", values.Get("csrf"), b.cookies["appkit_shell_demo"].Value, "alex@example.test", "/Volumes/", "/Users/"} {
		if secret != "" && strings.Contains(diagnostics, secret) {
			t.Fatal("diagnostic copy text includes account/session/local-path data")
		}
	}
}

func TestShellDemoSessionCapacityCookiesAndNonce(t *testing.T) {
	h := shellDemoTestHandler()
	first := shellDemoTestNewBrowser(h)
	page := first.get(t, "/shell/profile", 200)
	next := first.get(t, "/shell/profile", 200)
	if page.Header().Get("Content-Security-Policy") == next.Header().Get("Content-Security-Policy") {
		t.Fatal("separate responses reused a CSP nonce")
	}
	values := shellDemoTestForm(t, page.Body.String(), "save-profile")
	for _, contentType := range []string{"text/plain", "application/json", "application/x-www-form-urlencoded; charset=iso-8859-1"} {
		shellDemoTestStatus(t, first.request("POST", "/shell/action", values.Encode(), map[string]string{"Content-Type": contentType}), 415)
	}
	cookie := first.cookies["appkit_shell_demo"]
	duplicate := cookie.Name + "=" + cookie.Value + "; " + cookie.Name + "=" + cookie.Value
	shellDemoTestStatus(t, first.request("GET", "/shell/profile", "", map[string]string{"Cookie": duplicate}), 400)
	// A caller-chosen unknown cookie must not become the newly issued identity.
	fixed := strings.Repeat("0", 64)
	second := shellDemoTestNewBrowser(h)
	w := second.request("GET", "/shell/profile", "", map[string]string{"Cookie": "appkit_shell_demo=" + fixed})
	shellDemoTestStatus(t, w, 200)
	if issued := second.cookies["appkit_shell_demo"]; issued == nil || issued.Value == fixed {
		t.Fatal("caller-selected session identifier was adopted")
	}
	// The documented process-memory fixture has 128 slots, not an unbounded map.
	for count := 2; count < 128; count++ {
		shellDemoTestNewBrowser(h).get(t, "/shell/about", 200)
	}
	overflow := shellDemoTestNewBrowser(h)
	overflow.get(t, "/shell/about", 503)
	if len(overflow.cookies) != 0 {
		t.Fatal("full store issued an unusable session cookie")
	}
	// Refusing new sessions must not evict or change an existing browser's work.
	values.Set("display_name", "Capacity survivor")
	first.post(t, values, 303)
	if got := shellDemoTestForm(t, first.get(t, "/shell/profile", 200).Body.String(), "save-profile").Get("display_name"); got != "Capacity survivor" {
		t.Fatal("session-capacity refusal damaged an existing session")
	}
}

func TestShellDemoDigestErrorIsLinkedAndCanonicalDraftCanRetry(t *testing.T) {
	b := shellDemoTestNewBrowser(shellDemoTestHandler())
	values := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	values.Del("digest")
	b.post(t, values, 303)
	saved := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	values = shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	values.Set("digest", "bogus")
	w := b.post(t, values, 422)
	doc := shellDemoTestDOM(t, w.Body.String())
	checkbox, errorText, errorLink, retryForm := false, false, false, false
	shellDemoTestWalk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch {
		case n.Data == "input" && shellDemoTestAttr(n, "name") == "digest":
			checkbox = true
			if shellDemoTestAttr(n, "id") != "shell-digest" || shellDemoTestAttr(n, "aria-invalid") != "true" || shellDemoTestAttr(n, "aria-describedby") != "shell-digest-error" {
				t.Fatal("digest error is not associated with its invalid checkbox")
			}
		case shellDemoTestAttr(n, "id") == "shell-digest-error":
			errorText = strings.TrimSpace(shellDemoTestText(n)) != ""
			if shellDemoTestHasAttr(n, "hidden") || shellDemoTestAttr(n, "aria-hidden") == "true" {
				t.Fatal("digest validation message is hidden")
			}
		case n.Data == "a" && shellDemoTestAttr(n, "href") == "#shell-digest":
			errorLink = strings.TrimSpace(shellDemoTestText(n)) != ""
		case n.Data == "form" && shellDemoTestHasAttr(n, "data-shell-edit"):
			retryForm = shellDemoTestAttr(n, "data-shell-retry") == "true"
		}
	})
	if !checkbox || !errorText || !errorLink || !retryForm {
		t.Fatal("digest validation lost its visible linked error or explicit retry contract")
	}
	// An invalid checkbox value renders unchecked, matching the saved false value.
	// The retry marker must still allow an explicit submission in the browser.
	retry := shellDemoTestForm(t, w.Body.String(), "save-settings")
	if retry.Encode() != saved.Encode() {
		t.Fatal("canary no longer has identical canonical returned and saved fields")
	}
	current := shellDemoTestForm(t, b.get(t, "/shell/settings", 200).Body.String(), "save-settings")
	if current.Encode() != saved.Encode() {
		t.Fatal("invalid digest submission changed saved preferences/version")
	}
	b.post(t, retry, 303)
}

func TestShellDemoDeniedDraftRemainsExactlyCopyable(t *testing.T) {
	for _, state := range []string{"viewer", "loading", "error", "signedout"} {
		for _, page := range []string{"profile", "settings"} {
			t.Run(state+"/"+page, func(t *testing.T) {
				b := shellDemoTestNewBrowser(shellDemoTestHandler())
				action := "save-" + page
				ready := b.get(t, "/shell/"+page, 200)
				values := shellDemoTestForm(t, ready.Body.String(), action)
				want := ""
				if page == "profile" {
					values.Set("display_name", "  Draft </textarea><script>fixture()</script>&\nsecond line  ")
					want = "Display name: " + values.Get("display_name")
				} else {
					values.Set("timezone", "Europe/London")
					values.Set("start_page", "activity")
					values.Set("digest", "bogus</textarea><img src=x onerror=fixture()>")
					want = "Time zone: Europe/London\nStarting page: activity\nDigest submitted value: " + values.Get("digest")
				}
				status := 503
				switch state {
				case "signedout":
					b.post(t, shellDemoTestForm(t, ready.Body.String(), "signout"), 303)
					status = 401
				case "viewer":
					b.fixture(t, "viewer", "ready", false)
					status = 403
				default:
					b.fixture(t, "editor", state, false)
				}
				denied := b.post(t, values, status)
				doc := shellDemoTestDOM(t, denied.Body.String())
				found := false
				shellDemoTestWalk(doc, func(n *html.Node) {
					if n.Type != html.ElementNode {
						return
					}
					if shellDemoTestAttr(n, "id") == "shell-retained-draft" {
						found = true
						if n.Data != "textarea" || !shellDemoTestHasAttr(n, "readonly") || shellDemoTestText(n) != want {
							t.Fatalf("retained draft differs from exact submission: %q, want %q", shellDemoTestText(n), want)
						}
					}
					if n.Data == "img" && shellDemoTestAttr(n, "src") == "x" || n.Data == "script" && strings.Contains(shellDemoTestText(n), "fixture()") {
						t.Fatal("retained draft became active HTML")
					}
				})
				if !found || !strings.Contains(denied.Body.String(), "Copy this draft") || !strings.Contains(denied.Body.String(), "recovery navigation reloads saved values, not this submission") {
					t.Fatal("denied submission hides its copyable draft or misstates recovery persistence")
				}
				switch state {
				case "signedout":
					b.post(t, shellDemoTestForm(t, denied.Body.String(), "signin"), 303)
				case "viewer":
					b.fixture(t, "editor", "ready", false)
				default:
					b.post(t, shellDemoTestForm(t, denied.Body.String(), "retry-load"), 303)
				}
				current := shellDemoTestForm(t, b.get(t, "/shell/"+page, 200).Body.String(), action)
				if page == "profile" && current.Get("display_name") != "Alex Morgan" || page == "settings" && (current.Get("timezone") != "America/Phoenix" || current.Get("start_page") != "home" || current.Get("digest") != "on") {
					t.Fatal("denied submission or recovery overwrote the original saved values")
				}
			})
		}
	}
}

func TestShellDemoIntegratedDashboardAndShellHaveSingleLandmark(t *testing.T) {
	b := shellDemoTestNewBrowser(New())
	for _, path := range []string{"/dashboard", "/shell"} {
		t.Run(path, func(t *testing.T) {
			doc := shellDemoTestDOM(t, b.get(t, path, 200).Body.String())
			shellDemoTestWalk(doc, func(n *html.Node) {
				if n.Type != html.ElementNode || n.Data != "a" {
					return
				}
				if href := shellDemoTestAttr(n, "href"); href == "" || href == "#" {
					t.Fatalf("integrated %s contains an inert navigation link", path)
				}
			})
		})
	}
}
