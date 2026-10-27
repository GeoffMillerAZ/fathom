### Security

- Release archives and `checksums.txt` carry a build-provenance attestation, starting with the first release after v0.23.0. The release workflow grants permissions per job with every third-party action pinned by commit SHA. Verify a download with `gh attestation verify <archive> --repo GeoffMillerAZ/fathom --signer-workflow GeoffMillerAZ/fathom/.github/workflows/release.yml`.
