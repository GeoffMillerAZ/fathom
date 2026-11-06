package iacfacts

// cfn_replace.go reads the replacement facts of a CloudFormation registry schema against a change (IF-C4) and
// against a resource as it stands (IF-C3). The facts are the schema's own createOnlyProperties and
// conditionalCreateOnlyProperties: the contract states them, and no change set confirms them.

import (
	"fmt"
	"sort"
	"strings"
)

// Replacement classes of a CloudFormation property, as Catalog.Contract spells them for a Terraform attribute.
const (
	CFNReplace     = "replace"
	CFNConditional = "conditional"
)

// Finding kinds of CheckCloudFormationChange.
const (
	CFNForcesReplacement = "forces-replacement"
	CFNMayReplace        = "may-replace"
)

// CFNPointerPath spells a registry schema pointer the way a resource read spells an attribute path:
// /properties/Tags/*/Key reads Tags[*].Key. It reports false for a pointer that does not name a property.
func CFNPointerPath(ptr string) (string, bool) {
	rest, ok := strings.CutPrefix(ptr, "/properties/")
	if !ok || rest == "" {
		return "", false
	}
	unescape := strings.NewReplacer("~1", "/", "~0", "~")
	var sb strings.Builder
	for i, seg := range strings.Split(rest, "/") {
		switch {
		case seg == "":
			return "", false
		case seg == "*" && i == 0:
			return "", false
		case seg == "*":
			sb.WriteString("[*]")
		default:
			if i > 0 {
				sb.WriteByte('.')
			}
			sb.WriteString(unescape.Replace(seg))
		}
	}
	return sb.String(), true
}

// cfnReplPtr is one replacement fact of a schema: the class, the pointer as the schema states it, the reader path
// it spells and the schema field it was read from.
type cfnReplPtr struct{ class, pointer, path, field string }

// replacementPointers lists a schema's create-only and conditional create-only properties. A property both lists
// state reads as create-only, the stronger claim.
func (b cfnBlob) replacementPointers() []cfnReplPtr {
	var out []cfnReplPtr
	seen := map[string]bool{}
	for _, l := range []struct {
		class, field string
		ptrs         []string
	}{
		{CFNReplace, "createOnlyProperties", b.CreateOnly},
		{CFNConditional, "conditionalCreateOnlyProperties", b.CondCreateOnly},
	} {
		for _, p := range l.ptrs {
			path, ok := CFNPointerPath(p)
			if !ok || seen[path] {
				continue
			}
			seen[path] = true
			out = append(out, cfnReplPtr{class: l.class, pointer: p, path: path, field: l.field})
		}
	}
	return out
}

func hasPathPrefix(s, p string) bool {
	return len(s) > len(p) && strings.HasPrefix(s, p) && (s[len(p)] == '.' || s[len(p)] == '[')
}

// pathRelated reports whether two reader paths are the same property, or one holds the other. An empty path is a
// Properties value the reader could not walk and relates to nothing.
func pathRelated(q, c string) bool {
	return q != "" && c != "" && (q == c || hasPathPrefix(q, c) || hasPathPrefix(c, q))
}

// CFNReplaceFact is a replacement fact of the schema about a property a resource writes.
type CFNReplaceFact struct {
	// Class is CFNReplace or CFNConditional. Property is the top-level property name.
	Class, Property string
	// Pointer is the schema pointer, Path the schema field it was read from and Digest the sha256 of the blob.
	Pointer, Path, Digest string
	// Span is where the resource writes the property.
	Span Span
}

// CFNReplacementFacts lists the properties a resource writes that its schema says force replacement or may when
// they change. The binding is CheckCloudFormationUse's, and an unbound one lists nothing.
func CFNReplacementFacts(catalogs []IaCCatalogInfo, region string, r Resource) ([]CFNReplaceFact, CFNBinding) {
	u, b, digest, ok := resolveCFN(catalogs, region, r)
	if !ok {
		return nil, u.Binding
	}
	var out []CFNReplaceFact
	for _, p := range b.replacementPointers() {
		var at *Span
		for i := range r.Attrs {
			if pathRelated(r.Attrs[i].Path, p.path) && (at == nil || r.Attrs[i].Span.less(*at)) {
				at = &r.Attrs[i].Span
			}
		}
		if at != nil {
			out = append(out, CFNReplaceFact{Class: p.class, Property: topProperty(p.path), Pointer: p.pointer, Path: p.field, Digest: digest, Span: *at})
		}
	}
	rank := map[string]int{CFNReplace: 0, CFNConditional: 1}
	sort.Slice(out, func(i, j int) bool {
		a, c := out[i], out[j]
		if rank[a.Class] != rank[c.Class] {
			return rank[a.Class] < rank[c.Class]
		}
		if a.Property != c.Property {
			return a.Property < c.Property
		}
		return a.Pointer < c.Pointer
	})
	return out, u.Binding
}

// CFNChangeFinding is a changed property that the schema says replaces the resource.
type CFNChangeFinding struct {
	// Kind is CFNForcesReplacement or CFNMayReplace. Property is the top-level property name and Attr the changed
	// attribute path under the pointer. Change is "added", "removed" or "changed".
	Kind, Property, Attr, Change string
	// Span is where the head writes the attribute, or the head resource when it removed it.
	Span Span
	// Pointer, Path and Digest cite the schema fact as CFNFinding does.
	Pointer, Path, Digest string
}

// CFNChange is the result of CheckCloudFormationChange for one resource.
type CFNChange struct {
	Name, Type string
	Binding    CFNBinding
	// Findings are sorted by span, then kind, property and pointer.
	Findings []CFNChangeFinding
	// Notes say what the check could not see.
	Notes []string
}

// srcLines is a template split into lines, for reading the text a span covers.
type srcLines []string

func newSrcLines(src []byte) srcLines { return strings.Split(string(src), "\n") }

// text is the source a span covers with its whitespace collapsed, so a re-indented value reads as itself. ok is false
// for a span the source does not hold.
func (l srcLines) text(sp Span) (string, bool) {
	if sp.StartLine < 1 || sp.EndLine < sp.StartLine || sp.EndLine > len(l) {
		return "", false
	}
	var sb strings.Builder
	for ln := sp.StartLine; ln <= sp.EndLine; ln++ {
		r := []rune(l[ln-1])
		from, to := 0, len(r)
		if ln == sp.StartLine {
			from = min(max(sp.StartCol-1, 0), len(r))
		}
		if ln == sp.EndLine {
			to = min(max(sp.EndCol-1, 0), len(r))
		}
		if from > to {
			return "", false
		}
		sb.WriteString(string(r[from:to]))
		sb.WriteByte('\n')
	}
	return strings.Join(strings.Fields(sb.String()), " "), true
}

// attrSig is what a read says of one attribute: its class, the references it reads and the text its span covers.
// The text is compared and never kept: a read made without values still tells a changed literal from an unchanged
// one, and no value leaves this function.
func attrSig(a Attr, lines srcLines) string {
	t, _ := lines.text(a.Span)
	return string(a.Class) + "\x00" + strings.Join(a.Refs, ",") + "\x00" + t
}

// attrSigs groups a resource's attributes by path into one comparable string each.
func attrSigs(r Resource, src []byte) (sigs map[string]string, spans map[string]Span, opaque bool) {
	lines := newSrcLines(src)
	by := map[string][]string{}
	spans = map[string]Span{}
	for _, a := range r.Attrs {
		if a.Path == "" || strings.HasPrefix(topProperty(a.Path), "Fn::") {
			opaque = true
			continue
		}
		by[a.Path] = append(by[a.Path], attrSig(a, lines))
		if sp, ok := spans[a.Path]; !ok || a.Span.less(sp) {
			spans[a.Path] = a.Span
		}
	}
	sigs = map[string]string{}
	for p, s := range by {
		sort.Strings(s)
		sigs[p] = strings.Join(s, "\x01")
	}
	return sigs, spans, opaque
}

// CheckCloudFormationChange compares a resource as the base wrote it with the head, and reports each changed
// property the head's schema says forces replacement (create-only) or may (conditional create-only). baseSrc and
// headSrc are the templates the resources were read from: a property is changed when its class, its references or the
// source text of its span differ, so no value is read or kept. A resource whose Properties either side
// wrote through an intrinsic the reader could not walk proves no change, and a Type that changed is a different
// resource: both are said in Notes. The binding is CheckCloudFormationUse's. The schema states the replacement, so
// an answer says the contract states it, never that a change set confirms it.
func CheckCloudFormationChange(catalogs []IaCCatalogInfo, region string, base, head Resource, baseSrc, headSrc []byte) CFNChange {
	u, b, digest, ok := resolveCFN(catalogs, region, head)
	c := CFNChange{Name: head.Name, Type: head.Type, Binding: u.Binding}
	if !ok {
		return c
	}
	if base.Tool != ToolCloudFormation || base.Type != head.Type {
		c.Notes = append(c.Notes, fmt.Sprintf("%s was %s and is %s: a different type is a different resource, and no property is compared", head.Name, base.Type, head.Type))
		return c
	}
	bs, _, bopaque := attrSigs(base, baseSrc)
	hs, hspans, hopaque := attrSigs(head, headSrc)
	if bopaque || hopaque {
		side := "head"
		if bopaque {
			side = "base"
		}
		c.Notes = append(c.Notes, fmt.Sprintf("a property of %s could not be walked in the %s (an intrinsic function, an alias or a merge), so no change is claimed", head.Name, side))
		return c
	}
	changed := map[string]string{}
	for p, s := range hs {
		switch o, had := bs[p]; {
		case !had:
			changed[p] = "added"
		case o != s:
			changed[p] = "changed"
		}
	}
	for p := range bs {
		if _, has := hs[p]; !has {
			changed[p] = "removed"
		}
	}
	paths := make([]string, 0, len(changed))
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range b.replacementPointers() {
		for _, q := range paths {
			if !pathRelated(q, p.path) {
				continue
			}
			sp, found := hspans[q]
			if !found {
				sp = head.Span
			}
			kind := CFNMayReplace
			if p.class == CFNReplace {
				kind = CFNForcesReplacement
			}
			c.Findings = append(c.Findings, CFNChangeFinding{Kind: kind, Property: topProperty(p.path), Attr: q, Change: changed[q], Span: sp, Pointer: p.pointer, Path: p.field, Digest: digest})
			break
		}
	}
	sort.Slice(c.Findings, func(i, j int) bool {
		a, d := c.Findings[i], c.Findings[j]
		if a.Span != d.Span {
			return a.Span.less(d.Span)
		}
		if a.Kind != d.Kind {
			return a.Kind < d.Kind
		}
		if a.Property != d.Property {
			return a.Property < d.Property
		}
		return a.Pointer < d.Pointer
	})
	return c
}
