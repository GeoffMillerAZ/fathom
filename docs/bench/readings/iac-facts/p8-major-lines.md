IF-P8 verdict: PASS. The largest measured line, hashicorp/aws 5.x (125 releases), is 13,527,965 bytes as a zip (12.90 MiB, 40% of the 32 MiB bar) with 3,338 entries.

# IF-P8: one catalog per provider major line

## Summary

IF-P6 found that a recent-12 catalog binds only 3% to 60% of public lockfile selections. The candidate fix is one catalog per major
line holding every stable release, so exact-version binding reaches 100% once an operator adds that line. This reading asks whether
that is affordable, using the IF-P1 pruning and blob deduplication unchanged.

Bar, fixed before reading: every complete major line of `hashicorp/aws` is at most 32 MiB (33,554,432 bytes) zipped and at most
500,000 zip entries.

1. 6.x, 5.x and 4.x, every stable release exported and built: 9.98, 12.90 and 6.14 MiB, 3,847, 3,338 and 2,519 entries. PASS, measured.
2. 3.x and 2.x are projections: 10.77 and 8.65 MiB. PASS, projected, not measured (see the limits).
3. Worst line: 5.x measured at 12.90 MiB, 2.5 times under the size bar. Worst projection: 3.x at 10.77 MiB. Entries peak at 3,847,
   130 times under the entry cap.
4. A release costs about 79 to 121 KB as a zip on top of the line's first release, because most resources do not change between
   releases (here 175,502 blob references collapse to 3,769 blobs for 6.x).

## Numbers

Bytes are the catalog zip: deflate per entry, a manifest, one version index per release, blobs named by the sha256 of the pruned
schema. Largest blob is the largest pruned schema, uncompressed. Marginal is (line zip minus the line's first release as its own zip)
divided by (releases minus one).

| Line | Releases in line | Releases built | Status | Blobs | Zip entries | Zip bytes | Zip MiB | Largest blob (bytes) | Marginal bytes per release |
|---|---|---|---|---|---|---|---|---|---|
| aws 6.x | 77 | 77 | measured | 3,769 | 3,847 | 10,467,594 | 9.98 | 2,903,208 | 120,833 |
| aws 5.x | 125 | 125 | measured | 3,212 | 3,338 | 13,527,965 | 12.90 | 1,422,949 | 101,647 |
| aws 4.x | 75 | 75 | measured | 2,443 | 2,519 | 6,441,403 | 6.14 | 1,417,840 | 78,764 |
| aws 3.x | 97 | 0 | projected | n/a | n/a | 11,291,909 | 10.77 | n/a | 111,240 (assumed, mean of 5.x and 6.x) |
| aws 2.x | 77 | 0 | projected | n/a | n/a | 9,067,112 | 8.65 | n/a | 111,240 (assumed) |

First release of each measured line as its own zip: 6.0.0 is 1,284,310 bytes (2,045 entries), 5.0.0 is 923,736 (1,622 entries),
4.0.0 is 612,884 (1,179 entries). The single-release zip for 6.67.0 is 1,549,513 bytes, the same count IF-P1 reported.

All lines, one catalog per line:

| Total | Zip bytes | MiB |
|---|---|---|
| Measured only (6.x, 5.x and 4.x) | 30,436,962 | 29.03 |
| All five lines, projections included | 50,795,983 | 48.44 |
| One zip of every measured export (277 releases of 6.x, 5.x and 4.x), cross-line dedup | 29,545,858 | 28.18 |

The last row is smaller than the three measured line zips added up (30,436,962) because identical blobs are shared across lines.
Per-line catalogs are the unit the bar applies to, so no total has to fit under 32 MiB.

Pruned bytes before compression, for scale: 6.x holds 870,111,253 bytes of pruned schema across its 77 releases without dedup and
31,616,099 bytes in 3,769 unique blobs. The raw `tofu providers schema -json` exports total 1,291,702,485 bytes for 6.x and
1,517,961,735 for 5.x.

Share of releases whose pruned schema is identical to the previous release (the whole version index, provider block included):
6.x 4 of 77 (5.2%), 5.x 14 of 125 (11.2%), 4.x 5 of 75 (6.7%), 23 of 277 together (8.3%). Such a release still costs a version
index, so it is cheap, not free.

## Coverage of IF-P6 sample A

IF-P6 sample A holds 115 AWS selections. Cumulative from the newest line, with each line's measured or projected status here:

| Line | Selections | Share | Cumulative selections | Cumulative share | Size status |
|---|---|---|---|---|---|
| 6.x | 26 | 22.6% | 26 | 22.6% | measured |
| 5.x | 46 | 40.0% | 72 | 62.6% | measured |
| 4.x | 30 | 26.1% | 102 | 88.7% | measured |
| 3.x | 12 | 10.4% | 114 | 99.1% | projected |
| 2.x | 1 | 0.9% | 115 | 100.0% | projected |

The 80% coverage bar is first met by adding 4.x, so every line needed to reach it is measured: 6.x, 5.x and 4.x together are
102 of 115 selections (88.7%) at 29.03 MiB across three catalogs, each under the 32 MiB bar. 3.x and 2.x add 11.3 points and
rest on projections. The selection counts are IF-P6's, taken as given, not recomputed here.

### Projection method

For 3.x and 2.x: line zip = first-release zip + (releases minus one) times the mean per-release marginal of 6.x and 5.x
(111,240 bytes). The first-release term is the 4.0.0 zip (612,884 bytes), the smallest measured release, borrowed because 3.x and
2.x have no measured release. Cross-checks:

- A projection made before 4.x was exported, from the 4.9.0 zip and this marginal, was 8,889,895 bytes for 4.x. Measured, 4.x is
  6,441,403, so the method overestimated by 38%. 4.x's own marginal (78,764) is 29% below the 5.x and 6.x mean.
- Using 4.x's marginal puts 3.x at 8,174,206 bytes (7.80 MiB). Using the larger 6.x marginal (120,833) puts it at 12,212,822
  (11.65 MiB).
- A projection would have to be 3.0 times too low before 3.x reached the bar.

## Method

- Release lists, fetched at run time on 2026-11-05 (UTC) from both `registry.terraform.io` and the OpenTofu registry, stable releases
  only (pre-releases excluded), then merged: 6.x 77, 5.x 125, 4.x 75, 3.x 97, 2.x 77. The OpenTofu registry adds 5.71.0 and nine
  3.x patch releases (3.26.x and 3.64.3) that `registry.terraform.io` does not list.
- Export: the OpenTofu 1.13.0 darwin_arm64 build (release archive checked against its published SHA256SUMS), `tofu init` against a
  local filesystem mirror, then `tofu providers schema -json`, one release per working directory, at most three at a time under
  `nice` on an Apple-silicon Mac. Every provider archive was checked against the SHA256SUMS published beside it before use and
  deleted after its export. No credentials were used or transferred. The 277 exports ran in three runs: 208 releases in 12 minutes 46 seconds, 5.71.0 alone, then the 68 remaining 4.x releases in 3 minutes 7 seconds.
- Archives: the HashiCorp release archive for each version, darwin_arm64. 5.71.0 has no HashiCorp archive (404), so it comes from
  the OpenTofu registry's published build of the same version, whose archive sha256 matches the registry's own record.
- Build: the IF-P1 prototype, unchanged except that it reads gzip-compressed exports, orders versions numerically, and prints the
  entry count and unchanged-release count. Same pruning (drops `description` and `description_kind`, keeps names, types, flags,
  nesting and resource identity schema), same canonical JSON, blob dedup by sha256, deflate, zero timestamps. Nothing about the
  pruning was changed to pass. Building the 6.x line twice gave the same sha256.
- 4.x was first built from a seven-release sample (every tenth release) and then exported in full on request, because the coverage
  bar depends on it. The sample is not used in any number above.

## Export failures

None among the 277 attempted exports (every stable 6.x, 5.x and 4.x release). Every 4.x release, 4.0.0 included, has a
darwin_arm64 build, so no 4.x release is unmeasured. 5.71.0 failed its first download (HashiCorp 404) and was exported from the
OpenTofu registry build, counted as exported.

Not exported: all 97 3.x releases (3.0.0 to 3.76.1) and all 77 2.x releases (2.0.0 to 2.70.4), 174 in total. None has a
darwin_arm64 build and neither available Mac has Rosetta. None was attempted or substituted. The 3.x and 2.x rows rest on
5.x and 6.x costs alone, with no measured point in either line.

Listed by the HashiCorp release index but by neither registry, so excluded and not exported: 6.1.0 and 6.57.0. A lockfile cannot
pin them through a registry. IF-P1 used 6.57.0, which does not change the verdict (2 of 79 releases, about 120 KB each).

## Limits

1. 3.x and 2.x have no darwin_arm64 builds, so a Linux amd64 builder is needed to measure them. Their rows are projections with no
   measured point. Early AWS lines had fewer resources and the one early line measured (4.x) came in 29% under the 5.x and 6.x
   marginal, so the projection is more likely high than low, but that is reasoning, not a reading.
2. Only 6.x, 5.x and 4.x are measured. They cover 88.7% of IF-P6 sample A.
3. Replacement facts are not in these catalogs. IF-P1 bounded them by adding one extra key to every attribute of every schema:
   +1.9% for one AWS version and +1.2% for 12. The same over-estimate on full lines gives 6.x 10,521,216 bytes (+0.51%), 5.x
   13,594,580 (+0.49%) and 4.x 6,473,515 (+0.50%). IF-P3 measures the real replacement facts.
4. Blob size against the 4 MiB bound IF-M0 set: the largest blob is 2,903,208 bytes (2.77 MiB, 69% of the bound) in 6.x, 1.36 MiB
   in 5.x and 1.35 MiB in 4.x. Lookup cost on larger catalogs was not measured: IF-P1 saw the typical allocation grow with entry
   count, now up to 3,847, still far under the cap.
5. Provider archives are checked against published checksums, not their signatures.
6. Only `hashicorp/aws`. Other providers have fewer resources, so the size bar is expected to bind AWS first, but that was not read.
7. One build run. Sizes are deterministic, so repeating adds nothing.
