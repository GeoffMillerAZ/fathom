# SCSS configured load-path gap (draft finding)

Status: OPEN draft for a controller-assigned LB-F row. Owner: core-t1.
Delivery: a separate serialized ENGINE branch, with affected readings regraded.
No implementation, new grade, scope expansion or plan-box change is claimed here.

## Measured evidence

The [spent Bourbon full-population post-hoc reading](../bench/readings/scss-bourbon-runner-posthoc-2026-10-26.json)
uses the subject runner's in-repo load paths. Dart Sass resolves 85/85 imports;
Fathom resolves 45/85. Import resolution and file-edge recall are .5294,
file-edge precision is 1, and L3 is unmet. The original no-load-path reading
scored only 45 imports and does not validate the missing 40.

Foundation's [original holdout](../bench/readings/lang/holdout/scss-foundation.json)
read file-edge precision 117/142 (.8239). Its
[native-oracle post-hoc reading](../bench/readings/scss-native-foundation-posthoc-2026-10-26.json)
attests the remaining 25 pairs and reads 142/142 (1). That is a related
configuration/attestation limit, not evidence that Fathom missed those 25
resolutions. Keep it as a non-regression control. Bourbon establishes the
remaining product gap directly.

The [canonical reading](../bench/readings/lang/scss.json) remains L3 excellent
on its named subjects. This draft does not generalize that grade to projects
with configured in-repo load paths. Whether the claimed L3 box changes is an
owner decision; the lane leaves it untouched.

## Finding text for the owning row

SCSS import resolution must honor the subject's declared runner load paths
(`-I` / `loadPaths` configuration) while preserving relative-import precedence
and distinguishing external targets. Measured missing support: Bourbon 45/85
imports resolved under its own setup. Foundation 117/142 is the original
attestation gap, now 142/142 under the native oracle.

This engine gap is independent of fresh native-oracle validation. Completing
LB-Q-scss-rest's oracle-validation trigger must not close this finding.

## Checkable implementation and re-measure trigger

Before implementation, freeze the supported configuration source, precedence
and path boundaries. Add a failing fixture for a configured in-repo load-path
import, with relative precedence and unresolved/external controls. The fixture
must fail on the current implementation and pass after the change.

On the resulting ENGINE branch, regrade canonical Bootstrap/Primer and every
other reading affected by the shared resolver. Regrade the spent Bourbon
full 85-import population with its same runner paths and frozen native truth:
require 85/85 resolved imports (1), no loss of the original 45, and all L3 and
tenancy gates. Regrade spent Foundation with the same native truth and retain
142/142 file precision. Preserve all failed and unknown inputs in coverage.
These are regression regrades of spent subjects, never fresh validation.
Any fresh validation must have a separate pre-read pin and criterion and stay
outside the canonical corpus shard.

The controller assigns the durable LB-F id and serializes landing. Until then
this indexed draft is the owning record, linked from both LB-Q-scss and
LB-Q-scss-rest so closing the oracle row cannot hide the product gap.
