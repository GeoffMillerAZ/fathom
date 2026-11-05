IF-P2 verdict: PASS, the OpenTofu and HashiCorp builds of `aws` export byte-identical schemas at both versions tried (0 differences).

# IF-P2: registry equality

## Summary

`tofu providers schema -json` output for the OpenTofu re-release of `hashicorp/aws` and for the HashiCorp release is byte for
byte equal at 6.67.0 (the calibration pin) and at 6.72.0 (the latest at measurement time). Zero differences to count. The binaries
differ, so the equality is in the schema, not in the build.

## Numbers

| Version | OpenTofu build export | HashiCorp build export | Result |
|---|---|---|---|
| 6.67.0 | 19,517,134 bytes, sha256 `42c2bd77...b2005ce6f` | 19,517,134 bytes, same sha256 | `cmp` identical |
| 6.72.0 | same size as its twin, sha256 `4415550b...ae0e71` | sha256 `4415550b...ae0e71` | `cmp` identical |

Content of the 6.67.0 export: 1,728 resources, 683 data sources, 11 ephemeral resources, 4 functions, 540 resource identity
schemas.

The builds are different artifacts:

| Version | OpenTofu build archive sha256 | HashiCorp build archive sha256 | Binaries differ |
|---|---|---|---|
| 6.67.0 | `25afdd7e...98240d04` | `25e03c7f...b164b475f` | yes (`dd371fb4...` against `3e156550...`) |
| 6.72.0 | `0d37e9da...a51925` | `7fba5525...bc5d2` | yes (`0e32f4df...` against `d191ebf9...`) |

The HashiCorp archive checksums matched the published `SHA256SUMS` files; the 6.67.0 sums file hash matches the value recorded
in `pins-errata.json`. Each pair of exports was produced the same way: same OpenTofu 1.13.0 binary, a local filesystem mirror
holding only that provider's archive, `tofu init`, `tofu providers schema -json`.

## Method

1. Download the OpenTofu re-release archive from the `opentofu/terraform-provider-aws` GitHub release and the HashiCorp archive
   from `releases.hashicorp.com`, for the same version.
2. Verify each archive against its published checksum.
3. Install each through a local filesystem mirror under the same `registry.opentofu.org/hashicorp/aws` address, with the plugin
   cache disabled, so the binary that runs is the one under test (a shared plugin cache refused the second build and was turned
   off for this reading).
4. Export with `tofu providers schema -json` and compare the files with `cmp` and sha256.

## Limits

- Two versions, one platform (darwin_arm64). A difference at another version, platform or in `awscc` is not excluded.
- Both exports were read by OpenTofu. The Terraform CLI was not installed, so a Terraform-side difference in how it prints the
  same schema is unmeasured. The catalog is built under OpenTofu, so that does not affect a catalog.
- Equal schemas say nothing about equal behavior at plan time (IF-P3).

## For the design

One catalog per version serves both registries for `aws`. The lockfile binding can ignore which registry's build selected the
version. Check the equality again for AzureRM and Google before extending the claim to them.
