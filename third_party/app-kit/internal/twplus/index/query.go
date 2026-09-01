package index

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

// ErrNotFound is returned by GetComponent when no variant matches.
var ErrNotFound = errors.New("twplus/index: component not found")

// Index is a read-only handle to a built corpus.
type Index struct {
	db  *sql.DB
	dec *zstd.Decoder
}

// Open opens an existing index file read-only.
func Open(path string) (*Index, error) {
	db, err := driver.Open("file:"+path+"?mode=ro&immutable=1", func(c *sqlite3.Conn) error { return fts5.Register(c) })
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: open %s: %w", path, err)
	}
	dec, err := zstd.NewReader(nil)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: zstd reader: %w", err)
	}
	return &Index{db: db, dec: dec}, nil
}

// Close releases the database and decoder.
func (ix *Index) Close() error {
	ix.dec.Close()
	return ix.db.Close()
}

// Filters are the structural constraints shared by search and list.
type Filters struct {
	Section         string
	Framework       string
	TailwindVersion int // 0 = any
	Mode            string
}

func (f Filters) where(alias string, args *[]any) string {
	var cl []string
	if f.Section != "" {
		cl = append(cl, "c.section = ?")
		*args = append(*args, f.Section)
	}
	if f.Framework != "" {
		cl = append(cl, alias+".framework = ?")
		*args = append(*args, f.Framework)
	}
	if f.TailwindVersion != 0 {
		cl = append(cl, alias+".tailwind_version = ?")
		*args = append(*args, f.TailwindVersion)
	}
	if f.Mode != "" {
		cl = append(cl, alias+".mode = ?")
		*args = append(*args, f.Mode)
	}
	if len(cl) == 0 {
		return ""
	}
	return " AND " + strings.Join(cl, " AND ")
}

// SearchResult is one ranked hit.
type SearchResult struct {
	FullName        string  `json:"full_name"`
	Section         string  `json:"section"`
	Name            string  `json:"name"`
	Framework       string  `json:"framework"`
	TailwindVersion int     `json:"tailwind_version"`
	Mode            string  `json:"mode"`
	Score           float64 `json:"score"`      // BM25; lower is a stronger match
	MatchText       string  `json:"match_text"` // the indexed embed-text (taxonomy + axes + ranked classes)
}

// SearchComponents runs BM25 + structural search. An empty query falls back to
// a structural listing ordered by full_name (so filters alone still work).
func (ix *Index) SearchComponents(query string, f Filters, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	match := ftsQuery(query)
	if match == "" {
		return ix.searchStructural(f, limit)
	}
	args := []any{match}
	sqlText := `SELECT c.full_name, c.section, c.name, v.framework, v.tailwind_version, v.mode,
		bm25(fts_variant, 10.0, 8.0, 3.0, 2.0) AS score, v.embed_text
		FROM fts_variant
		JOIN variant v   ON v.id = fts_variant.rowid
		JOIN component c ON c.id = v.component_id
		WHERE fts_variant MATCH ?` + f.where("v", &args) + `
		ORDER BY score LIMIT ?`
	args = append(args, limit)
	return ix.scanResults(sqlText, args)
}

func (ix *Index) searchStructural(f Filters, limit int) ([]SearchResult, error) {
	var args []any
	sqlText := `SELECT c.full_name, c.section, c.name, v.framework, v.tailwind_version, v.mode,
		0.0 AS score, v.embed_text
		FROM variant v JOIN component c ON c.id = v.component_id
		WHERE 1=1` + f.where("v", &args) + `
		ORDER BY c.full_name, v.framework, v.tailwind_version, v.mode LIMIT ?`
	args = append(args, limit)
	return ix.scanResults(sqlText, args)
}

func (ix *Index) scanResults(sqlText string, args []any) ([]SearchResult, error) {
	rows, err := ix.db.Query(sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.FullName, &r.Section, &r.Name, &r.Framework, &r.TailwindVersion, &r.Mode, &r.Score, &r.MatchText); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// VariantAxes identifies one concrete variant of a component.
type VariantAxes struct {
	Framework       string `json:"framework"`
	TailwindVersion int    `json:"tailwind_version"`
	Mode            string `json:"mode"`
}

// GetResult is the full code for one variant plus the other axes available.
type GetResult struct {
	FullName         string        `json:"full_name"`
	Section          string        `json:"section"`
	Name             string        `json:"name"`
	Framework        string        `json:"framework"`
	Language         string        `json:"language"`
	TailwindVersion  int           `json:"tailwind_version"`
	Mode             string        `json:"mode"`
	SupportsDarkMode bool          `json:"supports_dark_mode"`
	Code             string        `json:"code"`
	Available        []VariantAxes `json:"available_variants"`
}

// GetComponent returns the code for a variant of full_name. Framework, tw and
// mode are optional filters; when they leave more than one variant, the
// lexically-first is returned and every available variant is listed so the
// caller can disambiguate.
func (ix *Index) GetComponent(fullName, framework string, tw int, mode string) (*GetResult, error) {
	f := Filters{Framework: framework, TailwindVersion: tw, Mode: mode}
	args := []any{fullName}
	q := `SELECT c.section, c.name, v.framework, v.language, v.tailwind_version, v.mode,
		v.supports_dark_mode, b.code_zstd, b.raw_len
		FROM variant v
		JOIN component c ON c.id = v.component_id
		JOIN blob b      ON b.content_hash = v.content_hash
		WHERE c.full_name = ?` + f.where("v", &args) + `
		ORDER BY v.framework, v.tailwind_version, v.mode LIMIT 1`
	var (
		res     GetResult
		packed  []byte
		rawLen  int
		darkInt int
	)
	res.FullName = fullName
	err := ix.db.QueryRow(q, args...).Scan(&res.Section, &res.Name, &res.Framework, &res.Language,
		&res.TailwindVersion, &res.Mode, &darkInt, &packed, &rawLen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, describe(fullName, framework, tw, mode))
	}
	if err != nil {
		return nil, fmt.Errorf("twplus/index: get %s: %w", fullName, err)
	}
	res.SupportsDarkMode = darkInt != 0
	code, err := ix.dec.DecodeAll(packed, make([]byte, 0, rawLen))
	if err != nil {
		return nil, fmt.Errorf("twplus/index: decompress %s: %w", fullName, err)
	}
	res.Code = string(code)

	avail, err := ix.availableVariants(fullName)
	if err != nil {
		return nil, err
	}
	res.Available = avail
	return &res, nil
}

func (ix *Index) availableVariants(fullName string) ([]VariantAxes, error) {
	rows, err := ix.db.Query(`SELECT v.framework, v.tailwind_version, v.mode
		FROM variant v JOIN component c ON c.id = v.component_id
		WHERE c.full_name = ?
		ORDER BY v.framework, v.tailwind_version, v.mode`, fullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VariantAxes
	for rows.Next() {
		var a VariantAxes
		if err := rows.Scan(&a.Framework, &a.TailwindVersion, &a.Mode); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ComponentSummary is one row of a browse listing.
type ComponentSummary struct {
	FullName   string `json:"full_name"`
	Section    string `json:"section"`
	Group      string `json:"group"`
	Category   string `json:"category"`
	Name       string `json:"name"`
	NumVariant int    `json:"variants"`
}

// ListComponents browses the catalog structurally (no text query).
func (ix *Index) ListComponents(f Filters, limit int) ([]ComponentSummary, error) {
	if limit <= 0 {
		limit = 200
	}
	var args []any
	q := `SELECT c.full_name, c.section, c."group", c.category, c.name, COUNT(v.id)
		FROM component c JOIN variant v ON v.component_id = c.id
		WHERE 1=1` + f.where("v", &args) + `
		GROUP BY c.id
		ORDER BY c.full_name LIMIT ?`
	args = append(args, limit)
	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: list: %w", err)
	}
	defer rows.Close()
	var out []ComponentSummary
	for rows.Next() {
		var s ComponentSummary
		if err := rows.Scan(&s.FullName, &s.Section, &s.Group, &s.Category, &s.Name, &s.NumVariant); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Facet is a value plus how many components carry it.
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Sections returns the section facets (component counts).
func (ix *Index) Sections() ([]Facet, error) {
	return ix.facets(`SELECT section, COUNT(*) FROM component GROUP BY section ORDER BY section`)
}

// Groups returns group facets, optionally within a section.
func (ix *Index) Groups(section string) ([]Facet, error) {
	if section == "" {
		return ix.facets(`SELECT "group", COUNT(*) FROM component GROUP BY "group" ORDER BY "group"`)
	}
	return ix.facets(`SELECT "group", COUNT(*) FROM component WHERE section = ? GROUP BY "group" ORDER BY "group"`, section)
}

func (ix *Index) facets(q string, args ...any) ([]Facet, error) {
	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: facets: %w", err)
	}
	defer rows.Close()
	var out []Facet
	for rows.Next() {
		var f Facet
		if err := rows.Scan(&f.Value, &f.Count); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Counts reports corpus totals for health checks / manifests.
func (ix *Index) Counts() (components, variants, blobs int, err error) {
	err = ix.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM component),
		(SELECT COUNT(*) FROM variant),
		(SELECT COUNT(*) FROM blob)`).Scan(&components, &variants, &blobs)
	return
}

// ftsQuery turns free text into a safe FTS5 MATCH expression: each token is
// double-quoted (escaping embedded quotes), so FTS5 operators in user input —
// including the '-' that broke the spike on "rounded-md" — are treated as
// literal tokens. Tokens are OR-joined for recall; BM25 handles ranking.
func ftsQuery(s string) string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':')
	})
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimFunc(f, func(r rune) bool { return r == '-' || r == ':' })
		if f == "" {
			continue
		}
		quoted = append(quoted, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func describe(fullName, framework string, tw int, mode string) string {
	var b strings.Builder
	b.WriteString(strconv.Quote(fullName))
	if framework != "" {
		b.WriteString(" framework=" + framework)
	}
	if tw != 0 {
		b.WriteString(" tailwind_version=" + strconv.Itoa(tw))
	}
	if mode != "" {
		b.WriteString(" mode=" + mode)
	}
	return b.String()
}
