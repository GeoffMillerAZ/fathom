# Slang triage

This is GPU Slang (`repository id 93882897`), not S-Lang scripting or the SystemVerilog
compiler named slang. The controller authorizes triage only. NP-Q5, the owner's
language-lane count decision, gates implementation. No shader has been parsed or
compiled, no compiler binary downloaded, and no support level or ceiling claimed.

## Frozen inputs

- [Grammar inventory](grammar-candidates.json) and [search](grammar-search.json):
  upstream v0.3.1, current upstream and a grammar-changing fork, each pinned to a
  full commit, downloaded archive hash and MIT license. HLSL/CUDA/C++ entries are
  existing in-tree precedents, not additional independent Slang grammars.
- [Oracle inventory](oracle-candidates.json) and [five-source search](oracle-search.json):
  Slang v2026.20, pinned source, published macOS ARM64 asset digest, source-audited
  compiler APIs/flags and official language-server methods. Every route is untried.
- [Dated discovery rule](search-rule-r3.json), [single-page amendment](search-protocol-r3a.json)
  and [G-R10 README/topic rule](search-rule-r4.json) were committed before their
  requests. [ParameterBlock](code-search-r3a.json) completed 109 disjoint size
  leaves with 4,963 distinct file observations from 333 repositories. All 373
  repositories from current and historical code requests remain candidates.
  [Four README/topic queries](repository-search-r4.json) completed 24 one-page
  star-band leaves, with 123 hits and 103 unique public repositories.
- The shader-attribute route is **incomplete** under G-R10: root `total_count`
  **39,040 files**, **one distinct page** (page 1: **100 of 39,040 reported file hits,
  51 repositories**) requested five times with narrowing size ranges that
  returned identical bytes. GitHub allowed **10 code requests/minute**. Exact
  probes are recorded in the receipt. No r3a shader-attribute requests followed. The full boundary combines
  the completed routes, old primary-language/name/description matches, seeds,
  and every prior discovery. It can miss projects matching none of them.
  Neither a reach-complete search nor an atomic GitHub snapshot is claimed.
  Any future ceiling must state this boundary plainly.
- [Current inventory](subject-search-r4.json) and [population rule](pool-rule.json)
  dispose every candidate without shader content or parse outcomes. Older
  inventories and the incomplete paged r3 receipt remain historical. The old
  ParameterBlock root count 2,260 differs from the leaf sum 4,963 for an
  undetermined reason. Preserve the observations rather than infer a cause.
- [Queue proposal](queue-proposal.json): controller-owned activation, target L2
  proposed. This does not edit the canonical queue or a plan row.

The selected pin rule prefers the recorded latest published release, otherwise
its exact default-branch snapshot. Previously recorded pins remain unchanged.
The metadata-only rule reserves the largest eligible whole project as a separate
holdout, with repository identity breaking ties. Every other eligible project is
primary, including tool fixtures and vendored Slang files. No new role exclusion
removes them. The compiler implementation/regression repository is diagnostic by
its preregistered role and also exceeds the selected-file ceiling.

The prospective primary is **47 projects, 2,752 files, 2,712 distinct Git blobs,
14,417,219 bytes**.
Forty duplicate blob occurrences remain counted.
The separate Falcor 9.0 holdout is **461 files, 461 distinct blobs,
2,951,994 bytes**, or 14.35% of eligible file occurrences. All 466 candidate
repositories have one disposition. The six nvpro projects (193 files) and
16 already-found additional projects (1,003 files) retain their pins.

Falcor shares 17 Git blobs with ReSTIR_PT and 239 with Area-ReSTIR. These
pair counts can overlap. No occurrence is removed. The holdout is not fully
independent of primary; report that limitation with future readings. All
per-project counts and intersections are in the population rule. These are
Git-tree metadata, not filesystem captures or measured parser denominators.
Root grants qualify the prospective metadata pool; per-file exceptions and
checkout fidelity remain admission checks. Stop and report any conflict.


Primary and holdout have separate gates: each must have at most 5% ERROR-or-MISSING
files and at most 0.1% faults on the same frozen grammar revision. Both rates
must accompany a proposed level or change note, and the weaker group caps the
claim. The holdout remains outside canonical shards and primary aggregation.

All tracked regular lowercase `.slang` and `.slangh` files enter each selected
whole repository. No parser-success or compiler-success filter is allowed.
Symlinks and gitlinks are counted separately and not followed; submodule contents
are uncovered. `.hlsl`, `.hlsli`, other suffixes and embedded host strings are
outside this initial proposal, not declared incompatible.

## Reproduce metadata discovery

Every search receipt names the exact request, bound, HTTP result and response
hash. Raw API responses can contain people and source paths; keep them private
under ignored `workspace/`, retaining only aggregate counts and hashes publicly.
For example, reproduce the pinned grammar source query and subject tree:

```sh
mkdir -p workspace/tmp/slang-triage
curl --fail --location --max-time 30 \
  https://api.github.com/repos/tree-sitter-grammars/tree-sitter-slang/commits/1dbcc4abc7b3cdd663eb03d93031167d6ed19f56 \
  --output workspace/tmp/slang-triage/grammar-commit.json
curl --fail --location --max-time 30 \
  'https://api.github.com/repos/NVIDIAGameWorks/Falcor/git/trees/759aad033ff610fb0d82c74f7e0a508d0096d5f2?recursive=1' \
  --output workspace/tmp/slang-triage/holdout-tree.json
shasum -a 256 workspace/tmp/slang-triage/holdout-tree.json
```

A fresh API response may differ in serialization or moving repository metadata.
The source commit and Git-tree/blob identities must still match; response hashes
identify the saved research response, not a promise of cross-run byte equality.

[Verification](verification.json) records the metadata/provenance checks and the
required repository checks; it is not a language reading.

Validate the committed metadata without downloading or executing a language tool:

```sh
for input in calibration/oracles/slang/*.json; do
  python3 -m json.tool "$input" > /dev/null || exit
done
```

## Qualification plan after owner approval

First confirm license exceptions and raw Git checkout identities without changing
membership. Build the three unmodified native grammar candidates from the pinned
archives and screen the whole frozen primary and separate whole holdout on the
same revision. Report ERROR-only, MISSING-only, combined failures and faults for
each candidate and subject; no failing file is removed. Apply the 5% and 0.1%
gates to each group separately. WASM and performance are later qualification,
not current evidence.

Prefer the compiler's public declaration-reflection tree and source-location API,
AST dump, and AST-backed `slangd` document symbols/definition lookup. Qualify
complete definitions/end ranges, written calls and explicit zero-call scope,
resolved import edges and source identity before writing an oracle adapter. The
existing langbench schemas, scope rules and tier rules apply. Reconstructed
contexts cap at tier B; a native executable alone does not grant tier A. The
same grammar queried differently caps at tier C. Reflection's layout types and
runtime `-trace-function-coverage` counters do not by themselves prove typed call
edges or complete static call coverage.

CLI probes recorded in the candidate inventory are hypotheses to test, not
successful commands or schema contracts. In particular, confirm whether
`-no-codegen` suppresses the IR phase before relying on any proposed IR dump.
Do not replace an untried route with a ceiling. No runtime service or decision
model is proposed, and no adapter, dialect, grammar registration or product code
is implemented here.

Original preregistration text remains frozen on the private, unpublished r1
branch, which must never be deleted. ID-only public derivatives are authorized
by X-R9a. The assessment records each original commit/date and both hashes,
plus the critic round 2 and round 3 attestations. The assessment also carries
the readable search boundary. Non-subject discovery identities are
public only as numeric GitHub repository IDs under X-R9.

Four primary projects carry **587 selected files under vendored Falcor trees**
(Area-ReSTIR 255, ReSTIR_PT 166, VolumetricReSTIRRelease 120, and the admitted
ReSTIR sample project 46). Future readings must report holdout beside primary
rates with and without these four whole projects. Membership and both whole
population gates remain unchanged. Exact blob overlap alone does not measure
all shared lineage.
