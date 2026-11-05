package iacfacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

// maxJSONDupDepth bounds the walk that looks for a repeated key.
const maxJSONDupDepth = 200

type jsonMember struct {
	key string
	raw json.RawMessage
}

// jsonTopMembers decodes the top-level object of a JSON configuration file in file order, keeping every occurrence
// of a repeated key. OpenTofu reads a repeated top-level `module` key as two blocks, which a Go map cannot hold.
func jsonTopMembers(src []byte) ([]jsonMember, error) {
	dec := json.NewDecoder(bytes.NewReader(src))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	var out []jsonMember
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, errors.New("not a JSON object")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		out = append(out, jsonMember{key, raw})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("data after the top-level object")
	}
	return out, nil
}

// jsonHasRepeatedKey reports whether any object inside raw repeats a key, or raw nests past the bound. OpenTofu's
// reading of a repeated key depends on where it sits (the last version wins, a repeated required_providers entry is
// an error), so the reader refuses it and does not choose.
func jsonHasRepeatedKey(raw json.RawMessage) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func(depth int) bool
	walk = func(depth int) bool {
		if depth > maxJSONDupDepth {
			return true
		}
		tok, err := dec.Token()
		if err != nil {
			return true
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return false
		}
		switch d {
		case '[':
			for dec.More() {
				if walk(depth + 1) {
					return true
				}
			}
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return true
				}
				k, _ := kt.(string)
				if seen[k] {
					return true
				}
				seen[k] = true
				if walk(depth + 1) {
					return true
				}
			}
		}
		_, err = dec.Token()
		return err != nil
	}
	return walk(0)
}

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
	members, err := jsonTopMembers(src)
	if err != nil {
		r.skip("%s is not valid JSON and was skipped", name)
		return
	}
	// Every block key but `module` must appear once: OpenTofu refuses a repeated `terraform` with the same provider
	// and the reader has no way to tell which repeats it merges. A repeated `module` is read as both blocks.
	byKey := map[string][]json.RawMessage{}
	for _, mem := range members {
		switch mem.key {
		case "terraform", "module", "provider":
			byKey[mem.key] = append(byKey[mem.key], mem.raw)
		}
	}
	for _, key := range []string{"terraform", "provider"} {
		if len(byKey[key]) > 1 {
			r.skip("%s repeats the key %q, which OpenTofu reads in a way this reader does not model, so it was skipped", name, key)
			byKey[key] = nil
		}
	}
	for _, key := range []string{"terraform", "module", "provider"} {
		for _, raw := range byKey[key] {
			if jsonHasRepeatedKey(raw) {
				r.skip("%s repeats a key inside %q, which OpenTofu reads in a way this reader does not model, so it was skipped", name, key)
				byKey[key] = nil
				break
			}
		}
	}
	for _, raw := range byKey["terraform"] {
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
	for _, raw := range byKey["provider"] {
		sets, ok := jsonObjects(raw)
		if !ok {
			r.skip("%s: provider is not a block and was skipped", name)
		}
		for _, set := range sets {
			for _, prov := range sortedKeys(set) {
				bodies, ok := jsonObjects(set[prov])
				if !ok {
					r.skip("%s: provider %q is not a block and was skipped", name, prov)
					continue
				}
				for _, body := range bodies {
					v, present := body["version"]
					if !present {
						continue
					}
					if override {
						r.skip("%s sets the version of provider %q in an override file, which this reader does not merge, so the constraint is not known", name, prov)
						continue
					}
					if s, ok := jsonString(v); ok && strings.TrimSpace(s) != "" {
						m.addProviderVersion(prov, s)
					} else {
						r.skip("%s: the version of provider %q is not a non-empty literal string, which OpenTofu evaluates or rejects and this reader does not", name, prov)
					}
				}
			}
		}
	}
	for _, raw := range byKey["module"] {
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
		if strings.TrimSpace(s) == "" {
			r.skip("%s: required_providers entry %q has an empty version constraint, which OpenTofu rejects", file, prov)
			return bindRequired{}
		}
		return bindRequired{constraint: s}
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		r.skip("%s: required_providers entry %q is not a literal string or object and was not read", file, prov)
		return bindRequired{}
	}
	var req bindRequired
	for _, key := range sortedKeys(obj) {
		switch key {
		case "source", "version":
		case "configuration_aliases":
			continue
		default:
			r.skip("%s: required_providers entry %q has the key %q, which OpenTofu rejects", file, prov, key)
			continue
		}
		v := obj[key]
		s, ok := jsonString(v)
		if !ok || strings.TrimSpace(s) == "" {
			r.skip("%s: the %s of required_providers entry %q is not a non-empty literal string and was not read", file, key, prov)
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
