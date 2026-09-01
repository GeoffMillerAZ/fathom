// Package normalize turns raw parsed components into typed, validated records:
// it derives the fields the dump omits (framework from language, full_name
// from the taxonomy path), maps Ecommerce's null mode to "none", content-
// addresses each code blob, and builds the text that later gets embedded.
package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/app-kit/internal/twplus/parse"
)

// Variant is one variant, shaped to validate against the CUE #Variant (json
// tags match the schema field names exactly — the struct is closed there).
type Variant struct {
	FullName         string `json:"full_name"`
	Name             string `json:"name"`
	Code             string `json:"code"`
	Language         string `json:"language"`
	Framework        string `json:"framework"`
	TailwindVersion  int    `json:"tailwind_version"`
	Mode             string `json:"mode"`
	SupportsDarkMode bool   `json:"supportsDarkMode"`
}

// Component is the validated typed form (matches CUE #Component).
type Component struct {
	Section  string    `json:"section"`
	Group    string    `json:"group"`
	Category string    `json:"category"`
	Name     string    `json:"name"`
	FullName string    `json:"full_name"`
	Variants []Variant `json:"variants"`
}

// VariantRow is the denormalized, index-ready row for one variant: taxonomy +
// axes + content-addressed code + the enrichment used for retrieval.
type VariantRow struct {
	Section, Group, Category, Name, FullName string
	Framework, Language                      string
	TailwindVersion                          int
	Mode                                     string
	SupportsDarkMode                         bool
	ContentHash                              string // hex sha256 of Code
	Code                                     string
	Classes                                  string // ranked, deduped, capped
	EmbedText                                string // taxonomy + axes + classes
}

// Build converts a parsed component into its typed Component plus index rows.
func Build(c parse.Component) (Component, []VariantRow, error) {
	section, group, category, name := taxonomy(c.Path)
	full := c.FullName()

	comp := Component{
		Section:  section,
		Group:    group,
		Category: category,
		Name:     name,
		FullName: full,
	}
	rows := make([]VariantRow, 0, len(c.Raw.Snippets))

	for i, s := range c.Raw.Snippets {
		fw, err := framework(s.Name)
		if err != nil {
			return Component{}, nil, fmt.Errorf("%s: snippet %d: %w", full, i, err)
		}
		mode := "none"
		if s.Mode != nil {
			mode = *s.Mode
		}
		v := Variant{
			FullName:         full,
			Name:             name,
			Code:             s.Code,
			Language:         s.Language,
			Framework:        fw,
			TailwindVersion:  s.Version,
			Mode:             mode,
			SupportsDarkMode: s.SupportsDarkMode,
		}
		comp.Variants = append(comp.Variants, v)

		classes := extractClasses(s.Code)
		sum := sha256.Sum256([]byte(s.Code))
		rows = append(rows, VariantRow{
			Section:          section,
			Group:            group,
			Category:         category,
			Name:             name,
			FullName:         full,
			Framework:        fw,
			Language:         s.Language,
			TailwindVersion:  s.Version,
			Mode:             mode,
			SupportsDarkMode: s.SupportsDarkMode,
			ContentHash:      hex.EncodeToString(sum[:]),
			Code:             s.Code,
			Classes:          classes,
			EmbedText:        embedText(section, group, category, name, fw, s.Version, mode, s.SupportsDarkMode, classes),
		})
	}
	return comp, rows, nil
}

// taxonomy splits the path [section, group, category, name]. It is tolerant of
// shorter paths so a malformed node fails in CUE validation (empty group/
// category) with a locatable name rather than panicking here.
func taxonomy(path []string) (section, group, category, name string) {
	get := func(i int) string {
		if i < len(path) {
			return path[i]
		}
		return ""
	}
	section, group = get(0), get(1)
	if len(path) > 0 {
		name = path[len(path)-1]
	}
	if len(path) >= 4 {
		// category is everything between group and the leaf name, joined.
		category = strings.Join(path[2:len(path)-1], ".")
	}
	// A shorter path leaves category "", which #Component's `category: !=""`
	// rejects with a locatable full_name — the documented intent.
	return
}

// framework is the dump's per-snippet `name` field, which (despite the field
// name) carries the actual framework: "html" | "vue" | "react". The sibling
// `language` field is only the syntax — "html" for both html and vue
// templates, "jsx" for react — so the framework cannot be derived from it.
func framework(snippetName string) (string, error) {
	switch snippetName {
	case "html", "vue", "react":
		return snippetName, nil
	default:
		return "", fmt.Errorf("unexpected framework %q (expected html|vue|react)", snippetName)
	}
}

var classAttr = regexp.MustCompile(`(?:class|className|:class)\s*=\s*(?:"([^"]*)"|'([^']*)'|` + "`" + `([^` + "`" + `]*)` + "`" + `)`)

// extractClasses pulls Tailwind utility classes out of code, ranks them by
// frequency (distinctive utilities rise above boilerplate like flex/p-4),
// dedupes, and caps the result so embed-text stays compact.
func extractClasses(code string) string {
	const cap = 120
	freq := map[string]int{}
	for _, m := range classAttr.FindAllStringSubmatch(code, -1) {
		val := m[1] + m[2] + m[3]
		for _, tok := range strings.Fields(val) {
			tok = strings.TrimSpace(tok)
			// Drop template-expression noise from JSX/Vue dynamic class binds.
			if tok == "" || strings.ContainsAny(tok, "{}$()") {
				continue
			}
			freq[tok]++
		}
	}
	if len(freq) == 0 {
		return ""
	}
	toks := make([]string, 0, len(freq))
	for t := range freq {
		toks = append(toks, t)
	}
	sort.Slice(toks, func(i, j int) bool {
		if freq[toks[i]] != freq[toks[j]] {
			return freq[toks[i]] > freq[toks[j]]
		}
		return toks[i] < toks[j]
	})
	if len(toks) > cap {
		toks = toks[:cap]
	}
	return strings.Join(toks, " ")
}

func embedText(section, group, category, name, framework string, tw int, mode string, dark bool, classes string) string {
	var b strings.Builder
	parts := []string{section, group, category, name}
	nonEmpty := parts[:0]
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	b.WriteString(strings.Join(nonEmpty, " > "))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s · tailwind v%d · %s", framework, tw, mode)
	if dark {
		b.WriteString(" · supports dark mode")
	}
	if classes != "" {
		b.WriteString("\nclasses: ")
		b.WriteString(classes)
	}
	return b.String()
}
