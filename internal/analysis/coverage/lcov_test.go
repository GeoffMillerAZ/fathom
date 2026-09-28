package coverage

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLcov(t *testing.T) {
	in := `TN:
SF:src/widget.py
DA:1,1
DA:2,0
DA:5,3
FN:1,widget
FNDA:1,widget
FNF:1
FNH:1
LF:3
LH:2
end_of_record
`
	blocks, err := ParseLcov(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseLcov: %v", err)
	}
	want := []ForeignBlock{
		{File: "src/widget.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "src/widget.py", StartLine: 2, EndLine: 2, Statements: 1, Covered: 0},
		{File: "src/widget.py", StartLine: 5, EndLine: 5, Statements: 1, Covered: 1},
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

func TestParseLcov_MultipleFilesAndPerTestAttribution(t *testing.T) {
	in := `TN:unit
SF:src/a.rb
DA:1,2
end_of_record
TN:integration
SF:src/b.rb
DA:10,0
end_of_record
`
	blocks, err := ParseLcov(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseLcov: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2: %+v", len(blocks), blocks)
	}
	if blocks[0].File != "src/a.rb" || len(blocks[0].Tests) != 1 || blocks[0].Tests[0] != "unit" {
		t.Errorf("blocks[0] = %+v, want File=src/a.rb Tests=[unit]", blocks[0])
	}
	if blocks[1].File != "src/b.rb" || len(blocks[1].Tests) != 1 || blocks[1].Tests[0] != "integration" || blocks[1].Covered != 0 {
		t.Errorf("blocks[1] = %+v, want File=src/b.rb Tests=[integration] Covered=0", blocks[1])
	}
}

func TestParseLcov_DAOutsideRecordErrors(t *testing.T) {
	if _, err := ParseLcov(strings.NewReader("DA:1,1\n")); err == nil {
		t.Fatal("expected an error for a DA line before any SF:, got nil")
	}
}

func TestParseLcov_MalformedDAErrors(t *testing.T) {
	in := "SF:src/a.rb\nDA:notanumber,1\nend_of_record\n"
	if _, err := ParseLcov(strings.NewReader(in)); err == nil {
		t.Fatal("expected an error for a malformed DA line number, got nil")
	}
}

func TestParseLcov_Empty(t *testing.T) {
	blocks, err := ParseLcov(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseLcov(empty): %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("len(blocks) = %d, want 0", len(blocks))
	}
}
