//go:build darwin && server && recoveryprobe

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/hostwire"
)

type nativeRecoveryLog struct {
	mu    sync.Mutex
	text  bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (l *nativeRecoveryLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if bytes.Contains(b, []byte("navigation acknowledged")) {
		l.once.Do(func() { close(l.ready) })
	}
	return l.text.Write(b)
}

func (l *nativeRecoveryLog) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.text.String() }

// Explicit opt-in: missing native inputs fail, not skip. The runner itself is a
// cgo-free server build; every selected companion below is a real macOS Wails binary.
func TestActualNativeCompanionRecovery(t *testing.T) {
	path, digest := os.Getenv("APPKIT_RECOVERY_NATIVE"), os.Getenv("APPKIT_RECOVERY_SHA256")
	if path == "" || digest == "" {
		t.Fatal("actual native recovery requires explicit binary and SHA256")
	}
	prepared, err := prepareCompanion(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(prepared.close)
	t.Logf("native_input_sha256=%s server_runner_cgo_free=true", digest)
	t.Run("native_crash_preserves_existing_http_session_and_stream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		h, err := startHost(ctx, gallery.New())
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		jar, _ := cookiejar.New(nil)
		transport := &http.Transport{Proxy: nil}
		defer transport.CloseIdleConnections()
		client := &http.Client{Jar: jar, Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		launch, err := h.auth.LaunchURL()
		if err != nil {
			t.Fatal("browser launch unavailable")
		}
		response, err := client.Get(launch)
		if err != nil {
			t.Fatal("browser bootstrap failed")
		}
		response.Body.Close()
		if response.StatusCode != 303 {
			t.Fatal("browser bootstrap not exchanged")
		}
		page := func(path string, values url.Values, want int) string {
			t.Helper()
			method, body := "GET", ""
			if values != nil {
				method, body = "POST", values.Encode()
			}
			r, _ := http.NewRequestWithContext(ctx, method, h.baseURL+path, strings.NewReader(body))
			if values != nil {
				r.Header.Set("Origin", h.baseURL)
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			res, err := client.Do(r)
			if err != nil {
				t.Fatal("HTTP workflow transport failed")
			}
			defer res.Body.Close()
			b, err := io.ReadAll(io.LimitReader(res.Body, 256<<10))
			if err != nil || res.StatusCode != want {
				t.Fatalf("workflow status=%d want=%d", res.StatusCode, want)
			}
			return string(b)
		}
		initial := page("/shell/profile", nil, 200)
		save, stale := profileFields(t, initial), profileFields(t, initial)
		save.Set("display_name", "Saved before actual native crash")
		page("/shell/action", save, 303)
		r, _ := http.NewRequestWithContext(ctx, "GET", h.baseURL+"/shell/events", nil)
		r.Header.Set("Accept", "text/event-stream")
		stream, err := client.Do(r)
		if err != nil || stream.StatusCode != 200 {
			t.Fatal("stream unavailable")
		}
		defer stream.Body.Close()
		frames := bufio.NewReader(stream.Body)
		if !strings.Contains(eventFrame(t, frames), "Saved before actual native crash") {
			t.Fatal("baseline stream state wrong")
		}
		log := &nativeRecoveryLog{ready: make(chan struct{})}
		opened := make(chan struct{}, 2)
		result := make(chan error, 1)
		go func() {
			result <- runCompanion(ctx, h, prepared.path, log, func(h *host) error {
				// A fresh host launch is exchanged, but the preexisting client keeps
				// its own Gallery cookie. This is not a cross-browser session transfer.
				url, err := h.auth.LaunchURL()
				if err != nil {
					return fmt.Errorf("launch unavailable")
				}
				res, err := client.Get(url)
				if err != nil {
					return fmt.Errorf("launch transport failed")
				}
				res.Body.Close()
				if res.StatusCode != 303 {
					return fmt.Errorf("launch exchange failed")
				}
				opened <- struct{}{}
				return nil
			}, 15*time.Second)
		}()
		select {
		case <-log.ready:
		case err := <-result:
			t.Fatalf("native returned before navigation: %v", err)
		case <-ctx.Done():
			t.Fatal("native navigation deadline")
		}
		child := exactNativeChild(t, prepared.path)
		if err := child.Kill(); err != nil {
			t.Fatal("could not inject native process failure")
		}
		select {
		case <-opened:
		case err := <-result:
			t.Fatalf("parent exited during native failure: %v", err)
		case <-time.After(8 * time.Second):
			t.Fatal("native crash did not reach browser continuation")
		}
		if got := profileFields(t, page("/shell/profile", nil, 200)).Get("display_name"); got != "Saved before actual native crash" {
			t.Fatal("saved browser state lost")
		}
		page("/shell/action", stale, 409)
		current := profileFields(t, page("/shell/profile", nil, 200))
		current.Set("display_name", "Saved after actual native crash")
		bad := profileFields(t, page("/shell/profile", nil, 200))
		bad.Set("csrf", "invalid")
		page("/shell/action", bad, 403)
		page("/shell/action", current, 303)
		if !strings.Contains(eventFrame(t, frames), "Saved after actual native crash") {
			t.Fatal("existing stream did not survive native crash")
		}
		if err := child.Signal(syscall.Signal(0)); err == nil {
			t.Fatal("crashed native process not reaped")
		}
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(7 * time.Second):
			t.Fatal("parent shutdown exceeded bound")
		}
		select {
		case <-opened:
			t.Fatal("duplicate automatic browser launch")
		default:
		}
		if strings.Contains(log.String(), "token=") || strings.Contains(log.String(), launch) {
			t.Fatal("credential in parent diagnostics")
		}
		t.Logf("actual_native_crash=passed saved_state=passed csrf_stale_refusal=passed existing_sse=passed native_reaped=passed diagnostics_sha256=%x", sha256.Sum256([]byte(log.String())))
	})
	t.Run("parent_control_eof_reaps_real_native", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
		defer cancel()
		h, err := startHost(ctx, gallery.New())
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		p, err := startCompanion(prepared.path)
		if err != nil {
			t.Fatal(err)
		}
		defer p.stop()
		launch, err := h.auth.LaunchURL()
		if err != nil {
			t.Fatal("launch failed")
		}
		if p.wire.Write(hostwire.Message{Version: 1, Kind: "init", Origin: h.baseURL, Launch: launch}) != nil {
			t.Fatal("native bootstrap write failed")
		}
		m, err := p.wire.Read(15 * time.Second)
		if err != nil || m.Kind != "ready" {
			t.Fatal("native navigation missing")
		}
		_ = p.wire.Close()
		select {
		case <-p.exited:
			if p.err != nil {
				t.Fatal("native did not exit cleanly on parent EOF")
			}
		case <-time.After(6 * time.Second):
			t.Fatal("native orphan after parent EOF")
		}
		select {
		case <-h.done:
			t.Fatal("native EOF handling closed parent HTTP")
		default:
		}
		t.Log("actual_native_parent_eof=passed native_reaped=passed")
	})
	t.Run("normal_native_termination_ends_parent_without_browser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
		defer cancel()
		h, err := startHost(ctx, gallery.New())
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		log := &nativeRecoveryLog{ready: make(chan struct{})}
		opened := make(chan struct{}, 1)
		result := make(chan error, 1)
		go func() {
			result <- runCompanion(ctx, h, prepared.path, log, func(*host) error {
				opened <- struct{}{}
				return fmt.Errorf("unexpected browser opening")
			}, 15*time.Second)
		}()
		select {
		case <-log.ready:
		case <-time.After(17 * time.Second):
			t.Fatal("native navigation missing")
		}
		child := exactNativeChild(t, prepared.path)
		if err := child.Signal(syscall.SIGTERM); err != nil {
			t.Fatal("native signal unavailable")
		}
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("native clean termination did not end parent")
		}
		select {
		case <-opened:
			t.Fatal("normal native exit invoked recovery")
		default:
		}
		select {
		case <-h.done:
		default:
			t.Fatal("normal native exit left HTTP running")
		}
		if child.Signal(syscall.Signal(0)) == nil {
			t.Fatal("normal native process not reaped")
		}
		t.Log("actual_native_sigterm_postshutdown_zero_exit=passed parent_closed=passed no_browser_open=passed")
	})
}

func exactNativeChild(t *testing.T, path string) *os.Process {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ids, err := exec.CommandContext(ctx, "/usr/bin/pgrep", "-P", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		t.Fatal("could not identify this test process's native child")
	}
	var found *os.Process
	for _, id := range strings.Fields(string(ids)) {
		command, err := exec.CommandContext(ctx, "/bin/ps", "-ww", "-p", id, "-o", "command=").Output()
		if err != nil || strings.TrimSpace(string(command)) != path+" -native-child" {
			continue
		}
		if found != nil {
			t.Fatal("ambiguous native child")
		}
		pid, err := strconv.Atoi(id)
		if err != nil {
			t.Fatal("invalid child process identifier")
		}
		found, err = os.FindProcess(pid)
		if err != nil {
			t.Fatal("native child not found")
		}
	}
	if found == nil {
		t.Fatal("exact staged native child absent")
	}
	return found
}
