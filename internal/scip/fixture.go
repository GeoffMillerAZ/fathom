package scip

import (
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// BuildTSFixtureIndex hand-builds, byte-for-byte, the SCIP index committed
// at testdata/tsfixture/index.scip (SU-9, docs/plans/substrate.md): three
// TypeScript files (testdata/tsfixture/{math,greeter,main}.ts) with an
// import chain and calls across them. It exists so the fixture is a
// regenerable Go value, never scip-typescript's own output — nothing in
// this repo or its tests ever requires scip-typescript to be installed.
//
// Fixture shape (see testdata/tsfixture/*.ts for the exact source this
// mirrors):
//
//	math.ts:    function add(a, b)
//	greeter.ts: import { add } from "./math"
//	            class Greeter { greet(name) { console.log(name); return add(1,2) + name.length } }
//	main.ts:    import { Greeter } from "./greeter"
//	            function run() { const g = new Greeter(); return g.greet("hi") }
//
// Every occurrence below is REAL with respect to that source (same line,
// same target), so a reader diffing the two never finds a mismatch — only
// the exact character columns are approximated (0-based, roughly where the
// token starts), since ImportIndex's containment logic only depends on
// line-level nesting for source this shallow.
//
// Referenced symbols, in scip-typescript's own moniker shape (opaque to
// ImportIndex — it never parses this grammar, see the package doc
// comment):
//
//	math/add():                   defined in math.ts
//	greeter/Greeter#:              defined in greeter.ts
//	greeter/Greeter#greet():       defined in greeter.ts, nested in Greeter#
//	main/run():                    defined in main.ts
//	npm typescript console/log():  NOT defined anywhere in this index —
//	                                the fixture's one deliberately external
//	                                reference, proving the EdgeUnresolved
//	                                path.
func BuildTSFixtureIndex() *scippb.Index {
	const (
		symAdd     = "scip-typescript npm tsfixture 0.0.1 math/add()."
		symGreeter = "scip-typescript npm tsfixture 0.0.1 greeter/Greeter#"
		symGreet   = "scip-typescript npm tsfixture 0.0.1 greeter/Greeter#greet()."
		symRun     = "scip-typescript npm tsfixture 0.0.1 main/run()."
		symConsole = "scip-typescript npm typescript 5.0.0 lib/console.log()."
	)

	defEnc := func(sym string, r, enc []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, EnclosingRange: enc, Symbol: sym, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, r []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, Symbol: sym}
	}
	info := func(sym, display string) *scippb.SymbolInformation {
		return &scippb.SymbolInformation{Symbol: sym, DisplayName: display}
	}

	mathDoc := &scippb.Document{
		RelativePath: "math.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			defEnc(symAdd, []int32{0, 16, 0, 19}, []int32{0, 0, 2, 1}),
		},
		Symbols: []*scippb.SymbolInformation{info(symAdd, "add")},
	}

	greeterDoc := &scippb.Document{
		RelativePath: "greeter.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			// import { add } from "./math" — a top-level reference, OUTSIDE
			// every definition's span: ImportIndex must skip it, not
			// misattribute it to whichever definition happens to be first.
			ref(symAdd, []int32{0, 9, 0, 12}),
			defEnc(symGreeter, []int32{2, 13, 2, 20}, []int32{2, 0, 7, 1}),
			defEnc(symGreet, []int32{3, 4, 3, 9}, []int32{3, 4, 6, 5}),
			// console.log(name) — a reference to a symbol this index never
			// defines: the fixture's EdgeUnresolved case.
			ref(symConsole, []int32{4, 8, 4, 19}),
			// add(1, 2) — resolves to math.ts's definition.
			ref(symAdd, []int32{5, 15, 5, 18}),
		},
		Symbols: []*scippb.SymbolInformation{
			info(symGreeter, "Greeter"),
			info(symGreet, "greet"),
		},
	}

	mainDoc := &scippb.Document{
		RelativePath: "main.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			// import { Greeter } from "./greeter" — top-level, skipped.
			ref(symGreeter, []int32{0, 9, 0, 16}),
			defEnc(symRun, []int32{2, 16, 2, 19}, []int32{2, 0, 5, 1}),
			// new Greeter() — a constructor reference tsextract's own
			// syntax pass never captures as a call (see
			// internal/indexbench's scip-oracle fixture test): SCIP sees
			// it, fathom's syntax pass does not — a real, honest recall
			// gap this fixture is built to show.
			ref(symGreeter, []int32{3, 18, 3, 25}),
			// g.greet("hi") — resolves to Greeter#greet.
			ref(symGreet, []int32{4, 13, 4, 18}),
		},
		Symbols: []*scippb.SymbolInformation{info(symRun, "run")},
	}

	return &scippb.Index{
		Metadata: &scippb.Metadata{
			Version:              scippb.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo:             &scippb.ToolInfo{Name: "scip-typescript", Version: "0.3.16"},
			ProjectRoot:          "file:///tsfixture",
			TextDocumentEncoding: scippb.TextEncoding_UTF8,
		},
		Documents: []*scippb.Document{mathDoc, greeterDoc, mainDoc},
	}
}
