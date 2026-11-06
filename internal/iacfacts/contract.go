package iacfacts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// IaCFactRef cites where a fact came from: the catalog file, the release, the digest of the blob and the JSON path
// inside it (design section 6). A reader can open the catalog and check it.
type IaCFactRef struct {
	Catalog string `json:"catalog"`
	Source  string `json:"source,omitempty"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
	Path    string `json:"path"`
}

// IaCAttrFlags are the pruned schema's flags for one attribute.
type IaCAttrFlags struct {
	Required   bool
	Optional   bool
	Computed   bool
	Sensitive  bool
	Deprecated bool
	WriteOnly  bool
}

// Names lists the flags that are on, in a fixed order.
func (f IaCAttrFlags) Names() []string {
	var out []string
	for _, p := range []struct {
		on   bool
		name string
	}{{f.Required, "required"}, {f.Optional, "optional"}, {f.Computed, "computed"}, {f.Sensitive, "sensitive"}, {f.Deprecated, "deprecated"}, {f.WriteOnly, "write_only"}} {
		if p.on {
			out = append(out, p.name)
		}
	}
	return out
}

// IaCContractAttr is one attribute of a contract block.
type IaCContractAttr struct {
	IaCAttrFlags
	// Nested is set for an attribute whose value is an object with its own attributes.
	Nested map[string]*IaCContractAttr
	// BlockForm is true for an attribute whose type is a list or set of objects. Terraform and OpenTofu accept
	// block syntax for such an attribute (attributes as blocks), so a block of that name is not a wrong form.
	BlockForm bool
	// Members names the members of a block-form attribute's object type, true for one the type marks optional.
	// It is nil when the type states none, and then the members are not checked.
	Members map[string]bool
}

// objectCollection reports whether a schema type is a list or set of objects: ["list",["object",{...}]].
func objectCollection(t json.RawMessage) bool {
	var outer []json.RawMessage
	if json.Unmarshal(t, &outer) != nil || len(outer) != 2 {
		return false
	}
	var kind string
	if json.Unmarshal(outer[0], &kind) != nil || (kind != "list" && kind != "set") {
		return false
	}
	var elem []json.RawMessage
	if json.Unmarshal(outer[1], &elem) != nil || len(elem) == 0 {
		return false
	}
	var ek string
	return json.Unmarshal(elem[0], &ek) == nil && ek == "object"
}

// objectMembers reads the member names of the object in a list or set of objects, and which of them the type marks
// optional: ["list",["object",{"a":"number"},["a"]]]. It returns nil when the type is no such collection.
func objectMembers(t json.RawMessage) map[string]bool {
	if !objectCollection(t) {
		return nil
	}
	var outer, elem []json.RawMessage
	if json.Unmarshal(t, &outer) != nil || json.Unmarshal(outer[1], &elem) != nil || len(elem) < 2 {
		return nil
	}
	var names map[string]json.RawMessage
	if json.Unmarshal(elem[1], &names) != nil {
		return nil
	}
	out := make(map[string]bool, len(names))
	for n := range names {
		out[n] = false
	}
	if len(elem) > 2 {
		var opt []string
		if json.Unmarshal(elem[2], &opt) == nil {
			for _, n := range opt {
				if _, ok := out[n]; ok {
					out[n] = true
				}
			}
		}
	}
	return out
}

// IaCContractBlock is a resource's schema block, or a nested block's.
type IaCContractBlock struct {
	Deprecated bool
	Attrs      map[string]*IaCContractAttr
	Blocks     map[string]*IaCContractNested
}

// IaCContractNested is a nested block type.
type IaCContractNested struct {
	Mode     string
	MinItems int
	Block    *IaCContractBlock
}

// IaCContract is the decoded contract of one resource or data source type at one release. Fact is the blob it was
// read from. Facts 1 to 4 (design section 3) all read from it.
type IaCContract struct {
	Fact  IaCFactRef
	Type  string
	Block *IaCContractBlock
}

// wire shapes of a pruned blob (see pruneIaCSchema).
type contractWire struct {
	Block *blockWire `json:"block"`
}

type blockWire struct {
	Attributes map[string]attrWire   `json:"attributes"`
	BlockTypes map[string]nestedWire `json:"block_types"`
	Deprecated bool                  `json:"deprecated"`
}

type attrWire struct {
	Type       json.RawMessage `json:"type"`
	Required   bool            `json:"required"`
	Optional   bool            `json:"optional"`
	Computed   bool            `json:"computed"`
	Sensitive  bool            `json:"sensitive"`
	Deprecated bool            `json:"deprecated"`
	WriteOnly  bool            `json:"write_only"`
	NestedType *struct {
		Attributes map[string]attrWire `json:"attributes"`
	} `json:"nested_type"`
}

type nestedWire struct {
	NestingMode string     `json:"nesting_mode"`
	MinItems    int        `json:"min_items"`
	Block       *blockWire `json:"block"`
}

func (w blockWire) decode() *IaCContractBlock {
	b := &IaCContractBlock{Deprecated: w.Deprecated, Attrs: make(map[string]*IaCContractAttr, len(w.Attributes)), Blocks: make(map[string]*IaCContractNested, len(w.BlockTypes))}
	for n, a := range w.Attributes {
		b.Attrs[n] = a.decode()
	}
	for n, nb := range w.BlockTypes {
		inner := &IaCContractBlock{Attrs: map[string]*IaCContractAttr{}, Blocks: map[string]*IaCContractNested{}}
		if nb.Block != nil {
			inner = nb.Block.decode()
		}
		b.Blocks[n] = &IaCContractNested{Mode: nb.NestingMode, MinItems: nb.MinItems, Block: inner}
	}
	return b
}

func (a attrWire) decode() *IaCContractAttr {
	out := &IaCContractAttr{IaCAttrFlags: IaCAttrFlags{Required: a.Required, Optional: a.Optional, Computed: a.Computed, Sensitive: a.Sensitive, Deprecated: a.Deprecated, WriteOnly: a.WriteOnly}, BlockForm: objectCollection(a.Type), Members: objectMembers(a.Type)}
	if a.NestedType != nil {
		out.Nested = make(map[string]*IaCContractAttr, len(a.NestedType.Attributes))
		for n, x := range a.NestedType.Attributes {
			out.Nested[n] = x.decode()
		}
	}
	return out
}

// LoadIaCContract reads the contract of one resource or data source type from the catalog a binding names. It
// answers only for a locked or constraint-matched binding: any other status has no facts, and a nearest version is
// never used. section is SectionResource, SectionData or SectionEphemeral. dir is the catalog directory the
// binding's catalogs were listed from. The blob is read whole under the catalog's bounds; a bound hit refuses this
// one answer with a reason. A type the release does not hold reports ErrIaCTypeAbsent.
func LoadIaCContract(dir string, b IaCBinding, section, typ string) (*IaCContract, error) {
	if b.Status != BindLocked && b.Status != BindConstraintMatched {
		return nil, fmt.Errorf("iac contract: a %s binding carries no facts", b.Status)
	}
	if b.Catalog == "" || b.Version == "" {
		return nil, fmt.Errorf("iac contract: the binding names no catalog or version")
	}
	f, err := ReadIaCCatalog(filepath.Join(dir, b.Catalog), IaCCatalogRequest{Version: b.Version, Kind: section, Type: typ})
	if err != nil {
		if ref, ok := err.(*IaCRefusal); ok && ref.Code == RefuseNotFound {
			return nil, &ErrIaCTypeAbsent{Type: typ, Version: b.Version, Catalog: b.Catalog}
		}
		return nil, err
	}
	var w contractWire
	if err := json.Unmarshal(f.Value, &w); err != nil {
		return nil, fmt.Errorf("iac contract: blob %s is unreadable: %w", f.Digest, err)
	}
	c := &IaCContract{
		Fact: IaCFactRef{Catalog: f.Catalog, Source: f.Source, Version: f.Version, Digest: f.Digest},
		Type: typ,
	}
	if w.Block == nil {
		c.Block = &IaCContractBlock{Attrs: map[string]*IaCContractAttr{}, Blocks: map[string]*IaCContractNested{}}
	} else {
		c.Block = w.Block.decode()
	}
	return c, nil
}

// ErrIaCTypeAbsent says a release holds no schema for a type. It is not a finding: the block may belong to another
// provider than the one the binding resolved.
type ErrIaCTypeAbsent struct{ Type, Version, Catalog string }

func (e *ErrIaCTypeAbsent) Error() string {
	return fmt.Sprintf("iac contract: %s is not a type of %s at %s", e.Type, e.Catalog, e.Version)
}

// ref returns the fact cited at one JSON path of the blob.
func (c *IaCContract) ref(path ...string) IaCFactRef {
	r := c.Fact
	r.Path = strings.Join(path, ".")
	return r
}

// blockJSONPath is the blob path of a nested block reached through names (each a block type name).
func blockJSONPath(names []string) []string {
	p := []string{"block"}
	for _, n := range names {
		p = append(p, "block_types", n, "block")
	}
	return p
}
