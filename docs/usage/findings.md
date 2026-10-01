### `findings` — findings as evidence (**experimental**; entry ≥15–20 resolved findings)

```sh
fathom findings near       <symbol|file> --store P --service S [--hops 2] [--include-superseded] [--as-of <RFC3339|sha>] [--json]
fathom findings status     --store P --service S [--json]
fathom findings import     --store P --service S --subject ORG/REPO/COMPONENT [--producer NAME] --file P [--dry-run] [--json]
fathom findings candidates --store P --service S [--min-sites 3] [--json]
```

`near` is the call-graph precedent lookup (mirror of the `findings_near`
MCP tool); `--as-of` (SU-19, ADR-041 decision 7) narrows to claims whose
bi-temporal validity window contains the resolved RFC3339 timestamp or
commit sha, BEFORE the superseded/refuted gate runs — "was this finding
true then", answered regardless of `--include-superseded` and regardless
of the claim's CURRENT status. `status` reports entry-condition progress
and, since the
findings loop (FL-23), `candidates`/`disagreements_by_rule` counts, plus
(SU-16) `rules_never_fired_30d` and `top_fired` — computed from the hooks
beat log every other `fathom metrics`/instrument reader shares
(`beatLogPath`/`readBeatsFor`). `candidates` derives rule candidates —
class, normalized shape, member citations, producers, code-change
resolutions, a drafted rule sentence, and the ADR-038 check family that
could enforce it — from patterns spanning at least `--min-sites` distinct
sites across native and external (review-kit-imported) findings alike.
Since SU-15, each candidate's score is sites × mean CODEOWNERS-ownership
weight of its member files (1.0 with no CODEOWNERS) × a decay applied once
per disputed (refuted) member; a caller-set cadence cap bounds how many
candidates are emitted per call, counting the rest as `deferred`; the
drafted rule carries a `promoted_from: <pattern id>, members: N,
producers: [...]` provenance tag. fathom never writes a rule; this is a
report a human decides whether to act on. Design:
`docs/design/findings-as-evidence.md`, `docs/vision/30-findings-loop.md`,
`docs/vision/31-substrate.md` §3 D8.

**The findings loop** (`docs/vision/30-findings-loop.md`, ADR-040;
`docs/plans/findings-loop.md`): `import` is the governed entry point for an
EXTERNAL review kit's own findings — validated against `.fathom/config.toml`'s
`[findings] subject` (no mapping, no import), deduplicated and ordered by a
per-claim cursor, capped at the producer's declared tier (`schema/semantic/
producers.cue`), and never accepted from a `fathom`-origin producer. The
receipt (`{schema_version, delivered, duplicate, stale, unclassified,
rejected: [{index, finding_id, reason}], truncated, cursors, dry_run}`)
exits 1 on any rejection. `candidates` derives rule candidates from
patterns recurring across `--min-sites` distinct sites (native and
imported findings together) — a report only; fathom never writes a rule.
When `.fathom/config.toml` names an `outbox`, the session hook imports it
once per session and reports the receipt in the brief — see
`docs/configuration.md`'s findings-loop section and
`docs/adoption/findings-loop.md` for the end-to-end guide. `fathom claim
add` (see the `claim` section above) is the loop's third verb: one
human-authored, sme-stated-capped claim minted directly, no recurrence or
import batch required; `fathom claim refute --dry-run` previews which
served rows a refutation would withhold before it is recorded.
