### Security

- Release archives and `checksums.txt` carry a build-provenance attestation, and the release workflow grants permissions per job with every third-party action pinned by commit SHA. Verify a download with `gh attestation verify <archive> --repo GeoffMillerAZ/fathom`.

### Changed

- `fathom version` on a binary built without a VCS revision (`go install`) reports `(module vX, sum h1:...)` instead of `(revision unknown)`. `version --json` gains `module_version` and `module_sum`, and `/statusz` build info gains the same two fields.
