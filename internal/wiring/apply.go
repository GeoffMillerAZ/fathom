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
func ApplyWiring(root string, spec Spec, opt ApplyOptions) (Result, error) {
	plan, err := PlanWiring(root, spec)
	if err != nil {
		return Result{}, err
	}
	res := Result{Plan: plan, Blocked: plan.Blocked(), Skipped: plan.Skipped()}

	if opt.ExpectDigest != "" && opt.ExpectDigest != plan.Digest {
		return res, fmt.Errorf("the repository changed since the plan was previewed (previewed %s, now %s) — re-run the preview and look at the new diff",
			short(opt.ExpectDigest), short(plan.Digest))
	}

	todo := plan.Applicable()
	if len(todo) == 0 {
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

	byFile := map[string][]Item{}
	var files []string
	for _, it := range todo {
		if _, seen := byFile[it.File]; !seen {
			files = append(files, it.File)
		}
		byFile[it.File] = append(byFile[it.File], it)
	}
	sort.Strings(files)

	ledger, lerr := LoadLedger(root)
	if lerr != nil {
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
	for _, file := range files {
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
				ledger.Record(file, OwnedKey{ID: k.ID(), Digest: digest(want), Wrote: receipt.Time})
				keysRecorded = true
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

		if content == before {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return res, err
		}
		rf.AfterExists = true
		rf.AfterDigest = digest(content)
		receipt.Files = append(receipt.Files, rf)
	}

	if len(receipt.Files) == 0 {
		return res, nil
	}

	// The ownership ledger is part of the same write, so it belongs in the
	// same receipt: an undo that restored the files but left the ledger
	// claiming keys that are no longer there would leave the repository
	// reading as DRIFTED — a false stop manufactured by the undo itself.
	if keysRecorded {
		lp := filepath.Join(root, filepath.FromSlash(ledgerRelPath))
		lBefore, lExisted, lrerr := readFileIfPresent(lp)
		if lrerr != nil {
			return res, lrerr
		}
		if err := SaveLedger(root, ledger); err != nil {
			return res, err
		}
		lAfter, _, lrerr := readFileIfPresent(lp)
		if lrerr != nil {
			return res, lrerr
		}
		if lAfter != lBefore {
			receipt.Files = append(receipt.Files, ReceiptFile{
				Path:         ledgerRelPath,
				Existed:      lExisted,
				Before:       lBefore,
				BeforeDigest: digest(lBefore),
				AfterExists:  true,
				AfterDigest:  digest(lAfter),
			})
		}
	}
	sort.Slice(receipt.Files, func(i, j int) bool { return receipt.Files[i].Path < receipt.Files[j].Path })

	written, err := AppendReceipt(root, receipt, opt.MaxReceipts)
	if err != nil {
		return res, err
	}
	res.Receipt = &written
	return res, nil
}

// placeRegion splices fathom's region into content. The three shapes are
// exactly applyStanza's three (cmd/fathom/init.go), with marker format v3's
// digest added: create, replace between the markers, or append a fresh
// block leaving every existing byte where it is.
func placeRegion(content string, existed bool, r Region) (string, error) {
	ending := lineEnding(content)
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
