package wiring

// ownedfile.go — tier 2½: a WHOLE file fathom owns outright, identified by
// a marker line (RP-4, docs/plans/reach-parity.md). The shipped skills are
// the case: `fathom init --skills` renders `.claude/skills/<name>/SKILL.md`
// and `.agents/skills/<name>/SKILL.md` carrying `<!-- fathom:owned … -->`,
// and a re-render is a rewrite of the whole file, not a splice.
//
// Ownership is claimed two ways, and the plan says which it relied on:
//
//   - The MARKER is the standing claim (the line itself says "delete this
//     line to adopt the file as your own"). A marker-bearing file that
//     differs from the template is fathom's to rewrite.
//   - The LEDGER (.fathom/owned.json, id "file") records the digest of the
//     bytes fathom last wrote. A marker-bearing file whose bytes no longer
//     hash to that digest was edited by hand UNDER the marker: law 15 stops
//     there — BLOCKED, drift, named — because a rewrite would destroy the
//     edit and a merge would guess.
//
// A file at the path WITHOUT the marker is a hand-authored file that
// happens to share a skill's name. fathom cannot prove it wrote it and
// will not overwrite it: BLOCKED, unproven, named by path.

import (
	"path/filepath"
	"strings"
)

// KindFile is the Item kind for a whole owned file.
const KindFile = "file"

// ownedFileID is the ledger id under which a whole file's digest is
// recorded — one per file, so the file path is the whole address.
const ownedFileID = "file"

// OwnedFile is one whole file fathom owns by marker.
type OwnedFile struct {
	File   string // repo-relative, slash-separated
	Marker string // the substring whose presence is fathom's ownership claim
	Body   string // the full content fathom wants
	// Known holds the digests of renders earlier fathom releases shipped
	// for this file: a marker-bearing file equal to one is fathom's own
	// older render, rewritten however the per-checkout ledger lags (FA-24).
	Known map[string]bool
}

// planOwnedFile computes the verdict for one owned file without touching
// anything. See the file comment for the four outcomes.
func planOwnedFile(root string, f OwnedFile, ledger Ledger) Item {
	it := Item{File: f.File, Target: ownedFileID, Kind: KindFile, After: f.Body}
	path := filepath.Join(root, filepath.FromSlash(f.File))

	current, existed, err := readFileIfPresent(path)
	if err != nil {
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "cannot be read: "+err.Error()
		return it
	}
	if !existed {
		it.Verdict = VerdictApply
		it.Reason = f.File + " does not exist — fathom will create it under its marker"
		return it
	}
	it.Present, it.Before = true, current

	if f.Marker == "" || !strings.Contains(current, f.Marker) {
		it.Verdict, it.Cause = VerdictBlocked, CauseUnproven
		it.Reason = f.File + " exists without fathom's marker — a file by this name that fathom did not write; fathom will not overwrite it (rename it, or remove it to take fathom's)"
		return it
	}

	owned, isOwned := ownedKey(ledger, f.File, ownedFileID)
	switch {
	case current == f.Body:
		// Before drift: a file equal to the shipped source is fathom's own even
		// when the per-checkout ledger recorded an older release.
		it.Verdict = VerdictSkip
		it.Reason = "already current"
	case f.Known[digest(current)]:
		// Likewise an earlier release's render: a writer that recorded no
		// baseline (fathom init before FA-23) left the ledger behind it.
		it.Verdict = VerdictApply
		it.Reason = "an earlier fathom release's render of this file — rewriting it under its marker and recording the digest"
	case isOwned && owned.Digest != digest(current):
		it.Verdict, it.Cause = VerdictBlocked, CauseDrift
		it.Reason = f.File + " was edited by hand under fathom's marker since fathom wrote it on " + orUnrecorded(owned.Wrote) +
			" (recorded " + short(owned.Digest) + ", now " + short(digest(current)) + ") — fathom will not merge; delete the marker line to keep yours, or remove the file to take fathom's"
	default:
		it.Verdict = VerdictApply
		it.Reason = "fathom's own file is behind the shipped source — rewriting it under its marker and recording the digest"
	}
	return it
}
