### Changed

- `symbol_context` loads and caches only the combined call graph it uses. Cold calls still return a named partial when the graph cannot finish within the deadline, and a failed typed-edge read still falls back to syntax for that call and retries later.
