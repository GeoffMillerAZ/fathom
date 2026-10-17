package mcp

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestKindToLang_CFamilyAndCMake pins CT-6 (docs/plans/cpp-toolchain.md):
// the languages field a service reports must not silently drop C-family
// languages — a pure C++ service used to report languages: [].
func TestKindToLang_CFamilyAndCMake(t *testing.T) {
	cases := []struct {
		kind core.SourceKind
		want string
	}{
		{core.KindCppClass, "cpp"},
		{core.KindCppFunc, "cpp"},
		{core.KindCppNamespace, "cpp"},
		{core.KindCFunc, "c"},
		{core.KindCStruct, "c"},
		{core.KindCTypedef, "c"},
		{core.KindCMakeTarget, "cmake"},
		{core.KindCMakeFunction, "cmake"},
		{core.KindRustFn, "rust"},
		{core.KindCSharpClass, "csharp"},
		{core.KindCUEDefinition, "cue"},
		// Regression guards: the C prefix must not swallow neighbours.
		{core.KindGoFunc, "go"},
		{core.KindJSFunc, "javascript"},
	}
	for _, c := range cases {
		if got := kindToLang(c.kind); got != c.want {
			t.Errorf("kindToLang(%q) = %q, want %q", c.kind, got, c.want)
		}
	}
}

// TestKindToLang_TypeScriptKinds: the TS-only kinds (interface, type alias,
// enum) report "typescript"; the shared JS* kinds stay "javascript" because
// a stored kind does not carry the file extension.
func TestKindToLang_TypeScriptKinds(t *testing.T) {
	for _, k := range []core.SourceKind{core.KindTSInterface, core.KindTSType, core.KindTSEnum} {
		if got := kindToLang(k); got != "typescript" {
			t.Errorf("kindToLang(%q) = %q, want typescript", k, got)
		}
	}
	got := langStringsForKinds([]core.SourceKind{core.KindJSFunc, core.KindTSInterface})
	if len(got) != 2 || got[0] != "javascript" || got[1] != "typescript" {
		t.Errorf("langStringsForKinds = %v, want [javascript typescript]", got)
	}
}

// TestKindToLang_JSONKeyIsNotJavaScript pins O1123: JSONKey, the kind every
// .json key gets, shares the two-letter prefix of the JS* kinds but names
// no programming language.
func TestKindToLang_JSONKeyIsNotJavaScript(t *testing.T) {
	if got := kindToLang(core.KindJSONKey); got == "javascript" || got == "typescript" {
		t.Errorf("kindToLang(%q) = %q, want no JavaScript language", core.KindJSONKey, got)
	}
}

// TestLangStringsForSummary_JSONKeyWithoutJSFiles pins O1123 on the summary
// path: JSONKey kinds with no js or ts file class list no javascript, with
// file classes known and on the kind-only fallback alike.
func TestLangStringsForSummary_JSONKeyWithoutJSFiles(t *testing.T) {
	kinds := []core.SourceKind{core.KindJSONKey, core.KindGoFunc}
	for _, fileLangs := range [][]string{{"go"}, nil} {
		got := langStringsForSummary(kinds, fileLangs)
		for _, l := range got {
			if l == "javascript" || l == "typescript" {
				t.Errorf("langStringsForSummary(%v, %v) = %v, lists %s from JSONKey alone", kinds, fileLangs, got, l)
			}
		}
		if len(got) != 1 || got[0] != "go" {
			t.Errorf("langStringsForSummary(%v, %v) = %v, want [go]", kinds, fileLangs, got)
		}
	}
}
