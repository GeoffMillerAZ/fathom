package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	launchPath  = "/auth/launch"
	launchTTL   = 120 * time.Second
	launchLimit = 8
)

// bootstrap is a process-local guard for the synthetic spike, not a kit API.
// Only the host can mint launch URLs; the HTTP surface never mints credentials.
type bootstrap struct {
	host       string
	origin     string
	cookieName string
	session    string
	next       http.Handler
	now        func() time.Time
	mu         sync.Mutex
	launches   map[[sha256.Size]byte]time.Time
}

func newBootstrap(host string, next http.Handler) (*bootstrap, error) {
	ip, portText, ok := strings.Cut(host, ":")
	port, err := strconv.Atoi(portText)
	if !ok || ip != "127.0.0.1" || err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portText {
		return nil, errors.New("bootstrap requires canonical 127.0.0.1:<port>")
	}
	if next == nil {
		return nil, errors.New("bootstrap requires an application handler")
	}
	session, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &bootstrap{
		host: host, origin: "http://" + host, cookieName: "appkit_spike_" + portText,
		session: session, next: next, now: time.Now,
		launches: make(map[[sha256.Size]byte]time.Time),
	}, nil
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("cannot generate bootstrap credential")
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (b *bootstrap) LaunchURL() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	for digest, expires := range b.launches {
		if !now.Before(expires) {
			delete(b.launches, digest)
		}
	}
	if len(b.launches) >= launchLimit {
		return "", errors.New("too many outstanding launch credentials")
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	b.launches[sha256.Sum256([]byte(token))] = now.Add(launchTTL)
	return b.origin + launchPath + "?token=" + token, nil
}

func (b *bootstrap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	deny := func() { http.Error(w, "Forbidden", http.StatusForbidden) }
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !peer.Addr().Is4() || !peer.Addr().IsLoopback() || peer.Port() == 0 || r.Host != b.host {
		deny()
		return
	}
	if r.URL == nil || r.URL.IsAbs() || r.URL.Host != "" || r.URL.User != nil || r.URL.Fragment != "" {
		deny()
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
		deny()
		return
	}
	origin, present, valid := singleHeader(r, "Origin")
	if !valid || (present && origin != b.origin) || (r.Method == http.MethodPost && !present) || !validFetchMetadata(r) {
		deny()
		return
	}
	cookie, count, valid := b.sessionCookie(r)
	if !valid || count > 1 {
		deny()
		return
	}
	if r.URL.Path == "/healthz" {
		if r.Method != http.MethodGet || r.URL.RawPath != "" || r.URL.RawQuery != "" || r.URL.ForceQuery || !emptyBody(r) {
			deny()
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
		return
	}
	if r.URL.Path == launchPath {
		token, ok := strings.CutPrefix(r.URL.RawQuery, "token=")
		if r.Method != http.MethodGet || r.URL.RawPath != "" || r.URL.ForceQuery || !emptyBody(r) || !ok || len(token) != 43 {
			deny()
			return
		}
		raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
		if err != nil || len(raw) != 32 {
			deny()
			return
		}
		digest := sha256.Sum256([]byte(token))
		b.mu.Lock()
		expires, found := b.launches[digest]
		accepted := found && b.now().Before(expires)
		delete(b.launches, digest)
		b.mu.Unlock()
		if !accepted {
			deny()
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: b.cookieName, Value: b.session, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/shell/profile", http.StatusSeeOther)
		return
	}
	if count != 1 || subtle.ConstantTimeCompare([]byte(cookie), []byte(b.session)) != 1 {
		deny()
		return
	}
	// Preserve the original writer so flushing, deadlines and disconnects reach
	// the real transport without a capability-losing middleware wrapper.
	b.next.ServeHTTP(w, r)
}

func emptyBody(r *http.Request) bool {
	return r.ContentLength == 0 && len(r.TransferEncoding) == 0 && (r.Body == nil || r.Body == http.NoBody)
}

func singleHeader(r *http.Request, name string) (value string, present, valid bool) {
	values, present := r.Header[http.CanonicalHeaderKey(name)]
	if !present {
		return "", false, true
	}
	if len(values) != 1 || values[0] == "" {
		return "", true, false
	}
	return values[0], true, true
}

func validFetchMetadata(r *http.Request) bool {
	// Missing metadata is supported for native HTTP clients. When supplied,
	// every value must be unambiguous; same-site is not same-origin on loopback.
	allowed := map[string]string{
		"Sec-Fetch-Site": "same-origin none",
		"Sec-Fetch-Mode": "cors navigate no-cors same-origin",
		"Sec-Fetch-Dest": "audio audioworklet document embed empty font frame iframe image manifest object paintworklet report script serviceworker sharedworker style track video worker xslt fencedframe webidentity dictionary",
		"Sec-Fetch-User": "?1",
	}
	for name, choices := range allowed {
		value, present, valid := singleHeader(r, name)
		if !valid || (present && !strings.Contains(" "+choices+" ", " "+value+" ")) || (present && strings.ContainsAny(value, " \t\r\n,")) {
			return false
		}
	}
	if r.Header.Get("Sec-Fetch-User") != "" && r.Header.Get("Sec-Fetch-Mode") != "navigate" {
		return false
	}
	return true
}

func (b *bootstrap) sessionCookie(r *http.Request) (value string, count int, valid bool) {
	for _, header := range r.Header.Values("Cookie") {
		cookies, err := http.ParseCookie(header)
		if err != nil {
			return "", 0, false
		}
		for _, cookie := range cookies {
			if cookie.Name == b.cookieName {
				value = cookie.Value
				count++
			}
		}
	}
	return value, count, true
}
