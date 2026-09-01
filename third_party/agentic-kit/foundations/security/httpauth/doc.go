// Package httpauth provides a minimal bearer-token auth primitive for
// gating an HTTP handler: Bearer(tokenEnvName) builds a Middleware that
// requires a constant-time-matched "Authorization: Bearer <token>" header,
// with the token itself read once from a NAMED environment variable at
// construction time.
//
// # Origin
//
// This is a clean re-implementation of a pattern proven in fathom's module
// host (github.com/geoffmilleraz/fathom, cmd/fathom/servehttp.go,
// modulesAuthMiddleware / GKE-MH-7) — not an import of that code, which
// lives in a separate repository and module. D13: once a second consumer
// needs the same primitive (the kit's hosted expertchat service,
// docs/plans/gke-expertchat.md), the pattern belongs in the kit rather than
// being re-copied a second time.
//
// # Contract
//
//   - The token is resolved from tokenEnvName at Bearer's call time, never
//     accepted as a literal parameter — the same env-var-NAME-indirection
//     fathom uses, so the token never appears in a flag, a config file, or a
//     process's argv.
//   - An empty or unset environment variable is a constructor ERROR, never a
//     silently-disabled gate: Bearer refuses to hand back a middleware that
//     would let every request through unauthenticated when the caller
//     believed auth was configured.
//   - The comparison is constant-time (crypto/subtle.ConstantTimeCompare),
//     so a byte-by-byte mismatch never leaks timing information about how
//     much of the token matched.
//   - The token never appears in a log line, an error message, or a
//     response body — only a boolean authorized/denied is observable from
//     the outside. Bearer's own error messages name the environment
//     variable, never its value.
//   - A missing or wrong Authorization header gets 401 Unauthorized plus a
//     WWW-Authenticate: Bearer response header; it never falls through to
//     the wrapped handler.
//
// # Scope
//
// Bearer gates a whole http.Handler. Scoping the check to one path or one
// mux route (fathom's own use limits it to /rpc/modules, leaving every
// other route unauthenticated) is the CALLER's job via ordinary mux
// composition — e.g. wrap only the sub-handler that needs it, or branch on
// r.URL.Path before delegating to the wrapped handler. Keeping that
// decision out of this package keeps it a minimal, reusable primitive
// rather than a router.
//
// # Concurrency
//
// The Middleware Bearer returns is safe for concurrent use: it closes over
// an immutable token string captured once at construction and holds no
// other mutable state, so it may be shared across goroutines and requests
// exactly like any stateless http.Handler decorator.
//
// # Intended consumers
//
// The kit's hosted expertchat service (kits/expertchat,
// docs/plans/gke-expertchat.md) is the first in-kit consumer once it grows
// a network-reachable serving mode; examples/expertchat-service is
// currently a CLI-only composition example with no listener, so it is not
// wired here.
package httpauth
