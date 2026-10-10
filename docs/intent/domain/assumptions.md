# Domain: assumptions

Part of the domain router, `../domain.md` (the vocabulary and the section index).

## Assumptions

- One person, one machine, one loopback daemon, one writer. If false: single-writer
  contention, `$HOME`-keyed ledgers and path assumptions break; revisit the daemon and
  store decisions and write the threat model first. Source: `docs/vision/20-ambient-fathom.md` §7 Q1.
- Serving the whole fleet's source unauthenticated on loopback to any local process is
  acceptable on a single-user workstation. Never accepted in writing. If false: law 9
  (auth + read-only) must land before any second adopter. Source: `docs/vision/20` §7 Q2.
- The obligations corpus stays tiny and binding (the instruction files, `AGENTS.md` first
  and any `CLAUDE.md`, plus `ADR.md` and `docs/plans/README.md`). If it grows aspirational
  content, precision falls below the 0.90 gate and injection becomes noise. Source:
  `docs/vision/01-thesis.md` §2, ADR-016. **Restated 2026-09-29 (ADR-059)**: the file list
  named `CLAUDE.md` alone; AGENTS.md is the one instruction file now.
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
- Fast typed-judgment models keep improving (P24) and Jev stays available. If false: nothing breaks by
  construction, because every judged feature has a deterministic path and provider `none` is the default; only
  an optional provider lane and its calibration readings are lost. Jev signups were paused on 2026-09-22 and its
  OpenRouter access is alpha. Source: ADR-056.
