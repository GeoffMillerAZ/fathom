# Decision-site delivery receipt

Baseline: `da35deae528f5a7dc50c6f4349fae26242935d14`. This receipt accompanies the
same commit as [DL-1..6](../../../designs/decision-lab.md), decision 0033, the CLI,
guide, survey and skill. The verification candidate was an uncommitted topic branch;
its evidence is not a claim of a clean release build or live-provider qualification.

## Delivered and checked

| Contract | Evidence |
|---|---|
| DL-1..3: offline preparation and captured-result checks | CLI tests cover identity, exact coverage, refusal/invalid/unknown, probabilities, private exclusive output, sanitization, malformed JSON and output errors |
| DL-4..5: independent scope and bounded deterministic diagnostics | Race tests and CLI probes cover duplicate exposure, case loss, mixed cached/fresh answers, typed causes/attempts, safe answer details and byte-identical reruns |
| DL-6: recipes and usable adoption path | Complete review/retrieval fixtures, clarification pack, source-backed ten-site survey, short guide and scoped skill; [guide-only trial](skill-trial.md) reached retrieval preparation within two seconds after build |
| Independent review | [Utility review](utility-review.md) closed three concrete findings; [final review](final-review.md) closed CUE/Go authoring drift with independently rerun counterexamples |

`TMPDIR=/Volumes/m2ext/tmp/agentic-kit-build task validate` passed, exit 0, after the
schema fix. It includes formatting, vet, constitution/CUE checks, script checks and
race tests, including the new CUE/Go boundary test. Focused
`go test -race ./cmd/decision-lab` passed. The schema test requires the project's
installed CUE tool and invokes it only over local schemas/temporary fixtures.
The skill-creator validator returned `Skill is valid!`.

Local logs: `/Volumes/m2ext/tmp/decision-sites-validate.log`,
`decision-sites-focused.log`, and `decision-sites-schema-{red,green}.log`.
The red run reproduced eleven schema acceptances before the fix; the green run
rejected them. Logs are local diagnostics, not a portable evidence store.

Final `cmd/decision-lab` content identity:
`sha256:d94ab504846a7573f44631c14d4e186faf46738a99ccc60825ea70c9a8fa67b5`.
Recipe: sorted relative file paths, each followed by NUL then its raw SHA-256 bytes,
concatenated and hashed with SHA-256. Includes Go, CUE, README and all fixtures.

No exported library API changed, so downstream consumer checks were not required or
claimed. Catalog regenerated to 214 packages; README count matches. Intent router and
scope remain within their 80/150-line budgets. New relative Markdown targets were checked.
AK-CLI-02 remains the documented dependency-free presentation divergence.

## Interpretation and remaining work

Complete synthetic studies exit 0 despite planted bad predictions. Retrieval reports
seven cases, five known answers, two unknowns, one false favorable and one abstention
against an expected favorable label. Omitting a capture exits 2 with the missing ID.
AI-authored labels are marked `frontier`; these fixtures establish diagnostics only.

No provider was called, credential loaded, production site enabled, consumer migrated,
global hook changed, or model quality/savings measured. Studies are not promotion or
action authority. The utility intentionally supports text-only string-choice packs;
other port kinds/native API controls retain their own conformance and domain oracles.
Broader AE-1/AE-2/AE-7 and admitted wave prerequisites remain open. Next adoption work
needs a receiver-owned independent observed cohort, explicit budget and paired baseline
comparison at a named site; DS-01/DS-03 are the prioritized candidates.

Fathom obligations remain raw/incomplete under O11; its partial pre-commit diff review
is not a proof. No new plan checkbox was checked. The completed implementation worktree
was removed only after its full patch was preserved externally and verified as integrated.
