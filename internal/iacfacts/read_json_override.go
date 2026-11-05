package iacfacts

import "encoding/json"

// tfJSONOver is what a .tf.json or .tofu.json override file names. The reader does not read the JSON syntax, but an
// override file changes what the other files declare, so the names it holds are enough to read those unknown: the
// resource attributes, variable defaults and locals it replaces.
type tfJSONOver struct {
	vars, locals []string
	res          []Resource
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
								default:
									res.Attrs = append(res.Attrs, Attr{Path: k, Class: ClassUnknown, Span: Span{File: file}})
								}
							}
						}
						ov.res = append(ov.res, res)
					}
				}
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
