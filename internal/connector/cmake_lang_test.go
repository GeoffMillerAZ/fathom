package connector

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestLanguageForPath_CMake pins CT-5 (docs/plans/cpp-toolchain.md): CMake
// files classify as language "cmake" — by full basename for CMakeLists.txt
// and by extension for *.cmake modules. The fusion↔connector bijection test
// then forces the extractor registration to exist too.
func TestLanguageForPath_CMake(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"CMakeLists.txt", "cmake"},
		{"src/CMakeLists.txt", "cmake"},
		{"cmake/Lto.cmake", "cmake"},
		{"src/foo.cpp", "cpp"},
		{"include/foo.h", "c"},
	}
	for _, c := range cases {
		if got := LanguageForPath(c.path); got != c.want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestKindForPath_CMake pins the classification kinds for CMake paths.
func TestKindForPath_CMake(t *testing.T) {
	base, ext := "CMakeLists.txt", ""
	c, ok := classify(base, ext)
	if !ok {
		t.Fatal("CMakeLists.txt has no classification")
	}
	if c.Kind != core.KindCMakeTarget {
		t.Errorf("CMakeLists.txt kind = %q, want %q", c.Kind, core.KindCMakeTarget)
	}
}
