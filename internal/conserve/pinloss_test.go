package conserve

// pinloss_test.go — PR-1 (ADR-057 decision 4, docs/design/served-not-kept.md
// section 3): PinLosses over the real i28 rename.
//
// Fixture provenance (testdata/pinloss/i28/, stored as .txt so no Go tool and no
// index ever reads them as source): *.base.txt are f67f06a's own files, the base
// every guards cell ran against; backend_test.go.head.txt and
// platform_store_test.go.head.txt are the head of guards-fathom-r2
// i28/full/t2, the cell CV-M1 replayed as "pilot 8" (O160), whose agent replaced
// TestBackendForPath_DuckDBLoudlyRefused with TestBackendForPath_DuckDBOpensAsSQLite
// and left invariants.md alone; invariants.md.retired.txt is guards-fathom-r2
// i28/full/t1's head invariants.md, whose agent rewrote I28's row to name the new
// test. The unchanged cross-file move and the pure rename are built in the test
// from those real files with plain string edits.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	pinBackendTest  = "internal/store/backend_test.go"
	pinPlatformTest = "internal/store/platform_store_test.go"
	pinInvariants   = "docs/intent/invariants.md"
	pinMovedTo      = "internal/store/resolve_store_files_test.go"
)

func readPinFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "pinloss", "i28", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// cutFunc removes the top-level func named name (with the doc comment directly
// above it) from src and returns the rest and the removed text.
func cutFunc(t *testing.T, src, name string) (rest, fn string) {
	t.Helper()
	start := strings.Index(src, "\nfunc "+name+"(")
	if start < 0 {
		t.Fatalf("func %s not in fixture", name)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("func %s has no closing brace", name)
	}
	end += start + len("\n}\n")
	docStart := start
	for {
		prev := strings.LastIndex(src[:docStart], "\n")
		if prev < 0 || !strings.HasPrefix(src[prev+1:docStart], "//") {
			break
		}
		docStart = prev
	}
	return src[:docStart] + src[end-1:], src[docStart+1 : end]
}

type pinRevs map[string]map[string]string

func (r pinRevs) read(rev, path string) ([]byte, bool, error) {
	s, ok := r[rev][path]
	return []byte(s), ok, nil
}

func TestPinLossesI28RenameIsLossAndMoveIsNot(t *testing.T) {
	baseBackend := readPinFixture(t, "backend_test.go.base.txt")
	headBackend := readPinFixture(t, "backend_test.go.head.txt")
	basePlatform := readPinFixture(t, "platform_store_test.go.base.txt")
	headPlatform := readPinFixture(t, "platform_store_test.go.head.txt")
	baseInv := readPinFixture(t, "invariants.md.base.txt")
	retiredInv := readPinFixture(t, "invariants.md.retired.txt")

	// The real head, plus an unchanged move: TestResolveStoreFiles (named by
	// I28 too) leaves backend_test.go for a new file, body untouched.
	headRest, moved := cutFunc(t, headBackend, "TestResolveStoreFiles")
	movedFile := "package store\n\nimport (\n\t\"path/filepath\"\n\t\"reflect\"\n\t\"testing\"\n)\n\n" + moved

	base := map[string]string{pinBackendTest: baseBackend, pinPlatformTest: basePlatform, pinInvariants: baseInv}
	head := map[string]string{pinBackendTest: headRest, pinPlatformTest: headPlatform, pinMovedTo: movedFile, pinInvariants: baseInv}
	in := PinLossInput{
		Base: "base", Head: "head",
		ChangedFiles: []string{pinBackendTest, pinPlatformTest, pinMovedTo},
		Read:         pinRevs{"base": base, "head": head}.read,
	}

	got, err := PinLosses(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("PinLosses = %+v, want exactly the i28 loss (the move is not a loss, the edited platform test is silent)", got)
	}
	l := got[0]
	if l.Test != "TestBackendForPath_DuckDBLoudlyRefused" || l.Kind != "deleted" {
		t.Fatalf("loss = %s %s, want deleted TestBackendForPath_DuckDBLoudlyRefused (renamed with a new body)", l.Kind, l.Test)
	}
	wantSrc := []PinSource{{Reason: "invariant_enforcement", Path: pinInvariants, Line: 23}}
	if !reflect.DeepEqual(l.Sources, wantSrc) {
		t.Errorf("sources = %+v, want %+v", l.Sources, wantSrc)
	}
	if l.BaseFile != pinBackendTest || l.BaseLine != 115 {
		t.Errorf("base = %s:%d, want %s:115", l.BaseFile, l.BaseLine, pinBackendTest)
	}
	if l.HeadFile != pinBackendTest || l.ReplacedBy != "TestBackendForPath_DuckDBOpensAsSQLite" || l.HeadLine == 0 {
		t.Errorf("head = %s:%d replaced by %q, want the lone new test in %s", l.HeadFile, l.HeadLine, l.ReplacedBy, pinBackendTest)
	}
	if !l.StillNamedAtHead || l.SourceEdited {
		t.Errorf("still named = %v, source edited = %v; want a silent loss (I28 still names the test, invariants.md untouched)", l.StillNamedAtHead, l.SourceEdited)
	}

	// The same loss when the agent also rewrites I28's row to name its new test
	// (guards-fathom-r2 i28/full/t1): a retired pin, not a silent one.
	head[pinInvariants] = retiredInv
	in.ChangedFiles = append(in.ChangedFiles, pinInvariants)
	got, err = PinLosses(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].StillNamedAtHead || !got[0].SourceEdited {
		t.Fatalf("with I28 rewritten: %+v, want one loss, not still named, source edited", got)
	}

	// A pure rename (same body, new name) of a named test is a loss too.
	renamed := strings.Replace(baseBackend, "func TestResolveStoreFiles(", "func TestResolveStoreFilesPrefersSQLite(", 1)
	in = PinLossInput{
		Base: "base", Head: "head",
		ChangedFiles: []string{pinBackendTest},
		Read: pinRevs{
			"base": {pinBackendTest: baseBackend, pinInvariants: baseInv},
			"head": {pinBackendTest: renamed, pinInvariants: baseInv},
		}.read,
	}
	got, err = PinLosses(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != "renamed" || got[0].Test != "TestResolveStoreFiles" ||
		got[0].RenamedTo != "TestResolveStoreFilesPrefersSQLite" || got[0].HeadFile != pinBackendTest {
		t.Fatalf("pure rename: %+v, want renamed TestResolveStoreFiles -> TestResolveStoreFilesPrefersSQLite", got)
	}
}

// TestPinLossesReadErrorIsReturned: a reader that fails is an operational
// error, never an empty (kept-looking) result.
func TestPinLossesReadErrorIsReturned(t *testing.T) {
	boom := func(rev, path string) ([]byte, bool, error) { return nil, false, os.ErrPermission }
	got, err := PinLosses(context.Background(), PinLossInput{
		Base: "base", Head: "head", ChangedFiles: []string{pinBackendTest}, Read: boom,
	})
	if err == nil || got != nil {
		t.Fatalf("PinLosses = %v, %v; want nil and the read error", got, err)
	}
}
