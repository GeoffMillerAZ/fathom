// Package readmanifest defines the CUE-first contract for a read manifest:
// the accounting of every path a report considered and what happened to
// it, so a count derived from an incomplete read can never be rendered as a
// bare total.
//
// CUE is authoritative for shape; Go types in types.go mirror these
// definitions; any drift is a bug.
//
// Honest scope claim (doc'd): a manifest knows only the paths its caller
// recorded. It never enumerates a tree itself, so a path the caller never
// considered is invisible to it too.
package readmanifest

// #Kind classifies one attempted read. Only "unreadable" makes a manifest
// incomplete: absence and a stated skip are facts a report can carry.
#Kind: "read" | "absent" | "unreadable" | "skipped"

// #Outcome is what happened to one attempted read.
#Outcome: {
	kind: #Kind

	// detail is the operating system's own wording for an unreadable path
	// ("permission denied", "operation not permitted") or the caller's
	// category for a skipped one ("binary", "test-file"). Required for those
	// two kinds; meaningless for the others.
	detail?: string & !=""
	if kind == "unreadable" {
		detail: string & !=""
	}
	if kind == "skipped" {
		detail: string & !=""
	}

	// bytes is the length of a successful read. Never negative.
	bytes?: int & >=0
}

// #Entry is one recorded path.
#Entry: {
	// path is the path as the Set rendered it — root-relative and
	// slash-separated when the Set was given a root, otherwise verbatim.
	path:    string & !=""
	outcome: #Outcome
}

// #Manifest is the accounting: every considered path, sorted by path (a Go
// guarantee — CUE states the shape, not the order).
#Manifest: {
	entries?: [...#Entry]
}
