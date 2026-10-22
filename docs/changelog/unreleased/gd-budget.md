### Fixed

- `review_diff`'s complete MCP result (text, structured data and metadata together) now stays within 16,000 bytes. An oversized review keeps all eight dimension statuses and counts, keeps as many leading findings, changed files, tests and layers as fit, names what it omitted (also in the call log), and refuses by name when nothing fits; a result that already fits is byte-identical.
