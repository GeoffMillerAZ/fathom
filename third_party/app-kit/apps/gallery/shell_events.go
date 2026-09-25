package gallery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	shellDemoMaxStreamsPerSession = 4
	shellDemoMaxStreams           = 32
	shellDemoMaxEventBytes        = 8 << 10
	shellDemoStreamWriteTimeout   = 5 * time.Second
	shellDemoStreamHeartbeat      = 15 * time.Second
)

// This is a synthetic, read-only transport specimen, not a production account,
// event history or reusable session API. A Datastar-shaped wire event alone does
// not establish compatibility with an actual pinned client or native webview.
type shellEventSnapshot struct {
	Version      string   `json:"version"`
	DisplayName  string   `json:"display_name"`
	TimeZone     string   `json:"time_zone"`
	StartPage    string   `json:"start_page"`
	Digest       bool     `json:"digest"`
	Access       string   `json:"access"`
	Availability string   `json:"availability"`
	Activity     []string `json:"activity"`
}

func shellDemoProjection(state shellDemoState) shellEventSnapshot {
	return shellEventSnapshot{
		Version: strconv.FormatUint(state.Version, 10), DisplayName: state.DisplayName,
		TimeZone: state.TimeZone, StartPage: state.StartPage, Digest: state.Digest,
		Access: state.Access, Availability: state.Availability,
		Activity: append([]string{}, state.Activity...),
	}
}

// Caller holds the shared store mutex. Closed-channel notifications broadcast
// without blocking writers or retaining an event queue for a slow reader.
func shellDemoChangedLocked(session *shellDemoSession) {
	if session.changed != nil {
		close(session.changed)
		session.changed = make(chan struct{})
	}
	if !session.State.SignedIn {
		shellDemoRevokeLocked(session)
	}
}

func shellDemoRevokeLocked(session *shellDemoSession) {
	if session.revoked != nil {
		close(session.revoked)
		session.revoked = nil
	}
}

func (store *shellDemoStore) serveShellEvents(w http.ResponseWriter, r *http.Request) {
	shellDemoHeaders(w, "")
	if !referenceRequestAllowed(r) {
		http.Error(w, "The synthetic stream requires same-origin local requests.", http.StatusForbidden)
		return
	}
	if r.URL.Path != "/shell/events" || r.URL.EscapedPath() != r.URL.Path {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) > 0 || !shellDemoEventCursorValid(r.Header.Values("Last-Event-ID")) {
		http.Error(w, "The stream accepts no query or body and only one canonical event cursor.", http.StatusBadRequest)
		return
	}
	if accept := r.Header.Values("Accept"); len(accept) != 1 || accept[0] != "text/event-stream" {
		http.Error(w, "Explicitly request text/event-stream to open this specimen.", http.StatusNotAcceptable)
		return
	}
	id, ok := shellDemoCookie(r)
	if !ok {
		http.Error(w, "The demo session cookie is ambiguous.", http.StatusBadRequest)
		return
	}

	store.mu.Lock()
	session := store.sessions[id]
	if session == nil || !time.Now().Before(session.Expires) {
		store.mu.Unlock()
		http.Error(w, "Open a demo page to start a live synthetic session first.", http.StatusForbidden)
		return
	}
	if !session.State.SignedIn {
		store.mu.Unlock()
		http.Error(w, "This synthetic session is signed out.", http.StatusUnauthorized)
		return
	}
	if session.streams >= shellDemoMaxStreamsPerSession || store.streams >= shellDemoMaxStreams {
		store.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		http.Error(w, "The bounded synthetic stream capacity is reached.", http.StatusTooManyRequests)
		return
	}
	if session.changed == nil {
		session.changed = make(chan struct{})
	}
	if session.revoked == nil {
		session.revoked = make(chan struct{})
	}
	// Initial observation and subscription are one atomic operation. No commit
	// can fall between reading state and registering for its successor.
	snapshot := shellDemoProjection(session.State)
	changed, revoked, expires := session.changed, session.revoked, session.Expires
	session.streams++
	store.streams++
	store.mu.Unlock()
	defer func() {
		store.mu.Lock()
		session.streams--
		store.streams--
		store.mu.Unlock()
	}()

	controller := http.NewResponseController(w)
	// Do not silently accept a writer that cannot bound blocked network I/O.
	if err := controller.SetWriteDeadline(time.Now().Add(shellDemoStreamWriteTimeout)); err != nil {
		http.Error(w, "This host cannot provide bounded stream writes.", http.StatusInternalServerError)
		return
	}
	defer controller.SetWriteDeadline(time.Time{}) // also clear on early failure
	frame, err := shellDemoEventFrame(&snapshot)
	if err != nil {
		http.Error(w, "The synthetic projection exceeds its stream bounds.", http.StatusInternalServerError)
		return
	}
	// A revocation/expiry that arrived while preparing the first response must
	// not be turned into a fresh snapshot. Already in-flight bytes cannot be
	// recalled; subsequent writes reauthorize and the terminal patch is redacted.
	select {
	case <-r.Context().Done():
		return
	case <-revoked:
		http.Error(w, "This synthetic stream session was revoked.", http.StatusUnauthorized)
		return
	default:
	}
	if !time.Now().Before(expires) {
		http.Error(w, "This synthetic stream session expired.", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-AppKit-Stream-Mode", "snapshot")
	if err := shellDemoWriteEvent(controller, w, frame); err != nil {
		return
	}
	lastVersion := snapshot.Version
	expiry := time.NewTimer(time.Until(expires))
	defer expiry.Stop()
	heartbeat := time.NewTicker(shellDemoStreamHeartbeat)
	defer heartbeat.Stop()
	end := func() {
		// Best effort: the receiver might already be gone or unable to drain.
		redacted, _ := shellDemoEventFrame(nil)
		_ = shellDemoWriteEvent(controller, w, redacted)
	}
	for {
		keepalive := false
		select {
		case <-r.Context().Done():
			return
		case <-revoked:
			end()
			return
		case <-expiry.C:
			end()
			return
		case <-changed:
		case <-heartbeat.C:
			keepalive = true
		}
		store.mu.Lock()
		valid := store.sessions[id] == session && time.Now().Before(session.Expires) && session.State.SignedIn
		if valid {
			snapshot, changed = shellDemoProjection(session.State), session.changed
		}
		store.mu.Unlock()
		select {
		case <-r.Context().Done():
			return
		case <-revoked:
			end()
			return
		default:
		}
		if !valid {
			end()
			return
		}
		if snapshot.Version != lastVersion {
			frame, err := shellDemoEventFrame(&snapshot)
			if err != nil || shellDemoWriteEvent(controller, w, frame) != nil {
				return
			}
			lastVersion = snapshot.Version
		} else if keepalive {
			if shellDemoWriteEvent(controller, w, []byte(": keepalive\n\n")) != nil {
				return
			}
		}
	}
}

func shellDemoEventCursorValid(values []string) bool {
	if len(values) == 0 || len(values) == 1 && values[0] == "" {
		return true
	}
	if len(values) != 1 || len(values[0]) > 20 {
		return false
	}
	v, err := strconv.ParseUint(values[0], 10, 64)
	return err == nil && v > 0 && strconv.FormatUint(v, 10) == values[0]
}

func shellDemoEventFrame(snapshot *shellEventSnapshot) ([]byte, error) {
	if snapshot != nil && (snapshot.Version == "" || !shellDemoEventCursorValid([]string{snapshot.Version})) {
		return nil, errors.New("synthetic projection has an invalid version")
	}
	data, err := json.Marshal(struct {
		Probe *shellEventSnapshot `json:"appkitShellProbe"`
	}{snapshot})
	if err != nil {
		return nil, err
	}
	id := ""
	if snapshot != nil {
		id = "id: " + snapshot.Version + "\n"
	}
	// JSON escapes newlines/control bytes. Never interpolate a label into SSE
	// fields or an executable expression. The only identifier is server-owned.
	frame := []byte("event: datastar-patch-signals\n" + id + "data: signals " + string(data) + "\n\n")
	if len(frame) > shellDemoMaxEventBytes {
		return nil, errors.New("synthetic event exceeds byte limit")
	}
	return frame, nil
}

func shellDemoWriteEvent(controller *http.ResponseController, w http.ResponseWriter, frame []byte) error {
	if len(frame) > shellDemoMaxEventBytes {
		return errors.New("synthetic event exceeds byte limit")
	}
	if err := controller.SetWriteDeadline(time.Now().Add(shellDemoStreamWriteTimeout)); err != nil {
		return fmt.Errorf("set stream write deadline: %w", err)
	}
	n, err := w.Write(frame)
	if err == nil && n != len(frame) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return err
	}
	if err := controller.Flush(); err != nil {
		return err
	}
	return controller.SetWriteDeadline(time.Time{})
}
