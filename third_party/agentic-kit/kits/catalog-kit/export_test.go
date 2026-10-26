package catalogkit

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

func TestCompareAndNDJSONIncludeHiddenState(t *testing.T) {
	s := fixture()
	before, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	s.Layers[0].Nodes[0].Children[0].Hidden = false
	after, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Compare(before, after)
	if err != nil || len(d.Changed) != 1 || d.Changed[0] != "review/evidence" {
		t.Fatalf("diff lost hidden change: %+v %v", d, err)
	}
	data, err := ExportNDJSON(before)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 2 {
		t.Fatal("hidden entry omitted")
	}
	var r Row
	if err := json.Unmarshal(lines[1], &r); err != nil || !r.Hidden {
		t.Fatalf("hidden state lost: %+v %v", r, err)
	}
	before.Catalog.Nodes[0].Title = "tampered"
	if out, err := ExportNDJSON(before); err == nil || len(out) != 0 {
		t.Fatal("tampered bundle exported")
	}
}

func TestCompareStableIdentityMoveAndReplacement(t *testing.T) {
	before, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.Catalog.Nodes = append([]catalog.Node(nil), before.Catalog.Nodes...)
	after.Catalog.Nodes[1].Slug = "proof"
	after.Digest = bundleDigest(after)
	d, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Moved, []Move{{"review.evidence", "review/evidence", "review/proof"}}) || len(d.Changed) != 0 || len(d.Added) != 0 || len(d.Removed) != 0 {
		t.Fatalf("pure move: %+v", d)
	}
	var encoded map[string]json.RawMessage
	data, _ := json.Marshal(d)
	if err := json.Unmarshal(data, &encoded); err != nil || string(encoded["moved"]) == "null" {
		t.Fatalf("moved not encoded: %s, %v", data, err)
	}
	after.Catalog.Nodes[1].Title = "Updated proof"
	after.Digest = bundleDigest(after)
	d, err = Compare(before, after)
	if err != nil || !reflect.DeepEqual(d.Changed, []string{"review/proof"}) || len(d.Moved) != 1 {
		t.Fatalf("move and change: %+v %v", d, err)
	}
	after.Catalog.Nodes[1].Slug = "evidence"
	after.Catalog.Nodes[1].StableKey = "new.evidence"
	after.Digest = bundleDigest(after)
	d, err = Compare(before, after)
	if err != nil || !reflect.DeepEqual(d.Added, []string{"review/evidence"}) || !reflect.DeepEqual(d.Removed, []string{"review/evidence"}) || len(d.Changed) != 0 || len(d.Moved) != 0 {
		t.Fatalf("key replacement: %+v %v", d, err)
	}
}

func TestCompareRefusesAmbiguityWithoutPartialResult(t *testing.T) {
	b, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	bad := b
	bad.Catalog.Nodes = append([]catalog.Node(nil), b.Catalog.Nodes...)
	bad.Catalog.Nodes[1].StableKey = "review"
	bad.Digest = bundleDigest(bad)
	if d, err := Compare(b, bad); err == nil || !reflect.DeepEqual(d, Difference{}) {
		t.Fatalf("ambiguous diff: %+v %v", d, err)
	}
	bad = b
	bad.Catalog.Nodes = append([]catalog.Node(nil), b.Catalog.Nodes...)
	bad.Catalog.Nodes[0].Title = "corrupt"
	if d, err := Compare(b, bad); err == nil || !reflect.DeepEqual(d, Difference{}) {
		t.Fatalf("corrupt diff: %+v %v", d, err)
	}
}

func TestCompareIdentitySwapAndRejectsInputOrder(t *testing.T) {
	before, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.Catalog.Nodes = append([]catalog.Node(nil), before.Catalog.Nodes...)
	// Both paths survive; identity, rather than the occupied ID, decides moves.
	after.Catalog.Nodes[0].StableKey, after.Catalog.Nodes[1].StableKey = after.Catalog.Nodes[1].StableKey, after.Catalog.Nodes[0].StableKey
	after.Digest = bundleDigest(after)
	d, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	want := []Move{{"review", "review", "review/evidence"}, {"review.evidence", "review/evidence", "review"}}
	if !reflect.DeepEqual(d.Moved, want) || len(d.Added) != 0 || len(d.Removed) != 0 {
		t.Fatalf("identity swap: %+v", d)
	}
	// A different traversal order breaks Catalog.Get's binary-search contract.
	order := after
	order.Catalog.Nodes = append([]catalog.Node(nil), after.Catalog.Nodes...)
	order.Catalog.Nodes[0], order.Catalog.Nodes[1] = order.Catalog.Nodes[1], order.Catalog.Nodes[0]
	order.Digest = bundleDigest(order)
	other, err := Compare(before, order)
	if err == nil || !reflect.DeepEqual(other, Difference{}) {
		t.Fatalf("noncanonical order accepted: %+v %v", other, err)
	}
}

func TestResolveRefusesInvalidUTF8FromGoCaller(t *testing.T) {
	s := fixture()
	s.Layers[0].Nodes[0].Title = string([]byte{0xff})
	if _, err := Resolve(s, Policy{}); err == nil {
		t.Fatal("invalid text accepted")
	}
}
