package extractor

// kotlindialect_test.go — LB-Q-kotlin. The kotlin dialect left
// TestTagsDialectParity's legacy projection (see migratedTagsDialects) because
// its package now comes from the package clause and it emits more than the
// pre-LB-Q-kotlin query did; this test keeps the guarantee that mattered: every
// symbol the pre-LB-Q-kotlin goldens listed (name, kind, first line) is still
// emitted. The table is those goldens' symbol lists as they were before
// LB-Q-kotlin (written by the pre-manifest engine, PROVENANCE.md); a symbol may
// be added to the fixtures' goldens, never removed from here.

import (
	"path/filepath"
	"testing"
)

type legacyKotlinSymbol struct {
	name, kind string
	line       int
}

var legacyKotlinSymbols = map[string][]legacyKotlinSymbol{
	"shapes.kt": {
		{"kotlin.math.PI", "KotlinImport", 3},
		{"com.example.util.Logger", "KotlinImport", 4},
		{"topLevel", "KotlinProperty", 6},
		{"mutableTop", "KotlinProperty", 7},
		{"Shape", "KotlinInterface", 9},
		{"Shape.area", "KotlinFunction", 10},
		{"Base", "KotlinClass", 13},
		{"Base.describe", "KotlinFunction", 16},
		{"Base.Nested", "KotlinClass", 23},
		{"Base.Nested.deep", "KotlinFunction", 24},
		{"Base.Nested.Inner", "KotlinClass", 26},
		{"Base.Nested.Inner.deeper", "KotlinFunction", 27},
		{"Base.Factory", "KotlinObject", 33},
		{"Base.Factory.create", "KotlinFunction", 34},
		{"Circle", "KotlinClass", 38},
		{"Circle.area", "KotlinFunction", 39},
		{"Color", "KotlinClass", 42},
		{"Result", "KotlinClass", 44},
		{"Result.Empty", "KotlinObject", 45},
		{"Registry", "KotlinObject", 48},
		{"Registry.register", "KotlinFunction", 49},
		{"shout", "KotlinFunction", 54},
		{"load", "KotlinFunction", 56},
		{"main", "KotlinFunction", 60},
	},
	"build.kts": {
		{"org.gradle.api.Project", "KotlinImport", 1},
		{"version", "KotlinProperty", 3},
		{"configure", "KotlinFunction", 5},
	},
	"broken.kt": {
		{"good", "KotlinFunction", 1},
		{"Survivor", "KotlinClass", 8},
		{"Survivor.ok", "KotlinFunction", 9},
	},
}

func TestKotlinDialectKeepsLegacySymbols(t *testing.T) {
	e, err := NewTSExtractor("kotlin")
	if err != nil {
		t.Fatalf("NewTSExtractor(kotlin): %v", err)
	}
	defer e.Close()
	for fixture, want := range legacyKotlinSymbols {
		t.Run(fixture, func(t *testing.T) {
			got := extractGolden(t, e, filepath.Join("dialects", "kotlin", "testdata", fixture))
			have := map[legacyKotlinSymbol]bool{}
			for _, s := range got.Symbols {
				have[legacyKotlinSymbol{s.Name, s.Kind, s.Line}] = true
			}
			for _, w := range want {
				if !have[w] {
					t.Errorf("%s: legacy symbol %s %s at line %d is no longer emitted", fixture, w.kind, w.name, w.line)
				}
			}
		})
	}
}
