# Multi-machine development (MM-)

Grammar: v2. Decision: ADR-078 (accepted owner-architect, 2026-10-22).
Design: `docs/design/multi-machine-development.md`. Guide: `docs/guides/multi-machine.md`.
Docs and skill now; no transport or placement implementation. The controller re-verifies after the commit.

- [x] MM-1: the guide exists {id: MM-1, oracle: lexical, path: docs/guides/multi-machine.md, pattern: "^## The job contract"}
- [x] MM-2: the shipped skill exists {id: MM-2, oracle: lexical, path: dist/agents/skills/fathom-offload/SKILL.md, pattern: "^name: fathom-offload"}
- [-] MM-3: receipt identity columns {id: MM-3, disposition: deferred, reason: "table freeze until RT-M1 reads PASS (ADR-049 decision 2); adds columns to test_runs"}
- [-] MM-4: helper cold start through export snapshot and ingest --cache-from {id: MM-4, disposition: deferred, reason: "IXS phase 1 after RT-M1 PASS and IXS-M0b parity (ADR-078 D3)"}
- [ ] (op) MM-M1: a real remote job round trip read: source identity in equals the result's, every returned sha256 verified, integration verified by fathom {id: MM-M1, oracle: lexical, path: docs/bench/readings/multi-machine/round-trip.md, pattern: "^MM-M1 verdict:"}
