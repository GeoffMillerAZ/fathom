//go:build darwin && server && recoveryprobe && !workflowprobe

package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

func TestActualNativeStartupRefusal(t *testing.T) {
	path, digest := os.Getenv("APPKIT_RECOVERY_REFUSAL_NATIVE"), os.Getenv("APPKIT_RECOVERY_REFUSAL_SHA256")
	prepared, err := prepareCompanion(path, digest)
	if err != nil {
		t.Fatal("explicit uncontained native binary identity is required")
	}
	defer prepared.close()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	preflight := exec.CommandContext(ctx, prepared.path, "-native-child")
	preflight.Env = []string{}
	output, err := preflight.CombinedOutput()
	var exit *exec.ExitError
	expected := "gallery-host: native experiment requires the prepared macOS HTTP-only patch and appkit_contained build tag; use -browser for independent HTTP delivery\n"
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || string(output) != expected {
		t.Fatal("selected native candidate did not produce the exact uncontained-profile refusal")
	}
	h, err := startHost(ctx, gallery.New())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	log := &nativeRecoveryLog{ready: make(chan struct{})}
	type recoveryPage struct {
		body string
		err  error
	}
	opened := make(chan recoveryPage, 2)
	result := make(chan error, 1)
	var opens atomic.Int32
	go func() {
		result <- runCompanion(ctx, h, prepared.path, log, func(h *host) error {
			opens.Add(1)
			launch, err := h.auth.LaunchURL()
			var body []byte
			if err == nil {
				jar, _ := cookiejar.New(nil)
				transport := &http.Transport{Proxy: nil}
				defer transport.CloseIdleConnections()
				client := &http.Client{Jar: jar, Transport: transport, Timeout: 5 * time.Second}
				response, fetchErr := client.Get(launch)
				if fetchErr != nil {
					err = errors.New("browser exchange unavailable")
				} else {
					body, err = io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
					response.Body.Close()
					if response.StatusCode != 200 || len(body) > 256<<10 {
						err = errors.New("browser profile unavailable")
					}
				}
			}
			opened <- recoveryPage{string(body), err}
			return err
		}, 15*time.Second)
	}()
	defer func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Error("refusal recovery parent returned failure during shutdown")
			}
		case <-time.After(7 * time.Second):
			t.Error("refusal recovery parent did not stop")
		}
		if opens.Load() != 1 {
			t.Error("refusal recovery was not exactly once through shutdown")
		}
		select {
		case <-log.ready:
			t.Error("uncontained native candidate claimed readiness before shutdown")
		default:
		}
	}()
	select {
	case page := <-opened:
		if page.err != nil {
			t.Fatal("refusal did not recover to the guarded HTTP workflow")
		}
		if profileFields(t, page.body).Get("display_name") != "Alex Morgan" {
			t.Fatal("recovery profile did not render")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native profile refusal stalled instead of recovering")
	}
	select {
	case <-log.ready:
		t.Fatal("uncontained native candidate claimed readiness")
	default:
	}
	h.auth.mu.Lock()
	outstanding := len(h.auth.launches)
	h.auth.mu.Unlock()
	if outstanding != 1 {
		t.Fatal("uncontained candidate consumed its native launch or browser launch was not exchanged")
	}
	if strings.Contains(log.String(), "token=") {
		t.Fatal("refusal diagnostics exposed launch authority")
	}
	select {
	case <-opened:
		t.Fatal("refusal recovered more than once")
	default:
	}
	t.Logf("native_refusal_sha256=%s exact_profile_refusal=passed no_navigation_ack=passed native_launch_unused=passed guarded_http_recovery=passed", digest)
}
