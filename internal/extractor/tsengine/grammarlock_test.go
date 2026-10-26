package tsengine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// TestGrammarLock is LB-F12 (design §8.1, §8.2, I55): the committed lock
// describes every embedded blob exactly — sha256 and size of the
// uncompressed module, size of the committed .wasm.gz — every grammar's
// licence is on the ADR-048 allowlist, and a blob or a lock row without its
// partner fails.
func TestGrammarLock(t *testing.T) {
	lock, err := LoadGrammarLock()
	if err != nil {
		t.Fatalf("LoadGrammarLock: %v", err)
	}
	if len(lock.Grammars) == 0 {
		t.Fatal("the lock names no grammar")
	}
	sub, err := fs.Sub(grammarFS, "grammars")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyGrammarLock(lock, sub); err != nil {
		t.Fatalf("the committed lock does not verify:\n%v", err)
	}
	for _, name := range Grammars() {
		if _, ok := lock.Row(name); !ok {
			t.Errorf("registered grammar %q has no lock row", name)
		}
	}
}

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// fixtureLock is a one-grammar lock and a matching directory.
func fixtureLock(t *testing.T) (GrammarLock, fstest.MapFS) {
	t.Helper()
	raw := []byte("\x00asm\x01\x00\x00\x00 fixture module bytes")
	z := gz(t, raw)
	lock := GrammarLock{
		Core: LockCore{Repo: "tree-sitter/tree-sitter", Tag: "v0.26.11",
			Commit: strings.Repeat("a", 40), TarballSHA256: strings.Repeat("b", 64), Licence: "MIT"},
		Toolchain: LockToolchain{Zig: "0.16.0", WasmOpt: "129"},
		Grammars: []GrammarLockRow{{
			Blob: "demo", Repo: "example/tree-sitter-demo", Tag: "v1.0.0",
			Commit: strings.Repeat("c", 40), TarballSHA256: strings.Repeat("d", 64),
			Licence: "MIT", EntryPoints: []string{"tree_sitter_demo"}, Scanner: true, ABI: 14,
			BlobSHA256: sum(raw), BlobBytes: int64(len(raw)), GzBytes: int64(len(z)),
		}},
	}
	return lock, fstest.MapFS{"demo.wasm.gz": {Data: z}}
}

func TestVerifyGrammarLockRefuses(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*GrammarLock, fstest.MapFS)
		want string
	}{
		{"clean", func(*GrammarLock, fstest.MapFS) {}, ""},
		{"blob digest differs", func(l *GrammarLock, _ fstest.MapFS) {
			l.Grammars[0].BlobSHA256 = strings.Repeat("e", 64)
		}, "sha256"},
		{"blob size differs", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].BlobBytes++ }, "blob_bytes"},
		{"gz size differs", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].GzBytes++ }, "gz_bytes"},
		{"blob without a lock row", func(_ *GrammarLock, m fstest.MapFS) {
			m["stray.wasm.gz"] = &fstest.MapFile{Data: []byte("x")}
		}, "stray"},
		{"lock row without a blob", func(_ *GrammarLock, m fstest.MapFS) { delete(m, "demo.wasm.gz") }, "no blob"},
		{"GPL grammar", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].Licence = "GPL-3.0-only" }, "allowlist"},
		{"no licence", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].Licence = "" }, "allowlist"},
		{"core licence off the allowlist", func(l *GrammarLock, _ fstest.MapFS) { l.Core.Licence = "BUSL-1.1" }, "allowlist"},
		{"short commit", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].Commit = "abc" }, "commit"},
		{"bad tarball digest", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].TarballSHA256 = "zz" }, "tarball_sha256"},
		{"no entry point", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].EntryPoints = nil }, "entry_points"},
		{"duplicate row", func(l *GrammarLock, _ fstest.MapFS) {
			l.Grammars = append(l.Grammars, l.Grammars[0])
		}, "duplicate"},
		{"unsorted rows", func(l *GrammarLock, m fstest.MapFS) {
			r := l.Grammars[0]
			r.Blob = "abc"
			l.Grammars = append(l.Grammars, r)
			m["abc.wasm.gz"] = m["demo.wasm.gz"]
		}, "sorted"},
		{"abi out of range", func(l *GrammarLock, _ fstest.MapFS) { l.Grammars[0].ABI = 9 }, "abi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lock, dir := fixtureLock(t)
			tc.mut(&lock, dir)
			err := VerifyGrammarLock(lock, dir)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("clean fixture refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error naming %q, got %v", tc.want, err)
			}
		})
	}
}

func TestParseGrammarLockStrict(t *testing.T) {
	if _, err := ParseGrammarLock([]byte(`{"core":{},"toolchain":{},"grammars":[],"extra":1}`)); err == nil {
		t.Fatal("an unknown top-level field must be refused")
	}
	if _, err := ParseGrammarLock([]byte(`{"core":{},"toolchain":{},"grammars":[{"blob":"x","licence_url":"y"}]}`)); err == nil {
		t.Fatal("an unknown row field must be refused")
	}
	const patched = `{"core":{},"toolchain":{},"grammars":[{"blob":"x","patches":[{"path":"patches/fix.patch","sha256":"` +
		`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` + `"}]}]}`
	l, err := ParseGrammarLock([]byte(patched))
	if err != nil || len(l.Grammars[0].Patches) != 1 || l.Grammars[0].Patches[0].Path != "patches/fix.patch" {
		t.Fatalf("valid patch metadata not parsed: %v, %+v", err, l)
	}
	if _, err := ParseGrammarLock([]byte(strings.Replace(patched, `"sha256"`, `"sha25"`, 1))); err == nil {
		t.Fatal("an unknown patch field must be refused")
	}
}

func TestLicenceAllowlist(t *testing.T) {
	for _, ok := range []string{"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "Unlicense", "CC0-1.0", "Zlib", "0BSD", "MPL-2.0"} {
		if !LicenceAllowed(ok) {
			t.Errorf("%s should be allowed", ok)
		}
	}
	for _, no := range []string{"", "GPL-2.0-only", "GPL-3.0-or-later", "LGPL-2.1-only", "AGPL-3.0-only", "SSPL-1.0", "EUPL-1.2", "PolyForm-Noncommercial-1.0.0", "BUSL-1.1", "FSL-1.1-MIT", "NOASSERTION", "mit"} {
		if LicenceAllowed(no) {
			t.Errorf("%q must be refused", no)
		}
	}
}

func TestPatchLockMetadata(t *testing.T) {
	for _, tc := range []struct{ name, path, hash, licence, want string }{
		{"valid", "patches/fix.patch", strings.Repeat("a", 64), "MIT", ""},
		{"nested", "patches/sub/fix.patch", strings.Repeat("a", 64), "MIT", ""},
		{"traversal", "patches/../fix.patch", strings.Repeat("a", 64), "MIT", "patch"},
		{"outside", "fix.patch", strings.Repeat("a", 64), "MIT", "patch"},
		{"absolute", "/tmp/fix.patch", strings.Repeat("a", 64), "MIT", "patch"},
		{"backslash", `patches\fix.patch`, strings.Repeat("a", 64), "MIT", "patch"},
		{"extension", "patches/fix.diff", strings.Repeat("a", 64), "MIT", "patch"},
		{"space", "patches/my fix.patch", strings.Repeat("a", 64), "MIT", "patch"},
		{"empty patch stem", "patches/.patch", strings.Repeat("a", 64), "MIT", "patch"},
		{"uppercase hash", "patches/fix.patch", strings.Repeat("A", 64), "MIT", "sha256"},
		{"short hash", "patches/fix.patch", "abc", "MIT", "sha256"},
		{"MPL patched", "patches/fix.patch", strings.Repeat("a", 64), "MPL-2.0", "MPL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, dir := fixtureLock(t)
			l.Grammars[0].Licence = tc.licence
			l.Grammars[0].Patches = []LockPatch{{Path: tc.path, SHA256: tc.hash}}
			err := VerifyGrammarLock(l, dir)
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %s: %v", tc.want, err)
			}
		})
	}
	l, dir := fixtureLock(t)
	p := LockPatch{Path: "patches/fix.patch", SHA256: strings.Repeat("a", 64)}
	l.Grammars[0].Patches = []LockPatch{p, p}
	if err := VerifyGrammarLock(l, dir); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestInTreePatchHashes(t *testing.T) {
	l, err := LoadGrammarLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyGrammarPatchFiles(l, os.DirFS("grammars")); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyGrammarPatchFiles(t *testing.T) {
	l, _ := fixtureLock(t)
	b := []byte("patch bytes")
	p := LockPatch{Path: "patches/fix.patch", SHA256: sum(b)}
	l.Grammars[0].Patches = []LockPatch{p}
	good := fstest.MapFS{"patches/fix.patch": {Data: b}}
	if err := VerifyGrammarPatchFiles(l, good); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{"wrong hash", fstest.MapFS{"patches/fix.patch": {Data: []byte("other")}}, "sha256"},
		{"missing", fstest.MapFS{}, "missing"},
		{"symlink", fstest.MapFS{"patches/fix.patch": {Mode: fs.ModeSymlink}}, "symlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyGrammarPatchFiles(l, tc.files)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
}
