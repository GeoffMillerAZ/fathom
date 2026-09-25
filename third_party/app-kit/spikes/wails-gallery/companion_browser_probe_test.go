//go:build darwin && server && workflowprobe && recoveryprobe

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

// Reuse the complete prior workflow oracle, not a smaller permissive finish
// profile. The injected opener uses an isolated headed Chrome, never LaunchServices.
func TestActualNativeBrowserRecovery(t *testing.T) {
	path, digest := os.Getenv("APPKIT_RECOVERY_NATIVE"), os.Getenv("APPKIT_RECOVERY_SHA256")
	prepared, err := prepareCompanion(path, digest)
	if err != nil {
		t.Fatal("explicit native identity is required")
	}
	defer prepared.close()
	captureDir := os.Getenv("APPKIT_RECOVERY_CAPTURE_DIR")
	info, err := os.Lstat(captureDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !filepath.IsAbs(captureDir) {
		t.Fatal("explicit private capture directory is required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	meter := new(workflowMeter)
	h, err := startHost(ctx, meter.handler(gallery.New()))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	log := &nativeRecoveryLog{ready: make(chan struct{})}
	type openedBrowser struct {
		driver *workflowBrowser
		err    error
	}
	opened := make(chan openedBrowser, 1)
	result := make(chan error, 1)
	var opens atomic.Int32
	go func() {
		result <- runCompanion(ctx, h, prepared.path, log, func(h *host) error {
			opens.Add(1)
			d, err := startWorkflowBrowser(ctx, exec.CommandContext(ctx, "node", "workflow_browser.mjs"), h.baseURL, true, captureDir)
			if err == nil {
				launch, launchErr := h.auth.LaunchURL()
				if launchErr != nil {
					err = errors.New("browser launch unavailable")
				} else {
					err = d.New("handoff", launch)
				}
			}
			opened <- openedBrowser{d, err}
			return err
		}, 15*time.Second)
	}()
	defer func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Error("recovery parent returned failure during shutdown")
			}
		case <-time.After(7 * time.Second):
			t.Error("recovery parent did not stop")
		}
		if opens.Load() != 1 {
			t.Error("browser recovery was not exactly once through shutdown")
		}
	}()
	select {
	case <-log.ready:
	case <-ctx.Done():
		t.Fatal("native navigation missing")
	}
	child := exactNativeChild(t, prepared.path)
	if err := child.Kill(); err != nil {
		t.Fatal("could not inject native failure")
	}
	var browser openedBrowser
	select {
	case browser = <-opened:
	case <-time.After(25 * time.Second):
		t.Fatal("headed Chrome handoff did not complete")
	}
	if browser.driver != nil {
		defer browser.driver.shutdown()
	}
	if browser.err != nil {
		t.Fatal("headed Chrome handoff failed")
	}
	if child.Signal(syscall.Signal(0)) == nil {
		t.Fatal("failed native child was not reaped")
	}
	d := browser.driver
	value, err := d.Eval(ctx, "handoff", workflowSnapshot)
	var dom workflowDOM
	if err != nil || json.Unmarshal([]byte(value), &dom) != nil || !dom.Ready || dom.Name != "Alex Morgan" || dom.Version != "1" || dom.Path != "/shell/profile" || dom.External || dom.Runtime {
		t.Fatal("fresh-browser handoff did not render the truthful initial Gallery session")
	}
	// The independent browser starts a new Gallery session. No native cookie,
	// CSRF value, draft or state is injected to fabricate cross-client continuity.
	capture, err := d.Capture("handoff")
	if err != nil {
		t.Fatal("handoff capture failed")
	}
	var image struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if json.Unmarshal([]byte(capture), &image) != nil || image.File != "handoff.png" || image.Width != 1440 || image.Height != 1000 {
		t.Fatal("capture identity invalid")
	}
	f, err := os.Open(filepath.Join(captureDir, image.File))
	if err != nil {
		t.Fatal("capture unavailable")
	}
	payload, readErr := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	closeErr := f.Close()
	hash := sha256.Sum256(payload)
	if readErr != nil || closeErr != nil || len(payload) == 0 || len(payload) > 4<<20 || hex.EncodeToString(hash[:]) != image.SHA256 {
		t.Fatal("capture identity mismatch")
	}
	config, err := png.DecodeConfig(bytes.NewReader(payload))
	if err != nil || config.Width != image.Width || config.Height != image.Height {
		t.Fatal("capture is not a PNG with the claimed dimensions")
	}
	if err := d.Close("handoff"); err != nil {
		t.Fatal("initial handoff page did not close")
	}
	receipt := workflowReceipt{Schema: 1, Profile: "browser", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Driver: "instrumented DOM; not physical keyboard, IME or pointer"}
	runWorkflow(ctx, h, meter, d, &receipt)
	finishWorkflowHost(h, meter, &receipt)
	if err := verifyWorkflow(receipt); err != nil {
		t.Fatalf("post-native-failure workflow failed: step=%s completed=%v windows=%d", receipt.Failure, receipt.Completed, receipt.WindowsClosed)
	}
	if opens.Load() != 1 {
		t.Fatal("browser recovery was not exactly once")
	}
	if strings.Contains(log.String(), "token=") {
		t.Fatal("launch secret in diagnostics")
	}
	// Only fixed non-secret receipt fields are retained; no DOM, URLs or cookies.
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal("workflow receipt unavailable")
	}
	t.Logf("workflow_receipt=%s", encoded)
	t.Logf("chrome_headed_configured=true handoff=passed native_reaped=passed workflow_steps=%d capture=%s capture_sha256=%s user_agent=%s dimensions=%dx%d theme=%s", len(receipt.Steps), image.File, image.SHA256, receipt.UserAgent, receipt.Width, receipt.Height, receipt.Theme)
}
