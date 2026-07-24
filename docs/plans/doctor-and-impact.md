# Doctor + Impact (owner directive 2026-07-23, adoption-kit addendum)

Two additions to the adoption kit: a wiring doctor (deterministic CLI verb
+ harness-aware skill) and an impact narrator (skill over the flywheel
gauge). No new MCP tools; the CLI carries the truth, the skills carry the
interpretation.

Design (normative):
- **`fathom doctor [--harness=claude|codex|auto]`.** Deterministic checks,
  each rendered checked/pass|fail|skipped + reason + a one-command remedy
  (never a clean-shaped answer for an uncheckable state — the C4-5
  discipline): MCP registration URL points at /rpc/agent (read the
  harness's own config where locally readable; else checked:false with the
  manual instruction); daemon reachable AND binary-fresh (healthz +
  version-vs-HEAD, the binary-staleness doctrine); hooks installed AND
  firing (settings.json/lefthook presence + recent edit-hook/verify-cli
  origins in the call log — existence, not just presence); CLAUDE.md +
  AGENTS.md stanzas present, markers uncorrupted; skills installed,
  fathom-owned, current; .fathom/ config CUE-valid; obligations compiled +
  fresh for THIS repo (served claims exist; staleness surfaced);
  surface_traffic flowing (reuse the existence-check semantics). --json +
  human table; exit 0 all-pass, 1 any-fail, 2 cannot-run.
- **`fathom-doctor` skill** (one skill, both harness renderings, shipped
  with the AK-1 set): run the verb, interpret failures, apply the remedies
  the operator approves (init --skills, stanza repair, deploy command,
  recompile pointer), and the tie-in/optimize layer: verify the harness
  actually sees the tools (a live tool call), check the session brief and
  hook latency against the budget ledger.
- **`fathom-impact` skill**: run `instrument --adoption` (+ plans status,
  open observations) and narrate honestly — injections served, gate
  catches (with the predates-instrumentation split stated), review
  findings, claims freshness, zero-call tools, open observations — with
  the explicit boundary: report what the instruments measured, never
  attribute outcomes the data can't support.
- File boundaries (concurrent Wave-5 workers): new files only —
  cmd/fathom/doctor.go(+test), dist/agents/skills/fathom-doctor/,
  dist/agents/skills/fathom-impact/ — plus skillsinstall.go's skill list
  and docs. DO NOT touch instrument_adoption.go, calllog.go, review.go,
  internal/semantic/, or internal/mcp/ surface files.

## Tasks

- [ ] fathom doctor: the deterministic checklist with remedies, --json + table, honest uncheckable states; tested per check incl. corrupted-stanza, stale-binary, dead-daemon, and all-pass fixtures {id: DI-1, symbol: cmdDoctor, package: cmd/fathom}
- [ ] fathom-doctor skill (both renderings), shipped + installable; every command verified against the verb {id: DI-2, symbol: cmdDoctor, package: cmd/fathom, skip_test: true, skip: "skill source verified by install tests + critic command-accuracy pass, the AK-1 pattern"}
- [ ] fathom-impact skill (both renderings), shipped + installable; narration honesty rules stated in the skill itself {id: DI-3, symbol: cmdDoctor, package: cmd/fathom, skip_test: true, skip: "skill source over the existing tested gauge; anchor shared with DI-1 (the command family this plan lands)"}
- [ ] Adoption docs + catalog updated (claude-code.md, codex.md, usage.md, README routing); Maintained lines current {id: DI-4, symbol: cmdDoctor, package: cmd/fathom, skip_test: true, skip: "docs rows anchored to the verb they document"}
