// Package designevidence stores bounded, private, revision-bound evidence bundles.
// It proves local byte integrity and original membership on exact reads. It does
// not execute reports or certify a transformer, target, semantics, or runtime.
package designevidence

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
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

const (
	MaxCatalogBytes = 16 << 20
	MaxEntries      = 4096
	maxCursorBytes  = 2048
)

//go:embed schema.cue
var schemaSource []byte

type Entry struct {
	ID           string                `json:"id"`
	SourceMember string                `json:"source_member"`
	Recorded     designreceipts.Report `json:"recorded"`
}

// Catalog is an immutable metadata snapshot. Exported fields are detached;
// List, Get and Search read the unexported copy.
type Catalog struct {
	SchemaVersion int     `json:"schema_version"`
	Revision      string  `json:"revision"`
	Entries       []Entry `json:"entries"`
	entries       []Entry
	revision      string
}

type Query struct {
	ArchiveID string `json:"archive_id"`
	SubjectID string `json:"subject_id"`
	Profile   string `json:"profile"`
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

// Search only reads the stored metadata. It deliberately does not reread
// originals or evidence bundles: use Read for exact integrity verification.
func (c *Catalog) Search(q Query) (Page, error) {
	if c == nil || c.revision == "" {
		return Page{}, fmt.Errorf("designevidence: uninitialized catalog")
	}
	cursor := q.Cursor
	q.Cursor = ""
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit < 1 || q.Limit > 50 {
		return Page{}, fmt.Errorf("designevidence: limit must be between 1 and 50")
	}
	if q.ArchiveID != "" && !archivePattern.MatchString(q.ArchiveID) {
		return Page{}, fmt.Errorf("designevidence: invalid archive filter")
	}
	if q.SubjectID != "" && !safeText(q.SubjectID, 96) {
		return Page{}, fmt.Errorf("designevidence: invalid subject filter")
	}
	if q.Profile != "" && !validProfile(q.Profile) {
		return Page{}, fmt.Errorf("designevidence: invalid profile filter")
	}
	if len(cursor) > maxCursorBytes {
		return Page{}, fmt.Errorf("designevidence: cursor exceeds limit")
	}
	queryBody, _ := json.Marshal(q)
	queryHash := digest(queryBody)
	matched := make([]Entry, 0)
	for _, e := range c.entries {
		s := e.Recorded.Subject
		if (q.ArchiveID != "" && q.ArchiveID != s.SourceArchive) || (q.SubjectID != "" && q.SubjectID != s.ID) || (q.Profile != "" && q.Profile != s.Profile) {
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
	p := Page{Revision: c.revision, Total: len(matched), Entries: cloneEntries(matched[offset:end])}
	if end < len(matched) {
		body, _ := json.Marshal(continuation{1, c.revision, queryHash, end})
		p.NextCursor = base64.RawURLEncoding.EncodeToString(body)
	}
	return p, nil
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
		return 0, fmt.Errorf("designevidence: malformed cursor")
	}
	var c continuation
	if json.Unmarshal(body, &c) != nil {
		return 0, fmt.Errorf("designevidence: malformed cursor")
	}
	canonical, _ := json.Marshal(c)
	if !bytes.Equal(body, canonical) || c.Version != 1 || !hashPattern.MatchString(c.Revision) || !hashPattern.MatchString(c.Query) {
		return 0, fmt.Errorf("designevidence: malformed cursor")
	}
	if c.Revision != revision {
		return 0, fmt.Errorf("designevidence: stale cursor revision; restart search")
	}
	if c.Query != query {
		return 0, fmt.Errorf("designevidence: cursor query changed; restart search")
	}
	if c.Offset <= 0 || c.Offset >= total || c.Offset%limit != 0 {
		return 0, fmt.Errorf("designevidence: cursor offset out of range")
	}
	return c.Offset, nil
}

func encodeCatalog(entries []Entry) ([]byte, error) {
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
	if err != nil || len(body) > MaxCatalogBytes {
		return nil, fmt.Errorf("designevidence: catalog exceeds byte limit")
	}
	if _, err := decodeCatalog(body); err != nil {
		return nil, err
	}
	return body, nil
}

func decodeCatalog(body []byte) (*Catalog, error) {
	if len(body) == 0 || len(body) > MaxCatalogBytes || !utf8.Valid(body) || !json.Valid(body) {
		return nil, fmt.Errorf("designevidence: invalid catalog encoding or size")
	}
	if err := preflightJSON(body); err != nil {
		return nil, err
	}
	var c Catalog
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("designevidence: invalid catalog JSON fields")
	}
	if d.Decode(&struct{}{}) != io.EOF || !hashPattern.MatchString(c.Revision) || c.SchemaVersion != 1 {
		return nil, fmt.Errorf("designevidence: invalid catalog")
	}
	if err := validateEntries(c.Entries); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(c)
	if err != nil || !bytes.Equal(body, canonical) {
		return nil, fmt.Errorf("designevidence: catalog is not canonical JSON")
	}
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(schemaSource)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("designevidence: embedded schema: %w", err)
	}
	v := ctx.CompileBytes(body).Unify(schema.LookupPath(cue.ParsePath("#Catalog")))
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("designevidence: catalog schema validation failed: %w", err)
	}
	revision, err := catalogRevision(c.Entries)
	if err != nil || c.Revision != revision {
		return nil, fmt.Errorf("designevidence: catalog revision mismatch")
	}
	c.entries, c.revision = cloneEntries(c.Entries), c.Revision
	return &c, nil
}

func catalogRevision(entries []Entry) (string, error) {
	body, err := json.Marshal(struct {
		SchemaVersion int     `json:"schema_version"`
		Entries       []Entry `json:"entries"`
	}{1, entries})
	if err != nil || len(body) > MaxCatalogBytes {
		return "", fmt.Errorf("designevidence: catalog exceeds byte limit")
	}
	return digest(body), nil
}
func validateEntries(entries []Entry) error {
	if entries == nil || len(entries) > MaxEntries {
		return fmt.Errorf("designevidence: catalog entries must be a bounded array")
	}
	for i, e := range entries {
		if !validID(e.ID) || !validMember(e.SourceMember) || !validReport(e.Recorded) {
			return fmt.Errorf("designevidence: invalid entry")
		}
		if i > 0 && entries[i-1].ID >= e.ID {
			return fmt.Errorf("designevidence: entry IDs must be unique and sorted")
		}
	}
	return nil
}

func validReport(r designreceipts.Report) bool {
	s := r.Subject
	if r.Integrity != "matched" || r.Verification != "not-verified" ||
		!hashPattern.MatchString(r.ContractSHA256) || !hashPattern.MatchString(r.ReceiptSHA256) ||
		!safeText(s.ID, 96) || !validProfile(s.Profile) || !safeText(s.Variant, 96) ||
		!archivePattern.MatchString(s.SourceArchive) || !hashPattern.MatchString(s.SourceRevision) ||
		!hashPattern.MatchString(s.ArtifactSHA256) || (r.Mode != "prospective" && r.Mode != "retrospective") ||
		len(r.ClaimedStatuses) != 9 || r.IncompleteGates == nil || len(r.IncompleteGates) > 9 || r.MatchedArtifacts < 0 || r.MatchedArtifacts > 864 {
		return false
	}
	for _, key := range evidenceGateNames {
		status, ok := r.ClaimedStatuses[key]
		if !ok || (status != "passed" && status != "failed" && status != "not-run" && status != "not-applicable") {
			return false
		}
	}
	for _, key := range r.IncompleteGates {
		if !evidenceGateName(key) {
			return false
		}
	}
	return true
}

var evidenceGateNames = []string{"reproducibility", "structure", "visual", "interaction", "lifecycle", "security", "data", "performance", "integration"}

func evidenceGateName(s string) bool {
	for _, name := range evidenceGateNames {
		if s == name {
			return true
		}
	}
	return false
}

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var archivePattern = regexp.MustCompile(`^[a-z0-9-]+@[a-f0-9]{64}$`)

func validID(id string) bool {
	return len(id) == 73 && strings.HasPrefix(id, "evidence@") && hashPattern.MatchString(id[9:])
}
func validMember(s string) bool {
	return safeText(s, 1024) && !strings.ContainsAny(s, "\\\x00:") && !strings.HasPrefix(s, "/") && path.Clean(s) == s && s != "." && s != ".." && !strings.HasPrefix(s, "../")
}
func safeText(s string, n int) bool {
	if s == "" || len(s) > n || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validProfile(s string) bool {
	return s == "static-web" || s == "go-web" || s == "react-web" || s == "portable" || s == "cli"
}
func digest(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }
func cloneEntry(e Entry) Entry {
	e.Recorded.ClaimedStatuses = mapsClone(e.Recorded.ClaimedStatuses)
	e.Recorded.IncompleteGates = slices.Clone(e.Recorded.IncompleteGates)
	return e
}
func cloneEntries(in []Entry) []Entry {
	out := make([]Entry, len(in))
	for i := range in {
		out[i] = cloneEntry(in[i])
	}
	return out
}
func mapsClone(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func preflightJSON(body []byte) error {
	d := json.NewDecoder(bytes.NewReader(body))
	var walk func(int, int) error
	walk = func(depth, limit int) error {
		if depth > 10 {
			return fmt.Errorf("designevidence: JSON nesting exceeds limit")
		}
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("designevidence: invalid JSON")
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				key, ok := k.(string)
				if err != nil || !ok || len(key) > 128 || keys[key] || len(keys) >= 32 {
					return fmt.Errorf("designevidence: invalid JSON object")
				}
				keys[key] = true
				// Only catalog.entries has the public 4,096 item allowance.
				// Nested metadata arrays retain the smaller amplification bound.
				childLimit := 1024
				if depth == 0 && key == "entries" {
					childLimit = MaxEntries
				}
				if err := walk(depth+1, childLimit); err != nil {
					return err
				}
			}
			if x, err := d.Token(); err != nil || x != json.Delim('}') {
				return fmt.Errorf("designevidence: invalid JSON object")
			}
		case '[':
			count := 0
			for d.More() {
				if count >= limit {
					return fmt.Errorf("designevidence: JSON array exceeds limit")
				}
				count++
				if err := walk(depth+1, limit); err != nil {
					return err
				}
			}
			if x, err := d.Token(); err != nil || x != json.Delim(']') {
				return fmt.Errorf("designevidence: invalid JSON array")
			}
		}
		return nil
	}
	if err := walk(0, MaxEntries); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("designevidence: trailing JSON")
	}
	return nil
}
