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

// GraphCacheStatsForTest exposes the server's watermark-keyed call-graph
// memo stats (entries, hits, misses) — O33: pins that verify_plan's second
// call at an unchanged watermark HITS the same cache the nav tools use
// (resolved_nav.go's loadNavGraphs) instead of rebuilding the graph from
// scratch every call.
func (s *Server) GraphCacheStatsForTest() (entries, hits, misses int) {
	return s.graphCache.Stats()
}

// ToolCostClassesForTest maps every registered tool to its declared cost
// class name (RT-55); args-dependent tools report their no-argument class.
func (s *Server) ToolCostClassesForTest() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.tools))
	for _, e := range s.tools {
		out[e.desc.Name] = e.class.String()
	}
	return out
}

// CostClassOfForTest is costClassOf for a registered tool and its args.
func (s *Server) CostClassOfForTest(name, args string) string {
	te, ok := s.findTool(name)
	if !ok {
		return ""
	}
	return costClassOf(te, []byte(args)).String()
}

// RegisterUndeclaredToolForTest registers a tool with no cost class; it
// must panic (RT-55: registration without a class fails).
func (s *Server) RegisterUndeclaredToolForTest(name string) {
	s.registerTool(toolDescriptor{Name: name, InputSchema: map[string]any{"type": "object"}, group: GroupShared}, nil)
}
