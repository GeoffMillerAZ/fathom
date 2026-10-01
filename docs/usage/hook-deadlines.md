### Hook deadlines, the shared breaker and the kill switch

Every hook verb (`session-hook` and its `--event`s, `edit-hook [--pre]`, `prompt-hook`,
`stop-hook`, `session-end-hook`, `pre-compact-hook`, `config audit --hook`,
`doctor --brief`, `post-commit-hook`) runs under `internal/hookguard`
(docs/design/responsible-tenant.md §9, §11; RT-7..RT-10):

- **One deadline per invocation**: the harness descriptor's timeout minus max(1 s, 20%)
  (Claude Code's 5 s hooks get 4 s, 10 s hooks 8 s, Codex's 3 s SessionEnd 2 s). Every
  RPC's context ends at it and carries `X-Fathom-Deadline: <ms left>`. `post-commit-hook`
  is a git hook with no descriptor: its `--timeout` bounds each call. It queues a
  refresh of a service the daemon already indexes (`POST /refresh-request`) and never
  onboards one: in a repository the daemon does not index it prints the `ingest` MCP
  tool call (`{"service", "path"}`) that onboards it once.
- **Retries**: the service enumeration retries once, only on a refused or reset
  connection and only while the deadline has time left; a timeout or 5xx is never retried.
- **Shared breaker**: `$TMPDIR/fathom-hook-health/daemon-health-<sha8(url)>.json`, shared
  by every hook process of the user. Two failures (timeout, 5xx, refused) within 10 s open
  it for 5 s, doubling to 300 s; a 503 with `Retry-After` holds hooks off for that long
  (≤ 60 s) without counting as a failure. A caller's own client timeout counts as a
  failure; only a cancellation before any deadline does not. Updates to the file are
  serialized with `flock`, so concurrent failures all count. When it lapses exactly one
  hook takes a `flock` lease and probes `GET /healthz` (300 ms); every other hook fails
  open at once. A good probe closes the breaker but keeps the backoff; only a real RPC
  success resets it. While it is open hooks serve the identity cache stale where they can
  (up to 24 h old) and otherwise stay silent — the session primer prints nothing rather
  than "not responding". Each invocation that skipped RPCs records one beat per verdict:
  `breaker_open`, `busy` or `deadline_spent`, with the count.
- **Kill switch**: before any RPC a hook stats `~/.fathom/daemon/disabled.json` and
  `paused.json`. Either one silences a hook addressing the managed daemon — the file's
  `addr` (or `url`) field, which every writer records, else `http://127.0.0.1:7575` — with exit 0 and no output; a hook
  addressing any other daemon (a scratch `serve-http`) is unaffected. An empty file counts;
  an `until` in the past does not.

---
