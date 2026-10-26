package catalogkit

import (
	"testing"
	"time"
)

func TestBundleRefusesUnencodableProvenance(t *testing.T) {
	b, err := Resolve(fixture(), Policy{})
	if err != nil {
		t.Fatal(err)
	}
	b.Catalog.Nodes[0].Provenance.ObservedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	b.Digest = bundleDigest(b)
	if err := VerifyBundle(b); err == nil {
		t.Fatal("an unencodable bundle was accepted using a digest of empty bytes")
	}
	if _, err := LookupStableKey(b, b.Catalog.Nodes[0].StableKey); err == nil {
		t.Fatal("lookup accepted unencodable provenance")
	}
	if _, err := Compare(b, b); err == nil {
		t.Fatal("comparison accepted unencodable provenance")
	}
}
