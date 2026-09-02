# Domain

What is true about fathom's problem space that the code cannot tell you. Vocabulary with
project-specific meanings, domain truths with their evidence, the platform facts that
shaped decisions, and the assumptions the design rests on.

## Vocabulary

- **observation** — the atomic evidence record: service, source ref, kind, tier, symbol,
  claim, content hash, calls. Source: `internal/core/model.go`, `docs/architecture.md`.
- **evidence tier** — `verified` (ceiling 5) › `pre-scan` (4) › `doc-claimed` (2) ›
  `sme-stated` (2) › `assumed` (1); a doc claim never outranks a code fact. Source: ADR-007.
- **two strata** — the *deterministic index* (scraped, provable) and the *semantic layers*
  (LLM-compiled governed claims with citations, tiers, staleness). Source: ADR-015.
- **claim** — a compiled statement, content-addressed, citation-hashed, scope-verified,
  swept for $0 on every ingest, served flagged when stale. Source: ADR-016, `docs/vision/01-thesis.md` §1.
- **layer / pack** — a compiled claim set (`obligations`, `findings`, `adr-rules`,
  `spec-bindings`); strata `dialect` / `provider` / `platform` / `concern` name who
  authors it and how it earns trust. Source: ADR-024, `docs/layers.md`.
- **obligation** — a per-file rule injected before an edit, ≤150 tokens, zero output on a
  clean file. Source: ADR-016.
- **surface** — a path-scoped MCP tool allowlist over HTTP: `/rpc` (all), `/rpc/agent`
  (16 tools), group surfaces; ergonomics and measurement, never a security boundary.
  Source: ADR-014 and its amendment, `docs/mcp-tools.md`.
- **service** — one indexed corpus root under one id; a root is never claimed by two
  services. Source: `.fathom/observations.md` O1, IO-2.
- **fleet** — every service the shared daemon serves; `fathom fleet` reconciles their
  wiring. Source: `docs/guides/fleet-index.md`.
- **watermark** — the freshness token: git HEAD, else a content manifest hash. Source: ADR-017.
- **plan row / contract / disposition** — a gated checkbox line, its `{symbol, package,
  reachable_from…}` acceptance contract, and the typed reason a `[-]` row is skipped.
  Source: ADR-018, `docs/plans/README.md`.
- **test_reaches** — a `Test*` *statically* reaches the symbol through the call graph;
  never that the test passes. Source: `docs/plans/README.md` "Honesty boundary".
- **thread** — one strand of intent spanning sessions; sessions are events on it.
  Source: `docs/vision/21-continuity-threads.md`.
- **arm / unit / beat / ledger** — the A/B vocabulary: ON/OFF measurement arm, an
  arm-homogeneous unit (a covered session or window, ≥10 turns), a hook heartbeat row,
  the append-only session and verify ledgers. Source: `docs/design/production-tokenomics.md`, MI-5.
- **concern (custodian)** — a falsifiable want about agent configuration, audited
  mechanically. Source: `.fathom/concerns.cue`, `docs/design/config-custodian.md`.
- **guard** — a pre-edit note scoped to paths or commands. Source: `.fathom/guard.cue`.
- **expert module** — a conforming store plus a manifest. Source: `docs/spec/expert-module-v0.md`.
- **stamp** — a file recording that an activation step ran; plans cite stamps, not memory.
  Source: law 6, `docs/vision/11-dial-to-eleven.md` §2.
- **kill clock** — the 30-day zero-call window after which a tool is deleted; it starts
  only when a consumer population exists. Source: law 8; `docs/vision/05-surfaces.md`.
- **law N** — doctrine laws 1–15. Source: `docs/vision/11` §2 (1–8), `12` (9), `13`
  (10–12), `14` (13–14), `20` §6 (15); summarized in `principles.md`.

## Domain truths

- Pull delivery fails. The fathom-vs-grep discovery share measured 0–7%, then a
  baseline of 0 fathom calls vs 193 greps; the durable channels are hooks that inject
  and gates that block. Evidence: `docs/design/product-vision.md` §2, `docs/vision/01-thesis.md` §1.
- Agents over-claim "done". The first gated run (2026-07-07) caught the plan's own
  author; 180 of 436 gate runs in five days caught claimed > verified. Evidence:
  `docs/plans/README.md` "Recorded demo", `docs/vision/24-work-frontier.md` §7.
- The dominant defect class is the silent false signal that reads as healthy: a coin-flip
  service resolution (36% of edits got no rules), a hook asserting "not indexed", six
  days of inert injection, a zero-symbol index printing "0 failed", parser fields
  dropped for weeks. Evidence: `.fathom/observations.md` O1, O2, O15, O16;
  `docs/dogfood-observations.md` O24.
- Compiling *code* into summaries loses to query-time reasoning (the enrichment path has
  zero production callers); compiling *binding documents* into governed claims is the
  differentiated half. Evidence: `docs/vision/01-thesis.md` §2.
- Aspirational prose compiled into rules destroys trust; `docs/vision/` is excluded from
  the obligations corpus for that reason. Evidence: `docs/vision/README.md`.
- Knowledge without a consuming tool stalls: `adr-rules` and `spec-bindings` sat
  eval-perfect with zero production claims for weeks. Evidence: ADR-024.
- The measured win is modest and specific: equal-or-better answers at ~2× fewer tokens,
  2–3× faster; the "≤1/10th tokens" claim is retracted. Evidence:
  `docs/design/coding-agent-excellence.md` §0, `docs/design/product-vision.md` §3.
- A forced channel that lies or blocks for infrastructure reasons gets uninstalled within a
  week. Evidence: `docs/design/coding-agent-excellence.md` §5.3.

## Platform facts

- macOS TCC removable-volume and Full Disk Access grants bind to code identity; an ad-hoc
  signed Go binary changes cdhash every build, so a redeployed launchd daemon silently
  loses the grant (crash-loop or an unpromptable hang). Consequence: `task deploy` signs
  with the persistent `fathom-dev` identity. Source: `docs/dogfood-observations.md` O10, `Taskfile.yml`.
- `cp` over a running executable truncates it in place; an unsigned copy is refused
  (`OS_REASON_CODESIGNING`). Consequence: temp path + rename, then a serving assertion.
  Source: O25, `Taskfile.yml` deploy.
- launchd `KeepAlive` means the daemon exiting *is* a restart. Consequence: the daemon
  self-adopts a verified newer binary (AU-1..3). Source: `CHANGELOG.md` 2026-08-12.
- Reads under an external volume can fail `EPERM` for a whole process tree while `stat`
  succeeds (a wedged TCC verdict). Tools then emit false signals: an empty index reporting
  "0 failed", `gofmt -l` naming an innocent file, "88 of 88 tools zero-call".
  Consequence: read errors are classified and named, never rendered as zero (IT-1);
  the reset is `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`.
  Source: `.fathom/observations.md` O16, O17.
- Claude Code's `Stop` event fires per turn, not per session; 200 ledger rows were 8
  sessions. Consequence: denominators count distinct session ids. Source: `CHANGELOG.md` 2026-08-09 (MI-1..3).
- Claude Code `SessionStart` carries a source (`startup|resume|clear|compact`); the brief
  prints only for startup and clear. Codex has no SessionStart hook, so `fathom thread
  brief` is instructed instead. Source: `CHANGELOG.md` 2026-08-13 (TH), `AGENTS.md`.
- Go map iteration is randomized. Consequence: candidates are sorted before matching;
  determinism is a tested contract. Source: `.fathom/observations.md` O1, `internal/CLAUDE.md`.
- SQLite WAL gives many cross-process readers beside one writer; DuckDB's single writer
  forced daemon-stop rituals and made daemon wedges total outages. Source: ADR-020.
- The Go linker writes to `TMPDIR`, not `GOTMPDIR`; scratch inside the module makes
  `go vet` walk leftover build dirs and puts every `t.TempDir()` inside a git repo.
  Consequence: both point at a scratch dir on the roomy volume, outside the module.
  Source: `Taskfile.yml` env comment.
- lefthook buffers each step's output until it finishes, so a 22s advisory review reads as
  a hang and scripted pushes were killed at 60–90s. Consequence: `review --budget-seconds 20`. Source: O9.
- `git archive` embeds the commit timestamp in every tar entry, so identical trees hash
  differently as tars. Consequence: the vendor drift guard hashes per-file content. Source: ADR-019.
- Tree-sitter grammar `.wasm` blobs target the Emscripten ABI, not bare wazero; the working
  path compiles tree-sitter's own core plus one grammar to wasm32-wasi. Source: ADR-013, ADR-021 §4.

## Assumptions

- One person, one machine, one loopback daemon, one writer. If false: single-writer
  contention, `$HOME`-keyed ledgers and path assumptions break; revisit the daemon and
  store decisions and write the threat model first. Source: `docs/vision/20-ambient-fathom.md` §7 Q1.
- Serving the whole fleet's source unauthenticated on loopback to any local process is
  acceptable on a single-user workstation. Never accepted in writing. If false: law 9
  (auth + read-only) must land before any second adopter. Source: `docs/vision/20` §7 Q2.
- The obligations corpus stays tiny and binding (`CLAUDE.md`, `ADR.md`,
  `docs/plans/README.md`). If it grows aspirational content, precision falls below the
  0.90 gate and injection becomes noise. Source: `docs/vision/01-thesis.md` §2, ADR-016.
- Claude Code's hook envelope is the interchange format the other harnesses shim to
  (Codex ≥0.152 carries eleven Claude-shaped events; Copilot, VS Code, Gemini CLI,
  Cursor and Amp accept or export the same shapes). If false: the codecs in
  `internal/harness` need per-harness re-verification; the descriptor's
  `verified_against` field and the router footer's tool versions are the tripwire.
  **Restated 2026-09-02 (ADR-029)** — the earlier form, "Claude Code and Codex remain
  the harnesses and their hook events keep their shapes", FIRED on 2026-09-01: Codex
  gained a Claude-shaped 11-event hook contract (`docs/vision/25-harness-port.md` §1).
- A sibling `../agentic-kit` checkout exists for co-development; otherwise the in-tree
  mirror serves. If the kit becomes public, ADR-019's mirror is removable.
- Kill clocks have a consumer population (sessions from non-fathom repos in the call log).
  If false, zero-call is exposure-zero and nothing may be killed. Source: law 8.
