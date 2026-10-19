package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// otherEpochStats re-tags every vouching file_stats row as written under
// another ExtractorEpoch: the store a binary before an epoch bump left.
type otherEpochStats struct{ store.Store }

func (s otherEpochStats) FileStats(ctx context.Context, serviceID string) ([]store.FileStat, bool, error) {
	files, ok, err := s.Store.FileStats(ctx, serviceID)
	for i := range files {
		if r, vouches := statRecordFromRow(files[i]); vouches {
			r.Epoch = "old"
			files[i].ContentHash = statRowHash(r)
		}
	}
	return files, ok, err
}

// storedIndex is what an ingest leaves in the store for service, with the
// write times zeroed: the observations (calls included), the fingerprint
// digest, the file_stats rows and the summary's counts.
type storedIndex struct {
	Obs    []core.Observation
	Digest string
	Files  []store.FileStat
	Sum    store.ServiceSummary
}

func readStoredIndex(t *testing.T, st store.Store, service string) storedIndex {
	t.Helper()
	ctx := context.Background()
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range obs {
		obs[i].IngestedAt = time.Time{}
	}
	sort.Slice(obs, func(i, j int) bool {
		a, b := obs[i], obs[j]
		if a.SourceRef != b.SourceRef {
			return a.SourceRef < b.SourceRef
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Symbol < b.Symbol
	})
	digest, err := st.ServiceContentDigest(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := st.FileStats(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	sum, _, err := st.ServiceSummary(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	sum.Watermark, sum.WrittenAt = "", 0
	return storedIndex{Obs: obs, Digest: digest, Files: files, Sum: sum}
}

func readKeys(got map[string]int) string {
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// TestStatGateReadsFileStatsAcrossRestart is RT-180 (design §7.2): pass 1's
// prior stats come from the store's file_stats, so a pipeline built fresh
// over the same store, as a daemon restart or a one-shot CLI ingest builds
// one, reads only the files whose stat moved. A touched file is read alone; a
// changed file and a new file are read, a deleted one is purged; the stored
// index then equals a cold ingest's. A row written under another
// ExtractorEpoch vouches for nothing: every file is read.
func TestStatGateReadsFileStatsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"README.md": "# r\n"}
	for i := 0; i < 8; i++ {
		files[fmt.Sprintf("pkg/f%02d.go", i)] = fmt.Sprintf("package pkg\n\nfunc F%02d() int { return F%02d() + %d }\n", i, (i+1)%8, i)
	}
	writeCorpus(t, dir, files)
	backdate(t, dir)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	svc := core.ServiceRef{ID: "rs", Path: dir}
	ctx := context.Background()

	// process is one pipeline lifetime: open the store, build a pipeline
	// wired to it, ingest once, close. Nothing survives but the store.
	process := func(wrap func(store.Store) store.Store) (Report, map[string]int) {
		t.Helper()
		st, err := store.Open(storePath)
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		var s store.Store = st
		if wrap != nil {
			s = wrap(st)
		}
		conn := newCountingConn()
		p := New(conn, s, Config{Concurrency: 2, DisableTypedEdges: true})
		p.WithStoreStats()
		r, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		return r, conn.take()
	}

	if r, read := process(nil); len(read) != len(files) || r.StatSkipped != 0 {
		t.Fatalf("first ingest: read %d of %d files, stat-skipped %d", len(read), len(files), r.StatSkipped)
	}
	if r, read := process(nil); len(read) != 0 || r.StatSkipped != len(files) {
		t.Fatalf("restart over an unchanged tree: read %v, stat-skipped %d; want nothing read", read, r.StatSkipped)
	}

	// One file touched: its mtime alone moved.
	later := time.Now().Add(-30 * time.Minute) // moved, and outside the racy window
	if err := os.Chtimes(filepath.Join(dir, "pkg/f07.go"), later, later); err != nil {
		t.Fatal(err)
	}
	if r, read := process(nil); readKeys(read) != "pkg/f07.go" || r.StatSkipped != len(files)-1 {
		t.Fatalf("after touching one file: read %v, stat-skipped %d; want pkg/f07.go only", read, r.StatSkipped)
	}

	// One file changed, one new, one deleted.
	writeCorpus(t, dir, map[string]string{
		"pkg/f03.go":  "package pkg\n\nfunc F03() int { return 333 }\n",
		"pkg/new.go":  "package pkg\n\nfunc New() int { return F03() }\n",
		"docs/new.md": "# n\n",
	})
	for _, f := range []string{"pkg/f03.go", "pkg/new.go", "docs/new.md"} {
		if err := os.Chtimes(filepath.Join(dir, f), later, later); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "pkg/f05.go")); err != nil {
		t.Fatal(err)
	}
	if _, read := process(nil); readKeys(read) != "docs/new.md,pkg/f03.go,pkg/new.go" {
		t.Fatalf("after a change, an addition and a deletion: read %v; want exactly the changed and new files", read)
	}

	warm, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	got := readStoredIndex(t, warm, svc.ID)
	_ = warm.Close()
	cold, err := store.Open(filepath.Join(t.TempDir(), "cold.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer cold.Close()
	if _, err := New(newCountingConn(), cold, Config{Concurrency: 2, DisableTypedEdges: true}).Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	want := readStoredIndex(t, cold, svc.ID)
	if len(got.Obs) == 0 {
		t.Fatal("the warm store holds no observations")
	}
	for _, o := range got.Obs {
		if o.SourceRef == "pkg/f05.go" {
			t.Fatalf("the deleted file's observation %s survived", o.Symbol)
		}
	}
	if !reflect.DeepEqual(got.Obs, want.Obs) {
		t.Fatalf("observations after stat-gated restarts differ from a cold ingest's:\n got %+v\nwant %+v", got.Obs, want.Obs)
	}
	if got.Digest != want.Digest {
		t.Fatalf("fingerprint digest %s, cold %s", got.Digest, want.Digest)
	}
	if !reflect.DeepEqual(got.Files, want.Files) {
		t.Fatalf("file_stats differ from a cold ingest's:\n got %+v\nwant %+v", got.Files, want.Files)
	}
	if !reflect.DeepEqual(got.Sum, want.Sum) {
		t.Fatalf("summary differs from a cold ingest's:\n got %+v\nwant %+v", got.Sum, want.Sum)
	}

	// Rows written under another ExtractorEpoch: every file is read.
	other := func(st store.Store) store.Store { return otherEpochStats{st} }
	if r, read := process(other); len(read) != len(files)+1 || r.StatSkipped != 0 {
		t.Fatalf("rows from another epoch: read %d files, stat-skipped %d; want all %d read", len(read), r.StatSkipped, len(files)+1)
	}
}
