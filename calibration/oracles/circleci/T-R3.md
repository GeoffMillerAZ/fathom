# T-R3: fresh-subject parameter matrix

Protocol `parameters-t-r3.json` is committed before any oracle invocation on
these subject pins. It follows the controller's T-R3 ruling (2026-10-25).
The files and declared parameter types were inspected, not compiled or graded.

The two fresh cimg subjects each declare boolean `cron` with default false:
measure the default input and both boolean values, then union the compiler's
attested entities and workflow/job pairs. scikit-learn declares no pipeline
parameter, so its matrix is the default input alone. The pins are immutable;
source checkout identity must match the preregistration. A failed case fails
the subject, never silently narrows the input matrix. All three configurations
are measured whole. No declared workflow or job is deleted when disabled.
Public orbs are resolved by CircleCI itself; no stand-in is permitted. Only
`config process` executes, never a pipeline or one of its shell commands.

The corpus shard keeps the earlier subjects; these fresh subjects form the
holdout. Report a scope change explicitly beside any numeric comparison.
L1 uses the compiler's union at tier A; command/executor references remain C
until separate native ablation attests them. No tier is raised by this protocol.
No model judgment applies to these deterministic inputs or comparisons.

## Reproduce

Build the bench-only adapter with `go build` in this directory, then, for each
subject, run it twice (`-refs` on the second run), with `-subject`, `-subject-name`,
`-cli`, `-scratch`, and `-matrix calibration/oracles/circleci/parameters-t-r3.json`.
Pass the two oracle JSON files to `indexbench lang-oracle -dialect circleci`
without `-only`; write a separate holdout and give each subject its registered
name. Run `indexbench lang-retime -runs 3` with every recorded subject identity.
The committed holdout contains the three-run values and median.

CircleCI CLI 1.7.0's `circleci-cli_1.7.0_darwin_arm64.tar.gz` was checked against
its release checksums: SHA256
`c612876c351bb16de2f0fbabf81cd9a4cbcfc3a081f0be4aca27992e8be89e4f`.
The native compilation accepted every registered case. The holdout reads
11/11 definitions and 8/8 references, L2 (L1 excellent, L2 good); all earlier
canonical metrics remain unchanged. The 16.22 MB/CPU-s median was selected from
6.65, 16.22, and 17.00 on the development-machine class under nice 10, one job
at a time with the start load at most 6. No runtime workflow was started.
