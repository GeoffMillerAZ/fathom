package mcp

// export_test.go — test-only accessors. Visible to package mcp_test in this
// directory, never to production callers (the file is compiled only into the
// package's test binary), so asserting an internal contract costs no API
// surface.

// GroupOfForTest returns the registered ToolGroup of name, or "" when the
// tool is not registered.
//
// HF-5 (ADR-014 amendment 2) deleted the four path-scoped group surfaces
// (/rpc/coding, /rpc/investigation, /rpc/shared, /rpc/platform) because no
// wired client could reach them. The group TAXONOMY they were named after
// is untouched and still classifies every tool, so the tests that used to
// assert "tool X appears on the coding surface" now assert the fact they
// were really about: "tool X is registered in GroupCoding".
func (s *Server) GroupOfForTest(name string) ToolGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.tools {
		if e.desc.Name == name {
			return e.desc.group
		}
	}
	return ""
}

// ValidateAgainstSchemaForTest exposes validateAgainstSchema (RP-11) so the
// mcp_test live-result test can check every agent-surface tool's
// structuredContent against the outputSchema tools/list advertised.
func ValidateAgainstSchemaForTest(schema map[string]any, v any) string {
	return validateAgainstSchema("$", schema, v)
}
