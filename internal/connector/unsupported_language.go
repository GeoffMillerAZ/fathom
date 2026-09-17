package connector

import "path/filepath"

// unsupportedLangExtMap names real, well-known programming languages that
// fathom has NO registered extractor for (O69, wave P1 — Long Code Arena
// bench run found bumble-tech/appyx, a real Kotlin repository, indexed as
// near-empty and mislabeled with no signal that Kotlin itself was the gap).
// Kotlin itself was removed from this table in wave P2 (O81): it now has a
// registered tree-sitter/wazero extractor (extractor.NewTSExtractor
// ("kotlin")) and classifies via extClassMap instead — see
// TestUnsupportedLanguageForPath_RegisteredExtensionsNeverMatch, which pins
// that every entry here stays disjoint from what's actually extracted.
//
// This is deliberately a SEPARATE table from extClassMap (LT-1's single
// source of truth for what fathom DOES extract): every key here MUST be
// absent from extClassMap — TestUnsupportedLanguageForPath_
// RegisteredExtensionsNeverMatch pins the disjointness — and adding a
// language here never changes extraction behavior. Its only job is
// disclosure: so an ingest, list_services, or doctor caller sees
// "kotlin: 412 files, no extractor" instead of the files silently vanishing
// into the generic "no extractor registered" fingerprint-only path with no
// name attached to what was lost (ADR-021 §1's "never silently bucketed"
// promise, extended past the yaml/cue-family Unclassified counter to the
// general no-extractor path it didn't originally cover).
//
// Only single-purpose, unambiguous extensions are listed — no extension
// that could plausibly belong to more than one common language (".m",
// ".v", ".r", ".pl") is included, since a wrong guess here would be a new
// silent-wrongness class of its own.
var unsupportedLangExtMap = map[string]string{
	".swift":  "swift",
	".dart":   "dart",
	".ex":     "elixir",
	".exs":    "elixir",
	".clj":    "clojure",
	".cljs":   "clojure",
	".cljc":   "clojure",
	".hs":     "haskell",
	".ml":     "ocaml",
	".mli":    "ocaml",
	".fs":     "fsharp",
	".fsx":    "fsharp",
	".erl":    "erlang",
	".hrl":    "erlang",
	".zig":    "zig",
	".nim":    "nim",
	".jl":     "julia",
	".elm":    "elm",
	".cr":     "crystal",
	".groovy": "groovy",
}

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
