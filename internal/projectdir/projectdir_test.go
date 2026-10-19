package projectdir

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestProjectDirIsTheOneSpelling: every project-scope name joins under a
// root through File, and no non-test Go file in cmd/ or internal/ outside
// this package (and internal/fathomhome, the user-scope twin) spells a
// .fathom path as a bare literal. A literal with whitespace in it is display
// text (a message or a help line naming the file), not a join.
func TestProjectDirIsTheOneSpelling(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tc := range []struct {
		got, want string
	}{
		{Config.Path("/r"), filepath.Join("/r", ".fathom", "config.toml")},
		{SCIP.Path("/r", "*.scip"), filepath.Join("/r", ".fathom", "scip", "*.scip")},
		{ArchConformance.Path("/r"), "/r" + sep + ".fathom" + sep + "rules" + sep + "archconformance.cue"},
		{Guard.Path(""), filepath.Join(".fathom", "guard.cue")},
		{Under("/r"), filepath.Join("/r", ".fathom")},
		{Guard.Rel(), ".fathom/guard.cue"},
		{Trace.Rel("soc2.cue"), ".fathom/trace/soc2.cue"},
		{File("trace/soc2.cue").Path("/r"), filepath.Join("/r", ".fathom", "trace", "soc2.cue")},
		{ArchConformance.Rel(), ".fathom/rules/archconformance.cue"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
	// Every name's on-disk join and its repository-relative spelling agree.
	for _, f := range []File{Config, Capabilities, Modules, Guard, Concerns, Risk, Learning, Layers, Rules, ArchConformance, Trace, Observations, ObservationLog, Owned, Leases, SCIP, Memories} {
		if got, want := f.Path("/r"), filepath.Join("/r", filepath.FromSlash(f.Rel())); got != want {
			t.Errorf("%s: Path = %q, want %q", f, got, want)
		}
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	exempt := map[string]bool{"internal/projectdir": true, "internal/fathomhome": true}
	var hand []string
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				switch {
				case exempt[rel], d.Name() == "testdata", d.Name() == "vendor", d.Name() == "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, uerr := strconv.Unquote(lit.Value)
				if uerr == nil && bareSpelling(v) {
					hand = append(hand, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+": "+lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range hand {
		t.Errorf("%s spells a .fathom path by hand; join it through projectdir (a File's Path or Rel, or Under)", h)
	}
}

// bareSpelling reports whether v is the project directory or a path under
// it, spelled with no whitespace (so a path, not a sentence that names one).
func bareSpelling(v string) bool {
	if v != Dir && !strings.HasPrefix(v, Dir+"/") {
		return false
	}
	return !strings.ContainsAny(v, " \t\n")
}
