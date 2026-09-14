## graphify `--watch` against VB-17's freshness probe (VB-18, free half)

### Verdict first

graphify answered **4 of 5** post-edit questions correctly through its own
query surface while running `graphify watch .` on the same fixture and the
same five scripted edits VB-17's `FreshnessProbe` runs against fathom. VB-18's
kill clause is "if graphify answers four of five post-edit questions,
freshness leaves positioning" — **k=4/5, the clause fires: freshness leaves
positioning.** The one miss was not a watch-latency failure — watch caught up
on every edit it could detect (3.5-3.9s each) — it was a structural gap: a
literal return value has no field in graphify's tree-sitter graph, so no
question about it is answerable regardless of how fresh the graph is, and the
edit that changed only a literal never triggered a rebuild at all (confirmed
separately below, not by the 120s cap alone).

This is the free half only. The paid five-arm FRESH task (SB-51) against
`vs-pilot-4` is still running elsewhere; VB-18's box stays unchecked until
both halves land.

### Setup

- graphify **0.9.61** (`uv tool list`: `graphifyy v0.9.61` providing `graphify`
  and `graphify-mcp`), installed at `/Users/geoff/.local/bin/graphify`.
- Fixture: `internal/connector/testdata/fauxtown/ledger-service` (the same
  fixture `FreshnessProbe` stages), copied to a scratch dir outside the repo
  with a bare `go.mod` added (`module ledger`).
- Initial build: `graphify update . --no-cluster` (the bench arm command
  from `docs/bench/competitors.md`) — 11 nodes, 17 edges, no LLM.
- Then `graphify watch .`, left running for the whole edit sequence, killed
  after. **First run failed**: `error: watchdog not installed. Run: pip
  install watchdog` — `graphify watch` needs the `watchdog` Python package,
  which the base `uv tool install` of `graphifyy` does not pull in. Since a
  uv tool venv is isolated, a system/user `pip install watchdog` would not
  have reached it; it had to be installed directly into the tool's own venv
  (`uv pip install --python
  /Users/geoff/.local/share/uv/tools/graphifyy/bin/python watchdog`). This is
  a first-run friction point for anyone following the docs' `graphify
  --watch` invocation literally.
- `OPENAI_API_KEY`, `ANTHROPIC_DEFAULT_KEY`, `AZURE_OPENAI_KEY`,
  `AZURE_OPENAI_ENDPOINT`, `GEMINI_API_KEY`, `GOOGLE_API_KEY` were stripped
  from the watch process's environment, matching the bench arm's disclosed
  no-LLM/headless config and spending no LLM budget. `graphify watch` (unlike
  `update`, no `--no-cluster` flag exists for it) still ran its own
  non-LLM structural community detection on every rebuild — confirmed by
  `graph.json`'s `community` fields being bare integers (0-3) and by
  `watch.log` repeatedly saying "Run `graphify label` to refresh names with
  the LLM" instead of ever calling one. No LLM call was made at any point in
  this run.
- Query surface used (graphify's own CLI, not its files read directly):
  `graphify explain "<name>"` (existence + defining file) and `graphify
  affected "<name>" --relation calls` (caller list, reverse traversal).
  `graphify query "<question>"` was tried alongside `explain`/`affected` for
  two edits as a cross-check; same answer, more BFS noise.
- Reindex latency measured by polling `graphify-out/graph.json`'s mtime
  (nanosecond resolution) from the moment the edit was written to disk, capped
  at 120s per VB-18's instructions.

### Per-edit results

| Edit | Question | Want | graphify surface | Raw answer | Correct | Watch latency |
|---|---|---|---|---|---|---|
| rename | what does find_symbol return for checkEntry (validateEntry, renamed)? | `ledger.go:GoFunc` | `graphify explain "checkEntry"` | `Source: ledger.go L37` | **yes** | 3916 ms |
| add_param | how many callers does applyEntry have now (a dryRun param plus a new caller, PostDryRun)? | `2` | `graphify affected "applyEntry" --relation calls` | `Post() [calls] ledger.go:L25` + `PostDryRun() [calls] ledger.go:L61` = 2 | **yes** | 3945 ms |
| delete | does computeBalance still exist? | `<missing>` | `graphify explain "computeBalance"` | `No node matching 'computeBalance' found.` | **yes** | 3525 ms |
| move | which file defines Balance now? | `query.go:GoFunc` | `graphify explain "Balance"` | `Source: query.go L7` | **yes** | 3737 ms |
| change_literal | what value does Balance's non-empty branch return now? | `7` | `graphify explain "Balance"` / `graphify query "…"` | node shown, no literal-value field anywhere in the response; graph never rebuilt for this edit | **no** | never rebuilt (120s cap hit) |

**k = 4/5.**

### The change_literal miss, in full

Two independent things are true about edit 5, and both matter:

1. **graphify's graph.json has no field for a return literal.** Its nodes
   carry `id`, `label`, `file_type`, `source_file`, `source_location`,
   `community` — no source snippet, no value. `graphify explain` and
   `graphify query` only ever surface structure (which file, which
   neighbors, which calls); there is no CLI verb that would let an agent
   recover "7" from the graph itself. This is the same class of gap
   `internal/indexbench/freshness.go`'s package doc discloses for fathom's
   own store (no field for a const/var's literal value either) — but
   fathom's client falls back to a live source read (`Sources()`) for that
   one question, so it answers correctly regardless. graphify's CLI surface
   has no equivalent fallback: an agent using only `explain`/`query`/
   `affected` cannot answer this question after the edit, before it, or ever.
2. **Separately, the edit never triggered a rebuild.** `graphify-out/graph.json`'s
   mtime was unchanged after 120s (`poll_mtime.py` reported `timeout
   120179`), and `watch.log` gained no new line for this edit (compare: every
   one of the four structural edits logged a `community set changed since
   labeling` line; this edit logged nothing). This was not the watch loop
   being dead — appending a trivial new function (`diagWatchIsAlive`) to the
   same file immediately after triggered a rebuild in 3315 ms, logged as
   usual. So graphify's watch appears to diff on AST/graph structure only:
   editing `return 0, nil` to `return 7, nil` changes no node and no edge, so
   nothing fires. A literal-only edit is invisible to it twice over — no
   field to hold the value, and no trigger to notice the file changed at all.

### fathom's own probe, side by side

`go test ./internal/indexbench -run TestFreshnessProbe -v -count=1` — **PASS**,
5/5 correct after re-ingest (this is VB-17's gate; it always must pass). A
throwaway dump of the full report (never committed) gives the before/after
detail:

| Edit | Before (pre-re-ingest) | After (post-re-ingest) | ms to correct |
|---|---|---|---|
| rename | `<missing>` (wrong) | `ledger.go:GoFunc` (right) | 19 |
| add_param | `1` (wrong) | `2` (right) | 17 |
| delete | `ledger.go:GoFunc` (wrong) | `<missing>` (right) | 14 |
| move | `ledger.go:GoFunc` (wrong) | `query.go:GoFunc` (right) | 15 |
| change_literal | `7` (right — live `Sources()` read, no ingest needed) | `7` (right) | 0 |

fathom: **5/5** correct after its own re-ingest step (1/5 correct *before*
re-ingest — the live-read case only, exactly as the freshness.go package doc
predicts), all five resolved in under 20ms of in-process work once re-ingest
is triggered. graphify: **4/5** correct once its watch loop caught up, at
3.5-3.9s of wall-clock watch latency per structural edit, plus one question
its graph structure cannot represent at all.

The comparison is not apples-to-apples on timing — fathom's numbers are
in-process query cost after an explicit, deterministic `Ingest` call;
graphify's are wall-clock watch-loop latency against a real filesystem
watcher. Both are reported as measured, not normalized.

### Verdict

**k = 4/5 → VB-18's kill clause fires: freshness leaves positioning.**
graphify's `--watch` mode does close the four structural questions (rename,
add/remove a caller, delete, move) within single-digit seconds of an edit —
that is real freshness, not nothing. But it misses one of five, and the miss
is not a close call it might pass on a different run: it is a genuine surface
gap (no literal-value field) compounded by a genuine trigger gap (no rebuild
for a change that does not alter graph structure). Both are reproducible, not
flaky. fathom's own probe is not directly comparable on mechanism (explicit
re-ingest vs. filesystem watch) but answers the same five questions 5/5,
every time, in milliseconds once told to re-ingest — including the literal
one, via its live-source fallback.
