### Changed

- The call graph build resolves constructor-scoped calls (`<import-path>#.Method`) through an index built once per build, instead of scanning every node for each call. The resolutions are the same: an equality test holds the index to the old scan, which stays as the oracle and as the package-patch path (RT-195).
