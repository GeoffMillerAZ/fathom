// Counts-only inspection of the measured Protobuf dialect; never oracle truth.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

type membership struct {
	Files []string `json:"tenancy_files"`
}
type reference struct {
	Path, From string
	ToPath     string `json:"to_path"`
}
type imported struct {
	Path, Module string
	ToPath       string `json:"to_path"`
}

func read(path string, into any) {
	source, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	raw, err := io.ReadAll(io.LimitReader(source, 64*1024*1024+1))
	source.Close()
	if err != nil || len(raw) > 64*1024*1024 {
		panic("artifact cap")
	}
	if err = json.Unmarshal(raw, into); err != nil {
		panic(err)
	}
}
func main() {
	subject := flag.String("subject", "", "pinned subject")
	members := flag.String("membership", "", "harness membership")
	baseline := flag.String("baseline-membership", "", "restricted harness membership, optional")
	oracle := flag.String("oracle", "", "native oracle JSON")
	reading := flag.String("reading", "", "counts-only grades from a retained harness reading")
	details := flag.Bool("details", false, "private miss detail on stderr; never committed evidence")
	flag.Parse()
	if *reading != "" {
		var r langbench.Reading
		read(*reading, &r)
		rows := []map[string]any{}
		for i, subject := range r.Subjects {
			one := r
			one.Subjects = []langbench.Subject{subject}
			langbench.Regrade(&one)
			rows = append(rows, map[string]any{"subject_ordinal": i, "grade": one.Grade, "counts": one.Counts})
		}
		if problem := json.NewEncoder(os.Stdout).Encode(rows); problem != nil {
			panic(problem)
		}
		return
	}
	var input membership
	read(*members, &input)
	if len(input.Files) > 4096 {
		panic("file cap")
	}
	allowed := map[string]bool{}
	for _, p := range input.Files {
		allowed[p] = true
	}
	old := map[string]bool{}
	if *baseline != "" {
		var before membership
		read(*baseline, &before)
		for _, p := range before.Files {
			old[p] = true
		}
	}
	excluded := map[string]bool{}
	if *baseline != "" {
		for p := range allowed {
			if !old[p] {
				excluded[p] = true
			}
		}
	}
	ts, err := extractor.NewTSExtractor("protobuf")
	if err != nil {
		panic(err)
	}
	defer ts.Close()
	var files []core.SourceFile
	var nodes []core.SymbolNode
	texts := map[string]string{}
	extraDefs := 0
	for _, p := range input.Files {
		if filepath.IsAbs(p) || filepath.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
			panic("membership path")
		}
		raw, err := os.ReadFile(filepath.Join(*subject, p))
		if err != nil {
			panic(err)
		}
		if len(raw) > 2*1024*1024 {
			panic("source cap")
		}
		ns, err := ts.Extract("langbench", p, raw)
		if err != nil {
			panic(err)
		}
		nodes = append(nodes, ns...)
		files = append(files, core.SourceFile{Path: p, Content: raw})
		texts[p] = string(raw)
		if excluded[p] {
			extraDefs += len(ns)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	edges, report := fusion.ResolveFileEdges(ctx, "protobuf", *subject, files, nodes)
	if report.Skipped || report.Faults > 0 {
		panic("resolver refused")
	}
	byKey := map[string]core.SymbolNode{}
	for _, n := range nodes {
		byKey[n.Key()] = n
	}
	found := map[[3]string]bool{}
	for _, e := range edges {
		from, ok := byKey[e.FromSymbol]
		to, okTo := byKey[e.ToSymbol]
		if ok && okTo && to.FilePath != e.FromFile {
			found[[3]string{e.FromFile, langbench.BareName(from.Name), to.FilePath}] = true
		}
	}
	extraPairs := 0
	for p := range found {
		if excluded[p[0]] {
			extraPairs++
		}
	}
	var native struct {
		References []reference `json:"cross_references"`
		Imports    []imported  `json:"imports"`
	}
	read(*oracle, &native)
	direct := map[string][]string{}
	public := map[string][]string{}
	for _, i := range native.Imports {
		direct[i.Path] = append(direct[i.Path], i.ToPath)
		pattern := `(?m)^\s*import\s+public\s+"` + regexp.QuoteMeta(i.Module) + `"`
		if regexp.MustCompile(pattern).MatchString(texts[i.Path]) {
			public[i.Path] = append(public[i.Path], i.ToPath)
		}
	}
	var reaches func(string, string, map[string]bool) bool
	reaches = func(from, to string, seen map[string]bool) bool {
		if from == to {
			return true
		}
		if seen[from] {
			return false
		}
		seen[from] = true
		for _, next := range public[from] {
			if reaches(next, to, seen) {
				return true
			}
		}
		return false
	}
	truth := map[[3]string]bool{}
	causes := map[string]int{}
	matched := 0
	for _, r := range native.References {
		if allowed[r.Path] && allowed[r.ToPath] {
			truth[[3]string{r.Path, langbench.BareName(r.From), r.ToPath}] = true
		}
	}
	for p := range truth {
		if found[p] {
			matched++
			continue
		}
		if *details {
			fmt.Fprintln(os.Stderr, p)
		}
		cause := "reference_not_bound_despite_direct_import"
		viaPublic := false
		isDirect := false
		for _, to := range direct[p[0]] {
			if to == p[2] {
				isDirect = true
			}
			if to != p[2] && reaches(to, p[2], map[string]bool{}) {
				viaPublic = true
			}
		}
		if viaPublic && !isDirect {
			cause = "transitive_public_import_not_followed"
		} else if !isDirect {
			cause = "target_not_directly_imported_other"
		}
		if isDirect {
			declared := regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][A-Za-z0-9_.]*)\s*;`).FindStringSubmatch(texts[p[2]])
			if len(declared) == 2 && !strings.HasSuffix(filepath.ToSlash(filepath.Dir(p[2])), strings.ReplaceAll(declared[1], ".", "/")) && strings.Contains(texts[p[0]], declared[1]+".") {
				cause = "qualified_package_differs_from_directory"
			}
		}
		causes[cause]++
	}
	result := map[string]any{"measured_files": len(input.Files), "fathom_defs": len(nodes), "xref_truth": len(truth), "xref_found": matched, "xref_fathom": len(found), "miss_causes": causes, "excluded_files_measured": len(excluded), "excluded_fathom_defs": extraDefs, "excluded_xref_fathom": extraPairs, "graded": false}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode failed")
		os.Exit(1)
	}
}
