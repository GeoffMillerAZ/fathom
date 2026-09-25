//go:build darwin && !server && nativeprobe

// native-probe exercises actual WKWebViews using synthetic loopback pages.
// It is opt-in, self-closing, and deliberately does not use the Wails JS bridge.
package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type observation struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	UserAgent    string `json:"user_agent,omitempty"`
	External     bool   `json:"external,omitempty"`
	Runtime      bool   `json:"runtime,omitempty"`
	Cookie       bool   `json:"cookie,omitempty"`
	Opener       bool   `json:"opener,omitempty"`
	PostAccepted bool   `json:"post_accepted,omitempty"`
}

type result struct {
	ID           string        `json:"id"`
	Passed       bool          `json:"passed"`
	Closed       bool          `json:"closed"`
	Reason       string        `json:"reason"`
	Observations []observation `json:"observations"`
}

type receipt struct {
	Profile, Go, OS, Arch string
	Completed             bool
	ShutdownCallback      bool
	FrameworkErrors       int
	FrameworkWarnings     int
	HostBridgeMessages    int
	Cases                 []result
}

type fixture struct {
	mu                 sync.Mutex
	seen               map[string][]observation
	loads              map[string]int
	origin             string
	foreign            string
	token              string
	results            []result
	frameworkErrors    int
	frameworkWarnings  int
	hostBridgeMessages int
}

var cases = []string{"bootstrap", "redirect", "reload", "foreign-port", "foreign-host", "foreign-redirect", "about", "data", "wails", "frame-same", "frame-foreign", "popup-same", "popup-foreign", "bridge"}

const bridgeCanary = "appkit:containment-probe"

func (f *fixture) record(o observation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen[o.ID] = append(f.seen[o.ID], o)
}

func (f *fixture) snapshot(id string) []observation {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]observation(nil), f.seen[id]...)
}

func (f *fixture) recordHostMessage(window application.Window, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if window == nil || window.Name() != "probe-bridge" || message != bridgeCanary {
		f.frameworkErrors++
		return
	}
	f.hostBridgeMessages++
	f.seen["bridge"] = append(f.seen["bridge"], observation{ID: "bridge", Kind: "host-message"})
}

func (f *fixture) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := r.URL.Query().Get("id")
	if r.URL.Path == "/report" {
		peer, _, peerErr := net.SplitHostPort(r.RemoteAddr)
		if peerErr != nil || net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() ||
			r.Host != strings.TrimPrefix(f.origin, "http://") || r.Method != http.MethodPost ||
			r.Header.Get("Origin") != f.origin || r.Header.Get("Content-Type") != "application/json" ||
			r.Header.Get("X-AppKit-Probe") != f.token {
			http.Error(w, "refused", http.StatusForbidden)
			return
		}
		var o observation
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&o) != nil || !slices.Contains(cases, o.ID) ||
			!slices.Contains([]string{"started", "complete", "bridge", "popup", "retained"}, o.Kind) {
			http.Error(w, "bad report", http.StatusBadRequest)
			return
		}
		f.record(o)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/launch" {
		// Synthetic only: real launch-token/CSRF handling has separate host tests.
		http.SetCookie(w, &http.Cookie{Name: "probe", Value: "synthetic", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		f.record(observation{ID: id, Kind: "launch"})
		http.Redirect(w, r, "/case?id="+url.QueryEscape(id), http.StatusSeeOther)
		return
	}
	if r.URL.Path == "/redirect" || r.URL.Path == "/foreign-redirect" {
		f.record(observation{ID: id, Kind: "redirect-request"})
		target := f.origin
		if r.URL.Path == "/foreign-redirect" {
			target = f.foreign
		}
		http.Redirect(w, r, target+"/arrival?id="+url.QueryEscape(id), http.StatusFound)
		return
	}
	if r.URL.Path == "/arrival" || r.URL.Path == "/frame" || r.URL.Path == "/popup" {
		f.record(observation{ID: id, Kind: "destination-request"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><title>Synthetic destination</title><p>Synthetic destination reached.</p>")
		return
	}
	if r.URL.Path != "/case" {
		http.NotFound(w, r)
		return
	}
	f.mu.Lock()
	f.loads[id]++
	count := f.loads[id]
	f.mu.Unlock()
	cookie, err := r.Cookie("probe")
	f.record(observation{ID: id, Kind: "case-request", Cookie: err == nil && cookie.Value == "synthetic"})
	u, _ := url.Parse(f.origin)
	target := f.origin + "/arrival?id=" + url.QueryEscape(id)
	switch id {
	case "foreign-port", "frame-foreign", "popup-foreign":
		target = f.foreign + "/arrival?id=" + url.QueryEscape(id)
	case "foreign-host":
		target = "http://localhost:" + u.Port() + "/arrival?id=" + url.QueryEscape(id)
	case "foreign-redirect":
		target = f.origin + "/foreign-redirect?id=" + url.QueryEscape(id)
	case "redirect":
		target = f.origin + "/redirect?id=" + url.QueryEscape(id)
	case "about":
		target = "about:blank"
	case "data":
		target = "data:text/html,<title>Synthetic destination</title>"
	case "wails":
		target = "wails://wails.localhost/arrival"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, struct {
		ID, Target, Token string
		Count             int
	}{id, target, f.token, count})
}

// No CSP here: refusal must come from the native policy, not page restrictions.
// No external assets, real data, OS protocol handlers, or file reads are attempted.
var page = template.Must(template.New("case").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><title>App Kit native containment probe</title>
<h1>Native boundary probe</h1><p>Synthetic case: {{.ID}}. This window closes automatically.</p>
<script>
(async () => {
 const id={{.ID}}, target={{.Target}}, count={{.Count}}, token={{.Token}};
 const report=async(kind,extra={})=>{const r=await fetch('/report',{method:'POST',headers:{'Content-Type':'application/json','X-AppKit-Probe':token},body:JSON.stringify({id,kind,user_agent:navigator.userAgent,...extra})});if(!r.ok)throw Error('report refused');};
 await report('started');
 if(id==='bootstrap'){await report('complete');return;}
 if(id==='reload'){if(count===1)location.reload();else await report('complete');return;}
 if(id==='bridge'){setTimeout(()=>{let post_accepted=false;try{window.webkit.messageHandlers.external.postMessage('` + bridgeCanary + `');post_accepted=true;}catch{}return report('bridge',{post_accepted,external:typeof window.webkit?.messageHandlers?.external?.postMessage==='function',runtime:typeof window.wails!=='undefined'||typeof window._wails!=='undefined'});},700);return;}
 if(id.startsWith('frame-')){const f=document.createElement('iframe');f.src=target;document.body.append(f);}
 else if(id.startsWith('popup-')){const opened=window.open(target,'_blank');await report('popup',{opener:opened!==null});if(opened)opened.close();}
 else location.assign(target);
 setTimeout(()=>report('retained'),1000);
})().catch(()=>{});
</script></html>`))

// The same oracle is used for baseline and patched binaries. Missing execution,
// timeout, or a failed positive control is never interpreted as a refusal pass.
func judge(id string, observations []observation) result {
	r := result{ID: id, Observations: observations}
	for _, o := range observations {
		if o.ID != id {
			r.Reason = "observation case identity mismatch"
			return r
		}
		if o.Kind == "case-request" && !o.Cookie {
			r.Reason = "synthetic bootstrap cookie missing"
			return r
		}
		if o.Kind == "host-message" || o.Kind == "bridge" && (o.External || o.Runtime || o.PostAccepted) {
			r.Reason = "native bridge or injected runtime exposed"
			return r
		}
		if o.Kind == "popup" && o.Opener {
			r.Reason = "popup returned a window"
			return r
		}
	}
	expected := expectedObservationKinds(id)
	if len(expected) == 0 || len(observations) != len(expected) {
		r.Reason = "missing or duplicate expected observation"
		return r
	}
	for i, kind := range expected {
		if observations[i].Kind != kind {
			r.Reason = "observation order or kind mismatch"
			return r
		}
	}
	r.Passed = true
	r.Reason = "required controls and bounded observation passed"
	return r
}

func expectedObservationKinds(id string) []string {
	switch id {
	case "bootstrap":
		return []string{"launch", "case-request", "started", "complete"}
	case "reload":
		return []string{"launch", "case-request", "started", "case-request", "started", "complete"}
	case "redirect":
		return []string{"launch", "case-request", "started", "redirect-request", "destination-request"}
	case "foreign-redirect":
		return []string{"launch", "case-request", "started", "redirect-request", "retained"}
	case "popup-same", "popup-foreign":
		return []string{"launch", "case-request", "started", "popup", "retained"}
	case "bridge":
		return []string{"launch", "case-request", "started", "bridge"}
	case "foreign-port", "foreign-host", "about", "data", "wails", "frame-same", "frame-foreign":
		return []string{"launch", "case-request", "started", "retained"}
	default:
		return nil
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ack := flag.Bool("allow-uncontained-baseline", false, "explicitly permit the unchanged Wails pin for synthetic red-control testing")
	check := flag.String("check", "", "verify a completed JSON receipt without launching a WebView")
	flag.Parse()
	if *check != "" {
		data, err := os.ReadFile(*check)
		if err != nil {
			return err
		}
		r, err := decodeReceipt(data)
		if err != nil {
			return err
		}
		return verifyReceipt(r)
	}
	if !contained && !*ack {
		return errors.New("baseline requires -allow-uncontained-baseline; synthetic use only")
	}
	f := &fixture{seen: map[string][]observation{}, loads: map[string]int{}, token: rand.Text()}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	_ = srv.Listener.Close()
	srv.Listener = ln
	f.origin = "http://" + ln.Addr().String()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.record(observation{ID: r.URL.Query().Get("id"), Kind: "destination-request"})
		fmt.Fprint(w, "<!doctype html><p>Synthetic foreign origin</p>")
	}))
	f.foreign = foreign.URL
	srv.Start()
	defer srv.Close()
	defer foreign.Close()
	app := application.New(application.Options{
		Name: "App Kit Native Containment Probe", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets:                      application.AssetOptions{Handler: http.NotFoundHandler(), DisableLogging: true},
		DisableDefaultSignalHandler: true,
		ErrorHandler:                func(error) { f.mu.Lock(); f.frameworkErrors++; f.mu.Unlock() },
		WarningHandler:              func(string) { f.mu.Lock(); f.frameworkWarnings++; f.mu.Unlock() },
		RawMessageHandler: func(window application.Window, message string, _ *application.OriginInfo) {
			f.recordHostMessage(window, message)
		},
		OnShutdown: func() {
			// NSApp terminate exits the process; Go code after app.Run and Go
			// defers are not a reliable receipt/cleanup path at this pin.
			f.mu.Lock()
			r := receipt{Profile: profile, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
				Completed: len(f.results) == len(cases), ShutdownCallback: true,
				FrameworkErrors: f.frameworkErrors, FrameworkWarnings: f.frameworkWarnings,
				HostBridgeMessages: f.hostBridgeMessages, Cases: f.results}
			f.mu.Unlock()
			srv.Close()
			foreign.Close()
			if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
				fmt.Fprintln(os.Stderr, "receipt write failed")
			}
		},
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			var all []result
			for _, id := range cases {
				preferences := application.MacWebviewPreferences{}
				// The popup probe must exercise the native policy, not this preference.
				preferences.JavaScriptCanOpenWindowsAutomatically.Set(true)
				opts := application.WebviewWindowOptions{Name: "probe-" + id, Title: "Synthetic native probe: " + id, URL: f.origin + "/launch?id=" + id, Width: 640, Height: 400, Mac: application.MacWindow{WebviewPreferences: preferences}}
				setPolicy(&opts, f.origin)
				window := app.Window.NewWithOptions(opts)
				// Every case gets the same observation window; refusal is not inferred
				// from a lost page. 'retained' must arrive after the actual attempt.
				time.Sleep(2 * time.Second)
				r := judge(id, f.snapshot(id))
				window.Close()
				if !waitForWindowRemoval(app, opts.Name, time.Second) {
					r.Passed = false
					r.Reason = "native window did not close within bounded wait"
					all = append(all, r)
					break
				}
				r.Closed = true
				all = append(all, r)
			}
			f.mu.Lock()
			f.results = all
			f.mu.Unlock()
			app.Quit()
		}()
	})
	// A crashed/non-starting app must not hang a unattended probe indefinitely.
	timer := time.AfterFunc(50*time.Second, func() { fmt.Fprintln(os.Stderr, "native probe deadline exceeded"); os.Exit(2) })
	defer timer.Stop()
	if err := app.Run(); err != nil {
		return errors.New("native app failed to run")
	}
	return errors.New("native loop returned unexpectedly; require a complete shutdown receipt")
}

func waitForWindowRemoval(app *application.App, name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, exists := app.Window.GetByName(name); !exists {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, exists := app.Window.GetByName(name)
	return !exists
}

func decodeReceipt(data []byte) (receipt, error) {
	var r receipt
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return receipt{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return receipt{}, errors.New("receipt contains trailing JSON value")
		}
		return receipt{}, err
	}
	return r, nil
}

func verifyReceipt(r receipt) error {
	if r.Profile != profile || r.Go != runtime.Version() || r.OS != runtime.GOOS || r.Arch != runtime.GOARCH {
		return errors.New("native receipt provenance mismatch")
	}
	if !r.Completed || !r.ShutdownCallback || len(r.Cases) != len(cases) || r.FrameworkErrors != 0 || r.FrameworkWarnings != 0 || r.HostBridgeMessages != 0 {
		return errors.New("native execution incomplete or framework/bridge signal recorded")
	}
	for i, id := range cases {
		got := r.Cases[i]
		want := judge(id, got.Observations)
		if got.ID != id || !got.Closed || !want.Passed || got.Passed != want.Passed || got.Reason != want.Reason {
			return fmt.Errorf("native containment oracle failed: %s", id)
		}
	}
	return nil
}
