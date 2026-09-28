package coverage

import (
	"reflect"
	"strings"
	"testing"
)

const coberturaSample = `<?xml version="1.0"?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage line-rate="0.5" branch-rate="0.0" version="1.9">
  <packages>
    <package name="widget">
      <classes>
        <class name="Widget" filename="src/widget.py">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>
`

const jacocoSample = `<?xml version="1.0" encoding="UTF-8"?>
<report name="widget">
  <package name="com/widget">
    <sourcefile name="Widget.java">
      <line nr="1" mi="0" ci="2" mb="0" cb="0"/>
      <line nr="2" mi="1" ci="0" mb="0" cb="0"/>
    </sourcefile>
  </package>
</report>
`

func TestParseCoberturaJaCoCo(t *testing.T) {
	blocks, err := ParseCoberturaJaCoCo(strings.NewReader(coberturaSample))
	if err != nil {
		t.Fatalf("ParseCoberturaJaCoCo: %v", err)
	}
	want := []ForeignBlock{
		{File: "src/widget.py", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "src/widget.py", StartLine: 2, EndLine: 2, Statements: 1, Covered: 0},
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

func TestParseCoberturaJaCoCo_JaCoCo(t *testing.T) {
	blocks, err := ParseCoberturaJaCoCo(strings.NewReader(jacocoSample))
	if err != nil {
		t.Fatalf("ParseCoberturaJaCoCo: %v", err)
	}
	want := []ForeignBlock{
		{File: "Widget.java", StartLine: 1, EndLine: 1, Statements: 1, Covered: 1},
		{File: "Widget.java", StartLine: 2, EndLine: 2, Statements: 1, Covered: 0},
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

func TestParseCoberturaJaCoCo_UnknownRootErrors(t *testing.T) {
	if _, err := ParseCoberturaJaCoCo(strings.NewReader(`<?xml version="1.0"?><somethingelse/>`)); err == nil {
		t.Fatal("expected an error for an unrecognized XML root element, got nil")
	}
}

func TestParseCoberturaJaCoCo_MalformedXMLErrors(t *testing.T) {
	if _, err := ParseCoberturaJaCoCo(strings.NewReader(`not xml at all`)); err == nil {
		t.Fatal("expected an error for non-XML input, got nil")
	}
}
