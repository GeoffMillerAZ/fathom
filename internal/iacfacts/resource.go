// Package iacfacts holds infrastructure-as-code facts that need no store, daemon, index,
// git or network: resource reads (this file and read_*.go) and, beside them, ADR-091's
// provider contracts.
//
// A resource read keeps what extraction discards, the attributes of a resource, each with
// a path, a value class and a span (docs/design/lang-library.md section 5). It never runs
// init, a plan, a provider or a program, never expands a module, and keeps no value unless
// the caller passes WithValues. A value never appears in an error.
//
// This package joins the closure of the public door (pkg/lang/iac). It imports no store,
// fusion, connector, MCP, net/http or CUE package, and read_imports_test.go fails on one.
package iacfacts

import (
	"errors"
	"fmt"
	"sort"
)

// Class is how an attribute's value is known. There are five. Absence is not one: an
// unset attribute is learned by comparing a read with a contract.
type Class string

const (
	// ClassLiteral: written as a constant. A check may treat it as a fact.
	ClassLiteral Class = "literal"
	// ClassDefaulted: resolves only through a variable default or a local chain ending
	// in one. A check may treat it as an assumption an operator can override.
	ClassDefaulted Class = "defaulted"
	// ClassReference: names another resource, data source, module output or parameter.
	// A check may treat it as a relation.
	ClassReference Class = "reference"
	// ClassExpression: computed, with no static answer. A check treats it as unknown.
	ClassExpression Class = "expression"
	// ClassUnknown: the reader could not classify it. A check treats it as unknown.
	ClassUnknown Class = "unknown"
)

// Tool names the tool a resource was read from.
const (
	ToolTerraform      = "terraform"
	ToolCloudFormation = "cloudformation"
	ToolPulumi         = "pulumi"
)

// Mode says what a read block is. Terraform and OpenTofu have managed resources, data
// sources and module calls. A module call reads as a resource-like relation and is never
// expanded.
const (
	ModeManaged = "managed"
	ModeData    = "data"
	ModeModule  = "module"
	// ModeEphemeral is an OpenTofu ephemeral resource, which is read for the plan and never stored.
	ModeEphemeral = "ephemeral"
)

// MultiplicityKind is how many instances a resource declares.
type MultiplicityKind string

const (
	MultiSingle    MultiplicityKind = "single"
	MultiCount     MultiplicityKind = "count"
	MultiForEach   MultiplicityKind = "for_each"
	MultiCondition MultiplicityKind = "condition"
)

// Span is a region of a source file. Lines and columns are 1-based, and columns count
// characters, not bytes. End is exclusive. File is the path the caller gave.
type Span struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// less orders spans by file, then start, then end.
func (s Span) less(o Span) bool {
	if s.File != o.File {
		return s.File < o.File
	}
	if s.StartLine != o.StartLine {
		return s.StartLine < o.StartLine
	}
	if s.StartCol != o.StartCol {
		return s.StartCol < o.StartCol
	}
	if s.EndLine != o.EndLine {
		return s.EndLine < o.EndLine
	}
	return s.EndCol < o.EndCol
}

// Multiplicity is the instance count of a resource. Class is the class of the count,
// for_each or condition expression, and is empty for MultiSingle. Refs names what the
// expression reads.
type Multiplicity struct {
	Kind  MultiplicityKind
	Class Class
	Refs  []string
	Span  Span
}

// Attr is one attribute of a resource.
type Attr struct {
	// Path is the attribute's path. It is empty for a value the reader could not walk at all, such as a
	// CloudFormation Properties that is an intrinsic function or a Pulumi properties that is an interpolation: the
	// one attribute at the root stands for every attribute of the resource, and reads unknown.
	//
	// Otherwise it is versioning[*].enabled, rule[*].id. Every nested Terraform
	// block segment takes [*], one block or several, as tofu lists blocks, and so does a dynamic
	// block and every CloudFormation or Pulumi list element. A resource's own lifecycle block
	// is meta-configuration and takes none.
	Path  string
	Class Class
	// Value is set only under WithValues, and only for literal and defaulted classes
	// whose value could be computed without evaluating a function.
	Value any
	// Refs are the static references the value reads, sorted: var.x, aws_s3_bucket.logs.id,
	// Param, Res.Arn. They name things, never values.
	Refs []string
	Span Span
}

// Resource is one resource, data source or module call.
type Resource struct {
	Tool, Mode, Type, Name, Provider string
	Span                             Span
	Attrs                            []Attr
	Multiplicity                     Multiplicity
}

// ResourceSet is what one read returned. Resources are sorted by span, attributes by
// path and then span.
type ResourceSet struct {
	Tool      string
	Resources []Resource
	// SyntaxErrors counts the syntax errors the parser reported, keys a mapping repeats, YAML documents after
	// the first (which are not read), values the reader could not walk (a root attribute with an empty path),
	// a resource section that is not a mapping (an alias, a tag or a scalar) and a resource whose merge key
	// (`<<`) may carry its properties. A read still returns the resources it could parse, and an attribute it could not read is unknown.
	SyntaxErrors int
	// Dropped names the YAML or JSON entries of a resource section that were not read as resources: a
	// Fn::ForEach, an alias, a merge-only entry, a value that is not a mapping, or one without a type. Absence
	// is never silent: a resource that is not in Resources and not in Dropped does not exist in the file.
	Dropped []string
}

// Option configures a read.
type Option func(*config)

type config struct {
	values bool
}

// WithValues asks a read to keep Attr.Value. It is off by default, and the product never
// passes it: only the public door, its CLI and tests do.
func WithValues() Option { return func(c *config) { c.values = true } }

func newConfig(opts []Option) config {
	var c config
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return c
}

// The product's limits (langroute.MaxExtractSize and the module cap), plus the bounds a
// read needs of its own. A read past any of them is refused whole, never truncated.
const (
	// MaxFileBytes is the largest file a read accepts.
	MaxFileBytes = 2 << 20
	// MaxModuleBytes is the most a directory read accepts across its files.
	MaxModuleBytes = 64 << 20
	// MaxModuleFiles is the most files a directory read accepts.
	MaxModuleFiles = 4096
	// MaxDepth bounds block, bracket and node nesting.
	MaxDepth = 128
	// MaxNodes bounds the syntax nodes one file may hold.
	MaxNodes = 1 << 20
	// MaxReadVisits bounds the first visits a template read makes while it classifies intrinsic functions and
	// resolves variables. Every node is visited a bounded number of times, so a file at MaxNodes stays inside it.
	MaxReadVisits = 4 * MaxNodes
	// MaxValueBytes bounds the logical size of one value a read builds under WithValues, counted the way a
	// consumer would serialise it. A value past it refuses the whole input: a chain of doublings is small as
	// source and enormous as a value.
	MaxValueBytes = 1 << 20
	// MaxTotalValueBytes bounds the logical size of every value one read keeps under WithValues. MaxValueBytes
	// bounds one value, and a thousand attributes that each sit just under it are a gigabyte of strings. A read
	// past it is refused whole, the way a value past MaxValueBytes is.
	MaxTotalValueBytes = 16 << 20
	// MaxAttrs bounds the attributes one read may return.
	MaxAttrs = 1 << 18
	// MaxPathBytes bounds one attribute path.
	MaxPathBytes = 4096
)

// ErrLimit is wrapped by every refusal at a limit. Its message names the limit and a
// count, never source text.
var ErrLimit = errors.New("iacfacts: input past a limit")

func limitErr(file, what string) error {
	return fmt.Errorf("%w: %s: %s", ErrLimit, file, what)
}

// checkSize refuses a file past MaxFileBytes.
func checkSize(file string, n int) error {
	if n > MaxFileBytes {
		return limitErr(file, fmt.Sprintf("file is %d bytes, over %d", n, MaxFileBytes))
	}
	return nil
}

// joinPath appends a name to an attribute path.
func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// sortSet puts a set in its deterministic order: resources by span, attributes by path
// then span, references sorted and without duplicates.
func sortSet(set *ResourceSet) {
	for i := range set.Resources {
		r := &set.Resources[i]
		sort.SliceStable(r.Attrs, func(a, b int) bool {
			if r.Attrs[a].Path != r.Attrs[b].Path {
				return r.Attrs[a].Path < r.Attrs[b].Path
			}
			return r.Attrs[a].Span.less(r.Attrs[b].Span)
		})
		r.Multiplicity.Refs = sortedRefs(r.Multiplicity.Refs)
	}
	sort.SliceStable(set.Resources, func(a, b int) bool {
		x, y := set.Resources[a], set.Resources[b]
		if x.Span != y.Span {
			return x.Span.less(y.Span)
		}
		if x.Mode != y.Mode {
			return x.Mode < y.Mode
		}
		if x.Type != y.Type {
			return x.Type < y.Type
		}
		return x.Name < y.Name
	})
}

// sortedRefs returns refs sorted and deduplicated, nil when empty.
func sortedRefs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}
	out := append([]string(nil), refs...)
	sort.Strings(out)
	w := 0
	for i, r := range out {
		if i == 0 || r != out[w-1] {
			out[w] = r
			w++
		}
	}
	return out[:w]
}

// unionRefs merges reference lists into one sorted, duplicate-free list. A list handed over more than once, the
// memoized answer for a definition used many times, is merged once, so a structure that repeats one part n times
// costs the part's size and not n times it.
func unionRefs(lists ...[]string) []string {
	var all []string
	var seen map[*string]int
	for i, l := range lists {
		if len(l) == 0 {
			continue
		}
		if len(lists) > 16 {
			if seen == nil {
				seen = map[*string]int{}
			}
			if seen[&l[0]] >= len(l) {
				continue
			}
			seen[&l[0]] = len(l)
		} else {
			dup := false
			for _, e := range lists[:i] {
				if len(e) >= len(l) && &e[0] == &l[0] {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
		}
		all = append(all, l...)
	}
	return sortedRefs(all)
}

// emitter collects attributes under the bounds, for every reader.
type emitter struct {
	cfg   config
	attrs int
	// valBytes is the running logical size of the values kept so far.
	valBytes int
	err      error
}

func (e *emitter) fail(file, what string) {
	if e.err == nil {
		e.err = limitErr(file, what)
	}
}

// emit appends one attribute. It keeps a value only under WithValues, and only for the
// classes a check may treat as known.
func (e *emitter) emit(file string, out *[]Attr, p string, c readRes, span Span) {
	if e.err != nil {
		return
	}
	if len(p) > MaxPathBytes {
		e.fail(file, fmt.Sprintf("an attribute path is over %d bytes", MaxPathBytes))
		return
	}
	if e.attrs++; e.attrs > MaxAttrs {
		e.fail(file, fmt.Sprintf("more than %d attributes", MaxAttrs))
		return
	}
	if c.big {
		e.fail(file, fmt.Sprintf("a value is over %d bytes", MaxValueBytes))
		return
	}
	at := Attr{Path: p, Class: c.class, Refs: sortedRefs(c.refs), Span: span}
	if e.cfg.values && c.hasV && (c.class == ClassLiteral || c.class == ClassDefaulted) {
		if !c.charged {
			if e.valBytes += partSize(c); e.valBytes > MaxTotalValueBytes {
				e.fail(file, fmt.Sprintf("the values of one read are over %d bytes in all", MaxTotalValueBytes))
				return
			}
		}
		at.Value = c.val
	}
	*out = append(*out, at)
}

// hold charges a value a reader keeps in a memo against MaxTotalValueBytes when the value is built. Emit charges the
// values an attribute returns, but a memoized value is held whether or not any attribute keeps it: a thousand
// locals that each build a 900 KB string, read by attributes that are expressions, are a gigabyte that no emit sees.
//
// It returns c marked charged: a value that is both memoized and emitted as it stands (a CloudFormation intrinsic's
// answer) is one value, and emit does not charge it a second time.
func (e *emitter) hold(file string, c readRes) readRes {
	if e == nil || e.err != nil || !e.cfg.values || !c.hasV || c.charged {
		return c
	}
	if e.valBytes += partSize(c); e.valBytes > MaxTotalValueBytes {
		e.fail(file, fmt.Sprintf("the values of one read are over %d bytes in all", MaxTotalValueBytes))
	}
	c.charged = true
	return c
}

// readRes is the classification of one value: its class, what it references and, when
// a value was asked for and could be computed, the value.
type readRes struct {
	class Class
	refs  []string
	val   any
	hasV  bool
	// vsize is the logical size of val, which a composite sums over its parts even when they share storage.
	// big marks a value that went past MaxValueBytes: its hasV is false, and emit refuses the input.
	vsize int
	big   bool
	// charged marks a value already counted against MaxTotalValueBytes when it was built (emitter.hold).
	charged bool
}

// partSize is the logical size of one part of a composite value.
func partSize(p readRes) int {
	if p.vsize > 0 {
		return p.vsize
	}
	if s, ok := p.val.(string); ok {
		return len(s) + 1
	}
	return 8
}
