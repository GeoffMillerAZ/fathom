IF-M0 verdict: GO for AWS and CloudFormation with four amendments. The catalog format holds, facts 5 to 7 proceed for AWS, and version coverage moves from recent-N to one catalog per major line, which is proven only if IF-P8 reads PASS.

# IF-M0 phase 0 exit

Decided 2026-11-04 from IF-P1 to IF-P7. Azure, Google and Pulumi stay unread or partial here and keep their own lanes.

## What each reading decided

| Reading | Result | Decision |
|---|---|---|
| IF-P1 | Size and p95 pass with room: AWS 1.48 MiB, 12 releases 2.73 MiB, p95 4.46 ms. Six types exceed the 1 MiB blob bound, and 84 of 28,641 lookups allocate over 8 MiB, up to 41.8 MB | The format holds. A1 fixes both exceptions |
| IF-P2 | OpenTofu and HashiCorp `aws` builds export byte-identical schemas at two versions | One build serves both addresses. Identity stays distinct, as design section 5 says |
| IF-P3 | The AWS scan resolves 98.1% of top-level attributes, with 0 false alarms and 0 silent misses among those resolved | Fact 5 proceeds for AWS. Azure and Google scans were not run, so they stay unclaimed |
| IF-P4 | Pulumi carries replacement facts for SDKv2-bridged resources and Azure Native, and none for framework resources or Kubernetes | Pulumi stays behind its own phase, and YAML reads still come through `LL-33` |
| IF-P5 | `tofu validate` caught 23 of 23 seeded validity cases. No baseline states replacement or cascade. 60 of 79 arguments removed from AWS 5.100.0 to 6.67.0 gave no prior warning | The design's prediction held: facts 1 to 4 have little lift where `validate` runs. Facts 5 to 7 are the differential, so they proceed |
| IF-P6 | **FAIL.** The newest 12 releases bind 3% to 60% of selections, and AWS needs its newest 242 to 258 of 512 | Recent-N is dead. A2 replaces it |
| IF-P7 | 3.16 MiB per region, regions differ, and the schema files carry no licence allowing redistribution | A3 and A4 |

## Amendments

**A1, bounds.** The blob bound rises from 1 MiB to 4 MiB, since the largest measured type is 2.43 MiB. `IF-1` decodes only the requested path with a streaming decoder, and one fact stays under I48's 8 MiB point ceiling. A lookup that cannot stay under it refuses that one answer with a reason. A test holds this on the largest blob. The prototype decoded whole blobs, so this cost is unmeasured until `IF-1`.

**A2, version coverage.** One catalog per provider major line, holding every stable release of that line. Exact-version binding then reaches every selection whose line an operator has added, since every locked version was a real release. Binding still answers `unbound` without a catalog for the exact version, and the disclosure names the line to add. A labelled nearest version stays rejected: facts 1 to 4 pay for false alarms, and the alternative was never measured.

Size is the open half. IF-P1's per-release cost suggests about 0.11 MiB per release, but that is 12 consecutive releases, not a major line. `IF-P8` measures every AWS 5.x and 6.x release against IF-P1's budget of 32 MiB per catalog. `IF-6` waits on it. The reader, binding and checks do not, because the format is the same at any N.

If IF-P8 reads FAIL, no catalog is published for that provider, the bar stays unmet and is reported as unmet, and operators use `fathom iac import` from their own export. That path has exact-version coverage wherever an operator can run the provider, and it carries no replacement facts.

**A3, CloudFormation catalogs are operator-built.** AWS's terms bar redistribution absent a separate licence, and none was found. `IF-6` publishes no CloudFormation catalog, and no extract or built catalog is committed. So the builder cannot stay maintainer-only: `IF-C1` moves to `internal/iacfacts` and is reached through `fathom iac import <schema zip>`, reading a file the operator fetched. I56 still holds. Publishing remains an owner decision, not a blocker. The reading is not legal advice.

**A4, CloudFormation keys and the library's catalog path.** The key is source, region and a digest over entry names and bodies, because the zip embeds its build time. Regions differ by 2% to 50% of shared types, and create-only lists differ for 3 to 71 types, so an answer names its region. A standalone consumer of `pkg/lang/iac` has no `fathom` binary, so `IF-C8` adds `iac.ImportCatalog`, reached through `fathom-lang catalog import`, for both a schema zip and a `tofu providers schema -json` export.

## Conditions on later rows

- `IF-13` emits three classes: replace, conditional and unknown. `CustomizeDiff` (145 SDKv2 resources) and `ModifyPlan` (14 framework resources) read conditional. 29 of 581 replacing attributes read unknown today, 28 of them on one resource.
- `IF-M2` adds a per-plan timeout, because one AWS resource hung over four minutes. It adds known-value inputs too, because IF-P3's all-unknown input cannot exercise conditional replacement.
- `IF-M1` grades facts 1 to 4 against `tofu validate`.
- `IF-17`'s value is before `init`: `validate` reports removed arguments only after `init` on the new provider version.

## Decision models

None. Each amendment is a size or licence fact with a deterministic answer, so a decision model has no question to ask here.

## Limits

Lockfile samples read what code search exposes, at default branches. Region and licence readings cover one snapshot of 7 of 37 regions. Replacement accuracy covers top-level attributes on AWS 6.67.0 only.
