package fusion_test

// LB-Q-cmake (design §6): add_subdirectory(dir) and include(file) are whole-file imports through the
// relative-path strategy (index_files CMakeLists.txt, suffix .cmake); a target a file links resolves to the
// definition in a file it imports. A link written in a child directory to a target of its parent has no
// import to bind it: CMake's target namespace is global, an import graph is not (the measured ceiling).

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_CMakeSubdirectories(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "cmakefixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "cmake", fixtureFiles(t, dir, "CMakeLists.txt", ".cmake"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "cmake", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	want := []string{
		"CMakeLists.txt|app -> src/CMakeLists.txt|core",
		"src/CMakeLists.txt|core -> src/CMakeLists.txt|util",
	}
	got := namedEdges(edges, symbols)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
