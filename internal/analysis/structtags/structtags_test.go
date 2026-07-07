package structtags_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/structtags"
)

const cleanSrc = `package foo

type User struct {
	ID   int    ` + "`" + `json:"id"` + "`" + `
	Name string ` + "`" + `json:"name"` + "`" + `
	age  int    // unexported — no json tag required
}
`

const missingJSONSrc = `package foo

type Event struct {
	ID        int    ` + "`" + `json:"id"` + "`" + `
	EventType string          // missing json tag
	timestamp int64           // unexported — OK
}
`

const malformedTagSrc = `package foo

type Bad struct {
	Field string ` + "`" + `json:missing_quotes` + "`" + `
}
`

const multipleIssuesSrc = `package foo

type Mixed struct {
	Good   string ` + "`" + `json:"good"` + "`" + `
	NoTag  int
	BadTag bool   ` + "`" + `notvalid` + "`" + `
}
`

func TestScan_Clean(t *testing.T) {
	issues := structtags.Scan("clean.go", cleanSrc)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestScan_MissingJSON(t *testing.T) {
	issues := structtags.Scan("event.go", missingJSONSrc)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Field != "EventType" {
		t.Errorf("Field: got %q, want %q", issues[0].Field, "EventType")
	}
	if issues[0].File != "event.go" {
		t.Errorf("File: got %q, want %q", issues[0].File, "event.go")
	}
}

func TestScan_MalformedTag(t *testing.T) {
	issues := structtags.Scan("bad.go", malformedTagSrc)
	if len(issues) == 0 {
		t.Fatal("expected at least 1 issue for malformed tag")
	}
	found := false
	for _, iss := range issues {
		if iss.Field == "Field" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issue for field 'Field', got %v", issues)
	}
}

func TestScan_MultipleIssues(t *testing.T) {
	issues := structtags.Scan("mixed.go", multipleIssuesSrc)
	// NoTag (missing json) + BadTag (malformed)
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 issues, got %d: %v", len(issues), issues)
	}
}

func TestScan_ParseError(t *testing.T) {
	issues := structtags.Scan("bad.go", "this is not go source }{")
	if len(issues) == 0 {
		t.Fatal("expected parse error issue")
	}
}

func TestScan_Deterministic(t *testing.T) {
	a := structtags.Scan("f.go", multipleIssuesSrc)
	b := structtags.Scan("f.go", multipleIssuesSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at [%d]: %+v vs %+v", i, a[i], b[i])
		}
	}
}
