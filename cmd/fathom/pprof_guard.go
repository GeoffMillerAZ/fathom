package main

// pprof_guard.go — pprof and /debug/inflight on the daemon's TCP mux
// (docs/design/responsible-tenant.md §13.5, RT-123). They stay behind the
// loopback Host check, and guardPprof also requires a loopback peer
// (r.RemoteAddr): Host is what the client says, and a hosted daemon drops
// the server-wide Host guard for its bearer gate, so only the peer address
// keeps profiles to this machine. `seconds` is capped at 30, and one
// profile runs at a time: a second concurrent request is refused 429 with
// the reason, never queued behind a 30 s CPU profile. The unix socket is
// deferred (RT-D7) until RT-M3 records /metrics unreachable under load.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// pprofMaxSeconds caps a profile's or trace's `seconds` (§13.5).
const pprofMaxSeconds = 30

// pprofBusy is held while a profile is being written: one at a time.
var pprofBusy atomic.Bool

// guardPprof wraps one pprof handler: loopback Host, capped seconds, one
// profile at a time. The index page is guarded by Host only.
func guardPprof(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reason := debugRefusal(r); reason != "" {
			http.Error(w, "refused: "+reason+" — pprof serves this machine only", http.StatusForbidden)
			return
		}
		if v := r.URL.Query().Get("seconds"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				http.Error(w, fmt.Sprintf("seconds=%q is not a non-negative integer", v), http.StatusBadRequest)
				return
			}
			if n > pprofMaxSeconds {
				q := r.URL.Query()
				q.Set("seconds", strconv.Itoa(pprofMaxSeconds))
				r2 := r.Clone(r.Context())
				r2.URL.RawQuery = q.Encode()
				r = r2
				w.Header().Set("X-Fathom-Pprof-Seconds-Capped", fmt.Sprintf("%d→%d", n, pprofMaxSeconds))
			}
		}
		if r.URL.Path == "/debug/pprof/" {
			next.ServeHTTP(w, r)
			return
		}
		if !pprofBusy.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", strconv.Itoa(pprofMaxSeconds))
			http.Error(w, "refused: another profile is being written — one profile at a time (§13.5); retry when it finishes", http.StatusTooManyRequests)
			return
		}
		defer pprofBusy.Store(false)
		next.ServeHTTP(w, r)
	})
}

// debugRefusal is why a /debug request is refused ("" when it is served):
// the Host must be loopback and so must the peer. A peer address that is
// not host:port (a unix-socket connection) is this machine.
func debugRefusal(r *http.Request) string {
	if !hostIsLoopback(r.Host) {
		return fmt.Sprintf("Host %q is not loopback", r.Host)
	}
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if ip := net.ParseIP(strings.Trim(h, "[]")); ip == nil || !ip.IsLoopback() {
			return fmt.Sprintf("peer %s is not loopback", h)
		}
	}
	return ""
}

// mountPprof mounts the guarded pprof handlers and /debug/inflight.
func mountPprof(mux *http.ServeMux) {
	mux.Handle("/debug/pprof/", guardPprof(http.HandlerFunc(pprof.Index)))
	mux.Handle("/debug/pprof/cmdline", guardPprof(http.HandlerFunc(pprof.Cmdline)))
	mux.Handle("/debug/pprof/profile", guardPprof(http.HandlerFunc(pprof.Profile)))
	mux.Handle("/debug/pprof/symbol", guardPprof(http.HandlerFunc(pprof.Symbol)))
	mux.Handle("/debug/pprof/trace", guardPprof(http.HandlerFunc(pprof.Trace)))
	mux.Handle("/debug/inflight", inflightHandler(tenancy.DefaultInFlight, time.Now))
}

// inflightHandler is GET /debug/inflight: the in-flight table, oldest
// first, with each entry's class, tool, service, origin and age. Loopback
// Host only; it takes no profile slot (it is a mutex-guarded copy).
func inflightHandler(inf *tenancy.InFlight, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reason := debugRefusal(r); reason != "" {
			http.Error(w, "refused: "+reason, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		at := now()
		out := struct {
			At       string         `json:"at"`
			InFlight []inflightView `json:"in_flight"`
		}{At: at.UTC().Format(time.RFC3339Nano), InFlight: []inflightView{}}
		for _, e := range inf.Snapshot() {
			out.InFlight = append(out.InFlight, inflightView{Class: e.Class.String(), Tool: e.Tool, Service: e.Service, Origin: e.Origin, AgeSec: at.Sub(e.Started).Seconds()})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}
