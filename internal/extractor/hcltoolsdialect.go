package extractor

// hcltoolsdialect.go — the HashiCorp HCL tool configs native dialect
// (LB-Q-hcl-tools, docs/design/language-breadth.md §13, engine "native":
// hclsyntax, the parser the terraform dialect already uses). One extractor
// serves three tools, told apart by the file's name (the manifest's
// basenames, extensions and globs; a plain .hcl whose top-level block is
// `job` is a Nomad jobspec by content sniff, never by its name):
//
// Packer (*.pkr.hcl, *.pkrvars.hcl):
//
//   - source "t" "n" (source.t.n), build (build, or build.<name> by its name
//     argument), variable "n" (var.n), locals (local.k), data "t" "n"
//     (data.t.n), and inside a build each provisioner "t" and post-processor
//     "t" (<build>.provisioner.t, <build>.post-processor.t); a .pkrvars.hcl
//     file's keys (PackerVarValue). L2: a build's sources (`sources = [...]`
//     or source "t.n" blocks) call source.t.n; var., local. and data.
//     references call theirs.
//
// Nomad (*.nomad, *.nomad.hcl, a .hcl by sniff):
//
//   - job "j" (j), group "g" (j.g), task "t" (j.g.t), service (<parent>.service.<name>),
//     variable "n" (var.n), locals (local.k). L2: var. and local. references.
//     A task's config { image = "x" } is an image-ref cross-file reference
//     (edge "image"), resolved through the image-tag index
//     (fusion/imageref.go).
//
// docker-bake (docker-bake.hcl, docker-bake.override.hcl, docker-bake*.hcl):
//
//   - target "t" (t), group "g" (group.g), variable "v" (var.v), function "f"
//     (func.f). L2: a group's targets and a target's inherits (and a
//     contexts value "target:t") call the named target; a variable named
//     bare in an expression calls var.v, a call of a declared function
//     func.f. The image tags a bake target builds are NOT read here: the
//     image-ref index reads them through fusion.BakeBuilds, once.

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

const (
	kindPackerSource      core.SourceKind = "PackerSource"
	kindPackerBuild       core.SourceKind = "PackerBuild"
	kindPackerVariable    core.SourceKind = "PackerVariable"
	kindPackerLocal       core.SourceKind = "PackerLocal"
	kindPackerData        core.SourceKind = "PackerData"
	kindPackerProvisioner core.SourceKind = "PackerProvisioner"
	kindPackerPostProc    core.SourceKind = "PackerPostProcessor"
	kindPackerVarValue    core.SourceKind = "PackerVarValue"

	kindNomadJob      core.SourceKind = "NomadJob"
	kindNomadGroup    core.SourceKind = "NomadGroup"
	kindNomadTask     core.SourceKind = "NomadTask"
	kindNomadService  core.SourceKind = "NomadService"
	kindNomadVariable core.SourceKind = "NomadVariable"
	kindNomadLocal    core.SourceKind = "NomadLocal"

	kindBakeTarget   core.SourceKind = "BakeTarget"
	kindBakeGroup    core.SourceKind = "BakeGroup"
	kindBakeVariable core.SourceKind = "BakeVariable"
	kindBakeFunction core.SourceKind = "BakeFunction"

	// edgeImage is a Nomad task's image reference (strategy image-ref).
	edgeImage = "image"
)

// HCLToolsExtractor implements Extractor and DocResulter for the HashiCorp
// HCL tool configs (Packer, Nomad, docker-bake).
type HCLToolsExtractor struct{}

var (
	_ Extractor   = (*HCLToolsExtractor)(nil)
	_ DocResulter = (*HCLToolsExtractor)(nil)
)

// NewHCLToolsExtractor returns the hcl-tools dialect's extractor.
func NewHCLToolsExtractor() *HCLToolsExtractor { return &HCLToolsExtractor{} }

// Extract implements Extractor.
func (x *HCLToolsExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	r, err := x.ExtractDoc(service, relPath, src)
	return r.Nodes, err
}

// hclToolOf is which tool a path belongs to: "packer", "bake" or "nomad" (any
// other .hcl a dialect sniff sent here).
func hclToolOf(relPath string) string {
	b := path.Base(relPath)
	switch {
	case strings.HasSuffix(b, ".pkr.hcl") || strings.HasSuffix(b, ".pkrvars.hcl"):
		return "packer"
	case strings.HasPrefix(b, "docker-bake") && strings.HasSuffix(b, ".hcl"):
		return "bake"
	}
	return "nomad"
}

// ExtractDoc implements DocResulter.
func (x *HCLToolsExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	body, err := hclParseBody(relPath, src)
	if err != nil {
		return DocResult{}, err
	}
	if body == nil {
		return DocResult{}, nil
	}
	w := newHCLWalk(service, relPath, src)
	switch hclToolOf(relPath) {
	case "packer":
		if strings.HasSuffix(relPath, ".pkrvars.hcl") {
			for _, a := range hclSortedAttrs(body) {
				w.add(a.Name, kindPackerVarValue, a.Name+" = …", a.SrcRange.Start.Line, a.SrcRange.End.Line, nil)
			}
		} else {
			x.packer(w, body)
		}
	case "bake":
		x.bake(w, body)
	default:
		x.nomad(w, body)
	}
	return w.result(), nil
}

// ── Packer ───────────────────────────────────────────────────────────────────

// packerKey is the entity a Packer traversal reads: var.x, local.x,
// data.t.n, source.t.n.
func packerKey(t hcl.Traversal) string {
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return ""
	}
	attrs := func(n int) string {
		if len(t) < n {
			return ""
		}
		parts := []string{root.Name}
		for _, s := range t[1:n] {
			a, ok := s.(hcl.TraverseAttr)
			if !ok {
				return ""
			}
			parts = append(parts, a.Name)
		}
		return strings.Join(parts, ".")
	}
	switch root.Name {
	case "var", "local":
		return attrs(2)
	case "data", "source":
		return attrs(3)
	}
	return ""
}

func (x *HCLToolsExtractor) packer(w *hclWalk, body *hclsyntax.Body) {
	for _, b := range body.Blocks {
		if b.Body == nil {
			continue
		}
		line, end := b.TypeRange.Start.Line, b.Range().End.Line
		switch b.Type {
		case "source":
			if len(b.Labels) >= 2 {
				w.add("source."+b.Labels[0]+"."+b.Labels[1], kindPackerSource, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, packerKey, nil))
			}
		case "variable":
			if len(b.Labels) >= 1 {
				w.add("var."+b.Labels[0], kindPackerVariable, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, packerKey, nil))
			}
		case "data":
			if len(b.Labels) >= 2 {
				w.add("data."+b.Labels[0]+"."+b.Labels[1], kindPackerData, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, packerKey, nil))
			}
		case "locals":
			for _, a := range hclSortedAttrs(b.Body) {
				w.add("local."+a.Name, kindPackerLocal, "locals { "+a.Name+" }", a.SrcRange.Start.Line, a.SrcRange.End.Line, hclExprRefs(a.Expr, packerKey, nil))
			}
		case "build":
			name := "build"
			if a, ok := b.Body.Attributes["name"]; ok {
				if s, ok := hclLiteralString(a.Expr); ok && s != "" {
					name = "build." + s
				}
			}
			refs := hclBodyRefs(b.Body, map[string]bool{"provisioner": true, "post-processor": true, "post-processors": true}, map[string]bool{"sources": true}, packerKey, nil)
			refs = append(refs, packerSources(b.Body)...)
			name = w.add(name, kindPackerBuild, "build", line, end, refs)
			for _, c := range b.Body.Blocks {
				x.packerStep(w, name, c)
			}
		}
	}
}

// packerSources are a build's source references: `sources = [...]` (quoted
// "source.t.n" strings or bare traversals) and source "t.n" blocks.
func packerSources(b *hclsyntax.Body) []hclRef {
	var out []hclRef
	if a, ok := b.Attributes["sources"]; ok {
		for _, it := range hclTupleItems(a.Expr) {
			line := it.Range().Start.Line
			if s, ok := hclLiteralString(it); ok {
				out = append(out, hclRef{key: packerSourceKey(s), line: line})
			} else if t, d := hcl.AbsTraversalForExpr(it); !d.HasErrors() {
				if k := packerKey(t); k != "" {
					out = append(out, hclRef{key: k, line: line})
				}
			}
		}
	}
	for _, c := range b.Blocks {
		if c.Type == "source" && len(c.Labels) >= 1 {
			out = append(out, hclRef{key: packerSourceKey(c.Labels[0]), line: c.TypeRange.Start.Line})
		}
	}
	return out
}

// packerSourceKey is "source.t.n" for "t.n" and "source.t.n".
func packerSourceKey(s string) string {
	if strings.HasPrefix(s, "source.") {
		return s
	}
	return "source." + s
}

// packerStep adds a build's provisioner or post-processor (the latter also
// inside a post-processors chain).
func (x *HCLToolsExtractor) packerStep(w *hclWalk, build string, c *hclsyntax.Block) {
	if c.Body == nil {
		return
	}
	line, end := c.TypeRange.Start.Line, c.Range().End.Line
	switch c.Type {
	case "provisioner":
		if len(c.Labels) >= 1 {
			w.add(build+".provisioner."+c.Labels[0], kindPackerProvisioner, tgSig(c), line, end, hclBodyRefs(c.Body, nil, nil, packerKey, nil))
		}
	case "post-processor":
		if len(c.Labels) >= 1 {
			w.add(build+".post-processor."+c.Labels[0], kindPackerPostProc, tgSig(c), line, end, hclBodyRefs(c.Body, nil, nil, packerKey, nil))
		}
	case "post-processors":
		for _, d := range c.Body.Blocks {
			x.packerStep(w, build, d)
		}
	}
}

// ── Nomad ────────────────────────────────────────────────────────────────────

// nomadKey is the entity a Nomad traversal reads: var.x and local.x.
func nomadKey(t hcl.Traversal) string {
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok || len(t) < 2 || (root.Name != "var" && root.Name != "local") {
		return ""
	}
	if a, ok := t[1].(hcl.TraverseAttr); ok {
		return root.Name + "." + a.Name
	}
	return ""
}

var nomadNested = map[string]bool{"group": true, "task": true, "service": true}

func (x *HCLToolsExtractor) nomad(w *hclWalk, body *hclsyntax.Body) {
	for _, b := range body.Blocks {
		if b.Body == nil {
			continue
		}
		line, end := b.TypeRange.Start.Line, b.Range().End.Line
		switch b.Type {
		case "variable":
			if len(b.Labels) >= 1 {
				w.add("var."+b.Labels[0], kindNomadVariable, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, nomadKey, nil))
			}
		case "locals":
			for _, a := range hclSortedAttrs(b.Body) {
				w.add("local."+a.Name, kindNomadLocal, "locals { "+a.Name+" }", a.SrcRange.Start.Line, a.SrcRange.End.Line, hclExprRefs(a.Expr, nomadKey, nil))
			}
		case "job":
			if len(b.Labels) < 1 {
				continue
			}
			job := w.add(b.Labels[0], kindNomadJob, tgSig(b), line, end, hclBodyRefs(b.Body, nomadNested, nil, nomadKey, nil))
			x.nomadChildren(w, job, b.Body)
		}
	}
}

// nomadChildren adds the groups, tasks and services below a block of qualified
// name parent.
func (x *HCLToolsExtractor) nomadChildren(w *hclWalk, parent string, body *hclsyntax.Body) {
	for _, c := range body.Blocks {
		if c.Body == nil {
			continue
		}
		line, end := c.TypeRange.Start.Line, c.Range().End.Line
		switch c.Type {
		case "group":
			if len(c.Labels) >= 1 {
				g := w.add(parent+"."+c.Labels[0], kindNomadGroup, tgSig(c), line, end, hclBodyRefs(c.Body, nomadNested, nil, nomadKey, nil))
				x.nomadChildren(w, g, c.Body)
			}
		case "task":
			if len(c.Labels) >= 1 {
				t := w.add(parent+"."+c.Labels[0], kindNomadTask, tgSig(c), line, end, hclBodyRefs(c.Body, nomadNested, nil, nomadKey, nil))
				x.nomadImage(w, t, c.Body)
				x.nomadChildren(w, t, c.Body)
			}
		case "service":
			n := "service"
			if a, ok := c.Body.Attributes["name"]; ok {
				if s, ok := hclLiteralString(a.Expr); ok && s != "" {
					n = s
				}
			}
			w.add(parent+".service."+n, kindNomadService, "service "+n, line, end, hclBodyRefs(c.Body, nil, nil, nomadKey, nil))
		}
	}
}

// nomadImage records a task's config { image = "…" } as an image-ref
// reference; an image that is not a literal (a variable, an interpolation)
// names nothing the index can match and is left out.
func (x *HCLToolsExtractor) nomadImage(w *hclWalk, task string, body *hclsyntax.Body) {
	for _, c := range body.Blocks {
		if c.Type != "config" || c.Body == nil {
			continue
		}
		a, ok := c.Body.Attributes["image"]
		if !ok {
			continue
		}
		if s, ok := hclLiteralString(a.Expr); ok && s != "" {
			w.ref(DocCrossFileRef{From: task, Edge: edgeImage, Raw: s, Target: s, Strategy: "image-ref", Line: a.SrcRange.Start.Line})
		}
	}
}

// ── docker-bake ──────────────────────────────────────────────────────────────

func (x *HCLToolsExtractor) bake(w *hclWalk, body *hclsyntax.Body) {
	vars := map[string]bool{}
	funcs := map[string]bool{}
	targets := map[string]bool{}
	groups := map[string]bool{}
	for _, b := range body.Blocks {
		if len(b.Labels) < 1 {
			continue
		}
		switch b.Type {
		case "group":
			groups[b.Labels[0]] = true
		case "variable":
			vars[b.Labels[0]] = true
		case "function":
			funcs[b.Labels[0]] = true
		case "target":
			targets[b.Labels[0]] = true
		}
	}
	key := func(t hcl.Traversal) string {
		root, ok := t[0].(hcl.TraverseRoot)
		if !ok {
			return ""
		}
		if root.Name == "target" && len(t) > 1 {
			if a, ok := t[1].(hcl.TraverseAttr); ok && targets[a.Name] {
				return a.Name
			}
			return ""
		}
		if vars[root.Name] {
			return "var." + root.Name
		}
		return ""
	}
	fn := func(name string) string {
		if funcs[name] {
			return "func." + name
		}
		return ""
	}
	// names are the declared targets and groups a list of strings names (a
	// group's targets may be groups).
	names := func(x hclsyntax.Expression) []hclRef {
		var out []hclRef
		for _, it := range hclTupleItems(x) {
			s, ok := hclLiteralString(it)
			switch {
			case ok && targets[s]:
				out = append(out, hclRef{key: s, line: it.Range().Start.Line})
			case ok && groups[s]:
				out = append(out, hclRef{key: "group." + s, line: it.Range().Start.Line})
			}
		}
		return out
	}
	for _, b := range body.Blocks {
		if b.Body == nil || len(b.Labels) < 1 {
			continue
		}
		line, end := b.TypeRange.Start.Line, b.Range().End.Line
		switch b.Type {
		case "variable":
			w.add("var."+b.Labels[0], kindBakeVariable, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, key, fn))
		case "function":
			w.add("func."+b.Labels[0], kindBakeFunction, tgSig(b), line, end, hclBodyRefs(b.Body, nil, nil, key, fn))
		case "group":
			refs := hclBodyRefs(b.Body, nil, map[string]bool{"targets": true}, key, fn)
			if a, ok := b.Body.Attributes["targets"]; ok {
				refs = append(refs, names(a.Expr)...)
			}
			w.add("group."+b.Labels[0], kindBakeGroup, tgSig(b), line, end, refs)
		case "target":
			refs := hclBodyRefs(b.Body, nil, map[string]bool{"inherits": true}, key, fn)
			if a, ok := b.Body.Attributes["inherits"]; ok {
				refs = append(refs, names(a.Expr)...)
			}
			if a, ok := b.Body.Attributes["contexts"]; ok {
				if o, ok := a.Expr.(*hclsyntax.ObjectConsExpr); ok {
					for _, it := range o.Items {
						if s, ok := hclLiteralString(it.ValueExpr); ok && strings.HasPrefix(s, "target:") && targets[strings.TrimPrefix(s, "target:")] {
							refs = append(refs, hclRef{key: strings.TrimPrefix(s, "target:"), line: it.ValueExpr.Range().Start.Line})
						}
					}
				}
			}
			w.add(b.Labels[0], kindBakeTarget, tgSig(b), line, end, refs)
		}
	}
}

// HCLToolDialectExtractor is the extractor of a native dialect this file or
// terragruntdialect.go defines, by dialect id; ok is false for any other id.
func HCLToolDialectExtractor(id string) (Extractor, bool) {
	switch id {
	case "terragrunt":
		return NewTerragruntExtractor(), true
	case "hcl-tools":
		return NewHCLToolsExtractor(), true
	}
	return nil, false
}
