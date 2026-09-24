package conserve

// deprecatedcallers_test.go — CV-30 (design §3.2): a call site to a
// Deprecated: symbol that is new in the range is a finding; a call site
// already present at base is not.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeGoFile writes content to dir/rel, creating parent directories —
// deprecatedSymbolNames reads real files under in.Root, so the
// declaration side of these tests needs one, unlike BaseFile/HeadFile
// (map-backed closures suffice for those).
func writeGoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mapFile is a BaseFile/HeadFile closure backed by a plain map: "ok" iff
// the key is present. Read errors are never simulated here — that path
// is exercised by NewInput's own git-integration tests (input_test.go).
func mapFile(m map[string]string) func(string) ([]byte, bool, error) {
	return func(path string) ([]byte, bool, error) {
		v, ok := m[path]
		if !ok {
			return nil, false, nil
		}
		return []byte(v), true, nil
	}
}

func TestDeprecatedCallers_NewCallSiteInRange(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/old.go", `package pkg

// OldFunc does the old thing.
//
// Deprecated: use NewFunc instead.
func OldFunc() int { return 1 }
`)

	base := map[string]string{
		// This changed file existed at base, but did not call OldFunc yet
		// — its own presence/absence flips inside the range.
		"caller.go": "package pkg\n\nfunc Unrelated() { println(\"hi\") }\n",
	}
	head := map[string]string{
		"caller.go": "package pkg\n\nfunc Unrelated() { println(\"hi\") }\n\nfunc NewCaller() { OldFunc() }\n",
	}

	in := Input{
		Root:         root,
		ChangedFiles: []string{"caller.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}

	findings, missing := DeprecatedCallers(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1 (only the NEW call site)", findings)
	}
	f := findings[0]
	if f.Plane != "absence" || f.Kind != "new_deprecated_caller" || f.Key != "OldFunc" {
		t.Errorf("finding = %+v, want plane=absence kind=new_deprecated_caller key=OldFunc", f)
	}
	if f.Declared != DeclaredNone {
		t.Errorf("Declared = %q, want none (no excuse mechanism)", f.Declared)
	}
	if f.Head == "" {
		t.Errorf("Head citation is empty")
	}
}

func TestDeprecatedCallers_NoNewCallSiteIsClean(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/old.go", `package pkg

// Deprecated: use NewFunc instead.
func OldFunc() int { return 1 }
`)
	base := map[string]string{"caller.go": "package pkg\n\nfunc F() { OldFunc() }\n"}
	head := map[string]string{"caller.go": "package pkg\n\nfunc F() { OldFunc() }\n// a comment added, nothing new\n"}

	in := Input{
		Root:         root,
		ChangedFiles: []string{"caller.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}
	findings, missing := DeprecatedCallers(in)
	if len(missing) != 0 || len(findings) != 0 {
		t.Fatalf("findings=%+v missing=%v, want both empty", findings, missing)
	}
}

func TestDeprecatedCallers_NewFileCallingDeprecatedIsNew(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/old.go", `package pkg

// Deprecated: use NewFunc instead.
func OldFunc() int { return 1 }
`)
	head := map[string]string{"newcaller.go": "package pkg\n\nfunc F() { OldFunc() }\n"}

	in := Input{
		Root:         root,
		ChangedFiles: []string{"newcaller.go"},
		BaseFile:     mapFile(map[string]string{}), // absent at base entirely
		HeadFile:     mapFile(head),
	}
	findings, _ := DeprecatedCallers(in)
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1 (a wholly new file counts as new usage)", findings)
	}
}

func TestDeprecatedCallers_NoDeprecatedSymbolsIsConserved(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/clean.go", "package pkg\n\nfunc F() {}\n")

	in := Input{
		Root:         root,
		ChangedFiles: []string{"caller.go"},
		BaseFile:     mapFile(map[string]string{}),
		HeadFile:     mapFile(map[string]string{"caller.go": "package pkg\n\nfunc G() { F() }\n"}),
	}
	findings, missing := DeprecatedCallers(in)
	if len(findings) != 0 || len(missing) != 0 {
		t.Fatalf("findings=%+v missing=%v, want both empty (nothing is deprecated)", findings, missing)
	}
}

func TestDeprecatedCallers_NoRootIsMissing(t *testing.T) {
	in := Input{ChangedFiles: []string{"a.go"}, BaseFile: mapFile(nil), HeadFile: mapFile(nil)}
	findings, missing := DeprecatedCallers(in)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
	if len(missing) != 1 {
		t.Fatalf("missing = %v, want exactly one reason", missing)
	}
}

func TestDeprecatedCallers_PlaneName(t *testing.T) {
	var p deprecatedCallersPlane
	if got := p.Name(); got != "absence" {
		t.Errorf("Name() = %q, want absence", got)
	}
}
