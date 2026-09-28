package tsengine

import (
	"fmt"
	"io/fs"
	"sync"
)

var (
	lockOnce sync.Once
	lockVal  GrammarLock
	lockErr  error

	opensMu sync.Mutex
	opens   = map[string]int{}
)

// embeddedLock is the binary's grammars.lock.json, parsed once.
func embeddedLock() (GrammarLock, error) {
	lockOnce.Do(func() { lockVal, lockErr = LoadGrammarLock() })
	return lockVal, lockErr
}

// openGrammarBlob returns the uncompressed module for the named blob (LB-F13,
// design §8.3): the embedded <name>.wasm.gz is decompressed here, on the
// grammar's first compile and never before, and refused unless its bytes are
// exactly what the lock describes (I55). The caller compiles it and drops the
// buffer; nothing keeps the decompressed bytes alive.
func openGrammarBlob(name string) ([]byte, error) {
	lock, err := embeddedLock()
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(grammarFS, "grammars")
	if err != nil {
		return nil, err
	}
	raw, err := openGrammarBlobFrom(lock, sub, name)
	if err != nil {
		return nil, err
	}
	opensMu.Lock()
	opens[name]++
	opensMu.Unlock()
	return raw, nil
}

// openGrammarBlobFrom reads dir/<name>.wasm.gz and checks it against lock.
func openGrammarBlobFrom(lock GrammarLock, dir fs.FS, name string) ([]byte, error) {
	row, ok := lock.Row(name)
	if !ok {
		return nil, fmt.Errorf("tsengine: unknown grammar %q (no grammars.lock.json row)", name)
	}
	z, err := fs.ReadFile(dir, name+".wasm.gz")
	if err != nil {
		return nil, fmt.Errorf("tsengine: read embedded blob %q: %w", name, err)
	}
	raw, err := gunzip(z)
	if err != nil {
		return nil, fmt.Errorf("tsengine: decompress blob %q: %w", name, err)
	}
	if int64(len(raw)) != row.BlobBytes {
		return nil, fmt.Errorf("tsengine: blob %q is %d bytes, lock says %d; refused (I55)", name, len(raw), row.BlobBytes)
	}
	if got := sha256Hex(raw); got != row.BlobSHA256 {
		return nil, fmt.Errorf("tsengine: blob %q sha256 %s does not match the lock's %s; refused (I55)", name, got, row.BlobSHA256)
	}
	return raw, nil
}

// blobOpenCounts is how many times each blob has been decompressed in this
// process (TestLazyGrammarLoad's instrument).
func blobOpenCounts() map[string]int {
	opensMu.Lock()
	defer opensMu.Unlock()
	out := make(map[string]int, len(opens))
	for k, v := range opens {
		out[k] = v
	}
	return out
}
