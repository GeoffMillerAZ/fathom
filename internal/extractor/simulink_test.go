package extractor

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
	"unsafe"
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

// TestReadSimulinkModelStructureCaps is S-R2: the depth and element caps hold while a part
// or a .mdl is tokenized, before a tree is built past them. The .slx parts compress as a real
// part does (slxNoise), far inside the ratio cap, as the 56:1 nesting that once built a
// 10.8M-deep tree did.
func TestReadSimulinkModelStructureCaps(t *testing.T) {
	slx := func(part string) []byte {
		return slxPackage(t, "[Content_Types].xml", slxContentTypes, "simulink/systems/system_root.xml", part)
	}
	cases := map[string][]byte{
		// 32 MiB of nesting: it compresses far inside the ratio cap and once built a 10.8M-deep tree.
		"deep slx":     slx(slxNoise(1<<20, "<%s>", true)),
		"flat slx":     slx(slxNoise((SimulinkMaxInflated-32)/(3+slxNoiseWidth), "<%s/>", false)),
		"deep mdl":     []byte("Model {\n" + strings.Repeat("A {\n", SimulinkMaxDepth+1) + strings.Repeat("}\n", SimulinkMaxDepth+2)),
		"flat mdl":     []byte("Model {\n" + strings.Repeat("A {\n}\n", SimulinkMaxElements+1) + "}\n"),
		"wide spliced": nil,
	}
	// Each part nests 64 deep and names the next: the splice is deeper than any one part.
	var chain []string
	chain = append(chain, "[Content_Types].xml", slxContentTypes)
	for i := 0; i < SimulinkMaxDepth/32; i++ {
		ref := "system_root"
		if i > 0 {
			ref = "system_" + itoa(i)
		}
		chain = append(chain, "simulink/systems/"+ref+".xml",
			"<System>"+strings.Repeat(`<Block Name="b" SID="1"><System>`, 32)+`<Block Name="s" SID="2"><System Ref="system_`+itoa(i+1)+`"/></Block>`+strings.Repeat("</System></Block>", 32)+"</System>")
	}
	cases["wide spliced"] = slxPackage(t, chain...)
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			rel := "m.slx"
			if strings.HasSuffix(name, "mdl") {
				rel = "m.mdl"
			}
			_, err := ReadSimulinkModel(rel, src)
			if !errors.Is(err, ErrSimulinkCap) {
				t.Errorf("err = %v, want ErrSimulinkCap", err)
			}
		})
	}
	t.Run("under the caps", func(t *testing.T) {
		deep := "<System>" + strings.Repeat(`<Block Name="b" SID="1"><System>`, SimulinkMaxDepth/2-2) + strings.Repeat("</System></Block>", SimulinkMaxDepth/2-2) + "</System>"
		m, err := ReadSimulinkModel("m.slx", slx(deep))
		if err != nil || len(m.Blocks) != SimulinkMaxDepth/2-2 {
			t.Errorf("a model inside the depth cap: %d blocks, %v", len(m.Blocks), err)
		}
	})
}

// slxNoiseWidth is the width of an slxNoise name.
const slxNoiseWidth = 3

// slxNoise is a <System> part of n elements written by format from pseudo-random names (so
// the part compresses as a real one does, far inside the ratio cap): nested, each inside the
// last and closed in turn, or flat.
func slxNoise(n int, format string, nested bool) string {
	var b, closing strings.Builder
	b.WriteString("<System>")
	seed := uint32(11)
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var nm [slxNoiseWidth]byte
		for j := range nm {
			seed = seed*1664525 + 1013904223
			nm[j] = 'a' + byte(seed>>24)%26
		}
		b.WriteString(strings.Replace(format, "%s", string(nm[:]), 1))
		if nested {
			names = append(names, string(nm[:]))
		}
	}
	for i := len(names) - 1; i >= 0; i-- {
		closing.WriteString("</" + names[i] + ">")
	}
	b.WriteString(closing.String())
	b.WriteString("</System>")
	return b.String()
}

// slxDeclareCompressed rewrites the central-directory compressed size of entry name, a size
// an attacker controls.
func slxDeclareCompressed(t *testing.T, src []byte, name string, size uint32) []byte {
	t.Helper()
	out := append([]byte(nil), src...)
	for i := bytes.Index(out, []byte("PK\x01\x02")); i >= 0; {
		nlen := int(out[i+28]) | int(out[i+29])<<8
		if string(out[i+46:i+46+nlen]) == name {
			out[i+20], out[i+21], out[i+22], out[i+23] = byte(size), byte(size>>8), byte(size>>16), byte(size>>24)
			return out
		}
		next := bytes.Index(out[i+4:], []byte("PK\x01\x02"))
		if next < 0 {
			break
		}
		i += 4 + next
	}
	t.Fatalf("no central-directory entry %s", name)
	return nil
}

// TestReadSimulinkModelRealRatio is S-R3: the ratio cap counts the compressed bytes the
// inflater reads, never the compressed size the package declares.
func TestReadSimulinkModelRealRatio(t *testing.T) {
	root := "simulink/systems/system_root.xml"
	noise := make([]byte, 64<<10)
	seed := uint32(3)
	for i := range noise {
		seed = seed*1664525 + 1013904223
		noise[i] = byte(seed >> 24)
	}
	// A part that inflates about 1000:1, then a stored entry the false size can reach into.
	bomb := func() []byte {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		f, _ := w.Create(root)
		f.Write([]byte("<System>" + strings.Repeat(" ", 4<<20) + "</System>"))
		pad, _ := w.CreateHeader(&zip.FileHeader{Name: "padding.bin", Method: zip.Store})
		pad.Write(noise)
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}()
	zr, err := zip.NewReader(bytes.NewReader(bomb), int64(len(bomb)))
	if err != nil {
		t.Fatal(err)
	}
	real := zr.File[0].CompressedSize64
	if r := zr.File[0].UncompressedSize64 / real; r < SimulinkMaxRatio {
		t.Fatalf("the bomb inflates only %d:1", r)
	}
	// The declared compressed size reaches into the next entry, so the declared ratio
	// (4 MiB : about 70 KiB) is inside the cap; the bytes inflated per byte read are not.
	_, err = ReadSimulinkModel("m.slx", slxDeclareCompressed(t, bomb, root, uint32(real)+uint32(len(noise))))
	if !errors.Is(err, ErrSimulinkCap) || !strings.Contains(err.Error(), "inflates past 200:1") {
		t.Errorf("err = %v, want the ratio cap", err)
	}
	// A part that inflates as a real one does reads whole.
	m, err := ReadSimulinkModel("m.slx", slxPackage(t, root, slxNoise(1000, `<Block Name="%s" SID="1"/>`, false)))
	if err != nil || len(m.Blocks) != 1000 {
		t.Errorf("%d blocks, %v", len(m.Blocks), err)
	}
}

// simulinkNested is a .slx system part of depth blocks, each named name and inside the last,
// the deepest system holding inner.
func simulinkNested(depth int, name, inner string) string {
	return "<System>" + strings.Repeat(`<Block BlockType="SubSystem" Name="`+name+`"><System>`, depth) + inner +
		strings.Repeat("</System></Block>", depth) + "</System>"
}

// mdlNested is a .mdl model of depth blocks, each named name and inside the last, the deepest
// system holding inner.
func mdlNested(depth int, name, inner string) string {
	return "Model {\n System {\n" + strings.Repeat("Block {\nBlockType SubSystem\nName \""+name+"\"\nSystem {\n", depth) + inner +
		strings.Repeat("}\n}\n", depth) + " }\n}\n"
}

// TestReadSimulinkModelSharedPaths is S3-R2: a name endpoint resolves to the block's stored
// path, never a fresh copy, and a line drawn twice is one signal.
func TestReadSimulinkModelSharedPaths(t *testing.T) {
	line := `<Line><P Name="SrcBlock">s</P>` + strings.Repeat(`<Branch><P Name="DstBlock">d</P></Branch>`, 50) + `</Line>`
	inner := `<Block Name="s" SID="1"/><Block Name="d" SID="2"/>` + line + line
	m, err := ReadSimulinkModel("m.slx", slxPackage(t, "simulink/systems/system_root.xml", simulinkNested(4, "sub", inner)))
	if err != nil {
		t.Fatal(err)
	}
	mdlLine := "Line {\nSrcBlock \"s\"\n" + strings.Repeat("Branch {\nDstBlock \"d\"\n}\n", 50) + "}\n"
	mdl, err := ReadSimulinkModel("m.mdl", []byte(mdlNested(4, "sub", "Block {\nName \"s\"\n}\nBlock {\nName \"d\"\n}\n"+mdlLine+mdlLine)))
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]SimulinkModel{"slx": m, "mdl": mdl} {
		stored := map[*byte]bool{}
		for _, b := range m.Blocks {
			stored[unsafe.StringData(b.Path)] = true
		}
		if len(m.Signals) != 1 || m.Signals[0] != (SimulinkSignal{Src: "m/sub/sub/sub/sub/s", Dst: "m/sub/sub/sub/sub/d"}) {
			t.Fatalf("%s: signals %v, want the one line s -> d", name, m.Signals)
		}
		if s := m.Signals[0]; !stored[unsafe.StringData(s.Src)] || !stored[unsafe.StringData(s.Dst)] {
			t.Errorf("%s: a signal's endpoint is a fresh copy of a block's path", name)
		}
	}
}

// TestReadSimulinkModelAdversarial is S-R4: each hostile package ends in a bounded
// ErrSimulinkCap error, never a panic and never a partial model.
func TestReadSimulinkModelAdversarial(t *testing.T) {
	root := "simulink/systems/system_root.xml"
	letters := func(n int) string {
		b := make([]byte, n)
		s := uint32(5)
		for i := range b {
			s = s*1664525 + 1013904223
			b[i] = 'a' + byte(s>>24)%26
		}
		return string(b)
	}
	inner := slxPackage(t, root, `<System><Block Name="a" SID="1"/></System>`)
	cases := map[string][]byte{
		"duplicate entry names": slxPackage(t, root, `<System><Block Name="a" SID="1"/></System>`,
			root, `<System><Block Name="b" SID="2"/></System>`),
		"nested zip":         slxPackage(t, root, string(inner)),
		"huge attribute":     slxPackage(t, root, `<System><Block Name="`+letters(8<<20)+`" SID="1"/></System>`),
		"huge kept text":     slxPackage(t, root, `<System><Line><P Name="Src">`+letters(8<<20)+`</P></Line></System>`),
		"huge mdl attribute": []byte("Model {\n System {\n  Block {\n   Name \"" + letters(1<<20) + "\"\n  }\n }\n}\n"),
		"huge mdl continued": []byte("Model {\n System {\n  Block {\n   Name \"a\"\n" + strings.Repeat("   \""+letters(1000)+"\"\n", 100) + "  }\n }\n}\n"),
		"deep nesting":       slxPackage(t, root, slxNoise(SimulinkMaxDepth+1, "<%s>", true)),
		"flat flood":         slxPackage(t, root, slxNoise(SimulinkMaxElements+1, "<%s/>", false)),
		"stray end tags":     slxPackage(t, root, `<System><Block Name="a" SID="1"/></System></x></y>`),
		// S3-R1: a start tag is bounded on its bytes before the tokenizer builds its attributes.
		"attribute flood": slxPackage(t, root, `<System><Block Name="a" SID="1"`+strings.Repeat(` x="1"`, SimulinkMaxAttrs+1)+`/></System>`),
		"long start tag":  slxPackage(t, root, `<System><Block Name="a" Note="`+letters(SimulinkMaxStartTag)+`"/></System>`),
		// S3-R2: paths grow as name length times depth squared; both path caps hold.
		"deep long names":     slxPackage(t, root, simulinkNested(64, letters(16<<10), "")),
		"long path":           slxPackage(t, root, simulinkNested(SimulinkMaxPath/SimulinkMaxValue+1, letters(SimulinkMaxValue), "")),
		"deep long names mdl": []byte(mdlNested(64, letters(16<<10), "")),
		"long line mdl":       []byte("Model {\n System {\n  Block {\n   Name \"a\"\n  }\n  Note " + letters(MaxSimulinkLine) + "\n }\n}\n"),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			rel := "m.slx"
			if strings.Contains(name, "mdl") {
				rel = "m.mdl"
			}
			m, err := ReadSimulinkModel(rel, src)
			if !errors.Is(err, ErrSimulinkCap) {
				t.Errorf("err = %v, want ErrSimulinkCap", err)
			}
			if len(m.Blocks) != 0 {
				t.Errorf("%d blocks read from a refused model", len(m.Blocks))
			}
		})
	}
	t.Run("an ignored nested package", func(t *testing.T) {
		m, err := ReadSimulinkModel("m.slx", slxPackage(t, root, `<System><Block Name="a" SID="1"/></System>`, "simulink/inner.slx", string(inner)))
		if err != nil || len(m.Blocks) != 1 {
			t.Errorf("%d blocks, %v; a part the reader does not read is never opened", len(m.Blocks), err)
		}
	})
}
