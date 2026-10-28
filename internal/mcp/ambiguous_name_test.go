package mcp_test

import (
	"strings"
	"testing"
)

// A bare name two definitions share (Default in packages a and b) resolves
// to nothing in the call graph. That is ambiguity, never absence: rename
// and test_impact name the definitions and ask for a qualified name instead
// of answering "nothing to rename" or "no tests reach". find_callers
// already unions every candidate's callers (ResolvedCallersAcross), so it
// lists both definitions' callers. Pre-existing on main, before Go fields.
func TestAmbiguousNameIsNeverAbsence(t *testing.T) {
	srv := ingestFixtureServer(t, "afix", map[string]string{
		"go.mod":      "module afix\n\ngo 1.22\n",
		"a/a.go":      "package a\n\n// Default is a's.\nfunc Default() int { return 1 }\n",
		"b/b.go":      "package b\n\n// Default is b's.\nfunc Default() int { return 2 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestDefault(t *testing.T) {\n\tif Default() != 1 {\n\t\tt.Fatal(\"a\")\n\t}\n}\n",
		"c/c.go":      "package c\n\nimport (\n\t\"afix/a\"\n\t\"afix/b\"\n)\n\n// Sum calls both.\nfunc Sum() int { return a.Default() + b.Default() }\n",
	})
	for _, tc := range []struct{ tool, kind string }{{"blast_radius", "rename"}, {"test_impact", ""}} {
		t.Run(tc.tool, func(t *testing.T) {
			args := map[string]any{"service": "afix", "symbol": "Default"}
			if tc.kind != "" {
				args["kind"] = tc.kind
			}
			got := callTool(t, srv, tc.tool, args)
			s, _ := got["summary"].(string)
			for _, absence := range []string{"nothing to rename", "no direct callers", "no tests reach", "0 call sites"} {
				if strings.Contains(s, absence) {
					t.Errorf("%s(Default) summary %q claims absence for an ambiguous name", tc.tool, s)
				}
			}
			if got["ambiguous"] != true || !strings.Contains(s, "2 definitions") {
				t.Errorf("%s(Default) = %v, want ambiguous:true and a summary naming 2 definitions", tc.tool, got)
			}
			cands, _ := got["candidates"].([]any)
			if len(cands) != 2 {
				t.Errorf("%s(Default) candidates = %v, want the two definitions", tc.tool, got["candidates"])
			}
		})
	}
	got := callTool(t, srv, "find_callers", map[string]any{"service": "afix", "symbol": "Default"})
	if callers, _ := got["callers"].([]any); len(callers) != 2 {
		t.Errorf("find_callers(Default) = %v, want both definitions' callers (Sum, TestDefault)", got)
	}
}
