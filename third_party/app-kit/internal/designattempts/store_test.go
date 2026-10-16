package designattempts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

func attemptFixture(t *testing.T) (string, []designconvert.Translation, map[string][]byte) {
	t.Helper()
	src, root := t.TempDir(), filepath.Join(t.TempDir(), "assets")
	var body bytes.Buffer
	z := zip.NewWriter(&body)
	for _, member := range []struct{ name, content string }{
		{"html/static.html", "<p class=\"dark:text-white\">Static <strong>source</strong></p>"},
		{"html/interactive.html", "<el-dialog><button>Open</button></el-dialog>"},
		{"html/different.html", "<p>Another original</p>"},
	} {
		w, err := z.Create(member.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(member.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	download := filepath.Join(src, "marketing-v4.zip")
	if err := os.WriteFile(download, body.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := designassets.Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	a := catalog.List()[0]
	var results []designconvert.Translation
	for _, name := range []string{"html/static.html", "html/interactive.html"} {
		original, err := designassets.ReadFile(root, a.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := designconvert.Translate(designconvert.Source{ArchiveID: a.ID, Path: name,
			SHA256: original.SHA256, Size: len(original.Content)}, []byte(original.Content))
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, r)
	}
	preserved := map[string][]byte{}
	for _, name := range []string{download, filepath.Join(root, "catalog.json"), filepath.Join(root, "archives", a.SHA256+".zip")} {
		preserved[name], err = os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
	}
	return root, results, preserved
}

func assertPreserved(t *testing.T, files map[string][]byte) {
	t.Helper()
	for name, want := range files {
		got, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("original changed: %s: %v", name, err)
		}
	}
}

func TestPublishAndReadPreserveOriginalsAndUnsupportedAttempts(t *testing.T) {
	root, results, originals := attemptFixture(t)
	empty, err := Load(root)
	if err != nil || len(empty.List()) != 0 {
		t.Fatalf("optional empty store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "translations")); !os.IsNotExist(err) {
		t.Fatal("read created storage")
	}
	for _, result := range results {
		entry, err := Publish(root, result)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(filepath.Join(root, "translations", "catalog.json"))
		again, err := Publish(root, result)
		after, _ := os.ReadFile(filepath.Join(root, "translations", "catalog.json"))
		if err != nil || entry.ID != again.ID || !bytes.Equal(before, after) {
			t.Fatalf("save not idempotent: %v", err)
		}
		detail, err := Read(root, entry.ID)
		if err != nil || detail.Replay != "matches-current" || detail.Entry.Verification != "not-verified" {
			t.Fatalf("exact read: %+v, %v", detail.Entry, err)
		}
		want, _ := json.Marshal(result)
		got, _ := json.Marshal(detail.Translation)
		if !bytes.Equal(want, got) || result.Status == "unsupported" && detail.Translation.Artifact != nil {
			t.Fatal("read changed receipt or fabricated unsupported output")
		}
	}
	c, err := Load(root)
	if err != nil || len(c.List()) != 2 || c.Revision == empty.Revision {
		t.Fatalf("discovery: %v", err)
	}
	for _, name := range []string{"translations", "translations/entries", "translations/catalog.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("private permissions missing: %s %v", name, err)
		}
	}
	assertPreserved(t, originals)
}

func TestPublishRefusesForgedOriginAndReplayWithoutWrites(t *testing.T) {
	for _, mode := range []string{"missing source", "wrong member", "changed content", "stale", "certification"} {
		t.Run(mode, func(t *testing.T) {
			root, results, originals := attemptFixture(t)
			r := results[0]
			switch mode {
			case "missing source":
				r.Source.ArchiveID = "marketing@" + strings.Repeat("a", 64)
			case "wrong member":
				r.Source.Path = "html/different.html"
			case "changed content":
				r.Artifact.Content += "changed"
				r.Artifact.SHA256 = digest([]byte(r.Artifact.Content))
			case "stale":
				r.Transformer.GoVersion = "go1.25.0"
			case "certification":
				r.Verification = "verified"
			}
			if _, err := Publish(root, r); err == nil {
				t.Fatal("accepted forged/stale attempt")
			}
			if _, err := os.Stat(filepath.Join(root, "translations")); !os.IsNotExist(err) {
				t.Fatal("invalid attempt created store")
			}
			assertPreserved(t, originals)
		})
	}
}

// Simulate a saved same-schema historical attempt. There is intentionally no
// public API for saving a forged/stale receipt as freshly replayed evidence.
func installAttempt(t *testing.T, root string, result designconvert.Translation) Entry {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	entry := entryFor(result, body)
	if err := os.MkdirAll(filepath.Join(root, "translations", "entries"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "translations", entryPath(entry.ID)), body, 0600); err != nil {
		t.Fatal(err)
	}
	index, err := encodeCatalog([]Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "translations", "catalog.json"), index, 0600); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestReadDistinguishesHistoricalTransformerFromTampering(t *testing.T) {
	t.Run("historical", func(t *testing.T) {
		root, results, originals := attemptFixture(t)
		results[0].Transformer.GoVersion = "go1.25.0"
		results[0].Transformer.HTMLParser = "golang.org/x/net@v0.53.0"
		results[0].Transformer.RequiredTempl = "github.com/a-h/templ@v0.3.1000"
		entry := installAttempt(t, root, results[0])
		d, err := Read(root, entry.ID)
		if err != nil || d.Replay != "stale-transformer" || d.Translation.Verification != "not-verified" {
			t.Fatalf("historical attempt misreported: %s %v", d.Replay, err)
		}
		assertPreserved(t, originals)
	})
	t.Run("rehashed artifact", func(t *testing.T) {
		root, results, _ := attemptFixture(t)
		results[0].Artifact.Content = strings.ReplaceAll(results[0].Artifact.Content, "dark:text-white", "dark:text-black")
		results[0].Artifact.SHA256 = digest([]byte(results[0].Artifact.Content))
		entry := installAttempt(t, root, results[0])
		if _, err := Read(root, entry.ID); err == nil {
			t.Fatal("rehashed output tampering passed current replay")
		}
	})
	t.Run("missing original", func(t *testing.T) {
		root, results, _ := attemptFixture(t)
		entry, err := Publish(root, results[0])
		if err != nil {
			t.Fatal(err)
		}
		archive := strings.Split(results[0].Source.ArchiveID, "@")[1]
		if err := os.Rename(filepath.Join(root, "archives", archive+".zip"), filepath.Join(root, "archives", archive+".moved")); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, entry.ID); err == nil {
			t.Fatal("missing original accepted")
		}
	})
}

func TestStoreFilesystemRefusalsAndNoClobber(t *testing.T) {
	for _, obstruction := range []string{"store file", "store link", "entries link", "manifest link", "lock", "receipt corrupt", "receipt link"} {
		t.Run(obstruction, func(t *testing.T) {
			root, results, originals := attemptFixture(t)
			store := filepath.Join(root, "translations")
			if err := os.Mkdir(store, 0700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(t.TempDir(), "sentinel")
			if err := os.WriteFile(sentinel, []byte("do not change"), 0600); err != nil {
				t.Fatal(err)
			}
			originals[sentinel] = []byte("do not change")
			var err error
			switch obstruction {
			case "store file", "store link":
				if err := os.Remove(store); err != nil {
					t.Fatal(err)
				}
				if obstruction == "store file" {
					err = os.WriteFile(store, []byte("occupied"), 0600)
				} else {
					err = os.Symlink(filepath.Dir(sentinel), store)
				}
			case "entries link":
				err = os.Symlink(filepath.Dir(sentinel), filepath.Join(store, "entries"))
			case "manifest link":
				err = os.Symlink(sentinel, filepath.Join(store, "catalog.json"))
			case "lock":
				err = os.WriteFile(filepath.Join(store, ".publish.lock"), []byte("owned elsewhere"), 0600)
			case "receipt corrupt", "receipt link":
				body, _ := json.Marshal(results[0])
				entry := entryFor(results[0], body)
				if err := os.Mkdir(filepath.Join(store, "entries"), 0700); err != nil {
					t.Fatal(err)
				}
				name := filepath.Join(store, entryPath(entry.ID))
				if obstruction == "receipt corrupt" {
					err = os.WriteFile(name, []byte("corrupt"), 0600)
					originals[name] = []byte("corrupt")
				} else {
					err = os.Symlink(sentinel, name)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Publish(root, results[0]); err == nil {
				t.Fatal("obstruction was silently repaired/overwritten")
			}
			if obstruction == "lock" {
				b, _ := os.ReadFile(filepath.Join(store, ".publish.lock"))
				if string(b) != "owned elsewhere" {
					t.Fatal("removed another writer's lock")
				}
			}
			assertPreserved(t, originals)
		})
	}
}

func TestStoreRootAliasesAndConcurrentPublication(t *testing.T) {
	root, results, originals := attemptFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{alias, alias + "/", alias + "/."} {
		if _, err := Load(name); err == nil {
			t.Fatal("symlink root alias accepted")
		}
		if _, err := Publish(name, results[0]); err == nil {
			t.Fatal("symlink root alias writable")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Publish(root, results[i%2])
			if err != nil && !strings.Contains(err.Error(), "acquire publish lock") {
				t.Errorf("unexpected concurrent publish error: %v", err)
			}
		}()
	}
	wg.Wait()
	for _, result := range results {
		entry, err := Publish(root, result)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Read(root, entry.ID); err != nil {
			t.Fatal(err)
		}
	}
	c, err := Load(root)
	if err != nil || len(c.List()) != 2 {
		t.Fatalf("concurrent writes lost entries: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, "translations", ".write-*.tmp"))
	if len(leftovers) != 0 {
		t.Fatal("owned temporary files leaked")
	}
	assertPreserved(t, originals)
}
