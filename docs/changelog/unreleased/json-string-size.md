### Fixed

- MCP byte budgeting matches the active standard JSON encoder for malformed UTF-8, retaining exact encoded response sizes across Go encoder implementations. Evidence: the independent Marshal oracle in `internal/mcp/json_string_size_test.go`.
