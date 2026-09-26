package main

import (
	"context"
	"io"
	"net/http"
	"strings"

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
	return req, nil
}

// markHookOrigin marks a daemon request's ctx as a hook's when its declared
// origin names a hook (RT-146): a whole-tree client.Sources read under it
// never waits for a slot and is skipped by name instead. Hooks declare
// "<kind>-hook"; the session-start primer declares "prime".
func markHookOrigin(ctx context.Context, origin string) context.Context {
	if strings.HasSuffix(origin, "-hook") || origin == "prime" {
		return client.WithHookOrigin(ctx)
	}
	return ctx
}

// rpcSourcesScope makes one /rpc request a whole-tree scope (RT-146): a
// client.Sources slot taken while serving it is held until done (after the
// response is written, since the tree lives that long), and a client
// disconnect ends a wait for a slot and the read itself — the /rpc ctx is
// deliberately detached from the connection so an ingest survives one, and
// has no deadline of its own.
func rpcSourcesScope(ctx context.Context, r *http.Request) (context.Context, func()) {
	return client.WithSourcesScope(ctx, r.Context().Done())
}
