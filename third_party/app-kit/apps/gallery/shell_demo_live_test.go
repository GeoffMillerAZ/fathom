package gallery

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Exercise real HTTP transport without treating it as browser execution. The
// optional external probe creates its own synthetic sessions, never imports a
// browser cookie, follows a redirect, or changes another session's fixture.
func TestShellDemoLoopbackJourney(t *testing.T) {
	s := httptest.NewServer(New())
	defer s.Close()
	shellDemoLoopbackJourney(t, s.URL)
}

func TestShellDemoLiveJourney(t *testing.T) {
	base := os.Getenv("APPKIT_PROBE_SHELL_URL")
	if base == "" {
		t.Skip("set APPKIT_PROBE_SHELL_URL to an explicit loopback Gallery origin for a live synthetic-session probe")
	}
	shellDemoLoopbackJourney(t, base)
}

func shellDemoLoopbackJourney(t *testing.T, base string) {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		t.Fatal("probe requires an HTTP loopback origin with an explicit port and no credentials, path, query or fragment")
	}
	ip, port := net.ParseIP(u.Hostname()), u.Port()
	n, err := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		t.Fatal("probe requires a numeric loopback IP and canonical port")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started, requests := time.Now(), 0
	newClient := func() *http.Client {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal("could not create isolated synthetic cookie jar")
		}
		transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext}
		t.Cleanup(transport.CloseIdleConnections)
		return &http.Client{Jar: jar, Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	a, b := newClient(), newClient()
	request := func(client *http.Client, path string, values url.Values, want int) string {
		t.Helper()
		method, body := http.MethodGet, ""
		if values != nil {
			method, body = http.MethodPost, values.Encode()
		}
		r, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal("could not construct synthetic probe request")
		}
		if values != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", base)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatalf("synthetic %s %s transport failed", method, path)
		}
		defer response.Body.Close()
		requests++
		data, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
		if err != nil || len(data) > 256<<10 || response.StatusCode != want {
			t.Fatalf("synthetic %s %s status=%d want=%d or response exceeded bounds", method, path, response.StatusCode, want)
		}
		if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("synthetic session response lost privacy headers")
		}
		if want == http.StatusSeeOther {
			location := response.Header.Get("Location")
			if location != shellDemoPath(values.Get("return_to")) {
				t.Fatal("synthetic action redirected somewhere other than its explicit local return page")
			}
		} else if want == http.StatusOK || want == http.StatusConflict || want == http.StatusUnprocessableEntity || want == http.StatusServiceUnavailable {
			shellDemoTestDOM(t, string(data))
		}
		return string(data)
	}
	get := func(client *http.Client, path string) string { return request(client, path, nil, 200) }
	post := func(values url.Values, status int) string { return request(a, "/shell/action", values, status) }
	profile := get(a, "/shell/profile")
	first := shellDemoTestForm(t, profile, "save-profile")
	stale := shellDemoTestForm(t, profile, "save-profile")
	other := shellDemoTestForm(t, get(b, "/shell/profile"), "save-profile")
	if first.Get("csrf") == other.Get("csrf") || first.Get("display_name") != "Alex Morgan" || other.Get("display_name") != "Alex Morgan" {
		t.Fatal("live fixture is not fresh and session-isolated")
	}
	first.Set("display_name", strings.Repeat("x", 81))
	invalid := post(first, 422)
	first = shellDemoTestForm(t, invalid, "save-profile")
	if first.Get("display_name") != strings.Repeat("x", 81) {
		t.Fatal("live validation discarded the draft")
	}
	first.Set("display_name", "Synthetic Transport Avery")
	post(first, 303)
	stale.Set("display_name", "Synthetic Reviewed Casey")
	conflict := post(stale, 409)
	if !strings.Contains(conflict, "Apply reviewed draft") {
		t.Fatal("live conflict omitted explicit review")
	}
	post(shellDemoTestForm(t, conflict, "save-profile"), 303)
	if shellDemoTestForm(t, get(a, "/shell/profile"), "save-profile").Get("display_name") != "Synthetic Reviewed Casey" || shellDemoTestForm(t, get(b, "/shell/profile"), "save-profile").Get("display_name") != "Alex Morgan" {
		t.Fatal("live reviewed save or session isolation failed")
	}
	fixture := func(access string, fail bool) {
		values := shellDemoTestForm(t, get(a, "/shell/help"), "fixture")
		values.Set("access", access)
		values.Set("availability", "ready")
		values.Del("fail_next_save")
		if fail {
			values.Set("fail_next_save", "on")
		}
		post(values, 303)
	}
	fixture("editor", true)
	settingsPage := get(a, "/shell/settings")
	baseline := shellDemoTestForm(t, settingsPage, "save-settings")
	settings := shellDemoTestForm(t, settingsPage, "save-settings")
	settings.Set("timezone", "Europe/London")
	settings.Set("start_page", "activity")
	settings.Del("digest")
	failed := post(settings, 503)
	assertUnchanged := func(before url.Values) {
		t.Helper()
		after := shellDemoTestForm(t, get(a, "/shell/settings"), "save-settings")
		for _, field := range []string{"timezone", "start_page", "digest", "version"} {
			if after.Get(field) != before.Get(field) {
				t.Fatal("refused live save mutated persisted synthetic settings or version")
			}
		}
	}
	assertUnchanged(baseline)
	post(shellDemoTestForm(t, failed, "save-settings"), 303)
	saved := shellDemoTestForm(t, get(a, "/shell/settings"), "save-settings")
	if saved.Get("timezone") != "Europe/London" || saved.Get("start_page") != "activity" || saved.Has("digest") {
		t.Fatal("live failed-save retry lost settings")
	}
	fixture("viewer", false)
	baseline = shellDemoTestForm(t, get(a, "/shell/settings"), "save-settings")
	saved.Set("timezone", "UTC")
	post(saved, 403)
	assertUnchanged(baseline)
	fixture("editor", false)
	page := get(a, "/shell/profile")
	pending := shellDemoTestForm(t, page, "save-profile")
	post(shellDemoTestForm(t, page, "signout"), 303)
	post(pending, 401)
	signedOut := request(a, "/shell/profile", nil, 401)
	post(shellDemoTestForm(t, signedOut, "signin"), 303)
	for _, path := range []string{"/shell", "/shell/activity", "/shell/help", "/shell/about"} {
		get(a, path)
	}
	post(shellDemoTestForm(t, get(a, "/shell/settings"), "reset-demo"), 303)
	reset := shellDemoTestForm(t, get(a, "/shell/settings"), "save-settings")
	if reset.Get("timezone") != "America/Phoenix" || reset.Get("start_page") != "home" || reset.Get("digest") != "on" {
		t.Fatal("live synthetic reset did not restore this session's defaults")
	}
	t.Logf("loopback HTTP journey: requests=%d isolated_sessions=2 validation/conflict/retry/permission/signout/reset=pass elapsed=%s; no browser or production-backend claim", requests, time.Since(started).Round(time.Millisecond))
}
