# COV-1 dependency completeness census

Status: harness controls passed. No baseline reader run yet. COV-1 remains unchecked.

The preregistration in `docs/design/dependency-edges.md` fixes COV-D1 through
COV-D11. The original DEP-M1 and DEP-M2 populations remain development inputs.
The COV-0 pins fix the sealed holdout. No product reader, oracle semantics,
admission bar or public dependency model changes in this lane.

## Accounting protocol

B1 and B2 use `depbench.Score`. Zero complete files and complete files without
unique native truth are inconclusive, separately from positive missing or wrong
edges. The independent repository inventory also refutes complete external claims
that name a repository package or module, including files with native truth.
Native capture precedes construction of the product reader.

Every unknown contributes one occurrence. Distinct-file counts deduplicate a
bucket within each file. A file's blocking set is a conjunction: closing just one
of two buckets does not unlock it. Oracle coverage and gaps are reported alongside
reader buckets. Source forms without exact evidence remain `unknown-shape`.
Identity-only means every unknown has a corresponding external edge with a known
identity and no version. No other unknown code qualifies by inference.

The greedy table measures each marginal unlock after all previous buckets close.
A zero-unlock prerequisite is admitted only when an explicit remaining blocking set within the five-bucket cap
reaches the 2% development-file bar. At most five buckets are selected. This
mechanical shortlist does not authorize an oracle extension or a reader change.

Closure starts at each selected file and traverses all local reader edges,
including configuration references. Targets beyond the selection are read with
the unchanged reader against the same tracked repository view. Directory targets
expand to regular tracked files directly in that directory, not descendants.
Cycles are visited once. Configuration and other files without a measured reader
block closure. No config-file exemption or trust opt-in is used. Report closures
blocked solely by unmeasured files separately from measured and mixed blockers.
The denominator remains selected files, never all reached targets.

Holdout per-file observations and truth stay in memory. Compiler traces and file
lists exist only in a private transient directory and are removed on exit. The
public artifact exposes only the COV-D2 allowlist. The COV-D3 aggregate counts are
retained separately for the final measurement, outside reader-lane records.
The repeat and relocation are part of this baseline campaign. The final holdout
reading is not performed here.

## Reproduction

Source the lane environment before each command. Prepare fresh detached clean
clones from the pinned local caches using
`python3.13 -I -B -S calibration/oracles/dep-coverage/cov_run.py prepare`.
The thin runner's `--help` lists the required destination and source arguments.
No subject dependency installation or network fetch is needed.

The completed reading must record the exact leased test commands, aggregate
checksum, artifact location, COV-D7 and COV-D11 results and unresolved proof limits.
Independent rerun and the foreman's strict gate are required before COV-1 is claimed.
