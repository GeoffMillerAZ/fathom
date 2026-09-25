package designassets

import (
	"encoding/json"
	"reflect"
	"testing"
)

func queryFixture() *Catalog {
	return &Catalog{Revision: "fixture-revision", archives: []Archive{
		{ID: "catalyst@a", Family: "catalyst", Vendor: "tailwindplus", Kind: "ui-kit", Files: []File{
			{Path: "README.md", Kind: "documentation"},
			{Path: "typescript/button.tsx", Kind: "source"},
			{Path: "typescript/sidebar.tsx", Kind: "source"},
		}},
		{ID: "oatmeal@b", Family: "oatmeal", Vendor: "tailwindplus", Kind: "template", Files: []File{{Path: "pages/pricing.tsx", Kind: "source"}}},
		{ID: "studio@c", Family: "studio", Vendor: "tailwindplus", Kind: "template", Files: []File{{Path: "studio-ts/src/app/page.tsx", Kind: "source"}}},
	}}
}

func TestAssetSearchBoundedMetadata(t *testing.T) {
	c := queryFixture()
	page, err := c.Search(Query{Text: "TAILWINDPLUS typescript"})
	if err != nil || page.Total != 1 || page.Items[0].Family != "catalyst" {
		t.Fatalf("metadata/path query: %+v %v", page, err)
	}
	for _, q := range []Query{{Text: "never-present"}, {Kind: "template", Family: "catalyst"}} {
		page, err := c.Search(q)
		if err != nil || page.Total != 0 || page.Items == nil {
			t.Fatalf("valid no-match query must return []: %+v %v", page, err)
		}
	}
	for _, q := range []Query{{Kind: "react"}, {Limit: -1}, {Limit: 51}, {Cursor: "bad"}} {
		if _, err := c.Search(q); err == nil {
			t.Errorf("invalid query accepted: %+v", q)
		}
	}
	all := []Summary{}
	q := Query{Limit: 1}
	for {
		page, err := c.Search(q)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) > 1 {
			t.Fatal("page limit ignored")
		}
		all = append(all, page.Items...)
		if page.NextCursor == "" {
			break
		}
		q.Cursor = page.NextCursor
	}
	if len(all) != 3 || all[0].ID != "catalyst@a" || all[2].ID != "studio@c" {
		t.Fatalf("pagination is not deterministic: %+v", all)
	}
	b, _ := json.Marshal(all)
	var decoded []map[string]any
	_ = json.Unmarshal(b, &decoded)
	for _, row := range decoded {
		if _, ok := row["files"]; ok {
			t.Fatal("search dumped member inventory")
		}
		if _, ok := row["content"]; ok {
			t.Fatal("search exposed source")
		}
	}
}

func TestAssetCursorBindsRevisionAndEffectiveQuery(t *testing.T) {
	c := queryFixture()
	first, err := c.Search(Query{Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first page: %+v %v", first, err)
	}
	for _, q := range []Query{{Limit: 2, Cursor: first.NextCursor}, {Limit: 1, Text: "studio", Cursor: first.NextCursor}, {Limit: 1, Family: "studio", Cursor: first.NextCursor}} {
		if _, err := c.Search(q); err == nil {
			t.Errorf("cursor reused across query: %+v", q)
		}
	}
	c.Revision = "new-fixture-revision"
	if _, err := c.Search(Query{Limit: 1, Cursor: first.NextCursor}); err == nil {
		t.Fatal("cursor survived revision change")
	}
}

func TestAssetFilesPageFiltersAndCursor(t *testing.T) {
	c := queryFixture()
	q := FileQuery{ID: "catalyst@a", Prefix: "typescript/", Limit: 1}
	first, err := c.FilesPage(q)
	if err != nil || first.Total != 2 || first.Items[0].Path != "typescript/button.tsx" || first.NextCursor == "" {
		t.Fatalf("first file page: %+v %v", first, err)
	}
	q.Cursor = first.NextCursor
	second, err := c.FilesPage(q)
	if err != nil || second.NextCursor != "" || second.Items[0].Path != "typescript/sidebar.tsx" {
		t.Fatalf("second file page: %+v %v", second, err)
	}
	q.Kind = "source"
	if _, err := c.FilesPage(q); err == nil {
		t.Fatal("file cursor survived filter change")
	}
	for _, q := range []FileQuery{{ID: "missing"}, {ID: "catalyst@a", Prefix: "../"}, {ID: "catalyst@a", Prefix: "/"}, {ID: "catalyst@a", Prefix: "a\\b"}, {ID: "catalyst@a", Prefix: "./"}} {
		if _, err := c.FilesPage(q); err == nil {
			t.Errorf("invalid file query accepted: %+v", q)
		}
	}
	page, err := c.FilesPage(FileQuery{ID: "catalyst@a", Prefix: "not-present/"})
	if err != nil || !reflect.DeepEqual(page.Items, []File{}) {
		t.Fatalf("empty page: %+v %v", page, err)
	}
}
