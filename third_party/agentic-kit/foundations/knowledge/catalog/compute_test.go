package catalog

import (
	"reflect"
	"testing"
)

func TestRollup(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a"},
		{ParentID: "a", Slug: "b"},
		{ParentID: "a/b", Slug: "c"},
		{Slug: "leaf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := Rollup(cat)
	want := []RollupEntry{
		{ID: "a", Count: 2},
		{ID: "a/b", Count: 1},
		{ID: "a/b/c", Count: 0},
		{ID: "leaf", Count: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Rollup = %+v, want %+v", got, want)
	}
}

func TestDepthHistogram(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a"},
		{Slug: "b"},
		{ParentID: "a", Slug: "child"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := DepthHistogram(cat)
	want := []DepthBucket{{Depth: 1, Count: 2}, {Depth: 2, Count: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DepthHistogram = %+v, want %+v", got, want)
	}
}

func TestCoverageByFacet(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a", Facets: map[string][]string{"framework": {"react", "html"}}},
		{Slug: "b", Facets: map[string][]string{"framework": {"react"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := CoverageByFacet(cat)
	want := []FacetCoverage{
		{Axis: "framework", Value: "html", Count: 1},
		{Axis: "framework", Value: "react", Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CoverageByFacet = %+v, want %+v", got, want)
	}
}

func TestOrphans(t *testing.T) {
	cat := Catalog{Nodes: []Node{
		{ParentID: "missing-parent", Slug: "orphan"},
		{Slug: "fine"},
	}}
	got := Orphans(cat)
	want := []Orphan{{ID: "missing-parent/orphan", ParentID: "missing-parent"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Orphans = %+v, want %+v", got, want)
	}
}

func TestDuplicateSlugs(t *testing.T) {
	cat := Catalog{Nodes: []Node{
		{ParentID: "a", Slug: "dup"},
		{ParentID: "a", Slug: "dup"},
		{ParentID: "a", Slug: "unique"},
	}}
	got := DuplicateSlugs(cat)
	want := []DuplicateGroup{{ParentID: "a", Slug: "dup", Count: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DuplicateSlugs = %+v, want %+v", got, want)
	}
}

func TestDuplicateStableKeys(t *testing.T) {
	cases := []struct {
		name  string
		nodes []Node
		want  []DuplicateStableKey
	}{
		{
			name: "no stable keys",
			nodes: []Node{
				{Slug: "a"},
				{Slug: "b"},
			},
			want: nil,
		},
		{
			name: "unique stable keys",
			nodes: []Node{
				{Slug: "a", StableKey: "key-a"},
				{Slug: "b", StableKey: "key-b"},
			},
			want: nil,
		},
		{
			name: "one duplicate pair, IDs sorted",
			nodes: []Node{
				{Slug: "z", StableKey: "dup"},
				{Slug: "a", StableKey: "dup"},
			},
			want: []DuplicateStableKey{{StableKey: "dup", IDs: []string{"a", "z"}}},
		},
		{
			name: "duplicate triple",
			nodes: []Node{
				{Slug: "a", StableKey: "dup"},
				{Slug: "b", StableKey: "dup"},
				{Slug: "c", StableKey: "dup"},
			},
			want: []DuplicateStableKey{{StableKey: "dup", IDs: []string{"a", "b", "c"}}},
		},
		{
			name: "two independent duplicate groups, sorted by key",
			nodes: []Node{
				{Slug: "a", StableKey: "zzz"},
				{Slug: "b", StableKey: "zzz"},
				{Slug: "c", StableKey: "aaa"},
				{Slug: "d", StableKey: "aaa"},
			},
			want: []DuplicateStableKey{
				{StableKey: "aaa", IDs: []string{"c", "d"}},
				{StableKey: "zzz", IDs: []string{"a", "b"}},
			},
		},
		{
			name: "empty StableKey is never compared",
			nodes: []Node{
				{Slug: "a"},
				{Slug: "b"},
				{Slug: "c"},
			},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DuplicateStableKeys(Catalog{Nodes: c.nodes})
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("DuplicateStableKeys = %+v, want %+v", got, c.want)
			}
		})
	}
}
