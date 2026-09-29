package fusion_test

// swift_ingest_test.go — LB-Q-swift: a .swift file reaches the swift dialect through discovery and
// dispatch (it was disclosed as unsupported: "no extractor registered"), and what the dialect
// extracts reaches the store — a protocol and a class with their members, a struct, an enum and its
// cases, an extension qualifying its members under the extended type, a top-level function and
// variable, calls named by callee, and an `import` of a SwiftPM target resolved through Package.swift
// to a file-tier edge.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_SwiftReachesTheSwiftDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Package.swift": `// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "Acme",
    targets: [
        .executableTarget(name: "App", dependencies: ["Core"]),
        .target(name: "Core"),
    ]
)
`,
		"Sources/App/main.swift": `import Core

let registry = Model.build()

func run() {
    let m = Model(name: "x")
    m.save()
    print(describe(m))
}
`,
		"Sources/Core/Model.swift": `public enum Kind { case small, large }

public protocol Named {
    var name: String { get }
}

public final class Model: Named {
    public let name: String

    public init(name: String) { self.name = name }

    public static func build() -> Model { Model(name: "built") }

    public func save() {
        log()
    }

    private func log() {}
}

extension Model {
    var label: String { name }
}

public func describe(_ m: Model) -> String { m.label }
`,
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if lang, ok := connector.UnsupportedLanguageForPath("Sources/App/main.swift"); ok {
		t.Fatalf("a .swift file is still disclosed as unsupported (%s)", lang)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "swift-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	// An extension is named by the type it extends, so it shares its symbol name with the type's own
	// definition; both observations are stored, and the map below holds the definitions.
	got := map[string]core.Observation{}
	extensions := 0
	for _, o := range obs {
		if o.SourceKind == "SwiftExtension" {
			extensions++
			continue
		}
		got[o.Symbol] = o
	}
	if extensions != 1 {
		t.Errorf("SwiftExtension observations = %d, want 1 (extension Model)", extensions)
	}
	for symbol, kind := range map[string]core.SourceKind{
		"registry":    "SwiftVariable",
		"run":         "SwiftFunc",
		"Kind":        "SwiftEnum",
		"Kind.small":  "SwiftEnumCase",
		"Named":       "SwiftProtocol",
		"Named.name":  "SwiftProperty",
		"Model":       "SwiftClass",
		"Model.name":  "SwiftProperty",
		"Model.init":  "SwiftInit",
		"Model.build": "SwiftMethod",
		"Model.save":  "SwiftMethod",
		"Model.log":   "SwiftMethod",
		"Model.label": "SwiftProperty",
		"describe":    "SwiftFunc",
	} {
		o, ok := got[symbol]
		if !ok {
			t.Errorf("no observation for %q (got %v)", symbol, keysOf(got))
			continue
		}
		if o.SourceKind != kind {
			t.Errorf("%s: kind %q, want %q", symbol, o.SourceKind, kind)
		}
	}
	for symbol, want := range map[string]string{
		"run":         "Model,describe,print,save",
		"registry":    "build",
		"Model.save":  "log",
		"Model.build": "Model",
	} {
		if calls := strings.Join(got[symbol].Calls, ","); calls != want {
			t.Errorf("%s calls = %q, want %q", symbol, calls, want)
		}
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "Sources/App/main.swift" && strings.HasSuffix(e.ToSymbol, "describe") {
			found = true
		}
	}
	if !found {
		t.Errorf("no file-resolved edge from Sources/App/main.swift to describe (import Core); edges %+v", edges)
	}
}
