# HP-10 offline length-policy qualification

Scope: command-local v2 intake and deterministic assessment under
[0046](../../../intent/decisions/0046-separate-document-sufficiency-from-length-targets.md).
The [source manifest](receipt.json) binds exact command/test/input bytes. This
report does not qualify generated-document quality, model economics or UI entry.

## Independently authored inputs and oracles

The v2 corpus preserves all six scenarios, fact/section/semantic-goal rosters,
36 planned cells and 43 available negative seeds. A separate pinned scope fixes
all six length definitions and the required technical-design1200..1600 witness.
Independent static review checked both pins and compared every fixture with v1.
The original v1 corpus and failed real study remain unchanged.

Parent-authored entry-point tests check exact frozen pins, malformed/duplicate/null
scope intake, omitted goals, changed pins, explicit empty v1 flags, symlink refusal
and absence of artifact/journal creation or provider dispatch on refusal. A
coordinated shortened fixture plus matching replacement scope/corpus digest still
refuses because it cannot satisfy the required long witness. Caller pins bind
reviewed bytes; they are not authenticated organizational policy signatures.

## Executed offline behaviors

`TestDocumentLengthCLIOracle` uses an intentionally content-free generator and
fake support reviewer. It exercises the real command entry point and private
artifact path with Unicode-whitespace text:

| Case | Actual fields | Result |
|---|---:|---|
| Frozen v1 freeform range100..200 | 86 | Held |
| V2 advisory freeform range100..200 | 86 | Warning, candidate checked |
| V2 required long range1200..1600 | 86 | Held |
| V2 advisory within range | 150 | Length pass |
| V2 required within range | 1200 | Length pass |
| V2 advisory above range | 201 | Warning, deviation+1 |
| V2 empty response before assessment | Unavailable | Held, no length observation |

All semantic goals remain unknown; acceptance, evaluated-negative and publication
flags remain false. The1200-field fake proves routing only, not useful long output.
None of these fakes replay or upgrade the real86-word result. Private canonical
per-goal fitness profiles/results are digest-bound; artifact substitution refuses.
The count rule is `markdown-whitespace-fields/v1`, not a universal linguistic count.

`TestDocumentV1FrozenWireCompatibility` compares actual preparation and held-receipt
byte fingerprints with the exact892b9f5 command in a disposable external module.
Its dependencies use the unchanged local library trees. No source checkout or
root module dependency changed. This is v1 wire compatibility, not provider proof.

Go/CUE same-byte tests cover required/advisory reasons,4096/4097 UTF-8 byte limits,
multibyte content, missing/null fields and ASCII/Unicode whitespace. Actual Go
intake rejects duplicate JSON keys; CUE's JSON importer deduplicates them, so the
wire decoder remains a required enforcement layer. Actual v2 early-held, assessed
held, warning and pass receipts are checked with `cue vet -c`.
Assessed receipts also reject counts inconsistent with pass/warning/hold and
incorrect signed deviations. This arithmetic consistency is not authentication;
consumers must still verify the bound Markdown, scope and version.

## Findings retained and corrected

The first source review found Go's2048-byte cap versus unbounded CUE and Unicode
whitespace disagreement. Red/green boundary tests now enforce4096 bytes and the
same trim rule. The first receipt test found an optional-field CUE failure; a
presence-guard attempt exposed a dependency cycle. An explicit state disjunction
passes the actual receipt cases. Final review also identified missing arithmetic
constraints; forged count-state and deviation cases now refuse. The first full gate retained that receipt failure;
the corrected gate passed. Final source-bound gate evidence is in the receipt.
Fresh-index strict verification proves HP-10's structural contract and entry-point/
test reachability. It is the only claimed row of ten; Fathom runtime attestation
remains unknown. Actual runtime evidence comes from the separately recorded full
gate, not a static reachability claim.

All five exported roots and `go.mod` are unchanged from892b9f5. The prior four-target
[consumer run](../2026-10-24-integration/README.md) still applies to those identical
library trees; it was not rerun or upgraded to live-consumer proof here.

## Still open

No provider, credential, hook, global configuration or installation action occurred
in this qualification. No new paid study was frozen or executed. The original
failed/uncertain spend remains in its stage accounting. The36-cell live cohort,
independent semantic/negative reviews, paired Jev economics, broader session/group/
selection/disclosure work and HP-A14/A18 remain open. UI implementation stays gated.
