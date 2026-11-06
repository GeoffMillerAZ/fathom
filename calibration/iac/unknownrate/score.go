package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// The classes of a (rule, resource) pair. The first four are the reader's own classes folded by precedence
// (expression and unknown are one), and unset means the reader found no attribute there. See PREREGISTRATION.md.
const (
	classLiteral   = "literal"
	classReference = "reference"
	classDefaulted = "defaulted"
	classUnknown   = "unknown"
	classUnset     = "unset"
)

// Binding is where one rule looks in one tool: resource types and one attribute path.
type Binding struct {
	Types []string `json:"types"`
	Path  string   `json:"path"`
}

// Rule is an attribute check, spelled once per tool.
type Rule struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Terraform      Binding `json:"terraform"`
	CloudFormation Binding `json:"cloudformation"`
	Pulumi         Binding `json:"pulumi"`
}

// binding returns the rule's path for a resource when the resource is of a type the rule names. Terraform matches
// managed resources only.
func (r Rule) binding(res iac.Resource) (string, bool) {
	var b Binding
	switch res.Tool {
	case iac.ToolTerraform:
		if res.Mode != iac.ModeManaged {
			return "", false
		}
		b = r.Terraform
	case iac.ToolCloudFormation:
		b = r.CloudFormation
	case iac.ToolPulumi:
		b = r.Pulumi
	default:
		return "", false
	}
	for _, t := range b.Types {
		if t == res.Type {
			return b.Path, true
		}
	}
	return "", false
}

// Subject is one pinned Terraform directory, CloudFormation template or Pulumi.yaml.
type Subject struct {
	ID, Tool, Kind, Repo, SHA, Path string
}

// root is the directory a fetched repository sits in.
func (s Subject) root(dest string) string {
	return filepath.Join(dest, strings.ReplaceAll(s.Repo, "/", "_"))
}

func loadRules(file string) ([]Rule, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var rs []Rule
	if err := json.Unmarshal(b, &rs); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return rs, nil
}

func loadSubjects(file string) ([]Subject, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Subject
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			return nil, fmt.Errorf("%s: want 6 tab-separated fields, got %d in %q", file, len(f), line)
		}
		out = append(out, Subject{ID: f[0], Tool: f[1], Kind: f[2], Repo: f[3], SHA: f[4], Path: f[5]})
	}
	return out, sc.Err()
}

// readSubject reads a subject with no options: Terraform as one module directory, the rest as one file.
func readSubject(ctx context.Context, dest string, s Subject) (*iac.ResourceSet, error) {
	root := s.root(dest)
	if s.Tool == iac.ToolTerraform {
		return iac.ReadModule(ctx, os.DirFS(root), s.Path)
	}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s.Path)))
	if err != nil {
		return nil, err
	}
	return iac.ReadFile(ctx, s.Path, src)
}

// Pair is one scored (rule, matching resource).
type Pair struct {
	Rule    string `json:"rule"`
	Subject string `json:"subject"`
	Tool    string `json:"tool"`
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Line    int    `json:"line"`
	EndLine int    `json:"end_line"`
	Path    string `json:"path"`
	Class   string `json:"class"`
	// Attrs are the attributes the class came from, as path:class:line.
	Attrs []string `json:"attrs"`
	// AttrSpans are the start and end line of each attribute in Attrs.
	AttrSpans [][2]int `json:"attr_spans"`
}

// below reports whether an attribute path is the rule's path or nested under it.
func below(attrPath, p string) bool {
	return attrPath == p || strings.HasPrefix(attrPath, p+"[*]") || strings.HasPrefix(attrPath, p+".")
}

// above reports whether an attribute path is a proper ancestor of the rule's path, on a segment boundary, or the
// root attribute (empty path) that stands for a body the reader could not walk.
func above(attrPath, p string) bool {
	return attrPath == "" || strings.HasPrefix(p, attrPath+"[*]") || strings.HasPrefix(p, attrPath+".")
}

// classify is the class of a pair: unknown if any contributing attribute is expression or unknown, then
// defaulted, then reference, then literal, and unset when none contributes. An ancestor contributes only when it
// is expression or unknown.
func classify(r iac.Resource, p string) (string, []iac.Attr) {
	var used []iac.Attr
	var unknown, defaulted, reference, literal bool
	for _, a := range r.Attrs {
		switch {
		case below(a.Path, p):
		case above(a.Path, p) && (a.Class == iac.ClassExpression || a.Class == iac.ClassUnknown):
		default:
			continue
		}
		used = append(used, a)
		switch a.Class {
		case iac.ClassExpression, iac.ClassUnknown:
			unknown = true
		case iac.ClassDefaulted:
			defaulted = true
		case iac.ClassReference:
			reference = true
		default:
			literal = true
		}
	}
	switch {
	case unknown:
		return classUnknown, used
	case defaulted:
		return classDefaulted, used
	case reference:
		return classReference, used
	case literal:
		return classLiteral, used
	}
	return classUnset, used
}

// scoreSet scores every (rule, matching resource) of one read.
func scoreSet(set *iac.ResourceSet, s Subject, rules []Rule) []Pair {
	var out []Pair
	for _, rule := range rules {
		for _, res := range set.Resources {
			p, ok := rule.binding(res)
			if !ok {
				continue
			}
			class, used := classify(res, p)
			pair := Pair{Rule: rule.ID, Subject: s.ID, Tool: s.Tool, Kind: s.Kind, File: res.Span.File, Type: res.Type, Name: res.Name,
				Line: res.Span.StartLine, EndLine: res.Span.EndLine, Path: p, Class: class}
			for _, a := range used {
				pair.Attrs = append(pair.Attrs, fmt.Sprintf("%s:%s:%d", a.Path, a.Class, a.Span.StartLine))
				pair.AttrSpans = append(pair.AttrSpans, [2]int{a.Span.StartLine, a.Span.EndLine})
			}
			out = append(out, pair)
		}
	}
	return out
}

// Cell counts pairs by class.
type Cell struct {
	Pairs, Literal, Reference, Defaulted, Unknown, Unset int
}

func (c *Cell) add(class string) {
	c.Pairs++
	switch class {
	case classLiteral:
		c.Literal++
	case classReference:
		c.Reference++
	case classDefaulted:
		c.Defaulted++
	case classUnknown:
		c.Unknown++
	case classUnset:
		c.Unset++
	}
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// rate is the preregistered unknown rate: unknown over all pairs, unset included.
func (c Cell) rate() float64 { return ratio(c.Unknown, c.Pairs) }

// ratePresent leaves the unset pairs out of the denominator.
func (c Cell) ratePresent() float64 { return ratio(c.Unknown, c.Pairs-c.Unset) }

// rateWithDefaulted counts defaulted as unknown.
func (c Cell) rateWithDefaulted() float64 { return ratio(c.Unknown+c.Defaulted, c.Pairs) }

// auditKey orders the audit draw: the lowercase hex SHA-256 of the pair's identity under the preregistered seed.
func auditKey(p Pair) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("ll-m6-audit-v1|%s|%s|%s|%s|%s|%d", p.Rule, p.Subject, p.File, p.Type, p.Name, p.Line)))
	return hex.EncodeToString(sum[:])
}

// auditSelect takes, for Terraform and CloudFormation each, the perTool pairs whose keys sort first (all of them
// when fewer). Pulumi is not audited. The result is Terraform then CloudFormation, in key order.
func auditSelect(pairs []Pair, perTool int) []Pair {
	var out []Pair
	for _, tool := range []string{iac.ToolTerraform, iac.ToolCloudFormation} {
		var ps []Pair
		for _, p := range pairs {
			if p.Tool == tool {
				ps = append(ps, p)
			}
		}
		sort.SliceStable(ps, func(i, j int) bool {
			ki, kj := auditKey(ps[i]), auditKey(ps[j])
			if ki != kj {
				return ki < kj
			}
			return path.Join(ps[i].File, ps[i].Name) < path.Join(ps[j].File, ps[j].Name)
		})
		if len(ps) > perTool {
			ps = ps[:perTool]
		}
		out = append(out, ps...)
	}
	return out
}
