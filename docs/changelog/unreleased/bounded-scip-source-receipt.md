### Fixed

- SCIP source receipt validation releases decoded document and external-symbol payloads before source hashing while preserving protobuf validation of every index field. Large unused index payloads no longer remain live throughout receipt checks.
