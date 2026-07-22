package connector

import "bytes"

// binarySniffLimit is the number of leading bytes inspected for a NUL byte —
// the classic text-tool heuristic (precedent: agentic-kit's
// cmd/constitution-lint looksBinary). 8000 bytes is enough to catch binary
// headers (ELF/Mach-O/PE magic, compressed archives, images) cheaply, without
// reading whole multi-megabyte files just to classify them.
const binarySniffLimit = 8000

// IsBinary reports whether data appears to be a binary file: a NUL byte
// anywhere in the first binarySniffLimit bytes (ADR-021 §1 / LT-2). It is a
// cheap, dependency-free heuristic, not a guarantee: UTF-8 source never
// contains NUL, but UTF-16/UTF-32-encoded text files DO (every other byte
// for ASCII-range UTF-16) and will classify as binary — an accepted
// trade-off, since fathom's extractors only parse UTF-8 anyway. Conversely,
// a binary whose first 8000 bytes happen to be NUL-free passes as text.
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
