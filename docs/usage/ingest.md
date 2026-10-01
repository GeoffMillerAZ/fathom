### `ingest`

Ingest a service into the knowledge store.

```sh
fathom ingest --service <id> [--path <dir>] [--store <path>] [--mode local|faux] [--scip <path>]... [--rehash] [--json]
fathom ingest --service <id> --path <dir> --dry-run [--stats] [--json]
```

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Logical service ID, e.g. `payment-service` |
| `--path` | `.` | Source root directory (local mode) |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `--mode` | `local` | Connector mode: `local` or `faux` |
| `--scip` | auto-detected | SCIP index file to import as typed edges (ADR-041 decision 5, SU-9; repeatable). Unset: auto-detects `index.scip` at the service root and every `.fathom/scip/*.scip` |
| `--dry-run` | off | Discover and stat only (RT-70): report files, bytes, files over the 2 MiB read cap, the default-ignored and always-skipped directories, and the five largest files — without reading a file or opening the store. `--json` emits `{service, path, dry_run, files, bytes, oversize_files, oversize_bytes, read_cap_bytes, default_ignored_dirs, skipped_dirs, ignored_dirs, stat_failed, largest: [{path, size}]}`; `stat_failed > 0` makes `bytes` a lower bound |
| `--stats` | off | With `--dry-run`: the discovery statistics (what `--dry-run` prints) |
| `--respect-gitignore` | off | With `--dry-run`: discover a git work tree through `git ls-files -co --exclude-standard`, as `[connector] respect_gitignore = true` would (RT-68); `--json` adds `git` and `gitignored`. Run with and without it to measure the delta (RT-M11) |
| `--rehash` | off | Read and hash every file (RT-102). Without it an ingest reads only files whose size, mtime or inode moved since the last ingest the same process (the daemon) ran, or the store's recorded stats once `file_stats` exists; a one-shot CLI ingest reads everything today |
| `--json` | off | Emit the run as JSON: `{service, outcome: ok|degraded|refused, exit_code, discovered, extracted, skipped, symbols, read_failed, read_failures: [{path, err, errno?}], unclassified, oversize, classified_zero, unsupported_languages?, typed_edges?, scip_imports?, excluded?, default_ignored_dirs?, error?, hint?}` — `errno` is the class (`EPERM`, `EACCES`, `ENOENT`, `EIO`, `EISDIR`) derived from the connector's text, so CI can assert `read_failed == 0`. `unsupported_languages` (O69, present only when non-empty) is `{language: file_count}` — e.g. `{"zig": 412}` — for source in a KNOWN programming language fathom has no registered extractor for; see "Unsupported languages" below. `scip_imports` (present only when at least one SCIP index was imported) is `[{path, producer, language?, documents, edges, direct, unresolved, unmapped}]` — see "Typed edges by import" below |

Output: `discovered`, `extracted`, `skipped`, `symbols` counts, plus non-zero-only rows
(`unclassified`, `oversize`, `classified-but-zero-symbols`, `unsupported languages (no extractor)`,
`typed-edges`, `typed_edges: … via <producer>` per imported SCIP index, `excluded:` pattern
counts, and `read-failed`), `stat-skipped` (files not read: stat and fingerprint unchanged)
and `peak-inflight` (the most file content held at once — at most 64 MiB, RT-103); the
`typed-edges`, `cue-typed-edges` and `file_resolved` rows say `skipped — no input changed
since the last successful pass` when their input files did not change (RT-104, RT-105,
RT-108), and a `file_resolved` import-parse fault is a stderr warning naming the files.
`--json` adds `stat_skipped` and `peak_inflight_bytes`.

**Doc index (KR-8, KR-9).** An ingest also indexes the service's Markdown for `search kind: docs`:
the corpus is the Markdown discovery returned (so `.fathomignore` and a purge's scoped exclusions
apply), split into sections and written per file only when a file's units changed. The summary's
`docs:` line counts files scanned, re-indexed, unchanged and removed; a per-service byte budget
(64 MiB of heading and body text) keeps the live documents first, and what it cuts is named on a
`docs: N unit(s) elided` line; a document over 2 MiB or unreadable is a `warning: docs:` line, and
a pass that could not run (a store with no doc index, a service with no local root) says why. It
never fails the ingest. `--json` carries the same as `docs: {files, changed, unchanged, deleted,
units_written, links_written, units_elided?, elided?: [{path, heading, bytes}], failures?: [{path, err}],
skipped?}`.

**Doc-to-code links (KR-6).** Each unit the ingest (re)indexes is linked in the same pass: a
backticked span (or a lone token on a fenced-code line) that names exactly one indexed symbol, or a
repository path the index holds, becomes a `doc_links` row carrying the target's content hash at
link time (a contract key links once SG-2 registers keys). A span that resolves to nothing, or to
more than one symbol, is not linked. A unit the gate leaves unchanged is not re-linked, so its
link-time hashes stay what `staleness_report`'s `doc_link_drift` (KR-7) compares the live index
against; a link the doc no longer makes, or a removed section's, is closed, never deleted. The
summary adds `docs: N doc-to-code link(s) held by the re-indexed units`. The first ingest by a
binary with the link pass re-indexes every document once, to link units written before it existed.

**Typed-pass toolchain ceiling (RT-106).** The Go typed pass's toolchain children (`go list`
and the compiles it forks) run at background priority (`nice -n 10`, plus `taskpolicy -b` on
macOS) with `GOFLAGS` `-p` and `GOMAXPROCS` capped at 2 — a ceiling, never a raise: an
inherited `-p=1`/`GOMAXPROCS=1` stays 1, and every other inherited `GOFLAGS` flag passes
through unchanged. `FATHOM_TOOLCHAIN_PROCS=N` (env, positive integer) moves the ceiling;
`FATHOM_TOOLCHAIN_PROCS=off` lifts it (priority still lowered in the daemon); unset or
unparseable means 2. The CT-5 performance gate runs with it `off` so it measures the
pass's own cost, not the throttle.

**Typed edges by import (ADR-041 decision 5, SU-9).** Go gets typed call edges natively
(`internal/fusion.EnrichTypedEdges`, type-checking the source directly); every other
language gets them by IMPORTING an external indexer's SCIP output instead —
`internal/scip.ImportIndex` maps a SCIP index's definition/reference occurrences onto
fathom's `typed_edges` table, replaced per fathom package exactly like the native pass.
fathom never runs or vendors a per-language type-checker itself: produce the index
separately (`scip-typescript index`, `scip-python index`, …, per `docs/languages.md`'s
tracked delegate order) and either name it with `--scip PATH` (repeatable — one ingest
can import several indexes) or drop it at `index.scip` in the service root / anywhere
under `.fathom/scip/*.scip` for auto-detection. A reference occurrence resolves to
`core.EdgeDirect` when its symbol is defined somewhere in the same index, or
`core.EdgeUnresolved` when it names something outside the index (never silently
dropped). The summary line names the producer fathom read from the index's own
metadata, e.g. `typed_edges: 118 edge(s) (94 direct / 24 unresolved) via scip-typescript
0.3.16 [index.scip]` — `fathom capabilities --json`'s `typed_edge_producers` field names,
per language, which mechanism fathom's own code currently provides (`native` /
`scip:<tool>` / `file` / `syntax`).

Each definition is mapped to fathom's own symbol space by its POSITION — the same
service's already-extracted symbol table (the native or file-resolved pass's output for
the files the index covers, always present by the time `-scip` runs), matched by
document path + the definition's own line — falling back to parsing the SCIP symbol
string's own descriptor suffix only when that position match is ambiguous (O115: the
real, currently-published `scip-typescript`/`scip-python` releases never set a
definition's `DisplayName`, so a name reconstructed from it — the pre-fix behaviour —
never joined fathom's own symbol keys, and `find_callers`/`references`/`blast_radius`
silently resolved none of a SCIP-imported service's edges). A definition neither path can
place contributes NO edge, in either direction, and is counted in the summary's
`scip_imports[].unmapped` (JSON) — printed as a warning on stderr when non-zero, e.g.
`warning: scip import index.scip: 3 definition(s) could not be mapped to a fathom symbol
and contributed no edge (O115)` — never silently folded into `direct` or `unresolved`.

**Unsupported languages (O69).** `unclassified` lumps every file with no registered
extractor into one count, unnamed — a lockfile, an image, and a real Zig source file
all count the same way. When an unrecognized extension is also a KNOWN programming
language (`internal/connector.UnsupportedLanguageForPath` — currently:
Elixir, Clojure, Haskell, OCaml, F#, Erlang, Zig, Nim, Julia, Elm, Crystal, Groovy, and Swift,
whose dialect is built but refused: LB-Q-swift-rest),
ingest names it and counts its files separately, so a repo whose primary language has no
extractor reads as "zig: 412 files, no extractor" instead of a suspiciously low symbol
count with no explanation. The same breakdown is in `list_services`' per-service
`unsupported_languages` and the `unsupported_languages` doctor row (WARN, not FAIL — a
capability gap, not a misconfiguration). None of the languages above have a tree-sitter
grammar or extractor wired into this repo today; adding one is a separate, larger change
(a real extractor/grammar, TDD'd against a calibration fixture) — this is disclosure only,
never a fake extraction. Kotlin was on this list through wave P1; wave P2 (O79) closed it
— `.kt`/`.kts` now extract via `internal/extractor.NewTSExtractor("kotlin")` (tree-sitter
via wazero/WASM, ADR-021 §4, `internal/extractor/tsengine/grammars/kotlin.wasm.gz`) — the
first language added for new-language breadth rather than a CGO-incumbent swap.

The pipeline is hash-gated: unchanged files (same SHA-256 as the stored fingerprint) are
skipped. Re-ingesting a large repo where few files changed is near-free.

**Ingest honesty (IT-14/IT-15, O16).** A file the connector cannot *read*
(EPERM/EACCES/EIO, or ENOENT for a file deleted mid-run) is skipped and named:
`read-failed: N file(s)` with path + errno (first 5 inline, `... and N more`), its stored
evidence left untouched; every readable file is still indexed; the run exits 1 with
`error: ingest degraded: …`. A run that could read **nothing** while the store already
holds observations for the service **refuses to write** (no watermark advance, no purge):
`error: ingest refused: …`, exit 1, and on "operation not permitted" adds
`hint: external-volume TCC wedge? tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`.
Transport errors (context cancellation, connector failure) still abort before any write.
The daemon's `ingest` MCP tool returns the same cause + hint as a JSON-RPC error rather
than `ok:true`; `pkg/client` re-exports `client.ReadFailuresError` (`Refused`, `Existing`,
`Hint()`).

**Worktree re-root refusal (PC-33, I30, O22).** When the store already records a root
for `--service` and `--path` is a linked git worktree of that root (`git worktree add`,
or any path under `.claude/worktrees/`), the ingest is refused with
`error: ingest failed: ingest: worktree re-root refused: service "<id>" stays rooted at
<recorded>; <path> is a git worktree (...)`, exit 1, nothing written. The daemon's
`ingest` MCP tool applies the same check (`internal/worktree.CheckReroot`) and its
`ok` envelope names the `root` it refreshed. Moving a service from a worktree back to
its main checkout is allowed. Register a worktree under its own service id if you
mean to index it.

```sh
fathom ingest --service payment-service --path /src/payment
# service:    payment-service
# discovered: 142
# extracted:  38
# skipped:    104
# symbols:    891
```

---
