package iac

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// Catalog is a provider contract catalog an operator placed on disk (docs/design/iac-facts.md section 4): a
// deterministic zip of one provider's major line, holding every stable release's resource, data source and
// ephemeral resource schemas as names, types and flags. OpenCatalog validates its manifest and keeps nothing
// else: each lookup opens the zip, reads one version index and one blob, and closes it. A Catalog holds no file
// handle and no memo, is safe for concurrent use, and is never closed.
//
// The library fetches nothing. A catalog is built by the operator from a local schema export, with their own tools.
type Catalog struct {
	path string
	m    iacfacts.IaCCatalogManifest
}

// Kind names the section of a catalog a type is looked up in.
type Kind string

// The kinds a catalog holds.
const (
	KindResource   Kind = "resource"
	KindDataSource Kind = "data_source"
	KindEphemeral  Kind = "ephemeral_resource"
)

// CatalogError is a catalog or one answer that a bound or a check refused. Code is one of catalog-bytes, entries,
// manifest-bytes, index-bytes, blob-bytes, depth, fact-bytes, corrupt, format or not-found, and Scope is "catalog"
// when the whole catalog is refused and "answer" when one lookup is. Reason is a sentence a caller can print and
// holds no schema text.
type CatalogError struct {
	Scope  string
	Code   string
	Reason string
}

func (e *CatalogError) Error() string {
	return fmt.Sprintf("iac catalog: %s refused (%s): %s", e.Scope, e.Code, e.Reason)
}

// Is reports a not-found error as ErrNotFound.
func (e *CatalogError) Is(target error) bool {
	return target == ErrNotFound && e.Code == iacfacts.RefuseNotFound
}

// ErrNotFound matches (errors.Is) a lookup the catalog cannot answer: a version it does not hold, a type it does
// not list or a provider it was not built for.
var ErrNotFound = errors.New("iac catalog: not found")

func catalogErr(err error) error {
	var ref *iacfacts.IaCRefusal
	if errors.As(err, &ref) {
		return &CatalogError{Scope: ref.Scope, Code: ref.Code, Reason: ref.Reason}
	}
	return err
}

// OpenCatalog opens the catalog at path and validates its manifest: the format, the kind, a provider source and
// major line, and a version list that is valid, ascending and inside the line. A catalog over its size or entry
// bound, or with a corrupt manifest, is refused with a *CatalogError. Opening reads no schema.
func OpenCatalog(path string) (*Catalog, error) {
	m, err := iacfacts.OpenIaCCatalogManifest(path)
	if err != nil {
		return nil, catalogErr(err)
	}
	return &Catalog{path: path, m: m}, nil
}

// Source is the provider address the catalog was built for, host included.
func (c *Catalog) Source() string { return c.m.Source }

// Line is the provider's major version: one catalog holds every release of one line.
func (c *Catalog) Line() string { return c.m.Line }

// Versions lists the releases the catalog holds, oldest first.
func (c *Catalog) Versions() []string { return append([]string{}, c.m.Versions...) }

// ReplacementFacts is the catalog's own claim about replacement facts: "absent" for a catalog that carries none.
func (c *Catalog) ReplacementFacts() string { return c.m.Replacement }

// Replacement is one attribute's replacement fact: what changing it does to the resource, and where the claim
// came from. A catalog carries replacement facts only when its manifest says so (Catalog.ReplacementFacts is not
// "absent"), and an answer never says a change is safe: it says what the contract states.
type Replacement struct {
	// Class is "replace", "conditional" or "unknown".
	Class string `json:"class"`
	// Provenance names the provider source the fact was read from.
	Provenance string `json:"provenance"`
}

// ContractAttr is one attribute or nested block of a type's contract.
type ContractAttr struct {
	// Path spells a nested block with [*], as the resource reads do: versioning[*].enabled. A block's own entry is
	// versioning[*]. An attribute with nested attributes spells list and set nesting with [*] and the rest with a dot.
	Path string `json:"path"`
	// Block is true for a nested block, whose own entry carries MinItems and MaxItems.
	Block bool `json:"block"`
	// Type is the attribute's type as the schema export writes it ("string", or JSON such as ["list","string"]),
	// "nested_type" for an attribute that holds nested attributes, and "" for a block.
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	Optional   bool   `json:"optional"`
	Computed   bool   `json:"computed"`
	Sensitive  bool   `json:"sensitive"`
	WriteOnly  bool   `json:"write_only"`
	Deprecated bool   `json:"deprecated"`
	MinItems   int    `json:"min_items"`
	MaxItems   int    `json:"max_items"`
	// Replacement is nil when the catalog states none for this attribute.
	Replacement *Replacement `json:"replacement"`
	// JSONPath is where in the type's schema blob the entry was read, dot-joined, so a reader can check it.
	JSONPath string `json:"json_path"`
}

// Contract is what a catalog states about one type at one version. Every fact cites the catalog, the version and
// the digest of the blob it was read from.
type Contract struct {
	Catalog string `json:"catalog"`
	Source  string `json:"source"`
	Version string `json:"version"`
	Kind    Kind   `json:"kind"`
	Type    string `json:"type"`
	// Digest is the sha256 of the schema blob.
	Digest string `json:"digest"`
	// ReplacementFacts is the catalog's claim about replacement facts ("absent" when it carries none).
	ReplacementFacts string         `json:"replacement_facts"`
	Attributes       []ContractAttr `json:"attributes"`
}

// Contract returns the type's contract at an exact version. The type is looked up as a resource, then as a data
// source, then as an ephemeral resource, and Contract.Kind says which answered: a type that is both a resource and
// a data source reads as the resource, and KindContract asks for the other. It opens the catalog, reads one version
// index and one blob, and closes it.
//
// source is the provider address, which must be the catalog's (lower case; no host is the default registry, and the
// two default registries stand in for each other). version must be one of Versions: a nearest version is never
// answered. A type or version the catalog lacks is a *CatalogError that matches ErrNotFound.
func (c *Catalog) Contract(source, version, typ string) (*Contract, error) {
	if err := c.check(source, version, typ); err != nil {
		return nil, err
	}
	var last error
	for _, k := range []Kind{KindResource, KindDataSource, KindEphemeral} {
		ct, err := c.KindContract(source, version, k, typ)
		if err == nil {
			return ct, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		last = err
	}
	return nil, &CatalogError{Scope: iacfacts.ScopeAnswer, Code: iacfacts.RefuseNotFound,
		Reason: fmt.Sprintf("version %s has no resource, data source or ephemeral resource %q (%v)", version, typ, last)}
}

// check refuses a lookup for another provider or a version the catalog does not hold.
func (c *Catalog) check(source, version, typ string) error {
	if !iacfacts.SameProviderSource(c.m.Source, source) {
		return &CatalogError{Scope: iacfacts.ScopeAnswer, Code: iacfacts.RefuseNotFound,
			Reason: fmt.Sprintf("the catalog is built for %s, not %q", c.m.Source, source)}
	}
	if typ == "" {
		return errors.New("iac: type is empty")
	}
	for _, v := range c.m.Versions {
		if v == version {
			return nil
		}
	}
	return &CatalogError{Scope: iacfacts.ScopeAnswer, Code: iacfacts.RefuseNotFound,
		Reason: fmt.Sprintf("the catalog holds no version %s (it holds the %s line: %s)", version, c.m.Line, strings.Join(c.m.Versions, ", "))}
}

// KindContract is Contract for one kind.
func (c *Catalog) KindContract(source, version string, kind Kind, typ string) (*Contract, error) {
	if err := c.check(source, version, typ); err != nil {
		return nil, err
	}
	var section string
	switch kind {
	case KindResource:
		section = iacfacts.SectionResource
	case KindDataSource:
		section = iacfacts.SectionData
	case KindEphemeral:
		section = iacfacts.SectionEphemeral
	default:
		return nil, fmt.Errorf("iac: unknown kind %q", kind)
	}
	fact, err := iacfacts.ReadIaCCatalog(c.path, iacfacts.IaCCatalogRequest{Version: version, Kind: section, Type: typ})
	if err != nil {
		return nil, catalogErr(err)
	}
	read := contractAttrs
	if c.m.Kind == iacfacts.KindCloudFormation {
		read = cfnContractAttrs
	}
	attrs, err := read(fact.Value)
	if err != nil {
		return nil, &CatalogError{Scope: iacfacts.ScopeAnswer, Code: iacfacts.RefuseCorrupt, Reason: fmt.Sprintf("the schema of %s %s is not one this build reads: %v", kind, typ, err)}
	}
	return &Contract{
		Catalog: fact.Catalog, Source: c.m.Source, Version: version, Kind: kind, Type: typ,
		Digest: fact.Digest, ReplacementFacts: c.m.Replacement, Attributes: attrs,
	}, nil
}

// blockJSON, attrJSON and the rest decode the pruned schema a catalog stores: {version, block: {attributes,
// block_types, deprecated}}, with the flags the pruner keeps.
type schemaJSON struct {
	Block *blockJSON `json:"block"`
}

type blockJSON struct {
	Attributes map[string]attrJSON      `json:"attributes"`
	BlockTypes map[string]blockTypeJSON `json:"block_types"`
	Deprecated bool                     `json:"deprecated"`
}

type attrJSON struct {
	Type       json.RawMessage `json:"type"`
	NestedType *nestedJSON     `json:"nested_type"`
	Required   bool            `json:"required"`
	Optional   bool            `json:"optional"`
	Computed   bool            `json:"computed"`
	Sensitive  bool            `json:"sensitive"`
	WriteOnly  bool            `json:"write_only"`
	Deprecated bool            `json:"deprecated"`
	// Replacement is read when a catalog carries replacement facts (IF-13); none does yet.
	Replacement *Replacement `json:"replacement"`
}

type nestedJSON struct {
	Attributes  map[string]attrJSON `json:"attributes"`
	NestingMode string              `json:"nesting_mode"`
}

type blockTypeJSON struct {
	NestingMode string     `json:"nesting_mode"`
	Block       *blockJSON `json:"block"`
	MinItems    int        `json:"min_items"`
	MaxItems    int        `json:"max_items"`
}

// contractAttrs flattens a schema blob into the attribute list, sorted by path.
func contractAttrs(blob []byte) ([]ContractAttr, error) {
	var s schemaJSON
	dec := json.NewDecoder(bytes.NewReader(blob))
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data after the schema")
	}
	out := []ContractAttr{}
	if s.Block != nil {
		walkBlock(&out, s.Block, "", "block")
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func join(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func walkBlock(out *[]ContractAttr, b *blockJSON, prefix, jpath string) {
	for name, a := range b.Attributes {
		walkAttr(out, a, join(prefix, name), jpath+".attributes."+name)
	}
	for name, bt := range b.BlockTypes {
		p := join(prefix, name) + "[*]"
		j := jpath + ".block_types." + name
		ent := ContractAttr{Path: p, Block: true, MinItems: bt.MinItems, MaxItems: bt.MaxItems, JSONPath: j}
		if bt.Block != nil {
			ent.Deprecated = bt.Block.Deprecated
		}
		ent.Required = bt.MinItems > 0
		ent.Optional = bt.MinItems == 0
		*out = append(*out, ent)
		if bt.Block != nil {
			walkBlock(out, bt.Block, p, j+".block")
		}
	}
}

func walkAttr(out *[]ContractAttr, a attrJSON, path, jpath string) {
	ent := ContractAttr{
		Path: path, Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive,
		WriteOnly: a.WriteOnly, Deprecated: a.Deprecated, Replacement: a.Replacement, JSONPath: jpath,
	}
	switch {
	case a.NestedType != nil:
		ent.Type = "nested_type"
	case len(a.Type) > 0:
		var s string
		if json.Unmarshal(a.Type, &s) == nil {
			ent.Type = s
		} else {
			var buf bytes.Buffer
			if json.Compact(&buf, a.Type) == nil {
				ent.Type = buf.String()
			}
		}
	}
	*out = append(*out, ent)
	if a.NestedType != nil {
		sep := path
		if a.NestedType.NestingMode == "list" || a.NestedType.NestingMode == "set" {
			sep += "[*]"
		}
		for name, in := range a.NestedType.Attributes {
			walkAttr(out, in, join(sep, name), jpath+".nested_type.attributes."+name)
		}
	}
}
