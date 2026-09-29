package wiring

// link.go — a symbolic link fathom owns (ADR-059, AG-7). The shipped skills
// are the case: `.claude/skills` holds the only real copy, and
// `.agents/skills` is a relative directory link to it for the harnesses
// that read `.agents/skills` and not `.claude/skills` (Codex, Gemini CLI,
// the generic AGENTS.md tier). One link cannot drift from its target the
// way two rendered copies did.
//
// Law 15 applies to a path the same way it applies to a region:
//
//   - absent: fathom creates the link, and the receipt records it so
//     `fleet undo` removes it;
//   - a link with the same target: already current, skip;
//   - a link with any other target, a regular file, or a real directory
//     holding anything fathom cannot prove it wrote: BLOCKED, named, never
//     replaced.
//
// The one migration: a real directory whose every file is a fathom-owned
// file listed in Link.Owned (marker present, and not edited since fathom
// wrote it) is fathom's own earlier rendering. Those files are removed —
// each one's bytes in the receipt — and the directory is replaced by the
// link, in one apply that `fleet undo` reverses. A single foreign entry
// blocks the whole item: fathom never merges a directory.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// KindLink is the Item kind for an owned symbolic link.
const KindLink = "link"

// linkTarget is the Item target for a link.
const linkTarget = "fathom:link"

// Link is one symbolic link fathom owns.
type Link struct {
	// Path is where the link lives, repo-relative and slash-separated.
	Path string
	// Target is the link's content exactly as written — relative to the
	// link's own directory ("../.claude/skills"), so the repository can be
	// moved or cloned without the link dangling.
	Target string
	// Owned are the files fathom rendered under Path before the link
	// existed (Path-relative files named repo-relative, as OwnedFile.File).
	// A real directory at Path is replaced by the link only when every file
	// in it is one of these and none was edited since fathom wrote it.
	Owned []OwnedFile
}

// linkDirScan is what a real directory at Link.Path holds, split into
// what fathom may remove and what blocks.
type linkDirScan struct {
	removable []string // repo-relative files fathom owns and may remove
	foreign   []string // repo-relative entries fathom cannot prove it wrote
	edited    []string // fathom-marked files edited by hand since fathom wrote them
}

func planLink(root string, l Link, ledger Ledger) Item {
	it := Item{File: l.Path, Target: linkTarget, Kind: KindLink, After: l.Target}
	path := filepath.Join(root, filepath.FromSlash(l.Path))

	fi, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		it.Verdict = VerdictApply
		it.Reason = l.Path + " does not exist — fathom will create it as a link to " + l.Target
		return it
	case err != nil:
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "cannot be read: "+err.Error()
		return it
	}
	it.Present = true

	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		cur, rerr := os.Readlink(path)
		if rerr != nil {
			it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "the link cannot be read: "+rerr.Error()
			return it
		}
		it.Before = cur
		if filepath.ToSlash(cur) == l.Target {
			it.Verdict = VerdictSkip
			it.Reason = "already current (a link to " + l.Target + ")"
			return it
		}
		it.Verdict, it.Cause = VerdictBlocked, CauseUnproven
		it.Reason = l.Path + " is a link to " + cur + ", not " + l.Target + " — a link fathom did not write; fathom will not replace it"
		return it
	case !fi.IsDir():
		it.Verdict, it.Cause = VerdictBlocked, CauseUnproven
		it.Reason = l.Path + " is a regular file — fathom will not replace it with a link"
		return it
	}

	scan, serr := scanLinkDir(root, l, ledger)
	if serr != nil {
		it.Verdict, it.Cause, it.Reason = VerdictBlocked, CauseUnreadable, "the directory cannot be read: "+serr.Error()
		return it
	}
	it.Before = fmt.Sprintf("a directory holding %d fathom-owned file(s)", len(scan.removable))
	switch {
	case len(scan.edited) > 0:
		it.Verdict, it.Cause = VerdictBlocked, CauseDrift
		it.Reason = l.Path + " holds fathom-owned files edited by hand since fathom wrote them (" + strings.Join(scan.edited, ", ") +
			") — fathom will not remove them; delete the marker line to keep yours, or remove the file to take the link"
	case len(scan.foreign) > 0:
		it.Verdict, it.Cause = VerdictBlocked, CauseUnproven
		it.Reason = l.Path + " is a directory holding entries fathom did not write (" + strings.Join(scan.foreign, ", ") +
			") — fathom will not merge a directory; move them into " + l.Target + " or out of the way, then re-run"
	default:
		it.Verdict = VerdictApply
		it.Reason = l.Path + " is fathom's own earlier rendering (" + fmt.Sprint(len(scan.removable)) +
			" owned file(s), none edited) — fathom will remove it and write a link to " + l.Target + " in its place"
	}
	return it
}

// scanLinkDir classifies every entry under the real directory at l.Path.
func scanLinkDir(root string, l Link, ledger Ledger) (linkDirScan, error) {
	owned := map[string]OwnedFile{}
	for _, f := range l.Owned {
		owned[f.File] = f
	}
	var scan linkDirScan
	base := filepath.Join(root, filepath.FromSlash(l.Path))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		f, ok := owned[rel]
		if !ok || d.Type()&os.ModeSymlink != 0 {
			scan.foreign = append(scan.foreign, rel)
			return nil
		}
		// Blocked is either unmarked (a file fathom did not write) or edited
		// under the marker: planOwnedFile's cause says which.
		switch v := planOwnedFile(root, f, ledger); {
		case v.Verdict == VerdictBlocked && v.Cause == CauseDrift:
			scan.edited = append(scan.edited, rel)
		case v.Verdict == VerdictBlocked:
			scan.foreign = append(scan.foreign, rel)
		default:
			scan.removable = append(scan.removable, rel)
		}
		return nil
	})
	sort.Strings(scan.removable)
	sort.Strings(scan.foreign)
	sort.Strings(scan.edited)
	return scan, err
}

// applyLink executes one planned link: removes fathom's earlier rendering
// when the plan found one, then writes the link. Every removed file and the
// link itself land in the receipt, so `fleet undo` restores the directory.
//
// ledgerChanged reports whether a removed file's ledger entry was dropped,
// so the caller saves the ledger only when it moved.
func applyLink(root string, l Link, it Item, ledger *Ledger, receipt *Receipt) (ledgerChanged bool, err error) {
	path := filepath.Join(root, filepath.FromSlash(l.Path))
	fi, err := os.Lstat(path)
	switch {
	case err == nil && fi.IsDir():
		scan, serr := scanLinkDir(root, l, *ledger)
		if serr != nil {
			return false, serr
		}
		if len(scan.foreign) > 0 || len(scan.edited) > 0 {
			return false, fmt.Errorf("%s changed since the plan (now holds %s) — nothing removed", l.Path, strings.Join(append(scan.foreign, scan.edited...), ", "))
		}
		for _, rel := range scan.removable {
			fp := filepath.Join(root, filepath.FromSlash(rel))
			before, rerr := os.ReadFile(fp)
			if rerr != nil {
				return ledgerChanged, rerr
			}
			if rerr := os.Remove(fp); rerr != nil {
				return ledgerChanged, rerr
			}
			if _, owned := ownedKey(*ledger, rel, ownedFileID); owned {
				ledger.Forget(rel, ownedFileID)
				ledgerChanged = true
			}
			receipt.Files = append(receipt.Files, ReceiptFile{
				Path:         rel,
				Existed:      true,
				Before:       string(before),
				BeforeDigest: digest(string(before)),
				Touched:      []TouchedRegion{{Target: ownedFileID, Kind: KindFile, Present: true, After: ""}},
			})
		}
		if rerr := removeEmptyTree(path); rerr != nil {
			return ledgerChanged, fmt.Errorf("%s still holds entries after removing fathom's files: %w", l.Path, rerr)
		}
	case err == nil:
		return false, fmt.Errorf("%s changed since the plan — nothing written", l.Path)
	case !os.IsNotExist(err):
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ledgerChanged, err
	}
	if err := os.Symlink(filepath.FromSlash(l.Target), path); err != nil {
		return ledgerChanged, err
	}
	receipt.Files = append(receipt.Files, ReceiptFile{
		Path:         l.Path,
		Existed:      false,
		BeforeDigest: digest(""),
		AfterExists:  true,
		AfterDigest:  linkDigest(l.Target),
		Link:         l.Target,
		Touched:      []TouchedRegion{{Target: it.Target, Kind: KindLink, Present: it.Present, Before: it.Before, After: l.Target}},
	})
	return ledgerChanged, nil
}

// linkDigest is the digest a receipt records for a link: over its target,
// prefixed so it can never collide with a regular file's content digest.
func linkDigest(target string) string { return digest("link:" + target) }

// removeEmptyTree removes dir and every directory under it, bottom-up, and
// fails — having removed only empty directories — when any file remains.
func removeEmptyTree(dir string) error {
	var dirs []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() {
			return fmt.Errorf("%s is not a directory", p)
		}
		dirs = append(dirs, p)
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}

// LinkState reports what is at a repo-relative path right now: whether it
// is a symbolic link, and its target when it is. A reader that must not
// write through fathom's link (the skills installers, drift) asks this
// before treating the path as a directory of files.
func LinkState(root, rel string) (isLink bool, target string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false, ""
	}
	t, err := os.Readlink(p)
	if err != nil {
		return true, ""
	}
	return true, filepath.ToSlash(t)
}
