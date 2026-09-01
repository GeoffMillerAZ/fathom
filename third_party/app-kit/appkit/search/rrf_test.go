package search

import (
	"reflect"
	"testing"
)

// TestFuseRRFStrings exercises the string-id instantiation the portal uses
// (platform-engineering's portal/internal/search re-exports these names over
// document refs). The cases mirror the property tests that lived in that
// package before the extraction — they are the contract this package took
// over, not a new one.
func TestFuseRRFStrings(t *testing.T) {
	tests := []struct {
		name    string
		lists   [][]string
		k       float64
		wantIDs []string
		// wantRanks[i] is the expected Ranks slice for wantIDs[i].
		wantRanks [][]int
	}{
		{
			name:      "empty input yields no results",
			lists:     nil,
			k:         DefaultRRFK,
			wantIDs:   nil,
			wantRanks: nil,
		},
		{
			name:      "single list keeps its order",
			lists:     [][]string{{"a", "b", "c"}},
			k:         DefaultRRFK,
			wantIDs:   []string{"a", "b", "c"},
			wantRanks: [][]int{{1}, {2}, {3}},
		},
		{
			name:  "two retrievers agreeing on a document lift it above a single-list leader",
			lists: [][]string{{"a", "b", "c"}, {"c", "a"}},
			k:     DefaultRRFK,
			// a = 1/61 + 1/62, c = 1/63 + 1/61, b = 1/62.
			wantIDs:   []string{"a", "c", "b"},
			wantRanks: [][]int{{1, 2}, {3, 1}, {2, 0}},
		},
		{
			name:      "equal scores break on id, not on list order",
			lists:     [][]string{{"zebra"}, {"apple"}},
			k:         DefaultRRFK,
			wantIDs:   []string{"apple", "zebra"},
			wantRanks: [][]int{{0, 1}, {1, 0}},
		},
		{
			name:      "a repeated id is paid once, at its best rank",
			lists:     [][]string{{"a", "a", "b"}},
			k:         DefaultRRFK,
			wantIDs:   []string{"a", "b"},
			wantRanks: [][]int{{1}, {3}},
		},
		{
			name:      "empty ids are dropped rather than ranked",
			lists:     [][]string{{"", "a"}},
			k:         DefaultRRFK,
			wantIDs:   []string{"a"},
			wantRanks: [][]int{{2}},
		},
		{
			name:      "a non-positive k falls back to the default rather than dividing by zero",
			lists:     [][]string{{"a", "b"}},
			k:         0,
			wantIDs:   []string{"a", "b"},
			wantRanks: [][]int{{1}, {2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuseRRF(tt.lists, tt.k)
			if gotIDs := IDs(got); !reflect.DeepEqual(gotIDs, tt.wantIDs) && !(len(gotIDs) == 0 && len(tt.wantIDs) == 0) {
				t.Fatalf("order: got %v, want %v", gotIDs, tt.wantIDs)
			}
			for i, r := range got {
				if !reflect.DeepEqual(r.Ranks, tt.wantRanks[i]) {
					t.Errorf("%s ranks: got %v, want %v", r.ID, r.Ranks, tt.wantRanks[i])
				}
			}
			for i := 1; i < len(got); i++ {
				if got[i-1].Score < got[i].Score {
					t.Errorf("scores not descending at %d: %v", i, got)
				}
			}
			// Determinism: the same input must fuse to the same bytes, or a
			// golden query set is measuring noise.
			if again := FuseRRF(tt.lists, tt.k); !reflect.DeepEqual(again, got) {
				t.Errorf("not deterministic:\nrun1 %#v\nrun2 %#v", got, again)
			}
		})
	}

	t.Run("k decides how much a single retriever's confidence is worth", func(t *testing.T) {
		// "solo" tops one list; "cand" is third in both. A small k makes the
		// head of a list nearly all that matters, so solo wins; the default k
		// flattens the curve enough that two retrievers agreeing wins.
		lists := [][]string{{"solo", "x", "cand"}, {"u", "v", "cand"}}
		pos := func(rs []Ranked[string], id string) int {
			for i, r := range rs {
				if r.ID == id {
					return i
				}
			}
			t.Fatalf("%s missing from %v", id, IDs(rs))
			return -1
		}
		if small := FuseRRF(lists, 0.5); pos(small, "solo") > pos(small, "cand") {
			t.Errorf("k=0.5: got %v, want solo ahead of cand", IDs(small))
		}
		if big := FuseRRF(lists, DefaultRRFK); pos(big, "cand") > pos(big, "solo") {
			t.Errorf("k=60: got %v, want cand ahead of solo", IDs(big))
		}
	})
}

// TestFuseRRFInt64 exercises the int64-id instantiation internal/twplus/index
// uses to fuse BM25 and vector-KNN variant rowids. The zero-id and
// repeated-id rules are the ones that matter here: a SQLite INTEGER PRIMARY
// KEY rowid never legitimately produces 0, so dropping it as twplus's "no
// result" sentinel does not cost twplus a real candidate the way dropping ""
// costs the portal nothing either.
func TestFuseRRFInt64(t *testing.T) {
	lists := [][]int64{{10, 20, 30}, {30, 10}}
	got := FuseRRF(lists, DefaultRRFK)
	want := []int64{10, 30, 20}
	if gotIDs := IDs(got); !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("order: got %v, want %v", gotIDs, want)
	}

	t.Run("zero id dropped", func(t *testing.T) {
		got := FuseRRF([][]int64{{0, 7}}, DefaultRRFK)
		if want := []int64{7}; !reflect.DeepEqual(IDs(got), want) {
			t.Fatalf("got %v, want %v", IDs(got), want)
		}
	})

	t.Run("repeated id paid once at its best rank", func(t *testing.T) {
		got := FuseRRF([][]int64{{5, 5, 9}}, DefaultRRFK)
		if want := []int64{5, 9}; !reflect.DeepEqual(IDs(got), want) {
			t.Fatalf("got %v, want %v", IDs(got), want)
		}
		if got[0].Ranks[0] != 1 {
			t.Errorf("id 5 should be paid at rank 1 (its best), got %d", got[0].Ranks[0])
		}
	})
}

func TestIDsProjection(t *testing.T) {
	rs := []Ranked[string]{{ID: "z", Score: 1}, {ID: "a", Score: 2}}
	if got, want := IDs(rs), []string{"z", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v, want %v", got, want)
	}
	if got := IDs[string](nil); len(got) != 0 {
		t.Errorf("IDs(nil) = %v, want empty", got)
	}
}
