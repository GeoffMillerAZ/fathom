package extractor

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// slxPackage writes an OPC zip holding parts, in the order given (a .slx is one; the
// fixtures are built here, never committed as binaries).
func slxPackage(t *testing.T, parts ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(parts); i += 2 {
		f, err := w.Create(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(parts[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const slxContentTypes = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>
`

// The pre-R2020a layout: every block inline in blockdiagram.xml. BlockParameterDefaults
// holds template blocks that are no block of the model.
const slxInlineDiagram = `<?xml version="1.0" encoding="utf-8"?>
<ModelInformation Version="1.0">
  <Model Name="plant">
    <P Name="Version">9.0</P>
    <BlockParameterDefaults>
      <Block BlockType="Gain">
        <P Name="Gain">1</P>
      </Block>
    </BlockParameterDefaults>
    <GraphicalInterface>
      <ModelReference>
        <P Name="ModelRefBlockPath">$bdroot/Ctrl/Inner|inner_model</P>
      </ModelReference>
    </GraphicalInterface>
    <System>
      <P Name="Location">[0, 0, 100, 100]</P>
      <Block BlockType="Inport" Name="In1" SID="1">
      </Block>
      <Block BlockType="SubSystem" Name="Ctrl" SID="2">
        <System>
          <Block BlockType="Gain" Name="Gain" SID="3">
            <P Name="Gain">K</P>
          </Block>
          <Block BlockType="ModelReference" Name="Inner" SID="4">
            <P Name="ModelNameDialog">inner_model</P>
          </Block>
        </System>
      </Block>
      <Block BlockType="Gain" Name="Two&#xA;Lines" SID="5"/>
      <Block BlockType="Constant" Name="a/b" SID="6"/>
    </System>
  </Model>
</ModelInformation>
`

func TestReadSimulinkModelLayoutEras(t *testing.T) {
	type want struct {
		path, kind string
		line, end  int
	}
	check := func(t *testing.T, m SimulinkModel, wants []want) {
		t.Helper()
		var got []string
		for _, b := range m.Blocks {
			kind := "block"
			if b.Subsystem {
				kind = "subsystem"
			}
			got = append(got, b.Path+"|"+kind)
		}
		if len(m.Blocks) != len(wants) {
			t.Fatalf("blocks %q, want %d", got, len(wants))
		}
		for i, w := range wants {
			b := m.Blocks[i]
			if got[i] != w.path+"|"+w.kind {
				t.Errorf("block %d = %s, want %s|%s", i, got[i], w.path, w.kind)
			}
			if w.line != 0 && (b.Line != w.line || b.EndLine != w.end) {
				t.Errorf("block %s lines %d-%d, want %d-%d", b.Path, b.Line, b.EndLine, w.line, w.end)
			}
		}
	}

	t.Run("inline before R2020a", func(t *testing.T) {
		src := slxPackage(t, "[Content_Types].xml", slxContentTypes, "simulink/blockdiagram.xml", slxInlineDiagram)
		m, err := ReadSimulinkModel("models/plant.slx", src)
		if err != nil {
			t.Fatal(err)
		}
		if m.Name != "plant" || m.Layout != SimulinkLayoutInline {
			t.Errorf("model %q layout %q", m.Name, m.Layout)
		}
		check(t, m, []want{
			{"plant/In1", "block", 17, 18},
			{"plant/Ctrl", "subsystem", 19, 28},
			{"plant/Ctrl/Gain", "block", 21, 23},
			{"plant/Ctrl/Inner", "block", 24, 26},
			{"plant/Two Lines", "block", 29, 29},
			{"plant/a//b", "block", 30, 30},
		})
		if m.Blocks[2].Part != "simulink/blockdiagram.xml" || m.Blocks[2].BlockType != "Gain" || m.Blocks[2].Name != "Gain" {
			t.Errorf("block fields %+v", m.Blocks[2])
		}
		if m.Blocks[2].Parent != "plant/Ctrl" || m.Blocks[0].Parent != "plant" {
			t.Errorf("parents %q %q", m.Blocks[2].Parent, m.Blocks[0].Parent)
		}
		if got := m.SIDPath["3"]; got != "plant/Ctrl/Gain" {
			t.Errorf("SID 3 -> %q", got)
		}
		if len(m.ModelRefs) != 1 || m.ModelRefs[0] != (SimulinkModelRef{BlockPath: "plant/Ctrl/Inner", ModelName: "inner_model"}) {
			t.Errorf("model refs %+v", m.ModelRefs)
		}
	})

	root := `<?xml version="1.0" encoding="utf-8"?>
<System>
  <P Name="Location">[0, 0, 100, 100]</P>
  <Block BlockType="SubSystem" Name="Ctrl" SID="2">
    <System Ref="system_2"/>
  </Block>
  <Block BlockType="Outport" Name="Out1" SID="9"/>
</System>
`
	sub := `<?xml version="1.0" encoding="utf-8"?>
<System>
  <Block BlockType="Gain" Name="Gain" SID="3">
    <P Name="Gain">K</P>
  </Block>
  <Block BlockType="SubSystem" Name="Deep" SID="7">
    <System Ref="system_7"/>
  </Block>
</System>
`
	deep := `<?xml version="1.0" encoding="utf-8"?>
<System>
  <Block BlockType="ModelReference" Name="Inner" SID="8"/>
</System>
`
	diagram := `<?xml version="1.0" encoding="utf-8"?>
<ModelInformation Version="1.0">
  <Model Name="ctl">
    <System Ref="system_root"/>
  </Model>
</ModelInformation>
`
	systemsWant := []want{
		{"ctl/Ctrl", "subsystem", 4, 6},
		{"ctl/Ctrl/Gain", "block", 3, 5},
		{"ctl/Ctrl/Deep", "subsystem", 6, 8},
		{"ctl/Ctrl/Deep/Inner", "block", 3, 3},
		{"ctl/Out1", "block", 7, 7},
	}

	t.Run("systems parts from R2020a", func(t *testing.T) {
		gi := `<?xml version="1.0" encoding="utf-8"?>
<GraphicalInterface>
  <ModelReference>
    <P Name="ModelRefBlockPath">$bdroot/Ctrl/Deep/Inner|leaf</P>
  </ModelReference>
</GraphicalInterface>
`
		src := slxPackage(t, "[Content_Types].xml", slxContentTypes,
			"simulink/systems/system_7.xml", deep, "simulink/blockdiagram.xml", diagram,
			"simulink/systems/system_root.xml", root, "simulink/systems/system_2.xml", sub,
			"simulink/graphicalInterface.xml", gi)
		m, err := ReadSimulinkModel("ctl.slx", src)
		if err != nil {
			t.Fatal(err)
		}
		if m.Layout != SimulinkLayoutSystems {
			t.Errorf("layout %q", m.Layout)
		}
		check(t, m, systemsWant)
		if m.Blocks[1].Part != "simulink/systems/system_2.xml" {
			t.Errorf("part %q", m.Blocks[1].Part)
		}
		if len(m.ModelRefs) != 1 || m.ModelRefs[0] != (SimulinkModelRef{BlockPath: "ctl/Ctrl/Deep/Inner", ModelName: "leaf"}) {
			t.Errorf("model refs %+v", m.ModelRefs)
		}
	})

	t.Run("graphicalInterface.json from R2024b", func(t *testing.T) {
		gi := `{"GraphicalInterface": {"ModelReferences": [{"BlockPath": "$bdroot/Ctrl/Deep/Inner", "ModelName": "leaf"}]}}`
		src := slxPackage(t, "simulink/blockdiagram.xml", diagram, "simulink/systems/system_root.xml", root,
			"simulink/systems/system_2.xml", sub, "simulink/systems/system_7.xml", deep,
			"simulink/graphicalInterface.json", gi)
		m, err := ReadSimulinkModel("ctl.slx", src)
		if err != nil {
			t.Fatal(err)
		}
		check(t, m, systemsWant)
		if len(m.ModelRefs) != 1 || m.ModelRefs[0].ModelName != "leaf" || m.ModelRefs[0].BlockPath != "ctl/Ctrl/Deep/Inner" {
			t.Errorf("model refs %+v", m.ModelRefs)
		}
	})

	t.Run("blockDiagram.json from R2026b", func(t *testing.T) {
		gi := `{"_mw_element_name": "GraphicalInterface", "ModelReferences": {"BlockPath": "$bdroot/Ctrl/Deep/Inner", "ModelName": "leaf"}}`
		src := slxPackage(t, "simulink/blockDiagram.json", `{"BlockDiagram": {"Name": "ignored"}}`,
			"simulink/systems/system_root.xml", root, "simulink/systems/system_2.xml", sub,
			"simulink/systems/system_7.xml", deep, "simulink/graphicalInterface.json", gi)
		m, err := ReadSimulinkModel("lib/ctl.slx", src)
		if err != nil {
			t.Fatal(err)
		}
		check(t, m, systemsWant)
		if len(m.ModelRefs) != 1 || m.ModelRefs[0].ModelName != "leaf" {
			t.Errorf("model refs %+v", m.ModelRefs)
		}
	})

	t.Run("a cycle of refs and an unreached part", func(t *testing.T) {
		loop := `<System><Block BlockType="SubSystem" Name="Back" SID="2"><System Ref="system_2"/></Block></System>`
		orphan := `<System><Block BlockType="Gain" Name="Lost" SID="11"/></System>`
		src := slxPackage(t, "simulink/systems/system_root.xml", root, "simulink/systems/system_2.xml", loop,
			"simulink/systems/system_99.xml", orphan)
		m, err := ReadSimulinkModel("ctl.slx", src)
		if err != nil {
			t.Fatal(err)
		}
		check(t, m, []want{{"ctl/Ctrl", "subsystem", 0, 0}, {"ctl/Ctrl/Back", "subsystem", 0, 0}, {"ctl/Out1", "block", 0, 0}, {"ctl/Lost", "block", 0, 0}})
	})
}

func TestReadSimulinkModelCaps(t *testing.T) {
	t.Run("entries", func(t *testing.T) {
		parts := []string{}
		for i := 0; i <= SimulinkMaxEntries; i++ {
			parts = append(parts, "simulink/p"+itoa(i)+".xml", "<System/>")
		}
		_, err := ReadSimulinkModel("m.slx", slxPackage(t, parts...))
		if !errors.Is(err, ErrSimulinkCap) {
			t.Errorf("err = %v, want the entry cap", err)
		}
	})
	t.Run("ratio", func(t *testing.T) {
		bomb := "<System>" + strings.Repeat(" ", 4<<20) + "</System>"
		_, err := ReadSimulinkModel("m.slx", slxPackage(t, "simulink/systems/system_root.xml", bomb))
		if !errors.Is(err, ErrSimulinkCap) {
			t.Errorf("err = %v, want the ratio cap", err)
		}
	})
	t.Run("inflated bytes", func(t *testing.T) {
		var parts []string
		// Incompressible enough to pass the ratio cap, together over the byte cap.
		chunk := make([]byte, 1<<20)
		seed := uint32(7)
		for i := range chunk {
			seed = seed*1664525 + 1013904223
			chunk[i] = 'a' + byte(seed>>24)%26
		}
		body := "<System><!--" + string(chunk) + "--></System>"
		for i := 0; i*len(body) <= SimulinkMaxInflated; i++ {
			parts = append(parts, "simulink/systems/system_"+itoa(i)+".xml", body)
		}
		_, err := ReadSimulinkModel("m.slx", slxPackage(t, parts...))
		if !errors.Is(err, ErrSimulinkCap) {
			t.Errorf("err = %v, want the inflated-bytes cap", err)
		}
	})
	t.Run("not a package", func(t *testing.T) {
		if _, err := ReadSimulinkModel("m.slx", []byte("PK\x03\x04garbage")); err == nil {
			t.Error("a corrupt package read without error")
		}
	})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

const mdlClassic = `# A classic model file
Model {
  Name			  "plant"
  Version		  7.9
  BlockParameterDefaults {
    Block {
      BlockType		      Gain
      Gain		      "1"
    }
  }
  GraphicalInterface {
    ModelReference {
      ModelRefBlockPath	      "$bdroot/Ctrl/Inner|inner_model"
    }
  }
  System {
    Name		    "plant"
    Block {
      BlockType		      SubSystem
      Name		      "Ctrl"
      SID		      "2"
      System {
	Name			"Ctrl"
	Block {
	  BlockType		  ModelReference
	  Name			  "Inner"
	  SID			  "4"
	}
	Block {
	  BlockType		  Gain
	  Name			  "Two\nLines"
	  SID			  "5"
	  Gain			  "K"
	}
      }
    }
    Block {
      BlockType		      Constant
      Name		      "a/b \"q\" "
      "continued"
      SID		      "6"
      Value		      "{1, 2}"
    }
    Line {
      SrcBlock		      "Ctrl"
    }
  }
}
MatData {
  NumRecords		  0
}
`

func TestReadSimulinkModelMDL(t *testing.T) {
	m, err := ReadSimulinkModel("models/plant.mdl", []byte(mdlClassic))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "plant" || m.Layout != SimulinkLayoutMDL {
		t.Errorf("model %q layout %q", m.Name, m.Layout)
	}
	var got []string
	for _, b := range m.Blocks {
		got = append(got, b.Path+"|"+b.BlockType+"|"+itoa(b.Line)+"-"+itoa(b.EndLine))
	}
	want := []string{
		"plant/Ctrl|SubSystem|18-36",
		"plant/Ctrl/Inner|ModelReference|24-28",
		"plant/Ctrl/Two Lines|Gain|29-34",
		`plant/a//b "q" continued|Constant|37-43`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !m.Blocks[0].Subsystem || m.Blocks[1].Subsystem {
		t.Errorf("subsystem flags %+v", m.Blocks[:2])
	}
	if m.SIDPath["5"] != "plant/Ctrl/Two Lines" {
		t.Errorf("SID 5 -> %q", m.SIDPath["5"])
	}
	if len(m.ModelRefs) != 1 || m.ModelRefs[0] != (SimulinkModelRef{BlockPath: "plant/Ctrl/Inner", ModelName: "inner_model"}) {
		t.Errorf("model refs %+v", m.ModelRefs)
	}

	lib := "Library {\n  Name \"lib\"\n  System {\n    Block {\n      BlockType Gain\n      Name \"G\"\n    }\n  }\n}\n"
	if m, err := ReadSimulinkModel("lib.mdl", []byte(lib)); err != nil || len(m.Blocks) != 1 || m.Blocks[0].Path != "lib/G" {
		t.Errorf("library: %+v, %v", m, err)
	}

	for _, banner := range []string{"# MathWorks OPC Text Package\n", "# Simulink Text Package\n"} {
		_, err := ReadSimulinkModel("new.mdl", []byte(banner+"Model {\n  Version 10.0\n}\n__MWOPC_PACKAGE_BEGIN__\n"))
		if !errors.Is(err, ErrSimulinkTextPackage) {
			t.Errorf("%q: err = %v, want the text-package disclosure", banner, err)
		}
	}

	if _, err := ReadSimulinkModel("broken.mdl", []byte("Model {\n  System {\n    Block {\n")); err != nil {
		t.Errorf("an unclosed model should read what it holds: %v", err)
	}
}

func TestSimulinkExtractorNodes(t *testing.T) {
	src := slxPackage(t, "simulink/blockdiagram.xml", slxInlineDiagram)
	nodes, err := NewSimulinkExtractor().Extract("svc", "models/plant.slx", src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, n.Name+"|"+string(n.Kind)+"|"+n.Signature+"|"+n.Package)
	}
	want := []string{
		"plant|SimulinkModel||plant",
		"plant/In1|SimulinkBlock|Inport|plant",
		"plant/Ctrl|SimulinkSubsystem|SubSystem|plant",
		"plant/Ctrl/Gain|SimulinkBlock|Gain|plant",
		"plant/Ctrl/Inner|SimulinkBlock|ModelReference|plant",
		"plant/Two Lines|SimulinkBlock|Gain|plant",
		"plant/a//b|SimulinkBlock|Constant|plant",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("nodes\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if nodes, err := NewSimulinkExtractor().Extract("svc", "big.slx", []byte("PK\x03\x04junk")); err != nil || len(nodes) != 0 {
		t.Errorf("an unreadable package yields nothing and no error: %d nodes, %v", len(nodes), err)
	}
}

func TestReadSimulinkModelSignals(t *testing.T) {
	sigs := func(m SimulinkModel) string {
		var out []string
		for _, s := range m.Signals {
			out = append(out, s.Src+" -> "+s.Dst)
		}
		return strings.Join(out, "\n")
	}
	root := `<System>
  <Block BlockType="Inport" Name="In1" SID="1"/>
  <Block BlockType="SubSystem" Name="Ctrl" SID="2"><System Ref="system_2"/></Block>
  <Block BlockType="Scope" Name="Scope" SID="3"/>
  <Line><P Name="Src">1#out:1</P><P Name="Dst">2#in:1</P></Line>
  <Line><P Name="Src">2#out:1</P>
    <Branch><P Name="Dst">3#in:1</P></Branch>
    <Branch><Branch><P Name="Dst">99#in:1</P></Branch><P Name="Dst">1#in:1</P></Branch>
  </Line>
</System>`
	sub := `<System>
  <Block BlockType="Gain" Name="K" SID="4"/>
  <Block BlockType="Outport" Name="Out1" SID="5"/>
  <Line><P Name="Src">4#out:1</P><P Name="Dst">5#in:1</P></Line>
</System>`
	m, err := ReadSimulinkModel("ctl.slx", slxPackage(t, "simulink/systems/system_root.xml", root, "simulink/systems/system_2.xml", sub))
	if err != nil {
		t.Fatal(err)
	}
	want := "ctl/Ctrl/K -> ctl/Ctrl/Out1\nctl/In1 -> ctl/Ctrl\nctl/Ctrl -> ctl/Scope\nctl/Ctrl -> ctl/In1"
	if got := sigs(m); got != want {
		t.Errorf("slx signals\n%s\nwant\n%s", got, want)
	}

	mdl := "Model {\n  Name \"p\"\n  System {\n    Block {\n      BlockType Constant\n      Name \"Two\\nLines\"\n    }\n" +
		"    Block {\n      BlockType Gain\n      Name \"G\"\n      SID \"7\"\n    }\n" +
		"    Line {\n      SrcBlock \"Two\\nLines\"\n      SrcPort 1\n      Branch {\n        DstBlock \"G\"\n        DstPort 1\n      }\n      Branch {\n        DstBlock \"Missing\"\n      }\n    }\n" +
		"    Line {\n      Src \"7#out:1\"\n      Dst \"7#in:1\"\n    }\n  }\n}\n"
	m, err = ReadSimulinkModel("p.mdl", []byte(mdl))
	if err != nil {
		t.Fatal(err)
	}
	if got := sigs(m); got != "p/Two Lines -> p/G\np/G -> p/G" {
		t.Errorf("mdl signals\n%s", got)
	}

	nodes, _ := NewSimulinkExtractor().Extract("svc", "p.mdl", []byte(mdl))
	for _, n := range nodes {
		if n.Name == "p/Two Lines" && strings.Join(n.Calls, ",") != "p/G" {
			t.Errorf("calls of %s = %v", n.Name, n.Calls)
		}
	}
}

func TestSimulinkExtractDocModelRefs(t *testing.T) {
	gi := `{"GraphicalInterface": {"ModelReferences": [{"BlockPath": "$bdroot/Plant", "ModelName": "plant"}]}}`
	root := `<System><Block BlockType="ModelReference" Name="Plant" SID="1"/></System>`
	src := slxPackage(t, "simulink/systems/system_root.xml", root, "simulink/graphicalInterface.json", gi)
	var x DocResulter = NewSimulinkExtractor()
	r, err := x.ExtractDoc("svc", "sim/harness.slx", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Roots) != 1 || r.Roots[0] != "harness" {
		t.Errorf("roots %v", r.Roots)
	}
	want := DocCrossFileRef{From: "harness/Plant", Edge: SimulinkModelRefEdge, Raw: "plant", Target: "plant", Strategy: "module-source", Line: 1}
	if len(r.CrossFile) != 1 || r.CrossFile[0].From != want.From || r.CrossFile[0].Raw != want.Raw || r.CrossFile[0].Strategy != want.Strategy || r.CrossFile[0].Edge != want.Edge {
		t.Errorf("cross-file %+v, want %+v", r.CrossFile, want)
	}
}

// TestReadSimulinkModelStrayEndTags is the S-R1 regression: a part whose end tags outnumber
// or mismatch its start tags is a bounded ErrSimulinkCap error, never a panic.
func TestReadSimulinkModelStrayEndTags(t *testing.T) {
	for name, part := range map[string]string{
		"extra end tags":  `<System><Block Name="a" SID="1"/></System></x></y>`,
		"mismatched end":  `<System><Block Name="a" SID="1"></System></Block>`,
		"unclosed at end": `<System><Block Name="a" SID="1">`,
		"stray end only":  `</System>`,
	} {
		t.Run(name, func(t *testing.T) {
			for _, entry := range []string{"simulink/systems/system_root.xml", "simulink/blockdiagram.xml", "simulink/graphicalInterface.xml"} {
				src := slxPackage(t, "[Content_Types].xml", slxContentTypes, entry, part)
				_, err := ReadSimulinkModel("m.slx", src)
				if entry == "simulink/graphicalInterface.xml" {
					continue // a broken interface part drops the references, as before
				}
				if !errors.Is(err, ErrSimulinkCap) {
					t.Errorf("%s: err = %v, want an ErrSimulinkCap syntax error", entry, err)
				}
				if r, err := (SimulinkExtractor{}).ExtractDoc("svc", "m.slx", src); err != nil || len(r.Nodes) != 0 {
					t.Errorf("%s: ExtractDoc = %d nodes, %v; want none, nil", entry, len(r.Nodes), err)
				}
			}
		})
	}
	t.Run("mdl", func(t *testing.T) {
		m, err := ReadSimulinkModel("m.mdl", []byte("Model {\n}\n}\n}\nSystem {\n"))
		if err != nil || len(m.Blocks) != 0 {
			t.Errorf("stray braces: %d blocks, %v", len(m.Blocks), err)
		}
	})
}
