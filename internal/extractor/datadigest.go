package extractor

// datadigest.go — EmbeddedDataDigest, the data part of the extractor epoch (ADR-075 D4,
// docs/design/append-points.md section 6).

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// grammarLockPath is the lock's path among the digest's inputs, relative to this package.
const grammarLockPath = "tsengine/grammars/grammars.lock.json"

// dataFile is one embedded input of the data digest: its path and its bytes.
type dataFile struct {
	Path  string
	Bytes []byte
}

// dataDigestOnce computes the digest the first time it is asked for: the inputs are embedded,
// so one process never sees two values.
var dataDigestOnce = sync.OnceValue(func() string { return digestDataFiles(embeddedDataFiles()) })

// EmbeddedDataDigest is sha256, hex, over the sorted paths and bytes of every embedded dialect
// manifest, doc spec and query (dialects/*/*.json, dialects/*/*.scm) and the grammar lock. The
// lock pins every grammar blob's sha256, so no blob is hashed. The extractor epoch carries it, so
// a data change shipped without a bump still re-extracts every file (the O9 class). Computed
// once per process.
func EmbeddedDataDigest() string { return dataDigestOnce() }

// embeddedDataFiles is every file the digest covers, sorted by path: what dialectFS embeds and
// the lock. The inputs are embedded in the binary, so a read error is a broken build.
func embeddedDataFiles() []dataFile {
	var files []dataFile
	err := fs.WalkDir(dialectFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := dialectFS.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, dataFile{Path: p, Bytes: b})
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("extractor: embedded dialect data unreadable: %v", err))
	}
	lock, err := tsengine.GrammarLockBytes()
	if err != nil {
		panic(fmt.Sprintf("extractor: embedded grammar lock unreadable: %v", err))
	}
	files = append(files, dataFile{Path: grammarLockPath, Bytes: lock})
	slices.SortFunc(files, func(a, b dataFile) int { return strings.Compare(a.Path, b.Path) })
	return files
}

// digestDataFiles hashes files in path order whatever order they are given in: each file's path,
// a NUL, its length and its bytes, so no two different sets of files hash alike.
func digestDataFiles(files []dataFile) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b dataFile) int { return strings.Compare(a.Path, b.Path) })
	h := sha256.New()
	var n [8]byte
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		binary.BigEndian.PutUint64(n[:], uint64(len(f.Bytes)))
		h.Write(n[:])
		h.Write(f.Bytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}
