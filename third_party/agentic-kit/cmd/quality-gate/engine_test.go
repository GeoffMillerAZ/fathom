package main

import (
	"reflect"
	"testing"
)

func engineChange(path, mode, body string) change {
	return change{Path: path, AfterOID: "object", AfterMode: mode, After: []byte(body)}
}

func TestEvaluateRules(t *testing.T) {
	code := selector{Prefixes: []string{"src/"}}
	companion := selector{Prefixes: []string{"test/"}}
	tests := []struct {
		name         string
		rule         rule
		changes      []change
		boundary     string
		wantState    string
		wantMatched  int
		wantFindings []finding
	}{
		{"filtered", rule{ID: "R", Kind: "forbidden-path", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("doc/a", "100644", "x")}, "commit", "not-applicable", 0, nil},
		{"boundary", rule{ID: "R", Kind: "forbidden-path", Boundaries: []string{"push"}, Select: code}, []change{engineChange("src/a", "100644", "x")}, "commit", "not-applicable", 0, nil},
		{"forbidden", rule{ID: "R", Kind: "forbidden-path", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "x")}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "forbidden-path", Path: "src/a", Reason: "forbidden path added or modified"}}},
		{"delete-forbidden", rule{ID: "R", Kind: "forbidden-path", Boundaries: []string{"commit"}, Select: code}, []change{{Path: "src/a", BeforeOID: "old", BeforeMode: "100644"}}, "commit", "not-applicable", 1, nil},
		{"json", rule{ID: "R", Kind: "json-valid", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "{")}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "json-valid", Path: "src/a", Reason: "invalid JSON"}}},
		{"valid-json", rule{ID: "R", Kind: "json-valid", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "{}")}, "commit", "pass", 1, nil},
		{"bytes", rule{ID: "R", Kind: "max-bytes", Boundaries: []string{"commit"}, Select: code, MaxBytes: 2}, []change{engineChange("src/a", "100644", "abc")}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "max-bytes", Path: "src/a", Reason: "file exceeds maximum bytes"}}},
		{"bytes-at-limit", rule{ID: "R", Kind: "max-bytes", Boundaries: []string{"commit"}, Select: code, MaxBytes: 2}, []change{engineChange("src/a", "100755", "ab")}, "commit", "pass", 1, nil},
		{"markers", rule{ID: "R", Kind: "conflict-markers", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "ordinary\n<<<<<<< ours\n=======\n>>>>>>> theirs\n")}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "conflict-markers", Path: "src/a", Line: 2, Reason: "conflict marker present"}, {RuleID: "R", Kind: "conflict-markers", Path: "src/a", Line: 3, Reason: "conflict marker present"}, {RuleID: "R", Kind: "conflict-markers", Path: "src/a", Line: 4, Reason: "conflict marker present"}}},
		{"short-marker", rule{ID: "R", Kind: "conflict-markers", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "<<<<<<\n======\n>>>>>>")}, "commit", "pass", 1, nil},
		{"self-companion", rule{ID: "R", Kind: "require-companion", Boundaries: []string{"commit"}, Select: code, Companion: &code}, []change{engineChange("src/a", "100644", "x")}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "require-companion", Path: "src/a", Reason: "required changed companion is absent"}}},
		{"companion", rule{ID: "R", Kind: "require-companion", Boundaries: []string{"commit"}, Select: code, Companion: &companion}, []change{engineChange("src/a", "100644", "x"), engineChange("test/a", "100644", "x")}, "commit", "pass", 1, nil},
		{"deleted-companion", rule{ID: "R", Kind: "require-companion", Boundaries: []string{"commit"}, Select: code, Companion: &companion}, []change{engineChange("src/a", "100644", "x"), {Path: "test/a", BeforeOID: "old"}}, "commit", "fail", 1, []finding{{RuleID: "R", Kind: "require-companion", Path: "src/a", Reason: "required changed companion is absent"}}},
		{"symlink-unknown", rule{ID: "R", Kind: "json-valid", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "120000", "{}")}, "commit", "unknown", 1, []finding{{RuleID: "R", Kind: "unknown", Path: "src/a", Reason: "selected file is not regular"}}},
		{"gitlink-unknown", rule{ID: "R", Kind: "max-bytes", Boundaries: []string{"commit"}, Select: code, MaxBytes: 2}, []change{engineChange("src/a", "160000", "")}, "commit", "unknown", 1, []finding{{RuleID: "R", Kind: "unknown", Path: "src/a", Reason: "selected file is not regular"}}},
		{"binary-unknown", rule{ID: "R", Kind: "conflict-markers", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "\x00<<<<<<<")}, "commit", "unknown", 1, []finding{{RuleID: "R", Kind: "unknown", Path: "src/a", Reason: "selected content is binary or invalid UTF-8"}}},
		{"invalid-utf8", rule{ID: "R", Kind: "json-valid", Boundaries: []string{"commit"}, Select: code}, []change{engineChange("src/a", "100644", "\xff")}, "commit", "unknown", 1, []finding{{RuleID: "R", Kind: "unknown", Path: "src/a", Reason: "selected content is binary or invalid UTF-8"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, findings := evaluateRules(policy{Rules: []rule{tc.rule}}, tc.boundary, snapshot{Changes: tc.changes})
			if len(got) != 1 || got[0].State != tc.wantState || got[0].Matched != tc.wantMatched {
				t.Fatalf("result: %+v", got)
			}
			if tc.wantFindings == nil {
				tc.wantFindings = []finding{}
			}
			if !reflect.DeepEqual(findings, tc.wantFindings) {
				t.Fatalf("findings: got %+v, want %+v", findings, tc.wantFindings)
			}
		})
	}
}

func TestEvaluateRulesDeterministicAndFailDominatesUnknown(t *testing.T) {
	r := rule{ID: "R", Kind: "json-valid", Boundaries: []string{"commit"}, Select: selector{Prefixes: []string{"src/"}}}
	got, findings := evaluateRules(policy{Rules: []rule{r}}, "commit", snapshot{Changes: []change{
		engineChange("src/z", "100644", "{"), engineChange("src/a", "120000", "link"),
	}})
	if got[0].State != "fail" || len(findings) != 2 || findings[0].Path != "src/a" || findings[1].Path != "src/z" {
		t.Fatalf("result=%+v findings=%+v", got, findings)
	}
}

func TestEvaluateRulesEmptyCollectionsAndRuleOrder(t *testing.T) {
	results, findings := evaluateRules(policy{}, "commit", snapshot{})
	if results == nil || findings == nil || len(results) != 0 || len(findings) != 0 {
		t.Fatalf("empty collections: results=%#v findings=%#v", results, findings)
	}
	selectAll := selector{Prefixes: []string{"src/"}}
	results, findings = evaluateRules(policy{Rules: []rule{
		{ID: "first", Kind: "forbidden-path", Boundaries: []string{"commit"}, Select: selectAll},
		{ID: "second", Kind: "json-valid", Boundaries: []string{"commit"}, Select: selectAll},
	}}, "commit", snapshot{Changes: []change{engineChange("src/a", "100644", "{")}})
	if len(results) != 2 || results[0].ID != "first" || results[1].ID != "second" ||
		len(findings) != 2 || findings[0].RuleID != "first" || findings[1].RuleID != "second" {
		t.Fatalf("results=%+v findings=%+v", results, findings)
	}
}

func TestDeletedContractStillRequiresCompanion(t *testing.T) {
	r := rule{ID: "contract-tests", Kind: "require-companion", Boundaries: []string{"pre-commit"}, Select: selector{Prefixes: []string{"schema/"}}, Companion: &selector{Prefixes: []string{"tests/"}}}
	s := snapshot{Changes: []change{{Path: "schema/removed.cue", BeforeOID: "old", BeforeMode: "100644"}}}
	results, findings := evaluateRules(policy{Rules: []rule{r}}, "pre-commit", s)
	if results[0].State != "fail" || len(findings) != 1 {
		t.Fatal("contract deletion bypassed companion", results, findings)
	}
	s.Changes = append(s.Changes, engineChange("tests/schema_test.go", "100644", "updated test"))
	results, findings = evaluateRules(policy{Rules: []rule{r}}, "pre-commit", s)
	if results[0].State != "pass" || len(findings) != 0 {
		t.Fatal("changed companion rejected", results, findings)
	}
}
