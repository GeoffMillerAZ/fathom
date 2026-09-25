package gallery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestShellEventProjectionAndFrameAreInertAndBounded(t *testing.T) {
	state := defaultShellDemoState(^uint64(0))
	state.DisplayName = "A\nid: forged\n<script>not executable</script>"
	state.FailNextSave = true // private control must not become a client signal
	snapshot := shellDemoProjection(state)
	frame, err := shellDemoEventFrame(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != "18446744073709551615" || bytes.Count(frame, []byte("\n")) != 4 || bytes.Contains(frame, []byte("<script>")) {
		t.Fatalf("version lost precision or label escaped event framing: %q", frame)
	}
	var payload map[string]map[string]json.RawMessage
	start := bytes.Index(frame, []byte("data: signals "))
	if start < 0 || json.Unmarshal(bytes.TrimSpace(frame[start+len("data: signals "):]), &payload) != nil {
		t.Fatal("frame does not contain one JSON projection")
	}
	keys := payload["appkitShellProbe"]
	for _, key := range []string{"version", "display_name", "time_zone", "start_page", "digest", "access", "availability", "activity"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("projection missing %s", key)
		}
	}
	if len(payload) != 1 || len(keys) != 8 {
		t.Fatal("projection exposes fields outside its explicit contract")
	}
	snapshot.Activity[0] = "modified outside store lock"
	if state.Activity[0] == snapshot.Activity[0] {
		t.Fatal("projection aliases mutable session activity")
	}
	snapshot.Activity = []string{strings.Repeat("x", shellDemoMaxEventBytes)}
	if frame, err := shellDemoEventFrame(&snapshot); err == nil || frame != nil {
		t.Fatal("oversized event should fail without a partial frame")
	}
	snapshot.Version = "1\nevent: forged"
	if _, err := shellDemoEventFrame(&snapshot); err == nil {
		t.Fatal("invalid server version escaped event ID validation")
	}
	redacted, err := shellDemoEventFrame(nil)
	if err != nil || string(redacted) != "event: datastar-patch-signals\ndata: signals {\"appkitShellProbe\":null}\n\n" {
		t.Fatalf("terminal frame must erase the projection and contain no identity: %q, %v", redacted, err)
	}
}

func TestShellEventCursorIsCanonicalAndBounded(t *testing.T) {
	for _, values := range [][]string{nil, {""}, {"1"}, {"18446744073709551615"}} {
		if !shellDemoEventCursorValid(values) {
			t.Errorf("valid cursor refused: %q", values)
		}
	}
	for _, values := range [][]string{{"0"}, {"01"}, {"+1"}, {"-1"}, {" 1"}, {"1,2"}, {"1", "1"}, {"", ""}, {"18446744073709551616"}, {strings.Repeat("9", 21)}} {
		if shellDemoEventCursorValid(values) {
			t.Errorf("noncanonical cursor accepted: %q", values)
		}
	}
}

func TestShellEventNotificationsCoalesceWithoutBlockingMutation(t *testing.T) {
	session := &shellDemoSession{State: defaultShellDemoState(1), changed: make(chan struct{}), revoked: make(chan struct{})}
	first := session.changed
	oldAuthority := session.revoked
	for range 1000 {
		session.State.Version++
		shellDemoChangedLocked(session)
	}
	select {
	case <-first:
	default:
		t.Fatal("subscriber missed a committed change")
	}
	select {
	case <-session.changed:
		t.Fatal("latest subscription is already closed, causing a busy loop")
	default:
	}
	if got := shellDemoProjection(session.State).Version; got != "1001" {
		t.Fatalf("resnapshot lost final state: %s", got)
	}
	session.State.SignedIn = false
	shellDemoChangedLocked(session)
	session.State.SignedIn = true
	shellDemoChangedLocked(session)
	select {
	case <-oldAuthority:
	default:
		t.Fatal("quick signout/signin resurrected an old stream")
	}
	if session.revoked != nil {
		t.Fatal("new authority should be allocated only for a new subscription")
	}
}

type shellEventControlledWriter struct {
	*httptest.ResponseRecorder
	deadlineCalls int
	firstDeadline func()
	deadlineErr   error
	writeErr      error
	flushErr      error
	shortWrite    bool
	deadlines     []time.Time
}

func (w *shellEventControlledWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlineCalls++
	w.deadlines = append(w.deadlines, deadline)
	if w.deadlineCalls == 1 && w.firstDeadline != nil {
		w.firstDeadline()
	}
	return w.deadlineErr
}
func (w *shellEventControlledWriter) Write(body []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if w.shortWrite {
		return len(body) - 1, nil
	}
	return w.ResponseRecorder.Write(body)
}
func (w *shellEventControlledWriter) FlushError() error { return w.flushErr }

func shellEventStoreRequest() (*shellDemoStore, *shellDemoSession, *http.Request) {
	session := &shellDemoSession{State: defaultShellDemoState(1), CSRF: "not-for-the-stream", Expires: time.Now().Add(time.Hour)}
	store := &shellDemoStore{sessions: map[string]*shellDemoSession{"synthetic": session}}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/shell/events", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Accept", "text/event-stream")
	r.AddCookie(&http.Cookie{Name: shellDemoCookieName, Value: "synthetic"})
	return store, session, r
}

func TestShellEventsRequireBoundedWriterAndReleaseAdmission(t *testing.T) {
	store, session, r := shellEventStoreRequest()
	expires := session.Expires
	w := httptest.NewRecorder() // deliberately lacks deadline support
	store.serveShellEvents(w, r)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "appkitShellProbe") || store.streams != 0 || session.streams != 0 || !session.Expires.Equal(expires) {
		t.Fatal("unsupported host must fail without leaking/admitting/extending the stream")
	}
}

func TestShellEventsInitialAdmissionRechecksRevocationAndCancellation(t *testing.T) {
	for _, condition := range []string{"signout-signin", "request-cancel"} {
		t.Run(condition, func(t *testing.T) {
			store, session, r := shellEventStoreRequest()
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			r = r.WithContext(ctx)
			w := &shellEventControlledWriter{ResponseRecorder: httptest.NewRecorder()}
			w.firstDeadline = func() {
				if condition == "request-cancel" {
					cancel()
					return
				}
				store.mu.Lock()
				session.State.SignedIn = false
				shellDemoChangedLocked(session)
				session.State.SignedIn = true
				shellDemoChangedLocked(session)
				store.mu.Unlock()
			}
			store.serveShellEvents(w, r)
			if strings.Contains(w.Body.String(), "appkitShellProbe") || store.streams != 0 || session.streams != 0 {
				t.Fatal("pre-write cancellation/revocation admitted a full initial projection or leaked a slot")
			}
		})
	}
}

func TestShellEventWriteBoundaries(t *testing.T) {
	sentinel := errors.New("synthetic transport failure")
	for _, boundary := range []string{"deadline", "write", "short-write", "flush", "success"} {
		t.Run(boundary, func(t *testing.T) {
			w := &shellEventControlledWriter{ResponseRecorder: httptest.NewRecorder()}
			switch boundary {
			case "deadline":
				w.deadlineErr = sentinel
			case "write":
				w.writeErr = sentinel
			case "short-write":
				w.shortWrite = true
			case "flush":
				w.flushErr = sentinel
			}
			before := time.Now()
			err := shellDemoWriteEvent(http.NewResponseController(w), w, []byte(": keepalive\n\n"))
			if boundary == "success" {
				if err != nil || len(w.deadlines) != 2 || !w.deadlines[1].IsZero() {
					t.Fatalf("successful frame did not clear its write deadline: %v", err)
				}
			} else if boundary == "short-write" {
				if !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("short write lost failure: %v", err)
				}
			} else if !errors.Is(err, sentinel) {
				t.Fatalf("%s failure lost: %v", boundary, err)
			}
			if len(w.deadlines) == 0 || w.deadlines[0].Before(before) || w.deadlines[0].After(time.Now().Add(shellDemoStreamWriteTimeout)) {
				t.Fatal("write does not establish a bounded deadline")
			}
			if boundary == "deadline" && w.Body.Len() != 0 {
				t.Fatal("unsupported deadline still wrote a frame")
			}
		})
	}
}

func TestShellEventsCapacityExpiryAndBodyRefuseWithoutAllocation(t *testing.T) {
	for _, boundary := range []string{"session-cap", "global-cap", "expired", "unknown-length-body"} {
		t.Run(boundary, func(t *testing.T) {
			store, session, r := shellEventStoreRequest()
			want := http.StatusTooManyRequests
			switch boundary {
			case "session-cap":
				session.streams = shellDemoMaxStreamsPerSession
			case "global-cap":
				store.streams = shellDemoMaxStreams
			case "expired":
				session.Expires = time.Now().Add(-time.Second)
				want = http.StatusForbidden
			case "unknown-length-body":
				r.Body = io.NopCloser(strings.NewReader("body with no known length"))
				r.ContentLength = -1 // HTTP/2 need not use Transfer-Encoding.
				want = http.StatusBadRequest
			}
			beforeSession, beforeGlobal, expires := session.streams, store.streams, session.Expires
			w := httptest.NewRecorder()
			store.serveShellEvents(w, r)
			if w.Code != want || session.streams != beforeSession || store.streams != beforeGlobal || session.changed != nil || session.revoked != nil || !session.Expires.Equal(expires) {
				t.Fatalf("refused %s allocated/refreshed state or returned wrong status %d", boundary, w.Code)
			}
		})
	}
}

type shellEventBlockedWriter struct {
	*shellEventControlledWriter
	entered chan struct{}
	release chan struct{}
}

func (w *shellEventBlockedWriter) Write(body []byte) (int, error) {
	close(w.entered)
	<-w.release
	return w.ResponseRecorder.Write(body)
}

func TestShellEventsBlockedWriteDoesNotHoldMutationLock(t *testing.T) {
	store, session, r := shellEventStoreRequest()
	ctx, cancel := context.WithCancel(r.Context())
	r = r.WithContext(ctx)
	w := &shellEventBlockedWriter{
		shellEventControlledWriter: &shellEventControlledWriter{ResponseRecorder: httptest.NewRecorder()},
		entered:                    make(chan struct{}), release: make(chan struct{}),
	}
	done := make(chan struct{})
	go func() { defer close(done); store.serveShellEvents(w, r) }()
	defer func() {
		cancel() // only one frame may reach this deliberately stalled writer
		close(w.release)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("stream did not release after the controlled write was unblocked")
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		if session.streams != 0 || store.streams != 0 {
			t.Error("canceled blocked writer leaked admission")
		}
	}()
	select {
	case <-w.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("stream never reached its initial write")
	}
	values := url.Values{"action": {"save-profile"}, "csrf": {"not-for-the-stream"}, "version": {"1"}, "display_name": {"Saved while observer was stalled"}}
	post := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/shell/action", strings.NewReader(values.Encode()))
	post.RemoteAddr = "127.0.0.1:12345"
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", "http://127.0.0.1:8765")
	post.AddCookie(&http.Cookie{Name: shellDemoCookieName, Value: "synthetic"})
	accepted := make(chan int, 1)
	go func() {
		response := httptest.NewRecorder()
		store.serveHTTP(response, post)
		accepted <- response.Code
	}()
	select {
	case status := <-accepted:
		if status != http.StatusSeeOther {
			t.Fatalf("authoritative mutation failed while reader stalled: %d", status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stalled stream prevented the independent POST from completing")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if session.State.DisplayName != values.Get("display_name") || session.State.Version != 2 {
		t.Fatal("completed POST did not commit the independently expected snapshot")
	}
}
