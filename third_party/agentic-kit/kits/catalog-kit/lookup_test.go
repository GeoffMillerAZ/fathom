package catalogkit

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

func TestLookupStableKeyExactDetachedAndHidden(t *testing.T) {
	b, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	n, err := LookupStableKey(b, "review.evidence")
	if err != nil || n.ID() != "review/evidence" || !n.Hidden {
		t.Fatalf("hidden lookup: %+v %v", n, err)
	}
	n.Facets["language"][0] = "changed"
	n.Provenance.ContributingLayers[0] = "changed"
	original, err := LookupStableKey(b, "review.evidence")
	if err != nil || original.Facets["language"][0] != "go" || original.Provenance.ContributingLayers[0] != "general" {
		t.Fatalf("lookup aliases bundle: %+v %v", original, err)
	}
	for _, key := range []string{"", "Review.Evidence", string([]byte{0xff}), "absent"} {
		if got, err := LookupStableKey(b, key); err == nil || got.ID() != "" {
			t.Fatalf("invalid key %q: %+v %v", key, got, err)
		}
	}
	b.Catalog.Nodes[0].Title = "corrupt"
	if got, err := LookupStableKey(b, "review.evidence"); err == nil || got.ID() != "" {
		t.Fatalf("corrupt bundle: %+v %v", got, err)
	}
}

func TestCanonicalNodeOrderRequiredAcrossBundleConsumers(t *testing.T) {
	good, bind, definition, evidence := decisionFixture(t)
	bad := good
	bad.Catalog.Nodes = append([]catalog.Node(nil), good.Catalog.Nodes...)
	bad.Catalog.Nodes[0], bad.Catalog.Nodes[1] = bad.Catalog.Nodes[1], bad.Catalog.Nodes[0]
	bad.Digest = bundleDigest(bad)
	if err := VerifyBundle(bad); err == nil {
		t.Fatal("reordered catalog verified")
	}
	data, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeBundle(data); err == nil || !reflect.DeepEqual(got, Bundle{}) {
		t.Fatalf("reordered catalog decoded: %+v %v", got, err)
	}
	if got, err := LookupStableKey(bad, "review.evidence"); err == nil || got.ID() != "" {
		t.Fatalf("reordered lookup: %+v %v", got, err)
	}
	if diff, err := Compare(good, bad); err == nil || !reflect.DeepEqual(diff, Difference{}) {
		t.Fatalf("reordered diff: %+v %v", diff, err)
	}
	if diff, err := Compare(bad, good); err == nil || !reflect.DeepEqual(diff, Difference{}) {
		t.Fatalf("reordered before diff: %+v %v", diff, err)
	}
	if prepared, err := PrepareDecision(bad, bind, definition, evidence, 4096); err == nil || !reflect.DeepEqual(prepared, Prepared{}) {
		t.Fatalf("reordered decision: %+v %v", prepared, err)
	}
}

func TestLookupStableKeyRefusesAmbiguousBundle(t *testing.T) {
	b, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	b.Catalog.Nodes[1].StableKey = "review"
	b.Digest = bundleDigest(b)
	if _, err := LookupStableKey(b, "review"); err == nil {
		t.Fatal("duplicate stable key accepted")
	}
}

func TestLookupDoesNotGrantDecisionAuthority(t *testing.T) {
	b, bind, definition, evidence := decisionFixture(t)
	b.Catalog.Nodes[1].Hidden = true
	b.Digest = bundleDigest(b)
	n, err := LookupStableKey(b, "review.evidence")
	if err != nil || !n.Hidden {
		t.Fatalf("hidden lookup: %+v %v", n, err)
	}
	bind.NodeDigest = NodeDigest(n)
	if _, err := PrepareDecision(b, bind, definition, evidence, 4096); err == nil {
		t.Fatal("hidden lookup granted authority")
	}
	b.Catalog.Nodes[1].Hidden = false
	b.Catalog.Nodes[1].Slug = "proof"
	b.Digest = bundleDigest(b)
	n, err = LookupStableKey(b, "review.evidence")
	if err != nil || n.ID() != "review/proof" {
		t.Fatalf("moved lookup: %+v %v", n, err)
	}
	if _, err := PrepareDecision(b, bind, definition, evidence, 4096); err == nil {
		t.Fatal("stale path binding accepted")
	}
}

func TestStableIdentityRejectsMalformedRedigestedCatalog(t *testing.T) {
	good, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*Bundle){
		"duplicate ID distinct key": func(b *Bundle) {
			n := b.Catalog.Nodes[0]
			n.StableKey = "other"
			b.Catalog.Nodes = append(b.Catalog.Nodes, n)
		},
		"invalid slug":        func(b *Bundle) { b.Catalog.Nodes[0].Slug = "Bad Slug" },
		"missing parent":      func(b *Bundle) { b.Catalog.Nodes[1].ParentID = "absent" },
		"invalid parent path": func(b *Bundle) { b.Catalog.Nodes[1].ParentID = "review//bad" },
		"invalid key text":    func(b *Bundle) { b.Catalog.Nodes[0].StableKey = string([]byte{0xff}) },
		"invalid title text":  func(b *Bundle) { b.Catalog.Nodes[0].Title = string([]byte{0xff}) },
		"invalid facet text":  func(b *Bundle) { b.Catalog.Nodes[1].Facets = map[string][]string{"a": {string([]byte{0xff})}} },
		"invalid source text": func(b *Bundle) { b.Catalog.Nodes[0].Provenance.Source = string([]byte{0xff}) },
		"missing source":      func(b *Bundle) { b.Catalog.Nodes[0].Provenance.Source = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			bad := good
			bad.Catalog.Nodes = make([]catalog.Node, len(good.Catalog.Nodes))
			for i, n := range good.Catalog.Nodes {
				bad.Catalog.Nodes[i] = cloneNode(n)
			}
			mutate(&bad)
			bad.Digest = bundleDigest(bad)
			if got, err := LookupStableKey(bad, "review.evidence"); err == nil || got.ID() != "" {
				t.Fatalf("lookup accepted malformed bundle: %+v %v", got, err)
			}
			for _, pair := range [][2]Bundle{{good, bad}, {bad, good}} {
				if diff, err := Compare(pair[0], pair[1]); err == nil || !reflect.DeepEqual(diff, Difference{}) {
					t.Fatalf("compare accepted malformed bundle: %+v %v", diff, err)
				}
			}
		})
	}
}
