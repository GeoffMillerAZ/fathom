### Changed

- The extractor epoch is derived from per-wave bump files (`internal/fusion/epochs/<wave>.md`) and a digest of the embedded dialect manifests, queries and grammar lock, so a dialect data change re-extracts without a hand bump; `extractor_epoch` in `fathom version --json` and SARIF run properties changes form from `68` to `<number>.<12 hex>`, and the first ingest after upgrading re-extracts every file once.
