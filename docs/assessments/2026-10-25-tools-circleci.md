# CircleCI: native reference evidence under T-R7

The canonical five-subject reading now measures L2 excellent (was good):
48/49 definitions, 74/75 references, reference precision .9867 and recall 1.
Counts and extraction are unchanged; the reference authority changes from C
to A after Q6/T-R7. Canonical L1 remains good. The separate three-subject
T-R3 holdout reads 11/11 definitions and 8/8 references, L1/L2 excellent.
The holdout shard never enters the canonical shard. No extractor, shared
engine, epoch, generated surface or plan checkbox changes.

T-R3's immutable matrix was committed at f6e47f021 before native execution:
default/false/true cron on cimg-python and cimg-node, default on scikit-learn.
Every registered case succeeds. The adapter refuses dirty or wrong-pin source,
missing/unregistered configs, parse failures and unsuccessful cases. Disabled
workflows are preserved; no configuration, declaration or orb is replaced.
Canonical cimg-base's monthly-release remains a valid declaration disabled
by its original default input, explaining its one definition/reference extra.
The fresh-subject union tests the input method without tuning the extractor
on exposed subjects.

## Native population and divergence

The comparison protocol at 3a7d13e34 preceded all native queries. The final
reference instrument was committed at 953c6b487 before the full rerun.
CircleCI's own Apache-2.0 YAML language server 0.54.0 is pinned by source,
release archive checksum and executable checksum in lsp-protocol-r2.json.
Independent source positions exactly match the existing Go walk's local pairs
in every file. They only locate queries; native misses never fall back to YAML.
The wrapper fails on an incomplete population or a missing native reference.

Across all five canonical and three holdout configurations, native references
attest 34/34 local uses/pairs: 27 commands and seven executors. Native definition
agrees on 33/34. Cypress's pack-workflows step (zero-based line 158) defines
to the same-name job at line 148, rather than the command at line 74. Querying
references on the command returns the correct use. All sanitized native
definition and reference target ranges are committed in lsp-comparison-r2.json.
Both readings disclose this divergence through the grade driver's `-note`.
Two qualified orb uses resolve externally and remain outside local scoring.

T-R7's compiler cross-check agrees on 3/3 representative bindings: check's
linux executor fields, check's inlined store_results command, and Cypress's
same-name pack-workflows command expansion. Full expected/actual fragments
are in compiler-crosscheck-t-r7.json; compiler_crosscheck.py reproduces them.
No pipeline, shell body, container or Docker executes. The server has an
isolated home/cache and receives no token.

## Candidate audit and reproducibility

| Candidate | Version | Result |
|---|---|---|
| CircleCI CLI config process | 1.7.0 | All original and registered inputs run; native workflow/job/requires pairs. Commands/executors are inlined, so their source identity is measured with the vendor server. |
| CircleCI CLI config validate | 1.7.0 | Help exposes validation, without the source identity pairs being measured. Acceptance alone is insufficient. |
| CircleCI YAML language server | 0.54.0, source 9c9c4cedb448caa2444bc89672233e4dae66eeea | Full native population measured; reference bindings admitted A by Q6/T-R7. Definition's one namespace disagreement is disclosed, not used as fallback truth. |
| Compiler ablation | 1.7.0 | Previously proposed but unimplemented; no longer necessary to attest these measured reference bindings. |
| Independent yaml.v3 source walk | v3.0.1 | Position/pair cross-check only in the native instrument; by itself C under P-T1. |

T-R3.md records exact build, native population, cross-check and grade commands.
native_refs.py runs the full population; grade_native.py emits separate fresh
readings and retimes both three times. Canonical timing is 11.43 MB/CPU-s
(median of 11.43, 14.62, 10.15); holdout is 15.00 (15.00, 15.12, 14.04).
These small, warm extractions do not establish dialect-wide throughput.
All eight Python tests, bench adapter race tests, release-notes check and
merge-debris check pass. X-R2 checks are repeated before READY.

Decision models: none. Native bindings, input identity and deterministic
comparisons require no model judgment.
