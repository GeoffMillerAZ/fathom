//go:build darwin && !server && capabilityprobe && !workflowprobe && appkit_contained

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type capabilityPresence struct {
	Fixture                string `json:"fixture"`
	UserAgent              string `json:"user_agent"`
	Secure                 bool   `json:"secure"`
	Media                  bool   `json:"media"`
	Geolocation            bool   `json:"geolocation"`
	Notifications          bool   `json:"notifications"`
	NotificationPermission string `json:"notification_permission"`
	ClipboardRead          bool   `json:"clipboard_read"`
	Visible                bool   `json:"visible"`
	Focused                bool   `json:"focused"`
}

type capabilityPageResult struct {
	Started bool   `json:"started"`
	Done    bool   `json:"done"`
	Result  string `json:"result"`
	Tracks  int    `json:"tracks"`
}

type capabilityCase struct {
	ID       string                      `json:"id"`
	Presence capabilityPresence          `json:"presence"`
	Before   capabilityNativeObservation `json:"before"`
	After    capabilityNativeObservation `json:"after"`
	Page     capabilityPageResult        `json:"page"`
	Failure  string                      `json:"failure,omitempty"`
}

type capabilityReceipt struct {
	Schema            int              `json:"schema"`
	Profile           string           `json:"profile"`
	Go                string           `json:"go"`
	OS                string           `json:"os"`
	Arch              string           `json:"arch"`
	Cases             []capabilityCase `json:"cases"`
	Shutdown          bool             `json:"shutdown"`
	ListenerClosed    bool             `json:"listener_closed"`
	WindowsClosed     int              `json:"windows_closed"`
	FrameworkErrors   int32            `json:"framework_errors"`
	FrameworkWarnings int32            `json:"framework_warnings"`
	BridgeMessages    int32            `json:"bridge_messages"`
	Failure           string           `json:"failure,omitempty"`
	DiagnosticCase    string           `json:"diagnostic_case,omitempty"`
}

var capabilityIDs = [3]string{"camera", "microphone", "camera-and-microphone"}

func validateCapabilityCase(c capabilityCase, index int) error {
	if index < 0 || index >= len(capabilityIDs) || c.ID != capabilityIDs[index] || c.Failure != "" {
		return errors.New("missing, reordered or failed media case")
	}
	if c.Presence.Fixture != "appkit-media-denial-v1" || !c.Presence.Secure || !c.Presence.Media || !c.Presence.Visible || !c.Presence.Focused || !strings.Contains(c.Presence.UserAgent, "AppleWebKit/") {
		return errors.New("native secure-context/media prerequisite absent")
	}
	if !validCapabilityPreflight(c.Before) {
		return errors.New("native direct-denial preflight failed")
	}
	want := [3]uint64{1, 1, 1}
	want[index]++
	if !c.After.Available || !c.After.OriginConfigured || !c.After.OriginMatched || c.After.Denials != want || c.After.PreflightDenied != [3]bool{} || c.After.PreflightCalls != [3]uint64{} || c.After.PreflightResults != [3]int64{-1, -1, -1} {
		return errors.New("actual request did not reach the expected native denial callback")
	}
	if !c.Page.Started || !c.Page.Done || c.Page.Result != "NotAllowedError" || c.Page.Tracks != 0 {
		return errors.New("page did not observe completed denial without a media stream")
	}
	if !c.Presence.Notifications && c.Presence.NotificationPermission != "unavailable" || c.Presence.Notifications && c.Presence.NotificationPermission != "default" && c.Presence.NotificationPermission != "denied" && c.Presence.NotificationPermission != "granted" {
		return errors.New("invalid presence-only notification observation")
	}
	return nil
}

func validCapabilityPreflight(o capabilityNativeObservation) bool {
	// WKPermissionDecision: Prompt=0, Grant=1, Deny=2 in the pinned SDK contract.
	return o.Available && o.OriginConfigured && o.WindowVisible && o.WindowKey && o.ApplicationActive && o.Denials == [3]uint64{1, 1, 1} && !o.OriginMatched &&
		o.PreflightDenied == [3]bool{true, true, true} && o.PreflightCalls == [3]uint64{1, 1, 1} && o.PreflightResults == [3]int64{2, 2, 2}
}

func validateCapabilityReceipt(r capabilityReceipt) error {
	if r.Schema != 1 || r.Profile != "macos-http-only-media" || r.OS != "darwin" || r.Arch != "arm64" || !strings.HasPrefix(r.Go, "go1.") || r.Failure != "" || r.DiagnosticCase != "" {
		return errors.New("invalid or failed native media receipt")
	}
	if len(r.Cases) != 3 || !r.Shutdown || !r.ListenerClosed || r.WindowsClosed != 3 || r.FrameworkErrors != 0 || r.FrameworkWarnings != 0 || r.BridgeMessages != 0 {
		return errors.New("native media cases or teardown incomplete")
	}
	for i, c := range r.Cases {
		if err := validateCapabilityCase(c, i); err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	return nil
}

func readCapabilityReceipt(reader io.Reader) (capabilityReceipt, error) {
	var r capabilityReceipt
	data, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return r, errors.New("receipt unreadable or too large")
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return r, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return r, errors.New("trailing receipt content")
	}
	return r, validateCapabilityReceipt(r)
}

func TestMain(m *testing.M) {
	switch os.Getenv("APPKIT_CAPABILITY_MODE") {
	case "":
		os.Exit(m.Run())
	case "check":
		f, err := os.Open(os.Getenv("APPKIT_CAPABILITY_RECEIPT"))
		if err == nil {
			_, err = readCapabilityReceipt(f)
			_ = f.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "native media receipt failed:", err)
			os.Exit(1)
		}
		fmt.Println("native media receipt passed: 3 actual denial cases and teardown")
		os.Exit(0)
	case "run":
		selected := os.Getenv("APPKIT_CAPABILITY_DIAGNOSTIC_CASE")
		if selected != "" && selected != capabilityIDs[0] && selected != capabilityIDs[1] && selected != capabilityIDs[2] {
			fmt.Fprintln(os.Stderr, "unknown diagnostic media case")
			os.Exit(2)
		}
		runCapabilityMain()
		// A successful native run terminates through OnShutdown. Returning here
		// means that path was not established; exit zero alone is never proof.
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "unsupported native capability mode")
		os.Exit(2)
	}
}

func runCapabilityMain() {
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	h, err := startHost(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shell/profile" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		// No CSP or Permissions-Policy that could masquerade as native denial.
		// Only this inert synthetic page is served; no scripts or network assets.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>App Kit media denial probe</title><body data-fixture="appkit-media-denial-v1"><h1>Native media denial probe</h1><p>Synthetic local test. No capability grants. This window closes automatically.</p></body></html>`)
	}))
	if err != nil || requireNativeContainment() != nil {
		fmt.Fprintln(os.Stderr, "native media host unavailable")
		return
	}
	completed := make(chan capabilityReceipt, 1)
	var frameworkErrors, frameworkWarnings, bridge atomic.Int32
	initial := capabilityReceipt{Schema: 1, Profile: "macos-http-only-media", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, DiagnosticCase: os.Getenv("APPKIT_CAPABILITY_DIAGNOSTIC_CASE")}
	app := application.New(application.Options{
		Name: "App Kit Native Media Denial Probe", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets: httpOnlyAssets(), DisableDefaultSignalHandler: true,
		Mac:          application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		ErrorHandler: func(error) { frameworkErrors.Add(1) }, WarningHandler: func(string) { frameworkWarnings.Add(1) },
		RawMessageHandler: func(application.Window, string, *application.OriginInfo) { bridge.Add(1) },
		OnShutdown: func() {
			r := initial
			select {
			case r = <-completed:
			default:
				r.Failure = "shutdown-before-media-probe"
			}
			r.Shutdown = h.Close() == nil
			conn, dialErr := net.DialTimeout("tcp4", strings.TrimPrefix(h.baseURL, "http://"), time.Second)
			r.ListenerClosed = dialErr != nil
			if conn != nil {
				_ = conn.Close()
			}
			r.FrameworkErrors, r.FrameworkWarnings, r.BridgeMessages = frameworkErrors.Load(), frameworkWarnings.Load(), bridge.Load()
			if json.NewEncoder(os.Stdout).Encode(r) != nil {
				os.Exit(2)
			}
		},
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			r := initial
			for i, id := range capabilityIDs {
				if initial.DiagnosticCase != "" && initial.DiagnosticCase != id {
					continue
				}
				c, closed := runCapabilityCase(ctx, h, app, i, id)
				r.Cases = append(r.Cases, c)
				if closed {
					r.WindowsClosed++
				}
				if c.Page.Result == "granted" || !closed {
					break
				}
			}
			completed <- r
			app.Quit()
		}()
	})
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native media hard deadline"); os.Exit(2) })
	defer watchdog.Stop()
	if app.Run() != nil {
		fmt.Fprintln(os.Stderr, "native media event loop failed")
	}
}

func runCapabilityCase(ctx context.Context, h *host, app *application.App, index int, id string) (c capabilityCase, closed bool) {
	c.ID = id
	launch, err := h.auth.LaunchURL()
	if err != nil {
		c.Failure = "launch-unavailable"
		return
	}
	name := "media-" + id
	window := application.NewWindow(galleryNativeWindowOptions(name, launch, h.baseURL))
	loaded := make(chan struct{}, 1)
	window.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		select {
		case loaded <- struct{}{}:
		default:
		}
	})
	app.Window.Add(window)
	window.Run()
	defer func() {
		window.Close()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok := app.Window.GetByName(name); !ok {
				closed = true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		c.Failure = "window-close-timeout"
	}()
	select {
	case <-loaded:
	case <-ctx.Done():
		c.Failure = "context-ended"
		return
	case <-time.After(8 * time.Second):
		c.Failure = "navigation-timeout"
		return
	}
	// Wails' own Show listener and our navigation listener run independently.
	// Navigation completion alone does not establish visible/active page state.
	window.Show()
	window.Focus()
	var value string
	visibilityDeadline := time.Now().Add(4 * time.Second)
	for {
		value, err = workflowNativeEval(ctx, window, capabilityPresenceScript)
		if err != nil || json.Unmarshal([]byte(value), &c.Presence) != nil {
			c.Failure = "presence-unavailable"
			return
		}
		if c.Presence.Visible && c.Presence.Focused {
			break
		}
		if time.Now().After(visibilityDeadline) {
			c.Failure = "page-not-visible-focused-no-device-request"
			return
		}
		select {
		case <-ctx.Done():
			c.Failure = "context-ended"
			return
		case <-time.After(25 * time.Millisecond):
		}
	}
	c.Before, err = observeCapability(ctx, window, h.baseURL, true)
	if err != nil || !validCapabilityPreflight(c.Before) {
		c.Failure = "native-preflight-failed-no-device-request"
		return
	}
	if c.Presence.Fixture != "appkit-media-denial-v1" || !c.Presence.Secure || !c.Presence.Media {
		c.Failure = "media-prerequisite-absent-no-device-request"
		return
	}
	constraints := [3]string{`{video:true}`, `{audio:true}`, `{video:true,audio:true}`}
	value, err = workflowNativeEval(ctx, window, strings.Replace(capabilityStartScript, "CONSTRAINTS", constraints[index], 1))
	if err != nil || value != `"started"` {
		c.Failure = "media-start-failed"
		return
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		value, err = workflowNativeEval(ctx, window, `JSON.stringify(window.__appkitMediaAttempt)`)
		if err != nil || json.Unmarshal([]byte(value), &c.Page) != nil {
			c.Failure = "media-result-unavailable"
			return
		}
		if c.Page.Done {
			break
		}
		select {
		case <-ctx.Done():
			c.Failure = "media-timeout"
			return
		case <-time.After(25 * time.Millisecond):
		}
	}
	c.After, err = observeCapability(ctx, window, h.baseURL, false)
	if err != nil {
		c.Failure = "native-final-observation-failed"
		return
	}
	if err = validateCapabilityCase(c, index); err != nil {
		c.Failure = err.Error()
	}
	return
}

const capabilityPresenceScript = `JSON.stringify({fixture:document.body.dataset.fixture,user_agent:navigator.userAgent,secure:window.isSecureContext,visible:document.visibilityState==='visible',focused:document.hasFocus(),media:typeof navigator.mediaDevices?.getUserMedia==='function',geolocation:typeof navigator.geolocation?.getCurrentPosition==='function',notifications:typeof Notification!=='undefined',notification_permission:typeof Notification==='undefined'?'unavailable':Notification.permission,clipboard_read:typeof navigator.clipboard?.read==='function'||typeof navigator.clipboard?.readText==='function'})`

// Only invoked after native direct-denial preflight. No enumerateDevices, device
// IDs, frame/audio reads, notification/geo/clipboard calls or permission grants.
const capabilityStartScript = `(function(){window.__appkitMediaAttempt={started:true,done:false,result:'pending',tracks:0};(async()=>{try{const stream=await navigator.mediaDevices.getUserMedia(CONSTRAINTS);const tracks=stream.getTracks();tracks.forEach(t=>t.stop());window.__appkitMediaAttempt={started:true,done:true,result:'granted',tracks:tracks.length};}catch(e){const names=['NotAllowedError','NotFoundError','NotReadableError','SecurityError','TypeError','AbortError','OverconstrainedError'];window.__appkitMediaAttempt={started:true,done:true,result:names.includes(e?.name)?e.name:'unexpected-error',tracks:0};}})();return JSON.stringify('started');})()`
