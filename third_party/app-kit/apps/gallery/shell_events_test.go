package gallery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// These tests use actual loopback HTTP and isolated synthetic cookie jars. They
// do not execute Datastar, a browser, consumer code, or a production backend.
type shellEventsTestSnapshot struct {
	Version      string   `json:"version"`
	DisplayName  string   `json:"display_name"`
	TimeZone     string   `json:"time_zone"`
	StartPage    string   `json:"start_page"`
	Digest       bool     `json:"digest"`
	Access       string   `json:"access"`
	Availability string   `json:"availability"`
	Activity     []string `json:"activity"`
}

type shellEventsTestFrame struct {
	id, event, data string
	err             error
}

type shellEventsTestStream struct {
	frames <-chan shellEventsTestFrame
	close  func()
}

func shellEventsTestClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Jar: jar, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func shellEventsTestPage(t *testing.T, client *http.Client, base, path string, values url.Values, status int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	method, body := http.MethodGet, ""
	if values != nil {
		method, body = http.MethodPost, values.Encode()
	}
	r, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if values != nil {
		r.Header.Set("Origin", base)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
	if err != nil || len(data) > 256<<10 || response.StatusCode != status {
		t.Fatalf("%s %s: status=%d want=%d bytes=%d err=%v", method, path, response.StatusCode, status, len(data), err)
	}
	if status == http.StatusSeeOther && response.Header.Get("Location") != shellDemoPath(values.Get("return_to")) {
		t.Fatal("existing action redirect changed")
	}
	return string(data)
}

func shellEventsTestOpen(t *testing.T, client *http.Client, base string, headers http.Header) shellEventsTestStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/shell/events", nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	r.Header.Set("Accept", "text/event-stream")
	for key, values := range headers {
		r.Header[key] = append([]string{}, values...)
	}
	// A separate header timer makes an unflushed initial response fail promptly.
	timer := time.AfterFunc(3*time.Second, cancel)
	response, err := client.Do(r)
	timer.Stop()
	if err != nil {
		cancel()
		t.Fatalf("SSE initial headers were not flushed: %v", err)
	}
	closeStream := func() { cancel(); _ = response.Body.Close() }
	t.Cleanup(closeStream)
	if response.StatusCode != http.StatusOK {
		closeStream()
		t.Fatalf("open stream status=%d, want 200", response.StatusCode)
	}
	for name, want := range map[string]string{
		"Content-Type": "text/event-stream", "Cache-Control": "no-store",
		"X-Content-Type-Options": "nosniff", "X-AppKit-Stream-Mode": "snapshot",
	} {
		if got := response.Header.Get(name); got != want {
			closeStream()
			t.Fatalf("stream %s=%q, want %q", name, got, want)
		}
	}
	if len(response.Header.Values("Set-Cookie")) != 0 {
		closeStream()
		t.Fatal("stream allocated or refreshed a cookie")
	}
	frames := make(chan shellEventsTestFrame, 8)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 1024), 8193)
		for {
			frame, size := shellEventsTestFrame{}, 0
			terminated := false
			seen := map[string]bool{}
			for scanner.Scan() {
				line := scanner.Text()
				size += len(line) + 1
				if size > 8<<10 {
					frame.err = fmt.Errorf("SSE frame exceeds 8 KiB")
					break
				}
				if line == "" {
					terminated = true
					break
				}
				name, value, ok := strings.Cut(line, ":")
				if !ok || seen[name] || (name != "id" && name != "event" && name != "data") {
					frame.err = fmt.Errorf("unexpected or duplicate SSE field %q", name)
					break
				}
				seen[name] = true
				value = strings.TrimPrefix(value, " ")
				switch name {
				case "id":
					frame.id = value
				case "event":
					frame.event = value
				case "data":
					frame.data = value
				}
			}
			if size == 0 || scanner.Err() != nil {
				frame.err = scanner.Err()
				if frame.err == nil {
					frame.err = io.EOF
				}
			} else if !terminated && frame.err == nil {
				frame.err = io.ErrUnexpectedEOF
			}
			select {
			case frames <- frame:
			case <-ctx.Done():
				return
			}
			if frame.err != nil {
				return
			}
		}
	}()
	return shellEventsTestStream{frames: frames, close: closeStream}
}

func shellEventsTestNext(t *testing.T, stream shellEventsTestStream) shellEventsTestFrame {
	t.Helper()
	select {
	case frame, ok := <-stream.frames:
		if !ok || frame.err != nil {
			t.Fatalf("stream ended before expected snapshot: open=%v err=%v", ok, frame.err)
		}
		return frame
	case <-time.After(3 * time.Second):
		stream.close()
		t.Fatal("stream did not flush the expected snapshot within 3 seconds")
		return shellEventsTestFrame{}
	}
}

func shellEventsTestExpect(t *testing.T, stream shellEventsTestStream, want shellEventsTestSnapshot, secrets ...string) {
	t.Helper()
	frame := shellEventsTestNext(t, stream)
	if frame.event != "datastar-patch-signals" || frame.id != want.Version || !strings.HasPrefix(frame.data, "signals ") {
		t.Fatalf("unexpected SSE envelope: %+v", frame)
	}
	raw := strings.TrimPrefix(frame.data, "signals ")
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil || compact.String() != raw {
		t.Fatal("snapshot data is not one compact JSON value")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope) != 1 || envelope["appkitShellProbe"] == nil {
		t.Fatal("snapshot changed its bounded signal namespace")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(envelope["appkitShellProbe"], &fields); err != nil || len(fields) != 8 {
		t.Fatal("snapshot fields are missing or disclose undeclared state")
	}
	for _, key := range []string{"version", "display_name", "time_zone", "start_page", "digest", "access", "availability", "activity"} {
		if fields[key] == nil {
			t.Fatalf("snapshot missing %s", key)
		}
	}
	var got shellEventsTestSnapshot
	if err := json.Unmarshal(envelope["appkitShellProbe"], &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot=%+v, independent expected=%+v: %v", got, want, err)
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(raw, secret) {
			t.Fatal("snapshot disclosed a session or CSRF secret")
		}
	}
}

func shellEventsTestQuiet(t *testing.T, stream shellEventsTestStream) {
	t.Helper()
	select {
	case frame := <-stream.frames:
		t.Fatalf("unexpected stream notification or closure: %+v", frame)
	case <-time.After(100 * time.Millisecond):
	}
}

func shellEventsTestRedactedClose(t *testing.T, stream shellEventsTestStream) {
	t.Helper()
	redacted := shellEventsTestNext(t, stream)
	if redacted.id != "" || redacted.event != "datastar-patch-signals" || redacted.data != `signals {"appkitShellProbe":null}` {
		t.Fatalf("stream did not send only terminal redaction: %+v", redacted)
	}
	select {
	case frame, ok := <-stream.frames:
		if ok && frame.err != io.EOF {
			t.Fatalf("redacted stream did not close cleanly: %+v", frame)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("redacted stream remained open")
	}
}

func TestShellEventsWireJourney(t *testing.T) {
	server := httptest.NewServer(New())
	t.Cleanup(server.Close)
	a, b := shellEventsTestClient(t), shellEventsTestClient(t)
	profile := shellEventsTestPage(t, a, server.URL, "/shell/profile", nil, 200)
	save := shellDemoTestForm(t, profile, "save-profile")
	stale := shellDemoTestForm(t, profile, "save-profile")
	foreignProfile := shellEventsTestPage(t, b, server.URL, "/shell/profile", nil, 200)
	foreignSave := shellDemoTestForm(t, foreignProfile, "save-profile")
	if save.Get("csrf") == foreignSave.Get("csrf") {
		t.Fatal("synthetic sessions are not independent")
	}
	want := shellEventsTestSnapshot{"1", "Alex Morgan", "America/Phoenix", "home", true, "editor", "ready",
		[]string{"Started a synthetic session. No real account was accessed."}}
	foreignWant := want
	first, second, foreign := shellEventsTestOpen(t, a, server.URL, nil), shellEventsTestOpen(t, a, server.URL, nil), shellEventsTestOpen(t, b, server.URL, nil)
	defer first.close()
	defer second.close()
	defer foreign.close()
	shellEventsTestExpect(t, first, want, save.Get("csrf"))
	shellEventsTestExpect(t, second, want, save.Get("csrf"))
	shellEventsTestExpect(t, foreign, foreignWant, foreignSave.Get("csrf"))

	save.Set("display_name", `Avery <review> & "Morgan"`)
	shellEventsTestPage(t, a, server.URL, "/shell/action", save, 303)
	want.Version, want.DisplayName = "2", `Avery <review> & "Morgan"`
	want.Activity = append(want.Activity, "Profile saved in this synthetic session only.")
	shellEventsTestExpect(t, first, want, save.Get("csrf"))
	shellEventsTestExpect(t, second, want, save.Get("csrf"))
	stale.Set("display_name", "Stale draft must not become a snapshot")
	shellEventsTestPage(t, a, server.URL, "/shell/action", stale, 409)
	shellEventsTestQuiet(t, first)
	shellEventsTestQuiet(t, foreign)

	second.close()
	settings := shellDemoTestForm(t, shellEventsTestPage(t, a, server.URL, "/shell/settings", nil, 200), "save-settings")
	settings.Set("timezone", "Europe/London")
	settings.Set("start_page", "activity")
	settings.Del("digest")
	shellEventsTestPage(t, a, server.URL, "/shell/action", settings, 303)
	want.Version, want.TimeZone, want.StartPage, want.Digest = "3", "Europe/London", "activity", false
	want.Activity = append(want.Activity, "Settings saved in this synthetic session only.")
	shellEventsTestExpect(t, first, want)
	for _, last := range []string{"1", "18446744073709551615", ""} {
		reconnected := shellEventsTestOpen(t, a, server.URL, http.Header{"Last-Event-Id": {last}})
		shellEventsTestExpect(t, reconnected, want)
		reconnected.close()
	}

	signout := shellDemoTestForm(t, shellEventsTestPage(t, a, server.URL, "/shell/profile", nil, 200), "signout")
	shellEventsTestPage(t, a, server.URL, "/shell/action", signout, 303)
	shellEventsTestRedactedClose(t, first)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/shell/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Accept", "text/event-stream")
	response, err := a.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	refusal, err := io.ReadAll(io.LimitReader(response.Body, (8<<10)+1))
	if err != nil || len(refusal) > 8<<10 || response.StatusCode != 401 || response.Header.Get("Set-Cookie") != "" || strings.Contains(string(refusal), "Avery") {
		t.Fatal("signed-out session reopened or refreshed a stream, or disclosed saved state")
	}
	foreignSave.Set("display_name", "Independent Blake")
	shellEventsTestPage(t, b, server.URL, "/shell/action", foreignSave, 303)
	foreignWant.Version, foreignWant.DisplayName = "2", "Independent Blake"
	foreignWant.Activity = append(foreignWant.Activity, "Profile saved in this synthetic session only.")
	shellEventsTestExpect(t, foreign, foreignWant)
}

func TestShellEventsWireRequestRefusals(t *testing.T) {
	server := httptest.NewServer(New())
	t.Cleanup(server.Close)
	client := shellEventsTestClient(t)
	shellEventsTestPage(t, client, server.URL, "/shell/profile", nil, 200)
	u, err := url.Parse(server.URL + "/shell/events")
	if err != nil {
		t.Fatal(err)
	}
	cookies := client.Jar.Cookies(u)
	if len(cookies) != 1 {
		t.Fatal("expected one synthetic session cookie")
	}
	cookie := cookies[0].Name + "=" + cookies[0].Value
	for _, tc := range []struct {
		name, method, path, body, cookie, host string
		headers                                http.Header
		status                                 int
	}{
		{name: "missing_cookie", status: 403},
		{name: "unknown_cookie", cookie: "appkit_shell_demo=" + strings.Repeat("a", 64), status: 403},
		{name: "duplicate_cookie", cookie: cookie + "; " + cookie, status: 400},
		{name: "query", cookie: cookie, path: "/shell/events?theme=dark", status: 400},
		{name: "empty_query", cookie: cookie, path: "/shell/events?", status: 400},
		{name: "encoded_path", cookie: cookie, path: "/shell/%65vents", status: 404},
		{name: "descendant", cookie: cookie, path: "/shell/events/extra", status: 404},
		{name: "body", cookie: cookie, body: "not-empty", status: 400},
		{name: "head", cookie: cookie, method: "HEAD", status: 405},
		{name: "post", cookie: cookie, method: "POST", status: 405},
		{name: "missing_accept", cookie: cookie, headers: http.Header{"Accept": nil}, status: 406},
		{name: "wildcard_accept", cookie: cookie, headers: http.Header{"Accept": {"*/*"}}, status: 406},
		{name: "accept_parameter", cookie: cookie, headers: http.Header{"Accept": {"text/event-stream; charset=utf-8"}}, status: 406},
		{name: "accept_list", cookie: cookie, headers: http.Header{"Accept": {"text/event-stream, text/html"}}, status: 406},
		{name: "duplicate_accept", cookie: cookie, headers: http.Header{"Accept": {"text/event-stream", "text/event-stream"}}, status: 406},
		{name: "zero_cursor", cookie: cookie, headers: http.Header{"Last-Event-Id": {"0"}}, status: 400},
		{name: "noncanonical_cursor", cookie: cookie, headers: http.Header{"Last-Event-Id": {"01"}}, status: 400},
		{name: "negative_cursor", cookie: cookie, headers: http.Header{"Last-Event-Id": {"-1"}}, status: 400},
		{name: "overflow_cursor", cookie: cookie, headers: http.Header{"Last-Event-Id": {"18446744073709551616"}}, status: 400},
		{name: "duplicate_cursor", cookie: cookie, headers: http.Header{"Last-Event-Id": {"1", "2"}}, status: 400},
		{name: "foreign_origin", cookie: cookie, headers: http.Header{"Origin": {"http://example.invalid"}}, status: 403},
		{name: "null_origin", cookie: cookie, headers: http.Header{"Origin": {"null"}}, status: 403},
		{name: "duplicate_origin", cookie: cookie, headers: http.Header{"Origin": {server.URL, server.URL}}, status: 403},
		{name: "cross_site", cookie: cookie, headers: http.Header{"Sec-Fetch-Site": {"cross-site"}}, status: 403},
		{name: "foreign_host", cookie: cookie, host: "example.invalid", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			method, path := tc.method, tc.path
			if method == "" {
				method = "GET"
			}
			if path == "" {
				path = "/shell/events"
			}
			r, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Accept", "text/event-stream")
			if tc.cookie != "" {
				r.Header.Set("Cookie", tc.cookie)
			}
			for key, values := range tc.headers {
				r.Header[key] = values
			}
			if tc.host != "" {
				r.Host = tc.host
			}
			// No jar: each negative case supplies its exact cookie headers.
			response, err := (&http.Client{Transport: client.Transport, CheckRedirect: client.CheckRedirect}).Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(io.LimitReader(response.Body, (8<<10)+1))
			if err != nil || len(body) > 8<<10 || response.StatusCode != tc.status {
				t.Fatalf("status=%d want=%d bytes=%d err=%v", response.StatusCode, tc.status, len(body), err)
			}
			if response.Header.Get("Set-Cookie") != "" || strings.Contains(string(body), cookies[0].Value) || strings.Contains(string(body), "Alex Morgan") {
				t.Fatal("rejected stream allocated a session or disclosed saved state")
			}
			if tc.status == 405 && response.Header.Get("Allow") != "GET" {
				t.Fatal("stream method refusal omitted Allow: GET")
			}
		})
	}
	// A forged forwarded address does not replace the actual peer guard. The
	// application HTTP transport always has a real loopback peer in this test.
	r := httptest.NewRequest("GET", "http://127.0.0.1:8765/shell/events", nil)
	r.RemoteAddr = "192.0.2.10:4321"
	r.Header.Set("Accept", "text/event-stream")
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	New().ServeHTTP(w, r)
	if w.Code != 403 || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("non-loopback peer bypassed the stream authority guard")
	}
}

func TestShellEventsWireViewerSnapshot(t *testing.T) {
	server := httptest.NewServer(New())
	t.Cleanup(server.Close)
	client := shellEventsTestClient(t)
	page := shellEventsTestPage(t, client, server.URL, "/shell/help", nil, 200)
	fixture := shellDemoTestForm(t, page, "fixture")
	fixture.Set("access", "viewer")
	fixture.Set("availability", "loading")
	fixture.Set("fail_next_save", "on")
	shellEventsTestPage(t, client, server.URL, "/shell/action", fixture, 303)
	stream := shellEventsTestOpen(t, client, server.URL, nil)
	defer stream.close()
	shellEventsTestExpect(t, stream, shellEventsTestSnapshot{"2", "Alex Morgan", "America/Phoenix", "home", true, "viewer", "loading", []string{
		"Started a synthetic session. No real account was accessed.",
		"Simulation controls applied. This does not change real authorization or service availability.",
	}})
}

func TestShellEventsWireCancellationAndFixedExpiry(t *testing.T) {
	for _, mode := range []string{"cancel", "expire"} {
		t.Run(mode, func(t *testing.T) {
			// Only this test supplies a private store, to observe resource release
			// and shorten expiry. The journey above exercises the real New router.
			session := &shellDemoSession{State: defaultShellDemoState(9007199254740993), CSRF: "private-wire-test-csrf"}
			const id = "private-wire-test-session"
			store := &shellDemoStore{sessions: map[string]*shellDemoSession{id: session}}
			mux := http.NewServeMux()
			mux.HandleFunc("/shell/events", store.serveShellEvents)
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			client := shellEventsTestClient(t)
			u, err := url.Parse(server.URL + "/shell/events")
			if err != nil {
				t.Fatal(err)
			}
			client.Jar.SetCookies(u, []*http.Cookie{{Name: "appkit_shell_demo", Value: id, Path: "/shell"}})
			expires := time.Now().Add(time.Hour)
			if mode == "expire" {
				expires = time.Now().Add(2 * time.Second)
			}
			session.Expires = expires
			stream := shellEventsTestOpen(t, client, server.URL, nil)
			shellEventsTestExpect(t, stream, shellEventsTestSnapshot{
				"9007199254740993", "Alex Morgan", "America/Phoenix", "home", true, "editor", "ready",
				[]string{"Started a synthetic session. No real account was accessed."},
			}, session.CSRF, id)
			store.mu.Lock()
			admitted := store.streams == 1 && session.streams == 1 && session.Expires.Equal(expires) && len(store.sessions) == 1
			store.mu.Unlock()
			if !admitted {
				t.Fatal("opening the stream did not admit exactly one slot or altered session lifetime")
			}
			if mode == "cancel" {
				stream.close()
			} else {
				shellEventsTestRedactedClose(t, stream)
				if time.Now().Before(expires) {
					t.Fatal("stream expired before its fixed deadline")
				}
			}
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			for {
				store.mu.Lock()
				released := store.streams == 0 && session.streams == 0
				preserved := session.Expires.Equal(expires) && len(store.sessions) == 1 && store.sessions[id] == session
				store.mu.Unlock()
				if !preserved {
					t.Fatal("stream allocated, replaced, or refreshed a session")
				}
				if released {
					break
				}
				select {
				case <-deadline.C:
					t.Fatal("closed HTTP stream retained its global or session slot")
				case <-tick.C:
				}
			}
		})
	}
}
