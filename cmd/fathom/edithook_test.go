package main

// edithook_test.go — DG-7 (diagnostics sweep): fetchObligations used to
// collapse every internal failure to a bare ok=false, discarding the
// actual cause. The caller's beat then recorded a degradation with a
// session id and file path but no reason — structurally indistinguishable
// from "no guard names this file." This pins that every failure branch
// now names a distinct, non-empty reason via the function's new error
// return, without touching the hook's silent-to-the-agent contract
// (proven separately by the existing edit-hook integration tests, which
// still see empty stdout on a daemon error).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestFetchObligations_ReturnsReasonOnEachFailureBranch is the DG-7
// contract test: six independent failure injection points, each expected
// to yield ok=false plus a non-empty, distinguishable reason.
func TestFetchObligations_ReturnsReasonOnEachFailureBranch(t *testing.T) {
	rpcEnvelope := func(inner string) string {
		b, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": inner},
			}},
		})
		return string(b)
	}

	tests := []struct {
		name       string
		urlFor     func(t *testing.T) string // returns the baseURL to call fetchObligations with
		wantSubstr string                    // required substring in the returned error
	}{
		{
			name: "bad URL",
			urlFor: func(t *testing.T) string {
				// A control character makes url.Parse (inside
				// http.NewRequest/originHeader) refuse the URL outright —
				// no network I/O is even attempted.
				return "http://\x7f"
			},
			wantSubstr: "build request",
		},
		{
			name: "closed connection",
			urlFor: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				u := srv.URL
				srv.Close() // nothing listens on u anymore: connection refused
				return u
			},
			wantSubstr: "daemon request",
		},
		{
			name: "non-200 status",
			urlFor: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"boom"}`))
				}))
				t.Cleanup(srv.Close)
				return srv.URL
			},
			wantSubstr: "HTTP 500",
		},
		{
			name: "truncated body",
			urlFor: func(t *testing.T) string {
				// Hijack the connection to write a Content-Length that
				// promises more bytes than are actually sent, then close —
				// the client's body reader sees an unexpected EOF short of
				// the promised length.
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("listen: %v", err)
				}
				go func() {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					defer conn.Close()
					// Drain the request so the client's write doesn't block.
					br := bufio.NewReader(conn)
					http.ReadRequest(br)
					fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Length: 1000\r\nContent-Type: application/json\r\n\r\nshort")
				}()
				t.Cleanup(func() { ln.Close() })
				return "http://" + ln.Addr().String()
			},
			wantSubstr: "read response body",
		},
		{
			name: "malformed RPC envelope",
			urlFor: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("not json at all"))
				}))
				t.Cleanup(srv.Close)
				return srv.URL
			},
			wantSubstr: "unwrap RPC result",
		},
		{
			name: "malformed JSON payload",
			urlFor: func(t *testing.T) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(rpcEnvelope("not valid json for the payload")))
				}))
				t.Cleanup(srv.Close)
				return srv.URL
			},
			wantSubstr: "decode obligations payload",
		},
	}

	seenReasons := map[string]string{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := tc.urlFor(t)
			obl, stale, ok, err := fetchObligations(url, "myservice", "a.go", 0, 500*time.Millisecond, semantic.ObligationLayerAll)
			if ok {
				t.Fatalf("ok = true, want false for %s", tc.name)
			}
			if err == nil {
				t.Fatalf("err = nil, want a non-nil reason for %s", tc.name)
			}
			if stale {
				t.Errorf("stale = true on a failure branch, want false")
			}
			if len(obl.Rules) != 0 {
				t.Errorf("obl.Rules non-empty on a failure branch")
			}
			reason := err.Error()
			if reason == "" {
				t.Fatalf("reason is empty for %s", tc.name)
			}
			if !strings.Contains(reason, tc.wantSubstr) {
				t.Errorf("reason = %q, want substring %q", reason, tc.wantSubstr)
			}
			for otherName, otherReason := range seenReasons {
				if otherReason == reason {
					t.Errorf("reason %q for %s is identical to %s's — branches are not distinguishable", reason, tc.name, otherName)
				}
			}
			seenReasons[tc.name] = reason
		})
	}
}
