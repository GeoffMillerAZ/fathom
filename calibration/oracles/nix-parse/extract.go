package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The oracle's output shape is `indexbench lang-oracle -tool-oracle`'s (calibration/oracles/README.md).

type defOut struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	End  int    `json:"end"`
	Kind string `json:"kind"`
}
type pairOut struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}
type importOut struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
	Module string `json:"module,omitempty"`
}

type fileResult struct {
	defs    []defOut
	calls   []pairOut
	imports []importOut
	count   map[string]int
	notes   []string
	exports map[string]bool // the root set's keys
	cands   []cand
}

var (
	optionHeads  = map[string]bool{"mkOption": true, "mkEnableOption": true, "mkPackageOption": true}
	packageHeads = map[string]bool{
		"callPackage": true, "mkDerivation": true, "buildPythonPackage": true, "buildPythonApplication": true,
		"buildGoModule": true, "buildRustPackage": true, "buildNpmPackage": true, "writeShellApplication": true, "writeShellScriptBin": true,
	}
)

// headName is the name an application calls: the identifier, or the last attribute of a selected name.
func headName(h *node) string {
	switch h.kind {
	case nVar:
		return h.name
	case nSelect:
		return h.name
	}
	return ""
}

// classify is the kind of a binding whose key is the whole attribute path (`a = value`).
func classify(v *node, scope string) string {
	if v != nil {
		switch v.kind {
		case nLambda:
			return "function"
		case nApp:
			if h := headName(v.head); optionHeads[h] {
				return "option"
			} else if packageHeads[h] {
				return "package"
			}
		}
	}
	if scope == "let" {
		return "variable"
	}
	return "attribute"
}

// callees collects the names applied inside n, once each. The printer writes an operator on numbers and a
// search path as an application of a double-underscore builtin (`x * 2` is `__mul x 2`, `<nixpkgs>` is
// `__findFile`): those are not written calls and are left out.
func callees(n *node, into map[string]bool) {
	if n == nil {
		return
	}
	if n.kind == nApp {
		if h := headName(n.head); h != "" && !strings.HasPrefix(h, "__") {
			into[h] = true
		}
	}
	callees(n.head, into)
	callees(n.body, into)
	for _, k := range n.kids {
		callees(k, into)
	}
	for _, k := range n.order {
		callees(n.binds[k], into)
	}
}

// rootScopes peels the parsed tree the way readRoot peels the tokens and returns the root let and set
// bindings and the parameter names it passed.
func rootScopes(root *node) (lets, sets map[string]*node, params map[string]bool) {
	params = map[string]bool{}
	n := root
	wraps := 0
	for n != nil && wraps <= maxWrappers {
		switch n.kind {
		case nLambda:
			if wraps == maxWrappers {
				return
			}
			for _, f := range n.formals {
				params[f] = true
			}
			if n.arg != "" {
				params[n.arg] = true
			}
			n = n.body
			wraps++
		case nWith:
			if wraps == maxWrappers {
				return
			}
			n = n.body
			wraps++
		case nLet:
			if wraps == maxWrappers {
				sets = mergeInto(sets, n.binds)
				return
			}
			lets = mergeInto(lets, n.binds)
			n = n.body
			wraps++
		case nSet:
			sets = mergeInto(sets, n.binds)
			return
		default:
			return
		}
	}
	return
}

func mergeInto(dst, src map[string]*node) map[string]*node {
	if dst == nil {
		dst = map[string]*node{}
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

var flakePath = regexp.MustCompile(`^(?:path:)?(\.\.?/[^"]*|\.\.?)$`)

// extract reads one file: src is its text, printed the reference parse's output, rel its subject-relative path.
func extract(subject, rel string, src []byte, printed string) (*fileResult, error) {
	res := &fileResult{count: map[string]int{}}
	tree, err := parseAST(printed)
	if err != nil {
		res.count["ast_unreadable"]++
		return res, err
	}
	toks := lex(src)
	info := readRoot(toks)
	lets, sets, params := rootScopes(tree)
	res.exports = map[string]bool{}
	for k := range sets {
		res.exports[k] = true
	}
	xs := &xr{subject: subject}
	xs.root(tree, scope{})
	res.cands = xs.cands

	known := map[string]bool{}
	for _, s := range info.sites {
		var v *node
		var ok bool
		if s.scope == "let" {
			v, ok = lets[s.name]
		} else {
			v, ok = sets[s.name]
		}
		if !ok {
			res.count["sites_unattested"]++
			res.notes = append(res.notes, fmt.Sprintf("unattested %s:%d %s", rel, s.line, s.name))
			continue
		}
		kind := "attribute"
		if s.scope == "let" {
			kind = "variable"
		}
		switch {
		case s.name == "options":
			kind = "option"
		case s.pathLen == 1:
			kind = classify(v, s.scope)
		}
		res.defs = append(res.defs, defOut{rel, s.name, s.line, s.end, kind})
		known[s.name] = true
		cs := map[string]bool{}
		callees(v, cs)
		for c := range cs {
			res.calls = append(res.calls, pairOut{rel, s.name, c})
		}
	}
	for _, p := range info.params {
		if !params[p.name] {
			res.count["params_unattested"]++
			continue
		}
		res.defs = append(res.defs, defOut{rel, p.name, p.line, p.line, "param"})
	}

	// imports: path literals that name a Nix file of the subject, and a flake input's local path
	astPaths, astStrs := map[string]bool{}, map[string]bool{}
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		if n.kind == nPath {
			astPaths[path.Clean(n.name)] = true
		}
		if n.kind == nStr {
			astStrs[n.name] = true
		}
		walk(n.head)
		walk(n.body)
		for _, k := range n.kids {
			walk(k)
		}
		for _, k := range n.order {
			walk(n.binds[k])
		}
	}
	walk(tree)
	dir := path.Dir(rel)
	seen := map[[2]string]bool{}
	add := func(line int, module, to string) {
		k := [2]string{module, to}
		if to == "" || to == rel || seen[k] && false {
			return
		}
		res.imports = append(res.imports, importOut{rel, line, to, module})
	}
	for i, t := range toks {
		switch {
		case t.kind == tPath && !t.interp && (strings.HasPrefix(t.text, "./") || strings.HasPrefix(t.text, "../")):
			abs := path.Clean(path.Join(subject, dir, t.text))
			if !astPaths[abs] {
				res.count["paths_unattested"]++
				continue
			}
			add(t.line, t.text, resolveNix(subject, path.Join(dir, t.text), false))
		case t.kind == tString && !t.interp && !t.indent && i >= 2 && isP(toks[i-1], "=") && toks[i-2].kind == tIdent && toks[i-2].text == "url":
			m := flakePath.FindStringSubmatch(t.text)
			if m == nil {
				continue
			}
			if !astStrs[t.text] {
				res.count["paths_unattested"]++
				continue
			}
			if strings.HasPrefix(t.text, "path:") {
				res.count["flake_path_scheme"]++
			}
			add(t.line, t.text, resolveNix(subject, path.Join(dir, m[1]), true))
		}
	}
	return res, nil
}

// resolveNix is the file a Nix path names: the file itself, or a directory's default.nix (a flake input
// directory's flake.nix). "" when it names no file of the subject.
func resolveNix(subject, rel string, flake bool) string {
	rel = path.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	fi, err := os.Stat(filepath.Join(subject, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	if !fi.IsDir() {
		if strings.HasSuffix(rel, ".nix") && !flake {
			return rel
		}
		return ""
	}
	names := []string{"default.nix"}
	if flake {
		names = []string{"flake.nix"}
	}
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(subject, filepath.FromSlash(rel), n)); err == nil {
			return path.Join(rel, n)
		}
	}
	return ""
}

func sortResult(r *fileResult) {
	slices.SortFunc(r.defs, func(a, b defOut) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(r.calls, func(a, b pairOut) int {
		return strings.Compare(a.Path+"\x00"+a.From+"\x00"+a.To, b.Path+"\x00"+b.From+"\x00"+b.To)
	})
	r.calls = slices.CompactFunc(r.calls, func(a, b pairOut) bool { return a == b })
	slices.SortFunc(r.imports, func(a, b importOut) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.ToPath, b.ToPath)
	})
}
