# Untracked quality work: preserve the consumer handoff

Packet R1/R5, owner-funded adoption maintenance; baseline `ff2ad7b`.
The untracked next action said to record discovery context "through" two read-only
document lookups. The guides already define the destination, but the direct response
did not distinguish retrieving those instructions from preparing a durable record.

The shared reader now names the existing consumer-owned `docs/app-kit.md` or issue
for review, with original exact search query/profile, task, selected capability ID,
inventory revision and catalog revision. It explicitly says the lookup writes nothing.
The original search query/profile is caller context, not something the quality lookup
received or can reconstruct. Empty results still mean untracked, not complete.

Scope: reader next-action prose, structural/protocol regressions and owning guidance.
ADR-0066/0069 and I8/I18 already govern this repair; no new design decision or fired
promotion trigger. No fields, schemas, tools, catalog pins, quality statuses or limits
change. No consumer, installed binary, host registration, dependency or global changes.

Acceptance: keep the 240-byte action bound; test matched, filtered and untracked
selection; exercise actual MCP tool calls and compare Gallery JSON; refuse unknown
IDs and recover in the same session. No browser acquisition or human adoption claim.
Budget: one focused regression run and one full affected-package/race run; investigate
a concrete failure before any retry. Preserve source snapshots before execution.

Verification and source snapshots are retained in
`/Volumes/m2ext/appkit-evidence/appkit-adoption-handoff.atsP0bBs`.
The full check covers `./internal/qualityplan`, `./cmd/appkit-mcp`, `./apps/gallery`
and the independent radio structural regression in `./appkit/ui/form`.

All four packages passed with Go1.26.3 using
`go test -mod=readonly -count=1 -race -timeout=120s` and offline dependency settings.
The source was frozen before the run; unchanged production/test bytes were reviewed
independently. The action is222 bytes, retaining the240-byte test limit.130 local
Markdown paths resolved and CLAUDE.md/AGENTS.md remained identical. These are source
checks, not current-host adoption, browser or performance evidence.

- `baseline.tar`: `7f7cd1de93a13dc4379ca77ce5fd778d6b5e526450c88c3b7e379fa50c575ca9`
- `checked-overlay.tar`: `7feee72cc64c924d44fd8e96e3e50469703fe33dbe2e667576c7d7bd947b2a0a`
- `tests.log` retains the successful four-package output. Results paragraphs were
  added after execution; the archive preserves the exact pre-run source and docs.

Contribution disposition: returned as a kit-owned guidance/test improvement; no
originating-app migration or second-app propagation is inferred. Rebuild the server
for the clarified response. Source protocol evidence is not installed-host delivery.
This closes the selected handoff gap, not G1/G4 or the broader uplift goal.
