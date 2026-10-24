// Command fathom-cue-oracle is the language-breadth harness's oracle for fathom's
// own CUE configuration formats (LB-Q-fathom-concerns, LB-Q-fathom-layer-packs,
// tier A: the cue binary, design §1.3 and §18.3). It writes indexbench
// lang-oracle's tool-oracle JSON:
//
//	go run . -format concerns|layer-packs -subject <checkout> -cue <cue v0.16.1> \
//	  -cue-oracle <built calibration/oracles/cue> -cache <CUE_CACHE_DIR> -out <file>
//
// Both words come from the cue binary, run as separate processes, never fathom's
// loader and never cue/load in process:
//
//   - what exists: `cue export --out json <file>` per file of the format. A file
//     the binary rejects is named on stderr and measured as nothing. For
//     concerns it is the keys of `concerns` (including those a comprehension
//     generates, which only evaluation names); for a layer.cue the string at
//     `name`; for a file under .fathom/rules/ whether `rules` exists (named by
//     the file's stem);
//   - where it is: calibration/oracles/cue (cue lsp documentSymbol) run on the
//     same subject. A written concern is the depth-1 field of its name inside a
//     `concerns` field; a generated one is the dynamic field the language
//     server lists at depth 1 inside it; a pack's name and a rule file's rules
//     are the top-level fields `name` and `rules`.
//
// References (the harness scores them as L2 beside a tool oracle) are the cue
// oracle's own calls (textDocument/definition at every identifier a lexical
// scan proposes), kept for the format's files. The format's entities carry no
// references of their own, so they belong to the CUE fields around them. Bench-only; never a product dependency.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}

type pair struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

type output struct {
	Tool            string            `json:"tool"`
	Version         string            `json:"version"`
	Tier            string            `json:"tier"`
	Definitions     []def             `json:"definitions"`
	Calls           []pair            `json:"calls,omitempty"`
	References      []pair            `json:"references"`
	CrossReferences []json.RawMessage `json:"cross_references"`
	Imports         []json.RawMessage `json:"imports"`
}

func main() {
	format := flag.String("format", "", "concerns or layer-packs (required)")
	subject := flag.String("subject", "", "subject checkout (required)")
	out := flag.String("out", "", "file to write the oracle JSON to (default stdout)")
	cueBin := flag.String("cue", "cue", "the cue binary (v0.16.1)")
	cueOracle := flag.String("cue-oracle", "", "the built calibration/oracles/cue program (required)")
	cache := flag.String("cache", "", "CUE_CACHE_DIR outside the subject")
	flag.Parse()
	if (*format != "concerns" && *format != "layer-packs") || *subject == "" || *cueOracle == "" {
		fmt.Fprintln(os.Stderr, "usage: fathom-cue-oracle -format concerns|layer-packs -subject <checkout> -cue <cue> -cue-oracle <bin> [-cache <dir>] [-out <file>]")
		os.Exit(2)
	}
	root, err := filepath.Abs(*subject)
	if err != nil {
		fail(err)
	}
	files := formatFiles(root, *format)
	var base output
	args := []string{"-subject", root, "-cue", *cueBin}
	if *cache != "" {
		args = append(args, "-cache", *cache)
	}
	if b, err := runCmd("", *cueOracle, args...); err != nil {
		fail(fmt.Errorf("cue oracle: %w", err))
	} else if err := json.Unmarshal(b, &base); err != nil {
		fail(fmt.Errorf("cue oracle output: %w", err))
	}
	res := output{Tool: "cue export + cue lsp (" + *format + ")", Version: base.Version, Tier: "A",
		Definitions: []def{}, References: []pair{}, CrossReferences: []json.RawMessage{}, Imports: []json.RawMessage{}}
	keep := map[string]bool{}
	for _, rel := range files {
		names, err := exported(root, *cueBin, rel, *format)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fathom-cue-oracle: %s: rejected by cue export: %v\n", rel, err)
			continue
		}
		keep[rel] = true
		var fileDefs []def
		for _, d := range base.Definitions {
			if d.Path == rel {
				fileDefs = append(fileDefs, d)
			}
		}
		ds, missing := place(rel, *format, names, fileDefs)
		res.Definitions = append(res.Definitions, ds...)
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "fathom-cue-oracle: %s: %s has no range in the language server's symbols; left out\n", rel, m)
		}
	}
	for _, c := range base.Calls {
		if keep[c.Path] {
			res.References = append(res.References, c)
		}
	}
	fmt.Fprintf(os.Stderr, "fathom-cue-oracle: %d of %d files, %d definitions, %d references\n", len(keep), len(files), len(res.Definitions), len(res.References))
	buf, _ := json.MarshalIndent(res, "", " ")
	buf = append(buf, '\n')
	if *out == "" {
		os.Stdout.Write(buf)
		return
	}
	if err := os.WriteFile(*out, buf, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fathom-cue-oracle:", err)
	os.Exit(1)
}

func runCmd(dir, bin string, args ...string) ([]byte, error) {
	c := exec.Command(bin, args...)
	c.Dir = dir
	var so, se bytes.Buffer
	c.Stdout, c.Stderr = &so, &se
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(se.String()))
	}
	return so.Bytes(), nil
}

// formatFiles are the subject's files of the format, slash paths, sorted.
func formatFiles(root, format string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if matches(rel, format) {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// matches is the format's file set: the manifests' globs.
func matches(rel, format string) bool {
	segs := strings.Split(rel, "/")
	n := len(segs)
	switch format {
	case "concerns":
		return n >= 2 && segs[n-2] == ".fathom" && segs[n-1] == "concerns.cue"
	case "layer-packs":
		if n >= 4 && segs[n-4] == ".fathom" && segs[n-3] == "layers" && segs[n-1] == "layer.cue" {
			return true
		}
		return n >= 3 && segs[n-3] == ".fathom" && segs[n-2] == "rules" && strings.HasSuffix(segs[n-1], ".cue")
	}
	return false
}

// exported asks the cue binary what the file declares: concern ids, the
// pack's name, or the rule file's stem (when `rules` exists).
func exported(root, cueBin, rel, format string) ([]string, error) {
	b, err := runCmd(root, cueBin, "export", "--out", "json", "./"+rel)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	switch {
	case format == "concerns":
		var m map[string]json.RawMessage
		if raw, ok := doc["concerns"]; ok {
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, err
			}
		}
		ids := make([]string, 0, len(m))
		for id := range m {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids, nil
	case path.Base(rel) == "layer.cue":
		var name string
		if raw, ok := doc["name"]; ok && json.Unmarshal(raw, &name) == nil && name != "" {
			return []string{name}, nil
		}
		return nil, nil
	default:
		if _, ok := doc["rules"]; ok {
			return []string{strings.TrimSuffix(path.Base(rel), ".cue")}, nil
		}
		return nil, nil
	}
}

// place gives each exported name its range from the language server's symbols
// of the file. Names with no range come back as missing.
func place(rel, format string, names []string, defs []def) (out []def, missing []string) {
	// The server lists a parent before its children, so an enclosing symbol is
	// an earlier one whose lines hold the later one's (equal lines included: a
	// one-line `a: {b: 1}` holds b).
	encloses := func(i, j int) bool { return i < j && defs[i].Line <= defs[j].Line && defs[j].End <= defs[i].End }
	topLevel := func(j int) bool {
		for i := range defs {
			if encloses(i, j) {
				return false
			}
		}
		return true
	}
	switch {
	case format == "concerns":
		var containers []int
		for j, d := range defs {
			if d.Name == "concerns" && d.Kind == "field" && topLevel(j) {
				containers = append(containers, j)
			}
		}
		// depth 1: the symbol's nearest enclosing symbol is a concerns field.
		depth1 := func(j int) bool {
			for k := j - 1; k >= 0; k-- {
				if encloses(k, j) {
					for _, c := range containers {
						if c == k {
							return true
						}
					}
					return false
				}
			}
			return false
		}
		static := map[string]def{}
		var dynamic []def
		for j, d := range defs {
			if !depth1(j) {
				continue
			}
			switch d.Kind {
			case "field", "definition":
				if _, ok := static[d.Name]; !ok {
					static[d.Name] = d
				}
			case "dynamic":
				dynamic = append(dynamic, d)
			}
		}
		for _, id := range names {
			switch d, ok := static[id]; {
			case ok:
				out = append(out, def{Path: rel, Name: id, Line: d.Line, End: d.End, Kind: "rule"})
			case len(dynamic) > 0:
				out = append(out, def{Path: rel, Name: id, Line: dynamic[0].Line, End: dynamic[0].End, Kind: "rule"})
			default:
				missing = append(missing, id)
			}
		}
	default:
		field, kind := "rules", "rule"
		if path.Base(rel) == "layer.cue" {
			field, kind = "name", "policy"
		}
		for _, n := range names {
			placed := false
			for j, d := range defs {
				if d.Name == field && d.Kind == "field" && topLevel(j) {
					out = append(out, def{Path: rel, Name: n, Line: d.Line, End: d.End, Kind: kind})
					placed = true
					break
				}
			}
			if !placed {
				missing = append(missing, n)
			}
		}
	}
	return out, missing
}
