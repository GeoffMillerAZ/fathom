package extractor

// docengine_predicates.go — K20 (wave c2): the document-path engine's value
// predicates (docs/design/language-breadth.md §4), universal (K15: no
// dialect's names in the engine) and opt-in, so a spec that sets none of them
// extracts byte for byte as before:
//
//   - where.not_has_key, the key-absent selector (O631): a step with a run and
//     no id;
//   - where.matches, an anchored RE2 over a selected scalar, compiled once at
//     load (an invalid one is refused there, never at extraction);
//   - the `kv:<KEY>` value transform, a k=v split of a string value
//     (OTEL_RESOURCE_ATTRIBUTES' service.name, a Compose `NAME=value`);
//   - an entity's refuse_templates: a value decided at deploy time yields no
//     entity (TL-5's never-guessed rule, held at extraction);
//   - an entity's unique: one entity per name and source kind in a file.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// docKVPrefix begins the k=v value transform (` kv:<KEY>`).
const docKVPrefix = "kv:"

// docTemplateMarkers begin an interpolation some deploy-time tool expands: a
// shell's, Compose's or the collector's `${`, a Helm or Go template's `{{`, a
// Kubernetes or shell `$(`.
var docTemplateMarkers = []string{"${", "{{", "$("}

// compileWhere compiles a where filter's value conditions: its equals, then
// its matches (each anchored ^(?:...)$), each in key order. An RE2 that does
// not compile, or a key that is not a selector, is refused here, at load.
func compileWhere(w DocWhere, where string) ([]docEquals, error) {
	out, err := compileEquals(w.Equals, where)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(w.Matches))
	for k := range w.Matches {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p, err := parseDocPath(k)
		if err != nil {
			return nil, fmt.Errorf("%s: where.matches %q: %w", where, k, err)
		}
		re, err := regexp.Compile(`^(?:` + w.Matches[k] + `)$`)
		if err != nil {
			return nil, fmt.Errorf("%s: where.matches %q: %w", where, k, err)
		}
		out = append(out, docEquals{p: p, re: re})
	}
	return out, nil
}

// holds reports whether a scalar satisfies the condition: equal to want, or,
// for a matches, matched whole by its RE2.
func (eq docEquals) holds(s string) bool {
	if eq.re != nil {
		return eq.re.MatchString(s)
	}
	return s == eq.want
}

// lacksKeys reports whether none of keys is an entry of n (the key-absent
// selector); a node that is not a map has no entries.
func lacksKeys(n *docNode, keys []string) bool {
	for _, k := range keys {
		if n.get(k) != nil {
			return false
		}
	}
	return true
}

// kvValue reads s as comma-separated key=value pairs and returns key's value,
// keys and values trimmed of spaces; "" when no pair names key. A comma
// separates two pairs only when the text after it holds an `=`, so a value may
// carry a comma (`APP=a,b`). `service.name=x,deployment.environment=y` gives x
// for service.name; a Compose list element `NAME=value` gives value for NAME.
func kvValue(s, key string) string {
	var pairs []string
	for _, seg := range strings.Split(s, ",") {
		if len(pairs) > 0 && !strings.Contains(seg, "=") {
			pairs[len(pairs)-1] += "," + seg
			continue
		}
		pairs = append(pairs, seg)
	}
	for _, p := range pairs {
		if k, v, ok := strings.Cut(p, "="); ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// isTemplated reports whether a value is decided at deploy time: it carries an
// interpolation (docTemplateMarkers), or the scalar it was read from carries a
// YAML local tag (`!Ref`, `!Sub`: an intrinsic a deploy-time tool computes).
func isTemplated(v string, src *docNode) bool {
	if src != nil && src.tag != "" {
		return true
	}
	for _, m := range docTemplateMarkers {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}

// yamlLocalTag is a YAML node's tag when it is a local one (`!Name`), the
// form an intrinsic is written in; "" for a standard tag (`!!str`) or none.
func yamlLocalTag(tag string) string {
	if len(tag) < 2 || tag[0] != '!' || tag[1] == '!' {
		return ""
	}
	return tag
}
