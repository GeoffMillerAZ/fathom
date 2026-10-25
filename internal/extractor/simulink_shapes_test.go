package extractor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// simulinkLetters is n pseudo-random letters, so a name compresses as a real one does.
func simulinkLetters(n int, seed uint32) string {
	b := make([]byte, n)
	for i := range b {
		seed = seed*1664525 + 1013904223
		b[i] = 'a' + byte(seed>>24)%26
	}
	return string(b)
}

// simulinkFanPairs is a model of kids children of one parent named parent, every child's
// output joined to every child's input: a .mdl when mdl, else a .slx system part.
func simulinkFanPairs(kids int, parent string, mdl bool) string {
	var b strings.Builder
	if mdl {
		b.WriteString("Model {\n System {\n  Block {\n   BlockType SubSystem\n   Name \"" + parent + "\"\n   System {\n")
		for i := 0; i < kids; i++ {
			fmt.Fprintf(&b, "Block {\nName \"c%d\"\nSID \"%d\"\n}\n", i, i+1)
		}
		for i := 0; i < kids; i++ {
			fmt.Fprintf(&b, "Line {\nSrc \"%d#out:1\"\n", i+1)
			for j := 0; j < kids; j++ {
				fmt.Fprintf(&b, "Branch {\nDst \"%d#in:1\"\n}\n", j+1)
			}
			b.WriteString("}\n")
		}
		b.WriteString("   }\n  }\n }\n}\n")
		return b.String()
	}
	b.WriteString(`<System><Block BlockType="SubSystem" Name="` + parent + `"><System>`)
	for i := 0; i < kids; i++ {
		fmt.Fprintf(&b, `<Block Name="c%d" SID="%d"/>`, i, i+1)
	}
	for i := 0; i < kids; i++ {
		fmt.Fprintf(&b, `<Line><P Name="Src">%d#out:1</P>`, i+1)
		for j := 0; j < kids; j++ {
			fmt.Fprintf(&b, `<P Name="Dst">%d#in:1</P>`, j+1)
		}
		b.WriteString(`</Line>`)
	}
	b.WriteString(`</System></Block></System>`)
	return b.String()
}

// simulinkHashburn is a model whose one leaf block, at the end of depth subsystems each
// named name, is a line's source and each of its dsts destinations, each to a pseudo-random
// port (so the part compresses as a real one does): a .mdl when mdl, else a .slx system part.
func simulinkHashburn(depth int, name string, dsts int, mdl bool) string {
	var b strings.Builder
	format := `<P Name="Dst">1#in:%d</P>`
	if mdl {
		format = "Branch {\nDst \"1#in:%d\"\n}\n"
	}
	for i, seed := 0, uint32(13); i < dsts; i++ {
		seed = seed*1664525 + 1013904223
		fmt.Fprintf(&b, format, seed>>27)
	}
	if mdl {
		return mdlNested(depth, name, "Block {\nName \"x\"\nSID \"1\"\n}\nLine {\nSrc \"1#out:1\"\n"+b.String()+"}\n")
	}
	return simulinkNested(depth, name, `<Block Name="x" SID="1"/><Line><P Name="Src">1#out:1</P>`+b.String()+`</Line>`)
}

// TestReadSimulinkModelSignalBytes is S4-R1: a model's distinct signals hold at most
// SimulinkMaxPathBytes of endpoint paths together, else the model is ErrSimulinkCap and read as
// nothing; a model just inside the budget reads in full.
func TestReadSimulinkModelSignalBytes(t *testing.T) {
	root := "simulink/systems/system_root.xml"
	parent := simulinkLetters(16_000, 3)
	for _, mdl := range []bool{false, true} {
		read := func(kids int) (SimulinkModel, error) {
			if mdl {
				return ReadSimulinkModel("m.mdl", []byte(simulinkFanPairs(kids, parent, true)))
			}
			return ReadSimulinkModel("m.slx", slxPackage(t, root, simulinkFanPairs(kids, parent, false)))
		}
		// 32 children: 1,024 signals of about 32,000 bytes, inside the budget.
		m, err := read(32)
		if err != nil || len(m.Signals) != 32*32 {
			t.Errorf("mdl=%v: 32 children: %d signals, %v; want 1,024 and no error", mdl, len(m.Signals), err)
		}
		// 64 children: 4,096 signals, about 131 MB of endpoint paths.
		m, err = read(64)
		if !errors.Is(err, ErrSimulinkCap) || len(m.Blocks) != 0 || len(m.Signals) != 0 {
			t.Errorf("mdl=%v: 64 children: %d blocks, %d signals, %v; want ErrSimulinkCap and nothing", mdl, len(m.Blocks), len(m.Signals), err)
		}
	}
}

// TestReadSimulinkModelPathCap is S4-R1's path cap: a block path past SimulinkMaxPath, 16 KiB,
// is ErrSimulinkCap; one at it reads.
func TestReadSimulinkModelPathCap(t *testing.T) {
	root := "simulink/systems/system_root.xml"
	at := simulinkLetters(SimulinkMaxPath-len("m/"), 5)
	if m, err := ReadSimulinkModel("m.slx", slxPackage(t, root, `<System><Block Name="`+at+`" SID="1"/></System>`)); err != nil || len(m.Blocks) != 1 {
		t.Errorf("a %d-byte path: %d blocks, %v; want it read", SimulinkMaxPath, len(m.Blocks), err)
	}
	if _, err := ReadSimulinkModel("m.slx", slxPackage(t, root, `<System><Block Name="`+at+`x" SID="1"/></System>`)); !errors.Is(err, ErrSimulinkCap) {
		t.Errorf("a %d-byte path: %v; want ErrSimulinkCap", SimulinkMaxPath+1, err)
	}
}

// TestSimulinkExtractDocOutputBound is S4-R1's output bound: the Calls ExtractDoc emits hold
// at most SimulinkMaxPathBytes together, so blocks that share a path (each given its calls)
// cannot multiply a signal set inside its budget; past it the file yields nothing.
func TestSimulinkExtractDocOutputBound(t *testing.T) {
	parent := simulinkLetters(14_000, 9)
	build := func(dups int) string {
		var b strings.Builder
		b.WriteString(`<System><Block BlockType="SubSystem" Name="` + parent + `"><System>`)
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&b, `<Block Name="d%d" SID="%d"/>`, i, i+1)
		}
		b.WriteString(strings.Repeat(`<Block Name="s"/>`, dups))
		b.WriteString(`<Line><P Name="SrcBlock">s</P>`)
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&b, `<P Name="Dst">%d#in:1</P>`, i+1)
		}
		b.WriteString(`</Line></System></Block></System>`)
		return b.String()
	}
	x := NewSimulinkExtractor()
	one, err := x.ExtractDoc("svc", "m.slx", slxPackage(t, "simulink/systems/system_root.xml", build(1)))
	if err != nil || len(one.Nodes) != 1003 {
		t.Fatalf("one source: %d nodes, %v; want 1,003", len(one.Nodes), err)
	}
	many, err := x.ExtractDoc("svc", "m.slx", slxPackage(t, "simulink/systems/system_root.xml", build(1000)))
	calls := 0
	for _, n := range many.Nodes {
		for _, c := range n.Calls {
			calls += len(c)
		}
	}
	if err != nil || len(many.Nodes) != 0 {
		t.Errorf("1,000 sources sharing a path: %d nodes, %d bytes of calls, %v; want none", len(many.Nodes), calls, err)
	}
}

// TestSimulinkDecoderStrict is S4-R3: startTagCap counts attributes by their `=`, which bounds
// them only while the decoder is strict (a lax one reads a bare attribute name as one). A start
// tag of bare names past SimulinkMaxAttrs must be refused, so the decoder must stay strict.
func TestSimulinkDecoderStrict(t *testing.T) {
	if !newSimulinkDecoder(nil).Strict {
		t.Fatal("the Simulink XML decoder is not strict: startTagCap no longer bounds a tag's attributes")
	}
	bare := strings.Repeat(" a", SimulinkMaxAttrs+1)
	if _, err := ReadSimulinkModel("m.slx", slxPackage(t, "simulink/systems/system_root.xml", `<System><Block Name="a"`+bare+`/></System>`)); !errors.Is(err, ErrSimulinkCap) {
		t.Errorf("bare attribute names: %v; want ErrSimulinkCap", err)
	}
}
