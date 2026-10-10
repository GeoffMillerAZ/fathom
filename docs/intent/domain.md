# Domain

What is true about fathom's problem space that the code cannot tell you. Vocabulary with
project-specific meanings, domain truths with their evidence, the platform facts that
shaped decisions, and the assumptions the design rests on.

The vocabulary stays here (the console's glossary is built from it); each other section lives byte-intact
in its member, and `fathom intent audit` (U15) holds the table to the members.

| Section | Member |
|---|---|
| Domain truths | `domain/truths.md` |
| Platform facts | `domain/platform-facts.md` |
| Assumptions | `domain/assumptions.md` |

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
  `spec-bindings`); strata `dialect` / `tech` (named `provider` until 2026-10-09) / `platform` /
  `concern` name who authors it and how it earns trust. Source: ADR-024, `docs/layers.md`.
- **tech pack** — deterministic data a technology ships about itself (a `pack` header; `guards`,
  `generated`, `formats`, `concerns`, fixtures) in a `fathom-pack/` directory, read from the
  repository's own pinned copy; the project's `.fathom/` uses the same format (plus `orientation`), and
  the custodian's shipped packs become fathom's built-in tech packs (K10: one noun, one format). A
  dialect pack (ADR-048) is a different thing: a pinned grammar at user scope. Source: ADR-061
  (proposed), `docs/design/fathom-dir.md`.
- **zone** — where a file fathom reads or writes belongs: authored config, authored knowledge, local
  state (never committed), user scope. Source: `docs/design/fathom-dir.md` §4.
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
- **decision-port site** — one typed judgment question shape (`route`, `certify_suggest`, `triage_failure`,
  `ask`, ...) that talks to a provider only when the repository lists it in `[decide] sites`; every site added
  after ADR-056 is opt-in. Source: ADR-042, ADR-056.
- **judged** — the tier label on any decision-port answer shown to a reader: a model's judgment, uncalibrated
  until its (site, template, provider, resolved model) reading clears the bar, never a code fact or proof.
  Source: ADR-056, I62.
- **exploration map** — an anchored record (symbol key plus content hash plus a why line) of which code matters for
  a purpose, composable by reference and delivered at handoff rather than looked up; a live map is a saved query
  beside the semantic layers' claims. Source: ADR-056, `docs/design/exploration-maps.md`.
- **pin / pin loss / pin line** — a *pin* is a test that a governing source names (an invariant's test column, a
  `// certifies:` marker, a `tests/*.cue` manifest entry, a scope evidence line, a guard's `done needs:`); a
  *pin loss* is a commit that deletes, renames, weakens or skips it, changes its golden file or moves its marker
  (*silent* when its source still names it at head, *retired* when the same range edits the naming source); the
  *pin line* is the one factual line the hook adds when a commit loses a pin. Source: ADR-057,
  `docs/design/served-not-kept.md`.
- **law N** — doctrine laws 1–15. Source: `docs/vision/11` §2 (1–8), `12` (9), `13`
  (10–12), `14` (13–14), `20` §6 (15); summarized in `principles.md`.
