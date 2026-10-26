package conserve

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestConcernDeltasFingerprintBasis(t *testing.T) {
	for _, mode := range []string{"dirty-ingest-then-revert", "absent-at-head", "extractor-skew", "missing-fingerprint"} {
		t.Run(mode, func(t *testing.T) {
			r := newInputTestRepo(t)
			const path = "deploy/api.yaml"
			r.write(path, concernManifest("api", 3))
			r.commitAll("base")
			base := r.headSHA()
			if mode == "absent-at-head" {
				r.run("rm", path)
			} else {
				r.write(path, concernManifest("api", 1))
			}
			r.commitAll("head")
			st := store.NewMemoryStore()
			if mode == "dirty-ingest-then-revert" || mode == "absent-at-head" {
				r.write(path, concernManifest("api", 9))
			}
			concernIngest(t, r, st)
			switch mode {
			case "dirty-ingest-then-revert":
				r.run("restore", "--", path)
			case "absent-at-head":
				if err := os.Remove(filepath.Join(r.root, path)); err != nil {
					t.Fatal(err)
				}
			case "extractor-skew":
				if err := st.SetFingerprint(context.Background(), "svc", path, "previous-extractor-epoch"); err != nil {
					t.Fatal(err)
				}
			case "missing-fingerprint":
				if err := st.DeleteFingerprint(context.Background(), "svc", path); err != nil {
					t.Fatal(err)
				}
			}
			rep, err := ConcernDeltas(context.Background(), concernInput(t, r, base, st))
			if err != nil {
				t.Fatal(err)
			}
			if rep.Basis.State != "uncommitted" || rep.Basis.IndexCommit != r.headSHA() {
				t.Fatalf("false exact basis: %+v", rep.Basis)
			}
			if len(rep.Deltas) != 1 {
				t.Fatalf("deltas: %+v", rep.Deltas)
			}
			d := rep.Deltas[0]
			want := "1"
			if mode == "dirty-ingest-then-revert" || mode == "absent-at-head" {
				want = "9"
			}
			if d.Basis != rep.Basis || d.After == nil || *d.After != want {
				t.Fatalf("index evidence replaced: %+v", d)
			}
		})
	}
}

func TestConcernDeltasStoreBytesUnchanged(t *testing.T) {
	r := newInputTestRepo(t)
	r.write("deploy/api.yaml", concernManifest("api", 3))
	r.commitAll("base")
	base := r.headSHA()
	r.write("deploy/api.yaml", concernManifest("api", 1))
	r.commitAll("head")
	db := filepath.Join(t.TempDir(), "index.sqlite")
	st, err := store.NewSQLiteStore(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	concernIngest(t, r, st)
	in := concernInput(t, r, base, st)
	hashes := func() map[string][32]byte {
		result := map[string][32]byte{}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			data, err := os.ReadFile(db + suffix)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			result[suffix] = sha256.Sum256(data)
		}
		return result
	}
	before := hashes()
	for range 2 {
		rep, err := ConcernDeltas(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Deltas) != 1 || rep.Basis.State != "exact" {
			t.Fatalf("report: %+v", rep)
		}
		if !reflect.DeepEqual(before, hashes()) {
			t.Fatal("concern deltas wrote sqlite, WAL or SHM bytes")
		}
	}
}
