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
// adds an entry at Tags[*].Key. Nested properties otherwise are not in the pruned schema.

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
	// What is left names a nested property of the schema. The pruned schema keeps no nested required list, so the
	// entry states neither required nor optional.
	for path, r := range repl {
		out = append(out, ContractAttr{Path: path, Replacement: r, JSONPath: field[path]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
