package extractor

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// TestEmbeddedDataDigestCoversManifestsQueriesAndLock pins the data part of the extractor epoch
// (ADR-075 D4): EmbeddedDataDigest covers every embedded dialect manifest, doc spec and query and
// the grammar lock, so one changed byte in any of them changes it, and the order the files are
// listed in does not.
func TestEmbeddedDataDigestCoversManifestsQueriesAndLock(t *testing.T) {
	files := embeddedDataFiles()
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	if !slices.IsSorted(paths) {
		t.Fatalf("embedded data files are not sorted by path: %v", paths)
	}
	// Every embedded manifest, doc spec and query is covered, and the lock as the binary embeds it.
	var manifest, query, lock = -1, -1, -1
	for i, f := range files {
		switch {
		case strings.HasSuffix(f.Path, "/dialect.json") && manifest < 0:
			manifest = i
		case strings.HasSuffix(f.Path, ".scm") && query < 0:
			query = i
		case f.Path == grammarLockPath:
			lock = i
		}
	}
	if manifest < 0 || query < 0 || lock < 0 {
		t.Fatalf("digest inputs miss a class: manifest %d, query %d, lock %d of %d files", manifest, query, lock, len(files))
	}
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if !slices.Contains(paths, "dialects/"+d.ID+"/dialect.json") {
			t.Errorf("dialect %s's manifest is not a digest input", d.ID)
		}
	}
	wantLock, err := tsengine.GrammarLockBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files[lock].Bytes, wantLock) {
		t.Fatal("the lock the digest covers is not the embedded grammars.lock.json")
	}

	base := EmbeddedDataDigest()
	if base != digestDataFiles(files) {
		t.Fatal("EmbeddedDataDigest is not the digest of the embedded data files")
	}
	if len(base) != 64 || strings.Trim(base, "0123456789abcdef") != "" {
		t.Fatalf("digest %q is not 64 hex", base)
	}
	if again := EmbeddedDataDigest(); again != base {
		t.Fatalf("digest moved between calls: %s then %s", base, again)
	}

	// One changed byte of a manifest, a query or the lock changes the digest.
	for name, i := range map[string]int{"manifest": manifest, "query": query, "lock": lock} {
		changed := slices.Clone(files)
		b := slices.Clone(changed[i].Bytes)
		b[len(b)/2] ^= 0x01
		changed[i] = dataFile{Path: changed[i].Path, Bytes: b}
		if digestDataFiles(changed) == base {
			t.Errorf("one byte of the %s %s changed and the digest did not", name, files[i].Path)
		}
	}
	// A renamed file changes it: the path is part of what is hashed.
	renamed := slices.Clone(files)
	renamed[query] = dataFile{Path: renamed[query].Path + ".x", Bytes: renamed[query].Bytes}
	if digestDataFiles(renamed) == base {
		t.Error("a renamed query left the digest unchanged")
	}
	// File order does not.
	reversed := slices.Clone(files)
	slices.Reverse(reversed)
	if digestDataFiles(reversed) != base {
		t.Error("the digest depends on the order the files are listed in")
	}
	// Bytes moved across a file boundary change it: each file's length is hashed with it.
	a := []dataFile{{Path: "a", Bytes: []byte("xy")}, {Path: "b", Bytes: []byte("z")}}
	b := []dataFile{{Path: "a", Bytes: []byte("x")}, {Path: "b", Bytes: []byte("yz")}}
	if digestDataFiles(a) == digestDataFiles(b) {
		t.Error("bytes moved from one file to the next left the digest unchanged")
	}
}

// BenchmarkEmbeddedDataDigestCold is the digest's one-time cost per process: reading every
// embedded input and hashing it, uncached (O1369 records the reading).
func BenchmarkEmbeddedDataDigestCold(b *testing.B) {
	for b.Loop() {
		_ = digestDataFiles(embeddedDataFiles())
	}
}
