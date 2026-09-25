package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGalleryConfiguration(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, tc := range []struct {
		name, env, root, wantError string
		args                       []string
	}{
		{name: "default permits absent optional store", root: "data/design-assets"},
		{name: "disabled", args: []string{"-design-assets="}},
		{name: "flag overrides environment", args: []string{"-design-assets="}, env: missing},
		{name: "explicit missing", args: []string{"-design-assets", missing}, wantError: "selected design-assets store is unavailable"},
		{name: "environment missing", env: missing, wantError: "selected design-assets store is unavailable"},
		{name: "IPv6 loopback", args: []string{"-addr", "[::1]:8765"}, root: "data/design-assets"},
		{name: "remote", args: []string{"-addr", "0.0.0.0:8765"}, wantError: "loopback IP"},
		{name: "DNS name", args: []string{"-addr", "example.com:8765"}, wantError: "loopback IP"},
		{name: "invalid address", args: []string{"-addr", "127.0.0.1"}, wantError: "bad -addr"},
		{name: "extra args", args: []string{"serve"}, wantError: "unexpected arguments"},
		{name: "unknown option", args: []string{"-anything"}, wantError: "flag provided but not defined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseConfig(tc.args, tc.env, io.Discard)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil || cfg.root != tc.root {
				t.Fatalf("config = %+v, error = %v", cfg, err)
			}
		})
	}
	if _, err := parseConfig([]string{"-help"}, "", io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help = %v", err)
	}
}

func galleryTestListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func galleryTestClient(t *testing.T) *http.Client {
	t.Helper()
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 3 * time.Second}
}

func galleryTestWait(t *testing.T, done <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal(message)
	}
}

func TestServeGalleryCancelsActiveStreamBeforeShutdown(t *testing.T) {
	listener := galleryTestListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	streamCanceled := make(chan struct{})
	handlerReturned := make(chan struct{})
	ctxErrors := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ordinary" {
			_, _ = io.WriteString(w, "ordinary synthetic response")
			return
		}
		defer close(handlerReturned)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: ready\ndata: {}\n\n")
		if err := http.NewResponseController(w).Flush(); err != nil {
			ctxErrors <- err
			return
		}
		<-r.Context().Done()
		ctxErrors <- r.Context().Err()
		close(streamCanceled)
	})
	served := make(chan error, 1)
	go func() { served <- serveGallery(ctx, listener, handler) }()
	client := galleryTestClient(t)
	base := "http://" + listener.Addr().String()
	ordinary, err := client.Get(base + "/ordinary")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(ordinary.Body)
	_ = ordinary.Body.Close()
	if err != nil || ordinary.StatusCode != http.StatusOK || string(body) != "ordinary synthetic response" {
		t.Fatalf("ordinary request: status=%d body=%q error=%v", ordinary.StatusCode, body, err)
	}
	stream, err := client.Get(base + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("stream response was not served")
	}
	reader := bufio.NewReader(stream.Body)
	for _, want := range []string{"event: ready\n", "data: {}\n", "\n"} {
		if line, err := reader.ReadString('\n'); err != nil || line != want {
			t.Fatalf("initial frame: got %q, want %q, error=%v", line, want, err)
		}
	}
	cancel()
	galleryTestWait(t, streamCanceled, "server shutdown did not cancel the active request context")
	galleryTestWait(t, handlerReturned, "stream handler did not return after cancellation")
	if err := <-ctxErrors; !errors.Is(err, context.Canceled) {
		t.Fatalf("stream context = %v, want cancellation", err)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("graceful shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveGallery did not return after stream cancellation")
	}
	if body, err := io.ReadAll(reader); err != nil || len(body) != 0 {
		t.Fatalf("gracefully closed stream: bytes=%d error=%v", len(body), err)
	}
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 250*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("server listener remained open after shutdown")
	}
}

func TestServeGalleryListenerFailureIsNotSuccess(t *testing.T) {
	listener := galleryTestListener(t)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := serveGallery(ctx, listener, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("closed listener unexpectedly served a request")
	}))
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("closed-listener failure = %v, want net.ErrClosed", err)
	}
}

func TestServeGalleryForcesCloseAfterShutdownDeadline(t *testing.T) {
	listener := galleryTestListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		galleryTestWait(t, handlerDone, "test's uncooperative handler did not finish")
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		_, _ = io.WriteString(w, "started\n")
		_ = http.NewResponseController(w).Flush()
		// Simulate work that ignores request cancellation. Close can terminate
		// its connection, not arbitrary application code; cleanup releases it.
		<-release
	})
	served := make(chan error, 1)
	go func() { served <- serveGalleryWithShutdownTimeout(ctx, listener, handler, 20*time.Millisecond) }()
	response, err := galleryTestClient(t).Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != "started\n" {
		t.Fatalf("initial response: %q, %v", line, err)
	}
	cancel()
	select {
	case err := <-served:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("uncooperative request shutdown = %v, want deadline failure", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deadline fallback did not return")
	}
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("forced-close connection remained readable")
	} else if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("client timed out instead of observing the server's forced close: %v", err)
	}
}
