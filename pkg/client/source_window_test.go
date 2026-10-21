package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func windowFixture(t *testing.T) (*Client, *store.MemoryStore, string) {
	t.Helper()
	root := t.TempDir()
	small := []byte("[FR-1]")
	large := bytes.Repeat([]byte("x"), connector.MaxFetchSize+17)
	for name, data := range map[string][]byte{"a.go": small, "large.dat": large} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	conn := connector.NewLocalConnector()
	ctx := context.Background()
	policy, err := conn.DiscoveryPolicyDigest(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(small)
	p := &store.SourceRefProjection{Policy: policy, ExtractorVersion: store.SourceRefExtractorVersion, Rows: []store.SourceRefFact{
		{Path: "a.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: store.SourceRefExtractorVersion, Refs: []string{"[FR-1]"}},
		{Path: "large.dat", ExtractorVersion: store.SourceRefExtractorVersion, Excluded: "oversize"},
	}}
	st := store.NewMemoryStore()
	meta := store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "head", IndexedAt: 1}
	if err := st.WriteServiceSummary(ctx, meta, store.ServiceSummary{RefProjection: p}, []store.FileStat{{Path: "a.go"}, {Path: "large.dat", Excluded: "oversize"}}); err != nil {
		t.Fatal(err)
	}
	return New(conn, st), st, root
}

func advanceWindowPass(t *testing.T, c *Client, w *SourceWindow, want int) {
	t.Helper()
	for i := 0; i < 30 && w.Pass == want && !w.Ready; i++ {
		before := w.Bytes
		if err := c.AdvanceSourceWindow(context.Background(), w, 256<<10, 5*time.Second); err != nil {
			t.Fatal(err)
		}
		if w.Bytes-before > 256<<10 {
			t.Fatalf("unit read %d bytes", w.Bytes-before)
		}
	}
	if want == 0 && w.Pass != 1 || want == 1 && !w.Ready {
		t.Fatalf("window did not complete pass %d: %+v", want, w)
	}
}

func TestSourceWindowChunksLargeCorpusAndCloses(t *testing.T) {
	c, _, _ := windowFixture(t)
	w, err := c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	advanceWindowPass(t, c, w, 0)
	if w.Ready || w.Projection.Rows[1].Refs != nil {
		t.Fatal("provisional window or excluded refs became final")
	}
	advanceWindowPass(t, c, w, 1)
	wantBytes := int64(2 * (len("[FR-1]") + connector.MaxFetchSize + 17))
	if !w.Ready || w.Bytes != wantBytes || w.Files != 4 || w.Offset != 0 {
		t.Fatalf("two-pass bytes=%d ready=%v", w.Bytes, w.Ready)
	}
}

func TestSourceWindowRefusesDriftAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *store.MemoryStore, string)
	}{
		{"examined bytes", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.WriteFile(filepath.Join(root, "a.go"), []byte("[FR-2]"), 0600)
		}},
		{"new member", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.WriteFile(filepath.Join(root, "new.go"), []byte("new"), 0600)
		}},
		{"policy", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.WriteFile(filepath.Join(root, ".fathomignore"), []byte("new.go\n"), 0600)
		}},
		{"excluded bytes", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.WriteFile(filepath.Join(root, "large.dat"), bytes.Repeat([]byte("z"), connector.MaxFetchSize+17), 0600)
		}},
		{"excluded eligibility", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.WriteFile(filepath.Join(root, "large.dat"), []byte("small"), 0600)
		}},
		{"symlink", func(t *testing.T, _ *store.MemoryStore, root string) {
			os.Remove(filepath.Join(root, "large.dat"))
			os.Symlink(filepath.Join(root, "a.go"), filepath.Join(root, "large.dat"))
		}},
		{"watermark", func(t *testing.T, st *store.MemoryStore, _ string) {
			st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: "/changed", IndexedCommit: "new", IndexedAt: 2})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, st, root := windowFixture(t)
			w, err := c.BeginSourceWindow(context.Background(), "svc")
			if err != nil {
				t.Fatal(err)
			}
			advanceWindowPass(t, c, w, 0)
			tc.mutate(t, st, root)
			failed := false
			for i := 0; i < 30 && !w.Ready && !failed; i++ {
				failed = c.AdvanceSourceWindow(context.Background(), w, 256<<10, 5*time.Second) != nil
			}
			if !failed || !w.Failed || w.Ready {
				t.Fatalf("drift accepted: failed=%v state=%+v", failed, w)
			}
		})
	}
	c, _, _ := windowFixture(t)
	w, err := c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.AdvanceSourceWindow(ctx, w, 256<<10, time.Second); err == nil || !w.Failed {
		t.Fatalf("cancel accepted: %v %+v", err, w)
	}
	if !strings.Contains(c.AdvanceSourceWindow(context.Background(), w, 256<<10, time.Second).Error(), "invalid continuation") {
		t.Fatal("failed window replayed")
	}
}

func TestSourceWindowRejectsMutationBetweenPartialOffsets(t *testing.T) {
	c, _, root := windowFixture(t)
	w, err := c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	for w.Index == 0 {
		if err := c.AdvanceSourceWindow(context.Background(), w, 64<<10, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.AdvanceSourceWindow(context.Background(), w, 64<<10, time.Second); err != nil {
		t.Fatal(err)
	}
	if w.Index != 1 || w.Offset == 0 {
		t.Fatalf("large file did not retain offset: %+v", w)
	}
	if err := os.WriteFile(filepath.Join(root, "large.dat"), bytes.Repeat([]byte("z"), connector.MaxFetchSize+17), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.AdvanceSourceWindow(context.Background(), w, 64<<10, time.Second); err == nil || !w.Failed {
		t.Fatalf("partial-file mutation accepted: %v %+v", err, w)
	}
}

type windowDriftStats struct {
	*store.MemoryStore
	change func(*store.FileStat)
}

func (s *windowDriftStats) FileStats(ctx context.Context, service string) ([]store.FileStat, bool, error) {
	rows, ok, err := s.MemoryStore.FileStats(ctx, service)
	if s.change != nil && len(rows) > 0 {
		s.change(&rows[0])
	}
	return rows, ok, err
}

func TestSourceWindowRefusesEveryFileStatFieldDrift(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*store.FileStat)
	}{
		{"size", func(f *store.FileStat) { f.Size++ }},
		{"mtime", func(f *store.FileStat) { f.MtimeNS++ }},
		{"inode", func(f *store.FileStat) { f.Ino++ }},
		{"content hash", func(f *store.FileStat) { f.ContentHash = "changed" }},
		{"language", func(f *store.FileStat) { f.Lang = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, st, _ := windowFixture(t)
			drift := &windowDriftStats{MemoryStore: st}
			c.st = drift
			w, err := c.BeginSourceWindow(context.Background(), "svc")
			if err != nil {
				t.Fatal(err)
			}
			advanceWindowPass(t, c, w, 0)
			drift.change = tc.change
			for i := 0; i < 30 && !w.Ready && !w.Failed; i++ {
				_ = c.AdvanceSourceWindow(context.Background(), w, 256<<10, 5*time.Second)
			}
			if !w.Failed || w.Ready {
				t.Fatalf("file_stats %s drift accepted", tc.name)
			}
		})
	}
}

func TestSourceWindowAbandonAndGovernorRefuse(t *testing.T) {
	c, _, _ := windowFixture(t)
	w, err := c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	abandon := make(chan struct{})
	ctx, done := WithSourcesScope(context.Background(), abandon)
	defer done()
	close(abandon)
	if err := c.AdvanceSourceWindow(ctx, w, 256<<10, time.Second); err == nil || !w.Failed {
		t.Fatalf("abandoned read accepted: %v", err)
	}
	c, _, _ = windowFixture(t)
	w, err = c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	tenancy.PublishLevel(tenancy.LevelRefuse)
	defer tenancy.PublishLevel(tenancy.LevelNormal)
	if err := c.AdvanceSourceWindow(context.Background(), w, 256<<10, time.Second); err == nil || !w.Failed || !strings.Contains(err.Error(), "producer refusal") {
		t.Fatalf("governor read accepted: %v", err)
	}
}

func TestSourceWindowLateReadAndMidReadRefusalInvalidate(t *testing.T) {
	c, _, _ := windowFixture(t)
	w, err := c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	entered := false
	w.readChunk = func(ctx context.Context, f *os.File, buf []byte) (int, error) {
		entered = true
		<-ctx.Done()
		return f.Read(buf)
	}
	if err := c.AdvanceSourceWindow(context.Background(), w, 256<<10, time.Second); !entered || !errors.Is(err, context.DeadlineExceeded) || !w.Failed || w.Ready || w.Bytes != 0 {
		t.Fatalf("late read accepted: entered=%v err=%v failed=%v ready=%v bytes=%d", entered, err, w.Failed, w.Ready, w.Bytes)
	}

	c, _, _ = windowFixture(t)
	w, err = c.BeginSourceWindow(context.Background(), "svc")
	if err != nil {
		t.Fatal(err)
	}
	w.readChunk = func(_ context.Context, f *os.File, buf []byte) (int, error) {
		n, err := f.Read(buf)
		tenancy.PublishLevel(tenancy.LevelRefuse)
		return n, err
	}
	defer tenancy.PublishLevel(tenancy.LevelNormal)
	if err := c.AdvanceSourceWindow(context.Background(), w, 256<<10, time.Second); err == nil || !w.Failed || w.Bytes != 0 || !strings.Contains(err.Error(), "producer refusal") {
		t.Fatalf("mid-read refusal accepted: err=%v failed=%v bytes=%d", err, w.Failed, w.Bytes)
	}
}
