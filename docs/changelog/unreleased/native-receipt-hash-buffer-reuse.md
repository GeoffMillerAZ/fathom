### Fixed

- Native Unity and SCIP receipt validation reuse one bounded hashing buffer per call while rehashing every input on each validation. This reduces allocation growth for large source inventories without changing freshness checks.
