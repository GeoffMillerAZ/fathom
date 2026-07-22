package connector

import "bytes"

// binarySniffLimit is the number of leading bytes inspected for a NUL byte —
// the classic text-tool heuristic (precedent: agentic-kit's
// cmd/constitution-lint looksBinary). 8000 bytes is enough to catch binary
// headers (ELF/Mach-O/PE magic, compressed archives, images) cheaply, without
// reading whole multi-megabyte files just to classify them.
const binarySniffLimit = 8000

// IsBinary reports whether data appears to be a binary file: a NUL byte
// anywhere in the first binarySniffLimit bytes (ADR-021 §1 / LT-2). Text
// source in every language fathom extracts is NUL-free, so this is a cheap,
// dependency-free classifier with no false positives on real source.
//
// Callers use IsBinary to exclude binary content from extraction and from
// line-counting (cloc): binaries are reported in a files/bytes bucket,
// never as counted lines — see core.LangBinary.
func IsBinary(data []byte) bool {
	limit := binarySniffLimit
	if len(data) < limit {
		limit = len(data)
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}
