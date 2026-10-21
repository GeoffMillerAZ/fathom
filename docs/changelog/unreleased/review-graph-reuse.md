### Fixed

- `review_diff` reuses one complete service snapshot and typed call graph for impact, test impact, resolution, and receipts. A symbol, typed-edge, or graph load failure now stops review before finding reconciliation instead of producing a clean-looking partial answer.
