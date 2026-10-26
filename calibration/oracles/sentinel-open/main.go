// Bench-only parse coverage of original policy files; never a native-runtime oracle.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/glennsarti/sentinel-parser/sentinel/ast"
	"github.com/glennsarti/sentinel-parser/sentinel/parser"
)

type totals struct {
	Files       int `json:"files"`
	Bytes       int `json:"bytes"`
	Errors      int `json:"files_in_error"`
	Panics      int `json:"panics"`
	Diagnostics int `json:"error_diagnostics"`
	BadNodes    int `json:"bad_nodes"`
	Calls       int `json:"call_nodes"`
}

func parse(src []byte) (result totals) {
	result.Files, result.Bytes = 1, len(src)
	defer func() {
		if recover() != nil {
			result.Panics = 1
			result.Errors = 1
		}
	}()
	tree, _, diags, err := parser.ParseFile("latest", "probe.sentinel", src)
	result.Diagnostics = len(diags.Errors())
	if err != nil || tree == nil || diags.HasErrors() {
		result.Errors = 1
	}
	if tree != nil {
		var visit ast.VisitFunc
		visit = func(n ast.Node) ast.VisitFunc {
			switch n.(type) {
			case *ast.BadExpression, *ast.BadStatement:
				result.BadNodes++
			case *ast.CallExpression:
				result.Calls++
			}
			return visit
		}
		if err := ast.Walk(visit, tree); err != nil {
			result.Errors = 1
		}
	}
	if result.BadNodes > 0 {
		result.Errors = 1
	}
	return
}

func add(a *totals, b totals) {
	a.Files += b.Files
	a.Bytes += b.Bytes
	a.Errors += b.Errors
	a.Panics += b.Panics
	a.Diagnostics += b.Diagnostics
	a.BadNodes += b.BadNodes
	a.Calls += b.Calls
}

func main() {
	root := flag.String("subject", "", "original subject root")
	identity := flag.String("identity", "", "public repository and immutable revision")
	details := flag.String("details", "", "optional private per-file receipt under workspace")
	flag.Parse()
	if *root == "" || *identity == "" {
		panic("subject and identity required")
	}
	var paths []string
	err := filepath.WalkDir(*root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".sentinel") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		panic("no original .sentinel files")
	}
	groups := map[string]*totals{"all": {}, "mock_paths": {}, "other_paths": {}}
	manifest := sha256.New()
	var records []map[string]any
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			panic(err)
		}
		rel, err := filepath.Rel(*root, p)
		if err != nil {
			panic(err)
		}
		rel = filepath.ToSlash(rel)
		h := sha256.Sum256(src)
		fmt.Fprintf(manifest, "%s\x00%x\n", rel, h)
		r := parse(src)
		add(groups["all"], r)
		group := "other_paths"
		for _, c := range strings.Split(rel, "/") {
			if c == "mock" || c == "mocks" || strings.HasPrefix(c, "mock-") {
				group = "mock_paths"
			}
		}
		add(groups[group], r)
		records = append(records, map[string]any{"path": rel, "sha256": hex.EncodeToString(h[:]), "counts": r})
	}
	version := "unknown"
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range build.Deps {
			if dep.Path == "github.com/glennsarti/sentinel-parser" {
				version = dep.Version
			}
		}
	}
	if *details != "" {
		b, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(*details, append(b, '\n'), 0600); err != nil {
			panic(err)
		}
	}
	out := map[string]any{"subject": *identity, "parser": "glennsarti/sentinel-parser", "parser_version": version,
		"spec": "latest supported by pinned parser", "source_manifest_sha256": hex.EncodeToString(manifest.Sum(nil)),
		"groups": groups, "scope": "every original .sentinel file, no rewriting or exclusions", "runtime_used": false,
		"classification": "open product-parser feasibility only; not independent oracle accuracy"}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
