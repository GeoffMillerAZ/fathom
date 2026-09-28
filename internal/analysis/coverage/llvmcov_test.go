package coverage

import (
	"reflect"
	"strings"
	"testing"
)

const llvmCovSample = `{
  "version": "2.0.1",
  "type": "llvm.coverage.json.export",
  "data": [
    {
      "files": [
        {
          "filename": "src/widget.c",
          "segments": [
            [1, 1, 3, true, true, false],
            [2, 1, 0, true, true, false],
            [2, 5, 3, true, false, false],
            [3, 1, 0, false, false, false],
            [4, 1, 0, true, true, true]
          ]
        }
      ]
    }
  ]
}`

func TestParseLLVMCov(t *testing.T) {
	blocks, err := ParseLLVMCov(strings.NewReader(llvmCovSample))
	if err != nil {
		t.Fatalf("ParseLLVMCov: %v", err)
	}
	// Line 1: count 3 -> covered. Line 2: two segments, count 0 then count
	// 3 -> OR-merges to covered. Line 3: hasCount false -> skipped
	// entirely (never emitted). Line 4: isGapRegion true -> skipped.
	want := []ForeignBlock{
		{File: "src/widget.c", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "src/widget.c", StartLine: 2, EndLine: 2, Statements: 1, Covered: 1},
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

func TestParseLLVMCov_UncoveredLineStaysUncovered(t *testing.T) {
	doc := `{"type": "llvm.coverage.json.export", "data": [{"files": [
		{"filename": "f.c", "segments": [[5, 1, 0, true, true, false]]}
	]}]}`
	blocks, err := ParseLLVMCov(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ParseLLVMCov: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Covered != 0 || blocks[0].StartLine != 5 {
		t.Fatalf("blocks = %+v, want one uncovered block at line 5", blocks)
	}
}

func TestParseLLVMCov_WrongSchemaTypeErrors(t *testing.T) {
	if _, err := ParseLLVMCov(strings.NewReader(`{"type": "something.else", "data": []}`)); err == nil {
		t.Fatal("expected an error for an unrecognized schema type, got nil")
	}
}

func TestParseLLVMCov_MalformedSegmentErrors(t *testing.T) {
	doc := `{"type": "llvm.coverage.json.export", "data": [{"files": [
		{"filename": "f.c", "segments": [[1, 1]]}
	]}]}`
	if _, err := ParseLLVMCov(strings.NewReader(doc)); err == nil {
		t.Fatal("expected an error for a short segment row, got nil")
	}
}

func TestParseLLVMCov_MalformedJSONErrors(t *testing.T) {
	if _, err := ParseLLVMCov(strings.NewReader(`not json`)); err == nil {
		t.Fatal("expected an error for non-JSON input, got nil")
	}
}
