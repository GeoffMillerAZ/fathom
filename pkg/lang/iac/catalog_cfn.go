package iac

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// A CloudFormation catalog holds one pruned registry schema per type: the top-level property names with their JSON
// types, the required list, and the lists the schema states its facts in. Contract spells it as it spells a
// provider schema: a top-level property is a path of its own, and a create-only or conditional create-only property
// carries a Replacement whose provenance names the schema field. A nested pointer such as /properties/Tags/*/Key
// adds an entry at Tags[*].Key. The importer reads the nested object's required list from the raw schema, through
// definitions and $ref, and the entry then reads required, or optional unless read-only, as a top-level one does.
// An entry whose object it could not resolve states neither.

type cfnSchemaJSON struct {
	Properties map[string]struct {
		Type json.RawMessage `json:"type"`
	} `json:"properties"`
	Required       []string `json:"required"`
	CreateOnly     []string `json:"createOnlyProperties"`
	CondCreateOnly []string `json:"conditionalCreateOnlyProperties"`
	ReadOnly       []string `json:"readOnlyProperties"`
	WriteOnly      []string `json:"writeOnlyProperties"`
	Deprecated     []string `json:"deprecatedProperties"`
	// Nested maps a nested create-only path to whether its parent object requires it. A path the importer could
	// not resolve is absent.
	Nested map[string]bool `json:"nested"`
}

// nameSet is the set of property names the schema's required list holds, which are names and not pointers.
func nameSet(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

// topNames is the set of reader paths a list of schema pointers spells.
func topNames(ptrs []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range ptrs {
		if path, ok := iacfacts.CFNPointerPath(p); ok {
			out[path] = true
		}
	}
	return out
}

// cfnContractAttrs flattens a CloudFormation schema blob into the attribute list, sorted by path.
func cfnContractAttrs(blob []byte) ([]ContractAttr, error) {
	var s cfnSchemaJSON
	dec := json.NewDecoder(bytes.NewReader(blob))
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data after the schema")
	}
	required, readOnly, writeOnly, deprecated := nameSet(s.Required), topNames(s.ReadOnly), topNames(s.WriteOnly), topNames(s.Deprecated)
	repl := map[string]*Replacement{}
	field := map[string]string{}
	for _, l := range []struct {
		class, field string
		ptrs         []string
	}{{"replace", "createOnlyProperties", s.CreateOnly}, {"conditional", "conditionalCreateOnlyProperties", s.CondCreateOnly}} {
		for _, p := range l.ptrs {
			path, ok := iacfacts.CFNPointerPath(p)
			if !ok || repl[path] != nil {
				continue
			}
			repl[path] = &Replacement{Class: l.class, Provenance: iacfacts.CFNSource + " registry schema " + l.field}
			field[path] = l.field
		}
	}
	out := []ContractAttr{}
	for name, p := range s.Properties {
		ent := ContractAttr{Path: name, Required: required[name], Optional: !required[name] && !readOnly[name], Computed: readOnly[name], WriteOnly: writeOnly[name],
			Deprecated: deprecated[name], Replacement: repl[name], JSONPath: "properties." + name}
		var str string
		if json.Unmarshal(p.Type, &str) == nil {
			ent.Type = str
		} else {
			var buf bytes.Buffer
			if json.Compact(&buf, p.Type) == nil {
				ent.Type = buf.String()
			}
		}
		out = append(out, ent)
		delete(repl, name)
	}
	// What is left names a nested property of the schema. The importer carries whether the nested object requires
	// it when it resolved that object, and then the entry reads as a top-level one does. When it could not, the
	// entry states neither required nor optional.
	for path, r := range repl {
		ent := ContractAttr{Path: path, Replacement: r, JSONPath: field[path]}
		if req, ok := s.Nested[path]; ok {
			ent.Required, ent.Optional, ent.Computed = req, !req && !readOnly[path], readOnly[path]
		}
		out = append(out, ent)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
