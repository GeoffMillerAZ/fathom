// Package designattempts keeps private syntax-attempt metadata separate from
// original design archives and from any future runtime-compatibility evidence.
package designattempts

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

const (
	MaxCatalogBytes    = 16 << 20
	MaxEntries         = 4096
	maxSourcePathBytes = 1024
	maxArchiveIDBytes  = 256
	maxCursorBytes     = 2048
)

//go:embed schema.cue
var schemaSource []byte

type Entry struct {
	ID                 string                    `json:"id"`
	Source             designconvert.Source      `json:"source"`
	Target             string                    `json:"target"`
	Transformer        designconvert.Transformer `json:"transformer"`
	Status             string                    `json:"status"`
	Verification       string                    `json:"verification"`
	ArtifactSHA256     string                    `json:"artifact_sha256,omitempty"`
	DiagnosticCodes    []string                  `json:"diagnostic_codes"`
	OmittedDiagnostics int                       `json:"omitted_diagnostics"`
}

// Catalog is a bounded metadata snapshot, not fresh artifact or archive proof.
// Exported fields are detached views; methods use the immutable internal copy.
type Catalog struct {
	SchemaVersion int     `json:"schema_version"`
	Revision      string  `json:"revision"`
	Entries       []Entry `json:"entries"`
	entries       []Entry
	revision      string
}

type Query struct {
	ArchiveID string `json:"archive_id"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Cursor    string `json:"cursor"`
	Limit     int    `json:"limit"`
}

type Page struct {
	Revision   string  `json:"revision"`
	Total      int     `json:"total"`
	Entries    []Entry `json:"entries"`
	NextCursor string  `json:"next_cursor"`
}

func (c *Catalog) List() []Entry {
	if c == nil {
		return []Entry{}
	}
	return cloneEntries(c.entries)
}

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

// Search uses exact filters and stable ID ordering. Neither matching a current
// transformer nor finding an index entry asserts fresh replay or compatibility.
func (c *Catalog) Search(q Query) (Page, error) {
	if c == nil || c.revision == "" {
		return Page{}, fmt.Errorf("designattempts: uninitialized catalog")
	}
	cursor := q.Cursor
	q.Cursor = ""
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit < 1 || q.Limit > 50 {
		return Page{}, fmt.Errorf("designattempts: limit must be between 1 and 50")
	}
	if q.ArchiveID != "" && !validArchiveID(q.ArchiveID) {
		return Page{}, fmt.Errorf("designattempts: invalid archive filter")
	}
	if q.Path != "" && !validSourcePath(q.Path) {
		return Page{}, fmt.Errorf("designattempts: invalid member path filter")
	}
	if q.Status != "" && q.Status != "translated" && q.Status != "unsupported" {
		return Page{}, fmt.Errorf("designattempts: status must be translated or unsupported")
	}
	if len(cursor) > maxCursorBytes {
		return Page{}, fmt.Errorf("designattempts: cursor exceeds limit")
	}
	queryBody, _ := json.Marshal(q)
	queryHash := digest(queryBody)
	matched := make([]Entry, 0)
	for _, e := range c.entries {
		if q.ArchiveID != "" && q.ArchiveID != e.Source.ArchiveID || q.Path != "" && q.Path != e.Source.Path || q.Status != "" && q.Status != e.Status {
			continue
		}
		matched = append(matched, e)
	}
	offset := 0
	if cursor != "" {
		var err error
		offset, err = decodeCursor(cursor, c.revision, queryHash, len(matched), q.Limit)
		if err != nil {
			return Page{}, err
		}
	}
	end := min(offset+q.Limit, len(matched))
	page := Page{Revision: c.revision, Total: len(matched), Entries: cloneEntries(matched[offset:end])}
	if end < len(matched) {
		body, _ := json.Marshal(continuation{1, c.revision, queryHash, end})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(body)
	}
	return page, nil
}

type continuation struct {
	Version  int    `json:"v"`
	Revision string `json:"revision"`
	Query    string `json:"query"`
	Offset   int    `json:"offset"`
}

func decodeCursor(raw, revision, query string, total, limit int) (int, error) {
	body, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(body) != raw {
		return 0, fmt.Errorf("designattempts: malformed cursor")
	}
	var c continuation
	if err := json.Unmarshal(body, &c); err != nil {
		return 0, fmt.Errorf("designattempts: malformed cursor")
	}
	canonical, _ := json.Marshal(c)
	if !bytes.Equal(body, canonical) || c.Version != 1 || !hashPattern.MatchString(c.Revision) || !hashPattern.MatchString(c.Query) {
		return 0, fmt.Errorf("designattempts: malformed cursor")
	}
	if c.Revision != revision {
		return 0, fmt.Errorf("designattempts: stale cursor revision; restart search")
	}
	if c.Query != query {
		return 0, fmt.Errorf("designattempts: cursor query changed; restart search")
	}
	if c.Offset <= 0 || c.Offset >= total || c.Offset%limit != 0 {
		return 0, fmt.Errorf("designattempts: cursor offset out of range")
	}
	return c.Offset, nil
}

// entryFor assumes the caller already verified original membership, canonical
// receipt encoding and current replay. The complete receipt, including lineage
// and unsupported diagnostics, determines identity; artifact bytes alone do not.
func entryFor(result designconvert.Translation, canonicalBody []byte) Entry {
	e := Entry{ID: "syntax@" + digest(canonicalBody), Source: result.Source, Target: result.Target,
		Transformer: result.Transformer, Status: result.Status, Verification: result.Verification,
		DiagnosticCodes: []string{}, OmittedDiagnostics: result.OmittedDiagnostics}
	if result.Artifact != nil {
		e.ArtifactSHA256 = result.Artifact.SHA256
	}
	for _, d := range result.Diagnostics {
		e.DiagnosticCodes = append(e.DiagnosticCodes, d.Code)
	}
	return e
}

func encodeCatalog(entries []Entry) ([]byte, error) {
	if len(entries) > MaxEntries {
		return nil, fmt.Errorf("designattempts: catalog exceeds entry limit")
	}
	entries = cloneEntries(entries)
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.ID, b.ID) })
	if err := validateEntries(entries); err != nil {
		return nil, err
	}
	revision, err := catalogRevision(entries)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(Catalog{SchemaVersion: 1, Revision: revision, Entries: entries})
	if err != nil {
		return nil, err
	}
	if len(body) > MaxCatalogBytes {
		return nil, fmt.Errorf("designattempts: catalog exceeds byte limit")
	}
	if _, err := decodeCatalog(body); err != nil {
		return nil, err
	}
	return body, nil
}

// decodeCatalog accepts only canonical JSON produced by encodeCatalog. Before
// typed decoding/CUE, a streaming pass bounds arrays, object fields and depth so
// a short array of millions of empty entries cannot cause unbounded allocation.
func decodeCatalog(body []byte) (*Catalog, error) {
	if len(body) > MaxCatalogBytes {
		return nil, fmt.Errorf("designattempts: catalog exceeds byte limit")
	}
	if !utf8.Valid(body) || len(body) == 0 {
		return nil, fmt.Errorf("designattempts: catalog must be UTF-8 JSON")
	}
	if err := preflightJSON(body); err != nil {
		return nil, err
	}
	var c Catalog
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("designattempts: invalid catalog JSON fields")
	}
	if len(c.Revision) != 64 || !hashPattern.MatchString(c.Revision) {
		return nil, fmt.Errorf("designattempts: invalid catalog revision")
	}
	if err := validateEntries(c.Entries); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(c)
	if err != nil || !bytes.Equal(body, canonical) {
		return nil, fmt.Errorf("designattempts: catalog is not canonical JSON")
	}
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(schemaSource)
	value := ctx.CompileBytes(body).Unify(schema.LookupPath(cue.ParsePath("#Catalog")))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("designattempts: catalog schema validation failed")
	}
	revision, err := catalogRevision(c.Entries)
	if err != nil {
		return nil, err
	}
	if c.Revision != revision {
		return nil, fmt.Errorf("designattempts: catalog revision mismatch")
	}
	c.entries, c.revision = cloneEntries(c.Entries), c.Revision
	return &c, nil
}

func catalogRevision(entries []Entry) (string, error) {
	// Bound aggregate JSON escaping before marshaling a whole caller-supplied
	// index. Each entry was individually bounded by validateEntries first.
	var body bytes.Buffer
	body.WriteString(`{"schema_version":1,"entries":[`)
	for i, e := range entries {
		encoded, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		if body.Len()+len(encoded)+3 > MaxCatalogBytes {
			return "", fmt.Errorf("designattempts: catalog exceeds byte limit")
		}
		if i != 0 {
			body.WriteByte(',')
		}
		body.Write(encoded)
	}
	body.WriteString("]}")
	return digest(body.Bytes()), nil
}

func validateEntries(entries []Entry) error {
	if entries == nil || len(entries) > MaxEntries {
		return fmt.Errorf("designattempts: catalog entries must be a bounded array")
	}
	for i, e := range entries {
		if !validID(e.ID) || !validArchiveID(e.Source.ArchiveID) || !validSourcePath(e.Source.Path) {
			return fmt.Errorf("designattempts: invalid entry or source identity")
		}
		if i > 0 && entries[i-1].ID >= e.ID {
			return fmt.Errorf("designattempts: entry IDs must be unique and sorted")
		}
		for _, f := range []struct {
			value string
			limit int
		}{
			{e.Source.SHA256, 64}, {e.Target, 128}, {e.Transformer.ID, 128}, {e.Transformer.SHA256, 64},
			{e.Transformer.GoVersion, 256}, {e.Transformer.HTMLParser, 256}, {e.Transformer.RequiredTempl, 256},
			{e.Status, 32}, {e.Verification, 32}, {e.ArtifactSHA256, 64},
		} {
			if len(f.value) > f.limit || !utf8.ValidString(f.value) {
				return fmt.Errorf("designattempts: entry text exceeds byte or UTF-8 bounds")
			}
		}
		if e.DiagnosticCodes == nil || len(e.DiagnosticCodes) > designconvert.MaxDiagnostics {
			return fmt.Errorf("designattempts: diagnostic codes must be a bounded array")
		}
		for _, code := range e.DiagnosticCodes {
			if len(code) > 128 || !utf8.ValidString(code) {
				return fmt.Errorf("designattempts: diagnostic code exceeds byte or UTF-8 bounds")
			}
		}
	}
	return nil
}

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var archivePattern = regexp.MustCompile(`^[a-z0-9-]+@[a-f0-9]{64}$`)

func validID(id string) bool {
	return len(id) == 71 && strings.HasPrefix(id, "syntax@") && hashPattern.MatchString(id[7:])
}

func validArchiveID(id string) bool {
	return len(id) <= maxArchiveIDBytes && archivePattern.MatchString(id)
}

func validSourcePath(name string) bool {
	if name == "" || len(name) > maxSourcePathBytes || !utf8.ValidString(name) || strings.ContainsAny(name, "\\\x00:") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func cloneEntry(e Entry) Entry { e.DiagnosticCodes = slices.Clone(e.DiagnosticCodes); return e }

func cloneEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = cloneEntry(e)
	}
	return out
}

func digest(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func preflightJSON(body []byte) error {
	d := json.NewDecoder(bytes.NewReader(body))
	var walk func(depth, arrayLimit int) error
	walk = func(depth, arrayLimit int) error {
		if depth > 8 {
			return fmt.Errorf("designattempts: catalog JSON nesting exceeds limit")
		}
		t, err := d.Token()
		if err != nil {
			return fmt.Errorf("designattempts: invalid catalog JSON")
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				t, err := d.Token()
				key, ok := t.(string)
				if err != nil || !ok || len(key) > 128 || keys[key] || len(keys) == 32 {
					return fmt.Errorf("designattempts: invalid or duplicate catalog JSON key")
				}
				keys[key] = true
				limit := designconvert.MaxDiagnostics
				if depth == 0 && key == "entries" {
					limit = MaxEntries
				}
				if err := walk(depth+1, limit); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim('}') {
				return fmt.Errorf("designattempts: invalid catalog JSON object")
			}
		case '[':
			count := 0
			for d.More() {
				if count == arrayLimit {
					return fmt.Errorf("designattempts: catalog JSON array exceeds limit")
				}
				count++
				if err := walk(depth+1, designconvert.MaxDiagnostics); err != nil {
					return err
				}
			}
			if end, err := d.Token(); err != nil || end != json.Delim(']') {
				return fmt.Errorf("designattempts: invalid catalog JSON array")
			}
		default:
			return fmt.Errorf("designattempts: invalid catalog JSON delimiter")
		}
		return nil
	}
	if err := walk(0, MaxEntries); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("designattempts: trailing catalog JSON")
	}
	return nil
}
