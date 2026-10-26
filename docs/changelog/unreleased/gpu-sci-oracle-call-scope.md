### Fixed

- Language calibration counts call false positives in every file an independent tool oracle explicitly read, including files with no oracle calls. Explicit empty call dumps now report zero truth instead of silently omitting the calls family.
