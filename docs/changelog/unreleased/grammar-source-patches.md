### Added

- Grammar builds can apply ordered, checksum-pinned in-tree source patches after verifying upstream sources. Builds apply the staged patch bytes under isolated Git settings and refuse unsafe paths, file metadata or attribution edits, and patches with no source-byte change. Generated provenance identifies patched grammars and every patch digest.
