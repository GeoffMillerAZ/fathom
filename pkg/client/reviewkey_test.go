package client_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestClientReviewKey_Deterministic proves pkg/client.ReviewKey (FL-19)
// resolves a Request into a Key deterministically: two calls with the same
// manifest over the same corpus produce the identical key, and an
// unreadable/unknown path is honestly reported rather than silently
// hashing as empty.
func TestClientReviewKey_Deterministic(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ms := store.NewMemoryStore()
	if err := ms.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), ms)
	if err := c.IngestService(ctx, "svc"); err != nil {
		t.Fatalf("IngestService: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "x.go"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	req := reviewkey.Request{
		Service:  "svc",
		Manifest: []reviewkey.ManifestEntry{{Path: "x.go", SHA256: sha}},
	}
	first, err := c.ReviewKey(ctx, req)
	if err != nil {
		t.Fatalf("ReviewKey: %v", err)
	}
	if first.SchemaVersion != reviewkey.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", first.SchemaVersion, reviewkey.SchemaVersion)
	}
	if first.Key == "" {
		t.Fatal("Key must not be empty")
	}

	second, err := c.ReviewKey(ctx, req)
	if err != nil {
		t.Fatalf("ReviewKey (second call): %v", err)
	}
	if first.Key != second.Key {
		t.Fatalf("ReviewKey is not deterministic: %s vs %s", first.Key, second.Key)
	}

	// An unreadable manifest path is excluded and named, never hashed empty.
	reqUnreadable := reviewkey.Request{
		Service:  "svc",
		Manifest: []reviewkey.ManifestEntry{{Path: "does-not-exist.go", Unreadable: true}},
	}
	unreadableKey, err := c.ReviewKey(ctx, reqUnreadable)
	if err != nil {
		t.Fatalf("ReviewKey (unreadable): %v", err)
	}
	if unreadableKey.Complete {
		t.Fatal("an unreadable manifest path must make Complete false")
	}
	found := false
	for _, u := range unreadableKey.Unknown {
		if u == "manifest: does-not-exist.go is unreadable" {
			found = true
		}
	}
	if !found {
		t.Errorf("Unknown = %v, want an entry naming does-not-exist.go", unreadableKey.Unknown)
	}

	// Diff against a prior key.
	req.Prior = &first
	same, err := c.ReviewKey(ctx, req)
	if err != nil {
		t.Fatalf("ReviewKey (with prior): %v", err)
	}
	if len(same.ChangedSincePrior) != 0 {
		t.Errorf("ChangedSincePrior = %v, want none for an identical re-run", same.ChangedSincePrior)
	}
}
