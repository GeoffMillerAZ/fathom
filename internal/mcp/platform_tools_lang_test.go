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
