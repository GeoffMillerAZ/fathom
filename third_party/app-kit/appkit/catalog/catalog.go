// Package catalog supplies the shared, CUE-validated first-party capability
// records used by the Gallery and kit guidance. It needs no licensed corpus.
package catalog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema.cue
var schemaSource []byte

//go:embed catalog.cue
var defaultSource []byte

// Source locates the implementation; it never embeds a licensed source body.
type Source struct {
	Path    string   `json:"path"`
	Symbols []string `json:"symbols"`
}

// Provenance identifies an entry's origin and its permitted audience.
type Provenance struct {
	Source   string `json:"source"`
	License  string `json:"license"`
	Audience string `json:"audience"`
}

// Entry is an exact capability contract. Proof and Gaps distinguish existing
// implementation from unverified behavior or unavailable broader workflows.
type Entry struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	Summary      string     `json:"summary"`
	Tasks        []string   `json:"tasks"`
	Layers       []string   `json:"layers"`
	Profiles     []string   `json:"profiles"`
	Dependencies []string   `json:"dependencies"`
	Source       Source     `json:"source"`
	Usage        string     `json:"usage"`
	States       []string   `json:"states"`
	AppOwns      []string   `json:"app_owns"`
	Maturity     string     `json:"maturity"`
	Proof        []string   `json:"proof"`
	Gaps         []string   `json:"gaps"`
	Provenance   Provenance `json:"provenance"`
	Consumers    []string   `json:"consumers"`
	Preview      string     `json:"preview"`
}

// Catalog is a validated snapshot. Exported fields are detached copies for
// inspection/serialization; Get and Search always use the original snapshot.
// Construct catalogs with Load or Default, not a struct literal.
type Catalog struct {
	SchemaVersion int     `json:"schema_version"`
	Revision      string  `json:"revision"`
	Entries       []Entry `json:"entries"`
	revision      string
	entries       []Entry
}

// Query selects compatible entries. Filters are exact, case-sensitive enum
// values; valid filters that have no common entry return an empty page.
type Query struct {
	Text     string `json:"text,omitempty"`
	Layer    string `json:"layer,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Maturity string `json:"maturity,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

// Summary is the bounded discovery projection. Get supplies the full contract.
type Summary struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Summary  string   `json:"summary"`
	Layers   []string `json:"layers"`
	Profiles []string `json:"profiles"`
	Maturity string   `json:"maturity"`
	Preview  string   `json:"preview"`
}

// Page carries a revision-bound continuation and the total matching count.
type Page struct {
	Revision   string    `json:"revision"`
	Items      []Summary `json:"items"`
	NextCursor string    `json:"next_cursor"`
	Total      int       `json:"total"`
}

// Load validates a top-level schema_version/entries CUE value. A revision is
// SHA-256 of its canonical JSON, with entries sorted by stable ID. CUE source
// formatting and record order do not change it; contract/evidence changes do.
func Load(src []byte) (*Catalog, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(schemaSource)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("catalog: compile embedded schema: %w", err)
	}
	value := ctx.CompileBytes(src)
	if err := value.Err(); err != nil {
		return nil, fmt.Errorf("catalog: compile source: %w", err)
	}
	value = schema.LookupPath(cue.ParsePath("#Catalog")).Unify(value)
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("catalog: source does not satisfy schema: %w", err)
	}
	var data struct {
		SchemaVersion int     `json:"schema_version"`
		Entries       []Entry `json:"entries"`
	}
	if err := value.Decode(&data); err != nil {
		return nil, fmt.Errorf("catalog: decode: %w", err)
	}
	sort.Slice(data.Entries, func(i, j int) bool { return data.Entries[i].ID < data.Entries[j].ID })
	for _, entry := range data.Entries {
		if len(entry.Layers) == 0 && !slices.Contains(entry.Profiles, "cli") {
			return nil, fmt.Errorf("catalog: entry %q requires a layer unless its profile is cli", entry.ID)
		}
	}
	for i := 1; i < len(data.Entries); i++ {
		if data.Entries[i-1].ID == data.Entries[i].ID {
			return nil, fmt.Errorf("catalog: duplicate entry ID %q", data.Entries[i].ID)
		}
	}
	canonical, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("catalog: encode canonical source: %w", err)
	}
	revision := digest(canonical)
	c := &Catalog{SchemaVersion: data.SchemaVersion, Revision: revision, revision: revision, entries: data.Entries}
	c.Entries = make([]Entry, len(data.Entries))
	for i, e := range data.Entries {
		c.Entries[i] = cloneEntry(e)
	}
	return c, nil
}

// Default loads the embedded first-party catalog into an independent snapshot.
func Default() (*Catalog, error) { return Load(defaultSource) }

// Get returns a detached exact contract for a stable ID.
func (c *Catalog) Get(id string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	i, ok := slices.BinarySearchFunc(c.entries, id, func(e Entry, id string) int { return strings.Compare(e.ID, id) })
	if !ok {
		return Entry{}, false
	}
	return cloneEntry(c.entries[i]), true
}

// Search requires every normalized query token to match the ID/name or one
// declared task phrase. Explanatory prose and unrelated tasks cannot be combined
// into a capability claim. Summary, usage, gaps and proof are not search fields.
// Proven recipes rank before other native entries, which rank before references;
// within each group, weighted token matches rank first and equal scores use ID.
// Limits default to 10 and must be 1..50. A cursor is valid only for the same
// catalog revision and normalized query, including the effective limit.
func (c *Catalog) Search(q Query) (Page, error) {
	if c == nil || c.revision == "" {
		return Page{}, fmt.Errorf("catalog: uninitialized catalog")
	}
	cursor := q.Cursor
	q, terms, err := normalizeQuery(q)
	if err != nil {
		return Page{}, err
	}
	encodedQuery, err := json.Marshal(q)
	if err != nil {
		return Page{}, fmt.Errorf("catalog: encode query: %w", err)
	}
	queryDigest := digest(encodedQuery)
	type match struct {
		entry Entry
		score int
		rank  int
	}
	matches := make([]match, 0)
	for _, e := range c.entries {
		if q.Layer != "" && !slices.Contains(e.Layers, q.Layer) ||
			q.Profile != "" && !slices.Contains(e.Profiles, q.Profile) ||
			q.Kind != "" && e.Kind != q.Kind || q.Maturity != "" && e.Maturity != q.Maturity {
			continue
		}
		score, ok := textScore(e, terms)
		if !ok {
			continue
		}
		rank := 1
		if e.Kind == "reference" {
			rank = 0
		} else if e.Kind == "recipe" && e.Maturity == "proven" {
			rank = 2
		}
		matches = append(matches, match{e, score, rank})
	}
	sort.Slice(matches, func(i, j int) bool {
		if len(terms) > 0 && matches[i].rank != matches[j].rank {
			return matches[i].rank > matches[j].rank
		}
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].entry.ID < matches[j].entry.ID
	})
	start := 0
	if cursor != "" {
		start, err = decodeCursor(cursor, c.revision, queryDigest, len(matches), q.Limit)
		if err != nil {
			return Page{}, err
		}
	}
	page := Page{Revision: c.revision, Items: make([]Summary, 0), Total: len(matches)}
	end := min(start+q.Limit, len(matches))
	for _, m := range matches[start:end] {
		e := m.entry
		page.Items = append(page.Items, Summary{e.ID, e.Name, e.Kind, e.Summary, slices.Clone(e.Layers), slices.Clone(e.Profiles), e.Maturity, e.Preview})
	}
	if end < len(matches) {
		encoded, err := json.Marshal(continuation{Version: searchCursorVersion, Revision: c.revision, Query: queryDigest, Offset: end})
		if err != nil {
			return Page{}, fmt.Errorf("catalog: encode cursor: %w", err)
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return page, nil
}

// Search semantics affect pagination even when a custom catalog is unchanged.
const searchCursorVersion = 2

type continuation struct {
	Version  int    `json:"v"`
	Revision string `json:"revision"`
	Query    string `json:"query"`
	Offset   int    `json:"offset"`
}

func decodeCursor(raw, revision, query string, total, limit int) (int, error) {
	if len(raw) > 2048 {
		return 0, fmt.Errorf("catalog: malformed cursor")
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return 0, fmt.Errorf("catalog: malformed cursor: %w", err)
	}
	var cur continuation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cur); err != nil {
		return 0, fmt.Errorf("catalog: malformed cursor: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF || cur.Revision == "" || cur.Query == "" {
		return 0, fmt.Errorf("catalog: malformed cursor")
	}
	if cur.Version != searchCursorVersion {
		return 0, fmt.Errorf("catalog: cursor search version changed; restart search")
	}
	if cur.Revision != revision {
		return 0, fmt.Errorf("catalog: stale cursor revision; restart search")
	}
	if cur.Query != query {
		return 0, fmt.Errorf("catalog: cursor query changed; restart search")
	}
	if cur.Offset <= 0 || cur.Offset >= total || cur.Offset%limit != 0 {
		return 0, fmt.Errorf("catalog: cursor offset out of range")
	}
	return cur.Offset, nil
}

func normalizeQuery(q Query) (Query, []string, error) {
	filters := []struct {
		name, value string
		allowed     []string
	}{
		{"layer", q.Layer, []string{"L0", "L1", "L2", "L3a", "L3b", "L4"}},
		{"profile", q.Profile, []string{"portable", "go-web", "static-web", "react-web", "cli"}},
		{"kind", q.Kind, []string{"token", "primitive", "recipe", "reference"}},
		{"maturity", q.Maturity, []string{"experimental", "available", "proven"}},
	}
	for _, f := range filters {
		if f.value != "" && !slices.Contains(f.allowed, f.value) {
			return Query{}, nil, fmt.Errorf("catalog: unknown %s %q (expected %s)", f.name, f.value, strings.Join(f.allowed, ", "))
		}
	}
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit < 1 || q.Limit > 50 {
		return Query{}, nil, fmt.Errorf("catalog: limit must be between 1 and 50")
	}
	terms := tokenize(q.Text)
	slices.Sort(terms)
	terms = slices.Compact(terms)
	q.Text = strings.Join(terms, " ")
	q.Cursor = ""
	return q, terms, nil
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func textScore(e Entry, terms []string) (int, bool) {
	type field struct {
		text   string
		weight int
	}
	fields := []field{{e.ID + " " + e.Name, 8}}
	for _, task := range e.Tasks {
		fields = append(fields, field{task, 5})
	}
	best, found := 0, false
	for _, f := range fields {
		tokens := tokenize(f.text)
		score, complete := 0, true
		for _, term := range terms {
			matched := 0
			for _, token := range tokens {
				if token == term {
					matched = max(matched, f.weight*2)
				} else if len(term) >= 3 && strings.HasPrefix(token, term) {
					matched = max(matched, f.weight)
				}
			}
			if matched == 0 {
				complete = false
				break
			}
			score += matched
		}
		if complete {
			best, found = max(best, score), true
		}
	}
	return best, found
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneEntry(e Entry) Entry {
	e.Tasks = slices.Clone(e.Tasks)
	e.Layers = slices.Clone(e.Layers)
	e.Profiles = slices.Clone(e.Profiles)
	e.Dependencies = slices.Clone(e.Dependencies)
	e.Source.Symbols = slices.Clone(e.Source.Symbols)
	e.States = slices.Clone(e.States)
	e.AppOwns = slices.Clone(e.AppOwns)
	e.Proof = slices.Clone(e.Proof)
	e.Gaps = slices.Clone(e.Gaps)
	e.Consumers = slices.Clone(e.Consumers)
	return e
}
