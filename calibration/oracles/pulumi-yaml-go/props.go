package main

// The -props-out report (LL-M5): every attribute leaf of every resource of the programs listed in -props-list, as
// pulumi-yaml's own template parse (pulumiyaml.LoadYAMLBytes, the parse the YAML language host runs) sees it. Nothing
// here reads the YAML by a rule written in this file, and no fathom package is imported.
//
// A leaf has a path (a mapping key joins with '.', a list element takes '[*]', a properties value that is not a
// mapping is one leaf with the empty path) and an oracle class:
//
//	constant    a null, boolean, number or string with no interpolation
//	references  a symbol or interpolation naming a resource or configuration key, or a builtin holding one
//	empty       a builtin that holds none, and ${pulumi.*}
//	unresolved  a symbol rooted in nothing the program declares, or a variable cycle
//
// A symbol rooted in a declared variable takes the class of the variable's value, and fn::secret takes the class of its
// argument. defaultable is true when every name the leaf references is a configuration key with a scalar default, read
// whole (no property access).

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-yaml/pkg/pulumiyaml"
	yast "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/ast"
)

type propLeaf struct {
	Class       string `json:"class"`
	Defaultable bool   `json:"defaultable"`
}

type propResource struct {
	Type   string                `json:"type"`
	Leaves map[string][]propLeaf `json:"leaves"`
}

type propProgram struct {
	OK        bool                    `json:"ok"`
	Error     string                  `json:"error,omitempty"`
	Resources map[string]propResource `json:"resources,omitempty"`
}

type propCtx struct {
	resources map[string]bool
	vars      map[string]yast.Expr
	cfgs      map[string]*yast.ConfigParamDecl
	following map[string]bool
}

func combine(parts []propLeaf) propLeaf {
	var refs, empty, unres, allDef = false, false, false, true
	for _, p := range parts {
		switch p.Class {
		case "unresolved":
			unres = true
		case "references":
			refs = true
			if !p.Defaultable {
				allDef = false
			}
		case "empty":
			empty = true
		}
	}
	switch {
	case unres:
		return propLeaf{Class: "unresolved"}
	case refs:
		return propLeaf{Class: "references", Defaultable: allDef && !empty}
	case empty:
		return propLeaf{Class: "empty"}
	}
	return propLeaf{Class: "constant"}
}

func (c *propCtx) symbol(pa *yast.PropertyAccess) propLeaf {
	root := pa.RootName()
	switch {
	case root == "pulumi":
		return propLeaf{Class: "empty"}
	case c.resources[root]:
		return propLeaf{Class: "references"}
	case c.cfgs[root] != nil:
		d := c.cfgs[root].Default
		scalar := false
		switch d.(type) {
		case *yast.StringExpr, *yast.NumberExpr, *yast.BooleanExpr:
			scalar = true
		}
		return propLeaf{Class: "references", Defaultable: scalar && len(pa.Accessors) == 1}
	case c.vars[root] != nil:
		if c.following[root] {
			return propLeaf{Class: "unresolved"}
		}
		c.following[root] = true
		l := c.class(c.vars[root])
		delete(c.following, root)
		return l
	}
	return propLeaf{Class: "unresolved"}
}

// holdsSymbol reports whether a builtin's arguments hold a symbol rooted anywhere but pulumi.
func (c *propCtx) holdsSymbol(e yast.Expr) bool {
	switch x := e.(type) {
	case *yast.SymbolExpr:
		return x.Property.RootName() != "pulumi"
	case *yast.InterpolateExpr:
		for _, p := range x.Parts {
			if p.Value != nil && p.Value.RootName() != "pulumi" {
				return true
			}
		}
	case *yast.ListExpr:
		for _, el := range x.Elements {
			if c.holdsSymbol(el) {
				return true
			}
		}
	case *yast.ObjectExpr:
		for _, en := range x.Entries {
			if c.holdsSymbol(en.Key) || c.holdsSymbol(en.Value) {
				return true
			}
		}
	case yast.BuiltinExpr:
		return c.holdsSymbol(x.Args())
	}
	return false
}

func (c *propCtx) class(e yast.Expr) propLeaf {
	switch x := e.(type) {
	case nil, *yast.NullExpr, *yast.BooleanExpr, *yast.NumberExpr, *yast.StringExpr:
		return propLeaf{Class: "constant"}
	case *yast.SymbolExpr:
		return c.symbol(x.Property)
	case *yast.InterpolateExpr:
		var parts []propLeaf
		for _, p := range x.Parts {
			if p.Value != nil {
				parts = append(parts, c.symbol(p.Value))
			}
		}
		return combine(parts)
	case *yast.ListExpr:
		var parts []propLeaf
		for _, el := range x.Elements {
			parts = append(parts, c.class(el))
		}
		return combine(parts)
	case *yast.ObjectExpr:
		var parts []propLeaf
		for _, en := range x.Entries {
			parts = append(parts, c.class(en.Value))
		}
		return combine(parts)
	case *yast.SecretExpr:
		return c.class(x.Value)
	case yast.BuiltinExpr:
		if c.holdsSymbol(x.Args()) {
			return propLeaf{Class: "references"}
		}
		return propLeaf{Class: "empty"}
	}
	return propLeaf{Class: "unresolved"}
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func (c *propCtx) walk(e yast.Expr, path string, out map[string][]propLeaf) {
	switch x := e.(type) {
	case *yast.ObjectExpr:
		if len(x.Entries) == 0 {
			out[path] = append(out[path], propLeaf{Class: "constant"})
			return
		}
		for _, en := range x.Entries {
			k, ok := en.Key.(*yast.StringExpr)
			if !ok {
				out[path] = append(out[path], propLeaf{Class: "unresolved"})
				continue
			}
			c.walk(en.Value, join(path, k.Value), out)
		}
	case *yast.ListExpr:
		if len(x.Elements) == 0 {
			out[path] = append(out[path], propLeaf{Class: "constant"})
			return
		}
		for _, el := range x.Elements {
			c.walk(el, path+"[*]", out)
		}
	default:
		out[path] = append(out[path], c.class(e))
	}
}

func propsOf(rel string, b []byte) propProgram {
	tmpl, diags, err := pulumiyaml.LoadYAMLBytes(rel, b)
	if err != nil || tmpl == nil || diags.HasErrors() {
		msg := fmt.Sprint(err)
		if err == nil {
			msg = firstLine(strings.TrimSpace(diags.Error()))
		}
		return propProgram{Error: msg}
	}
	c := &propCtx{resources: map[string]bool{}, vars: map[string]yast.Expr{}, cfgs: map[string]*yast.ConfigParamDecl{}, following: map[string]bool{}}
	for _, e := range tmpl.Resources.Entries {
		c.resources[e.Key.Value] = true
	}
	for _, e := range tmpl.Variables.Entries {
		c.vars[e.Key.Value] = e.Value
	}
	// pulumi-yaml reads both spellings of the section, and a `config` entry replaces a `configuration` entry of the same name.
	for _, sec := range []*yast.ConfigMapDecl{&tmpl.Configuration, &tmpl.Config} {
		for _, e := range sec.Entries {
			c.cfgs[e.Key.Value] = e.Value
		}
	}
	p := propProgram{OK: true, Resources: map[string]propResource{}}
	for _, e := range tmpl.Resources.Entries {
		r := e.Value
		if r == nil || r.Type == nil || r.Type.Value == "" {
			continue
		}
		leaves := map[string][]propLeaf{}
		switch {
		case r.Properties.PropertyMap != nil:
			if len(r.Properties.PropertyMap.Entries) > 0 {
				obj := &yast.ObjectExpr{}
				for _, en := range r.Properties.PropertyMap.Entries {
					obj.Entries = append(obj.Entries, en.Object())
				}
				c.walk(obj, "", leaves)
			}
		case r.Properties.Expr != nil:
			leaves[""] = append(leaves[""], c.class(r.Properties.Expr))
		}
		p.Resources[e.Key.Value] = propResource{Type: r.Type.Value, Leaves: leaves}
	}
	return p
}

// propsMain reads a list of "key<TAB>file" lines and writes the report.
func propsMain(list, out string) int {
	b, err := os.ReadFile(list)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pulumi-yaml-oracle:", err)
		return 1
	}
	progs := map[string]propProgram{}
	var keys []string
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, f, ok := strings.Cut(ln, "\t")
		if !ok {
			continue
		}
		keys = append(keys, k)
		src, err := os.ReadFile(f)
		if err != nil {
			progs[k] = propProgram{Error: err.Error()}
			continue
		}
		progs[k] = propsOf(k, src)
	}
	sort.Strings(keys)
	rep := struct {
		Oracle   string                 `json:"oracle"`
		Programs map[string]propProgram `json:"programs"`
	}{"pulumi-yaml " + modVersion("github.com/pulumi/pulumi-yaml") + " pulumiyaml.LoadYAMLBytes", progs}
	data, err := json.MarshalIndent(rep, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "pulumi-yaml-oracle:", err)
		return 1
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "pulumi-yaml-oracle:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "pulumi-yaml-oracle: %d programs\n", len(keys))
	return 0
}
