package connector_test

// language_test.go — coverage for connector.LanguageForPath and
// RegisteredExtensions, the one ext→language source of truth (ADR-021 §1 /
// LT-1). The bijection between this registry and fusion's extractor
// registrations is asserted in internal/fusion (white-box, since fusion.New's
// extractor map is unexported) — see internal/fusion's language-registry
// parity test.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestLanguageForPath_KnownExtensions(t *testing.T) {
	cases := map[string]string{
		"main.go":   "go",
		"go.mod":    "go",
		"script.py": "py",
		"app.js":    "js",
		// J1 (app-kit O18/O32/O33): the four Node/TS module-system
		// extensions a plain .js/.ts glob never covers.
		"memory.mjs":                 "js",
		"config.cjs":                 "js",
		"probe.mts":                  "ts",
		"types.cts":                  "ts",
		"component.tsx":              "ts",
		"Main.java":                  "java",
		"App.kt":                     "kotlin",
		"build.gradle.kts":           "kotlin",
		"lib.rb":                     "ruby",
		"main.rs":                    "rust",
		"module.tf":                  "terraform",
		"schema.cue":                 "cue",
		"README.md":                  "md",
		"deploy.yaml":                "yaml",
		"deploy.yml":                 "yaml",
		"component.templ":            "templ",
		"charts/foo/Chart.yaml":      "yaml",
		"charts/foo/values.yaml":     "yaml",
		"charts/foo/templates/x.tpl": "yaml",
		"Dockerfile":                 "dockerfile",
		"api.dockerfile":             "dockerfile",
		"docker-compose.yml":         "yaml",
		"compose.yaml":               "yaml",
		"service.proto":              "proto",
		"schema.graphql":             "graphql",
		"schema.gql":                 "graphql",
		"Cargo.toml":                 "cargo",
		"rust-toolchain.toml":        "toml",
		"schema.sql":                 "sql",
		"data.json":                  "json",
		"Main.scala":                 "scala",
		"script.sc":                  "scala",
		"index.php":                  "php",
		"run.sh":                     "sh",
		"run.bash":                   "sh",
		"script.lua":                 "lua",
		"pkg/foo/bar.c":              "c",
		"pkg/foo/bar.h":              "c",
		"pkg/foo/bar.cpp":            "cpp",
		"pkg/foo/bar.hpp":            "cpp",
		"pkg/foo/bar.cs":             "cs",
	}
	for path, want := range cases {
		if got := connector.LanguageForPath(path); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestLanguageForPath_Unclassified(t *testing.T) {
	// No extractor is registered for these — including three of the OLD
	// langFromPath's hand-maintained lockfile special cases (go.sum,
	// Gemfile.lock, Cargo.lock, requirements.txt): with one registry driving
	// both Kind and Lang, a basename with no extractor is honestly
	// "unclassified", never silently mislabeled. (package-lock.json is NOT
	// in this list: its .json extension IS a registered extractor extension,
	// so it now correctly resolves to "json" rather than the old special
	// case's "js" — a more honest answer, not a regression.)
	for _, path := range []string{
		"README.rst",
		"data.xyz",
		"go.sum",
		"Gemfile.lock",
		"Cargo.lock",
		"requirements.txt",
		"noext",
	} {
		if got := connector.LanguageForPath(path); got != core.LangUnclassified {
			t.Errorf("LanguageForPath(%q) = %q, want %q", path, got, core.LangUnclassified)
		}
	}
}

// TestLanguageForPath_JSONExtensionWinsOverOldLockfileHeuristic documents a
// deliberate behavior change: the old langFromPath special-cased
// "package-lock.json" as "js" (a lockfile heuristic with no extractor behind
// it). The one-registry design derives language purely from the registered
// extractor set, so a .json file — including this one — is "json".
func TestLanguageForPath_JSONExtensionWinsOverOldLockfileHeuristic(t *testing.T) {
	if got := connector.LanguageForPath("package-lock.json"); got != "json" {
		t.Errorf("LanguageForPath(%q) = %q, want %q", "package-lock.json", got, "json")
	}
}

func TestRegisteredExtensions_SortedAndNonEmpty(t *testing.T) {
	exts := connector.RegisteredExtensions()
	if len(exts) == 0 {
		t.Fatal("RegisteredExtensions returned nothing")
	}
	for i := 1; i < len(exts); i++ {
		if exts[i-1] >= exts[i] {
			t.Fatalf("RegisteredExtensions not sorted: %q >= %q", exts[i-1], exts[i])
		}
	}
	// Every registered key must resolve to a non-unclassified language.
	for _, ext := range exts {
		path := "x" + ext
		if ext[0] != '.' {
			path = ext // full basename key, e.g. "Dockerfile", "go.mod"
		}
		if got := connector.LanguageForPath(path); got == core.LangUnclassified {
			t.Errorf("registered key %q classifies as unclassified", ext)
		}
	}
}
