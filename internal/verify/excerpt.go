package verify

// excerpt.go — EVX-1, EVX-5 (docs/plans/portable-receipts.md; vision 43
// section 6.2): the bytes an anchor cites and their digest, in one place.
// The excerpt is the bytes from the first byte of start_line through the
// last byte of end_line, that line's newline included when it has one, with
// no normalization (a CR stays a CR). Its digest is Review Kit's
// SpanRef.ContentHash formula: the lowercase hex SHA-256 of the raw bytes,
// no prefix. The digest is SARIF's partialFingerprints["fathomExcerpt/v1"],
// so a finding keeps its fingerprint when lines move above it. The shared
// vectors in testdata/excerpt pin the bytes, a file with no final newline
// among them. internal/receipt's AnchorAt reads the digest through
// ExcerptSHA256; nothing else hashes an excerpt.

import (
	"crypto/sha256"
	"encoding/hex"
)

// ExcerptFingerprintKey is the partialFingerprints key a SARIF result's
// excerpt digest is written under. The "/v1" names the formula above: a
// change to the bytes it covers is a new key, never a silent re-meaning.
const ExcerptFingerprintKey = "fathomExcerpt/v1"

// excerptBytes returns the lines [start, end] (1-based, inclusive) of
// content, and whether content holds them. start = end = 0 is the whole file
// (SpanRef: zero lines mean the whole file).
func excerptBytes(content []byte, start, end int) ([]byte, bool) {
	if start == 0 && end == 0 {
		return content, true
	}
	if start < 1 || end < start {
		return nil, false
	}
	line, from := 1, -1
	if start == 1 {
		from = 0
	}
	for i, b := range content {
		if b != '\n' {
			continue
		}
		if line == end {
			return content[from : i+1], true
		}
		line++
		if line == start {
			from = i + 1
		}
	}
	// The last line has no newline: it ends at the end of the file.
	if line == end && from >= 0 && from < len(content) {
		return content[from:], true
	}
	return nil, false
}

// excerptFingerprint is the excerpt's digest (the formula above), and
// whether content holds the lines; "" when it does not.
func excerptFingerprint(content []byte, start, end int) (string, bool) {
	b, ok := excerptBytes(content, start, end)
	if !ok {
		return "", false
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true
}

// ExcerptSHA256 is excerptFingerprint for internal/receipt's AnchorAt: the
// one formula, read from outside the package.
func ExcerptSHA256(content []byte, start, end int) (string, bool) {
	return excerptFingerprint(content, start, end)
}
