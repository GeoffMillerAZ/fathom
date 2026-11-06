package iacfacts

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// IaCOutput is one `output` block of a module, read by the same file rules and parser ReadTerraformModule uses
// (tfModuleFileNames, parseTerraform), so attribute checks and output checks agree about which files make a module.
type IaCOutput struct {
	Name string
	Span Span
	// Marked is true when the output is sensitive: `sensitive = true`, any `sensitive` expression the reader cannot
	// tell from true (as a variable's flag reads), or a value wrapped in sensitive().
	Marked bool
	// Reads are the static traversals of the value, rendered as the attribute reader renders a reference
	// (aws_db_instance.main.password). A traversal inside nonsensitive() or sensitive() is not listed.
	Reads []string
	// Unread says why the output cannot be judged: a syntax error touches it.
	Unread string
}

// IaCOutputSet is what ReadTerraformOutputs found.
type IaCOutputSet struct {
	Outputs []IaCOutput
	// Skipped names inputs the read did not look at (JSON and override files), so absence is never silent.
	Skipped []string
}

// ReadTerraformOutputs reads the output blocks of every .tf and .tofu file directly inside dir of fsys. JSON-syntax
// and override files are not read for outputs and are named in Skipped. The bounds of ReadTerraformModule hold.
func ReadTerraformOutputs(fsys fs.FS, dir string) (*IaCOutputSet, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("iacfacts: read module directory: %w", err)
	}
	names := tfModuleFileNames(entries)
	if len(names) > MaxModuleFiles {
		return nil, limitErr(dir, fmt.Sprintf("%d files, over %d", len(names), MaxModuleFiles))
	}
	link := map[string]bool{}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			link[e.Name()] = true
		}
	}
	set := &IaCOutputSet{}
	total := 0
	for _, n := range names {
		p := path.Join(dir, n)
		if isTFJSON(n) || isOverrideFile(n) {
			set.Skipped = append(set.Skipped, p)
			continue
		}
		if link[n] {
			if st, err := fs.Stat(fsys, p); err != nil || !st.Mode().IsRegular() {
				set.Skipped = append(set.Skipped, p)
				continue
			}
		}
		f, err := fsys.Open(p)
		if err != nil {
			if link[n] {
				set.Skipped = append(set.Skipped, p)
				continue
			}
			return nil, fmt.Errorf("iacfacts: open module file: %w", err)
		}
		src, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("iacfacts: read module file: %w", err)
		}
		if total += len(src); total > MaxModuleBytes {
			return nil, limitErr(dir, fmt.Sprintf("module is over %d bytes", MaxModuleBytes))
		}
		pf, err := parseTerraform(p, src)
		if err != nil {
			return nil, err
		}
		for _, b := range pf.body.Blocks {
			if b.Type != "output" || len(b.Labels) != 1 {
				continue
			}
			set.Outputs = append(set.Outputs, readOutput(pf, b))
		}
	}
	sort.SliceStable(set.Outputs, func(i, j int) bool { return set.Outputs[i].Span.less(set.Outputs[j].Span) })
	return set, nil
}

func readOutput(pf *tfParsed, b *hclsyntax.Block) IaCOutput {
	o := IaCOutput{Name: b.Labels[0], Span: spanOf(b.Range())}
	if a, ok := b.Body.Attributes["sensitive"]; ok {
		if pf.suspect(a.SrcRange) {
			o.Unread = "the sensitive flag sits in a region with a syntax error"
		}
		o.Marked = !isFalseLiteral(a.Expr)
	}
	va, ok := b.Body.Attributes["value"]
	if !ok {
		return o
	}
	if pf.suspect(va.SrcRange) {
		o.Unread = "the value sits in a region with a syntax error"
		return o
	}
	w := &outputWalker{}
	_ = hclsyntax.Walk(va.Expr, w)
	o.Reads = sortedRefs(w.reads)
	if w.wrapped {
		o.Marked = true
	}
	return o
}

// outputWalker collects the traversals of an expression that are not inside nonsensitive() or sensitive(). A value
// wrapped whole in sensitive() marks the output.
type outputWalker struct {
	depth   int
	reads   []string
	wrapped bool
	seen    bool
}

func (w *outputWalker) Enter(n hclsyntax.Node) hcl.Diagnostics {
	if !w.seen {
		w.seen = true
		if fc, ok := n.(*hclsyntax.FunctionCallExpr); ok && fc.Name == "sensitive" {
			w.wrapped = true
		}
	}
	switch x := n.(type) {
	case *hclsyntax.FunctionCallExpr:
		if x.Name == "nonsensitive" || x.Name == "sensitive" {
			w.depth++
		}
	case *hclsyntax.ScopeTraversalExpr:
		if w.depth == 0 {
			w.reads = append(w.reads, renderReadPath(x.Traversal))
		}
	}
	return nil
}

func (w *outputWalker) Exit(n hclsyntax.Node) hcl.Diagnostics {
	if x, ok := n.(*hclsyntax.FunctionCallExpr); ok && (x.Name == "nonsensitive" || x.Name == "sensitive") {
		w.depth--
	}
	return nil
}

// renderReadPath spells a traversal the way a value reads it: the root, then each attribute, with an index step
// dropped (aws_db_instance.main[0].password reads the password of the instance) and a splat ending the spelling.
func renderReadPath(t hcl.Traversal) string {
	var sb strings.Builder
	for _, s := range t {
		switch x := s.(type) {
		case hcl.TraverseRoot:
			sb.WriteString(x.Name)
		case hcl.TraverseAttr:
			sb.WriteByte('.')
			sb.WriteString(x.Name)
		case hcl.TraverseIndex:
			continue
		default:
			return sb.String()
		}
	}
	return sb.String()
}
