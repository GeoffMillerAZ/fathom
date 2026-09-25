// Package catalogtest shares source-free task expectations across catalog,
// MCP, and Gallery tests. Production binaries do not import this package.
package catalogtest

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"slices"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

//go:embed testdata/selection.json
var corpus []byte

type selectionCase struct {
	Name      string        `json:"name"`
	Query     catalog.Query `json:"query"`
	Top       []string      `json:"top"`
	Forbidden []string      `json:"forbidden"`
	Empty     bool          `json:"empty"`
	Reason    string        `json:"reason"`
}

func decode(t *testing.T, data []byte, out any) {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		t.Fatal(err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatalf("trailing JSON: %v", err)
	}
}

// Run checks the same authored task expectations through any search transport.
// It follows every page: a forbidden recommendation cannot hide behind a cursor.
// Corpus expectations describe supported tasks, not snapshots of current ranking.
func Run(t *testing.T, c *catalog.Catalog, fetch func(*testing.T, catalog.Query) []byte) {
	t.Helper()
	var cases []selectionCase
	decode(t, corpus, &cases)
	names, covered := map[string]bool{}, map[string]bool{}
	for _, tc := range cases {
		if tc.Name == "" || tc.Reason == "" || names[tc.Name] || tc.Empty == (len(tc.Top) > 0) || tc.Query.Cursor != "" || tc.Query.Limit < 0 || tc.Query.Limit > 10 {
			t.Fatalf("invalid or duplicate selection case: %+v", tc)
		}
		names[tc.Name] = true
		for _, id := range append(slices.Clone(tc.Top), tc.Forbidden...) {
			if _, ok := c.Get(id); !ok {
				t.Fatalf("%s expects unknown ID %s", tc.Name, id)
			}
		}
		for _, id := range tc.Top {
			if slices.Contains(tc.Forbidden, id) {
				t.Fatalf("%s both requires and forbids %s", tc.Name, id)
			}
			covered[id] = true
		}
	}
	for _, entry := range c.Entries {
		if !covered[entry.ID] {
			t.Errorf("catalog entry %s needs an authored positive task expectation", entry.ID)
		}
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			q := tc.Query
			limit := q.Limit
			if limit == 0 {
				limit = 10
			}
			ids := []string{}
			cursors := map[string]bool{}
			total := -1
			for {
				body := fetch(t, q)
				if len(body) > 16*1024 {
					t.Fatalf("search exceeds discovery budget: %d bytes", len(body))
				}
				var page catalog.Page
				decode(t, body, &page)
				if total == -1 {
					total = page.Total
				}
				if page.Revision != c.Revision || page.Total != total || total < 0 || total > len(c.Entries) || page.Items == nil || len(page.Items) != min(limit, total-len(ids)) {
					t.Fatalf("inconsistent or unbounded page: %+v", page)
				}
				for _, item := range page.Items {
					if slices.Contains(ids, item.ID) {
						t.Fatalf("repeated recommendation: %s", item.ID)
					}
					e, ok := c.Get(item.ID)
					if !ok || q.Profile != "" && !slices.Contains(e.Profiles, q.Profile) || q.Layer != "" && !slices.Contains(e.Layers, q.Layer) || q.Kind != "" && e.Kind != q.Kind || q.Maturity != "" && e.Maturity != q.Maturity {
						t.Fatalf("incompatible recommendation: %+v", item)
					}
					ids = append(ids, item.ID)
				}
				if page.NextCursor == "" {
					if len(ids) != total {
						t.Fatalf("pagination omitted %d recommendations", total-len(ids))
					}
					break
				}
				if cursors[page.NextCursor] || len(ids) >= total {
					t.Fatal("repeated or unnecessary continuation")
				}
				cursors[page.NextCursor] = true
				q.Cursor = page.NextCursor
			}
			if tc.Empty && len(ids) != 0 {
				t.Errorf("unavailable task recommended %v: %s", ids, tc.Reason)
			}
			if len(ids) < len(tc.Top) || !slices.Equal(ids[:min(len(ids), len(tc.Top))], tc.Top) {
				t.Errorf("recommendations %v; want leading %v: %s", ids, tc.Top, tc.Reason)
			}
			for _, id := range tc.Forbidden {
				if slices.Contains(ids, id) {
					t.Errorf("forbidden recommendation %s: %s", id, tc.Reason)
				}
			}
		})
	}
}
