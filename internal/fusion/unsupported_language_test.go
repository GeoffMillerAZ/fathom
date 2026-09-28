package fusion_test

// unsupported_language_test.go — O69 (wave P1): the Long Code Arena bench
// run found bumble-tech/appyx, a real Kotlin repository, indexed with only
// ~165 symbols and no signal anywhere that Kotlin — the repo's actual
// primary language — was the reason. Kotlin had no tree-sitter grammar or
// extractor wired into this repo at the time; TestIngest_UnsupportedLanguageDisclosed
// originally pinned Kotlin as the disclosure fixture. Wave P2 (O81) closed
// the Kotlin gap itself (extractor.NewTSExtractor("kotlin"),
// internal/extractor/tsengine/grammars/kotlin.wasm.gz), so this file now uses
// Swift — a language that still has no extractor — to keep proving the
// DISCLOSURE mechanism in general: Report.UnsupportedLanguages must name the
// language and count the files fusion silently skips, rather than leaving
// that fact recoverable only by noticing a suspiciously low symbol count.
// TestIngest_KotlinExtractsRealSymbols proves the other half: the same shape
// of repo that used to trip this disclosure path for Kotlin now actually
// extracts symbols instead.

import "testing"

func TestIngest_UnsupportedLanguageDisclosed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Sources/Example/Main.swift", "import Foundation\n\nfunc main() {}\n")
	writeFile(t, dir, "Sources/Example/Widget.swift", "class Widget {}\n")
	writeFile(t, dir, "README.md", "# Example\n")

	_, report := ingestDir(t, dir, "swift-fixture")

	if report.UnsupportedLanguages == nil {
		t.Fatalf("Report.UnsupportedLanguages is nil, want a swift entry")
	}
	if got, want := report.UnsupportedLanguages["swift"], 2; got != want {
		t.Errorf("UnsupportedLanguages[swift] = %d, want %d", got, want)
	}
	if _, ok := report.UnsupportedLanguages["md"]; ok {
		t.Errorf("UnsupportedLanguages should not list \"md\" — .md has a registered extractor")
	}
}

// A repo with nothing but registered-extractor languages reports no
// unsupported languages at all — the zero value must be honest (nil/empty
// map), not a stray zero-count entry manufactured for languages that never
// appeared.
func TestIngest_NoUnsupportedLanguages_WhenAllExtensionsSupported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "README.md", "# Example\n")

	_, report := ingestDir(t, dir, "go-only-fixture")

	if len(report.UnsupportedLanguages) != 0 {
		t.Errorf("UnsupportedLanguages = %v, want empty", report.UnsupportedLanguages)
	}
}

// TestIngest_KotlinExtractsRealSymbols is the wave-P2 closure proof for O69:
// the exact repo shape that used to fall through to UnsupportedLanguages
// (see the file header) now extracts real KotlinClass/KotlinFunction
// symbols and reports zero unsupported kotlin files.
func TestIngest_KotlinExtractsRealSymbols(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/src/main/kotlin/com/example/Main.kt", "package com.example\n\nfun main() {}\n")
	writeFile(t, dir, "app/src/main/kotlin/com/example/Widget.kt", "package com.example\n\nclass Widget {\n    fun render() {}\n}\n")
	writeFile(t, dir, "build.gradle.kts", "plugins {\n    kotlin(\"jvm\")\n}\n")
	writeFile(t, dir, "README.md", "# Example\n")

	obs, report := ingestDir(t, dir, "kotlin-fixture")

	if _, ok := report.UnsupportedLanguages["kotlin"]; ok {
		t.Errorf("UnsupportedLanguages still lists kotlin: %v (O69 gap should be closed)", report.UnsupportedLanguages)
	}
	if report.Symbols == 0 {
		t.Fatal("report.Symbols = 0, want kotlin symbols extracted")
	}

	var sawClass, sawFunc bool
	for _, o := range obs {
		if o.Symbol == "Widget" {
			sawClass = true
		}
		if o.Symbol == "Widget.render" || o.Symbol == "main" {
			sawFunc = true
		}
	}
	if !sawClass {
		t.Error("expected a Widget class symbol from Widget.kt")
	}
	if !sawFunc {
		t.Error("expected a function symbol (main or Widget.render) from the .kt files")
	}
}
