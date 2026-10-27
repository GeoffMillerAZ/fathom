package main

// admission_peer_test.go — ADR-083: the Host header is the client's to
// choose, so requireLoopbackHost reads the peer address too.

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A LAN peer that sends Host: 127.0.0.1:7591 is refused with 403 and the
// named reason on every route, a zoned link-local one included (the form
// Go's TCPAddr.String gives, which net.ParseIP does not parse); loopback
// peers (and a peer address that is not host:port, a unix socket) are
// served.
func TestDaemonServerRefusesNonLoopbackPeer(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	s := newDaemonServer(inner, daemonServerOpts{})
	serve := func(peer, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.Host = "127.0.0.1:7591" // the spoofed Host: loopback, as the guard wants
		r.RemoteAddr = peer
		w := httptest.NewRecorder()
		s.Handler.ServeHTTP(w, r)
		return w
	}
	for _, peer := range []string{"192.168.1.20:51515", "[fe80::1]:51515", "[fe80::1%en0]:51515", "10.0.0.7:7591"} {
		host, _, _ := net.SplitHostPort(peer)
		for _, path := range []string{"/rpc", "/rpc/agent", "/", "/healthz"} {
			w := serve(peer, path)
			if w.Code != http.StatusForbidden {
				t.Fatalf("peer %s with a spoofed loopback Host on %s: %d, want 403", peer, path, w.Code)
			}
			if body := w.Body.String(); !strings.Contains(body, "peer "+host+" is not loopback") {
				t.Fatalf("the refusal must name the peer: %q", body)
			}
		}
	}
	for _, peer := range []string{"127.0.0.1:51515", "[::1]:51515", "127.0.0.2:1", "[::ffff:127.0.0.1]:51515", "@", ""} {
		if w := serve(peer, "/rpc"); w.Code != http.StatusOK {
			t.Fatalf("loopback peer %q refused: %d %s", peer, w.Code, w.Body.String())
		}
	}
}
