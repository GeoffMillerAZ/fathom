package wiring

// apply.go — the second phase (design §4): execute exactly the items the
// plan marked `apply`, and nothing else.
//
// Two properties are load-bearing and both are tested:
//
//  1. A single BLOCKED item does not block the others in the same run. A
//     drifted CLAUDE.md must not cost a repository its missing pre-push
//     gate; the drift is reported and the rest proceeds.
//  2. Content outside fathom's regions and keys survives byte-for-byte,
//     line endings included. Every write here is a splice into an existing
//     string, never a re-render of a whole file fathom does not own.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ApplyOptions carries the caller's clock (determinism is a contract: this
// package never reads the wall clock itself unless the caller declines to
// supply one) and the preview→apply freshness token.
type ApplyOptions struct {
	// Now supplies the receipt timestamp. Defaults to time.Now for CLI
	// callers; tests inject a fixed instant.
	Now func() time.Time
	// ExpectDigest, when set, is the digest of a plan a human already looked
	// at. Apply recomputes the plan and refuses if it no longer matches —
	// the repository changed between preview and apply, so the diff the
	// human approved is not the diff that would be written.
	ExpectDigest string
	// MaxReceipts bounds the receipt log by COUNT, not age, so an undo is
	// always available for the last N applies however long ago they were.
	MaxReceipts int
	// MigrateHooks completes the one-time hook adoption pass, even when
	// no keys need adopting. Set only after reading and planning hooks cleanly.
	MigrateHooks bool
}

func (o ApplyOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Result is what an apply did, and — just as importantly — what it declined
// to do and why.
type Result struct {
	Plan    Plan     `json:"plan"`
	Receipt *Receipt `json:"receipt,omitempty"` // nil when nothing was written
	Applied []Item   `json:"applied"`
	Skipped []Item   `json:"skipped"`
	Blocked []Item   `json:"blocked"`
}

// ApplyWiring computes the plan and then writes exactly its `apply` items.
//
// It returns an error only when the CALL is wrong (bad spec, stale plan
// digest, an unwritable file). A drifted repository is not an error: it is
// a Result whose Blocked list says what fathom refused to touch.
func ApplyWiring(root string, spec Spec, opt ApplyOptions) (res Result, err error) {
	plan, err := PlanWiring(root, spec)
	if err != nil {
		return Result{}, err
	}
	res = Result{Plan: plan, Blocked: plan.Blocked(), Skipped: plan.Skipped()}

	if opt.ExpectDigest != "" && opt.ExpectDigest != plan.Digest {
		return res, fmt.Errorf("the repository changed since the plan was previewed (previewed %s, now %s) — re-run the preview and look at the new diff",
			short(opt.ExpectDigest), short(plan.Digest))
	}

	todo := plan.Applicable()
	if len(todo) == 0 && !opt.MigrateHooks {
		return res, nil
	}

	regions := map[string]Region{}
	for _, r := range spec.Regions {
		regions[r.File] = r
	}
	keys := map[string]Key{}
	for _, k := range spec.Keys {
		keys[k.File+"\x00"+k.ID()] = k
	}
	files := map[string]OwnedFile{}
	for _, f := range spec.Files {
		files[f.File] = f
	}
	links := map[string]Link{}
	for _, l := range spec.Links {
		links[l.Path] = l
	}

	byFile := map[string][]Item{}
	var fileOrder []string
	for _, it := range todo {
		if _, seen := byFile[it.File]; !seen {
			fileOrder = append(fileOrder, it.File)
		}
		byFile[it.File] = append(byFile[it.File], it)
	}
	sort.Strings(fileOrder)

	ledger, lerr := LoadLedger(root)
	if lerr != nil {
		if opt.MigrateHooks {
			return res, lerr // An unreadable ledger cannot complete migration.
		}
		// Already named in plan.LedgerNote. Fathom owns nothing it cannot
		// prove it owns, so it starts a fresh ledger rather than guessing.
		ledger = NewLedger()
	}

	receipt := Receipt{
		Time:       opt.now().UTC().Format(time.RFC3339),
		Root:       root,
		PlanDigest: plan.Digest,
	}

	keysRecorded := false

	// A TORN apply must still be undoable. Every write below can fail
	// mid-loop — a read-only lefthook.yml is enough — and returning at
	// that point used to discard the receipt entirely, leaving files
	// already rewritten with no way to reverse them and `fathom fleet
	// receipts` stating, falsely, that fathom had written nothing here.
	//
	// That is this phase's own covenant failing on an ordinary error: the
	// undo path is the entire justification for letting fathom edit a
	// repository at all (law 15). So the receipt is flushed on EVERY exit
	// once any file has been touched, success or not.
	defer func() {
		if opt.MigrateHooks && err == nil { // a torn apply keeps the migration pending
			ledger.Version = ledgerVersion
			keysRecorded = true
		}
		if keysRecorded {
			if lerr := flushLedger(root, ledger, &receipt); lerr != nil && err == nil {
				err = lerr
			}
		}
		if len(receipt.Files) == 0 {
			return
		}
		sort.Slice(receipt.Files, func(i, j int) bool { return receipt.Files[i].Path < receipt.Files[j].Path })
		written, aerr := AppendReceipt(root, receipt, opt.MaxReceipts)
		if aerr != nil {
			// Losing the receipt is worse than the original failure, so it
			// wins the error slot when nothing else claimed it.
			if err == nil {
				err = fmt.Errorf("files were written but the receipt could not be saved (undo is unavailable): %w", aerr)
			}
			return
		}
		res.Receipt = &written
	}()
	for _, file := range fileOrder {
		// A link is a path, not a file's bytes: it writes its own receipt
		// entries (the files it replaced, then the link) and is applied
		// after every file, so nothing is ever written through it.
		if _, isLink := links[file]; isLink {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(file))
		before, existed, rerr := readFileIfPresent(path)
		if rerr != nil {
			return res, fmt.Errorf("reading %s: %w", file, rerr)
		}
		content := before
		rf := ReceiptFile{
			Path:         file,
			Existed:      existed,
			Before:       before,
			BeforeDigest: digest(before),
		}

		// Ledger records wait for the write (FA-23): each is the digest of
		// bytes this apply put on disk, and a write that fails records
		// nothing.
		type pendingRecord struct {
			file string
			key  OwnedKey
		}
		var pending []pendingRecord
		for _, it := range byFile[file] {
			switch it.Kind {
			case KindRegion:
				r, ok := regions[file]
				if !ok {
					return res, fmt.Errorf("internal: no region spec for %s", file)
				}
				next, perr := placeRegion(content, existed, r)
				if perr != nil {
					return res, fmt.Errorf("writing the fathom region in %s: %w", file, perr)
				}
				content = next
			case KindKey:
				k, ok := keys[file+"\x00"+it.Target]
				if !ok {
					return res, fmt.Errorf("internal: no key spec for %s in %s", it.Target, file)
				}
				next, perr := k.place(content)
				if perr != nil {
					return res, fmt.Errorf("writing %s in %s: %w", it.Target, file, perr)
				}
				content = next
				want, derr := k.desired()
				if derr != nil {
					return res, derr
				}
				pending = append(pending, pendingRecord{file, OwnedKey{ID: k.ID(), Digest: digest(want), Wrote: receipt.Time}})
			case KindFile:
				// A whole owned file (ownedfile.go): the content IS the
				// body, and the digest of what fathom wrote is recorded so
				// a later hand edit under the marker is detectable — under
				// both ids that watch these bytes, the planner's and
				// DriftSkills', so neither keeps the bytes it replaced.
				f, ok := files[file]
				if !ok {
					return res, fmt.Errorf("internal: no owned-file spec for %s", file)
				}
				content = f.Body
				pending = append(pending,
					pendingRecord{file, OwnedKey{ID: ownedFileID, Digest: digest(f.Body), Wrote: receipt.Time}},
					pendingRecord{file, OwnedKey{ID: skillKeyID, Digest: digest(f.Body), Wrote: receipt.Time}})
			}
			rf.Touched = append(rf.Touched, TouchedRegion{
				Target:  it.Target,
				Kind:    it.Kind,
				Present: it.Present,
				Before:  it.Before,
				After:   it.After,
			})
			res.Applied = append(res.Applied, it)
		}

		record := func() {
			for _, p := range pending {
				ledger.Record(p.file, p.key)
				keysRecorded = true
			}
		}
		if content == before {
			record() // the bytes on disk are already the ones recorded
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return res, err
		}
		record()
		rf.AfterExists = true
		rf.AfterDigest = digest(content)
		receipt.Files = append(receipt.Files, rf)
	}

	for _, file := range fileOrder {
		l, isLink := links[file]
		if !isLink {
			continue
		}
		for _, it := range byFile[file] {
			changed, lerr := applyLink(root, l, it, &ledger, &receipt)
			keysRecorded = keysRecorded || changed
			if lerr != nil {
				return res, fmt.Errorf("writing the link %s: %w", file, lerr)
			}
			res.Applied = append(res.Applied, it)
		}
	}

	return res, nil
}

// flushLedger saves the ownership ledger and records it in the receipt.
//
// The ledger is part of the same write, so it belongs in the same receipt:
// an undo that restored the files but left the ledger claiming keys that
// are no longer there would leave the repository reading as DRIFTED — a
// false stop manufactured by the undo itself.
func flushLedger(root string, ledger Ledger, receipt *Receipt) error {
	lp := ledgerPath(root)
	lBefore, lExisted, err := readFileIfPresent(lp)
	if err != nil {
		return err
	}
	if err := SaveLedger(root, ledger); err != nil {
		return err
	}
	lAfter, _, err := readFileIfPresent(lp)
	if err != nil {
		return err
	}
	if lAfter == lBefore {
		return nil
	}
	receipt.Files = append(receipt.Files, ReceiptFile{
		Path:         ledgerRelPath,
		Existed:      lExisted,
		Before:       lBefore,
		BeforeDigest: digest(lBefore),
		AfterExists:  true,
		AfterDigest:  digest(lAfter),
	})
	return nil
}

// placeRegion splices fathom's region into content. The three shapes are
// exactly applyStanza's three (cmd/fathom/init.go), with marker format v3's
// digest added: create, replace between the markers, or append a fresh
// block leaving every existing byte where it is.
func placeRegion(content string, existed bool, r Region) (string, error) {
	ending := lineEnding(content)
	if r.Remove {
		return removeRegion(content, existed, r)
	}
	if !existed {
		return renderRegion(r.Syntax, r.Version, r.Body, ending) + ending, nil
	}
	reg, err := parseOwnedRegion(content, r.Syntax)
	if err != nil {
		return "", err
	}
	if reg.Present {
		// The region's OWN line ending wins, so a splice into a CRLF file
		// stays CRLF and the bytes on either side are untouched.
		rendered := renderRegion(r.Syntax, r.Version, r.Body, reg.Ending)
		return content[:reg.Start] + rendered + content[reg.End:], nil
	}
	sep := ending + ending
	switch {
	case content == "":
		// An existing but EMPTY file is still a file someone made; fathom
		// appends into it rather than leading with blank lines.
		sep = ""
	case strings.HasSuffix(content, ending+ending):
		sep = ""
	case strings.HasSuffix(content, ending):
		sep = ending
	}
	return content + sep + renderRegion(r.Syntax, r.Version, r.Body, ending) + ending, nil
}

// removeRegion is placeRegion for Region.Remove: the marker pair, the body
// and the region's own line terminator go; a blank separator line that
// only set the region apart goes with it; every other byte stays.
func removeRegion(content string, existed bool, r Region) (string, error) {
	if !existed {
		return content, nil
	}
	reg, err := parseOwnedRegion(content, r.Syntax)
	if err != nil {
		return "", err
	}
	if !reg.Present {
		return content, nil
	}
	before, after := content[:reg.Start], content[reg.End:]
	after = strings.TrimPrefix(after, reg.Ending)
	double := reg.Ending + reg.Ending
	switch {
	case after == "" && strings.HasSuffix(before, double):
		// appended at the end with a blank line before it
		before = strings.TrimSuffix(before, reg.Ending)
	case strings.HasSuffix(before, double) && strings.HasPrefix(after, reg.Ending):
		// set apart by blank lines on both sides: keep one
		after = strings.TrimPrefix(after, reg.Ending)
	case before == "" && strings.HasPrefix(after, reg.Ending):
		after = strings.TrimPrefix(after, reg.Ending)
	}
	return before + after, nil
}
