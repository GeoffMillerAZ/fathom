package scope_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// capabilityMethodDisposition classifies every method of every OPTIONAL
// store capability (an exported interface in internal/store other than
// Store itself) for the read chokepoint, exactly as storeMethodDisposition
// does for store.Store: a capability's read reaches content too, and an
// unclassified one is a silent bypass of the filter. Every entry here is an
// exemption with its reason ("" would mean filtered, and none of these is).
var capabilityMethodDisposition = map[string]string{
	// FileImportsStore (RT-108): the ingest-time file-resolve pass.
	"FileImportsStore.GetFileImports":     "ingest-time read of the pass's own parsed import specifiers; suppression must not change what gets indexed — never served to a surface",
	"FileImportsStore.SetFileImports":     "write",
	"FileImportsStore.DeleteFileImports":  "write",
	"FileImportsStore.SourceObservations": "ingest-time read of the file-resolve pass's own observations (suppression must not change what gets indexed); its only caller is internal/fusion's resolveLang — a request-path caller must use the filtered QueryObservations instead",
	// IngestStateStore (RT-109): a service's ingest progress row.
	"IngestStateStore.SetIngestState":    "write",
	"IngestStateStore.GetIngestState":    "service-level state (owner pid, start, corpus root); carries no path-anchored content",
	"IngestStateStore.DeleteIngestState": "write",
	// KindObservationsStore (EW-57): the doc-xref pass's Lambda gate.
	"KindObservationsStore.ObservationsOfKinds": "ingest-time read of the declaring kinds' rows for the doc-xref pass's Lambda gate (suppression must not change what gets indexed); its only caller is internal/fusion's serviceDeclaresLambda, which reduces it to whether the pass runs and serves nothing — a request-path caller must use the filtered QueryObservations instead",
}

// notStoreCapabilities are exported interfaces in internal/store that no
// store implements: seams the store calls out to.
var notStoreCapabilities = map[string]string{
	"Store":        "classified method by method in storeMethodDisposition",
	"PathMatcher":  "purge's pattern seam (internal/filter implements it)",
	"CacheFlusher": "purge's result-cache seam",
	"VectorPurger": "purge's embedding-index seam",
	"MemoryPurger": "purge's anchored-memory seam",
}

// TestSuppressionCoversOptionalStoreCapabilities (SP-2, RT-108): every
// method of every exported interface in internal/store is classified — an
// optional capability's content-bearing read cannot be added (or exported
// on SQLiteStore and MemoryStore) without a written disposition.
func TestSuppressionCoversOptionalStoreCapabilities(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "store", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var unclassified []string
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, sp := range gd.Specs {
				ts := sp.(*ast.TypeSpec)
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				if _, skip := notStoreCapabilities[ts.Name.Name]; skip {
					continue
				}
				for _, m := range it.Methods.List {
					for _, n := range m.Names {
						key := ts.Name.Name + "." + n.Name
						seen[key] = true
						if reason, ok := capabilityMethodDisposition[key]; !ok || strings.TrimSpace(reason) == "" {
							unclassified = append(unclassified, key)
						}
					}
				}
			}
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf("optional store capability method(s) %v have no suppression disposition — classify each in capabilityMethodDisposition", unclassified)
	}
	for k := range capabilityMethodDisposition {
		if !seen[k] {
			t.Errorf("capabilityMethodDisposition names %q, which internal/store no longer declares", k)
		}
	}
}
