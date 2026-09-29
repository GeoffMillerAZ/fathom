package fusion_test

// swift_ingest_test.go — LB-Q-swift, refused at integration (2026-09-29): the
// swift dialect is built and measured (L1 excellent, L2 good against the
// compiler's parser), but 36 of its 642 calibration files (5.6%) parse with an
// error under the pinned grammar, over design §14.2's 5% refusal line. A
// refused language "stays disclosed as unsupported": ingest does not route a
// .swift file to the dialect, extracts nothing from it, and names it in
// Report.UnsupportedLanguages. The dialect itself stays tested
// (TestDialectGoldens/swift, TestResolveFileEdges_SwiftImports) for the day a
// grammar parses the subjects under the line.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestIngest_SwiftIsRefusedAndDisclosed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Package.swift", "// swift-tools-version:5.9\nimport PackageDescription\n\nlet package = Package(name: \"Acme\", targets: [.target(name: \"Core\")])\n")
	writeFile(t, dir, "Sources/Core/Model.swift", "public final class Model {\n    public func save() {}\n}\n")
	writeFile(t, dir, "README.md", "# Acme\n")

	if lang, ok := connector.UnsupportedLanguageForPath("Sources/Core/Model.swift"); !ok || lang != "swift" {
		t.Fatalf("UnsupportedLanguageForPath(.swift) = %q, %v; want swift disclosed (refused, §14.2)", lang, ok)
	}
	obs, report := ingestDir(t, dir, "swift-refused")
	for _, o := range obs {
		if strings.HasSuffix(strings.SplitN(o.SourceRef, ":", 2)[0], ".swift") {
			t.Errorf("a refused language was extracted: %s %s (%s)", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	if got, want := report.UnsupportedLanguages["swift"], 2; got != want {
		t.Errorf("UnsupportedLanguages[swift] = %d, want %d (%v)", got, want, report.UnsupportedLanguages)
	}
}
