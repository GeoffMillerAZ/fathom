// grammar-screen reproduces the two Swift parse-error predicates with the
// committed WASM grammar. This is a product-parser screen, not an oracle or
// accuracy reading. Run from the repository root; outputs go under workspace/.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

type subject struct {
	Name        string   `json:"name"`
	Files       int      `json:"files"`
	Bytes       int      `json:"bytes"`
	ErrorNode   []string `json:"files_with_error_node"`
	MissingOnly []string `json:"files_with_missing_only"`
	Either      []string `json:"files_with_error_or_missing"`
	Faults      []string `json:"faulting_files"`
}

func main() {
	alamofire := flag.String("alamofire", "workspace/calibration/alamofire", "Alamofire subject root at the Swift corpus pin")
	nio := flag.String("swift-nio", "workspace/calibration/swift-nio", "swift-nio subject root at the Swift corpus pin")
	out := flag.String("out", "workspace/artifacts/swift-error-measures.json", "per-file result JSON")
	flag.Parse()

	p, e := tsengine.NewParser(context.Background(), "swift", "tree_sitter_swift")
	if e != nil {
		panic(e)
	}
	defer p.Close()
	errors, e := p.CompileQuery("(ERROR) @error")
	if e != nil {
		panic(e)
	}
	defer errors.Close()
	either, e := p.CompileQuery("[(ERROR) (MISSING)] @error")
	if e != nil {
		panic(e)
	}
	defer either.Close()
	var results []subject
	for _, pair := range [][2]string{{"Alamofire/Alamofire", *alamofire}, {"apple/swift-nio", *nio}} {
		r := subject{Name: pair[0], ErrorNode: []string{}, MissingOnly: []string{}, Either: []string{}, Faults: []string{}}
		e = filepath.WalkDir(pair[1], func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".swift" {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(pair[1], path)
			if e != nil {
				return e
			}
			r.Files++
			r.Bytes += len(b)
			tree, e := p.Parse(b)
			if e != nil {
				r.Faults = append(r.Faults, rel)
				return nil
			}
			defer tree.Close()
			em, e := errors.Run(tree)
			if e != nil {
				return e
			}
			am, e := either.Run(tree)
			if e != nil {
				return e
			}
			// ERROR includes files that also contain MISSING. MISSING-only
			// is the union minus that set, not every file with MISSING.
			if len(em) > 0 {
				r.ErrorNode = append(r.ErrorNode, rel)
			}
			if len(am) > 0 {
				r.Either = append(r.Either, rel)
				if len(em) == 0 {
					r.MissingOnly = append(r.MissingOnly, rel)
				}
			}
			return nil
		})
		if e != nil {
			panic(e)
		}
		fmt.Printf("%s files=%d bytes=%d ERROR=%d MISSING-only=%d ERROR-or-MISSING=%d faults=%d\n", r.Name, r.Files, r.Bytes, len(r.ErrorNode), len(r.MissingOnly), len(r.Either), len(r.Faults))
		results = append(results, r)
	}
	b, e := json.MarshalIndent(results, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.MkdirAll(filepath.Dir(*out), 0755); e != nil {
		panic(e)
	}
	if e = os.WriteFile(*out, append(b, '\n'), 0644); e != nil {
		panic(e)
	}
}
