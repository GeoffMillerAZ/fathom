package docindex

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestDepDocsNeverDial pins I56 for this package: nothing in docindex may
// import the network or spawn a process — dependency docs come only from
// what the toolchain already placed on disk.
func TestDepDocsNeverDial(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "net" || strings.HasPrefix(p, "net/") || p == "os/exec" {
				t.Fatalf("%s imports %q: docindex must never dial or exec (I56)", n, p)
			}
		}
	}
}

func TestParsePkgRef(t *testing.T) {
	cases := []struct {
		in      string
		want    PkgRef
		wantKey string
		wantErr string
	}{
		{in: "pkg:golang/github.com/x/y@v1.2.3", want: PkgRef{"golang", "github.com/x/y", "v1.2.3"}, wantKey: "pkg:golang/github.com/x/y@v1.2.3"},
		{in: "golang/github.com/x/y@v1.2.3", want: PkgRef{"golang", "github.com/x/y", "v1.2.3"}, wantKey: "pkg:golang/github.com/x/y@v1.2.3"},
		{in: "pkg:NPM/@scope/name@2.0.0", want: PkgRef{"npm", "@scope/name", "2.0.0"}, wantKey: "pkg:npm/@scope/name@2.0.0"},
		{in: "pkg:pypi/requests@2.31.0", want: PkgRef{"pypi", "requests", "2.31.0"}, wantKey: "pkg:pypi/requests@2.31.0"},
		{in: "pkg:golang/github.com/x/y", want: PkgRef{"golang", "github.com/x/y", ""}, wantKey: "pkg:golang/github.com/x/y"},
		{in: "pkg:golang/x@v1#Frob", wantErr: "symbol"},
		{in: "pkg:bogus/x@1", wantErr: "ecosystem"},
		{in: "", wantErr: "empty"},
		{in: "pkg:golang/", wantErr: "ecosystem"},
	}
	for _, c := range cases {
		got, err := ParsePkgRef(c.in)
		if c.wantErr != "" {
			if err == nil || !errors.Is(err, ErrBadPkgKey) || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("ParsePkgRef(%q) err = %v, want ErrBadPkgKey mentioning %q", c.in, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePkgRef(%q): %v", c.in, err)
			continue
		}
		if got != c.want || got.Key() != c.wantKey {
			t.Errorf("ParsePkgRef(%q) = %+v key %q, want %+v key %q", c.in, got, got.Key(), c.want, c.wantKey)
		}
	}
}
