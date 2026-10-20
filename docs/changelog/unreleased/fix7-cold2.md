### Fixed
- `symbol_context` answers a name with more than one definition, or with none, at once instead of waiting for the call graph its answer never reads. After a daemon start, a service with no measured build answers the partial (`not computed (cold graph)`) at once while its build keeps running for the next call.
