package lang_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLangNeverDialsOrWrites watches scratch roots, so a write to a path the
// code hard-codes elsewhere would pass it. This test reads the source of every
// fathom package the door links: a call that writes to the file system may
// appear only in a file whose path comes from the caller (a cache directory the
// consumer passes in), and no write names an absolute path as a literal. A
// process that starts another process is refused outright.

// writeCalls are the os functions that create, change or remove a file.
var writeCalls = map[string]bool{
	"Create": true, "OpenFile": true, "WriteFile": true, "Mkdir": true, "MkdirAll": true,
	"MkdirTemp": true, "CreateTemp": true, "Rename": true, "Remove": true, "RemoveAll": true,
	"Symlink": true, "Link": true, "Truncate": true, "Chmod": true, "Chown": true, "Chtimes": true,
}

// callerPathWriters are the files that write, each to a path its caller gave:
// the compilation cache directory EnableCompilationCache validates, and the
// tenancy ring and day log, which open the path their caller names and which
// the door never opens.
var callerPathWriters = map[string]bool{
	"pkg/lang/cache.go":                        true,
	"internal/extractor/tsengine/compcache.go": true,
	"internal/tenancy/ring.go":                 true,
	"internal/tenancy/daysummary.go":           true,
}

// writeFindings returns what is wrong with the source files in dirs, keyed by
// repository-relative path. allowed names the files that may write at all.
func writeFindings(t *testing.T, files map[string][]byte, allowed map[string]bool) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	for rel, src := range files {
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "os/exec" {
				out = append(out, rel+": imports os/exec")
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "os" || !writeCalls[sel.Sel.Name] {
				return true
			}
			where := fset.Position(call.Pos()).String()
			if !allowed[rel] {
				out = append(out, where+": os."+sel.Sel.Name+" in a file that may not write")
			}
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, _ := strconv.Unquote(lit.Value); strings.HasPrefix(s, "/") || filepath.IsAbs(s) {
						out = append(out, where+": os."+sel.Sel.Name+" writes the literal absolute path "+lit.Value)
					}
				}
			}
			return true
		})
	}
	return out
}

// closureSources reads the non-test Go files of every fathom package the door
// links, keyed by repository-relative path.
func closureSources(t *testing.T) map[string][]byte {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Standard,Dir,GoFiles", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, stderr.String())
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p struct {
			ImportPath string
			Standard   bool
			Dir        string
			GoFiles    []string
		}
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if p.Standard || !strings.HasPrefix(p.ImportPath, "github.com/geoffmilleraz/fathom/") {
			continue
		}
		for _, name := range p.GoFiles {
			b, err := os.ReadFile(filepath.Join(p.Dir, name))
			if err != nil {
				t.Fatal(err)
			}
			rel, err := filepath.Rel(root, filepath.Join(p.Dir, name))
			if err != nil {
				t.Fatal(err)
			}
			files[filepath.ToSlash(rel)] = b
		}
	}
	if len(files) < 20 || files["pkg/lang/reader.go"] == nil {
		t.Fatalf("read %d source files and not pkg/lang/reader.go", len(files))
	}
	return files
}

func TestLangWritesOnlyToCallerPaths(t *testing.T) {
	files := closureSources(t)
	for _, problem := range writeFindings(t, files, callerPathWriters) {
		t.Error(problem)
	}
	for path := range callerPathWriters {
		if files[path] == nil {
			t.Errorf("the allowlist names %s, which the door no longer links; remove it", path)
		}
	}
}

func TestWriteScanFailsOnBrokenSources(t *testing.T) {
	for name, src := range map[string]string{
		"hard-coded path":     "package p\nimport \"os\"\nfunc f() { os.WriteFile(\"/var/tmp/x\", nil, 0o600) }\n",
		"write in a new file": "package p\nimport \"os\"\nfunc f(p string) { os.MkdirAll(p, 0o700) }\n",
		"remove":              "package p\nimport \"os\"\nfunc f(p string) { os.Remove(p) }\n",
		"starts a process":    "package p\nimport \"os/exec\"\nvar _ = exec.Command\n",
	} {
		if got := writeFindings(t, map[string][]byte{"pkg/lang/x.go": []byte(src)}, nil); len(got) == 0 {
			t.Errorf("%s was not caught", name)
		}
	}
	allowed := map[string]bool{"pkg/lang/cache.go": true}
	if got := writeFindings(t, map[string][]byte{"pkg/lang/cache.go": []byte("package p\nimport \"os\"\nfunc f(p string) { os.MkdirAll(p, 0o700) }\n")}, allowed); len(got) != 0 {
		t.Errorf("a write to a caller's path in an allowed file was refused: %v", got)
	}
	if got := writeFindings(t, map[string][]byte{"pkg/lang/cache.go": []byte("package p\nimport \"os\"\nfunc f() { os.MkdirAll(\"/etc/x\", 0o700) }\n")}, allowed); len(got) == 0 {
		t.Error("a hard-coded absolute path in an allowed file was not caught")
	}
}
