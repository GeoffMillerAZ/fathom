package explmap

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Source is the read side of freshness: the only way this package sees code.
// The CLI implements it over the repository's files, the index (symbol spans)
// and git (the text at filing); a test implements it in memory.
type Source interface {
	// Lines returns the current text of path's 1-based inclusive line range,
	// normalized as core.LinesText does; ok is false when the file or the
	// range no longer exists.
	Lines(path string, start, end int) (text string, ok bool)
	// Symbol resolves a symbol key to its current span; ok is false when the
	// key no longer resolves (or the index cannot be reached).
	Symbol(key string) (path string, start, end int, ok bool)
	// Filed returns the text of path's range as it was at commit, the base a
	// stale entry's diff size is measured against; ok is false when that text
	// cannot be recovered.
	Filed(path, commit string, start, end int) (text string, ok bool)
}

// State is an entry's freshness at read time.
type State string

const (
	// Fresh: the span still hashes to what was filed.
	Fresh State = "fresh"
	// ReResolved: the span drifted, but the symbol key found the same text at
	// a new span, which is where the entry now points.
	ReResolved State = "re-resolved"
	// Stale: the text changed or is gone. Served flagged, with its diff size,
	// never as if it were fresh.
	Stale State = "stale"
)

// Freshness is what re-resolving one entry found.
type Freshness struct {
	State State
	// Anchor is where the entry's code is now (`path#Lx-Ly`): the filed span
	// when fresh, the new span when re-resolved or when a symbol that changed
	// still resolves, the filed span otherwise.
	Anchor string
	// Added and Removed are the lines that differ between the text at filing
	// and the text now; meaningful only when DiffKnown.
	Added, Removed int
	// DiffKnown is false when the text at filing could not be recovered (no
	// commit, or the commit's text does not hash to the entry), so the size is
	// unknown and said so rather than guessed.
	DiffKnown bool
	// Note says why a stale entry is stale.
	Note string
}

// DiffSize is the number of lines changed since filing, or -1 when unknown.
func (f Freshness) DiffSize() int {
	if !f.DiffKnown {
		return -1
	}
	return f.Added + f.Removed
}

// resolveEntryFresh re-resolves one entry at read time (XM-5). It re-hashes the
// filed span with core.HashLines' contract (the comparison that invalidates a
// layer claim's citation, semantic-layers.md 4.3); a match is fresh. On a
// mismatch an entry that names a symbol is re-resolved by its key: the same
// text at a new span is re-resolved (the code moved and did not change).
// Everything else is stale, with the diff size between the text at filing and
// the text now, so a drifted entry is never served silently wrong.
func resolveEntryFresh(e Entry, src Source) Freshness {
	filedRef := e.SpanRef()
	cur, curOK := src.Lines(e.Path, e.Start, e.End)
	if curOK && HashText(cur) == e.Hash {
		return Freshness{State: Fresh, Anchor: filedRef, DiffKnown: true}
	}

	now, nowRef, nowOK := cur, filedRef, curOK
	var note string
	if e.Symbol != "" {
		if p, s, en, ok := src.Symbol(e.Symbol); ok {
			if txt, ok := src.Lines(p, s, en); ok {
				ref := core.CiteRef(p, s, en)
				if HashText(txt) == e.Hash {
					return Freshness{State: ReResolved, Anchor: ref, DiffKnown: true}
				}
				now, nowRef, nowOK = txt, ref, true
				note = fmt.Sprintf("symbol %s changed since it was filed", e.Symbol)
			}
		} else {
			note = fmt.Sprintf("symbol %s no longer resolves", e.Symbol)
			nowOK = false
		}
	}
	if note == "" {
		switch {
		case !curOK && !fileExists(src, e.Path):
			note = fmt.Sprintf("%s is gone", e.Path)
		case !curOK:
			note = fmt.Sprintf("%s no longer has lines %d-%d", e.Path, e.Start, e.End)
		default:
			note = fmt.Sprintf("%s changed since it was filed", filedRef)
		}
	}

	f := Freshness{State: Stale, Anchor: nowRef, Note: note}
	base, ok := src.Filed(e.Path, e.Commit, e.Start, e.End)
	if e.Commit == "" || !ok || HashText(base) != e.Hash {
		f.Note += "; diff size unknown (the text at filing is not recoverable)"
		return f
	}
	if !nowOK {
		f.Removed = lineCount(base)
	} else {
		f.Added, f.Removed = lineDiff(base, now)
	}
	f.DiffKnown = true
	return f
}

// fileExists asks the source for the file's first line.
func fileExists(src Source, path string) bool {
	_, ok := src.Lines(path, 1, 1)
	return ok
}

func lineCount(s string) int {
	return len(strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n"))
}

// lineDiffCap bounds the quadratic line diff; spans are small, and past the cap
// a positional count stands in (still a size, never a silence).
const lineDiffCap = 4_000_000

// lineDiff counts the lines added and removed between a and b by longest
// common subsequence over lines.
func lineDiff(a, b string) (added, removed int) {
	al := strings.Split(strings.ReplaceAll(a, "\r\n", "\n"), "\n")
	bl := strings.Split(strings.ReplaceAll(b, "\r\n", "\n"), "\n")
	if len(al)*len(bl) > lineDiffCap {
		n := len(al)
		if len(bl) < n {
			n = len(bl)
		}
		same := 0
		for i := 0; i < n; i++ {
			if al[i] == bl[i] {
				same++
			}
		}
		return len(bl) - same, len(al) - same
	}
	prev := make([]int, len(bl)+1)
	cur := make([]int, len(bl)+1)
	for i := 1; i <= len(al); i++ {
		for j := 1; j <= len(bl); j++ {
			switch {
			case al[i-1] == bl[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	lcs := prev[len(bl)]
	return len(bl) - lcs, len(al) - lcs
}
