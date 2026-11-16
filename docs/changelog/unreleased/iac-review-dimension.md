### Added

- `review_diff` now includes an `iac` review dimension over existing Terraform and CloudFormation catalog findings. It reports unchecked coverage and a reason for missing catalogs or source, incomplete or capped work, and dated reviews. The caller's exact `as_of` revision survives response compaction. Existing top-level `iac` sections remain compatible.

### Fixed

- CloudFormation reviews no longer infer complete coverage from empty resource reads, including the comparison base.
