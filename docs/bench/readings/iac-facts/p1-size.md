IF-P1 verdict: PASS on the bars as written (size and p95 cost); two design bounds are exceeded by 6 AWS resource types.

# IF-P1: catalog size and lookup cost

## Summary

A pruned, blob-deduplicated catalog is small. One AWS version is 1.48 MiB as a zip (the bar is 16 MiB). Twelve consecutive AWS
versions are 2.73 MiB, 1.85 times one version (the bar is under 2x). A one-fact lookup has p95 4.5 ms over every key of the
12-version catalog (the bar is 20 ms), and allocates 2.05 MiB at p95 (the bar is 8 MiB). The raw export would have missed the size
bar (18.6 MiB), so pruning is what passes it, not compression.

Exceptions, reported first because they bear on the design's section 9 bounds:

1. Six AWS schemas exceed the 1 MiB blob bound: `aws_wafv2_web_acl_rule` 2.43 MiB, `aws_quicksight_dashboard`, `_analysis`
   and `_template` 1.36 MiB each, `aws_wafv2_web_acl` 1.30 MiB, `aws_wafv2_rule_group` 1.04 MiB (pruned, uncompressed). Their
   compressed size is far smaller. Under the bound as written, the catalog refuses these types.
2. Decoding those same blobs with a generic JSON decoder allocates up to 41.8 MB for one lookup. 84 of 28,641 lookups (0.29%)
   in the 12-version catalog allocate more than 8 MiB, and 115 (0.40%) take over 20 ms (maximum 177 ms). The bar is read as a
   p95, which passes. Read as a maximum it fails for exactly these types. A typed or streaming decode was not tried.

## Numbers

Bytes. Raw is the `tofu providers schema -json` export. Pruned drops every `description` and `description_kind` key and keeps
names, types, flags, nesting and the resource identity schema, as unique blobs, uncompressed. Zip is the catalog, deflate per
entry, with its manifest and version index.

| Catalog | Resources + data sources | Raw | Pruned (unique blobs) | Zip |
|---|---|---|---|---|
| aws 6.67.0 | 2,411 | 19,517,134 (18.61 MiB) | 12,984,448 (12.38 MiB) | 1,549,513 (1.48 MiB) |
| azurerm 5.11.0 | 1,504 | 3,204,662 | 1,987,612 | 878,178 |
| google 8.8.1 | 1,853 | 9,782,236 | 2,980,494 | 1,088,646 |
| kubernetes 3.4.0 | 108 | 3,133,434 | 772,304 | 109,405 |
| helm 3.3.0 | 2 | 27,361 | 9,584 | 2,903 |
| aws, 12 versions 6.57.0 to 6.67.0 | 28,641 keys, 2,473 unique blobs | 232,432,645 | 15,750,429 (155,378,593 without dedup) | 2,860,073 (2.73 MiB) |

Each added AWS version costs about 119 KB (the 11 extra versions add 1,310,560 bytes). 28,774 blob references collapse to 2,473
blobs, so most resources are unchanged between consecutive releases.

Sensitivity for replacement facts, which the export does not carry (IF-P3 measures them): adding one extra key to every
attribute of every schema, a deliberate over-estimate, grows the single-version zip to 1,579,739 (+1.9%) and the 12-version zip
to 2,893,008 (+1.2%).

Lookup, open the zip, read one version index, read one blob, decode it, close. No residency, no memo. Every key looked up once, in
sorted order, in one process:

| Catalog | Lookups | p50 | p95 | p99 | max | alloc p50 / p95 / max |
|---|---|---|---|---|---|---|
| aws, 12 versions | 28,641 | 2.69 ms | 4.46 ms | 9.12 ms | 177 ms | 1.97 MB / 2.05 MB / 41.8 MB |
| aws 6.67.0 | 2,411 | 2.19 ms | 2.76 ms | 6.74 ms | 33.5 ms | 1.95 MB / 2.04 MB / 41.7 MB |
| azurerm 5.11.0 | 1,504 | 1.60 ms | 2.65 ms | 4.00 ms | 18.2 ms | 1.29 MB / 1.33 MB / 1.54 MB |
| google 8.8.1 | 1,853 | 3.06 ms | 4.70 ms | 7.33 ms | 30.2 ms | 1.61 MB / 1.67 MB / 2.29 MB |

One-shot process, one lookup, peak resident set size as reported by the OS (includes the Go runtime): `aws_instance` from the
12-version catalog 8.9 MB with 2.35 MB allocated and 5.6 ms; a data source 8.6 MB; the largest type (`aws_wafv2_web_acl_rule`)
44.4 MB, 42 MB allocated, 43 ms.

The typical allocation (about 2 MB) is the cost of reading the zip's central directory and the version index, not the blob. It
grows with entries in the catalog, 2,486 here, far under the 500,000 cap.

Determinism: building the 12-version catalog twice gave the same sha256 (5b974be0...01cc01).

## Method

- Providers: the OpenTofu 1.13.0 build of `hashicorp/aws`, installed from a local filesystem mirror, versions 6.57.0, 6.57.1 and
  6.58.0 to 6.67.0. These are the 12 consecutive releases ending at the calibration pin (6.67.0, archive sha256
  `25afdd7e...98240d04`, binary sha256 `dd371fb4...906228a4`, both matched). AzureRM 5.11.0 and Google 8.8.1 are the pins in
  `calibration/oracles/terraform-provider-schema/pins.json`, the HashiCorp archives, archive and binary sha256 both matched.
  Kubernetes 3.4.0 and Helm 3.3.0 have no pin: they are the highest stable releases in the HashiCorp index at measurement time,
  taken as the pins for this reading. All darwin_arm64.
- Export: `tofu providers schema -json` after `tofu init`, one provider per working directory.
- Prototype (throwaway, not committed): a Go program that prunes each schema, writes canonical key-sorted JSON, names each blob by
  the sha256 of its bytes, and writes a zip of `manifest.json`, `versions/<version>.json` (type to blob hash for resources,
  data sources and ephemeral resources, plus the provider block) and `blobs/<sha256>`, sorted, deflate, zero timestamps. The
  reader opens the zip with `archive/zip`, scans for the version index, decodes it, finds one blob, decodes it into a generic
  map and closes.
- Timing: wall clock per lookup around the whole open-to-close call, in one process, after the build wrote the file. Allocation:
  `runtime.MemStats.TotalAlloc` delta per lookup. Peak memory: `getrusage` and `/usr/bin/time -l` on a fresh process.

## Limits

- Warm operating-system file cache. A cold-cache read was not measured (flushing it needs elevated rights).
- One machine (Apple silicon, Go 1.27.3), one run of the exhaustive pass, no repeated trials for the latency distribution.
- Pruned blobs are not the product format: the product adds replacement facts and the `CustomizeDiff` flag. Only an upper bound
  for their size is given.
- The export has no description prose for these providers beyond `description_kind`, so pruning saves less on AWS (33%) than on
  Kubernetes (75%), whose export carries prose.
- Zip entries are compressed one by one. A whole-catalog compression would be smaller and was not tried.
- IF-P6 decides whether 12 versions is enough coverage. This reading says only what N costs.

## For the design

- Section 9: raise the 1 MiB blob bound to at least 3 MiB, or accept that six AWS types are refused. The largest fact allocated
  42 MB with a generic decoder, so test a typed decoder before hardening the 8 MiB allocation figure.
- Size has about a 10x margin against the 16 MiB bar, so the catalog may carry more than 12 versions (IF-P6 sets how many).
- Raw exports must never be shipped: 18.6 MiB for one version.
