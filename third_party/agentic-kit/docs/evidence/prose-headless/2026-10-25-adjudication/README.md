# HP-13 offline document review qualification

Scope: [DA-A01..10](../../../designs/prose-document-adjudication.md), bound to the
[48-file command/test/input manifest](receipt.json). This evidence exercises retained
v2 artifact consistency, explicit content disclosure and canonical caller-declared
goal replay. It contains no real human observation, provider call, source revalidation,
independent document acceptance, publication or UI-entry evidence.

## Independently exercised behavior

Parent-owned tests drive runLab rather than the new loader or assessment helper.
They construct ordinary v2 candidates through the existing capture engine using
intentionally meaningless generated words and a fake support reviewer. Those words
prove plumbing only; they cannot qualify clarity, factual fidelity or usefulness.

A metadata packet excludes generated text, private reviewer identity and rationale.
Explicit content reveals the selected text, facts, goals, claims and UTF-8 node
coordinates. Both views bind the same packet, and later evaluation time preserves
its identity. All actual packet and report forms are checked with CUE.

| Supplied declaration / artifact | Declared result / exit |
|---|---|
| No observations | Unknown /2 |
| Five independently declared human passes | Pass /0, identity and acceptance still unverified |
| Agent heuristic findings | Unknown /2 |
| Human producer or unknown relationship | Unknown /2 |
| Stale, future, wrong subject or definition, disallowed N/A | Unknown /2 |
| Known required failure | Fail /1 |
| All semantic passes with required long-length hold | Fail /1; original held receipt unchanged |
| Agent claiming human method, foreign/duplicate/length criterion, null observations or wrong packet | Refused /2 |

A coordinated sidecar policy forgery uses a freshly pinned receipt, changed methods,
recomputed canonical assessment and matching sidecar digest. It still refuses because
policy is rebuilt from the pinned fixture. Read-only 0500 roots and 0400 files work
without chmod. Canceled reads, broken output and an explicitly empty report-only
flag refuse. Root and subcommand help work without private inputs; help never masks
an unknown private flag. Original artifact bytes and directory entries are unchanged.

## Repairs retained

The first actual content packet failed CUE because generated claim IDs use
node#c1..128, while the new schema used a plain command ID. A dedicated bounded
claim-ID type fixes the projection without broadening the shared ID rule.

A Windows cross-build found the POSIX-only O_NOFOLLOW constant. Tiny unix/other
flag files preserve available no-follow behavior and the existing pre-Lstat/opened
regular-file/SameFile checks before bounded reads. Non-Unix retains the file identity checks and requires a trusted
operator namespace. Cross-builds prove compilation only;
no native Windows/Linux private-permission behavior or adoption is qualified.

Independent negative CUE tests then found that the schema accepted agent/human
method contradictions and a declared pass over a required length hold, although Go
already refused both. Cross-field CUE constraints now reject them. Canonical
assessment enums are reused; the final owner tests pass all refusal cases.
The [source review](../../../assessments/2026-10-25-prose-adjudication.md) records
scope, history, blast radius and residual boundaries.

## Integration gate

`task validate` passed with the final source manifest, including full workspace
race tests, CUE validation, scripts and constitution checks. No exported library
roots or dependencies changed, so downstream consumers were not rerun. Strict static
plan proof at the integrated source confirms symbol presence, product wiring and
eight reaching tests. It does not attest runtime success. The separate actual
test results above do. The [contribution reconciliation](contributions.md) records
worker preservation, removals and the retained Herdr boundary.

## Remaining work

At HP-13 qualification, HP-14 had not run. The subsequent [one-cell pilot](../2026-10-25-long-pilot/README.md) held before complete artifacts. Actual independent
human observations and the required long-artifact witness remain absent. The full 36-cell campaign still needs its frozen
acceptance/adjudication and negative-execution protocol. The original failed v1
cell and its exhausted repair remain failed. Full document/HITL/UI acceptance and
the broader governed-command objective remain open. Library roots/go.mod are
unchanged; prior consumer evidence is not upgraded or rerun here.
