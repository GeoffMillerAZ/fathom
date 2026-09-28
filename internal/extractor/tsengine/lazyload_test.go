package tsengine

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// TestLazyGrammarLoad is LB-F13 (design §8.3, §9): a grammar blob is
// decompressed and compiled only when a parser for it is first asked for,
// once, and not at package init or for any other grammar.
func TestLazyGrammarLoad(t *testing.T) {
	dropCompiled()
	before := blobOpenCounts()

	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	after := blobOpenCounts()
	for _, name := range Grammars() {
		want := before[name]
		if name == "python" {
			want++
		}
		if after[name] != want {
			t.Errorf("%s opened %d time(s) after one python parser, want %d", name, after[name]-before[name], want-before[name])
		}
	}

	p2, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	p2.Close()
	if got := blobOpenCounts()["python"]; got != after["python"] {
		t.Fatalf("a second python parser decompressed the blob again (%d -> %d)", after["python"], got)
	}
}

// TestOpenGrammarBlobChecksLock (I55): a blob whose bytes the lock does not
// describe is never handed to the compiler.
func TestOpenGrammarBlobChecksLock(t *testing.T) {
	lock, dir := fixtureLock(t)
	if _, err := openGrammarBlobFrom(lock, dir, "demo"); err != nil {
		t.Fatalf("clean fixture: %v", err)
	}
	lock.Grammars[0].BlobSHA256 = strings.Repeat("0", 64)
	if _, err := openGrammarBlobFrom(lock, dir, "demo"); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("a digest mismatch must refuse the blob, got %v", err)
	}
	if _, err := openGrammarBlobFrom(lock, dir, "absent"); err == nil {
		t.Fatal("a blob without a lock row must be refused")
	}
}

// TestEmbeddedGrammarBudget (LB-F13, design §8.3): the compressed grammars
// embedded in the binary stay under 40 MiB. Past it, the lowest-demand
// dialects move to packs (LB-F14) — no language raises the budget.
func TestEmbeddedGrammarBudget(t *testing.T) {
	const capBytes = 40 << 20
	var total int64
	var n int
	err := fs.WalkDir(grammarFS, "grammars", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".wasm") {
			t.Errorf("%s is embedded uncompressed", path)
		}
		if strings.HasSuffix(path, ".wasm.gz") {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no .wasm.gz blob embedded")
	}
	t.Logf("embedded grammars: %d blobs, %d bytes compressed (cap %d)", n, total, capBytes)
	if total > capBytes {
		t.Fatalf("embedded .wasm.gz total %d bytes exceeds the 40 MiB cap", total)
	}
}

func TestVerifyGrammarLockRefusesUncompressed(t *testing.T) {
	lock, dir := fixtureLock(t)
	dir["demo.wasm"] = &fstest.MapFile{Data: []byte("x")}
	if err := VerifyGrammarLock(lock, dir); err == nil || !strings.Contains(err.Error(), "demo.wasm") {
		t.Fatalf("an uncompressed blob must be refused, got %v", err)
	}
}
