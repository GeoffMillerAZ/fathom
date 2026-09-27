package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestFileImportsLifecycle (RT-108): both backends keep file_imports per
// (service, path), and RemoveService, RenameService and PurgeService cover
// it; SourceObservations reads only the named refs.
func TestFileImportsLifecycle(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	for name, st := range map[string]interface {
		Store
		FileImportsStore
	}{"sqlite": sq, "memory": NewMemoryStore()} {
		t.Run(name, func(t *testing.T) {
			fi := FileImports{ServiceID: "a", Path: "x.js", ContentHash: "h1", ImportsJSON: `[{"Module":"./y"}]`}
			if err := st.SetFileImports(ctx, fi); err != nil {
				t.Fatal(err)
			}
			got, ok, err := st.GetFileImports(ctx, "a", "x.js")
			if err != nil || !ok || got != fi {
				t.Fatalf("round trip: %+v %v %v", got, ok, err)
			}
			fi.ContentHash, fi.ImportsJSON, fi.Fault = "h2", "", "parse fault"
			_ = st.SetFileImports(ctx, fi)
			if got, _, _ := st.GetFileImports(ctx, "a", "x.js"); got.Fault != "parse fault" || got.ContentHash != "h2" {
				t.Fatalf("upsert: %+v", got)
			}
			if err := st.RenameService(ctx, "a", "b"); err != nil {
				// a service with no observations may be refused; seed one.
				_ = st.PutObservations(ctx, []core.Observation{{ServiceID: "a", SourceRef: "x.js", Symbol: "f", SourceKind: core.KindJSFunc}})
				if err := st.RenameService(ctx, "a", "b"); err != nil {
					t.Fatal(err)
				}
			}
			if _, ok, _ := st.GetFileImports(ctx, "b", "x.js"); !ok {
				t.Fatal("RenameService did not carry file_imports")
			}
			if err := st.PurgeService(ctx, "b"); err != nil {
				t.Fatal(err)
			}
			if _, ok, _ := st.GetFileImports(ctx, "b", "x.js"); ok {
				t.Fatal("PurgeService left file_imports")
			}
			_ = st.SetFileImports(ctx, FileImports{ServiceID: "c", Path: "z.py", ContentHash: "h"})
			_ = st.PutObservations(ctx, []core.Observation{
				{ServiceID: "c", SourceRef: "z.py", Symbol: "g", SourceKind: core.KindPyFunc},
				{ServiceID: "c", SourceRef: "w.py", Symbol: "h", SourceKind: core.KindPyFunc},
			})
			obs, err := st.SourceObservations(ctx, "c", []string{"z.py"})
			if err != nil || len(obs) != 1 || obs[0].Symbol != "g" {
				t.Fatalf("SourceObservations: %+v %v", obs, err)
			}
			if _, err := st.RemoveService(ctx, "c"); err != nil {
				t.Fatal(err)
			}
			if _, ok, _ := st.GetFileImports(ctx, "c", "z.py"); ok {
				t.Fatal("RemoveService left file_imports")
			}
		})
	}
}
