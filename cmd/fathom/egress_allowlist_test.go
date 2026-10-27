package main

// egress_allowlist_test.go — A5 (ADR-083): every package in the fathom
// binary that can reach the network or run a program is named, with the
// reason it needs to and the gate that holds it, in
// testdata/egress-allowlist.txt. TestEgressImportAllowlist reads `go list
// -deps -json ./cmd/fathom` for each release target and lists every
// non-standard package that imports one of egressImports (the standard
// packages that dial, serve, run a program or load one) or calls one of
// egressCalls (the functions that open a socket or start a program without
// such an import: os.StartProcess, syscall.Exec, ForkExec and Socket,
// x/sys/unix's Exec and Socket), found in its source. A new edge (a
// package, or a new import or call in a listed one) fails until it is
// listed with its reason and gate; a listed edge the binary no longer has
// fails too, so the list stays true.
//
// cmd/fathom itself has to stay on the list, so this static check cannot
// see a new outbound call inside it: the runtime tripwire in the end-to-end
// cold-cache ingest test (goProxyTripwire in
// TestCmdIngest_ColdModuleCacheNamesTheRemedy,
// ingest_typed_disclosure_test.go) covers the go toolchain's module
// downloads there.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// egressImports are the standard packages that open a connection, serve
// one, run a program or load one.
var egressImports = []string{
	"crypto/tls", "log/syslog", "net", "net/http", "net/http/cgi", "net/http/fcgi", "net/http/httputil",
	"net/rpc", "net/rpc/jsonrpc", "net/smtp", "net/textproto", "os/exec", "plugin",
}

// egressCalls are the functions that open a socket or start a program with
// none of egressImports: "<import path>.<name>". TestEgressImportAllowlist
// finds a call (or any other reference) by selector on the import that
// names its package.
var egressCalls = []string{
	"golang.org/x/sys/unix.Exec", "golang.org/x/sys/unix.Socket",
	"os.StartProcess", "syscall.Exec", "syscall.ForkExec", "syscall.Socket",
}

// egressCallPackages are the packages egressCalls belong to.
var egressCallPackages = []string{"golang.org/x/sys/unix", "os", "syscall"}

// egressCallFiles memoizes each parsed file's egressCalls across targets.
type egressCallFiles map[string][]string

// calls returns the egressCalls file references, sorted: a selector whose
// left side is the name the file imports one of egressCallPackages under.
func (m egressCallFiles) calls(t *testing.T, file string) []string {
	t.Helper()
	if got, ok := m[file]; ok {
		return got
	}
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	var found []string
	if bytes.Contains(src, []byte("Exec")) || bytes.Contains(src, []byte("StartProcess")) || bytes.Contains(src, []byte("Socket")) {
		f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		names := map[string]string{}
		for _, spec := range f.Imports {
			imp, _ := strconv.Unquote(spec.Path.Value)
			if !slices.Contains(egressCallPackages, imp) {
				continue
			}
			name := path.Base(imp)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name == "." {
				t.Errorf("%s dot-imports %s: its egress calls cannot be found by selector", file, imp)
			}
			names[name] = imp
		}
		seen := map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] != "" {
					if call := names[id.Name] + "." + sel.Sel.Name; slices.Contains(egressCalls, call) && !seen[call] {
						seen[call] = true
						found = append(found, call)
					}
				}
			}
			return true
		})
		slices.Sort(found)
	}
	m[file] = found
	return found
}

// egressAllowlistPath is the reviewed list, one package per line:
// "<package> | <imports and calls, space-separated> | <reason> | <gate>".
const egressAllowlistPath = "testdata/egress-allowlist.txt"

// egressTargets are the release targets (.goreleaser.yaml): goos, goarch,
// CGO_ENABLED.
var egressTargets = [][3]string{{"linux", "amd64", "0"}, {"linux", "arm64", "0"}, {"darwin", "arm64", "1"}, {"darwin", "amd64", "1"}}

// egressEdges returns "package import" for every non-standard package in
// cmd/fathom's dependency set, on every release target, that imports one of
// egressImports, and "package call" for each of egressCalls its source
// references.
func egressEdges(t *testing.T) map[string]bool {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	edges := map[string]bool{}
	parsed := egressCallFiles{}
	for _, target := range egressTargets {
		cmd := exec.CommandContext(t.Context(), "go", "list", "-e", "-deps", "-json=ImportPath,Standard,Imports,Error,Dir,GoFiles,CgoFiles", ".")
		cmd.Env = append(os.Environ(), "GOOS="+target[0], "GOARCH="+target[1], "CGO_ENABLED="+target[2], "GOFLAGS=")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s/%s go list -deps: %v\n%s", target[0], target[1], err, stderr.String())
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		n := 0
		for {
			var p struct {
				ImportPath        string
				Standard          bool
				Imports           []string
				Error             *struct{ Err string }
				Dir               string
				GoFiles, CgoFiles []string
			}
			if err := dec.Decode(&p); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("decode go list output: %v", err)
			}
			n++
			if p.Error != nil {
				t.Errorf("%s/%s: %s: %s", target[0], target[1], p.ImportPath, p.Error.Err)
			}
			if p.Standard {
				continue
			}
			callsPossible := false
			for _, imp := range p.Imports {
				if slices.Contains(egressImports, imp) {
					edges[p.ImportPath+" "+imp] = true
				}
				callsPossible = callsPossible || slices.Contains(egressCallPackages, imp)
			}
			if !callsPossible {
				continue
			}
			for _, name := range append(slices.Clone(p.GoFiles), p.CgoFiles...) {
				for _, call := range parsed.calls(t, filepath.Join(p.Dir, name)) {
					edges[p.ImportPath+" "+call] = true
				}
			}
		}
		if n == 0 {
			t.Fatalf("%s/%s: go list -deps listed no packages", target[0], target[1])
		}
	}
	return edges
}

// readEgressAllowlist parses the allowlist into "package import" edges,
// failing on a malformed line, an empty reason or gate, or a duplicate.
func readEgressAllowlist(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(egressAllowlistPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	listed := map[string]bool{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cols := strings.Split(text, " | ")
		if len(cols) != 4 {
			t.Errorf("%s:%d: want \"<package> | <imports> | <reason> | <gate>\", got %q", egressAllowlistPath, line, text)
			continue
		}
		pkg, imports, reason, gate := strings.TrimSpace(cols[0]), strings.Fields(cols[1]), strings.TrimSpace(cols[2]), strings.TrimSpace(cols[3])
		if pkg == "" || len(imports) == 0 || reason == "" || gate == "" {
			t.Errorf("%s:%d: %q: package, imports, reason and gate are all required", egressAllowlistPath, line, text)
			continue
		}
		if seen[pkg] {
			t.Errorf("%s:%d: %s is listed twice", egressAllowlistPath, line, pkg)
		}
		seen[pkg] = true
		for _, imp := range imports {
			if !slices.Contains(egressImports, imp) && !slices.Contains(egressCalls, imp) {
				t.Errorf("%s:%d: %s: %q is not one of %v or %v", egressAllowlistPath, line, pkg, imp, egressImports, egressCalls)
			}
			listed[pkg+" "+imp] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return listed
}

// TestEgressImportAllowlist (A5, ADR-083): the binary's network and exec
// edges are exactly the reviewed list.
func TestEgressImportAllowlist(t *testing.T) {
	got := egressEdges(t)
	listed := readEgressAllowlist(t)
	for _, e := range slices.Sorted(maps.Keys(got)) {
		if !listed[e] {
			pkg, imp, _ := strings.Cut(e, " ")
			t.Errorf("new egress edge: %s uses %s. Review it, then list it in %s with its reason and the gate that holds it", pkg, imp, egressAllowlistPath)
		}
	}
	for _, e := range slices.Sorted(maps.Keys(listed)) {
		if !got[e] {
			pkg, imp, _ := strings.Cut(e, " ")
			t.Errorf("stale allowlist entry: %s no longer uses %s on any release target; remove it from %s", pkg, imp, egressAllowlistPath)
		}
	}
}
