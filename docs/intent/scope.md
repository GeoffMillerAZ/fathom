# Scope

Where the edges are right now. Present tense; done goals are deleted, git remembers.
Computed progress: `fathom plans status --service fathom` (computed on demand; never copied here).

This file routes (K8): every goal and section lives byte-intact in the member named beside it under
`scope/`, and `fathom intent audit` (U15) holds each listed id to the member that defines it.

## Current goals

A goal is a pre-registered bet — `{bet, instrument, threshold, review}` on its `Done when:`,
the instrument a `fathom` verb, a Taskfile target or a repo script — or `fathom intent
doctrine` refuses it at load and names what it lacks (DP-4, ADR-026 amendment 2026-09-25).

**G2. The A/B becomes admissible.** `scope/goals-1.md`\
**G3. The continuity-thread bet reaches its sample.** `scope/goals-1.md`\
**G4. Ambient fathom closes.** `scope/goals-1.md`\
**G9. The bench is honest, cheap to re-run, and the 2026-09-13 reading is the baseline.** `scope/goals-1.md`\
**G13. The Observatory console closes.** `scope/goals-1.md`\
**G14. Fathom is a responsible tenant (ADR-047, vision 36).** `scope/goals-1.md`\
**G15. Language breadth and depth, measured (ADR-048, vision 37).** `scope/goals-1.md`\
**G16. Reach: fathom installs in one step and serves the clients people use (ADR-049, vision 38).** `scope/goals-1.md`\
**G17. The system graph: proof crosses service boundaries (ADR-050).** `scope/goals-1.md`\
**G18. Dependencies: upgrade impact from what is already on disk (ADR-050).** `scope/goals-1.md`\
**G19. Spec interop and the certified proof tier (ADR-049).** `scope/goals-1.md`\
**G20. Knowledge retrieval: doc questions answered at grep's rate or better (ADR-049).** `scope/goals-1.md`\
**G21. Governance that learns and reaches reviewers (ADR-049).** `scope/goals-1.md`\
**G22. Progress and economics, computed (ADR-049).** `scope/goals-1.md`\
**G23. Test intelligence: the tests that matter, ranked and selected (ADR-051).** `scope/goals-1.md`\
**G24. Work intelligence: effort, friction and cost joined to proof (ADR-051).** `scope/goals-1.md`\
**G25. Change and compliance receipts (ADR-051).** `scope/goals-1.md`\
**G26. The feature map is measured against the owner's own scorecard (ADR-055).** `scope/goals-2.md`\
**G27. Level 10 through fathom: the test-run hook is measured against control and raw level 10 (ADR-056).** `scope/goals-2.md`\
**G28. Does a typed judgment model read code? Decided before any site reads code with one (ADR-056).** `scope/goals-2.md`\
**G29. Judgments are calibrated, not assumed (ADR-056).** `scope/goals-2.md`\
**G30. Exploration maps cut sub-agent tokens without losing accuracy, or are not built (ADR-056).** `scope/goals-2.md`\
**G31. Context survives compaction, measured, or the deliberate silence stands (ADR-056).** `scope/goals-2.md`\
**G32. Work signals: stalls detected without a model, weights estimated with a published error (ADR-056).** `scope/goals-2.md`\
**G33. A lost pin is caught in the turn that lost it, or the line stays a record (ADR-057).** `scope/goals-2.md`\
**G34. Stewardship facts earn their place, or are cut (ADR-060).** `scope/goals-2.md`\
**G35. The `.fathom/` zones and tech packs v1: one format, three authors (ADR-061, proposed; F5 promoted).** `scope/goals-2.md`\
**G36. Every file accounted for: the repository graph's facts earn their place, or are cut (ADR-067).** `scope/goals-2.md`\
**G37. Concern facts earn a place in a Review Kit bundle, or are cut (ADR-069).** `scope/goals-2.md`\
**G39. Readouts earn their place, or are cut (ADR-071).** `scope/goals-3.md`\
**G40. Claude Code mods earn their place, or are cut (ADR-072).** `scope/goals-3.md`\
**G41. Every interception seam rated Medium or higher works, or its own reading re-rates it (readouts design §20).** `scope/goals-3.md`\
**G11. A reading, a hand-off and an answer carry their evidence (ADR-034).** `scope/goals-2.md`\
**G12. The surface leads with the answers grep cannot give, and every answer is falsifiable (ADR-035).** `scope/goals-2.md`\
**G7. The evidence plane's waves 1–2 ship and its oracle gate is read (ADR-026).** `scope/goals-2.md`

## Current boundaries

Languages, surfaces, store, harnesses, the findings loop, the fleet, the substrate and the platform:
`scope/boundaries.md`.

## Compatibility surface

A change to any item is breaking: commit `!`, `### Breaking` in `CHANGELOG.md`, a
migration note (`docs/rename-migration.md` is the precedent; new ones go to
`docs/upgrades/<version>-<slug>.md`).

- Governed Store Format v0: additive within v0; a rename, retype, or removal is v1 and a
  `format_version` bump. Evidence: `docs/spec/store-format-v0.md`,
  `internal/store/format_v0_test.go` (`TestStoreFormatV0_RequiredSchema`).
- Expert Module v0 manifest. Evidence: `docs/spec/expert-module-v0.md`, `schema/module/schema.cue`,
  `internal/modspec/schema_test.go`.
- Plan grammar v2: checkbox states, annotation keys, `id:`, dispositions, the
  checkbox-line rule. Evidence: `docs/plans/README.md`, `schema/plan/schema.cue`,
  `internal/verify/markdown.go`.
- `fathom verify` exit codes 0/1/2 and the `--strict`, `--require-all`, `--quiet` flags;
  an unproven row whose evidence could not be read (index-stale, graph fallback, unanswered)
  is operational: exit 2 when every unproven row is one;
  every wired repo's `lefthook.yml` depends on them. Evidence: `cmd/fathom/verify.go`.
- Hook verbs and their stdin/stdout contracts: `session-hook` (`--event
  instructions-loaded|subagent-start|tool-failure|cwd-changed|test-run|failed-test-run`),
  `edit-hook [--pre]`,
  `stop-hook`, `session-end-hook`, `pre-compact-hook`, `config audit --hook [--event
  file-changed]`, `doctor --brief`, each with an additive `--harness <id>` (absent ⇒
  `claude-code`). `fathom init` writes them into `~/.claude/settings.json` and, under
  `--harness=gemini-cli`, into `.gemini/settings.json`'s `hooks` key; it writes no Codex
  hook file (codex-cli only loads a user-global one — `fathom harness hooks --harness
  codex` prints the entries instead). Evidence: `docs/adoption/claude-code.md`, ADR-029.
- `fathom harness list|capabilities|hooks|ladder|matrix` (`--json` where offered) and
  the exported descriptor shape, including each descriptor's event catalogue
  (`schema/harness`). Evidence: ADR-029.
- `fathom intent index --check` (exit 1 naming each `ADR-NNN` whose heading in `ADR.md` or row in
  `docs/intent/decisions/README.md` is missing or duplicated). Evidence: `cmd/fathom/intentindex.go`, ADR-074.
- The conservation contracts (ADR-044): `fathom conserve --json`'s `fathom.conserve/v1` shape
  (schema string, plane and kind names, field names, plane-kind-key sort order), the plan keys
  `gone:`, `retires:`, `surface_change:`, `baseline:` and `tests:`, and `.fathom/config.toml`'s
  `[[surface.pattern]]` block. Evidence: `docs/design/refactor-conservation.md`,
  `cmd/fathom/conserve.go` (`TestCmdConserve_JSONSchemaV1`).
- The certification contracts (ADR-043): the `// certifies: <kind> <id> [(<strength>)]` marker
  grammar, the `certified_by:` plan key, and `fathom certify gaps --json`'s
  `fathom.certify.gaps/v1` shape (schema string, field names, kind-then-id sort order) that
  Review Kit's `fathom-certify` adapter parses. Evidence: `docs/design/test-certifications.md`,
  `cmd/fathom/certify_gaps.go` (`TestCmdCertifyGaps_RemedyLinesAndJSON`).
- Plan grammar v2's lexical oracle keys on plan rows (`oracle: lexical`, `path`,
  `pattern`). Evidence: ADR-030, `schema/plan/schema.cue`.
- The spec-interop plan keys (ADR-049): `weight:` (a positive integer, default 1) and
  `accept:` (quoted acceptance text, never gating by itself). Evidence:
  `docs/design/spec-interop.md` §4, `schema/plan/schema.cue`.
- The `<!-- fathom:begin -->`…`<!-- fathom:end -->` stanza region in `AGENTS.md`, and the
  fathom-marked `@AGENTS.md` import region in a root `CLAUDE.md`, `.claude/CLAUDE.md` or
  `CLAUDE.local.md` that fathom did not create (fleet reconcile compares content, law 15).
  Evidence: ADR-059.
- The default agent surface (8 tools, named in ADR-053: `find_symbol`, `search`,
  `read_source`, `symbol_context`, `obligations`, `verify_plan`, `blast_radius`,
  `review_diff`) at `POST /rpc/agent`; the `extended` opt-in surface (21 tools) at
  `POST /rpc/extended`; tool names and schemas; the `/rpc/companion` alias (→ `extended`).
  Evidence: `docs/mcp-tools.md`, `internal/mcp/surface.go`.
- The `.fathom/` files an adopter commits: `config.toml`, `concerns.cue`, `guard.cue`, `capabilities.cue`,
  `modules.cue`, `risk.cue`, `learning.cue`, `layers/`, `rules/`, `trace/`, `observations/` (one `O<n>.md` per entry,
  EW-32) and the legacy `observations.md`, read and never written; and `.fathomignore` at the root. Evidence:
  `schema/{binding,guard,config,semantic,rules,risk,learning,trace}`, `cmd/fathom/observe.go`. The whole of
  `config.toml`, `.fathomignore`, `risk.cue`, `learning.cue` and `trace/` were named 2026-10-09: adopters already commit
  them and fathom already reads them (`docs/design/fathom-dir.md` §2). ADR-061 (proposed) adds the `#Pack` and
  `#Project` keys and the `.fathom/.gitignore` region when its rows land.
- The `~/.fathom/` files: `config.toml`, `capabilities.cue`, `concerns.cue`,
  `verify-ledger.jsonl`, `ignore.d/<service>`. Evidence: `docs/configuration.md`, `CHANGELOG.md` 2026-08-14.
- Telemetry record shapes carrying `schema_version` and `fathom_version`: call log JSONL,
  beats, session ledger, rollups, `/statusz` JSON; the research bundle
  (`schema/research-bundle`). Evidence: `CHANGELOG.md` IO-11..13.
- The daemon's admission wire format (ADR-047 §3.4): HTTP 503 with `Retry-After`,
  `X-Fathom-Busy-Scope` (`class` | `daemon`) and JSON-RPC `-32050` (`data: reason, class,
  level, retry_after_ms, in_flight`), the named `abandoned` outcome, `budget` (`partial`,
  `elided`, `bytes_read`, `reason`), and the managed daemon's 409 to a test caller.
  Evidence: `internal/mcp/admission.go` (`TestAdmissionReturnsNamed503WithRetryAfter`),
  `cmd/fathom/admission_http.go` (`TestTestCallerRefusedByManagedDaemon`).
- The stewardship reports' JSON (ADR-060): `fathom plans lines --json` (the `worklines.Report`
  shape: field names, class names `landed`, `at-risk`, `idle`, `active`, no plan-row fields
  since the 2026-10-01 amendment, kind-then-name order). Evidence: `docs/usage/stewardship.md`,
  `cmd/fathom/plans_lines.go` (`TestCmdPlansLinesReportsAndNeverActs`); and `fathom report used
  <dir> --json` (the `usedset.Result` shape: field names, unit kinds `go-package`, `embed`,
  `css-source`, `css-import`, `js-module`, `module-file`, kind-then-id order, `used_files`).
  Evidence: `cmd/fathom/report_used.go` (`TestCmdReportUsedJSONReasonsAndUnreached`).
- `pkg/client` and `pkg/platform` exported Go API. Evidence: `pkg/client/client.go`.
- CLI verbs and their `--json` envelopes (law 10: every capability is a verb first).

Pre-1.0: breaking changes bump minor. The line still gets written.

## Non-goals

What fathom deliberately does not do, each with its revisit condition: `scope/non-goals.md`.
