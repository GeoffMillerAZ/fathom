package main

import "testing"

func TestParserErrorsAndRecoveredBadNodesCountAsFileErrors(t *testing.T) {
	if r := parse([]byte("main = rule { true }\n")); r.Errors != 0 || r.Files != 1 {
		t.Fatalf("valid policy: %+v", r)
	}
	if r := parse([]byte("main = rule { ( }\n")); r.Errors != 1 {
		t.Fatalf("invalid policy: %+v", r)
	}
}

// These probes intentionally describe defects in the pinned upstream parser.
// A future parser pin that fixes one must update the corresponding expectation.
func TestKnownParserDefects(t *testing.T) {
	cases := []struct {
		name, broken, control string
	}{
		{"escaped_quote", "x = \"a\\\"b\"\n", "x = \"ab\"\n"},
		{"keyword_selector", "x = tfconfig.variables[\"x\"].default\n", "x = tfconfig.variables[\"x\"].value\n"},
		{"no_final_newline", "main = rule { true }", "main = rule { true }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if r := parse([]byte(tc.broken)); r.Errors != 1 || r.Panics != 0 {
				t.Fatalf("known upstream defect changed: %+v", r)
			}
			if r := parse([]byte(tc.control)); r.Errors != 0 {
				t.Fatalf("control should parse: %+v", r)
			}
			if cause := failureCause([]byte(tc.broken)); cause != tc.name {
				t.Fatalf("attribution = %q, want %q", cause, tc.name)
			}
		})
	}
}

func TestAttributionDoesNotForceUnknownOrCombinedFailuresIntoOneClass(t *testing.T) {
	for _, src := range []string{"main = rule { ( }\n", "x = \"a\\\"b\""} {
		if cause := failureCause([]byte(src)); cause != "unclassified_or_multiple" {
			t.Fatalf("unsupported single-cause attribution: %q", cause)
		}
	}
}
