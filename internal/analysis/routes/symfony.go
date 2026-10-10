package routes

// symfony.go — the Symfony route reader (LB-F332, wave c2 item phppy).
//
// PHP side: a #[Route] attribute on a controller class is the prefix (and
// default methods) of every #[Route] on its methods; a method's path is
// the class path concatenated with its own, as Symfony's attribute loader
// concatenates them; no methods is ANY; a class route with no method
// routes and an __invoke method routes __invoke. The handler is
// Class.method, the class as declared.
//
// YAML side, gated by path (config/routes.yaml, config/routes/**/*.yaml,
// and their when@prod sections; when@dev and when@test are not production
// routes): an entry with a path is a route (controller:, defaults:
// _controller:, template: as TemplateController.templateAction); an entry
// with a resource is an import, a Mount under its prefix (a localized
// prefix map, one Mount per locale): a directory or file resource relative
// to the YAML file, and Symfony 7's routing.controllers as the project's
// src/Controller/ directory. Symfony loads a directory recursively, and
// each class's #[Route] carries its full path, so a directory resource is
// a "/**" Target: every file under it, no folder prefix (O698). A bundle
// resource (@SomeBundle/...) and a service or glob resource name nothing
// in the repository and are skipped.

import (
	"bytes"
	stdpath "path"
	"strings"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(Reader{Framework: "symfony", Langs: []string{"php", "yaml"}, Read: readSymfony})
}

func readSymfony(p string, src []byte) Result {
	if isTestPath(p) {
		return Result{}
	}
	switch strings.ToLower(stdpath.Ext(p)) {
	case ".php":
		if !bytes.Contains(src, []byte("#[")) || !bytes.Contains(src, []byte("Route")) {
			return Result{}
		}
		return readSymfonyPHP(p, src)
	case ".yaml", ".yml":
		if !isSymfonyRoutesYAML(p) {
			return Result{}
		}
		return readSymfonyYAML(p, src)
	}
	return Result{}
}

// isSymfonyRoutesYAML reports a Symfony routing file: config/routes.yaml
// or a YAML file under config/routes/.
func isSymfonyRoutesYAML(p string) bool {
	base := stdpath.Base(p)
	dir := stdpath.Dir(p)
	if (base == "routes.yaml" || base == "routes.yml") && (dir == "config" || strings.HasSuffix(dir, "/config")) {
		return true
	}
	return strings.HasPrefix(p, "config/routes/") || strings.Contains(p, "/config/routes/")
}

// symfonyRouteAttr reports whether an attribute expression is a Route
// attribute (Route, or a qualified name ending \Route), returning its call.
func symfonyRouteAttr(a *phpNode, aliases map[string]string) (*phpNode, bool) {
	if a == nil {
		return nil, false
	}
	if a.kind == pkName {
		a = &phpNode{kind: pkCall, x: a}
	}
	if a.kind != pkCall || a.x == nil || a.x.kind != pkName {
		return nil, false
	}
	if unalias(aliases, shortPHPName(a.x.name)) != "Route" {
		return nil, false
	}
	return a, true
}

// symfonyPaths reads a Route attribute's path(s): one string, or a
// localized map's values.
func symfonyPaths(call *phpNode) []string {
	v := phpArg(call, 0, "path")
	if v == nil {
		return []string{""}
	}
	if v.kind == pkArray {
		return phpStrings(nil, v)
	}
	if s, ok := phpLiteral(v, nil); ok {
		return []string{s}
	}
	return nil
}

// symfonyMethods reads a Route attribute's methods: a list or a "GET|POST"
// string; none is ANY.
func symfonyMethods(call *phpNode) []string {
	v := phpArg(call, -1, "methods")
	if v == nil {
		return nil
	}
	var out []string
	for _, s := range phpStrings(nil, v) {
		for _, m := range strings.Split(s, "|") {
			if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
				out = append(out, m)
			}
		}
	}
	return symfonyDropHead(out)
}

// symfonyDropHead drops HEAD beside GET (Symfony matches HEAD on a GET
// route; it is not a route of its own).
func symfonyDropHead(ms []string) []string {
	hasGet := false
	for _, m := range ms {
		if m == "GET" {
			hasGet = true
		}
	}
	if !hasGet {
		return ms
	}
	out := ms[:0]
	for _, m := range ms {
		if m != "HEAD" {
			out = append(out, m)
		}
	}
	return out
}

func readSymfonyPHP(p string, src []byte) Result {
	stmts := parsePHP(src)
	aliases := phpAliases(stmts)
	var out Result
	var classes []*phpNode
	var collect func([]*phpNode)
	collect = func(ns []*phpNode) {
		for _, n := range ns {
			if n == nil {
				continue
			}
			switch n.kind {
			case pkClass:
				classes = append(classes, n)
			case pkBlock:
				collect(n.body)
			}
		}
	}
	collect(stmts)
	for _, c := range classes {
		prefixes := []string{""}
		var classMethods []string
		var classRoutes []*phpNode
		for _, a := range c.attrs {
			if call, ok := symfonyRouteAttr(a, aliases); ok {
				classRoutes = append(classRoutes, call)
			}
		}
		if len(classRoutes) > 0 {
			prefixes = symfonyPaths(classRoutes[0])
			classMethods = symfonyMethods(classRoutes[0])
		}
		emitted := 0
		var invoke *phpNode
		for _, m := range c.body {
			if m.kind != pkFunc {
				continue
			}
			if m.name == "__invoke" {
				invoke = m
			}
			for _, a := range m.attrs {
				call, ok := symfonyRouteAttr(a, aliases)
				if !ok {
					continue
				}
				methods := symfonyMethods(call)
				if len(methods) == 0 {
					methods = classMethods
				}
				for _, pre := range prefixes {
					for _, path := range symfonyPaths(call) {
						out.Routes = append(out.Routes, symfonyRoutes(methods, pre+path, c.name+"."+m.name, p, a.line)...)
						emitted++
					}
				}
			}
		}
		if emitted == 0 && invoke != nil && len(classRoutes) > 0 {
			for _, call := range classRoutes {
				for _, path := range symfonyPaths(call) {
					out.Routes = append(out.Routes, symfonyRoutes(symfonyMethods(call), path, c.name+".__invoke", p, call.line)...)
				}
			}
		}
	}
	return out
}

func symfonyRoutes(methods []string, path, handler, file string, line int) []Route {
	if path == "" {
		path = "/"
	}
	if len(methods) == 0 {
		methods = []string{"ANY"}
	}
	out := make([]Route, 0, len(methods))
	for _, m := range methods {
		out = append(out, Route{Method: m, Path: path, Handler: handler, File: file, Line: line})
	}
	return out
}

// symfonyHandler names a YAML controller value: "X::method" or
// "Ns\X::method" is X.method, a class alone is X.__invoke.
func symfonyHandler(ctl string) string {
	ctl = strings.TrimSpace(ctl)
	if ctl == "" {
		return ""
	}
	if i := strings.Index(ctl, "::"); i >= 0 {
		return shortPHPName(ctl[:i]) + "." + ctl[i+2:]
	}
	if i := strings.LastIndexByte(ctl, ':'); i >= 0 {
		return shortPHPName(ctl[:i]) + "." + ctl[i+1:]
	}
	return shortPHPName(ctl) + ".__invoke"
}

func readSymfonyYAML(p string, src []byte) Result {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil || len(doc.Content) == 0 {
		return Result{}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return Result{}
	}
	var out Result
	project := symfonyProjectRoot(p)
	var entries func(m *yaml.Node)
	entries = func(m *yaml.Node) {
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			if strings.HasPrefix(k.Value, "when@") {
				if k.Value == "when@prod" && v.Kind == yaml.MappingNode {
					entries(v)
				}
				continue
			}
			if v.Kind != yaml.MappingNode {
				continue
			}
			symfonyYAMLEntry(p, project, k, v, &out)
		}
	}
	entries(root)
	return out
}

// symfonyProjectRoot is the directory holding config/ for a routing file.
func symfonyProjectRoot(p string) string {
	if strings.HasPrefix(p, "config/") {
		return ""
	}
	if i := strings.Index(p, "/config/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func yamlMapGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// yamlStrings reads a scalar, a sequence of scalars, or a map's scalar
// values (a localized path or prefix).
func yamlStrings(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}
	case yaml.SequenceNode:
		var out []string
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				out = append(out, c.Value)
			}
		}
		return out
	case yaml.MappingNode:
		var out []string
		for i := 1; i < len(n.Content); i += 2 {
			if n.Content[i].Kind == yaml.ScalarNode {
				out = append(out, n.Content[i].Value)
			}
		}
		return out
	}
	return nil
}

func symfonyYAMLEntry(p, project string, k, v *yaml.Node, out *Result) {
	if res := yamlMapGet(v, "resource"); res != nil {
		target := ""
		if res.Kind == yaml.MappingNode {
			res = yamlMapGet(res, "path")
		}
		if res == nil || res.Kind != yaml.ScalarNode {
			return
		}
		typ := ""
		if t := yamlMapGet(v, "type"); t != nil {
			typ = t.Value
		}
		r := res.Value
		switch {
		case r == "routing.controllers":
			target = "/" + strings.TrimPrefix(stdpath.Join(project, "src/Controller")+treeSuffix, "/")
		case typ == "service", strings.HasPrefix(r, "@"), strings.ContainsAny(r, "*?{"), strings.Contains(r, "%"):
			return
		case strings.HasPrefix(r, "/"):
			return
		default:
			target = normalizeTarget(r)
			if strings.HasSuffix(r, "/") {
				target = strings.TrimSuffix(target, "/") + treeSuffix
			}
		}
		prefixes := yamlStrings(yamlMapGet(v, "prefix"))
		if len(prefixes) == 0 {
			prefixes = []string{""}
		}
		for _, pre := range prefixes {
			out.Mounts = append(out.Mounts, Mount{Prefix: pre, Target: target, File: p, Line: k.Line})
		}
		return
	}
	paths := yamlStrings(yamlMapGet(v, "path"))
	if len(paths) == 0 {
		return
	}
	ctl := ""
	if c := yamlMapGet(v, "controller"); c != nil {
		ctl = c.Value
	} else if c := yamlMapGet(yamlMapGet(v, "defaults"), "_controller"); c != nil {
		ctl = c.Value
	}
	handler := symfonyHandler(ctl)
	switch {
	case handler != "":
	case yamlMapGet(v, "template") != nil:
		handler = "TemplateController.templateAction"
	case yamlMapGet(v, "redirect_to_route") != nil:
		handler = "RedirectController.redirectAction"
	case yamlMapGet(v, "redirect_to") != nil:
		handler = "RedirectController.urlRedirectAction"
	}
	var methods []string
	for _, s := range yamlStrings(yamlMapGet(v, "methods")) {
		for _, m := range strings.Split(s, "|") {
			if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
				methods = append(methods, m)
			}
		}
	}
	methods = symfonyDropHead(methods)
	for _, path := range paths {
		out.Routes = append(out.Routes, symfonyRoutes(methods, path, handler, p, k.Line)...)
	}
}
