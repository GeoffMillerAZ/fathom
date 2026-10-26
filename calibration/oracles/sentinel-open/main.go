// Bench-only parse coverage of original policy files; never a native-runtime oracle.
package main

import (
	"bytes"
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

// Diagnostic counterfactuals only: original parse results always decide admission.
// A class is attributed only when exactly one isolated change clears all errors.
// No changed bytes are written, graded, or used as independent oracle evidence.
func failureCause(src []byte) string {
	variants := map[string][]byte{}
	if bytes.Contains(src, []byte(`\"`)) {
		variants["escaped_quote"] = bytes.ReplaceAll(src, []byte(`\"`), []byte("XX"))
	}
	if bytes.Contains(src, []byte(".default")) {
		variants["keyword_selector"] = bytes.ReplaceAll(src, []byte(".default"), []byte(".default_value"))
	}
	if !bytes.HasSuffix(src, []byte("\n")) {
		variants["no_final_newline"] = append(append([]byte(nil), src...), '\n')
	}
	var matches []string
	for name, changed := range variants {
		if parse(changed).Errors == 0 {
			matches = append(matches, name)
		}
	}
	if len(matches) != 1 {
		return "unclassified_or_multiple"
	}
	return matches[0]
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
	groups := map[string]*totals{"all": {}}
	causes := map[string]int{"escaped_quote": 0, "keyword_selector": 0, "no_final_newline": 0, "unclassified_or_multiple": 0}
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
		if r.Errors != 0 {
			causes[failureCause(src)]++
		}
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
		"failure_causes": causes, "attribution": "unique isolated in-memory diagnostic change clearing errors; never used for admission",
		"classification": "open product-parser feasibility only; not independent oracle accuracy"}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
