package iacfacts

import (
	"bytes"
	"encoding/json"
)

// tfJSONOver is what a .tf.json or .tofu.json override file names. The reader does not read the JSON syntax, but an
// override file changes what the other files declare, so the names it holds are enough to read those unknown: the
// resource attributes, variable defaults and locals it replaces.
type tfJSONOver struct {
	vars, locals []string
	res          []Resource
	// flags maps a variable to the sensitive and ephemeral flags the override sets, each true unless the literal
	// false. A flag the override does not name is absent, and the base's stays.
	flags map[string]map[string]bool
}

// jsonOverrideBody reads the keys of a resource or module override body into res: the meta-arguments, and every
// other key as an attribute or nested block that reads unknown. A `dynamic` key names the blocks it generates, so
// each label is an attribute of its own and the key itself is not.
func jsonOverrideBody(file string, bodies []map[string]json.RawMessage, res *Resource) {
	for _, body := range bodies {
		for _, k := range sortedKeys(body) {
			switch k {
			case "count":
				res.Multiplicity = Multiplicity{Kind: MultiCount, Class: ClassUnknown, Span: Span{File: file}}
			case "for_each":
				res.Multiplicity = Multiplicity{Kind: MultiForEach, Class: ClassUnknown, Span: Span{File: file}}
			case "provider":
				if s, ok := jsonString(body[k]); ok {
					res.Provider = s
				}
			case "dynamic":
				dyn, ok := jsonObjects(body[k])
				if !ok {
					res.Attrs = append(res.Attrs, Attr{Path: k, Class: ClassUnknown, Span: Span{File: file}})
					continue
				}
				for _, labels := range dyn {
					for _, label := range sortedKeys(labels) {
						res.Attrs = append(res.Attrs, Attr{Path: label, Class: ClassUnknown, Span: Span{File: file}})
					}
				}
			default:
				res.Attrs = append(res.Attrs, Attr{Path: k, Class: ClassUnknown, Span: Span{File: file}})
			}
		}
	}
}

// readJSONOverride reads the names an override file in the JSON syntax holds. It returns nil for a file that is not
// a JSON document of the shape OpenTofu reads, which the caller counts as a syntax error.
func readJSONOverride(file string, src []byte) *tfJSONOver {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(src, &top); err != nil {
		return nil
	}
	ov := &tfJSONOver{}
	for _, kind := range []struct{ key, mode string }{{"resource", ModeManaged}, {"data", ModeData}, {"ephemeral", ModeEphemeral}} {
		raw, ok := top[kind.key]
		if !ok {
			continue
		}
		byType, ok := jsonObjects(raw)
		if !ok {
			return nil
		}
		for _, types := range byType {
			for _, typ := range sortedKeys(types) {
				byName, ok := jsonObjects(types[typ])
				if !ok {
					return nil
				}
				for _, names := range byName {
					for _, name := range sortedKeys(names) {
						bodies, ok := jsonObjects(names[name])
						if !ok {
							return nil
						}
						res := Resource{Tool: ToolTerraform, Mode: kind.mode, Type: typ, Name: name, Multiplicity: Multiplicity{Kind: MultiSingle}}
						jsonOverrideBody(file, bodies, &res)
						ov.res = append(ov.res, res)
					}
				}
			}
		}
	}
	if raw, ok := top["module"]; ok {
		// A module call has one label, so its name is the first key.
		blocks, ok := jsonObjects(raw)
		if !ok {
			return nil
		}
		for _, byName := range blocks {
			for _, name := range sortedKeys(byName) {
				bodies, ok := jsonObjects(byName[name])
				if !ok {
					return nil
				}
				res := Resource{Tool: ToolTerraform, Mode: ModeModule, Type: "module", Name: name, Multiplicity: Multiplicity{Kind: MultiSingle}}
				jsonOverrideBody(file, bodies, &res)
				ov.res = append(ov.res, res)
			}
		}
	}
	if raw, ok := top["variable"]; ok {
		blocks, ok := jsonObjects(raw)
		if !ok {
			return nil
		}
		for _, byName := range blocks {
			for _, name := range sortedKeys(byName) {
				bodies, ok := jsonObjects(byName[name])
				if !ok {
					return nil
				}
				for _, body := range bodies {
					if _, has := body["default"]; has {
						ov.vars = append(ov.vars, name)
					}
					for _, flag := range []string{"sensitive", "ephemeral"} {
						if v, has := body[flag]; has {
							if ov.flags == nil {
								ov.flags = map[string]map[string]bool{}
							}
							if ov.flags[name] == nil {
								ov.flags[name] = map[string]bool{}
							}
							// Only the literal false clears the flag, as for HCL.
							ov.flags[name][flag] = string(bytes.TrimSpace(v)) != "false"
						}
					}
				}
			}
		}
	}
	if raw, ok := top["locals"]; ok {
		blocks, ok := jsonObjects(raw)
		if !ok {
			return nil
		}
		for _, one := range blocks {
			ov.locals = append(ov.locals, sortedKeys(one)...)
		}
	}
	return ov
}
