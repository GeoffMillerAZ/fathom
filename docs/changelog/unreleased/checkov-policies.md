### Added

- Checkov checks are read as policies: a Python check under `checks/{resource,data,provider,module}/` becomes a symbol named for its id (`CKV_AWS_18`) with the resource types it covers as its calls, and a YAML graph check under `checks/graph_checks/` becomes one named for `metadata.id` with the resource types its definition names. Measured at L2, excellent, against Checkov's own loaders on Checkov 3.2.500 (1,636 checks, 2,131 resource-type references). Python files under those directories no longer carry Python symbols, and a check directory of any other name is not claimed.
