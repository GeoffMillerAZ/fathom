package specs_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestFoundationsImportDownward checks production imports, including files with
// inactive build tags. External tests may compose kits to verify compatibility.
func TestFoundationsImportDownward(t *testing.T) {
	const module = "github.com/geoffmilleraz/agentic-kit/"
	err := filepath.WalkDir("../foundations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "_reference" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if strings.Contains(name, "/_reference/") || (strings.HasPrefix(name, module) && !strings.HasPrefix(name, module+"foundations/")) {
				t.Errorf("%s imports %s: foundations may import only stdlib and sibling foundations", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
