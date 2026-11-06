package iacfacts

import (
	"fmt"
	"sort"
	"strings"
)

// Finding kinds of CheckAttributeUse (design section 3, facts 1 to 4).
const (
	FindingUnknownAttribute = "unknown-attribute"
	FindingRequiredUnset    = "required-unset"
	FindingDeprecated       = "deprecated"
	FindingSensitiveOutput  = "sensitive-output"
)

// Where a finding was read.
const (
	ViaAttribute = "attribute"
	ViaTraversal = "traversal"
	ViaOutput    = "output"
)

// MaxCheckFindings is the most findings one CheckAttributeUse call returns. Past it the rest are counted in a
// disclosure.
const MaxCheckFindings = 500

// IaCFinding is one fact the contract states about what a block wrote. Every finding cites its catalog, release,
// blob digest and JSON path.
type IaCFinding struct {
	Kind string `json:"kind"`
	// Block is the address of the block: aws_s3_bucket.logs, data.aws_ami.base or output.name.
	Block string `json:"block"`
	// Path is the attribute path as the file wrote it, the required path that is missing, or the traversal.
	Path string `json:"path"`
	Via  string `json:"via"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	// Binding is the status of the contract the fact was read from: locked or constraint-matched.
	Binding string     `json:"binding"`
	Message string     `json:"message"`
	Fact    IaCFactRef `json:"fact"`
}

// IaCWrittenAttr is one attribute a block wrote, with the flags its contract gives it.
type IaCWrittenAttr struct {
	Path  string `json:"path"`
	Class Class  `json:"class"`
	Line  int    `json:"line,omitempty"`
	// Known is false for an attribute the contract does not have.
	Known bool     `json:"known"`
	Flags []string `json:"flags,omitempty"`
	// Fact cites the contract entry, set only when Known.
	Fact *IaCFactRef `json:"fact,omitempty"`
}

// IaCCheckInput is what CheckAttributeUse reads. Resource is the block to check (nil to check only Outputs).
// Binding and Contract are the resource's: Contract is nil unless the binding is locked or constraint-matched, and
// then the call reports no resource facts. Resolve names another block of the module by mode, type and name and
// returns its binding and contract (nil contract when it has none); a nil Resolve skips traversals and outputs.
type IaCCheckInput struct {
	Resource *Resource
	Binding  IaCBinding
	Contract *IaCContract
	Resolve  func(mode, typ, name string) (IaCBinding, *IaCContract)
	Outputs  []IaCOutput
}

// IaCCheck is the result. Written lists every attribute the block wrote, in path order.
type IaCCheck struct {
	Block       string
	Status      string
	Version     string
	Findings    []IaCFinding
	Written     []IaCWrittenAttr
	Disclosures []string
}

// CheckAttributeUse walks a block's attributes and static traversals against its bound contract and an output's
// reads against the contracts of what they name: an unknown attribute, a required argument unset, a deprecated
// attribute and a sensitive value in an unmarked output (facts 1 to 4).
//
// The attribute reads are the ones ReadTerraformModule makes (and pkg/lang/iac wraps), never a second parser. A
// fact is stated only from a bound contract. Required arguments are checked per block level, at the top and in each
// nested block the file writes, and a level that holds an attribute the reader gave class unknown is not checked at
// all, with a disclosure: an unknown attribute may be absent. A nested block the file writes empty leaves no trace
// in the read, so a required nested block, and a required argument of an empty block, are not reported. A traversal
// is checked at its first attribute and, for an output, down the nested attributes of an object; a traversal the
// reader cut at an index or splat names the whole object and is not judged.
func CheckAttributeUse(in IaCCheckInput) IaCCheck {
	ck := &attrChecker{in: in, seen: map[string]bool{}}
	out := IaCCheck{Status: in.Binding.Status, Version: in.Binding.Version}
	if r := in.Resource; r != nil {
		out.Block = blockAddress(r)
		ck.block = out.Block
		if in.Contract == nil {
			ck.disclose(fmt.Sprintf("no facts for %s: its binding is %s", out.Block, orUnbound(in.Binding.Status)))
		} else if r.Mode == ModeModule {
			ck.disclose(out.Block + " is a module call, which has no provider contract")
		} else {
			ck.checkResource(r, in.Contract)
		}
		ck.checkTraversals(r)
	}
	ck.checkOutputs()
	out.Findings = ck.findings
	out.Written = ck.written
	out.Disclosures = ck.notes
	sort.SliceStable(out.Findings, func(i, j int) bool {
		a, b := out.Findings[i], out.Findings[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Path < b.Path
	})
	if len(out.Findings) > MaxCheckFindings {
		out.Disclosures = append(out.Disclosures, fmt.Sprintf("%d findings past the %d one call returns were dropped", len(out.Findings)-MaxCheckFindings, MaxCheckFindings))
		out.Findings = out.Findings[:MaxCheckFindings]
	}
	return out
}

func orUnbound(s string) string {
	if s == "" {
		return BindUnbound
	}
	return s
}

func blockAddress(r *Resource) string {
	switch r.Mode {
	case ModeData:
		return "data." + r.Type + "." + r.Name
	case ModeEphemeral:
		return "ephemeral." + r.Type + "." + r.Name
	case ModeModule:
		return "module." + r.Name
	}
	return r.Type + "." + r.Name
}

type attrChecker struct {
	in       IaCCheckInput
	block    string
	findings []IaCFinding
	written  []IaCWrittenAttr
	notes    []string
	seen     map[string]bool
}

func (ck *attrChecker) disclose(s string) {
	for _, n := range ck.notes {
		if n == s {
			return
		}
	}
	ck.notes = append(ck.notes, s)
}

// add records a finding once per kind, block and path.
func (ck *attrChecker) add(f IaCFinding) {
	key := f.Kind + "\x00" + f.Block + "\x00" + f.Path + "\x00" + f.Via
	if ck.seen[key] {
		return
	}
	ck.seen[key] = true
	ck.findings = append(ck.findings, f)
}

// metaPath reports an attribute the language owns and no provider schema holds.
func metaPath(path string) bool {
	top := path
	if i := strings.IndexByte(top, '.'); i >= 0 {
		top = top[:i]
	}
	return top == "depends_on" || top == "lifecycle"
}

func (ck *attrChecker) checkResource(r *Resource, c *IaCContract) {
	status := ck.in.Binding.Status
	written := make(map[string]bool, len(r.Attrs))
	unknownLevels := map[string]bool{}
	rootUnknown := false
	exprs := 0
	for _, a := range r.Attrs {
		if a.Path == "" {
			rootUnknown = true
			continue
		}
		if metaPath(a.Path) {
			continue
		}
		written[a.Path] = true
		switch a.Class {
		case ClassUnknown:
			unknownLevels[levelOf(a.Path)] = true
		case ClassExpression:
			exprs++
		}
	}
	for _, a := range r.Attrs {
		if a.Path == "" || metaPath(a.Path) {
			continue
		}
		w := IaCWrittenAttr{Path: a.Path, Class: a.Class, Line: a.Span.StartLine}
		blk := c.Block
		var names []string
		for i, seg := range strings.Split(a.Path, ".") {
			last := i == strings.Count(a.Path, ".")
			isBlock := strings.HasSuffix(seg, "[*]")
			name := strings.TrimSuffix(seg, "[*]")
			if isBlock {
				nb := blk.Blocks[name]
				if nb == nil {
					if at := blk.Attrs[name]; at != nil && at.BlockForm {
						// A list or set of objects is also written as blocks. The attribute is known, and a
						// member is judged against the object type the attribute states, as tofu does.
						ref := c.ref(append(blockJSONPath(names), "attributes", name)...)
						if at.Deprecated {
							ck.add(IaCFinding{Kind: FindingDeprecated, Block: ck.block, Path: pathUpTo(a.Path, i), Via: ViaAttribute, File: a.Span.File, Line: a.Span.StartLine,
								Binding: status, Message: fmt.Sprintf("%q is deprecated in %s %s", pathUpTo(a.Path, i), c.Type, c.Fact.Version), Fact: ref})
						}
						segs := strings.Split(a.Path, ".")
						if last || at.Members == nil {
							w.Known = true
							w.Flags = at.Names()
							w.Fact = &ref
							break
						}
						member := strings.TrimSuffix(segs[i+1], "[*]")
						typeRef := c.ref(append(blockJSONPath(names), "attributes", name, "type")...)
						opt, ok := at.Members[member]
						if !ok {
							ck.add(IaCFinding{Kind: FindingUnknownAttribute, Block: ck.block, Path: a.Path, Via: ViaAttribute, File: a.Span.File, Line: a.Span.StartLine,
								Binding: status, Message: fmt.Sprintf("%q is not a member of %q in %s %s: its type has no such member", a.Path, pathUpTo(a.Path, i), c.Type, c.Fact.Version), Fact: typeRef})
							break
						}
						w.Known = true
						if opt {
							w.Flags = []string{"optional"}
						}
						w.Fact = &typeRef
						break
					}
					ck.unknown(r, a, c, names, name, blk.Attrs[name] != nil)
					break
				}
				names = append(names, name)
				blk = nb.Block
				if blk.Deprecated {
					ck.add(IaCFinding{Kind: FindingDeprecated, Block: ck.block, Path: pathUpTo(a.Path, i), Via: ViaAttribute, File: a.Span.File, Line: a.Span.StartLine,
						Binding: status, Message: fmt.Sprintf("the block %q is deprecated in %s %s", name, c.Type, c.Fact.Version), Fact: c.ref(append(blockJSONPath(names), "deprecated")...)})
				}
				if last {
					w.Known = true
					ref := c.ref(blockJSONPath(names)...)
					w.Fact = &ref
				}
				continue
			}
			at := blk.Attrs[name]
			if at == nil {
				ck.unknown(r, a, c, names, name, blk.Blocks[name] != nil)
				break
			}
			w.Known = true
			w.Flags = at.Names()
			ref := c.ref(append(blockJSONPath(names), "attributes", name)...)
			w.Fact = &ref
			if at.Deprecated {
				ck.add(IaCFinding{Kind: FindingDeprecated, Block: ck.block, Path: a.Path, Via: ViaAttribute, File: a.Span.File, Line: a.Span.StartLine,
					Binding: status, Message: fmt.Sprintf("%q is deprecated in %s %s", a.Path, c.Type, c.Fact.Version), Fact: ref})
			}
			break
		}
		ck.written = append(ck.written, w)
	}

	if rootUnknown {
		ck.disclose(fmt.Sprintf("required arguments of %s were not checked: the reader could not walk the block, so every attribute reads unknown", ck.block))
	} else {
		ck.checkRequired(r, c, written, unknownLevels)
	}
	if exprs > 0 {
		ck.disclose(fmt.Sprintf("%s has %d attributes that read as expressions: their presence is checked, their values are not", ck.block, exprs))
	}
	if len(c.Block.Blocks) > 0 {
		ck.disclose("required nested blocks are not checked: a block written empty leaves no trace in the read")
	}
}

// levelOf is the block prefix an attribute path sits in: "" at the top, "rule[*]" inside rule.
func levelOf(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[:i]
	}
	return ""
}

func pathUpTo(path string, seg int) string {
	parts := strings.Split(path, ".")
	return strings.Join(parts[:seg+1], ".")
}

func (ck *attrChecker) unknown(r *Resource, a Attr, c *IaCContract, names []string, name string, otherKind bool) {
	where := "the top of"
	if len(names) > 0 {
		where = "the block " + strings.Join(names, ".") + " of"
	}
	msg := fmt.Sprintf("%q is not an argument of %s %s in %s %s", a.Path, where, c.Type, c.Fact.Source, c.Fact.Version)
	if otherKind {
		msg = fmt.Sprintf("%q is written in the wrong form: the contract has a block and an argument by that name apart", a.Path)
	}
	ck.add(IaCFinding{Kind: FindingUnknownAttribute, Block: ck.block, Path: a.Path, Via: ViaAttribute, File: a.Span.File, Line: a.Span.StartLine,
		Binding: ck.in.Binding.Status, Message: msg, Fact: c.ref(blockJSONPath(names)...)})
}

// writtenAsBlocks reports whether a list or set of objects was written as blocks, static or dynamic: block form
// writes only full[*] and full[*].member paths, never full itself.
func writtenAsBlocks(written map[string]bool, full string) bool {
	prefix := full + "[*]"
	for p := range written {
		if p == prefix || strings.HasPrefix(p, prefix+".") {
			return true
		}
	}
	return false
}

func (ck *attrChecker) checkRequired(r *Resource, c *IaCContract, written map[string]bool, unknownLevels map[string]bool) {
	// The block levels the file wrote: every prefix that ends in a block segment.
	levels := map[string]bool{"": true}
	for p := range written {
		parts := strings.Split(p, ".")
		for i, seg := range parts {
			if strings.HasSuffix(seg, "[*]") {
				levels[strings.Join(parts[:i+1], ".")] = true
			}
		}
	}
	names := make([]string, 0, len(levels))
	for l := range levels {
		names = append(names, l)
	}
	sort.Strings(names)
	line := r.Span.StartLine
	for _, level := range names {
		if unknownLevels[level] {
			ck.disclose(fmt.Sprintf("required arguments of %s%s were not checked: an attribute there reads unknown, and an unknown attribute may be absent", ck.block, levelSuffix(level)))
			continue
		}
		blk := c.Block
		var path []string
		ok := true
		if level != "" {
			for _, seg := range strings.Split(level, ".") {
				nb := blk.Blocks[strings.TrimSuffix(seg, "[*]")]
				if nb == nil {
					ok = false
					break
				}
				path = append(path, strings.TrimSuffix(seg, "[*]"))
				blk = nb.Block
			}
		}
		if !ok {
			continue // an unknown block is already a finding
		}
		var req []string
		for n, at := range blk.Attrs {
			if at.Required {
				req = append(req, n)
			}
		}
		sort.Strings(req)
		for _, n := range req {
			full := n
			if level != "" {
				full = level + "." + n
			}
			if written[full] || writtenAsBlocks(written, full) {
				continue
			}
			ck.add(IaCFinding{Kind: FindingRequiredUnset, Block: ck.block, Path: full, Via: ViaAttribute, File: r.Span.File, Line: line,
				Binding: ck.in.Binding.Status, Message: fmt.Sprintf("%q is required by %s %s and is not set", full, c.Type, c.Fact.Version),
				Fact: c.ref(append(blockJSONPath(path), "attributes", n)...)})
		}
	}
}

func levelSuffix(level string) string {
	if level == "" {
		return ""
	}
	return " in " + level
}

// refTarget splits a reference into the block it names and the attribute path read from it. ok is false for
// anything that is not a resource or data source read: a variable, a local, a module output, an iterator.
func refTarget(ref string) (mode, typ, name string, attr []string, ok bool) {
	parts := strings.Split(ref, ".")
	switch parts[0] {
	case "var", "local", "module", "each", "count", "self", "path", "terraform", "tofu", "provider", "ephemeral":
		return "", "", "", nil, false
	case "data":
		if len(parts) >= 4 {
			return ModeData, parts[1], parts[2], parts[3:], true
		}
		return "", "", "", nil, false
	}
	if len(parts) >= 3 {
		return ModeManaged, parts[0], parts[1], parts[2:], true
	}
	return "", "", "", nil, false
}

func targetAddress(mode, typ, name string) string {
	if mode == ModeData {
		return "data." + typ + "." + name
	}
	return typ + "." + name
}

// checkTraversals checks each static reference an attribute reads against the contract of the block it names.
func (ck *attrChecker) checkTraversals(r *Resource) {
	if ck.in.Resolve == nil {
		return
	}
	for _, a := range r.Attrs {
		for _, ref := range a.Refs {
			mode, typ, name, attr, ok := refTarget(ref)
			if !ok {
				continue
			}
			b, c := ck.in.Resolve(mode, typ, name)
			if c == nil {
				continue
			}
			first := attr[0]
			addr := targetAddress(mode, typ, name)
			if _, isAttr := c.Block.Attrs[first]; !isAttr && c.Block.Blocks[first] == nil && first != "id" {
				ck.add(IaCFinding{Kind: FindingUnknownAttribute, Block: blockAddress(r), Path: ref, Via: ViaTraversal, File: a.Span.File, Line: a.Span.StartLine,
					Binding: b.Status, Message: fmt.Sprintf("%q reads %q, which %s %s does not have", ref, first, c.Type, c.Fact.Version), Fact: c.ref(blockJSONPath(nil)...)})
				continue
			}
			if at := c.Block.Attrs[first]; at != nil && at.Deprecated {
				ck.add(IaCFinding{Kind: FindingDeprecated, Block: blockAddress(r), Path: ref, Via: ViaTraversal, File: a.Span.File, Line: a.Span.StartLine,
					Binding: b.Status, Message: fmt.Sprintf("%q reads %q of %s, which is deprecated in %s", ref, first, addr, c.Fact.Version), Fact: c.ref("block", "attributes", first)})
			}
		}
	}
}

// checkOutputs finds an output that is not marked sensitive and reads a sensitive attribute.
func (ck *attrChecker) checkOutputs() {
	if ck.in.Resolve == nil {
		return
	}
	for _, o := range ck.in.Outputs {
		addr := "output." + o.Name
		if o.Unread != "" {
			ck.disclose(fmt.Sprintf("%s was not judged: %s", addr, o.Unread))
			continue
		}
		if o.Marked {
			continue
		}
		for _, ref := range o.Reads {
			mode, typ, name, attr, ok := refTarget(ref)
			if !ok {
				continue
			}
			b, c := ck.in.Resolve(mode, typ, name)
			if c == nil {
				continue
			}
			at := c.Block.Attrs[attr[0]]
			if at == nil {
				continue
			}
			path := []string{"block", "attributes", attr[0]}
			for _, seg := range attr[1:] {
				if !at.Sensitive && at.Nested != nil && at.Nested[seg] != nil {
					at = at.Nested[seg]
					path = append(path, "nested_type", "attributes", seg)
					continue
				}
				break
			}
			if !at.Sensitive {
				continue
			}
			ck.add(IaCFinding{Kind: FindingSensitiveOutput, Block: addr, Path: ref, Via: ViaOutput, File: o.Span.File, Line: o.Span.StartLine,
				Binding: b.Status, Message: fmt.Sprintf("%s reads %q, which %s %s marks sensitive, and the output is not marked sensitive", addr, ref, c.Type, c.Fact.Version),
				Fact: c.ref(path...)})
		}
	}
}
