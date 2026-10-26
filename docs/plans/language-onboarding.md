# Language onboarding: rejected experiment, guidance-only closeout

Controller ruling LH-R2 rejects the replay experiment. There are **no active build tasks**
in this work order. No checkbox is claimed. The previous open experiment entries are
retired, rather than left available for a worker to execute after rejection.

| Retired task reference | Disposition |
|---|---|
| LH-1: freeze replay protocol | Rejected by LH-R2; held-in experiment does not justify tooling. |
| LH-2: baseline arm trials | Rejected with the experiment; no trials fabricated. |
| LH-3: pre-oracle feasibility null | Recorded in the design; no box flipped or gate claim. |
| LH-4: new-predicate trials | Rejected by LH-R2; no scorer or runtime predicate. |
| LH-5: computed trial outcome | Rejected with the experiment; no KEEP result fabricated. |

The [design](../design/language-onboarding.md) and
[unnumbered decision draft](../design/language-onboarding-decision-draft.md) contain the
null and wins-if conditions. The [assessment](../assessments/2026-10-25-language-onboarding.md)
retains the public source census, including the pre-first-reading MATLAB self-repair.
The [decision data](../assessments/data/2026-10-25-language-onboarding-criteria.json)
records rejection, not an executable criterion.

LH-R2's authorized deliverable is checklist guidance in recipe §13 and the existing
add-language skill, reusing #7 E2 tests where landed and disclosing coverage gaps. It creates
no new gated implementation work, support level, runtime behavior or record identifier.

## Controller-owned hygiene follow-up

Tracked here under issue #16, pending a separate controller-filed implementation brief.
The controller owns filing and assigning the following units, and confirming they land
before the preflight wins-if condition is evaluated. No worker is assigned by this document,
no external issue has been filed by this lane, and LH-R2 does not authorize implementation here.

| Unit | Required existing seam and proof | Owner / state |
|---|---|---|
| Bytecode hygiene | Add `__pycache__/` and `*.py[co]` to `.gitignore`; cover force-added bytecode in `scripts/check-merge-debris.sh` with red/green cases in `scripts/check-merge-debris_test.sh`. | Controller: file brief and assign worker; pending. |
| Calibration literal hygiene | Cover home/volume-rooted literal shapes under `calibration/` in the existing `internal/guardspec` scan, with neutral red fixtures and a green whole-tracked-tree run. | Controller: file brief and assign worker; pending. |
