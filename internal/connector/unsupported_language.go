package connector

import "path/filepath"

// unsupportedLangExtMap (unsupported_language_gen.go) names real,
// well-known programming languages fathom has NO registered extractor for
// (O69 — the Long Code Arena run found a real Kotlin repository indexed
// near-empty with no signal that Kotlin was the gap). Since LB-F25 it is
// generated from the language-breadth queue (`indexbench lang-disclosure`,
// TestUnsupportedTableMatchesQueue): every queued, unregistered language's
// unambiguous extensions, until its dialect registers.
//
// It is deliberately separate from extClassMap (LT-1's single source of
// truth for what fathom DOES extract): the two stay disjoint
// (TestUnsupportedLanguageForPath_RegisteredExtensionsNeverMatch), and the
// table only discloses — "swift: 412 files, no extractor" instead of files
// vanishing into the no-extractor fingerprint path (ADR-021 §1). An
// extension two languages could own (".m", ".v", ".pl", ".fs") is never
// listed: a wrong guess would be a silent-wrongness class of its own.

// UnsupportedLanguageForPath reports the display name of a well-known
// programming language for path when path's extension is recognized as
// belonging to that language but fathom has no registered extractor for it
// (see unsupportedLangExtMap). ok is false for any path that IS covered by
// the registered extractor set (classify succeeds) — a supported language
// is never also "unsupported" — and for any extension this table does not
// name, so a config file, lockfile, or generic text file never gets
// mislabeled as a "gap".
func UnsupportedLanguageForPath(path string) (lang string, ok bool) {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	if _, registered := classify(base, ext); registered {
		return "", false
	}
	lang, ok = unsupportedLangExtMap[ext]
	return lang, ok
}
