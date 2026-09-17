package fusion_test

// unsupported_language_test.go — O69 (wave P1): the Long Code Arena bench
// run found bumble-tech/appyx, a real Kotlin repository, indexed with only
// ~165 symbols and no signal anywhere that Kotlin — the repo's actual
// primary language — was the reason. Kotlin has no tree-sitter grammar or
// extractor wired into this repo (verified: internal/extractor has no
// kotlinextract.go and internal/extractor/tsengine/grammars ships only
// javascript/python/ruby/typescript .wasm). Faking a grammar is out of
// scope; this pins the DISCLOSURE half instead — Report.UnsupportedLanguages
// must name the language and count the files fusion silently skipped,
// rather than leaving that fact recoverable only by noticing a suspiciously
// low symbol count.

import "testing"

func TestIngest_UnsupportedLanguageDisclosed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app/src/main/kotlin/com/example/Main.kt", "package com.example\n\nfun main() {}\n")
	writeFile(t, dir, "app/src/main/kotlin/com/example/Widget.kt", "package com.example\n\nclass Widget\n")
	writeFile(t, dir, "build.gradle.kts", "plugins {\n    kotlin(\"jvm\")\n}\n")
	writeFile(t, dir, "README.md", "# Example\n")

	_, report := ingestDir(t, dir, "kotlin-fixture")

	if report.UnsupportedLanguages == nil {
		t.Fatalf("Report.UnsupportedLanguages is nil, want a kotlin entry")
	}
	if got, want := report.UnsupportedLanguages["kotlin"], 3; got != want {
		t.Errorf("UnsupportedLanguages[kotlin] = %d, want %d (2 .kt + 1 .kts)", got, want)
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
