// Package parse streams the raw 336MB TailwindPlus dump into per-component
// records. The dump nests section → group → category → component, where a
// component node carries a "snippets" array of variant payloads. Previews
// (258MB of the 336MB) are dropped on decode: RawSnippet has no preview
// field, so json discards those bytes instead of retaining them.
package parse

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// RawSnippet is one variant payload as the dump emits it. Preview is the full
// inline HTML document (258MB of the 336MB dump across all snippets); the main
// pipeline (normalize → index) never reads it, so it stays out of the corpus
// index — only the optional `appkit data previews` builder consumes it.
type RawSnippet struct {
	Code             string  `json:"code"`
	Language         string  `json:"language"` // "html" | "jsx"
	Name             string  `json:"name"`     // the framework: "html" | "vue" | "react"
	Mode             *string `json:"mode"`     // null for Ecommerce
	Version          int     `json:"version"`  // Tailwind major: 3 | 4
	SupportsDarkMode bool    `json:"supportsDarkMode"`
	Preview          string  `json:"preview"` // full HTML doc; consumed only by the previews builder
}

// RawComponent is a leaf node: a display name plus its variant snippets.
type RawComponent struct {
	Name     string       `json:"name"`
	Snippets []RawSnippet `json:"snippets"`
}

// Component pairs a raw component with its taxonomy path. For this dump the
// path is always [section, group, category, name].
type Component struct {
	Path []string
	Raw  RawComponent
}

// FullName is the dotted taxonomy, e.g. "Application UI.Elements.Badges.Flat".
func (c Component) FullName() string { return strings.Join(c.Path, ".") }

// Walk decodes the dump and invokes fn once per component, in deterministic
// (lexical path) order. fn returning an error aborts the walk.
func Walk(r io.Reader, fn func(Component) error) error {
	var top struct {
		Tailwindplus map[string]json.RawMessage `json:"tailwindplus"`
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&top); err != nil {
		return fmt.Errorf("twplus/parse: decode top level: %w", err)
	}
	if len(top.Tailwindplus) == 0 {
		return fmt.Errorf("twplus/parse: no \"tailwindplus\" object in dump")
	}
	sections := sortedKeys(top.Tailwindplus)
	for _, s := range sections {
		raw := top.Tailwindplus[s]
		top.Tailwindplus[s] = nil // release as we go
		if err := walkNode([]string{s}, raw, fn); err != nil {
			return err
		}
	}
	return nil
}

func walkNode(path []string, raw json.RawMessage, fn func(Component) error) error {
	// A component node has a non-empty "snippets" array.
	var probe struct {
		Snippets json.RawMessage `json:"snippets"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil && len(probe.Snippets) > 0 {
		var rc RawComponent
		if err := json.Unmarshal(raw, &rc); err != nil {
			return fmt.Errorf("twplus/parse: component %q: %w", strings.Join(path, "."), err)
		}
		return fn(Component{Path: path, Raw: rc})
	}
	// Interior node: a map of children.
	var children map[string]json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil {
		return fmt.Errorf("twplus/parse: node %q: %w", strings.Join(path, "."), err)
	}
	for _, k := range sortedKeys(children) {
		next := append(append([]string(nil), path...), k)
		if err := walkNode(next, children[k], fn); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
