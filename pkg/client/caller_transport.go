package client

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// IsDaemonHost reports whether host (host[:port]) is one a fathom daemon
// listens on: a loopback address, localhost, or the host of fathomHTTP
// (FATHOM_HTTP) when set. X-Fathom-Caller goes only there — never to an
// LLM provider or any other outbound host.
func IsDaemonHost(host, fathomHTTP string) bool {
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	h = strings.Trim(h, "[]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		return true
	}
	if fathomHTTP != "" {
		if u, err := url.Parse(fathomHTTP); err == nil && u.Host != "" {
			return strings.EqualFold(u.Host, host) || strings.EqualFold(u.Hostname(), h)
		}
	}
	return false
}

// CallerTransport stamps X-Fathom-Caller on every request to a daemon host
// that does not already carry it (RT-164, §10.4: every client sends it,
// whether its request came from originHeader or was built by hand).
type CallerTransport struct {
	Base   http.RoundTripper // nil: the transport installed before this one
	Origin func() string     // the process's channel identity; may be nil
}

// RoundTrip implements http.RoundTripper; the request is cloned, never
// mutated.
func (t CallerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if r.Header.Get(CallerHeaderName) == "" && IsDaemonHost(r.URL.Host, os.Getenv("FATHOM_HTTP")) {
		origin := ""
		if t.Origin != nil {
			origin = t.Origin()
		}
		r = r.Clone(r.Context())
		r.Header.Set(CallerHeaderName, CallerHeader(origin, ""))
	}
	return base.RoundTrip(r)
}

// InstallCallerTransport wraps http.DefaultTransport — which every
// http.Client with a nil Transport uses — in a CallerTransport. A binary
// calls it once, from main (never a package init: `go test` stays
// unarmed unless a test asks). Idempotent.
func InstallCallerTransport(origin func() string) {
	if _, done := http.DefaultTransport.(CallerTransport); done {
		return
	}
	http.DefaultTransport = CallerTransport{Base: http.DefaultTransport, Origin: origin}
}
