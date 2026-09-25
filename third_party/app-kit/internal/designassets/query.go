package designassets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Summary deliberately excludes source and the potentially large member list.
type Summary struct {
	ID        string `json:"id"`
	Family    string `json:"family"`
	Kind      string `json:"kind"`
	Vendor    string `json:"vendor"`
	Filename  string `json:"filename"`
	SHA256    string `json:"sha256"`
	SourceURL string `json:"source_url"`
	Size      int64  `json:"size"`
	FileCount int    `json:"file_count"`
	Projects  int    `json:"project_count"`
}

func Summarize(a Archive) Summary {
	return Summary{ID: a.ID, Family: a.Family, Kind: a.Kind, Vendor: a.Vendor,
		Filename: a.Filename, SHA256: a.SHA256, SourceURL: a.SourceURL,
		Size: a.Size, FileCount: len(a.Files), Projects: len(a.Projects)}
}

type Query struct {
	Text   string `json:"query,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Family string `json:"family,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type Page struct {
	Revision   string    `json:"revision"`
	Items      []Summary `json:"items"`
	Total      int       `json:"total"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

// Search searches descriptive metadata and member paths, never source bodies.
// Results are source references, not claims of consumer/runtime compatibility.
func (c *Catalog) Search(q Query) (Page, error) {
	if len(q.Text) > 2048 || len(q.Family) > 256 {
		return Page{}, fmt.Errorf("query or family exceeds metadata search bounds")
	}
	q.Text = strings.Join(strings.Fields(strings.ToLower(q.Text)), " ")
	if q.Kind != "" && q.Kind != "template" && q.Kind != "ui-kit" && q.Kind != "ui-blocks" {
		return Page{}, fmt.Errorf("unknown design asset kind %q; want template, ui-kit or ui-blocks", q.Kind)
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return Page{}, err
	}
	q.Limit = limit
	cursor := q.Cursor
	q.Cursor = ""
	key := queryKey(q)
	all := c.List()
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	items := make([]Summary, 0)
	for _, a := range all {
		if q.Kind != "" && a.Kind != q.Kind || q.Family != "" && a.Family != q.Family {
			continue
		}
		var hay strings.Builder
		hay.WriteString(strings.ToLower(a.Family + " " + a.Filename + " " + a.Kind + " " + a.Vendor))
		for _, f := range a.Files {
			hay.WriteByte(' ')
			hay.WriteString(strings.ToLower(f.Path))
		}
		match := true
		for _, word := range strings.Fields(q.Text) {
			if !strings.Contains(hay.String(), word) {
				match = false
				break
			}
		}
		if match {
			items = append(items, Summarize(a))
		}
	}
	start, end, next, err := pageBounds(c.Revision, key, cursor, len(items), limit)
	if err != nil {
		return Page{}, err
	}
	return Page{Revision: c.Revision, Items: items[start:end], Total: len(items), NextCursor: next}, nil
}

type FileQuery struct {
	ID     string `json:"id"`
	Prefix string `json:"prefix,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type FilePage struct {
	Revision   string `json:"revision"`
	ArchiveID  string `json:"archive_id"`
	Items      []File `json:"items"`
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// FilesPage reads the validated manifest only. Exact source reads verify archive
// integrity separately. Prefix is a case-sensitive archive-relative path prefix.
func (c *Catalog) FilesPage(q FileQuery) (FilePage, error) {
	if len(q.Prefix) > MaxPathBytes {
		return FilePage{}, fmt.Errorf("prefix exceeds maximum archive path size")
	}
	switch q.Kind {
	case "", "source", "manifest", "readme", "license", "theme", "asset", "documentation", "config", "sensitive", "other":
	default:
		return FilePage{}, fmt.Errorf("unknown file kind %q; use a kind from an earlier file listing", q.Kind)
	}
	if strings.ContainsAny(q.Prefix, "\\\x00") || strings.HasPrefix(q.Prefix, "/") {
		return FilePage{}, fmt.Errorf("prefix must be an archive-relative path")
	}
	for _, part := range strings.Split(q.Prefix, "/") {
		if part == ".." || part == "." {
			return FilePage{}, fmt.Errorf("prefix may not contain dot path segments")
		}
	}
	limit, err := pageLimit(q.Limit)
	if err != nil {
		return FilePage{}, err
	}
	q.Limit = limit
	cursor := q.Cursor
	q.Cursor = ""
	key := queryKey(q)
	a, ok := c.Get(q.ID)
	if !ok {
		return FilePage{}, fmt.Errorf("unknown design asset %q; search for an exact archive ID", q.ID)
	}
	items := make([]File, 0)
	for _, f := range a.Files {
		if strings.HasPrefix(f.Path, q.Prefix) && (q.Kind == "" || q.Kind == f.Kind) {
			items = append(items, f)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	start, end, next, err := pageBounds(c.Revision, key, cursor, len(items), limit)
	if err != nil {
		return FilePage{}, err
	}
	return FilePage{Revision: c.Revision, ArchiveID: q.ID, Items: items[start:end], Total: len(items), NextCursor: next}, nil
}

type assetCursor struct {
	Revision string `json:"revision"`
	Query    string `json:"query"`
	Offset   int    `json:"offset"`
}

func pageLimit(n int) (int, error) {
	if n == 0 {
		return 10, nil
	}
	if n < 1 || n > 50 {
		return 0, fmt.Errorf("limit must be between 1 and 50")
	}
	return n, nil
}

func queryKey(q any) string {
	b, _ := json.Marshal(q)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func pageBounds(revision, key, cursor string, count, limit int) (int, int, string, error) {
	start := 0
	if cursor != "" {
		var cur assetCursor
		if len(cursor) > 1024 {
			return 0, 0, "", fmt.Errorf("invalid cursor")
		}
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(b, &cur) != nil || cur.Revision != revision || cur.Query != key || cur.Offset < 1 || cur.Offset >= count || cur.Offset%limit != 0 {
			return 0, 0, "", fmt.Errorf("invalid or stale cursor; restart with unchanged filters and current catalog")
		}
		start = cur.Offset
	}
	end := min(start+limit, count)
	next := ""
	if end < count {
		b, _ := json.Marshal(assetCursor{Revision: revision, Query: key, Offset: end})
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	return start, end, next, nil
}
