package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func bootstrapServer(t *testing.T, next http.Handler) (*bootstrap, *httptest.Server, *http.Client) {
	t.Helper()
	server := httptest.NewUnstartedServer(next)
	b, err := newBootstrap(server.Listener.Addr().String(), next)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = b
	server.Start()
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	return b, server, client
}

func launchForTest(t *testing.T, b *bootstrap) string {
	t.Helper()
	u, err := b.LaunchURL()
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func responseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestBootstrapCanonicalHost(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, host := range []string{"", "localhost:1234", "127.0.0.2:1234", "0.0.0.0:1234", "[::1]:1234", "http://127.0.0.1:1234", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:-1", "127.0.0.1:+1234", "127.0.0.1:01234", "127.0.0.1:1234/", "127.0.0.1:1234?x=1"} {
		t.Run(host, func(t *testing.T) {
			if _, err := newBootstrap(host, next); err == nil {
				t.Fatal("accepted noncanonical host")
			}
		})
	}
	if _, err := newBootstrap("127.0.0.1:1234", nil); err == nil {
		t.Fatal("accepted nil application")
	}
}

func TestBootstrapBrowserSessionAndHTMLPost(t *testing.T) {
	var calls atomic.Int32
	b, server, client := bootstrapServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "csrf=owned-by-gallery&version=3" || r.Header.Get("X-AppKit-Cmd") != "" {
				t.Errorf("HTML form was changed: %q", body)
			}
		}
		_, _ = w.Write([]byte("synthetic application"))
	}))
	launch := launchForTest(t, b)
	response, err := client.Get(launch)
	if err != nil {
		t.Fatal("launch transport failed")
	}
	body := responseBody(t, response)
	token := strings.TrimPrefix(launch, b.origin+launchPath+"?token=")
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/shell/profile" || calls.Load() != 0 {
		t.Fatalf("bootstrap response %d, location %q, handler calls %d", response.StatusCode, response.Header.Get("Location"), calls.Load())
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" || strings.Contains(body+fmt.Sprint(response.Header), token) || strings.Contains(body, b.session) {
		t.Fatal("bootstrap leaked a credential or omitted privacy headers")
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" || cookies[0].Domain != "" || cookies[0].MaxAge != 0 || !cookies[0].Expires.IsZero() || !strings.HasSuffix(cookies[0].Name, strings.TrimPrefix(b.host, "127.0.0.1:")) {
		t.Fatal("incorrect session cookie attributes")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		var body io.Reader
		if method == http.MethodPost {
			body = strings.NewReader("csrf=owned-by-gallery&version=3")
		}
		request, _ := http.NewRequest(method, server.URL+"/shell/profile", body)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		request.Header.Set("Sec-Fetch-Mode", "navigate")
		request.Header.Set("Sec-Fetch-Dest", "document")
		request.Header.Set("Sec-Fetch-User", "?1")
		if method == http.MethodPost {
			request.Header.Set("Origin", server.URL)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response, err = client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		bodyText := responseBody(t, response)
		if response.StatusCode != http.StatusOK || (method != http.MethodHead && bodyText != "synthetic application") {
			t.Fatalf("authenticated %s failed: %d", method, response.StatusCode)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("got %d delegated requests", calls.Load())
	}
	response, err = client.Get(launch)
	if err != nil {
		t.Fatal("launch replay transport failed")
	}
	if response.StatusCode != http.StatusForbidden || strings.Contains(responseBody(t, response), token) || len(response.Cookies()) != 0 {
		t.Fatal("replayed launch credential accepted or exposed")
	}
}

func TestBootstrapRejectsUntrustedRequests(t *testing.T) {
	var calls atomic.Int32
	b, err := newBootstrap("127.0.0.1:4567", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*http.Request)
	}{
		{"missing cookie", func(r *http.Request) { r.Header.Del("Cookie") }},
		{"static without cookie", func(r *http.Request) { r.URL.Path = "/static/app.css"; r.Header.Del("Cookie") }},
		{"wrong cookie", func(r *http.Request) { r.Header.Set("Cookie", b.cookieName+"=wrong") }},
		{"duplicate cookie", func(r *http.Request) { r.Header.Add("Cookie", b.cookieName+"="+b.session) }},
		{"duplicate cookie in line", func(r *http.Request) { r.Header.Set("Cookie", b.cookieName+"=wrong; "+b.cookieName+"="+b.session) }},
		{"malformed cookie", func(r *http.Request) { r.Header.Add("Cookie", b.cookieName+"=\"unterminated") }},
		{"localhost", func(r *http.Request) { r.Host = "localhost:4567" }},
		{"wrong port", func(r *http.Request) { r.Host = "127.0.0.1:4568" }},
		{"DNS rebinding", func(r *http.Request) { r.Host = "127.0.0.1.attacker.test:4567" }},
		{"absolute target", func(r *http.Request) { r.URL.Scheme = "http"; r.URL.Host = b.host }},
		{"remote peer", func(r *http.Request) { r.RemoteAddr = "192.0.2.3:1234" }},
		{"IPv6 peer", func(r *http.Request) { r.RemoteAddr = "[::1]:1234" }},
		{"IPv4 mapped peer", func(r *http.Request) { r.RemoteAddr = "[::ffff:127.0.0.1]:1234" }},
		{"missing peer port", func(r *http.Request) { r.RemoteAddr = "127.0.0.1" }},
		{"forwarded peer cannot authorize", func(r *http.Request) {
			r.RemoteAddr = "192.0.2.3:1234"
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("Forwarded", "for=127.0.0.1;host="+b.host)
		}},
		{"forwarded host cannot authorize", func(r *http.Request) { r.Host = "attacker.test"; r.Header.Set("X-Forwarded-Host", b.host) }},
		{"bad origin", func(r *http.Request) { r.Header.Set("Origin", "https://attacker.test") }},
		{"null origin", func(r *http.Request) { r.Header.Set("Origin", "null") }},
		{"origin slash", func(r *http.Request) { r.Header.Set("Origin", b.origin+"/") }},
		{"empty origin", func(r *http.Request) { r.Header.Set("Origin", "") }},
		{"duplicate origin", func(r *http.Request) { r.Header.Add("Origin", b.origin); r.Header.Add("Origin", b.origin) }},
		{"origin list", func(r *http.Request) { r.Header.Set("Origin", b.origin+", "+b.origin) }},
		{"POST without origin", func(r *http.Request) { r.Method = http.MethodPost }},
		{"same-site fetch", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-site") }},
		{"cross-site fetch", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{"unknown fetch site", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "trusted") }},
		{"empty fetch site", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "") }},
		{"duplicate fetch site", func(r *http.Request) {
			r.Header.Add("Sec-Fetch-Site", "same-origin")
			r.Header.Add("Sec-Fetch-Site", "same-origin")
		}},
		{"fetch list", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin, none") }},
		{"fetch whitespace", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin none") }},
		{"bad fetch mode", func(r *http.Request) { r.Header.Set("Sec-Fetch-Mode", "garbage") }},
		{"bad fetch destination", func(r *http.Request) { r.Header.Set("Sec-Fetch-Dest", "garbage") }},
		{"bad fetch user", func(r *http.Request) { r.Header.Set("Sec-Fetch-User", "?0") }},
		{"inconsistent fetch user", func(r *http.Request) { r.Header.Set("Sec-Fetch-User", "?1"); r.Header.Set("Sec-Fetch-Mode", "cors") }},
		{"PUT", func(r *http.Request) { r.Method = http.MethodPut }},
		{"OPTIONS", func(r *http.Request) { r.Method = http.MethodOptions }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/shell/profile", nil)
			request.Host, request.RemoteAddr = b.host, "127.0.0.1:12345"
			request.AddCookie(&http.Cookie{Name: b.cookieName, Value: b.session})
			test.edit(request)
			response := httptest.NewRecorder()
			b.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden || calls.Load() != 0 || response.Body.String() != "Forbidden\n" || len(response.Result().Cookies()) != 0 || strings.Contains(fmt.Sprint(response.Header()), b.session) {
				t.Fatalf("untrusted request escaped guard: status %d, handler calls %d", response.Code, calls.Load())
			}
		})
	}
}

func TestBootstrapLaunchValidationDoesNotConsume(t *testing.T) {
	var calls atomic.Int32
	b, server, client := bootstrapServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	launch := launchForTest(t, b)
	parsed, _ := url.Parse(launch)
	token := parsed.Query().Get("token")
	tests := []struct {
		name   string
		method string
		target string
		body   io.Reader
		edit   func(*http.Request)
	}{
		{"HEAD", "HEAD", launch, nil, nil},
		{"POST", "POST", launch, nil, nil},
		{"query extra", "GET", launch + "&extra=1", nil, nil},
		{"duplicate token", "GET", launch + "&token=" + token, nil, nil},
		{"encoded key", "GET", strings.Replace(launch, "?token=", "?%74oken=", 1), nil, nil},
		{"encoded token", "GET", b.origin + launchPath + "?token=%" + fmt.Sprintf("%02X", token[0]) + token[1:], nil, nil},
		{"encoded path", "GET", b.origin + "/auth/%6caunch?token=" + token, nil, nil},
		{"missing token", "GET", b.origin + launchPath, nil, nil},
		{"wrong token", "GET", b.origin + launchPath + "?token=" + strings.Repeat("A", 43), nil, nil},
		{"noncanonical token", "GET", b.origin + launchPath + "?token=" + strings.Repeat("A", 42) + "B", nil, nil},
		{"body", "GET", launch, strings.NewReader("x"), nil},
		{"cross-origin", "GET", launch, nil, func(r *http.Request) { r.Header.Set("Origin", "https://attacker.test") }},
		{"cross-site", "GET", launch, nil, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{"duplicate cookie", "GET", launch, nil, func(r *http.Request) { r.Header.Set("Cookie", b.cookieName+"=a; "+b.cookieName+"=b") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(test.method, test.target, test.body)
			if err != nil {
				t.Fatal("could not construct invalid launch request")
			}
			if test.edit != nil {
				test.edit(request)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal("invalid launch transport failed")
			}
			body := responseBody(t, response)
			if response.StatusCode != http.StatusForbidden || len(response.Cookies()) != 0 || calls.Load() != 0 || strings.Contains(body+fmt.Sprint(response.Header), token) {
				t.Fatalf("invalid launch escaped guard: %d", response.StatusCode)
			}
		})
	}
	response, err := client.Get(launch)
	if err != nil {
		t.Fatal("launch validation transport failed")
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatal("invalid requests consumed valid launch token")
	}
	// A different process on the same port cannot accept this process's cookie.
	other, err := newBootstrap(b.host, b.next)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/shell/profile", nil)
	request.Host, request.RemoteAddr = b.host, "127.0.0.1:1234"
	serverURL, _ := url.Parse(server.URL)
	for _, cookie := range client.Jar.Cookies(serverURL) {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	other.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatal("process restart did not invalidate session")
	}
}

func TestBootstrapLaunchBoundExpiryAndConcurrentReplay(t *testing.T) {
	b, _, client := bootstrapServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("launch delegated") }))
	clock := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return clock }
	first := launchForTest(t, b)
	for i := 1; i < launchLimit; i++ {
		launchForTest(t, b)
	}
	if _, err := b.LaunchURL(); err == nil || len(b.launches) != launchLimit {
		t.Fatal("outstanding launch credentials are not bounded")
	}
	clock = clock.Add(launchTTL)
	response, err := client.Get(first)
	if err != nil {
		t.Fatal("expired launch transport failed")
	}
	responseBody(t, response)
	if response.StatusCode != http.StatusForbidden {
		t.Fatal("token accepted at its expiration boundary")
	}
	fresh := launchForTest(t, b)
	if len(b.launches) != 1 {
		t.Fatal("expired tokens were not pruned")
	}
	var winners atomic.Int32
	var rejected atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			response, err := client.Get(fresh)
			if err != nil {
				t.Error("concurrent launch transport failed")
				return
			}
			_, _ = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			switch response.StatusCode {
			case http.StatusSeeOther:
				winners.Add(1)
			case http.StatusForbidden:
				rejected.Add(1)
			default:
				t.Errorf("unexpected concurrent response: %d", response.StatusCode)
			}
		}()
	}
	close(start)
	wg.Wait()
	if winners.Load() != 1 || rejected.Load() != 23 {
		t.Fatalf("launch exchange winners %d, rejections %d", winners.Load(), rejected.Load())
	}
}

func TestBootstrapSeparateLaunchesShareProcessSession(t *testing.T) {
	b, server, firstClient := bootstrapServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	clock := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return clock }
	first, second := launchForTest(t, b), launchForTest(t, b)
	if first == second {
		t.Fatal("separate launches reused a token")
	}
	clock = clock.Add(launchTTL - time.Nanosecond)
	jar, _ := cookiejar.New(nil)
	secondClient := &http.Client{Jar: jar, Timeout: 3 * time.Second, CheckRedirect: firstClient.CheckRedirect}
	t.Cleanup(secondClient.CloseIdleConnections)
	var session string
	for i, client := range []*http.Client{firstClient, secondClient} {
		response, err := client.Get([]string{first, second}[i])
		if err != nil {
			t.Fatal("independent launch transport failed")
		}
		responseBody(t, response)
		if response.StatusCode != http.StatusSeeOther || len(response.Cookies()) != 1 {
			t.Fatal("valid independent launch rejected before expiration")
		}
		if i == 0 {
			session = response.Cookies()[0].Value
		} else if response.Cookies()[0].Value != session {
			t.Fatal("windows did not share the process session")
		}
		response, err = client.Get(server.URL + "/shell/profile")
		if err != nil {
			t.Fatal(err)
		}
		responseBody(t, response)
		if response.StatusCode != http.StatusNoContent {
			t.Fatal("independent window session did not authorize")
		}
	}
}

func TestBootstrapHealthAndWriterCapabilities(t *testing.T) {
	b, server, client := bootstrapServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Errorf("write deadline lost: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: synthetic\n\n"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("flush lost: %v", err)
		}
	}))
	for _, path := range []string{"/healthz", "/healthz?", "/healthz?x=1", "/%68ealthz", "/static/app.css", "/"} {
		response, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body := responseBody(t, response)
		if path == "/healthz" {
			if response.StatusCode != http.StatusOK || body != "ok" {
				t.Fatal("health is not minimal and available")
			}
		} else if response.StatusCode != http.StatusForbidden {
			t.Fatalf("unauthenticated content exposed for %s", path)
		}
		if len(response.Cookies()) != 0 || strings.Contains(body, b.session) {
			t.Fatal("unauthenticated response exposed a session")
		}
	}
	for _, tc := range []struct {
		name, method string
		body         io.Reader
	}{
		{"head", http.MethodHead, nil},
		{"post", http.MethodPost, nil},
		{"get body", http.MethodGet, strings.NewReader("x")},
		{"get chunked body", http.MethodGet, io.NopCloser(strings.NewReader("x"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(tc.method, server.URL+"/healthz", tc.body)
			if err != nil {
				t.Fatal("could not construct health refusal")
			}
			request.Header.Set("Origin", server.URL)
			response, err := client.Do(request)
			if err != nil {
				t.Fatal("health refusal transport failed")
			}
			body := responseBody(t, response)
			if response.StatusCode != http.StatusForbidden || len(response.Cookies()) != 0 || strings.Contains(body, b.session) {
				t.Fatal("health method/body exception was broadened")
			}
		})
	}
	response, err := client.Get(launchForTest(t, b))
	if err != nil {
		t.Fatal("stream launch transport failed")
	}
	responseBody(t, response)
	response, err = client.Get(server.URL + "/shell/events")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || responseBody(t, response) != "data: synthetic\n\n" {
		t.Fatal("authenticated stream failed")
	}
}
