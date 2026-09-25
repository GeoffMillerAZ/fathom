package gallery

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	shellDemoCookieName  = "appkit_shell_demo"
	shellDemoMaxSessions = 128
	shellDemoSessionTTL  = 30 * time.Minute
	shellDemoMaxBody     = 8 << 10
)

type shellDemoSession struct {
	State              shellDemoState
	CSRF               string
	Expires            time.Time
	Notice, NoticeRole string
	// Only the explicitly opened transport specimen uses these notifications.
	changed, revoked chan struct{}
	streams          int
}

// Each Gallery handler has its own bounded, memory-only synthetic sessions.
// The fixed lifetime is not extended by reads, and no real identity is stored.
type shellDemoStore struct {
	mu       sync.Mutex
	sessions map[string]*shellDemoSession
	streams  int
}

func registerShellDemo(mux *http.ServeMux) {
	store := &shellDemoStore{sessions: make(map[string]*shellDemoSession)}
	mux.HandleFunc("/shell", store.serveHTTP)
	mux.HandleFunc("/shell/{page}", store.serveHTTP)
	mux.HandleFunc("/shell/action", store.serveHTTP)
	mux.HandleFunc("/shell/events", store.serveShellEvents)
	// Keep unknown descendants inside the same response-security boundary.
	mux.HandleFunc("/shell/", store.serveHTTP)
}

func (store *shellDemoStore) serveHTTP(w http.ResponseWriter, r *http.Request) {
	nonce, err := shellDemoRandom()
	if err != nil {
		shellDemoHeaders(w, "")
		http.Error(w, "The synthetic demo could not initialize securely.", http.StatusInternalServerError)
		return
	}
	shellDemoHeaders(w, nonce)
	if !referenceRequestAllowed(r) {
		http.Error(w, "The synthetic demo is available only to same-origin local requests.", http.StatusForbidden)
		return
	}
	page := "home"
	isAction := r.URL.Path == "/shell/action"
	if r.URL.Path != "/shell" && !isAction {
		page = strings.TrimPrefix(r.URL.Path, "/shell/")
		if !shellDemoPage(page) || !strings.HasPrefix(r.URL.Path, "/shell/") {
			http.NotFound(w, r)
			return
		}
	}
	if r.URL.EscapedPath() != r.URL.Path {
		http.NotFound(w, r)
		return
	}
	method := http.MethodGet
	if isAction {
		method = http.MethodPost
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	var form url.Values
	theme := ""
	if isAction {
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			http.Error(w, "The action endpoint does not accept query parameters.", http.StatusBadRequest)
			return
		}
		var status int
		form, status = shellDemoForm(w, r)
		if status != 0 {
			http.Error(w, "The submitted demo form is malformed or unsupported.", status)
			return
		}
	} else {
		query, ok := shellDemoValues(r.URL.RawQuery)
		if !ok || len(query) > 1 || (len(query) == 1 && !query.Has("theme")) || (query.Has("theme") && query.Get("theme") != "light" && query.Get("theme") != "dark") {
			http.Error(w, "Use only an optional light or dark theme query.", http.StatusBadRequest)
			return
		}
		theme = query.Get("theme")
	}
	cookieID, cookieOK := shellDemoCookie(r)
	if !cookieOK {
		http.Error(w, "The demo session cookie is ambiguous.", http.StatusBadRequest)
		return
	}

	store.mu.Lock()
	now := time.Now()
	for id, session := range store.sessions {
		if !now.Before(session.Expires) {
			shellDemoRevokeLocked(session)
			delete(store.sessions, id)
		}
	}
	session := store.sessions[cookieID]
	if session == nil {
		if isAction {
			store.mu.Unlock()
			http.Error(w, "Open the demo page to start a session before submitting.", http.StatusForbidden)
			return
		}
		if len(store.sessions) >= shellDemoMaxSessions {
			store.mu.Unlock()
			http.Error(w, "The synthetic demo session limit is reached. Retry after a session expires.", http.StatusServiceUnavailable)
			return
		}
		id, idErr := shellDemoRandom()
		csrf, csrfErr := shellDemoRandom()
		if idErr != nil || csrfErr != nil {
			store.mu.Unlock()
			http.Error(w, "The synthetic demo could not initialize securely.", http.StatusInternalServerError)
			return
		}
		// Never accept a caller-selected identifier as a new session ID.
		if _, exists := store.sessions[id]; exists {
			store.mu.Unlock()
			http.Error(w, "The synthetic demo could not initialize securely.", http.StatusInternalServerError)
			return
		}
		session = &shellDemoSession{State: defaultShellDemoState(1), CSRF: csrf, Expires: now.Add(shellDemoSessionTTL)}
		store.sessions[id] = session
		http.SetCookie(w, &http.Cookie{Name: shellDemoCookieName, Value: id, Path: "/shell", HttpOnly: true,
			SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: int(shellDemoSessionTTL.Seconds()), Expires: session.Expires})
	}
	if isAction {
		if subtle.ConstantTimeCompare([]byte(form.Get("csrf")), []byte(session.CSRF)) != 1 {
			store.mu.Unlock()
			http.Error(w, "The demo form token is invalid. Reload the page before submitting.", http.StatusForbidden)
			return
		}
		beforeVersion := session.State.Version
		state, out := shellDemoApply(session.State, form)
		session.State = state
		if state.Version != beforeVersion {
			shellDemoChangedLocked(session)
		}
		if out.Status == http.StatusSeeOther {
			session.Notice, session.NoticeRole = out.Notice, out.NoticeRole
			store.mu.Unlock()
			http.Redirect(w, r, shellDemoPath(out.Page), http.StatusSeeOther)
			return
		}
		view := shellDemoView{Page: out.Page, State: copyShellDemoState(session.State), CSRF: session.CSRF,
			Nonce: nonce, Fields: out.Fields, Errors: out.Errors, Notice: out.Notice, NoticeRole: out.NoticeRole, Status: out.Status}
		store.mu.Unlock()
		writeShellDemo(w, view)
		return
	}
	out := shellDemoGET(session.State, page)
	if out.Notice == "" {
		out.Notice, out.NoticeRole = session.Notice, session.NoticeRole
	}
	session.Notice, session.NoticeRole = "", ""
	view := shellDemoView{Page: page, State: copyShellDemoState(session.State), CSRF: session.CSRF,
		Nonce: nonce, Theme: theme, Notice: out.Notice, NoticeRole: out.NoticeRole, Status: out.Status}
	store.mu.Unlock()
	writeShellDemo(w, view)
}

func shellDemoHeaders(w http.ResponseWriter, nonce string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	// Same-origin navigations retain the browser provenance that the strict Origin
	// guard requires for native form POSTs; cross-origin destinations receive none.
	w.Header().Set("Referrer-Policy", "same-origin")
	script := "'none'"
	if nonce != "" {
		script = "'nonce-" + nonce + "' 'self'"
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src "+script+"; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
}

func writeShellDemo(w http.ResponseWriter, view shellDemoView) {
	body, err := renderShellDemo(view)
	if err != nil {
		http.Error(w, "The synthetic demo page could not be rendered.", http.StatusInternalServerError)
		return
	}
	if view.Page == "activity" && view.State.SignedIn {
		w.Header().Set("Content-Security-Policy", w.Header().Get("Content-Security-Policy")+"; connect-src 'self'")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(view.Status)
	_, _ = w.Write(body)
}

func shellDemoRandom() (string, error) {
	var body [32]byte
	if _, err := rand.Read(body[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(body[:]), nil
}

func shellDemoCookie(r *http.Request) (string, bool) {
	value := ""
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == shellDemoCookieName {
			value = cookie.Value
			count++
		}
	}
	return value, count <= 1
}

func shellDemoValues(raw string) (url.Values, bool) {
	if len(raw) > shellDemoMaxBody {
		return nil, false
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, false
	}
	for key, list := range values {
		if key == "" || len(list) != 1 || !utf8.ValidString(key) || !utf8.ValidString(list[0]) || strings.ContainsRune(key, 0) || strings.ContainsRune(list[0], 0) {
			return nil, false
		}
	}
	return values, true
}

func shellDemoForm(w http.ResponseWriter, r *http.Request) (url.Values, int) {
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		return nil, http.StatusUnsupportedMediaType
	}
	mediaType, params, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, http.StatusUnsupportedMediaType
	}
	for key, value := range params {
		if key != "charset" || !strings.EqualFold(value, "utf-8") {
			return nil, http.StatusUnsupportedMediaType
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, shellDemoMaxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, http.StatusRequestEntityTooLarge
		}
		return nil, http.StatusBadRequest
	}
	form, ok := shellDemoValues(string(body))
	if !ok {
		return nil, http.StatusBadRequest
	}
	allowed := map[string]bool{"csrf": true, "action": true, "return_to": true}
	switch form.Get("action") {
	case "save-profile":
		allowed["version"], allowed["display_name"] = true, true
	case "save-settings":
		allowed["version"], allowed["timezone"], allowed["start_page"], allowed["digest"] = true, true, true, true
	case "fixture":
		allowed["access"], allowed["availability"], allowed["fail_next_save"] = true, true, true
		if form.Has("access") && form.Get("access") != "editor" && form.Get("access") != "viewer" {
			return nil, http.StatusBadRequest
		}
		if form.Has("availability") && form.Get("availability") != "ready" && form.Get("availability") != "loading" && form.Get("availability") != "error" {
			return nil, http.StatusBadRequest
		}
		if form.Has("fail_next_save") && form.Get("fail_next_save") != "on" {
			return nil, http.StatusBadRequest
		}
	case "retry-load", "signout", "signin", "reset-demo":
	default:
		return nil, http.StatusBadRequest
	}
	for key := range form {
		if !allowed[key] {
			return nil, http.StatusBadRequest
		}
	}
	if page := form.Get("return_to"); page != "" && !shellDemoPage(page) {
		return nil, http.StatusBadRequest
	}
	if allowed["version"] {
		version := form.Get("version")
		parsed, err := strconv.ParseUint(version, 10, 64)
		if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != version {
			return nil, http.StatusBadRequest
		}
	}
	return form, 0
}
