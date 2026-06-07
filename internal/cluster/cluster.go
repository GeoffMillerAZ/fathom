// Package cluster implements similarity clustering for clone detection.
package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

// Item is a unit to be clustered by content hash.
type Item struct {
	ID   string
	Hash string
}

// Cluster groups items that share the same Hash value.  Each group is a sorted
// slice of item IDs.  The returned outer slice is sorted by the lexicographically
// smallest ID in each group, making results fully deterministic.
// Items with an empty Hash each form their own singleton group.
func Cluster(items []Item) [][]string {
	if len(items) == 0 {
		return nil
	}

	// group by hash
	groups := make(map[string][]string)
	for _, it := range items {
		groups[it.Hash] = append(groups[it.Hash], it.ID)
	}

	// Build result: sort IDs within each group, then sort groups by their
	// minimum (first) element for determinism.
	result := make([][]string, 0, len(groups))
	for _, ids := range groups {
		sort.Strings(ids)
		result = append(result, ids)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i][0] < result[j][0]
	})
	return result
}

// NormalizedHash computes a structural-clone fingerprint of src by:
//  1. Collapsing all whitespace runs to a single space.
//  2. Replacing identifier tokens (sequences starting with a letter or
//     underscore, optionally followed by letters/digits/underscores) with the
//     canonical placeholder "X" so that variable/function renames are ignored.
//     Pure digit sequences (numeric literals) are preserved verbatim so that
//     structural changes (e.g. return 1 vs return 2) are detected.
//  3. SHA-256 hashing the resulting string.
//
// The returned value is a lowercase hex-encoded SHA-256 digest.
func NormalizedHash(src string) string {
	normalized := normalizeSource(src)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// tokenKind is the class of character being accumulated.
type tokenKind int

const (
	tkNone    tokenKind = iota
	tkIdent             // starts with letter or '_'
	tkNumeric           // pure digit sequence
)

// normalizeSource applies whitespace collapsing and identifier erasure.
// Numeric literals are preserved; identifiers are replaced with "X".
func normalizeSource(src string) string {
	var sb strings.Builder
	sb.Grow(len(src))

	kind := tkNone
	for _, r := range src {
		switch {
		case isIdentStart(r):
			// Start or continue an identifier.
			if kind != tkIdent {
				sb.WriteString("X")
				kind = tkIdent
			}
			// Absorb rune (do not write it).

		case unicode.IsDigit(r):
			if kind == tkIdent {
				// Digit continuing an identifier; absorb without writing.
			} else {
				// Standalone numeric literal: preserve the digit.
				if kind != tkNumeric {
					kind = tkNumeric
				}
				sb.WriteRune(r)
			}

		default:
			kind = tkNone
			if unicode.IsSpace(r) {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(r)
			}
		}
	}

	// Collapse multiple spaces into one.
	result := collapseSpaces(sb.String())
	return strings.TrimSpace(result)
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func collapseSpaces(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	prev := false
	for _, r := range s {
		if r == ' ' {
			if !prev {
				sb.WriteRune(r)
			}
			prev = true
		} else {
			sb.WriteRune(r)
			prev = false
		}
	}
	return sb.String()
}
