# ADR-0008: Embed NATS JetStream in-process as the event/state spine

- **Status:** Superseded by ADR-0021 (optional event profile)
- **Date:** 2026-06-10
- **Decision drivers:** the Tao-of-Datastar loop (ADR-0003) needs a watch/
  fan-out primitive behind every SSE stream; the single-binary story
  (ADR-0005) forbids external broker processes; device sync must later be a
  config change, not a rewrite; persistence and replay should come from the
  spine, not be reinvented per app.

## Context

Every kit app renders idempotent snapshots down one long-lived SSE stream
per tab ([ADR-0003](0003-hypermedia-first-datastar.md)). Something must tell
the renderer *when state changed* — across handlers, across tabs, across
windows, and eventually across devices. That is a pub/sub + watch problem,
and the Loopback Invariant ([ADR-0005](0005-loopback-invariant.md)) means
whatever solves it must live inside the one Go binary.

The facts about embedded NATS, verified 2026-06-10:

- nats-server v2.14.2 released 2026-06-02 (built with Go 1.26.3); the 2.13.x
  line was skipped; 2.12.x remains an LTS line (v2.12.11, 2026-06-09). Client
  nats.go is at v1.52.0. (https://github.com/nats-io/nats-server/releases)
- Embedding is officially supported: `server.NewServer(&server.Options{
  DontListen: true, JetStream: true, StoreDir: dir})`, then
  `nats.Connect("", nats.InProcessServer(ns))`. `DontListen` skips the TCP
  accept loop entirely; the client rides a `net.Pipe` from the server's
  `InProcessConn()` — no sockets exist at all.
  (https://pkg.go.dev/github.com/nats-io/nats-server/v2/server)
- Measured cost (our research measurement, 2026-06-10, macOS arm64,
  Go 1.26.2, nats-server v2.14.2 + nats.go v1.52.0): a minimal embedded
  NATS+JetStream binary is **16 MB stripped** (`-ldflags='-s -w'`; 23 MB
  unstripped) and idles at **~19 MB RSS** with JetStream enabled and an
  in-process client connected. NATS docs corroborate ~11 MB RSS for core
  NATS even at 2M msgs/sec (https://github.com/nats-io/nats.docs/issues/321).
- In-process performance is essentially identical to external NATS in
  benchmarks over millions of messages
  (https://gosuda.org/blog/posts/how-embedded-nats-communicate-with-go-application-z36089af0,
  2025-08-04).
- JetStream KV and Object Store are layers over streams in the same Go
  module — no extra dependency — and KV watchers provide change feeds
  (https://pkg.go.dev/github.com/nats-io/nats.go/jetstream).
- Leaf nodes dial **outbound-only** to a hub (default port 7422), so a
  desktop app can sync through NAT/firewalls with zero inbound rules;
  JetStream domains + mirror/source give store-and-forward
  (https://docs.nats.io/running-a-nats-service/configuration/leafnodes/jetstream_leafnodes).
  But leaf-node JetStream resync after offline periods has open bug reports:
  nats-server #6468 (leaf stream messages not syncing to hub after
  reconnect) and #7530 (leafnode not syncing with remote).

This pairing has a name and a lineage: **GONAD** — Go + NATS + Datastar
(https://gonads.net/). Its center of gravity is Delaney Gillilan, creator of
Datastar *and* Senior Product Strategy Engineer at Synadia, the NATS company
— the two halves of our reactive loop are designed by the same hands. The
flagship community boilerplate, zangster300/northstar (v0.16.0, tagged
2026-06-03, ~260 stars), embeds the NATS server and drives its UI from a
JetStream KV bucket (https://github.com/zangster300/northstar).

Embedding has documented sharp edges, all with open threads as of
2026-06-10: (1) the official docs' embedding example omits
`DontListen`+`InProcessServer`, so naive copies silently fall back to TCP
(gosuda, 2025-08-04); (2) enabling JetStream for a non-default account is a
chicken-and-egg with `server.Options`
(https://github.com/nats-io/nats-server/discussions/6242); (3) JetStream
memory storage cannot be fully disabled in embedded scenarios without
workarounds (https://github.com/nats-io/nats-server/issues/6756);
(4) `DontListen: true` conflicts with cluster/leaf listening configs
(https://github.com/nats-io/nats-server/issues/5174).

## Decision

Embedded NATS JetStream is the default event/state spine of every kit app.
We pin nats-server v2.14.x and nats.go v1.52.x and treat upgrades as
deliberate events.

**The bus interface.** Apps program against `appkit/bus`, a deliberately
small interface — not against NATS types:

```go
package bus

type Bus interface {
    Publish(ctx context.Context, subject string, data []byte) error
    Subscribe(ctx context.Context, subject string, h func(Msg)) (CancelFunc, error)
    KV(ctx context.Context, bucket string) (KV, error)
}

type KV interface {
    Get(ctx context.Context, key string) (Entry, error)
    Put(ctx context.Context, key string, value []byte) (rev uint64, err error)
    Watch(ctx context.Context, keyPattern string) (Watcher, error)
}
```

Two implementations ship: `bus/natsbus` (default — embedded server,
`DontListen: true`, JetStream on, per-OS `StoreDir`, in-process client) and
`bus/chanbus` (channels + in-memory map with watch semantics; no
persistence, no replay, documented as such). Trivial tools opt down to
`chanbus`; everything else gets NATS for 16 MB of binary.

**The canonical loop.** KV-watch → re-render → SSE patch. A command POST
mutates a KV bucket; the per-tab SSE goroutine holds a `Watch` on the keys
that page renders; on change it re-renders the templ snapshot and emits a
Datastar patch via datastar-go. This is the GONAD wiring (northstar;
"Pair SSE with Datastar and NATS", wlvrn, 2025-09-21) and it makes
multi-tab convergence, reconnect self-healing, and job progress all fall
out of one mechanism.

**Subjects and buckets are CUE.** Subject hierarchies and KV bucket schemas
(including retention limits) are defined in CUE per
[ADR-0007](0007-cue-single-source-of-truth.md) and validated at build time.
Convention: subjects are `<app>.<domain>.<event>` and buckets are
`<app>_<domain>` from day one — redundant inside a single embedded server,
load-bearing the moment subjects from many apps meet at a hub in leaf mode.

**Listen mode is one CUE knob.** `bus: mode: "in-process" | "leaf" |
"external"` in `app.cue`. In-process is the default (pure `DontListen`).
External takes a NATS URL — the cloud deployment shape. Leaf configures
outbound remotes to our hub — the device-sync shape. The application code
path is identical in all three; sync becomes a config change. Leaf mode is
**gated**: it does not graduate from experimental until our offline/
reconnect integration suite passes against a real hub, given open resync
bugs #6468 and #7530.

**northstar is a pattern reference, never a dependency.** We copy its
embedding bootstrap and KV-watch wiring into the kit, adapted; we do not
import it. One maintainer's boilerplate is something to learn from, not
live on.

Boundaries: SQLite remains the durable relational truth
([ADR-0009](0009-sqlite-default-store.md)); JetStream KV is reactive and
ephemeral-to-replayable state. Cross-device consistency guarantees are
explicitly out of scope until leaf mode graduates.

## Alternatives considered

- **Bespoke channel bus as the default.** Zero dependencies, zero binary
  cost, idiomatic Go. Lost because it has no persistence, no replay, no
  durable KV with watch semantics, and — decisively — no path to device
  sync: every one of those becomes code we write and maintain ourselves,
  which is the opposite of the kit's purpose. It survives as `chanbus`, the
  documented opt-out for trivial tools, not the default.
- **Redis (+ keyspace notifications).** Mature, fast, well-understood. Lost
  because it is an external process — installing and supervising a daemon on
  every user machine breaks the single-binary invariant (ADR-0005) outright;
  keyspace notifications are fire-and-forget with no replay; and it brings a
  second non-Go runtime into a Go-maximalist stack.
- **SQLite as queue/state (poll tables for changes).** Attractive because
  we already ship SQLite. Lost because polling is latency-vs-CPU tuning
  forever, there is no cross-subscriber watch fan-out primitive, and it
  conflates the durable store with the reactive layer — the division of
  labor in ADR-0009 exists precisely to avoid that.
- **External NATS as a separate process.** All the NATS semantics, none of
  the embedding gotchas. Lost for desktop (process management, install
  footprint) — but it is exactly the `external` listen mode for cloud, so
  this alternative is half-adopted rather than rejected.

Real costs of the winner, accepted with eyes open: a +16 MB binary /
~19 MB RSS floor on every default app; an under-documented embedding API
(the official example is wrong; we read `server.Options` source); JetStream
file-store hygiene on laptops (retention limits and `StoreDir` placement
are *mandatory* fields in the CUE bucket schema, because unbounded streams
will eat a disk); and a meme-tier community brand whose reference material
is thin.

## Consequences

- Easier: real-time fan-out to SSE, multi-window/multi-tab convergence,
  long-running job progress, event replay and audit, and a device-sync
  future that is configuration rather than architecture.
- Harder: storage hygiene and upgrade discipline. Mitigation: CUE bucket
  schemas require explicit limits; nats-server is pinned and upgraded like
  a compiler. Debugging an in-process broker has no socket for the `nats`
  CLI — the kit adds a dev-mode flag that enables a loopback listener
  (mindful of the #5174 listen-config conflicts) so standard tooling works
  during development.
- Risk: leaf-node resync bugs (#6468, #7530) make the sync story
  not-turn-key. Tripwire: both issues closed *and* our offline/reconnect
  suite green against a hub → leaf mode graduates.
- Risk: JetStream memory-storage cannot be fully disabled embedded (#6756);
  we carry the documented workaround. Tripwire: issue closed → delete it.
- Risk: nats-server under gomobile is not an officially supported target.
  Tripwire: mobile phase starts → spike nats-server on iOS/Android before
  promising the same spine there; `chanbus` + remote sync is the fallback.

## Revisit when

- nats-server #6468 and #7530 are resolved and our resync suite passes —
  unlock leaf mode for sync-wanting apps.
- The mobile phase begins (verify in-process NATS under gomobile/WKWebView
  hosts before extending the spine there).
- nats-server ships a 3.x or changes the embedding surface — re-read
  `server.Options` against our bootstrap.
- A kit app emerges where the ~19 MB RSS floor genuinely matters — confirm
  `chanbus` still covers it, or revisit the default.
