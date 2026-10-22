package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileStatForPointReceipt(t *testing.T) {
	sql, err := NewSQLiteStore(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer sql.Close()
	for name, st := range map[string]Store{"memory": NewMemoryStore(), "sqlite": sql} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			want := FileStat{Path: "sample.go", Size: 42, Ino: 7, MtimeNS: 10, ContentHash: "previous:unvouched:digest", Lang: "GoFunc"}
			if err := st.WriteServiceSummary(ctx, ServiceMeta{ServiceID: "sample"}, ServiceSummary{}, []FileStat{want, {Path: "other.go", Size: 9}}); err != nil {
				t.Fatal(err)
			}
			point := st.(interface {
				FileStatFor(context.Context, string, string) (FileStat, bool, error)
			})
			got, found, err := point.FileStatFor(ctx, "sample", "sample.go")
			if err != nil || !found || got != want {
				t.Fatalf("point=(%+v,%t,%v), want %+v", got, found, err, want)
			}
			if _, found, err := point.FileStatFor(ctx, "sample", "missing.go"); err != nil || found {
				t.Fatalf("missing=(%t,%v)", found, err)
			}
			if _, found, err := point.FileStatFor(ctx, "other", "sample.go"); err != nil || found {
				t.Fatalf("other service=(%t,%v)", found, err)
			}
		})
	}
}
