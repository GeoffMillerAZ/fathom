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
