// Package hookguard keeps fathom's hooks from piling up on a slow or dead
// daemon (docs/design/responsible-tenant.md §11, ADR-047, I9).
//
// Three mechanisms, all cheap enough to run before every hook RPC:
//
//   - One monotonic deadline per hook invocation (HookDeadline): the
//     harness descriptor's TimeoutSec minus max(1 s, 20%). Every RPC the
//     hook makes derives its context from it and sends the milliseconds
//     remaining as X-Fathom-Deadline, so no RPC outlives the hook and the
//     daemon can drop work nobody will read.
//   - A per-URL breaker file shared by every hook process on the machine
//     (BreakerAllows): 2 failures within 10 s — or a 503 carrying
//     Retry-After — open it; backoff doubles from 5 s to 300 s; when it
//     lapses exactly one hook holds a flock lease and probes /healthz
//     while every other hook fails open at once. A good probe keeps the
//     backoff ladder; only a real RPC success resets it.
//   - The kill switch: disabled.json / paused.json under the managed
//     daemon's directory are stat'ed before any RPC to the managed URL,
//     and a hook addressing it returns silently.
//
// Everything here fails open: an unreadable state file is a closed
// breaker, an unwritable one is a no-op. A hook must never fail the
// harness that called it.
package hookguard
