### Fixed

- Newest test-case reads return the native binding once per selected run and reuse it across that run's joined cases, reducing memory allocated for large native test suites while preserving each case's evidence and newest-run ordering.
