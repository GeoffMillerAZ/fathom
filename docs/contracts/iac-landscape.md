# IaC primary-source landscape: #270 / HZ-0

Status: complete contract draft for independent criticism, not dispatch approval.
Source base: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`. Only this contract is changed.
No implementation, raw population, native run, measured RED, gate or row claim.
`READY` means the immutable draft is ready to review. Input/grant holds below remain.

## Governing intent

Router: `docs/intent/README.md`; P2/P3/P20/P22; I38/I56/I62;
ADR-091 decisions 2/4/6/7, ADR-092 decisions 7/8/10, ADR-099,
ADR-106 decisions 1/2. Catalogs are local read-only inputs, values remain
withheld in the product, plans are bench-only, no added door package, table,
cache, hook or tool. No kernel or future evaluator is commissioned. W2-1
settles the fired LL-M1 trigger at 80 door / 81 consumer packages with no
headroom. No evidence here fires the other revisit triggers. F97's broader
identity stays deferred. Additive API/JSON changes retain compatibility review.
W3-1/4 house rules apply to additions: named closed types/constants, comparable
structured identity, explicit snake_case JSON, recursively nonnil result slices,
context only for I/O and typed refusal/limit errors. No future fields.

## Decisions and alternatives

- D1 Recommended: audit each design claim against a pinned primary source and
  retain unknowns. Alternative: copy search snippets or historical prose as
  current evidence, rejected by HZ-0 and I38.
- D2 Recommended: count unique (platform, rule id) in a complete pinned policy
  inventory, reporting both per-platform and deduplicated totals. Alternative:
  call a marketing total a measured census, rejected by P22 and HZ-0.
- D3 Recommended: a reading owner gathers evidence and an exclusive design owner
  applies the exact prior-tag updates below. Alternative: infer permission to
  edit design from a one-file reading issue, forbidden by CT-D2/5 and W4-3.
- D4 Recommended: HZ-1 completes first and any below-floor stop is resolved before
  this lane. Alternative: use this landscape to bypass HZ-1, rejected by
  change-hazards plan introduction and stop rules.

## Source finding and exact paths

At this base `docs/design/change-hazards.md` contains no landscape table or
per-tool `[prior]` rows. The two literal occurrences are the tag legend (§0)
and R7 (§13). A contract that simply says remove table tags would invent sites.
#270 owns only new `docs/bench/readings/change-hazards/landscape.md`.
No schema, plan, product or external-source checkout is changed by #270.
The integrator/design owner needs an explicit serial grant for the existing
`docs/design/change-hazards.md` §4 and §13 plus its opening tag legend.
That grant is requested separately; it is not issued by this contract.

## Complete primary-source manifest

Embed a fenced JSON `manifest` in the reading, schema_version=1, freeze_commit,
cutoff_utc, design_base and `claims:[]`. Each claim has claim_id, tool,
claim_kind (named closed status/licence/count/capability), design_anchor,
platform, primary_url, source_revision or response_etag, capture_utc,
raw_sha256, raw_workspace_ref, locator, result_state (verified/unknown),
result and reason. Unknown has a reason and no asserted result. Raw bytes stay
under workspace; use relative artifact references, never a private home path.
Record HTTP failures/redirects/archival time rather than infer a vanished tool.
Live source identities are absent today; these URLs are acquisition targets,
not verified status, permission or count evidence.

Minimum fixed claim roster, derived from design §4 K1/K2/K3 and its correction
paragraph, §8 and §13 R7:

| Claim ids / required claims | Primary acquisition targets |
|---|---|
| terrascan-status, terrascan-licence, terrascan-count | https://github.com/tenable/terrascan ; repository metadata, LICENSE and policy tree at a single commit |
| datree-status, datree-licence, datree-count | https://github.com/datreeio/datree ; archive metadata, maintainer notice, LICENSE and available policy inventory |
| trivy-licence, trivy-count, trivy-resolution | https://github.com/aquasecurity/trivy ; pinned LICENSE, IaC rule inventory and resolver source; resolve the exact owned upstream policy repository from that pin |
| checkov-licence, checkov-count, checkov-unknown | https://github.com/bridgecrewio/checkov ; LICENSE, framework rule inventory and UNKNOWN/reporting source at a pin |
| kics-licence, kics-count | https://github.com/Checkmarx/kics ; LICENSE and query tree at a pin |
| tflint-aws-licence, tflint-aws-count, tflint-enum-source | https://github.com/terraform-linters/tflint-ruleset-aws ; LICENSE, generated rule inventory and generator source |
| cfn-guard-licence, cfn-guard-states | https://github.com/aws-cloudformation/cloudformation-guard ; LICENSE and PASS/FAIL/SKIP primary specification |
| cdk-offline-diff | #271's pinned CLI source, archive identity and native witness; catalog agreement is a separate #309 reading |
| semgrep-hooks, snyk-hooks, trivy-mcp | Maintainer documentation linked from each pinned official project; publish exact page/version/digest, not the design's existing web tag |
| terraform-ls-deprecated, kube-linter-dangling | https://github.com/hashicorp/terraform-ls and https://github.com/stackrox/kube-linter ; exact diagnostics/check definitions at a pin |

A maintainer's status notice establishes a maintainer assertion. An API archived
flag establishes that flag as of capture. Neither proves business closure by
itself. Keep those columns distinct. Licence result states SPDX expression,
exact file path, pin and governing exceptions. A statement of current project
licence is not a decision on relicensing this product or catalog publication;
OQ4 remains untouched. Keep quotations minimal and link exact primary bytes.

For each count tool above enumerate the platform list from its pinned source
manifest, then every production rule/query. Preserve disabled/deprecated and
unclassifiable rows with reasons and report them separately. Exclude tests,
fixtures, duplicate generated copies and helper modules by a published path
predicate frozen before counting. Record source total, included, excluded,
unclassified and each per-platform count. A shared rule on two platforms
counts once on each platform but once in the deduplicated union. Do not sum
platform counts and label it a global unique-rule total. Missing inventory,
unreadable path or inaccessible private corpus is unknown or a labelled lower
bound, never zero or a complete total. Marketing minima such as 750+ are
separate quoted claims, never an executable denominator. Do not invent a
cross-tool normalized meaning of a rule.

## Full acceptance and exact design update

1. Freeze the complete claim/source manifest before adjudication. Verify every
   digest and source locator, publish all mandatory status/licence/count rows
   including unavailable evidence. Independently reproduce count predicates on
   the frozen inventories. All required claims need primary evidence to PASS;
   unknowns yield INCOMPLETE with honest lower bounds. False old claims are
   corrected, not retained under a new tag. HZ-0 does not require favorable news.
2. Design owner adds §4 subsection `Primary-source landscape` immediately after
   the correction paragraph, linking the reading and its manifest digest.
   Copy the verified status/licence/per-platform results, each with its primary
   source locator and as-of date. Unknown results are explicitly unknown.
3. Replace opening tag legend with: `Evidence tags: web means a primary source
   checked during the cited review. Current landscape evidence, dates and
   limitations are recorded in the HZ-0 reading linked in section 4.`
   This removes the unused prior-tag definition without claiming old web tags
   were refreshed.
4. Replace §13 R7 with: `R7 Landscape evidence ages. The primary-source landscape
   in section 4 names its capture dates, complete count populations and unknowns.
   Recheck the affected claim before using it as a current capability assertion.`
   Add a reading link there. No other risk/decision/phase/threshold is changed.
5. Independent critic compares the reading, raw manifest and exact design diff,
   confirms zero literal prior tags in that design, and rejects presence-only
   evidence. Only after the serial design commit is included in the proven
   lane head may `HZ-0 verdict: PASS` be claimed. Missing design grant/update
   yields `HZ-0 verdict: INCOMPLETE`; a nonempty reading cannot close HZ-0.

The two replacement paragraphs above are frozen wording. Findings may require
changes to other claims in §4, only at the explicitly granted design-owner turn.
A conflicting accepted decision or a need to change other design sections is a
new exact request, not inherited permission. No unrelated prior tags elsewhere
in the repository are in scope.

## Prospective acceptance checks

This reading-only issue gets no new production test or executable dependency.
The lane captures these independently checked assertions in the reading:

| Named check in reading | Exact assertions |
|---|---|
| landscape_manifest_complete | Every claim id above appears exactly once; each verified result has reachable captured primary bytes matching digest and locator; all status/licence/count claims use one identified revision per tool; missing/failing input is listed and makes PASS impossible. |
| landscape_count_reproduction | Seed a scratch inventory with one shared rule on two platforms, one test fixture and one unreadable path: per-platform 1/1, deduplicated 1, excluded fixture 1, unreadable 1/lower-bound, no complete total. Then reproduce every real tool's count from its full raw population. Scratch fixture result alone is not HZ-0. |
| landscape_design_join | Design-base and reading commit/digest match; exact three design regions above changed under the design owner's grant; every inserted result matches primary manifest; literal prior-tag search returns no matches; no plan box or threshold was changed. |

Use `rg -n -F '[prior]' docs/design/change-hazards.md` and require exit 1,
not an ignored command status. Use actual primary-read/census receipts plus
independent review for the other checks; `test -s` and lexical verify only
establish artifact form. Scratch ingest/verify cannot replace the assertions.
No such reading/check ran here. The contract itself changes no live claims.

## Input and reservation holds

HZ-1 result, raw primary captures/revisions/digests, complete inventories and
exclusive design-owner grant are missing. Author A receives the exact design
reservation request. The reading may be drafted honestly while unknown, but
no source fact or HZ-0 acceptance follows from contract presence. No architect
policy choice is requested: these rules follow HZ-0, I38 and CT-D2/5.

## Decision models

None. Primary citations, counts and owner turns are deterministic evidence
bookkeeping. Provider none; opt out by leaving HZ-0 unclaimed.
