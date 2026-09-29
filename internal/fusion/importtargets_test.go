package fusion_test

// LB-F40 (design §1.2 import_resolution): ResolveImportSpecs is the file
// resolver's module resolution on its own — every import spec of every file,
// resolved by the language's manifest-selected strategy to the files of the
// universe, with the line span of the statement that binds it. The language
// harness scores it against an oracle's import references; nothing here
// writes an edge.

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// importTargetFixtures are the per-language resolver fixtures (each a small
// multi-file project the dialect's own file-resolve test already reads).
var importTargetFixtures = []struct {
	lang, dir string
	rels      []string
	want      []string // "file:start-end:local<-module=>targets", in extraction order
}{
	{
		lang: "rust", dir: "rustfixture",
		rels: []string{"src/main.rs", "src/util.rs", "src/net/mod.rs"},
		want: []string{
			"src/main.rs:1-1:net<-net=>src/net/mod.rs",
			"src/main.rs:2-2:util<-util=>src/util.rs",
			"src/main.rs:4-4:helper<-crate::util=>src/util.rs",
			"src/main.rs:4-4:helper<-crate::util::helper=>src/util.rs",
			"src/main.rs:5-5:connect<-net=>src/net/mod.rs",
			"src/main.rs:5-5:connect<-net::connect=>", // the whole-module reading of an item: no module file
		},
	},
	{
		lang: "ruby", dir: "rubyfixture",
		rels: []string{"lib/app.rb", "lib/app/helper.rb", "lib/app/models/user.rb", "lib/app/unused.rb"},
		want: []string{
			"lib/app.rb:1-1:json<-json=>",
			"lib/app.rb:2-2:app/helper<-app/helper=>lib/app/helper.rb",
			"lib/app.rb:3-3:app/models/user<-app/models/user=>lib/app/models/user.rb",
		},
	},
	{
		lang: "dart", dir: "dartfixture",
		rels: []string{"lib/main.dart", "lib/src/model.dart", "lib/src/net.dart", "lib/src/util.dart"},
		want: []string{
			"lib/main.dart:1-1:*<-package:acme/src/model.dart=>lib/src/model.dart", // a Dart import brings every name into scope
			"lib/main.dart:2-2:*<-package:flutter/material.dart=>",
			"lib/main.dart:3-3:*<-src/util.dart=>lib/src/util.dart",
			"lib/main.dart:4-4:net<-src/net.dart=>lib/src/net.dart",
		},
	},
	{
		lang: "kotlin", dir: "ktfixture",
		rels: []string{"com/example/util/Helper.kt", "com/example/util/Strings.kt", "com/example/app/Main.kt"},
		want: []string{
			"com/example/app/Main.kt:3-3:Helper<-com.example.util.Helper=>com/example/util/Helper.kt",
			"com/example/app/Main.kt:4-4:*<-com.example.util=>com/example/util/Helper.kt,com/example/util/Strings.kt",
		},
	},
	{
		lang: "cpp", dir: "cppfixture",
		rels: []string{"main.cpp", "util.hpp"},
		want: []string{
			"main.cpp:1-1:util.hpp<-util.hpp=>util.hpp",
			"main.cpp:2-2:<vector><-<vector>=>",
		},
	},
	{
		lang: "php", dir: "phpfixture",
		rels: []string{"src/Consumer.php", "src/Util/functions.php", "src/Util/Helper.php"},
		want: []string{
			`src/Consumer.php:5-5:*<-App\Util=>src/Util/Helper.php,src/Util/functions.php`,
			`src/Consumer.php:6-6:*<-App\Util\Helper=>src/Util/Helper.php`,
			`src/Consumer.php:7-7:Assist<-App\Util\Helper=>src/Util/Helper.php`,
		},
	},
}

func resolvedImportStrings(ris []fusion.ResolvedImport) []string {
	var out []string
	for _, ri := range ris {
		out = append(out, fmt.Sprintf("%s:%d-%d:%s<-%s=>%s", ri.File, ri.StartLine, ri.EndLine, ri.Spec.LocalName, ri.Spec.Module, strings.Join(ri.Targets, ",")))
	}
	return out
}

func TestResolveImportSpecs(t *testing.T) {
	for _, fx := range importTargetFixtures {
		t.Run(fx.lang, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", fx.dir))
			if err != nil {
				t.Fatal(err)
			}
			files, symbols := loadFileResolveFixture(t, dir, fx.lang, fx.rels)
			ris, report := fusion.ResolveImportSpecs(context.Background(), fx.lang, dir, files, symbols)
			if report.Skipped {
				t.Fatalf("skipped: %s", report.SkipReason)
			}
			if report.FilesScanned != len(fx.rels) || report.ImportsParsed != len(ris) {
				t.Errorf("report %+v, want %d files and %d specs", report, len(fx.rels), len(ris))
			}
			got := resolvedImportStrings(ris)
			if !slices.Equal(got, fx.want) {
				t.Errorf("resolved imports\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(fx.want, "\n     "))
			}
		})
	}
}

// TestResolveImportSpecsCoverFileEdges: the two passes share one resolver, so
// every file-resolved edge ResolveFileEdges draws from a file to a symbol of
// another file lands on a file one of the importer's specs resolves to — the
// import resolution the harness measures is the one the call edges stand on.
func TestResolveImportSpecsCoverFileEdges(t *testing.T) {
	for _, fx := range importTargetFixtures {
		t.Run(fx.lang, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", fx.dir))
			if err != nil {
				t.Fatal(err)
			}
			files, symbols := loadFileResolveFixture(t, dir, fx.lang, fx.rels)
			ris, _ := fusion.ResolveImportSpecs(context.Background(), fx.lang, dir, files, symbols)
			targets := map[string]map[string]bool{}
			for _, ri := range ris {
				if targets[ri.File] == nil {
					targets[ri.File] = map[string]bool{}
				}
				for _, t := range ri.Targets {
					targets[ri.File][t] = true
				}
			}
			fileOf := map[string]string{}
			for _, s := range symbols {
				fileOf[s.Key()] = s.FilePath
			}
			edges, _ := fusion.ResolveFileEdges(context.Background(), fx.lang, dir, files, symbols)
			cross := 0
			for _, e := range edges {
				to := fileOf[e.ToSymbol]
				if e.Kind != core.EdgeFileResolved || to == e.FromFile {
					continue
				}
				cross++
				if !targets[e.FromFile][to] {
					t.Errorf("edge %s -> %s lands on %s, which no import of %s resolves to (%v)", e.FromSymbol, e.ToSymbol, to, e.FromFile, targets[e.FromFile])
				}
			}
			if cross == 0 {
				t.Errorf("fixture draws no cross-file edge: the property is vacuous")
			}
		})
	}
}
