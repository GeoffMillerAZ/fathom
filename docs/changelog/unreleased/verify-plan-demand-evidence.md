### Changed

- `verify_plan` reads the call graph and product reach only when a plan contains a symbol or `gone:` row. File-only and empty plans no longer wait for graph construction; spec-only plans still read the indexed symbol snapshot and source references.
