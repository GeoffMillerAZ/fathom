//go:build workflowprobe

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

// This oracle drives the real Gallery, bootstrap and HTTP host. Only the DOM
// driver differs. No saved state, auth headers, form body or SSE frame is forged.
type workflowDriver interface {
	New(string, string) error
	Eval(context.Context, string, string) (string, error)
	Navigate(string, string) error
	Close(string) error
	Finish() error
}

type workflowStep struct {
	ID           string `json:"id"`
	Passed       bool   `json:"passed"`
	Milliseconds int64  `json:"milliseconds"`
}

type workflowReceipt struct {
	Schema            int            `json:"schema"`
	Profile           string         `json:"profile"`
	Go                string         `json:"go"`
	OS                string         `json:"os"`
	Arch              string         `json:"arch"`
	Driver            string         `json:"driver"`
	UserAgent         string         `json:"user_agent"`
	Width             int            `json:"width_css_px"`
	Height            int            `json:"height_css_px"`
	Theme             string         `json:"theme"`
	Completed         bool           `json:"completed"`
	Shutdown          bool           `json:"shutdown"`
	ListenerClosed    bool           `json:"listener_closed"`
	WindowsClosed     int            `json:"windows_closed"`
	FrameworkErrors   int            `json:"framework_errors"`
	FrameworkWarnings int            `json:"framework_warnings"`
	BridgeMessages    int            `json:"bridge_messages"`
	Failure           string         `json:"failure,omitempty"`
	Posts             []int          `json:"post_statuses"`
	Streams           int            `json:"streams_opened"`
	ActiveStreams     int            `json:"active_streams"`
	Steps             []workflowStep `json:"steps"`
}

var workflowIDs = []string{
	"same-session-two-windows", "validation-retains-draft", "corrected-save",
	"stale-draft-conflict", "explicit-reviewed-save", "arm-save-failure",
	"failed-save-retains-draft", "explicit-save-retry", "observer-idle",
	"explicit-connect", "second-window-live-update", "stop-without-reconnect",
	"manual-reconnect-preserves-draft", "signout-redacts-observer",
}

type workflowMeter struct {
	mu             sync.Mutex
	posts          []int
	opened, active int
}

// Never wrap streaming writers: Gallery must see the real transport's Flush
// and deadline support. The wrapper below is used ONLY for ordinary POSTs.
func (m *workflowMeter) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shell/events" {
			m.mu.Lock()
			m.opened++
			m.active++
			m.mu.Unlock()
			defer func() { m.mu.Lock(); m.active--; m.mu.Unlock() }()
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/shell/action" {
			status := &workflowStatus{ResponseWriter: w}
			next.ServeHTTP(status, r)
			m.mu.Lock()
			m.posts = append(m.posts, status.status)
			m.mu.Unlock()
			return
		}
		next.ServeHTTP(w, r)
	})
}

type workflowStatus struct {
	http.ResponseWriter
	status int
}

func (w *workflowStatus) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *workflowStatus) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func TestMain(m *testing.M) {
	profile := os.Getenv("APPKIT_WORKFLOW_PROFILE")
	if profile == "" {
		os.Exit(m.Run())
	}
	if profile == "check" {
		f, err := os.Open(os.Getenv("APPKIT_WORKFLOW_RECEIPT"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "receipt unavailable")
			os.Exit(1)
		}
		defer f.Close()
		var r workflowReceipt
		d := json.NewDecoder(io.LimitReader(f, 65537))
		d.DisallowUnknownFields()
		if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || verifyWorkflow(r) != nil {
			fmt.Fprintln(os.Stderr, "workflow receipt failed strict oracle")
			os.Exit(1)
		}
		fmt.Println("workflow receipt passes shared oracle")
		os.Exit(0)
	}
	if profile != "browser" && profile != "native" {
		fmt.Fprintln(os.Stderr, "unknown workflow profile")
		os.Exit(2)
	}
	r := workflowReceipt{Schema: 1, Profile: profile, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
		Driver: "instrumented DOM; not physical keyboard, IME or pointer"}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	meter := new(workflowMeter)
	h, err := startHost(ctx, meter.handler(gallery.New()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow HTTP startup failed")
		os.Exit(2)
	}
	// No launch capability, cookies, CSRF, full DOM or diagnostics enter receipts.
	if profile == "native" {
		workflowNativeMain(ctx, h, meter, &r)
		os.Exit(2)
	}
	driver, err := newWorkflowBrowser(ctx, h.baseURL)
	if err != nil {
		r.Failure = "browser-driver-start"
	} else {
		runWorkflow(ctx, h, meter, driver, &r)
	}
	finishWorkflowHost(h, meter, &r)
	writeWorkflowReceipt(r)
	if verifyWorkflow(r) != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func finishWorkflowHost(h *host, meter *workflowMeter, r *workflowReceipt) {
	r.Shutdown = h.Close() == nil
	conn, err := net.DialTimeout("tcp4", h.auth.host, 250*time.Millisecond)
	if err == nil {
		conn.Close()
	} else {
		r.ListenerClosed = true
	}
	meter.mu.Lock()
	r.Posts = append([]int(nil), meter.posts...)
	r.Streams, r.ActiveStreams = meter.opened, meter.active
	meter.mu.Unlock()
}

func writeWorkflowReceipt(r workflowReceipt) {
	if json.NewEncoder(os.Stdout).Encode(r) != nil {
		fmt.Fprintln(os.Stderr, "workflow receipt write failed")
	}
}

func verifyWorkflow(r workflowReceipt) error {
	if r.Schema != 1 || !slices.Contains([]string{"browser", "native"}, r.Profile) ||
		r.Go != runtime.Version() || r.OS != runtime.GOOS || r.Arch != runtime.GOARCH ||
		r.Driver != "instrumented DOM; not physical keyboard, IME or pointer" ||
		r.UserAgent == "" || len(r.UserAgent) > 512 ||
		r.Width < 390 || r.Height < 400 || !slices.Contains([]string{"light", "dark"}, r.Theme) ||
		!r.Completed || !r.Shutdown || !r.ListenerClosed || r.WindowsClosed != 2 ||
		r.FrameworkErrors != 0 || r.FrameworkWarnings != 0 || r.BridgeMessages != 0 || r.Failure != "" ||
		r.Streams != 2 || r.ActiveStreams != 0 || len(r.Steps) != len(workflowIDs) ||
		!slices.Equal(r.Posts, []int{422, 303, 409, 303, 303, 503, 303, 303, 303}) {
		return errors.New("incomplete workflow")
	}
	for i, s := range r.Steps {
		if s.ID != workflowIDs[i] || !s.Passed || s.Milliseconds < 0 || s.Milliseconds > 15000 {
			return errors.New("workflow step failed")
		}
	}
	return nil
}

// Fixed, source-reviewed scripts only. Export neither an arbitrary-script flag
// nor a native HTTP eval route. Synthetic version numbers are not credentials.
const workflowSnapshot = `JSON.stringify({ready:document.readyState==='complete',path:location.pathname,ua:navigator.userAgent,
 width:innerWidth,height:innerHeight,theme:document.documentElement.classList.contains('dark')?'dark':'light',
 name:document.querySelector('#shell-display_name')?.value||'',
 version:document.querySelector('[data-shell-save]')?.form.querySelector('[name=version]')?.value||'',
 invalid:document.querySelector('#shell-display_name')?.getAttribute('aria-invalid')==='true',
 conflict:document.querySelector('[aria-labelledby=shell-conflict-title]')?.textContent||'',
 save:document.querySelector('[data-shell-save]')?.textContent.trim()||'',
 live:document.querySelector('#shell-live')?.dataset.state||'',
 summary:document.querySelector('#shell-live-summary')?.textContent||'',
 hidden:document.querySelector('#shell-live-content')?.hidden===true,
 signin:!!document.querySelector('input[name=action][value=signin]'),
 activity:document.querySelectorAll('#shell-live-activity li').length,
 connect:!!document.querySelector('#shell-live-start:not([disabled])'),
 external:typeof window.webkit?.messageHandlers?.external?.postMessage==='function',
 runtime:typeof window.wails!=='undefined'||typeof window._wails!=='undefined'})`

type workflowDOM struct {
	UserAgent string `json:"ua"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Theme     string `json:"theme"`
	Ready     bool   `json:"ready"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Invalid   bool   `json:"invalid"`
	Conflict  string `json:"conflict"`
	Save      string `json:"save"`
	Live      string `json:"live"`
	Summary   string `json:"summary"`
	Hidden    bool   `json:"hidden"`
	SignIn    bool   `json:"signin"`
	Activity  int    `json:"activity"`
	Connect   bool   `json:"connect"`
	External  bool   `json:"external"`
	Runtime   bool   `json:"runtime"`
}

func runWorkflow(ctx context.Context, h *host, meter *workflowMeter, d workflowDriver, r *workflowReceipt) {
	opened := []string{}
	defer func() {
		for _, name := range opened {
			if d.Close(name) == nil {
				r.WindowsClosed++
			} else {
				r.Failure = "window-close"
			}
		}
		if d.Finish() != nil {
			r.Failure = "driver-finish"
		}
	}()
	step := func(id string, work func(context.Context) error) bool {
		started := time.Now()
		bounded, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		err := work(bounded)
		r.Steps = append(r.Steps, workflowStep{ID: id, Passed: err == nil, Milliseconds: time.Since(started).Milliseconds()})
		if err != nil {
			r.Failure = id
			return false
		}
		return true
	}
	eval := func(ctx context.Context, name, code string) error {
		value, err := d.Eval(ctx, name, `(()=>{`+code+`;return JSON.stringify({ok:true})})()`)
		if err != nil || value != `{"ok":true}` {
			return errors.New("DOM action failed")
		}
		return nil
	}
	wait := func(ctx context.Context, name string, test func(workflowDOM) bool) error {
		for ctx.Err() == nil {
			value, err := d.Eval(ctx, name, workflowSnapshot)
			var dom workflowDOM
			if err == nil && json.Unmarshal([]byte(value), &dom) == nil && dom.Ready && !dom.External && !dom.Runtime && test(dom) {
				if r.UserAgent == "" {
					r.UserAgent = dom.UserAgent
					r.Width, r.Height, r.Theme = dom.Width, dom.Height, dom.Theme
				}
				if dom.UserAgent == "" || dom.UserAgent != r.UserAgent {
					return errors.New("client identity changed")
				}
				return nil
			}
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
		}
		return errors.New("DOM condition timeout")
	}
	fill := func(ctx context.Context, name, value string) error {
		quoted, _ := json.Marshal(value)
		return eval(ctx, name, `const e=document.querySelector('#shell-display_name');e.value=`+string(quoted)+`;e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}))`)
	}
	submit := func(ctx context.Context, name string, index, status int) error {
		if err := eval(ctx, name, `const b=document.querySelector('[data-shell-save]');setTimeout(()=>b.form.requestSubmit(b),0)`); err != nil {
			return err
		}
		return workflowWaitPosts(ctx, meter, index, status)
	}
	profile := func(ctx context.Context, name, value, version string) error {
		return wait(ctx, name, func(v workflowDOM) bool {
			return v.Path == "/shell/profile" && v.Name == value && v.Version == version && v.Save == "Save changes"
		})
	}
	if !step(workflowIDs[0], func(ctx context.Context) error {
		for _, name := range []string{"a", "b"} {
			launch, err := h.auth.LaunchURL()
			if err != nil {
				return err
			}
			if err = d.New(name, launch); err != nil {
				return err
			}
			opened = append(opened, name)
			if err = profile(ctx, name, "Alex Morgan", "1"); err != nil {
				return err
			}
		}
		return fill(ctx, "a", "Reviewed Workflow Avery")
	}) {
		return
	}
	if !step(workflowIDs[1], func(ctx context.Context) error {
		if err := fill(ctx, "b", strings.Repeat("x", 81)); err != nil {
			return err
		}
		if err := submit(ctx, "b", 1, 422); err != nil {
			return err
		}
		return wait(ctx, "b", func(v workflowDOM) bool {
			return v.Invalid && v.Name == strings.Repeat("x", 81) && v.Version == "1"
		})
	}) {
		return
	}
	if !step(workflowIDs[2], func(ctx context.Context) error {
		if err := fill(ctx, "b", "Newer Workflow Casey"); err != nil {
			return err
		}
		if err := submit(ctx, "b", 2, 303); err != nil {
			return err
		}
		return profile(ctx, "b", "Newer Workflow Casey", "2")
	}) {
		return
	}
	if !step(workflowIDs[3], func(ctx context.Context) error {
		if err := submit(ctx, "a", 3, 409); err != nil {
			return err
		}
		return wait(ctx, "a", func(v workflowDOM) bool {
			return v.Name == "Reviewed Workflow Avery" && v.Version == "2" && v.Save == "Apply reviewed draft" && strings.Contains(v.Conflict, "Newer Workflow Casey")
		})
	}) {
		return
	}
	if !step(workflowIDs[4], func(ctx context.Context) error {
		if err := submit(ctx, "a", 4, 303); err != nil {
			return err
		}
		return profile(ctx, "a", "Reviewed Workflow Avery", "3")
	}) {
		return
	}
	if !step(workflowIDs[5], func(ctx context.Context) error {
		if err := d.Navigate("b", "/shell/profile"); err != nil {
			return err
		}
		if err := profile(ctx, "b", "Reviewed Workflow Avery", "3"); err != nil {
			return err
		}
		if err := eval(ctx, "b", `const c=document.querySelector('[name=fail_next_save]');c.closest('details').open=true;c.checked=true;setTimeout(()=>c.form.requestSubmit(c.form.querySelector('button')),0)`); err != nil {
			return err
		}
		if err := workflowWaitPosts(ctx, meter, 5, 303); err != nil {
			return err
		}
		if err := d.Navigate("a", "/shell/profile"); err != nil {
			return err
		}
		return profile(ctx, "a", "Reviewed Workflow Avery", "4")
	}) {
		return
	}
	if !step(workflowIDs[6], func(ctx context.Context) error {
		if err := fill(ctx, "a", "Retried Workflow Avery"); err != nil {
			return err
		}
		if err := submit(ctx, "a", 6, 503); err != nil {
			return err
		}
		return wait(ctx, "a", func(v workflowDOM) bool {
			return v.Name == "Retried Workflow Avery" && v.Version == "4" && v.Path == "/shell/action"
		})
	}) {
		return
	}
	if !step(workflowIDs[7], func(ctx context.Context) error {
		if err := submit(ctx, "a", 7, 303); err != nil {
			return err
		}
		return profile(ctx, "a", "Retried Workflow Avery", "5")
	}) {
		return
	}
	if !step(workflowIDs[8], func(ctx context.Context) error {
		if err := d.Navigate("b", "/shell/activity"); err != nil {
			return err
		}
		if err := wait(ctx, "b", func(v workflowDOM) bool {
			return v.Live == "idle" && v.Connect && strings.Contains(v.Summary, "Page-load version 5 · Retried Workflow Avery")
		}); err != nil {
			return err
		}
		return workflowStreams(meter, 0, 0)
	}) {
		return
	}
	if !step(workflowIDs[9], func(ctx context.Context) error {
		if err := eval(ctx, "b", `document.querySelector('#shell-live-start').click()`); err != nil {
			return err
		}
		if err := wait(ctx, "b", func(v workflowDOM) bool {
			return v.Live == "watching" && v.Connect == false && strings.Contains(v.Summary, "Last observed version 5 · Retried Workflow Avery")
		}); err != nil {
			return err
		}
		return workflowStreams(meter, 1, 1)
	}) {
		return
	}
	if !step(workflowIDs[10], func(ctx context.Context) error {
		if err := fill(ctx, "a", "Live Workflow Avery"); err != nil {
			return err
		}
		if err := submit(ctx, "a", 8, 303); err != nil {
			return err
		}
		if err := profile(ctx, "a", "Live Workflow Avery", "6"); err != nil {
			return err
		}
		return wait(ctx, "b", func(v workflowDOM) bool {
			return v.Live == "watching" && strings.Contains(v.Summary, "Last observed version 6 · Live Workflow Avery") && v.Activity > 0
		})
	}) {
		return
	}
	if !step(workflowIDs[11], func(ctx context.Context) error {
		if err := fill(ctx, "a", "Unsubmitted workflow draft"); err != nil {
			return err
		}
		if err := eval(ctx, "b", `document.querySelector('#shell-live-stop').click()`); err != nil {
			return err
		}
		if err := wait(ctx, "b", func(v workflowDOM) bool { return v.Live == "paused" }); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1400 * time.Millisecond):
		}
		return workflowStreams(meter, 1, 0)
	}) {
		return
	}
	if !step(workflowIDs[12], func(ctx context.Context) error {
		if err := eval(ctx, "b", `document.querySelector('#shell-live-start').click()`); err != nil {
			return err
		}
		if err := wait(ctx, "b", func(v workflowDOM) bool {
			return v.Live == "watching" && strings.Contains(v.Summary, "Last observed version 6 · Live Workflow Avery")
		}); err != nil {
			return err
		}
		if err := workflowStreams(meter, 2, 1); err != nil {
			return err
		}
		return wait(ctx, "a", func(v workflowDOM) bool { return v.Name == "Unsubmitted workflow draft" && v.Version == "6" })
	}) {
		return
	}
	if !step(workflowIDs[13], func(ctx context.Context) error {
		if err := fill(ctx, "a", "Live Workflow Avery"); err != nil {
			return err
		}
		if err := eval(ctx, "a", `const f=document.querySelector('input[name=action][value=signout]').form;const detail=f.closest('details');if(detail)detail.open=true;setTimeout(()=>f.requestSubmit(f.querySelector('button')),0)`); err != nil {
			return err
		}
		if err := workflowWaitPosts(ctx, meter, 9, 303); err != nil {
			return err
		}
		if err := wait(ctx, "b", func(v workflowDOM) bool { return v.Live == "revoked" && v.Hidden && v.Summary == "" && v.Activity == 0 }); err != nil {
			return err
		}
		// The observer can revoke before the submitting view finishes its 303 ->
		// 401 navigation. The user's task is not settled until BOTH views reflect
		// sign-out; closing the submitter early also races browser teardown.
		return wait(ctx, "a", func(v workflowDOM) bool { return v.Path == "/shell/profile" && v.SignIn && v.Name == "" })
	}) {
		return
	}
	r.Completed = true
}

func workflowWaitPosts(ctx context.Context, m *workflowMeter, count, status int) error {
	for ctx.Err() == nil {
		m.mu.Lock()
		n := len(m.posts)
		got := 0
		if n > 0 {
			got = m.posts[n-1]
		}
		m.mu.Unlock()
		if n >= count {
			if n == count && got == status {
				return nil
			}
			return errors.New("POST sequence mismatch")
		}
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Millisecond):
		}
	}
	return errors.New("POST timeout")
}
func workflowStreams(m *workflowMeter, opened, active int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opened != opened || m.active != active {
		return errors.New("stream count mismatch")
	}
	return nil
}
func TestWorkflowOracleRejectsIncompleteReceipts(t *testing.T) {
	r := workflowReceipt{Schema: 1, Profile: "browser", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, Driver: "instrumented DOM; not physical keyboard, IME or pointer", UserAgent: "Synthetic oracle fixture", Completed: true, Shutdown: true, ListenerClosed: true, WindowsClosed: 2, Streams: 2, Posts: []int{422, 303, 409, 303, 303, 503, 303, 303, 303}}
	r.Width, r.Height, r.Theme = 1440, 1000, "light"
	for _, id := range workflowIDs {
		r.Steps = append(r.Steps, workflowStep{ID: id, Passed: true})
	}
	if err := verifyWorkflow(r); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*workflowReceipt){
		func(r *workflowReceipt) { r.Completed = false }, func(r *workflowReceipt) { r.Steps = r.Steps[:13] },
		func(r *workflowReceipt) { r.Posts = []int{303} }, func(r *workflowReceipt) { r.BridgeMessages = 1 },
		func(r *workflowReceipt) { r.WindowsClosed = 1 }, func(r *workflowReceipt) { r.ActiveStreams = 1 },
		func(r *workflowReceipt) { r.Streams = 3 }, func(r *workflowReceipt) { r.ListenerClosed = false },
		func(r *workflowReceipt) { r.Failure = "test" }, func(r *workflowReceipt) { r.Shutdown = false },
		func(r *workflowReceipt) { r.UserAgent = "" },
	} {
		copy := r
		mutate(&copy)
		if verifyWorkflow(copy) == nil {
			t.Fatal("accepted mutated receipt")
		}
	}
}
