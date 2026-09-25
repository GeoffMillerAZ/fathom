package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
	"golang.org/x/net/html"
)

func TestHostGalleryPOSTAndSSE(t *testing.T) {
	h, err := startHost(t.Context(), gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Jar: jar, Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	launch, err := h.auth.LaunchURL()
	if err != nil {
		t.Fatal("launch issuance failed")
	}
	response, err := client.Get(launch)
	if err != nil {
		t.Fatal("launch transport failed") // errors may contain the bearer URL
	}
	_ = response.Body.Close()
	if response.StatusCode != 303 || response.Header.Get("Location") != "/shell/profile" {
		t.Fatal("launch did not redirect to the shared profile workflow")
	}
	page := func(path string, values url.Values, want int) string {
		t.Helper()
		method, body := http.MethodGet, ""
		if values != nil {
			method, body = http.MethodPost, values.Encode()
		}
		r, _ := http.NewRequestWithContext(t.Context(), method, h.baseURL+path, strings.NewReader(body))
		if values != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", h.baseURL)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal("workflow transport failed")
		}
		defer res.Body.Close()
		data, err := io.ReadAll(io.LimitReader(res.Body, 256<<10))
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s %s: status %d, want %d", method, path, res.StatusCode, want)
		}
		return string(data)
	}
	initial := page("/shell/profile", nil, 200)
	save, stale := profileFields(t, initial), profileFields(t, initial)
	if save.Get("display_name") != "Alex Morgan" {
		t.Fatal("expected the existing synthetic Gallery session")
	}
	openStream := func() (*http.Response, *bufio.Reader) {
		t.Helper()
		r, _ := http.NewRequestWithContext(t.Context(), "GET", h.baseURL+"/shell/events", nil)
		r.Header.Set("Accept", "text/event-stream")
		res, err := client.Do(r)
		if err != nil || res.StatusCode != 200 {
			t.Fatal("guarded Gallery stream did not open")
		}
		t.Cleanup(func() { _ = res.Body.Close() })
		return res, bufio.NewReader(res.Body)
	}
	first, a := openStream()
	_, b := openStream()
	for _, reader := range []*bufio.Reader{a, b} {
		frame := eventFrame(t, reader)
		if !strings.Contains(frame, `"display_name":"Alex Morgan"`) || !strings.Contains(frame, "event: datastar-patch-signals") || strings.Contains(frame, save.Get("csrf")) {
			t.Fatal("initial frame was wrong or disclosed the CSRF secret")
		}
	}
	bad := profileFields(t, initial)
	bad.Set("csrf", "not-the-session-token")
	bad.Set("display_name", "Must not be saved")
	page("/shell/action", bad, 403)
	if profileFields(t, page("/shell/profile", nil, 200)).Get("display_name") != "Alex Morgan" {
		t.Fatal("bad CSRF command changed the shared workflow")
	}
	save.Set("display_name", "Synthetic Native Host Avery")
	page("/shell/action", save, 303)
	for _, reader := range []*bufio.Reader{a, b} {
		frame := eventFrame(t, reader)
		if !strings.Contains(frame, `"version":"2"`) || !strings.Contains(frame, `"display_name":"Synthetic Native Host Avery"`) {
			t.Fatal("ordinary HTML save did not reach both real HTTP streams")
		}
	}
	page("/shell/action", stale, 409)
	if profileFields(t, page("/shell/profile", nil, 200)).Get("display_name") != "Synthetic Native Host Avery" {
		t.Fatal("stale form overwrote the saved state")
	}
	_ = first.Body.Close()
	_, reconnected := openStream()
	if !strings.Contains(eventFrame(t, reconnected), `"version":"2"`) {
		t.Fatal("new stream did not resnapshot the current saved version")
	}
	if err := h.Close(); err != nil {
		t.Fatal("host shutdown failed")
	}
	if _, err := b.ReadByte(); err != io.EOF {
		t.Fatalf("active stream did not end cleanly: %v", err)
	}
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(h.baseURL, "http://"), time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("closed host still accepts connections")
	}
}

func profileFields(t *testing.T, body string) url.Values {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(body))
	var fields url.Values
	for {
		switch z.Next() {
		case html.ErrorToken:
			t.Fatal("missing complete save-profile form")
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if tok.Data == "form" {
				fields = url.Values{}
			} else if tok.Data == "input" && fields != nil {
				attrs := map[string]string{}
				for _, attr := range tok.Attr {
					attrs[attr.Key] = attr.Val
				}
				if attrs["name"] != "" {
					fields.Set(attrs["name"], attrs["value"])
				}
			}
		case html.EndTagToken:
			if z.Token().Data == "form" && fields.Get("action") == "save-profile" {
				if fields.Get("csrf") == "" || fields.Get("version") == "" {
					t.Fatal("profile form lost its command authority fields")
				}
				return fields
			}
		}
	}
}

func eventFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var frame strings.Builder
	for frame.Len() < 16<<10 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal("stream ended before a complete bounded frame")
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
	t.Fatal("stream frame exceeded the test bound")
	return ""
}

func TestHostCancelledStartCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h, err := startHost(ctx, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestServerBuildDoesNotImportNativeRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-tags", "server", ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("could not resolve the server build's dependency closure")
	}
	for _, pkg := range strings.Fields(string(output)) {
		if strings.HasPrefix(pkg, "github.com/wailsapp/") || pkg == "runtime/cgo" {
			t.Fatalf("server build inherited a native runtime: %s", pkg)
		}
	}
}
