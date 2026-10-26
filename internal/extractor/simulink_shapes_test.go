package extractor

import (
	"errors"
	"fmt"
	"path"
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

// simulinkDupSources is a model of dups blocks named A, then dsts (at most 1,296) blocks of
// two letters, and one line from A to each of them: every A carries the line's calls, 8,300 of
// them to 1,000 just under the byte cap in a model named m. A .mdl when mdl, else a
// .slx system part.
func simulinkDupSources(dups, dsts int, mdl bool) string {
	return simulinkDupSourcesPlus(dups, dsts, mdl, false)
}

// simulinkDupSourcesPlus is simulinkDupSources and, when extra, one more block B with one
// line to the first two-letter block: exactly one more emitted call.
func simulinkDupSourcesPlus(dups, dsts int, mdl, extra bool) string {
	var b strings.Builder
	const set = "abcdefghijklmnopqrstuvwxyz0123456789" // 1,296 two-letter names
	name := func(i int) string { return string(set[i/36%36]) + string(set[i%36]) }
	if mdl {
		b.WriteString("Model {\n System {\n")
		b.WriteString(strings.Repeat("Block {\nName \"A\"\n}\n", dups))
		for i := 0; i < dsts; i++ {
			fmt.Fprintf(&b, "Block {\nName \"%s\"\n}\n", name(i))
		}
		if extra {
			b.WriteString("Block {\nName \"B\"\n}\n")
		}
		b.WriteString("Line {\nSrcBlock \"A\"\n")
		for i := 0; i < dsts; i++ {
			fmt.Fprintf(&b, "Branch {\nDstBlock \"%s\"\n}\n", name(i))
		}
		b.WriteString("}\n")
		if extra {
			fmt.Fprintf(&b, "Line {\nSrcBlock \"B\"\nDstBlock \"%s\"\n}\n", name(0))
		}
		b.WriteString(" }\n}\n")
		return b.String()
	}
	b.WriteString(`<System>`)
	for i := 0; i < dups; i++ { // each its own SID, so the part compresses inside the ratio cap
		fmt.Fprintf(&b, `<Block Name="A" SID="%d"/>`, i+1)
	}
	for i := 0; i < dsts; i++ {
		fmt.Fprintf(&b, `<Block Name="%s"/>`, name(i))
	}
	if extra {
		b.WriteString(`<Block Name="B"/>`)
	}
	b.WriteString(`<Line><P Name="SrcBlock">A</P>`)
	for i := 0; i < dsts; i++ {
		fmt.Fprintf(&b, `<P Name="DstBlock">%s</P>`, name(i))
	}
	b.WriteString(`</Line>`)
	if extra {
		fmt.Fprintf(&b, `<Line><P Name="SrcBlock">B</P><P Name="DstBlock">%s</P></Line>`, name(0))
	}
	b.WriteString(`</System>`)
	return b.String()
}

// TestReadSimulinkModelCountCaps is S5-R1: short names pass the byte caps, so a model holds
// at most SimulinkMaxSignals distinct signals and its blocks at most SimulinkMaxCalls emitted
// call entries, else it is ErrSimulinkCap, read as nothing and extracted to nothing; a model
// at either cap reads in full.
func TestReadSimulinkModelCountCaps(t *testing.T) {
	root := "simulink/systems/system_root.xml"
	for _, mdl := range []bool{false, true} {
		read := func(src string) (SimulinkModel, DocResult, error) {
			rel, b := "m.slx", []byte(src)
			if mdl {
				rel = "m.mdl"
			} else {
				b = slxPackage(t, root, src)
			}
			m, err := ReadSimulinkModel(rel, b)
			r, xerr := NewSimulinkExtractor().ExtractDoc("svc", rel, b)
			// a refusal is the same on both paths: Extract's error wraps the reader's
			if (err != nil) != (xerr != nil) || (err != nil && !errors.Is(xerr, ErrModelRefused)) {
				t.Fatalf("reader err %v, ExtractDoc err %v; want a refusal on both or neither", err, xerr)
			}
			return m, r, err
		}
		// 256 short-named children, every pair joined: 65,536 signals, at the cap.
		if m, _, err := read(simulinkFanPairs(256, "p", mdl)); err != nil || len(m.Signals) != SimulinkMaxSignals {
			t.Errorf("mdl=%v: 256 children: %d signals, %v; want %d", mdl, len(m.Signals), err, SimulinkMaxSignals)
		}
		// 65 sources named A, 1,000 destinations: 65,000 emitted calls, inside the cap.
		if m, r, err := read(simulinkDupSources(65, 1000, mdl)); err != nil || len(m.Signals) != 1000 || len(r.Nodes) != 1066 {
			t.Errorf("mdl=%v: 65 sources: %d signals, %d nodes, %v; want 1,000 and 1,066", mdl, len(m.Signals), len(r.Nodes), err)
		}
		// 64 sources named A, 1,024 destinations: exactly 65,536 emitted calls, read in full.
		if m, r, err := read(simulinkDupSources(64, 1024, mdl)); err != nil || len(m.Signals) != 1024 || len(r.Nodes) != 64+1024+1 {
			t.Errorf("mdl=%v: 64 sources x 1,024: %d signals, %d nodes, %v; want 1,024 and 1,089", mdl, len(m.Signals), len(r.Nodes), err)
		}
		for _, c := range []struct{ name, src, why string }{
			{"257 short children", simulinkFanPairs(257, "p", mdl), "past 65536 signals"},
			{"700 short children", simulinkFanPairs(700, "p", mdl), "past 65536 signals"},
			{"66 sources named A", simulinkDupSources(66, 1000, mdl), "past 65536 emitted calls"},
			{"8300 sources named A", simulinkDupSources(8300, 1000, mdl), "past 65536 emitted calls"},
			// one past the cap: the 64 x 1,024 model above and one more line of one call.
			{"65,537 calls", simulinkDupSourcesPlus(64, 1024, mdl, true), "past 65536 emitted calls"},
		} {
			if c.name == "700 short children" && mdl {
				continue // 12 MB of .mdl; the heap test's shape is 260 children
			}
			m, r, err := read(c.src)
			if !errors.Is(err, ErrSimulinkCap) || !strings.Contains(err.Error(), c.why) || len(m.Blocks) != 0 || len(m.Signals) != 0 || len(r.Nodes) != 0 {
				t.Errorf("mdl=%v: %s: %d blocks, %d signals, %d nodes, %v; want ErrSimulinkCap (%s) and nothing",
					mdl, c.name, len(m.Blocks), len(m.Signals), len(r.Nodes), err, c.why)
			}
		}
	}
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
// cannot multiply a signal set inside its budget; past it the file yields nothing. Three
// sources of 1,000 long destinations are 3,000 calls, inside SimulinkMaxCalls, so the byte
// cap is the one that ends it.
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
	many, err := x.ExtractDoc("svc", "m.slx", slxPackage(t, "simulink/systems/system_root.xml", build(3)))
	calls := 0
	for _, n := range many.Nodes {
		for _, c := range n.Calls {
			calls += len(c)
		}
	}
	if !errors.Is(err, ErrModelRefused) || len(many.Nodes) != 0 {
		t.Errorf("3 sources sharing a path: %d nodes, %d bytes of calls, %v; want none and ErrModelRefused", len(many.Nodes), calls, err)
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

// simulinkRefsJSON is a graphicalInterface.json of n model references, each blockPath to
// model m<i>.
func simulinkRefsJSON(n int, blockPath string) string {
	var b strings.Builder
	b.WriteString(`{"GraphicalInterface": {"ModelReferences": [`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"BlockPath": %q, "ModelName": "m%d"}`, blockPath, i)
	}
	b.WriteString(`]}}`)
	return b.String()
}

// simulinkRefsXML is n packed model references, in a graphicalInterface.xml part (or the
// diagram's inline GraphicalInterface) or a .mdl's.
func simulinkRefsXML(n int, mdl bool) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if mdl {
			fmt.Fprintf(&b, "ModelReference {\nModelRefBlockPath \"$bdroot/R%d|m%d\"\n}\n", i, i)
		} else {
			fmt.Fprintf(&b, `<ModelReference><P Name="ModelRefBlockPath">$bdroot/R%d|m%d</P></ModelReference>`, i, i)
		}
	}
	return b.String()
}

// TestReadSimulinkModelRefCaps is S5-R2: model references are read under caps from every
// source, SimulinkMaxModelRefs per model and SimulinkMaxValue per BlockPath or ModelName, and
// graphicalInterface.json is decoded as a bounded token stream (its values nest at most
// SimulinkMaxDepth); past a cap the model is ErrSimulinkCap and read as nothing, never a
// truncated list.
func TestReadSimulinkModelRefCaps(t *testing.T) {
	sys := "simulink/systems/system_root.xml"
	block := `<System><Block Name="K" SID="1"/></System>`
	jsonPkg := func(gi string) []byte {
		return slxPackage(t, sys, block, "simulink/graphicalInterface.json", gi)
	}
	xmlPkg := func(n int) []byte {
		return slxPackage(t, sys, block, "simulink/graphicalInterface.xml", "<GraphicalInterface>"+simulinkRefsXML(n, false)+"</GraphicalInterface>")
	}
	inlinePkg := func(n int) []byte {
		return slxPackage(t, "simulink/blockdiagram.xml", `<ModelInformation><Model Name="m"><GraphicalInterface>`+
			simulinkRefsXML(n, false)+`</GraphicalInterface><System><Block Name="K" SID="1"/></System></Model></ModelInformation>`)
	}
	mdl := func(n int) []byte {
		return []byte("Model {\nGraphicalInterface {\n" + simulinkRefsXML(n, true) + "}\nSystem {\nBlock {\nName \"K\"\n}\n}\n}\n")
	}
	at := simulinkLetters(SimulinkMaxValue, 11) // compresses as a real value does, inside the ratio cap
	ok := map[string][]byte{
		"json at the ref cap":    jsonPkg(simulinkRefsJSON(SimulinkMaxModelRefs, "$bdroot/K")),
		"json value at the cap":  jsonPkg(simulinkRefsJSON(1, at)),
		"xml at the ref cap":     xmlPkg(SimulinkMaxModelRefs),
		"inline at the ref cap":  inlinePkg(SimulinkMaxModelRefs),
		"mdl at the ref cap":     mdl(SimulinkMaxModelRefs),
		"json nested at the cap": jsonPkg(`{"Other": ` + strings.Repeat("[", SimulinkMaxDepth-1) + strings.Repeat("]", SimulinkMaxDepth-1) + `, "ModelReferences": [{"BlockPath": "$bdroot/K", "ModelName": "a"}]}`),
	}
	for name, src := range ok {
		rel := "m.slx"
		if strings.HasPrefix(name, "mdl") {
			rel = "m.mdl"
		}
		m, err := ReadSimulinkModel(rel, src)
		if err != nil || len(m.Blocks) != 1 || len(m.ModelRefs) == 0 {
			t.Errorf("%s: %d blocks, %d model refs, %v; want it read", name, len(m.Blocks), len(m.ModelRefs), err)
		}
	}
	past := map[string][]byte{
		"json past the ref cap":   jsonPkg(simulinkRefsJSON(SimulinkMaxModelRefs+1, "$bdroot/K")),
		"json BlockPath too long": jsonPkg(simulinkRefsJSON(1, at+"p")),
		"json ModelName too long": jsonPkg(`{"ModelReferences": {"BlockPath": "$bdroot/K", "ModelName": "` + at + `p"}}`),
		"json skipped too long":   jsonPkg(`{"Other": "` + at + `p", "ModelReferences": []}`),
		"json nested too deep":    jsonPkg(`{"Other": ` + strings.Repeat("[", SimulinkMaxDepth+1) + strings.Repeat("]", SimulinkMaxDepth+1) + `}`),
		"xml past the ref cap":    xmlPkg(SimulinkMaxModelRefs + 1),
		"inline past the ref cap": inlinePkg(SimulinkMaxModelRefs + 1),
		"mdl past the ref cap":    mdl(SimulinkMaxModelRefs + 1),
	}
	why := map[string]string{ // the cap that must end each
		"json past the ref cap":   "past 4096 model references",
		"json BlockPath too long": "a string is longer than",
		"json ModelName too long": "a string is longer than",
		"json skipped too long":   "a string is longer than",
		"json nested too deep":    "JSON values nest past",
		"xml past the ref cap":    "past 4096 model references",
		"inline past the ref cap": "past 4096 model references",
		"mdl past the ref cap":    "past 4096 model references",
	}
	for name, src := range past {
		rel := "m.slx"
		if strings.HasPrefix(name, "mdl") {
			rel = "m.mdl"
		}
		m, err := ReadSimulinkModel(rel, src)
		r, _ := NewSimulinkExtractor().ExtractDoc("svc", rel, src)
		if !errors.Is(err, ErrSimulinkCap) || !strings.Contains(err.Error(), why[name]) ||
			len(m.Blocks) != 0 || len(m.ModelRefs) != 0 || len(r.Nodes) != 0 || len(r.CrossFile) != 0 {
			t.Errorf("%s: %d blocks, %d model refs, %d nodes, %d cross-file, %v; want ErrSimulinkCap and nothing",
				name, len(m.Blocks), len(m.ModelRefs), len(r.Nodes), len(r.CrossFile), err)
		}
	}
}

// TestReadSimulinkModelRefsJSONDecode: the token stream reads graphicalInterface.json as the
// decode into a map and a struct did: GraphicalInterface wins over a top-level list, field
// names match without case, a member given twice is its last value, and an element that is
// not a reference object is skipped.
func TestReadSimulinkModelRefsJSONDecode(t *testing.T) {
	sys := "simulink/systems/system_root.xml"
	cases := map[string]struct {
		gi   string
		want []SimulinkModelRef
	}{
		"inner wins": {`{"ModelReferences": [{"BlockPath": "a/X", "ModelName": "top"}], "GraphicalInterface": {"ModelReferences": {"BlockPath": "$bdroot/Y", "ModelName": "in"}}}`,
			[]SimulinkModelRef{{"m/Y", "in"}}},
		"any case": {`{"ModelReferences": [{"blockpath": "a/X", "MODELNAME": "c"}]}`, []SimulinkModelRef{{"a/X", "c"}}},
		"last member": {`{"ModelReferences": [{"BlockPath": "a/X", "ModelName": "one"}], "ModelReferences": [{"BlockPath": "a/Y", "ModelName": "two"}]}`,
			[]SimulinkModelRef{{"a/Y", "two"}}},
		"mixed list": {`{"ModelReferences": ["s", 1, [{"BlockPath": "a/Z", "ModelName": "no"}], {"BlockPath": "a/X", "ModelName": "ok", "Extra": {"k": [1, 2]}}, {"BlockPath": "a/W"}]}`,
			[]SimulinkModelRef{{"a/X", "ok"}}},
		"not an object": {`[1, 2]`, nil},
	}
	for name, c := range cases {
		m, err := ReadSimulinkModel("m.slx", slxPackage(t, sys, `<System/>`, "simulink/graphicalInterface.json", c.gi))
		if err != nil || fmt.Sprint(m.ModelRefs) != fmt.Sprint(c.want) {
			t.Errorf("%s: %v, %v; want %v", name, m.ModelRefs, err, c.want)
		}
	}
}

// TestReadSimulinkModelRefsPartRefused is S5-R3: a model-references part that is over a cap
// or does not parse refuses the whole model, as a system part does, never a silent read of
// the rest: an in-budget model beside it reads as nothing.
func TestReadSimulinkModelRefsPartRefused(t *testing.T) {
	sys := "simulink/systems/system_root.xml"
	block := `<System><Block Name="K" SID="1"/></System>`
	if m, err := ReadSimulinkModel("m.slx", slxPackage(t, sys, block)); err != nil || len(m.Blocks) != 1 {
		t.Fatalf("the model alone: %d blocks, %v; want it read", len(m.Blocks), err)
	}
	for name, part := range map[string][2]string{
		"xml over the element budget":  {"simulink/graphicalInterface.xml", slxNoise(SimulinkMaxElements, `<P Name="x">%s</P>`, false)},
		"xml not well formed":          {"simulink/graphicalInterface.xml", `<GraphicalInterface><ModelReference>`},
		"json not well formed":         {"simulink/graphicalInterface.json", `{"ModelReferences": [{"BlockPath": "a/b", "ModelName": `},
		"json data after the object":   {"simulink/graphicalInterface.json", `{"ModelReferences":[{"BlockPath":"$bdroot/K","ModelName":"a"}]} garbage`},
		"json second value":            {"simulink/graphicalInterface.json", `{}{`},
		"json data after a non-object": {"simulink/graphicalInterface.json", `[1] garbage`},
	} {
		m, err := ReadSimulinkModel("m.slx", slxPackage(t, sys, block, part[0], part[1]))
		if !errors.Is(err, ErrSimulinkCap) || !strings.Contains(err.Error(), path.Base(part[0])) || len(m.Blocks) != 0 {
			t.Errorf("%s: %d blocks, %v; want ErrSimulinkCap naming %s and nothing read", name, len(m.Blocks), err, path.Base(part[0]))
		}
	}
}

// TestReadSimulinkModelJSONRefsSpendElementBudget is S5-R2: each reference in
// graphicalInterface.json spends an element of the budget the package's XML parts share, so
// a system part that leaves room for two references reads with two and refuses with three.
func TestReadSimulinkModelJSONRefsSpendElementBudget(t *testing.T) {
	sys := "simulink/systems/system_root.xml"
	// n elements and the <System> root: SimulinkMaxElements-2 spent, two left.
	noise := slxNoise(SimulinkMaxElements-3, `<P Name="x">%s</P>`, false)
	for refs, wantErr := range map[int]bool{2: false, 3: true} {
		m, err := ReadSimulinkModel("m.slx", slxPackage(t, sys, noise, "simulink/graphicalInterface.json", simulinkRefsJSON(refs, "$bdroot/K")))
		if wantErr {
			if !errors.Is(err, ErrSimulinkCap) || !strings.Contains(err.Error(), "past 524288 elements") || len(m.ModelRefs) != 0 {
				t.Errorf("%d references: %d model refs, %v; want ErrSimulinkCap past 524288 elements", refs, len(m.ModelRefs), err)
			}
		} else if err != nil || len(m.ModelRefs) != refs {
			t.Errorf("%d references: %d model refs, %v; want them read", refs, len(m.ModelRefs), err)
		}
	}
}
