# Written-unit native comparison contract (2026-11-15)

This contract is specified before this lane's native run. Native truth consumes
only Jenkins ModelAST, never fathom nodes or the previous Groovy walk's labels.
The existing 23-input canonical population and bytes stay fixed. This is a
partial diagnostic, not a new native reading or an author-labelled oracle.

A written L1 unit is an explicit pipeline, named stage, agent, stages, environment,
post, parameters, options or steps declaration; script-level methods and library
imports also belong to the existing dialect population. Name, kind, start and end
line must match exactly. A section name is qualified by the nearest enclosing
stage name, not by a candidate-provided owner. L2 is a multiset of written
(caller definition, bare callee) pairs; repeated occurrences remain repeated.
DSL declaration keywords are not reference units, as specified by the existing
written-unit scope. A native refusal or missing model never removes a candidate
from the population.

`owner_compare.py` uses fixed owner class roles for pipeline/stage and the fixed
section classes documented in its `DECLARATIONS` table. It retains native ranges
without repairing them from candidate output. Arguments, structural nodes,
branches, opaque bodies and unknown classes are accounted for separately.
The sum of these categories must equal the raw model node population.
A call is compared only when its owner ancestry reaches a represented written
declaration without crossing a branch whose written owner cannot be established.

The pinned [owner parser](https://github.com/jenkinsci/pipeline-model-definition-plugin/blob/6e7193cec599b402ee79ee9afc5b8a417eb43967/pipeline-model-definition/src/main/groovy/org/jenkinsci/plugins/pipeline/modeldefinition/parser/ModelParser.groovy)
`parseStage` converts `steps` to `parseBranch("default", block)`, and parallel
branches also use `ModelASTBranch`; post conditions create default branches too.
A branch is not an independently certified written `steps` declaration. Its
block-body range is not silently substituted for the full declaration range.
Calls below such a branch have unresolved written ownership in this projection.
`ModelASTScriptBlock` and `ModelASTWhenExpression` have opaque bodies. The old
instrument retains only a `name` field, losing structural labels such as a post
condition. It does not enumerate script-level methods or library annotations.
Unknown classes fail completeness rather than extending the role map implicitly.

Consequently full L1/L2 coverage is false even on an accepted model. The probe
can measure represented native units and exact candidate intersections, but
cannot promote that intersection to a full-population metric. A full comparison
needs independently evidenced written steps ranges/owners and every omitted
written call, function and library unit; a new syntax walk would remain C under
P-T1. There is no native execution, runtime path coverage or L3 claim here.

Controls: `test_owner_compare.py` independently specifies exact tuples, duplicate
multiplicity, changed name/range/owner, a refused file, unknown classes and opaque
bodies. Input 128+1 and node 100,000+1 refuse the entire supplied receipt through
the existing validator. Changing candidate predictions cannot alter native truth.
The diagnostic's limits differ from the owner process's per-file node refusal
and post-write raw parsing bound; see NATIVE.md.

This probe's repeated written-call occurrences are deliberately not a new L2
metric: the product reports caller/callee edges, which can collapse repetitions.
An intersection of these populations must disclose multiplicity and never be
reported as canonical L2 recall. Candidate-side tuples are derived from the
public reader only after the native projection is fixed. No unmatched candidate
unit is labelled false merely because the native model omits it.


## Issue 211 first correction (2026-11-16)

The original `comparator-red.log` records `ModuleNotFoundError`, not an
assertion failure. It does not establish initial comparator TDD. That original
file is preserved unchanged. The later keyword controls and four historical
mutants establish their own failures, not earlier chronology. The correction
assertion RED precedes this repair and covers existing success output followed
by refusal, malformed positions and undefined caller ownership using authored
mock owner JSON only.

Both comparison entry points now invalidate previous output before reading the
unit and remove partial output on refusal. Refusals are structured as
`comparison_refused`; bounded JSON reads close their handles. Candidate symbol
and reference positions require exact integers (excluding booleans), ordered
in-source ranges and a unique defined caller whose range contains the call.
Invalid records refuse the entire comparison before any receipt is published;
no fallback caller string, coercion or partial-success receipt is used.

There is a historical hook evidence gap: current inspection cannot attest the
hooks active for the original commit. Required existing repository hooks were
restored using installed `lefthook install` before the repair commit; current
hook evidence is separate from that historical claim. Global gitleaks policy
is unchanged. All six `search_refresh` entries now disclose `UNKNOWN` historical
retrieval times because authentic timestamps were not retained. Existing URLs
and fetched-byte hashes are unchanged, and purposes are explicit. No mtime
proxy, page refetch, reconstructed receipt or saved-page transport was used.

This correction runs only authored Python controls. It does not rerun native,
SDK, canonical population or holdout work. The retained 23/23 acceptance, 753
nodes, 129/163 declaration intersection, 34 excluded step branches and six
opaque bodies remain historical results. Full L1/L2 remains HOLD; existing
L3 C-good, scope, denominator and support bars are unchanged. Earlier race
receipts remain historical evidence; no fresh race, gate or support claim is made.
