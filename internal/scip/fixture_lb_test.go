package scip

import (
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// The LB-F54 and LB-F55 fixtures live beside their only callers (scip_import_test.go),
// out of the production binary: no command reaches them.

// BuildNoEnclosingFixtureIndex hand-builds the LB-F54 fixture: the shape
// scip-php, scip-clang and scip-dotnet emit, where NO definition occurrence
// carries an EnclosingRange (calibration/oracles/scip-php/README.md: none of
// 4,334 definitions). Before LB-F54 every reference here fell outside every
// definition's name token and was skipped as "no-enclosing-definition"; with
// the fallback, a mapped definition takes its fathom node's own line span.
//
// Source shape (0-based lines; columns approximate, as in
// BuildTSFixtureIndex):
//
//	 0  <?php
//	 1  use Lib\Logger;                      top-level reference: skipped
//	 2  class Greeter {
//	 3    public function greet($name) {     $name is a Parameter definition
//	 4      $this->log($name);               greet -> log (Direct), $name: unmapped-endpoint
//	 5      return strlen($name);            greet -> strlen (external), $name again
//	 6    }
//	 7    private function log($m) {
//	 8      Logger::info($m);                log -> Logger (external)
//	 9    }
//	10  }
//	11  function helper() {                  mapped to a node with no EndLine: keeps Range
//	12    return strlen("x");                so this reference stays skipped
//	13  }
//	14  helper();                            top-level reference: skipped
func BuildNoEnclosingFixtureIndex() *scippb.Index {
	const (
		symGreeter = "scip-php composer acme/greeter 1.0.0 Acme/Greeter#"
		symGreet   = "scip-php composer acme/greeter 1.0.0 Acme/Greeter#greet()."
		symName    = "scip-php composer acme/greeter 1.0.0 Acme/Greeter#greet().($name)"
		symLog     = "scip-php composer acme/greeter 1.0.0 Acme/Greeter#log()."
		symM       = "scip-php composer acme/greeter 1.0.0 Acme/Greeter#log().($m)"
		symHelper  = "scip-php composer acme/greeter 1.0.0 Acme/helper()."
		symLogger  = "scip-php composer lib/logger 2.0.0 Lib/Logger#"
		symStrlen  = "scip-php composer php 8.3.0 strlen()."
	)
	def := func(sym string, r []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, Symbol: sym, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, r []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, Symbol: sym}
	}
	doc := &scippb.Document{
		RelativePath: "src/Greeter.php",
		Language:     "php",
		Occurrences: []*scippb.Occurrence{
			ref(symLogger, []int32{1, 8, 1, 14}),
			def(symGreeter, []int32{2, 6, 2, 13}),
			def(symGreet, []int32{3, 18, 3, 23}),
			def(symName, []int32{3, 24, 3, 29}),
			ref(symLog, []int32{4, 11, 4, 14}),
			ref(symName, []int32{4, 15, 4, 20}),
			ref(symStrlen, []int32{5, 11, 5, 17}),
			ref(symName, []int32{5, 18, 5, 23}),
			def(symLog, []int32{7, 19, 7, 22}),
			def(symM, []int32{7, 23, 7, 25}),
			ref(symLogger, []int32{8, 4, 8, 10}),
			def(symHelper, []int32{11, 9, 11, 15}),
			ref(symStrlen, []int32{12, 9, 12, 15}),
			ref(symHelper, []int32{14, 0, 14, 6}),
		},
	}
	return &scippb.Index{
		Metadata: &scippb.Metadata{
			ToolInfo:             &scippb.ToolInfo{Name: "scip-php", Version: "0.0.0-fixture"},
			ProjectRoot:          "file:///greeter",
			TextDocumentEncoding: scippb.TextEncoding_UTF8,
		},
		Documents: []*scippb.Document{doc},
	}
}

// BuildLocalsFixtureIndex hand-builds the LB-F55 fixture: a producer that
// sets EnclosingRange (scip-dart's shape) on "local N" definitions too, so a
// local variable's span covers its own initializer.
//
// Source shape (0-based lines):
//
//	 0  import 'util.dart';                  top-level reference: skipped
//	 1  void run() {
//	 2    final total = compute(1);          local 0; run -> compute (Direct, via the local's parent)
//	 3    print(total);                      run -> print (external); total: local-target
//	 4  }
//	 5
//	 6  int compute(int n) {                 n is a Parameter definition
//	 7    return n * 2;                      n: unmapped-endpoint (parameters unchanged)
//	 8  }
//	 9
//	10  class Box {
//	11    int mystery() {                    no fathom node: unmapped
//	12      final y = compute(2);            local 1 inside an UNMAPPED method: unmapped-endpoint
//	13      return y;                        y: local-target
//	14    }
//	15  }
func BuildLocalsFixtureIndex() *scippb.Index {
	const (
		symUtil    = "scip-dart pub app 1.0.0 lib/`util.dart`/"
		symRun     = "scip-dart pub app 1.0.0 lib/`app.dart`/run()."
		symCompute = "scip-dart pub app 1.0.0 lib/`app.dart`/compute()."
		symN       = "scip-dart pub app 1.0.0 lib/`app.dart`/compute().(n)"
		symBox     = "scip-dart pub app 1.0.0 lib/`app.dart`/Box#"
		symMystery = "scip-dart pub app 1.0.0 lib/`app.dart`/Box#mystery()."
		symPrint   = "scip-dart pub dart:core 3.0.0 dart:core/print()."
		symTotal   = "local 0"
		symY       = "local 1"
	)
	def := func(sym string, r, enc []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, EnclosingRange: enc, Symbol: sym, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, r []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, Symbol: sym}
	}
	doc := &scippb.Document{
		RelativePath: "lib/app.dart",
		Language:     "dart",
		Occurrences: []*scippb.Occurrence{
			ref(symUtil, []int32{0, 8, 0, 17}),
			def(symRun, []int32{1, 5, 1, 8}, []int32{1, 0, 4, 1}),
			def(symTotal, []int32{2, 8, 2, 13}, []int32{2, 2, 2, 26}),
			ref(symCompute, []int32{2, 16, 2, 23}),
			ref(symPrint, []int32{3, 2, 3, 7}),
			ref(symTotal, []int32{3, 8, 3, 13}),
			def(symCompute, []int32{6, 4, 6, 11}, []int32{6, 0, 8, 1}),
			def(symN, []int32{6, 16, 6, 17}, nil),
			ref(symN, []int32{7, 9, 7, 10}),
			def(symBox, []int32{10, 6, 10, 9}, []int32{10, 0, 15, 1}),
			def(symMystery, []int32{11, 6, 11, 13}, []int32{11, 2, 14, 3}),
			def(symY, []int32{12, 10, 12, 11}, []int32{12, 4, 12, 24}),
			ref(symCompute, []int32{12, 14, 12, 21}),
			ref(symY, []int32{13, 11, 13, 12}),
		},
	}
	return &scippb.Index{
		Metadata: &scippb.Metadata{
			ToolInfo:             &scippb.ToolInfo{Name: "scip-dart", Version: "0.0.0-fixture"},
			ProjectRoot:          "file:///app",
			TextDocumentEncoding: scippb.TextEncoding_UTF8,
		},
		Documents: []*scippb.Document{doc},
	}
}
