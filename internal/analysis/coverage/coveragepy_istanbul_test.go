package coverage

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const coveragePySample = `{
  "meta": {"format": 3, "version": "7.4.0"},
  "files": {
    "src/widget.py": {
      "executed_lines": [1, 3],
      "missing_lines": [2]
    }
  },
  "totals": {"covered_lines": 2}
}`

const istanbulSample = `{
  "/repo/src/widget.js": {
    "path": "/repo/src/widget.js",
    "statementMap": {
      "0": {"start": {"line": 1, "column": 0}, "end": {"line": 1, "column": 10}},
      "1": {"start": {"line": 3, "column": 0}, "end": {"line": 5, "column": 1}}
    },
    "s": {"0": 1, "1": 0}
  }
}`

func TestParseCoveragePyIstanbul(t *testing.T) {
	blocks, err := ParseCoveragePyIstanbul(strings.NewReader(coveragePySample))
	if err != nil {
		t.Fatalf("ParseCoveragePyIstanbul: %v", err)
	}
	want := []ForeignBlock{
		{File: "src/widget.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "src/widget.py", StartLine: 2, EndLine: 2, Statements: 1, Covered: 0},
		{File: "src/widget.py", StartLine: 3, EndLine: 3, Statements: 1, Covered: 1},
	}
	if len(blocks) != len(want) {
		t.Fatalf("len(blocks) = %d, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i, b := range blocks {
		if !reflect.DeepEqual(b, want[i]) {
			t.Errorf("blocks[%d] = %+v, want %+v", i, b, want[i])
		}
	}
}

func TestParseCoveragePyIstanbul_Istanbul(t *testing.T) {
	blocks, err := ParseCoveragePyIstanbul(strings.NewReader(istanbulSample))
	if err != nil {
		t.Fatalf("ParseCoveragePyIstanbul: %v", err)
	}
	want := []ForeignBlock{
		{File: "/repo/src/widget.js", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "/repo/src/widget.js", StartLine: 3, EndLine: 5, Statements: 1, Covered: 0},
	}
	if len(blocks) != len(want) {
		t.Fatalf("len(blocks) = %d, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i, b := range blocks {
		if !reflect.DeepEqual(b, want[i]) {
			t.Errorf("blocks[%d] = %+v, want %+v", i, b, want[i])
		}
	}
}

func TestParseCoveragePyIstanbul_StatementIDsSortNumerically(t *testing.T) {
	// 10 statements: lexicographic id sort ("0","1","10","2",...) would
	// misorder id "10" before "2" — this pins the numeric sort.
	doc := `{"/repo/f.js": {"statementMap": {`
	for i := 0; i < 11; i++ {
		if i > 0 {
			doc += ","
		}
		doc += `"` + strconv.Itoa(i) + `": {"start": {"line": ` + strconv.Itoa(i+1) + `}, "end": {"line": ` + strconv.Itoa(i+1) + `}}`
	}
	doc += `}, "s": {}}}`

	blocks, err := ParseCoveragePyIstanbul(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ParseCoveragePyIstanbul: %v", err)
	}
	if len(blocks) != 11 {
		t.Fatalf("len(blocks) = %d, want 11", len(blocks))
	}
	for i, b := range blocks {
		if b.StartLine != i+1 {
			t.Fatalf("blocks[%d].StartLine = %d, want %d (statement ids out of numeric order): %+v", i, b.StartLine, i+1, blocks)
		}
	}
}

func TestParseCoveragePyIstanbul_EmptyObjectErrors(t *testing.T) {
	if _, err := ParseCoveragePyIstanbul(strings.NewReader(`{}`)); err == nil {
		t.Fatal("expected an error for an empty JSON object, got nil")
	}
}

func TestParseCoveragePyIstanbul_MalformedJSONErrors(t *testing.T) {
	if _, err := ParseCoveragePyIstanbul(strings.NewReader(`not json`)); err == nil {
		t.Fatal("expected an error for non-JSON input, got nil")
	}
}
