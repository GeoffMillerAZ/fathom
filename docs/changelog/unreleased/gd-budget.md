### Fixed

- Bound `review_diff`'s complete serialized MCP result to 16,000 bytes, including text, structured data and metadata. Oversized reviews preserve all eight dimension statuses and original counts with named detail omissions; impossible or malformed results receive a named refusal. Results that already fit remain byte-identical.
