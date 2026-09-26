package main

import (
	"io"
	"net/http"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// originHeaderName carries the caller's channel identity to the daemon so
// the call log separates plumbing from agent tool choice (GI-5/GI-6).
const originHeaderName = "X-Fathom-Origin"

// rpcOrigin is this process's channel identity, set once by the cmd
// entrypoint ("edit-hook", "session-hook", "stop-hook", "verify-cli").
// Empty means unattributed (direct CLI use). One command per process, so a
// package var is safe.
var rpcOrigin string

// originHeader builds the standard POST /rpc request, stamping the
// process's origin header. Every hook/CLI HTTP call goes through it.
func originHeader(baseURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/rpc", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	// RT-164 (§10.4): who is calling, and whether it is a test — a test
	// that execs this binary inherits FATHOM_TEST=1 and is marked too.
	req.Header.Set(client.CallerHeaderName, client.CallerHeader(rpcOrigin, ""))
	return req, nil
}
