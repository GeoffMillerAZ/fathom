package scenariobench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func goldPatchFixture(t *testing.T) (Task, SourcePin, string, []byte) {
	t.Helper()
	patch := []byte("diff --git a/value.go b/value.go\n--- a/value.go\n+++ b/value.go\n@@ -1 +1 @@\n-old\n+new\n")
	task := Task{ID: "fixture-task", Gold: Gold{PatchSHA: fmt.Sprintf("%x", sha256.Sum256(patch))}}
	source := SourcePin{Name: "fixture-dataset", Revision: strings.Repeat("a", 40), License: "CC0-1.0"}
	return task, source, filepath.Join(t.TempDir(), "workspace", "gold-patches"), patch
}

func TestFetchGoldPatchVerifiedCache(t *testing.T) {
	task, source, cache, patch := goldPatchFixture(t)
	calls := 0
	fetch := func(_ context.Context, pin SourcePin, id string) ([]byte, error) {
		calls++
		if pin != source || id != task.ID {
			t.Fatalf("fetch lost source pin/task: %+v, %s", pin, id)
		}
		return patch, nil
	}
	for range 2 {
		got, err := FetchGoldPatch(t.Context(), task, source, cache, fetch)
		if err != nil || string(got) != string(patch) {
			t.Fatalf("patch = %q, %v", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
	files := goldCacheFiles(t, cache)
	if len(files) != 1 {
		t.Fatalf("cache contents = %v", files)
	}
	got, err := os.ReadFile(files[0])
	if err != nil || string(got) != string(patch) {
		t.Fatalf("cache bytes = %q, %v", got, err)
	}
	// A different revision must not reuse the first revision's cache entry.
	source.Revision = strings.Repeat("b", 40)
	if _, err := FetchGoldPatch(t.Context(), task, source, cache, fetch); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("cache ignored dataset revision")
	}
}

func goldCacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestFetchGoldPatchRejectsHashMismatch(t *testing.T) {
	task, source, cache, _ := goldPatchFixture(t)
	wrong := []byte("wrong patch")
	got, err := FetchGoldPatch(t.Context(), task, source, cache, func(context.Context, SourcePin, string) ([]byte, error) { return wrong, nil })
	actual := fmt.Sprintf("%x", sha256.Sum256(wrong))
	if err == nil || got != nil || !strings.Contains(err.Error(), task.Gold.PatchSHA) || !strings.Contains(err.Error(), actual) {
		t.Fatalf("mismatch did not name both hashes: %q, %v", got, err)
	}
	if files := goldCacheFiles(t, cache); len(files) != 0 {
		t.Fatalf("cached unverified patch: %v", files)
	}
}

func TestFetchGoldPatchRejectsMissingRevision(t *testing.T) {
	task, source, cache, patch := goldPatchFixture(t)
	for _, revision := range []string{"", "  "} {
		source.Revision = revision
		_, err := FetchGoldPatch(t.Context(), task, source, cache, func(context.Context, SourcePin, string) ([]byte, error) {
			t.Fatal("fetch called without a pinned revision")
			return patch, nil
		})
		if err == nil || !strings.Contains(err.Error(), "revision") {
			t.Fatalf("missing revision accepted: %v", err)
		}
	}
	if len(goldCacheFiles(t, cache)) != 0 {
		t.Fatal("invalid source wrote cache files")
	}
}

func TestFetchGoldPatchWritesOnlyInsideCache(t *testing.T) {
	task, source, cache, patch := goldPatchFixture(t)
	root := filepath.Dir(filepath.Dir(cache))
	outside := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	task.ID, source.Name, source.Revision = "../../task", "../../dataset", "../../revision"
	if _, err := FetchGoldPatch(t.Context(), task, source, cache, func(context.Context, SourcePin, string) ([]byte, error) { return patch, nil }); err != nil {
		t.Fatal(err)
	}
	var outsideFiles []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == cache {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			outsideFiles = append(outsideFiles, path)
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(outsideFiles, []string{outside}) {
		t.Fatalf("writes escaped cache: %v, %v", outsideFiles, err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "keep" {
		t.Fatal("outside file changed")
	}
}

func TestFetchGoldPatchRechecksCachedBytes(t *testing.T) {
	task, source, cache, patch := goldPatchFixture(t)
	if _, err := FetchGoldPatch(t.Context(), task, source, cache, func(context.Context, SourcePin, string) ([]byte, error) { return patch, nil }); err != nil {
		t.Fatal(err)
	}
	files := goldCacheFiles(t, cache)
	if err := os.WriteFile(files[0], []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := FetchGoldPatch(t.Context(), task, source, cache, func(context.Context, SourcePin, string) ([]byte, error) {
		t.Fatal("corrupt cache must be refused, not silently replaced")
		return patch, nil
	})
	if err == nil || !strings.Contains(err.Error(), task.Gold.PatchSHA) {
		t.Fatalf("corrupt cache accepted: %v", err)
	}
}

func TestFetchGoldPatchRefusesInvalidInputsAndFetcherErrors(t *testing.T) {
	for _, mode := range []string{"empty-hash", "invalid-hash", "empty-cache", "nil-fetcher", "fetch-error"} {
		t.Run(mode, func(t *testing.T) {
			task, source, cache, patch := goldPatchFixture(t)
			var fetch GoldPatchFetcher = func(context.Context, SourcePin, string) ([]byte, error) { return patch, nil }
			switch mode {
			case "empty-hash":
				task.Gold.PatchSHA = ""
			case "invalid-hash":
				task.Gold.PatchSHA = "../outside"
			case "empty-cache":
				cache = ""
			case "nil-fetcher":
				fetch = nil
			case "fetch-error":
				fetch = func(context.Context, SourcePin, string) ([]byte, error) {
					return nil, fmt.Errorf("fixture fetch failed")
				}
			}
			if _, err := FetchGoldPatch(t.Context(), task, source, cache, fetch); err == nil {
				t.Fatalf("accepted %s", mode)
			}
		})
	}
}
