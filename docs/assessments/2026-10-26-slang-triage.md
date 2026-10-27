# Slang triage and frozen subject proposal

Slang triage is authorized under G-R1, while implementation is held on owner
question NP-Q5. This branch records pinned candidates and a metadata-only
population rule before measurement. It changes no dialect, grammar lock/blob,
canonical corpus, reading, queue row, plan checkbox or generated tracker.
There is no Slang support-level or ceiling claim.

## Grammar candidates

| Candidate | Pin | Status |
|---|---|---|
| tree-sitter-grammars/tree-sitter-slang | v0.3.1, `c7ab1ee2bea2ac431fa78d72b80d8957d5a5bcd3` | MIT; archive and license verified; untried |
| Same upstream | master snapshot `1dbcc4abc7b3cdd663eb03d93031167d6ed19f56` | MIT; archive and license verified; untried |
| Vostranox/tree-sitter-slang | `e42d9b7d4ea82105218aa57666355e7ab5a85216` | MIT; two commits ahead, grammar/parser/corpus changes; untried |

[The inventory](../../calibration/oracles/slang/grammar-candidates.json) pins
archive and license hashes. The old theHamsta address redirects to the same
upstream, not another candidate. The two other discovered forks are behind or
old and divergent; their exact comparisons remain in the receipt. Existing
HLSL/CUDA/C++ grammars are precedents in the same extension family, not evidence
that stock C++ understands Slang. No grammar has been selected or measured.

## Native oracle routes

Slang v2026.20 is pinned at `d9ede2dd5836afc0e9586da18d9236b171d0faa8`,
Apache-2.0 with LLVM Exceptions. Its macOS ARM64 archive digest is published by
the release API; no compiler binary was downloaded or run.

The strongest source-audited candidates are the public declaration-reflection
tree, declaration source-location API, module dependency paths, AST dumps and
AST-backed `slangd` document symbols/definition lookup. Exact end ranges, written
call coverage, resolved file edges and typed-edge coverage remain unproved.
Layout reflection and executed-function coverage are useful existing features,
not substitutes for complete static edges.

[Candidate fields and source links](../../calibration/oracles/slang/oracle-candidates.json)
separate documented capability from unknown output fields and untried probes.
[The search receipt](../../calibration/oracles/slang/oracle-search.json) covers
the compiler, SCIP and LSIF lists/GitHub queries, language servers, registries
and runtime instrumentation. Search misses remain explicit. The initial
headerless crates.io request returned HTTP 403 because a User-Agent was missing.
The retry with a descriptive User-Agent returned HTTP 200. Registry records
now include tree-sitter-slang, shader-slang Rust bindings and PyPI slangpy;
compiler bindings remain in the own-toolchain oracle family.
No semantic-tool ceiling follows. Reconstructed contexts cap at tier B; tier A
requires independently proven native semantic output and untranslated subject
context. Same-grammar queries cap at tier C.

## Search preregistration and its boundary

Links below publish X-R9a ID-only derivatives. Dated commit references refer
to the original private r1 records, not r2 ancestry. The final section records
original and derivative hashes and the critic attestations.

The r2 queries matched primary-language Slang repositories and name/description
matches, plus named seeds. They missed Slang inside other language projects.
M3 is addressed by the dated [r3 rule](../../calibration/oracles/slang/search-rule-r3.json)
committed at `4c606debf` before authenticated code searches. It adds
`extension:slang ParameterBlock` and `extension:slang "[shader("`, retaining all
previous pins and seven review seeds. The six nvpro projects contribute 193
files. The excluded review seed (repository ID 688691303) has 62 files and a CC-BY-SA root grant and are excluded
by the preregistered OSI-license gate.

Paged code results changed totals on three retries. The dated
[single-page amendment](../../calibration/oracles/slang/search-protocol-r3a.json)
was committed at `bcb01b43c` before tighter slices. It preserves queries,
eligibility and every observed repository. The completed
[ParameterBlock receipt](../../calibration/oracles/slang/code-search-r3a.json)
certifies 109 disjoint size leaves covering the full registered integer domain:
198 requests, 195 HTTP 200 responses and three rate-limit HTTP 403 responses.
The leaves contain 4,963 distinct repository/path/blob observations from 333
repositories. All 373 repositories observed across current and historical code
requests remain in the candidate universe. The old root count of 2,260 and
leaf sum of 4,963 differ for an undetermined reason. This is no atomic snapshot.

G-R10 authorizes a narrower shader-attribute boundary. That route is explicitly
**incomplete**: its root reported **39,040 files**. Only **one distinct page** was read: **page 1, 100 of 39,040 reported file
hits from 51 repositories**. Five requests with narrowing size ranges returned
byte-identical responses. Exact queries remain in the receipt; no r3a
shader-attribute requests followed. GitHub's authenticated code-search limit
was **10 requests per minute**. This work does not enumerate every Slang project
or establish a reach-complete population. Any future ceiling must repeat this
boundary in plain words.

Before new requests, the dated [README/topic rule](../../calibration/oracles/slang/search-rule-r4.json)
was committed at `f052015bd`. All four exact queries completed across six
preregistered disjoint star bands: `"shader-slang" in:readme`,
`slang shader in:readme`, `topic:slang`, and `topic:shader-slang`, each at
100 or more stars. [The receipt](../../calibration/oracles/slang/repository-search-r4.json)
contains 24 certified one-page responses, 123 query hits and 103 unique public
repositories. The final boundary combines these routes with the earlier
primary-language/name/description queries, named seeds and every historical
observation. It can still miss projects that match none of them.

The old r1/r2 inventories and incomplete r3 receipt remain historical. The
r3 rule's inherited `related_source_followup_unexecuted` wording describes the
older registry gap, not current execution. The registry retry is recorded in
`oracle-search.json`; its 403 was a missing request header, not transient.

## Frozen population before measurement

[The current inventory](../../calibration/oracles/slang/subject-search-r4.json)
disposes **466 repositories**: **48 metadata-eligible**, 339 below reach,
64 zero-population, eight wrong-language, six license-excluded and one
compiler diagnostic. All 18 r2 pins and all 22 previously verified new pins
remain fixed, including the six nvpro projects (193 files) and 16 additional
projects (1,003 files). Every discovery is retained, including failed search
attempts. The same dated reach, root-license and whole-project size rules apply.

[The population rule](../../calibration/oracles/slang/pool-rule.json) now has
**47 primary projects, 2,752 files, 2,712 distinct Git blobs and
14,417,219 selected-suffix bytes**. All 40 duplicate occurrences remain counted.
Falcor 9.0 remains the largest whole-project holdout with **461 files,
461 distinct blobs and 2,951,994 bytes**. It is 14.35% of eligible file
occurrences. Complete per-project pins, counts, bytes and tree hashes are in
the rule. These are metadata counts, not measured parser denominators.

Every tracked regular exact-lowercase `.slang` or `.slangh` file enters its
whole eligible pinned repository. The ceiling is 5,000 selected files and
50,000,000 selected blob bytes per project. No source body or parser/compiler
outcome selected membership. Tool fixtures and vendored files remain included:
cloc contributes its one GPU Slang fixture and Gigi its two vendored Slang files.
Pinned primary documentation/history establishes language identity. Gigi and
Area-ReSTIR have manually assessed BSD-3-Clause root source-code grants;
separate marks/assets and third-party terms still need per-file admission review.

There are shared Git blobs between projects. Falcor shares **17** with
DQLin/ReSTIR_PT and **239** with Area-ReSTIR. These pair counts may overlap;
they are not added as unique holdout coverage. Twelve pairs across all complete
candidate trees share blobs. The pool records intersections without removing
any occurrence. The holdout is therefore not fully independent of the primary,
and a future reading must retain this limitation rather than claim zero overlap.

Four primary projects carry **587 selected files (21.3% of primary) under
vendored Falcor trees**: Area-ReSTIR 255 of 440, ReSTIR_PT 166 of 298,
VolumetricReSTIRRelease 120 of 220, and the admitted ReSTIR sample project
46 of 46. Exact blob intersection understates this lineage. The counts come
from the same pinned complete Git trees, using an exact `Falcor` directory
component, without reading shader bodies.

Future readings must report holdout beside primary rates **with all four
projects** (2,752 files) and **without those four whole projects** (1,748 files).
Report counts, ERROR-or-MISSING and faults on the same frozen grammar. This
comparison is diagnostic. Membership does not change, and whole-primary and
whole-holdout gates still govern every claim.

The holdout stays outside primary aggregation and canonical corpus shards.
Primary and holdout must **each** meet at most **5% ERROR-or-MISSING files** and
at most **0.1% faults**, on the same frozen grammar revision. Report both rates
and per-project counts beside any proposed level or change note. The weaker
result caps the claim. Per-file license terms and raw checkout fidelity remain
admission checks before measurement. On conflict, stop and report rather than
silently trim. Other suffixes, embedded strings, symlink targets and submodule
contents remain uncovered. Grammar parse rates, oracle coverage and performance
are unmeasured.


## Disposition and next gate

ADR-048's data-only dialect and measured-level rules, I19/X-R5 privacy, I23/I24
edge truth and I25 readable-file accounting apply. No documented revisit trigger
is shown to have fired. No decision model or new runtime service is proposed.

The [queue entry](../../calibration/oracles/slang/queue-proposal.json) is a
proposal only, targeting L2 if later measurement proves it. The owner answers
NP-Q5 before implementation. After approval, follow the [qualification plan and
reproduction commands](../../calibration/oracles/slang/README.md): license/raw
identity checks, separate primary and holdout native grammar screens, independent
oracle field and scope proofs, then product/WASM/performance/reading work. Both
5% and 0.1% gates apply separately to the two groups; there is no outcome-based
population change or template prepass proposal.

[The verification receipt](../../calibration/oracles/slang/verification.json) records
saved-response and source hashes, archive/embedded-commit/license checks,
JSON validation, repository checks and privacy review. None is a shader parse,
compiler execution or measured language level.

## Publication corrections

X-R9 keeps the 48 admitted subject slugs and pinned commits. All 418 other
discovered repositories are recorded only by numeric GitHub repository ID,
counts, disposition, discovery route and response hashes. Resolve an ID with
`GET /repositories/{id}`. Original name mappings and raw responses stay private.
No candidate or subject was removed.

The README/topic receipt also records four omitted requests after the dated
rule commit: three initial responses were overwritten around 23:19:28 to 23:19:32 UTC
and their outcomes are unknown. A manual 23:19:49 UTC check returned HTTP 200
and the same eight repository IDs as the certified leaf. There were 28 known
requests, including 24 certified leaves. The historical collector is disabled;
a private replacement creates an exclusive attempt directory and exclusively
creates each response file before any future use.

## Preregistration record after the squash

Under X-R9a, r2 publishes ID-only derivatives while the original r1 branch
remains on the lane machine, unpushed and never deleted. Only non-subject
identity representation changes. Search rules, queries, thresholds, results
and subject pins do not change. The squash carries no preregistration ordering.
The following original records and critic attestations carry that evidence.

| Preregistration file | Original r1 commit | Commit date | Original content SHA-256 | Published derivative SHA-256 | Attestation |
|---|---|---|---|---|---|
| search-rule-r2.json | `380f8801cd8b546703975bdb6ca087da85165804` | 2026-10-26T14:54:17-07:00 | `781f18c472b33df4173a8dc9ed8268f098adb7ec27fc6e8737fcb2cdca73541d` | `0620aee9c4998007e4e0bff01118d0a06874048f1b0c3e2c1ae89bcfa0ef21c6` | Round 2 |
| search-rule-r3.json | `4c606debf7a5870cc2dcc2a04934d33540e91c2f` | 2026-10-26T15:46:15-07:00 | `bafa9b8cdf6c4485a28084fa1bf347cb1f91890aa5fd410f3a1d05b3b7911ad8` | `26dde17cd3c5cd3821081eaeada4adcd010e9cb9f711280ed482121510821639` | Round 3 |
| search-protocol-r3a.json | `bcb01b43c9a2135c959ec8bd8f58d257db3aa11c` | 2026-10-26T15:56:40-07:00 | `b2896b47a70f57116a94afe144780b8c1cfef387c965b0c6fd847a42fd8f5aad` | `b2896b47a70f57116a94afe144780b8c1cfef387c965b0c6fd847a42fd8f5aad` | Round 3 |
| search-rule-r4.json | `f052015bd3f6a640f43b9257c8ae0975b4ae1bf0` | 2026-10-26T16:17:36-07:00 | `0b475fa273babfe964220b76da97c62c371b05a9cc26549b9a676b72f4e34b9f` | `0b475fa273babfe964220b76da97c62c371b05a9cc26549b9a676b72f4e34b9f` | Round 3 |

Controller binding critic round 2, reviewing `4dff9d4a4`, verified the r2
rule commit preceded its requests and its bytes were unchanged. Controller
binding critic round 3, reviewing `f739a34af`, verified that r3, the r3a
protocol amendment and the README/topic rule each preceded their requests,
and verified G-R10 in substance. Those reviews inspected the original r1
history and saved response times, before this privacy squash. The machine-readable
[provenance record](../../calibration/oracles/slang/preregistration-provenance.json)
records the same evidence. Frozen references inside the rules retain original
content hashes; mutable receipts bind the current published derivatives.
