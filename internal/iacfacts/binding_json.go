package iacfacts

import (
	"encoding/json"
	"sort"
	"strings"
)

// isTFJSON reports whether a configuration file name is OpenTofu's JSON syntax (.tf.json or .tofu.json).
func isTFJSON(name string) bool {
	return strings.HasSuffix(name, ".tf.json") || strings.HasSuffix(name, ".tofu.json")
}

// jsonObjects decodes a JSON block value, which the configuration syntax writes as one object or as a list of them.
func jsonObjects(raw json.RawMessage) ([]map[string]json.RawMessage, bool) {
	var one map[string]json.RawMessage
	if err := json.Unmarshal(raw, &one); err == nil {
		return []map[string]json.RawMessage{one}, true
	}
	var many []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, true
	}
	return nil, false
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// jsonString decodes a JSON string that holds no interpolation. In the JSON syntax a string is a template, so one
// with `${` or `%{` is an expression and not a literal.
func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || strings.Contains(s, "${") || strings.Contains(s, "%{") {
		return "", false
	}
	return s, true
}

// jsonModule reads what binding needs from a .tf.json or .tofu.json file: required_providers entries and module
// calls, the way the HCL reader does. A file that is not valid JSON is unread.
func (r *bindingReader) jsonModule(dir, name string, src []byte, override bool, m *bindModule) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(src, &top); err != nil {
		r.skip("%s is not valid JSON and was skipped", name)
		return
	}
	if raw, ok := top["terraform"]; ok {
		blocks, ok := jsonObjects(raw)
		if !ok {
			r.skip("%s: terraform is not a block and was skipped", name)
		}
		for _, blk := range blocks {
			rp, ok := blk["required_providers"]
			if !ok {
				continue
			}
			sets, ok := jsonObjects(rp)
			if !ok {
				r.skip("%s: required_providers is not a block and was skipped", name)
			}
			for _, set := range sets {
				for _, prov := range sortedKeys(set) {
					m.required[prov] = r.readJSONRequired(name, prov, set[prov])
				}
			}
		}
	}
	if raw, ok := top["module"]; ok {
		calls, ok := jsonObjects(raw)
		if !ok {
			r.skip("%s: module is not a block and was skipped", name)
		}
		for _, byName := range calls {
			for _, label := range sortedKeys(byName) {
				bodies, ok := jsonObjects(byName[label])
				if !ok {
					r.skip("%s: module %q is not a block and was skipped", name, label)
					continue
				}
				for _, body := range bodies {
					if !override {
						m.calls = append(m.calls, readJSONModuleCall(dir, label, body))
						continue
					}
					// An override is merged over the call by OpenTofu and not by this reader.
					_, hasSource := body["source"]
					_, hasProviders := body["providers"]
					if hasSource || hasProviders {
						r.skipOverrideModule(name, label)
					}
				}
			}
		}
	}
}

// readJSONRequired reads one required_providers entry: a constraint string (the legacy form) or an object.
// An entry, or a source or version in one, that is not a literal string refuses the answer, as in HCL.
func (r *bindingReader) readJSONRequired(file, prov string, raw json.RawMessage) bindRequired {
	if s, ok := jsonString(raw); ok {
		return bindRequired{constraint: s}
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		r.skip("%s: required_providers entry %q is not a literal string or object and was not read", file, prov)
		return bindRequired{}
	}
	var req bindRequired
	for _, key := range []string{"source", "version"} {
		v, present := obj[key]
		if !present {
			continue
		}
		s, ok := jsonString(v)
		if !ok {
			r.skip("%s: the %s of required_providers entry %q is not a literal string and was not read", file, key, prov)
			continue
		}
		if key == "source" {
			req.source = s
		} else {
			req.constraint = s
		}
	}
	return req
}

func readJSONModuleCall(dir, label string, body map[string]json.RawMessage) bindCall {
	c := bindCall{name: label, providers: map[string]string{}}
	if raw, ok := body["source"]; ok {
		if s, ok := jsonString(raw); !ok {
			c.dynamic = true
		} else {
			c.dir = localModuleDir(dir, s)
		}
	}
	if raw, ok := body["providers"]; ok {
		var provs map[string]string
		if json.Unmarshal(raw, &provs) == nil {
			for k, v := range provs {
				if k != "" && v != "" {
					c.providers[k] = v
				}
			}
		}
	}
	return c
}
