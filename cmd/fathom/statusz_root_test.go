package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// seedStatuszRootStore records each service (one observation, so Services
// lists it) with its corpus root and indexed commit.
func seedStatuszRootStore(t *testing.T, path string, metas ...store.ServiceMeta) {
	t.Helper()
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	for _, m := range metas {
		if err := st.PutObservations(ctx, []core.Observation{{
			ServiceID: m.ServiceID, SourceRef: "a.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: "Foo",
		}}); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		if err := st.SetServiceMeta(ctx, m); err != nil {
			t.Fatalf("SetServiceMeta: %v", err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// statuszRootJSON is statuszForRoot's answer for dir, decoded and raw.
func statuszRootJSON(t *testing.T, c *client.Client, dir string) (StatuszReport, string) {
	t.Helper()
	now := time.Now()
	r := statuszForRoot(context.Background(), c, now.Add(-time.Minute), now, dir)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back StatuszReport
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, raw)
	}
	return back, string(raw)
}

// statuszOnlyRow is the one service row a root answer carries.
func statuszOnlyRow(t *testing.T, r StatuszReport, raw string) StatuszServiceRow {
	t.Helper()
	if r.Root == nil || r.Root.Indexed == nil || !*r.Root.Indexed {
		t.Fatalf("root = %+v, want indexed true\n%s", r.Root, raw)
	}
	if len(r.Services) != 1 || r.Services[0].Service != r.Root.Service || r.Services[0].StatuszHead == nil {
		t.Fatalf("services = %+v, want exactly the root's service with its head fields\n%s", r.Services, raw)
	}
	return r.Services[0]
}

// TestStatuszForRootNeverReadsUnknownAsZero holds /statusz's root query to
// its contract: the one service whose root holds the directory (the deepest
// when roots nest), its HEAD, commits behind and dirty state from the
// read-only git wrapper, and null with a reason, never 0 or false, for
// every reading that could not be taken; without the query the JSON is
// unchanged.
func TestStatuszForRootNeverReadsUnknownAsZero(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c1 := gitRepo(t, repo)
	gitro.ResetHeadCache()
	t.Cleanup(gitro.ResetHeadCache)
	elsewhere := t.TempDir()
	gone := filepath.Join(elsewhere, "gone")
	lost := filepath.Join(elsewhere, "lost")
	unstamped := filepath.Join(elsewhere, "unstamped")
	for _, d := range []string{lost, unstamped} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		gitRepo(t, d)
	}
	bogus := strings.Repeat("ab", 20)

	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	seedStatuszRootStore(t, storePath,
		store.ServiceMeta{ServiceID: "outer", Path: base, IndexedCommit: c1},
		store.ServiceMeta{ServiceID: "fresh", Path: repo, IndexedCommit: c1},
		store.ServiceMeta{ServiceID: "gone", Path: gone, IndexedCommit: c1},
		store.ServiceMeta{ServiceID: "lost", Path: lost, IndexedCommit: bogus},
		store.ServiceMeta{ServiceID: "unstamped", Path: unstamped, IndexedCommit: ""},
	)
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()

	t.Run("indexed at HEAD, clean", func(t *testing.T) {
		r, raw := statuszRootJSON(t, c, filepath.Join(repo, "sub"))
		row := statuszOnlyRow(t, r, raw)
		if row.Service != "fresh" {
			t.Fatalf("service = %q, want the deepest root's (fresh)\n%s", row.Service, raw)
		}
		if row.HeadCommit == nil || *row.HeadCommit != c1 || row.Behind == nil || *row.Behind != 0 || row.Dirty == nil || *row.Dirty {
			t.Fatalf("head = %+v, want head %s, behind 0, dirty false\n%s", row.StatuszHead, c1, raw)
		}
	})

	t.Run("a tracked file changed", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r, raw := statuszRootJSON(t, c, repo)
		row := statuszOnlyRow(t, r, raw)
		if row.Dirty == nil || !*row.Dirty || row.Behind == nil || *row.Behind != 0 {
			t.Fatalf("head = %+v, want dirty true, behind 0\n%s", row.StatuszHead, raw)
		}
	})

	t.Run("one commit past the index", func(t *testing.T) {
		gitCommit(t, repo, "b.txt", "b\n")
		r, raw := statuszRootJSON(t, c, repo)
		row := statuszOnlyRow(t, r, raw)
		if row.Behind == nil || *row.Behind != 1 || row.HeadCommit == nil || *row.HeadCommit == c1 {
			t.Fatalf("head = %+v, want behind 1 at a new HEAD\n%s", row.StatuszHead, raw)
		}
		if row.Dirty == nil || *row.Dirty {
			t.Fatalf("dirty = %v, want false after the commit took every change\n%s", row.Dirty, raw)
		}
	})

	for _, tc := range []struct{ name, dir, service string }{
		{"an unreadable checkout", gone, "gone"},
		{"an indexed commit the checkout lacks", lost, "lost"},
		{"no indexed commit recorded", unstamped, "unstamped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, raw := statuszRootJSON(t, c, tc.dir)
			row := statuszOnlyRow(t, r, raw)
			if row.Service != tc.service {
				t.Fatalf("service = %q, want %q\n%s", row.Service, tc.service, raw)
			}
			if row.Behind != nil || row.BehindReason == "" {
				t.Fatalf("behind = %v (%q), want null with a reason\n%s", row.Behind, row.BehindReason, raw)
			}
			if !strings.Contains(raw, `"behind":null`) {
				t.Fatalf("JSON lacks an explicit behind null\n%s", raw)
			}
			if tc.service != "gone" && (row.HeadCommit == nil || row.Dirty == nil || *row.Dirty) {
				t.Fatalf("head = %+v, want HEAD read and dirty false beside the unknown behind\n%s", row.StatuszHead, raw)
			}
			if tc.service == "gone" {
				if row.Dirty != nil || row.DirtyReason == "" || row.HeadCommit != nil || !strings.Contains(raw, `"dirty":null`) {
					t.Fatalf("head = %+v, want dirty and head_commit null with reasons\n%s", row.StatuszHead, raw)
				}
			}
		})
	}

	t.Run("no service root holds the directory", func(t *testing.T) {
		other := t.TempDir()
		r, raw := statuszRootJSON(t, c, other)
		if r.Root == nil || r.Root.Indexed == nil || *r.Root.Indexed || r.Root.Service != "" || len(r.Services) != 0 {
			t.Fatalf("root = %+v services = %+v, want indexed false and no rows\n%s", r.Root, r.Services, raw)
		}
	})

	t.Run("a relative directory is unknown, not unindexed", func(t *testing.T) {
		r, raw := statuszRootJSON(t, c, "repo/sub")
		if r.Root == nil || r.Root.Indexed != nil || r.Root.Reason == "" || !strings.Contains(raw, `"indexed":null`) {
			t.Fatalf("root = %+v, want indexed null with a reason\n%s", r.Root, raw)
		}
	})

	t.Run("without root the JSON carries none of it", func(t *testing.T) {
		now := time.Now()
		raw, err := json.Marshal(buildStatuszReport(context.Background(), c, now, now))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{`"root"`, `"head_commit"`, `"behind"`, `"dirty"`, `"indexed"`} {
			if bytes.Contains(raw, []byte(key)) {
				t.Errorf("plain /statusz JSON carries %s\n%s", key, raw)
			}
		}
	})

	t.Run("served at GET /statusz", func(t *testing.T) {
		srv := mcp.NewServer(c)
		h := newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
		req := httptest.NewRequest(http.MethodGet, "/statusz?format=json&root="+url.QueryEscape(repo), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
		}
		var r StatuszReport
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, rec.Body.String())
		}
		row := statuszOnlyRow(t, r, rec.Body.String())
		if row.Service != "fresh" || r.Root.Dir != repo {
			t.Fatalf("root = %+v row = %+v, want fresh for %s", r.Root, row, repo)
		}
	})
}
