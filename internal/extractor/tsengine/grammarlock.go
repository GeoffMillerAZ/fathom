package tsengine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
)

// GrammarLock is grammars/grammars.lock.json (LB-F12, design §8.1): the one
// record of which upstream sources every embedded grammar blob was built
// from, and the exact bytes that build produced. build.sh reads it; the
// engine refuses a blob it does not describe (I55); PROVENANCE.md's tables
// are rendered from it (RenderProvenanceTables).
type GrammarLock struct {
	Core      LockCore         `json:"core"`
	Toolchain LockToolchain    `json:"toolchain"`
	Grammars  []GrammarLockRow `json:"grammars"`
}

// LockCore pins tree-sitter's C runtime, linked into every blob.
type LockCore struct {
	Repo          string `json:"repo"`
	Tag           string `json:"tag"`
	Commit        string `json:"commit"`
	TarballSHA256 string `json:"tarball_sha256"`
	Licence       string `json:"licence"`
}

// LockToolchain names the build-time tools that produced the committed
// blobs. Byte-identical reproduction across other versions is not claimed.
type LockToolchain struct {
	Zig     string `json:"zig"`
	WasmOpt string `json:"wasm_opt"`
	// TreeSitterCLI pins the CLI a "generate": true grammar runs through
	// npx; empty until a grammar needs it.
	TreeSitterCLI string `json:"tree_sitter_cli,omitempty"`
}

// GrammarLockRow is one blob: the upstream grammar repository it is built
// from and the digests of what the build produced.
type GrammarLockRow struct {
	Blob          string `json:"blob"`
	Repo          string `json:"repo"`
	Tag           string `json:"tag"`
	Commit        string `json:"commit"`
	TarballSHA256 string `json:"tarball_sha256"`
	Licence       string `json:"licence"`
	// EntryPoints are the tree_sitter_<lang> functions the blob exports.
	EntryPoints []string `json:"entry_points"`
	// Subdirs are the grammar directories under the repository root, each
	// holding src/parser.c (and src/scanner.c when Scanner); empty means the
	// root itself. tree-sitter-typescript carries two ("typescript", "tsx").
	Subdirs  []string `json:"subdirs,omitempty"`
	Scanner  bool     `json:"scanner"`
	Generate bool     `json:"generate"`
	// PredicateExport adds ts_query_predicates_for_pattern to the blob's
	// exports (build.sh). The engine never requires it (predicates.go reads
	// predicates from the query source); the five original blobs predate it.
	PredicateExport bool `json:"predicate_export,omitempty"`
	ABI             int  `json:"abi"`
	// BlobSHA256 and BlobBytes describe the uncompressed module; GzBytes the
	// committed <blob>.wasm.gz.
	BlobSHA256 string `json:"blob_sha256"`
	BlobBytes  int64  `json:"blob_bytes"`
	GzBytes    int64  `json:"gz_bytes"`
}

// allowedLicences is the ADR-048 allowlist (design §8.2) as SPDX ids.
// MPL-2.0 is allowed only for files taken unmodified: a grammar's sources
// are compiled as published, never edited. Everything else — GPL, LGPL,
// AGPL, SSPL, EUPL, PolyForm, BUSL, FSL, Commons Clause, no licence, an
// unclear licence — is refused.
var allowedLicences = map[string]bool{
	"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"ISC": true, "Unlicense": true, "CC0-1.0": true, "Zlib": true, "0BSD": true,
	"MPL-2.0": true,
}

// LicenceAllowed reports whether spdx (an exact SPDX id) is on the grammar
// licence allowlist.
func LicenceAllowed(spdx string) bool { return allowedLicences[spdx] }

// Minimum and maximum grammar ABI the linked core accepts (tree-sitter
// v0.26: TREE_SITTER_MIN_COMPATIBLE_LANGUAGE_VERSION 13, LANGUAGE_VERSION 15).
const (
	minGrammarABI = 13
	maxGrammarABI = 15
)

// ParseGrammarLock decodes a lock strictly: an unknown field is an error,
// so a typo cannot silently drop a pin.
func ParseGrammarLock(b []byte) (GrammarLock, error) {
	var l GrammarLock
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return GrammarLock{}, fmt.Errorf("tsengine: grammars.lock.json: %w", err)
	}
	return l, nil
}

// LoadGrammarLock returns the lock embedded in this binary.
func LoadGrammarLock() (GrammarLock, error) {
	b, err := grammarFS.ReadFile("grammars/grammars.lock.json")
	if err != nil {
		return GrammarLock{}, fmt.Errorf("tsengine: read embedded lock: %w", err)
	}
	return ParseGrammarLock(b)
}

// Row returns the lock row for blob.
func (l GrammarLock) Row(blob string) (GrammarLockRow, bool) {
	for _, r := range l.Grammars {
		if r.Blob == blob {
			return r, true
		}
	}
	return GrammarLockRow{}, false
}

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// VerifyGrammarLock checks lock against the blobs in dir (the grammars
// directory): every row well formed and on the licence allowlist, rows
// sorted by blob and unique, every row's blob present with exactly the
// recorded sizes and digest, and no blob in dir without a row. It returns
// every problem found, sorted, joined into one error; nil means the lock
// and the blobs agree.
func VerifyGrammarLock(lock GrammarLock, dir fs.FS) error {
	var probs []string
	add := func(format string, a ...any) { probs = append(probs, fmt.Sprintf(format, a...)) }

	c := lock.Core
	if c.Repo == "" || c.Tag == "" {
		add("core: repo and tag are required")
	}
	if !isHex(c.Commit, 40) {
		add("core: commit %q is not 40 lowercase hex", c.Commit)
	}
	if !isHex(c.TarballSHA256, 64) {
		add("core: tarball_sha256 %q is not 64 lowercase hex", c.TarballSHA256)
	}
	if !LicenceAllowed(c.Licence) {
		add("core: licence %q is not on the allowlist (design §8.2)", c.Licence)
	}
	if lock.Toolchain.Zig == "" || lock.Toolchain.WasmOpt == "" {
		add("toolchain: zig and wasm_opt versions are required")
	}

	rows := map[string]GrammarLockRow{}
	for i, r := range lock.Grammars {
		id := r.Blob
		if id == "" {
			add("grammars[%d]: blob is required", i)
			continue
		}
		if _, dup := rows[id]; dup {
			add("%s: duplicate lock row", id)
			continue
		}
		rows[id] = r
		if i > 0 && lock.Grammars[i-1].Blob > id {
			add("%s: rows must be sorted by blob", id)
		}
		if r.Repo == "" || r.Tag == "" {
			add("%s: repo and tag are required", id)
		}
		if !isHex(r.Commit, 40) {
			add("%s: commit %q is not 40 lowercase hex", id, r.Commit)
		}
		if !isHex(r.TarballSHA256, 64) {
			add("%s: tarball_sha256 %q is not 64 lowercase hex", id, r.TarballSHA256)
		}
		if !isHex(r.BlobSHA256, 64) {
			add("%s: blob_sha256 %q is not 64 lowercase hex", id, r.BlobSHA256)
		}
		if !LicenceAllowed(r.Licence) {
			add("%s: licence %q is not on the allowlist (design §8.2)", id, r.Licence)
		}
		if len(r.EntryPoints) == 0 {
			add("%s: entry_points is empty", id)
		}
		for _, ep := range r.EntryPoints {
			if !strings.HasPrefix(ep, "tree_sitter_") {
				add("%s: entry point %q is not tree_sitter_<lang>", id, ep)
			}
		}
		if r.ABI < minGrammarABI || r.ABI > maxGrammarABI {
			add("%s: abi %d outside the core's %d..%d", id, r.ABI, minGrammarABI, maxGrammarABI)
		}
		if r.BlobBytes <= 0 {
			add("%s: blob_bytes must be positive", id)
		}
	}

	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		add("read grammars directory: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		var blob string
		switch {
		case strings.HasSuffix(name, ".wasm.gz"):
			blob = strings.TrimSuffix(name, ".wasm.gz")
		case strings.HasSuffix(name, ".wasm"):
			add("%s: %s is committed uncompressed; blobs are <blob>.wasm.gz (design §8.3)", strings.TrimSuffix(name, ".wasm"), name)
			continue
		default:
			continue
		}
		seen[blob] = true
		r, ok := rows[blob]
		if !ok {
			add("%s: blob %s has no lock row", blob, name)
			continue
		}
		f, err := dir.Open(name)
		if err != nil {
			add("%s: open %s: %v", blob, name, err)
			continue
		}
		raw, gzBytes, err := readBlob(f)
		_ = f.Close()
		if err != nil {
			add("%s: read %s: %v", blob, name, err)
			continue
		}
		if gzBytes != r.GzBytes {
			add("%s: %s is %d bytes, lock gz_bytes %d", blob, name, gzBytes, r.GzBytes)
		}
		if int64(len(raw)) != r.BlobBytes {
			add("%s: module is %d bytes, lock blob_bytes %d", blob, len(raw), r.BlobBytes)
		}
		if got := sha256Hex(raw); got != r.BlobSHA256 {
			add("%s: module sha256 %s, lock blob_sha256 %s", blob, got, r.BlobSHA256)
		}
	}
	for id := range rows {
		if !seen[id] {
			add("%s: lock row has no blob (%s.wasm.gz)", id, id)
		}
	}

	if len(probs) == 0 {
		return nil
	}
	sort.Strings(probs)
	errs := make([]error, len(probs))
	for i, p := range probs {
		errs[i] = errors.New(p)
	}
	return errors.Join(errs...)
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// maxBlobBytes bounds one decompressed module: a gzip bomb in a committed
// file must fail, not exhaust memory. The largest blob today is 2.4 MiB.
const maxBlobBytes = 64 << 20

// readBlob returns the uncompressed module and the compressed file's size.
func readBlob(r io.Reader) (raw []byte, gzBytes int64, err error) {
	z, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	raw, err = gunzip(z)
	return raw, int64(len(z)), err
}

func gunzip(z []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(z))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, maxBlobBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBlobBytes {
		return nil, fmt.Errorf("module over %d bytes", maxBlobBytes)
	}
	return raw, nil
}
