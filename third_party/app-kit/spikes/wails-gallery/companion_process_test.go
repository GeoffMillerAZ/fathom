//go:build darwin && !workflowprobe && !capabilityprobe

package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/hostwire"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "-recovery-parent-fixture" {
		os.Exit(runRecoveryParentFixture())
	}
	if len(os.Args) > 1 && os.Args[1] == "-native-child" {
		name := filepath.Base(os.Args[0])
		if strings.HasPrefix(name, "helper-") {
			os.Exit(runCompanionHelper(strings.TrimPrefix(name, "helper-")))
		}
	}
	os.Exit(m.Run())
}

func runRecoveryParentFixture() int {
	path, digest := os.Getenv("APPKIT_RECOVERY_NATIVE"), os.Getenv("APPKIT_RECOVERY_SHA256")
	if path == "" || digest == "" {
		return 81
	}
	prepared, err := prepareCompanion(path, digest)
	if err != nil {
		return 82
	}
	defer prepared.close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		return 83
	}
	defer h.Close()
	metadata := struct {
		Origin string `json:"origin"`
		Native string `json:"native"`
	}{Origin: h.baseURL, Native: prepared.path}
	if err := json.NewEncoder(os.Stdout).Encode(metadata); err != nil {
		return 84
	}
	err = runCompanion(ctx, h, prepared.path, os.Stdout, func(*host) error {
		return errors.New("synthetic recovery opener unavailable")
	}, 15*time.Second)
	if err != nil {
		return 85
	}
	return 0
}

func runCompanionHelper(mode string) int {
	var raw net.Conn
	if mode == "malformed" || mode == "partialframe" {
		fd, err := syscall.Dup(3)
		if err != nil {
			return 91
		}
		file := os.NewFile(uintptr(fd), "raw-helper-control")
		raw, err = net.FileConn(file)
		_ = file.Close()
		if err != nil {
			return 92
		}
		defer raw.Close()
	}
	wire, err := inheritedCompanionWire()
	if err != nil {
		return 93
	}
	defer wire.Close()
	init, err := wire.Read(2 * time.Second)
	if err != nil || init.Kind != "init" {
		return 94
	}
	write := func(kind string) bool {
		return wire.Write(hostwire.Message{Version: 1, Kind: kind}) == nil
	}
	switch mode {
	case "earlyexit":
		return 0
	case "nonzeroafterready":
		if !write("ready") {
			return 95
		}
		return 17
	case "zero-withoutclosed":
		if !write("ready") {
			return 95
		}
		return 0
	case "startupstall":
		time.Sleep(30 * time.Second)
		return 0
	case "malformed":
		if !write("ready") {
			return 95
		}
		payload := []byte(`{"version":1,"kind":"unknown"}`)
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		_, _ = raw.Write(append(header[:], payload...))
		return 0
	case "partialframe":
		if !write("ready") {
			return 95
		}
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], 100)
		_, _ = raw.Write(append(header[:], '{'))
		time.Sleep(30 * time.Second)
		return 0
	case "wrongorder":
		if !write("browser") {
			return 95
		}
		return 0
	case "cleanclosed":
		if !write("ready") || !write("closed") {
			return 95
		}
		return 0
	case "cleanclosed-before-ready":
		if !write("closed") {
			return 95
		}
		return 0
	case "closed-before-ready-nonzero":
		if !write("closed") {
			return 95
		}
		return 18
	case "handoff-closed":
		if !write("ready") || !write("browser") {
			return 95
		}
		response, err := wire.Read(2 * time.Second)
		if err != nil || response.Kind != "browser-ok" || !write("closed") {
			return 96
		}
		return 0
	case "lateclosed-nonzero":
		if !write("ready") {
			return 95
		}
		time.Sleep(25 * time.Millisecond)
		if !write("closed") {
			return 95
		}
		return 19
	case "controlflood":
		if !write("ready") {
			return 95
		}
		for i := 0; i < 40; i++ {
			if !write("browser") {
				return 0
			}
			if _, err := wire.Read(2 * time.Second); err != nil {
				return 0
			}
		}
		time.Sleep(30 * time.Second)
		return 0
	case "ignoreSIGTERM":
		signal.Ignore(syscall.SIGTERM)
		if !write("ready") {
			return 95
		}
		time.Sleep(30 * time.Second)
		return 0
	default:
		return 97
	}
}

func TestCompanionFailuresFallBackExactlyOnce(t *testing.T) {
	tests := []struct {
		mode         string
		ready        bool
		readyTimeout time.Duration
	}{
		{mode: "earlyexit", readyTimeout: time.Second},
		{mode: "nonzeroafterready", ready: true, readyTimeout: time.Second},
		{mode: "zero-withoutclosed", ready: true, readyTimeout: time.Second},
		{mode: "startupstall", readyTimeout: 35 * time.Millisecond},
		{mode: "malformed", ready: true, readyTimeout: time.Second},
		{mode: "wrongorder", readyTimeout: time.Second},
		{mode: "closed-before-ready-nonzero", readyTimeout: time.Second},
		{mode: "lateclosed-nonzero", ready: true, readyTimeout: time.Second},
		{mode: "partialframe", ready: true, readyTimeout: time.Second},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			h, err := startHost(ctx, gallery.New())
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			var opens atomic.Int32
			var diagnostics strings.Builder
			started := time.Now()
			err = runCompanion(ctx, h, companionHelper(t, test.mode), &diagnostics, func(*host) error {
				opens.Add(1)
				cancel()
				return nil
			}, test.readyTimeout)
			if err != nil {
				t.Fatalf("bounded fallback failed: %v", err)
			}
			if opens.Load() != 1 {
				t.Fatalf("fallback opener called %d times", opens.Load())
			}
			output := diagnostics.String()
			assertSanitizedCompanionText(t, output)
			acknowledged := strings.Contains(output, "Native companion navigation acknowledged")
			if acknowledged != test.ready {
				t.Fatalf("ready evidence = %v, want %v; diagnostics %q", acknowledged, test.ready, output)
			}
			elapsed := time.Since(started)
			if (test.mode == "earlyexit" || test.mode == "wrongorder") && elapsed >= test.readyTimeout {
				t.Fatalf("%s reached readiness timeout instead of its fault branch: %v", test.mode, elapsed)
			}
			if test.mode == "partialframe" && elapsed < 2500*time.Millisecond {
				t.Fatalf("partial frame did not exercise the frame-completion bound: %v", elapsed)
			}
		})
	}
}

func TestCompanionCleanCloseEndsHTTPHost(t *testing.T) {
	for _, mode := range []string{"cleanclosed", "cleanclosed-before-ready"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			h, err := startHost(ctx, gallery.New())
			if err != nil {
				t.Fatal(err)
			}
			var opens atomic.Int32
			if err := runCompanion(ctx, h, companionHelper(t, mode), io.Discard, func(*host) error {
				opens.Add(1)
				return nil
			}, time.Second); err != nil {
				t.Fatal(err)
			}
			if opens.Load() != 0 {
				t.Fatal("clean native close invoked browser fallback")
			}
			select {
			case <-h.done:
			default:
				t.Fatal("clean native close left HTTP host running")
			}
			assertHostRefusesConnections(t, h)
		})
	}
}

func TestCompanionBrowserHandoffKeepsHTTPUntilCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	opened := make(chan struct{}, 1)
	result := make(chan error, 1)
	helper := companionHelper(t, "handoff-closed")
	go func() {
		result <- runCompanion(ctx, h, helper, io.Discard, func(*host) error {
			opened <- struct{}{}
			return nil
		}, time.Second)
	}()
	waitForSignal(t, opened, "browser handoff")
	select {
	case err := <-result:
		t.Fatalf("handoff ended HTTP host before cancellation: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	response, err := http.Get(h.baseURL + "/healthz")
	if err != nil {
		t.Fatal("HTTP host did not survive native handoff")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", response.StatusCode)
	}
	cancel()
	if err := waitResult(t, result); err != nil {
		t.Fatal(err)
	}
}

func TestCompanionOpenerFailureIsSanitized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	const secret = "super-secret-launch-token"
	var opens atomic.Int32
	opened := make(chan struct{}, 1)
	result := make(chan error, 1)
	helper := companionHelper(t, "earlyexit")
	go func() {
		result <- runCompanion(ctx, h, helper, io.Discard, func(*host) error {
			opens.Add(1)
			opened <- struct{}{}
			return &url.Error{Op: "open", URL: h.baseURL + "/auth/launch?token=" + secret, Err: syscall.EPERM}
		}, time.Second)
	}()
	waitForSignal(t, opened, "failed browser opener")
	if opens.Load() != 1 {
		t.Fatal("opener failure was not returned exactly once")
	}
	select {
	case err := <-result:
		t.Fatalf("opener failure ended the existing HTTP app early: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	response, err := http.Get(h.baseURL + "/healthz")
	if err != nil {
		t.Fatal("existing HTTP app did not survive opener failure")
	}
	_ = response.Body.Close()
	cancel()
	err = waitResult(t, result)
	if err == nil {
		t.Fatal("opener failure became successful after cancellation")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), h.baseURL) || strings.Contains(err.Error(), "token=") {
		t.Fatalf("opener error exposed launch authority: %q", err)
	}
}

func TestCompanionCancellationReapsSIGTERMIgnoringChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	diagnostics := newCompanionDiagnostics()
	result := make(chan error, 1)
	var opens atomic.Int32
	helper := companionHelper(t, "ignoreSIGTERM")
	started := time.Now()
	go func() {
		result <- runCompanion(ctx, h, helper, diagnostics, func(*host) error {
			opens.Add(1)
			return nil
		}, time.Second)
	}()
	waitForSignal(t, diagnostics.ready, "SIGTERM-ignoring helper readiness")
	cancel()
	err = waitResult(t, result)
	if err != nil {
		t.Fatal(err)
	}
	if opens.Load() != 0 {
		t.Fatal("ordinary cancellation invoked browser fallback")
	}
	if elapsed := time.Since(started); elapsed < 1900*time.Millisecond || elapsed > 4*time.Second {
		t.Fatalf("SIGTERM-ignoring helper exceeded reap bound: %v", elapsed)
	}
	if !strings.Contains(diagnostics.String(), "Native companion navigation acknowledged") {
		t.Fatal("test did not observe readiness after SIGTERM ignore was armed")
	}
}

func TestCompanionControlFloodTriggersBoundedRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	diagnostics := newCompanionDiagnostics()
	result := make(chan error, 1)
	var opens atomic.Int32
	helper := companionHelper(t, "controlflood")
	started := time.Now()
	go func() {
		result <- runCompanion(ctx, h, helper, diagnostics, func(*host) error {
			opens.Add(1)
			return nil
		}, time.Second)
	}()
	waitForSignal(t, diagnostics.ready, "control-flood readiness")
	waitForSignal(t, diagnostics.handoff, "initial browser handoff")
	waitForSignal(t, diagnostics.recovery, "control-flood recovery")
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("control-flood recovery exceeded bound: %v", elapsed)
	}
	if opens.Load() != 1 {
		t.Fatalf("control-flood browser opener called %d times", opens.Load())
	}
	cancel()
	if err := waitResult(t, result); err != nil {
		t.Fatal(err)
	}
	text := diagnostics.String()
	if !strings.Contains(text, "Native companion navigation acknowledged") ||
		!strings.Contains(text, "Browser continuation selected") ||
		!strings.Contains(text, "Native companion unavailable") {
		t.Fatalf("control-flood evidence incomplete: %q", text)
	}
	assertSanitizedCompanionText(t, text)
}

func TestGuardedGallerySessionAndStreamSurviveCompanionDeath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Jar: jar, Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	launch, err := h.auth.LaunchURL()
	if err != nil {
		t.Fatal("launch issuance failed")
	}
	response, err := client.Get(launch)
	if err != nil {
		t.Fatal("launch transport failed")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatal("guarded browser session was not established")
	}
	page := func(values url.Values, want int) string {
		t.Helper()
		method, body := http.MethodGet, ""
		path := "/shell/profile"
		if values != nil {
			method, body, path = http.MethodPost, values.Encode(), "/shell/action"
		}
		request, _ := http.NewRequestWithContext(ctx, method, h.baseURL+path, strings.NewReader(body))
		if values != nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Origin", h.baseURL)
		}
		res, err := client.Do(request)
		if err != nil {
			t.Fatal("guarded workflow transport failed")
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(res.Body, 256<<10))
		if res.StatusCode != want {
			t.Fatalf("workflow status %d, want %d", res.StatusCode, want)
		}
		return string(data)
	}
	initial := page(nil, http.StatusOK)
	save, stale := profileFields(t, initial), profileFields(t, initial)
	streamRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/shell/events", nil)
	streamRequest.Header.Set("Accept", "text/event-stream")
	stream, err := client.Do(streamRequest)
	if err != nil || stream.StatusCode != http.StatusOK {
		t.Fatal("guarded event stream did not open")
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	_ = eventFrame(t, reader)

	fellBack := make(chan struct{}, 1)
	result := make(chan error, 1)
	helper := companionHelper(t, "nonzeroafterready")
	go func() {
		result <- runCompanion(ctx, h, helper, io.Discard, func(*host) error {
			fellBack <- struct{}{}
			return nil
		}, time.Second)
	}()
	waitForSignal(t, fellBack, "companion death fallback")

	save.Set("display_name", "Recovery Casey")
	page(save, http.StatusSeeOther)
	frame := eventFrame(t, reader)
	if !strings.Contains(frame, `"display_name":"Recovery Casey"`) || strings.Contains(frame, save.Get("csrf")) {
		t.Fatal("surviving SSE missed saved state or exposed CSRF authority")
	}
	stale.Set("display_name", "Stale overwrite")
	page(stale, http.StatusConflict)
	if got := profileFields(t, page(nil, http.StatusOK)).Get("display_name"); got != "Recovery Casey" {
		t.Fatalf("state after helper death = %q", got)
	}
	cancel()
	if err := waitResult(t, result); err != nil {
		t.Fatal(err)
	}
}

func companionHelper(t *testing.T, mode string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "helper-"+mode)
	if err := os.Link(executable, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForSignal(t *testing.T, signal <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("companion lifecycle exceeded test bound")
		return nil
	}
}

func assertSanitizedCompanionText(t *testing.T, text string) {
	t.Helper()
	if strings.Contains(text, "token=") || strings.Contains(text, "/auth/launch") {
		t.Fatalf("companion diagnostics exposed launch authority: %q", text)
	}
}

func assertHostRefusesConnections(t *testing.T, h *host) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(h.baseURL, "http://"), time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("closed HTTP host still accepted connections")
	}
}

type companionDiagnostics struct {
	mu                       sync.Mutex
	text                     strings.Builder
	ready, handoff, recovery chan struct{}
	readySent, handoffSent   bool
	recoverySent             bool
}

func newCompanionDiagnostics() *companionDiagnostics {
	return &companionDiagnostics{
		ready: make(chan struct{}, 1), handoff: make(chan struct{}, 1), recovery: make(chan struct{}, 1),
	}
}

func (d *companionDiagnostics) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.text.Write(p)
	text := d.text.String()
	if !d.readySent && strings.Contains(text, "Native companion navigation acknowledged") {
		d.readySent = true
		d.ready <- struct{}{}
	}
	if !d.handoffSent && strings.Contains(text, "Browser continuation selected") {
		d.handoffSent = true
		d.handoff <- struct{}{}
	}
	if !d.recoverySent && strings.Contains(text, "Native companion unavailable") {
		d.recoverySent = true
		d.recovery <- struct{}{}
	}
	return len(p), nil
}

func (d *companionDiagnostics) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.text.String()
}
