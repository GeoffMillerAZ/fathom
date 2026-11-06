package iacfacts

// cfn_check.go checks a CloudFormation resource's Properties against its registry schema (IF-C2).

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Finding kinds of CheckCloudFormationUse.
const (
	CFNUnknownProperty = "unknown-property"
	CFNMissingRequired = "missing-required"
	CFNReadOnlySet     = "read-only-set"
	CFNDeprecated      = "deprecated"
)

// CFNUnversioned is the version a CloudFormation binding reads: a template names none.
const CFNUnversioned = "unversioned"

// CFNBinding says which catalog answered, or why none did.
type CFNBinding struct {
	Bound bool
	// Version is always CFNUnversioned.
	Version string
	// Catalog is the catalog file name, Region its region, Date its build day and Schema its snapshot digest. All are
	// empty when no catalog was chosen.
	Catalog, Region, Date, Schema string
	// Note is the statement an answer carries: what the facts are read from when bound, why not when unbound.
	Note string
}

// CFNFinding is one property of a resource that its schema contradicts.
type CFNFinding struct {
	// Kind is one of the CFN* kinds. Property is the top-level property name.
	Kind, Property string
	// Span is the property's span in the template, or the resource's for a missing required property.
	Span Span
	// Pointer is the schema pointer the schema states it with (/properties/Name), empty for an unknown property.
	Pointer string
	// Path is where in the catalog blob the fact sits (readOnlyProperties, deprecatedProperties, required,
	// properties) and Digest is that blob's sha256, so a finding cites the catalog it came from.
	Path, Digest string
}

// CFNUse is the result of CheckCloudFormationUse for one resource.
type CFNUse struct {
	Name, Type string
	Binding    CFNBinding
	// Findings are sorted by span, then kind, then property. A property the template sets once is reported once.
	Findings []CFNFinding
}

// cfnBlob is the pruned schema a catalog holds for one type.
type cfnBlob struct {
	TypeName             string                     `json:"typeName"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
	CreateOnly           []string                   `json:"createOnlyProperties"`
	CondCreateOnly       []string                   `json:"conditionalCreateOnlyProperties"`
	ReadOnly             []string                   `json:"readOnlyProperties"`
	Deprecated           []string                   `json:"deprecatedProperties"`
}

// CheckCloudFormationUse walks one CloudFormation resource's Properties against its registry schema and returns the
// unknown properties, the missing required ones, the read-only ones it sets and the deprecated ones it sets. It is
// what IF-C3 (symbol_context) and IF-C4 (review_diff) call, with the installed catalogs from ListIaCCatalogs and the
// resource from ReadCloudFormationFile.
//
// region picks the catalog. Empty reads the only CloudFormation catalog installed, and two or more with no region
// named is unbound, never a guess. Binding is unversioned: a template names no version, so the answer states the
// catalog's date. A type the schemas do not hold reads unbound: AWS::Serverless::* and other transform types, custom
// resources and third-party types are not in the registry schemas. A resource whose Properties the reader could not
// walk (an intrinsic, an alias, a merge) proves no unknown or missing property. Only top-level properties are
// checked: a nested path reads as its top-level property, and nested schema facts are not walked.
func CheckCloudFormationUse(catalogs []IaCCatalogInfo, region string, r Resource) CFNUse {
	u, b, digest, ok := resolveCFN(catalogs, region, r)
	if !ok {
		return u
	}
	return checkCFNProperties(u, b, digest, r)
}

// resolveCFN picks the catalog for one CloudFormation resource and reads its type's schema. ok is false when the
// binding is unbound, whose Note says why. digest is the sha256 of the schema blob the facts are read from.
func resolveCFN(catalogs []IaCCatalogInfo, region string, r Resource) (u CFNUse, b cfnBlob, digest string, ok bool) {
	u = CFNUse{Name: r.Name, Type: r.Type, Binding: CFNBinding{Version: CFNUnversioned}}
	unbound := func(format string, args ...any) (CFNUse, cfnBlob, string, bool) {
		u.Binding.Note = fmt.Sprintf(format, args...)
		return u, cfnBlob{}, "", false
	}
	if r.Tool != ToolCloudFormation {
		return unbound("%s is not a CloudFormation resource", r.Name)
	}
	if strings.HasPrefix(r.Type, "AWS::Serverless::") {
		return unbound("%s is a transform type (AWS::Serverless::*): the registry schemas do not hold it, so it reads unbound", r.Type)
	}
	var cfn []*IaCCatalogInfo
	for i := range catalogs {
		if catalogs[i].Manifest.Kind == KindCloudFormation {
			cfn = append(cfn, &catalogs[i])
		}
	}
	if len(cfn) == 0 {
		return unbound("no CloudFormation catalog is installed: fetch the registry schema zip with your own tools, then run `fathom iac import --region REGION <schemas.zip>`")
	}
	regions := make([]string, 0, len(cfn))
	var cat *IaCCatalogInfo
	for _, c := range cfn {
		regions = append(regions, c.Manifest.Line)
		if region != "" && c.Manifest.Line == region {
			cat = c
		}
	}
	sort.Strings(regions)
	switch {
	case region == "" && len(cfn) == 1:
		cat = cfn[0]
	case region == "":
		return unbound("%d CloudFormation catalogs are installed (regions %s) and no region was named", len(cfn), strings.Join(regions, ", "))
	case cat == nil:
		return unbound("no CloudFormation catalog is installed for %s (regions installed: %s)", region, strings.Join(regions, ", "))
	}
	m := cat.Manifest
	version := m.Versions[0]
	u.Binding.Catalog, u.Binding.Region, u.Binding.Date, u.Binding.Schema = cat.Name, m.Line, m.Date, m.Inputs[version]
	fact, err := ReadIaCCatalog(cat.Path, IaCCatalogRequest{Version: version, Kind: SectionResource, Type: r.Type})
	if err != nil {
		var ref *IaCRefusal
		if errors.As(err, &ref) && ref.Code == RefuseNotFound {
			return unbound("%s is not in the catalog %s (region %s); custom and third-party types are not in the registry schemas, so it reads unbound", r.Type, cat.Name, m.Line)
		}
		return unbound("the catalog %s could not answer for %s: %v", cat.Name, r.Type, err)
	}
	if err := json.Unmarshal(fact.Value, &b); err != nil {
		return unbound("the catalog %s holds an unreadable schema for %s: %v", cat.Name, r.Type, err)
	}
	u.Binding.Bound = true
	date := m.Date
	if date == "" {
		date = "an unrecorded date"
	}
	u.Binding.Note = fmt.Sprintf("a template names no version, so the facts are the registry schemas of region %s as of %s (catalog %s, schema digest %s); the contract states them", m.Line, date, cat.Name, u.Binding.Schema)
	return u, b, fact.Digest, true
}

// checkCFNProperties walks a resolved resource's Properties against its schema.
func checkCFNProperties(u CFNUse, b cfnBlob, digest string, r Resource) CFNUse {
	set := map[string]Span{} // top-level property name to its first span
	walkable := true
	for _, a := range r.Attrs {
		name := topProperty(a.Path)
		if name == "" || strings.HasPrefix(name, "Fn::") {
			walkable = false
			continue
		}
		if sp, ok := set[name]; !ok || a.Span.less(sp) {
			set[name] = a.Span
		}
	}
	add := func(kind, prop string, sp Span, pointer, path string) {
		u.Findings = append(u.Findings, CFNFinding{Kind: kind, Property: prop, Span: sp, Pointer: pointer, Path: path, Digest: digest})
	}
	ro, dep := pointerSet(b.ReadOnly), pointerSet(b.Deprecated)
	for name, sp := range set {
		if _, known := b.Properties[name]; !known && walkable && b.AdditionalProperties != nil && !*b.AdditionalProperties && !ro[name] && !dep[name] {
			add(CFNUnknownProperty, name, sp, "", "properties")
		}
		if ro[name] {
			add(CFNReadOnlySet, name, sp, "/properties/"+name, "readOnlyProperties")
		}
		if dep[name] {
			add(CFNDeprecated, name, sp, "/properties/"+name, "deprecatedProperties")
		}
	}
	if walkable {
		for _, name := range b.Required {
			if _, ok := set[name]; !ok {
				add(CFNMissingRequired, name, r.Span, "/properties/"+name, "required")
			}
		}
	}
	sort.Slice(u.Findings, func(i, j int) bool {
		a, c := u.Findings[i], u.Findings[j]
		if a.Span != c.Span {
			return a.Span.less(c.Span)
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		return a.Property < c.Property
	})
	return u
}

// topProperty is the first segment of a reader attribute path: Tags[*].Key reads Tags. An empty path is the whole
// Properties and has no name.
func topProperty(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// pointerSet holds the top-level property names a schema pointer list names (/properties/Name). A nested pointer
// (/properties/Name/Sub) is not a top-level fact and is left out.
func pointerSet(ptrs []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range ptrs {
		rest, ok := strings.CutPrefix(p, "/properties/")
		if ok && rest != "" && !strings.Contains(rest, "/") {
			out[rest] = true
		}
	}
	return out
}
