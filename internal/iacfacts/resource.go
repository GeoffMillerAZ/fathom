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
	// Path is the attribute's path: versioning.enabled, rule[*].id. A Terraform block
	// that repeats under one parent, a dynamic block and every CloudFormation or Pulumi
	// list element take [*].
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
	// SyntaxErrors counts the syntax errors the parser reported. A read still returns
	// the resources it could parse, and an attribute it could not read is unknown.
	SyntaxErrors int
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
