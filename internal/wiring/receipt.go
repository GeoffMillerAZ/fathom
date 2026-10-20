package wiring

// receipt.go — every write produces a receipt sufficient to undo it, and
// undo is a RESTORE FROM RECEIPT, not an inverse operation (design §5).
//
// The distinction matters. An inverse operation has to reason about what a
// change meant — which template version was replaced, which key was added,
// whether the human's edit came before or after. A restore does not: the
// receipt carries the full prior content of every file it touched, so undo
// writes bytes it already has.
//
// Receipts are append-only, local, and never leave the machine (law 13).
// They are pruned by COUNT, not by age, so an undo is always available for
// the last N applies regardless of how long ago they happened.
//
// O63 item 5: the receipts log is fathom's own runtime state, not
// something a subject repository should find in its working tree (a
// wired bench arm's diff carried it as one of fourteen to sixteen
// fathom-owned files an adopter would otherwise see). It lives under
// receiptsPath(root) — internal/statedir.Resolve(root), outside the repo
// by default — never a fixed root-relative join.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

const (
	receiptsFileName  = "receipts.jsonl"
	defaultMaxReceipt = 50
)

// receiptsPath is where root's receipts log lives: statedir.Resolve(root)
// — outside the repo tree by default, or root/workspace when root's own
// .fathom/config.toml opts into the in-tree convention — plus the leaf
// file name.
func receiptsPath(root string) string {
	return filepath.Join(statedir.Resolve(root), receiptsFileName)
}

// ReceiptsPath is where root's receipts log lives — for a reader that keys
// a cache on the log's size and mtime (every apply and undo appends to it).
func ReceiptsPath(root string) string { return receiptsPath(root) }

// ErrUndoRefused is returned when any file in a receipt has changed since
// the receipt was written. Undo is all-or-nothing: a partial restore across
// a configuration is worse than no restore at all.
var ErrUndoRefused = errors.New("undo refused: a file has changed since the receipt was written")

// TouchedRegion is the before/after of one region or key inside a file, so
// a human reading a receipt can see WHAT changed and not merely that the
// file did.
type TouchedRegion struct {
	Target  string `json:"target"`
	Kind    string `json:"kind"`
	Present bool   `json:"present"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after"`
}

// ReceiptFile is one file's full prior content plus the digests that make
// the restore refusable.
type ReceiptFile struct {
	Path         string          `json:"path"`
	Existed      bool            `json:"existed"`
	Before       string          `json:"before,omitempty"`
	BeforeDigest string          `json:"before_digest"`
	AfterExists  bool            `json:"after_exists"`
	AfterDigest  string          `json:"after_digest,omitempty"`
	Touched      []TouchedRegion `json:"touched,omitempty"`
	// Link is the target when the path AFTER this write is a symbolic link
	// fathom wrote (link.go); BeforeLink is the target when the path was a
	// link BEFORE it (an undo that removed one). Both empty for a file.
	Link       string `json:"link,omitempty"`
	BeforeLink string `json:"before_link,omitempty"`
}

// Receipt is one apply (or one undo — see UndoOf), append-only.
type Receipt struct {
	ID         string        `json:"id"`
	Time       string        `json:"time"`
	Fathom     string        `json:"fathom_version"`
	Root       string        `json:"root"`
	PlanDigest string        `json:"plan_digest,omitempty"`
	UndoOf     string        `json:"undo_of,omitempty"`
	Files      []ReceiptFile `json:"files"`
}

// Summary is the one-line spelling `fathom fleet receipts` prints.
func (r Receipt) Summary() string {
	kind := "apply"
	if r.UndoOf != "" {
		kind = "undo of " + r.UndoOf
	}
	names := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		names = append(names, f.Path)
	}
	return fmt.Sprintf("%s  %s  %s  %s", r.ID, r.Time, kind, strings.Join(names, ", "))
}

// AppendReceipt appends r (stamping its id and the producing binary) and
// prunes the log to the newest max entries. It returns the receipt as
// written, id included.
func AppendReceipt(root string, r Receipt, max int) (Receipt, error) {
	if max <= 0 {
		max = defaultMaxReceipt
	}
	if r.Fathom == "" {
		r.Fathom = buildinfo.Version()
	}
	if r.ID == "" {
		r.ID = receiptID(r)
	}
	line, err := json.Marshal(r)
	if err != nil {
		return Receipt{}, err
	}

	p := receiptsPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Receipt{}, err
	}
	lines, err := receiptLines(p)
	if err != nil {
		return Receipt{}, err
	}
	lines = append(lines, string(line))
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

// receiptID is derived, not random, so the same operation at the same
// injected instant yields the same id — determinism is a contract even for
// identifiers.
func receiptID(r Receipt) string {
	seed := r.Time + "|" + r.Root + "|" + r.PlanDigest + "|" + r.UndoOf
	for _, f := range r.Files {
		seed += "|" + f.Path + "|" + f.BeforeDigest
	}
	return strings.TrimPrefix(digest(seed), "sha256:")[:12]
}

func receiptLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

// LoadReceipts returns root's receipts NEWEST FIRST — the order a human
// asking "what did fathom just do here?" needs.
func LoadReceipts(root string) ([]Receipt, error) {
	path := receiptsPath(root)
	lines, err := receiptLines(path)
	if err != nil {
		return nil, err
	}
	out := make([]Receipt, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		var r Receipt
		if err := json.Unmarshal([]byte(lines[i]), &r); err != nil {
			return nil, fmt.Errorf("parsing %s line %d: %w", path, i+1, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// FindReceipt resolves an id — exactly, or by an unambiguous prefix. An
// ambiguous prefix is an error naming the candidates rather than a guess.
func FindReceipt(root, id string) (Receipt, error) {
	all, err := LoadReceipts(root)
	if err != nil {
		return Receipt{}, err
	}
	if len(all) == 0 {
		return Receipt{}, fmt.Errorf("no receipts in %s — fathom has written nothing here", receiptsPath(root))
	}
	if id == "" {
		return all[0], nil
	}
	var matches []Receipt
	for _, r := range all {
		if r.ID == id {
			return r, nil
		}
		if strings.HasPrefix(r.ID, id) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Receipt{}, fmt.Errorf("no receipt with id %q", id)
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return Receipt{}, fmt.Errorf("receipt id %q is ambiguous: %s", id, strings.Join(ids, ", "))
	}
}

// Refusal is one file undo would not touch, with the reason in words a
// human can act on.
type Refusal struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// UndoResult is what an undo did — or, when Refused is non-empty, exactly
// what it declined to do and why. Refused and Restored are never both
// populated: undo is all-or-nothing.
type UndoResult struct {
	Receipt  Receipt   `json:"receipt"`
	Restored []string  `json:"restored,omitempty"`
	Removed  []string  `json:"removed,omitempty"`
	Refused  []Refusal `json:"refused,omitempty"`
	// Undo is itself a write, so it is itself a receipt.
	Receipted *Receipt `json:"receipted,omitempty"`
}

// Undo restores every file in a receipt to its exact prior bytes.
//
// It refuses — writing NOTHING — if any file has changed since the receipt
// was written. That is law 15 again, from the other direction: the receipt
// describes a world, and restoring into a different world would destroy
// whatever changed it.
func Undo(root, id string, opt ApplyOptions) (UndoResult, error) {
	r, err := FindReceipt(root, id)
	if err != nil {
		return UndoResult{}, err
	}
	out := UndoResult{Receipt: r}

	type restore struct {
		file    ReceiptFile
		path    string
		current string
		exists  bool
		link    string // the path is a link to this target right now
	}
	plan := make([]restore, 0, len(r.Files))

	// Links this receipt wrote (link.go). A file the same apply removed from
	// under one reads THROUGH the link today — the link's target has a file
	// by that name — but the link is removed first, so for the check the
	// path is absent.
	var wroteLinks []string
	for _, f := range r.Files {
		if f.Link != "" {
			wroteLinks = append(wroteLinks, f.Path)
		}
	}
	underWrittenLink := func(rel string) bool {
		for _, lp := range wroteLinks {
			if strings.HasPrefix(rel, lp+"/") {
				if isLink, _ := LinkState(root, lp); isLink {
					return true
				}
			}
		}
		return false
	}

	for _, f := range r.Files {
		path := receiptTarget(root, f.Path)
		isLink, target := LinkState(root, f.Path)
		var current string
		var exists bool
		switch {
		case isLink:
			exists = true
		case underWrittenLink(f.Path):
			// absent once the link is gone (see above)
		case f.BeforeLink != "" && isDir(path):
			// Undoing an undo that restored a directory in the link's place:
			// the directory must hold nothing but files this receipt removes.
			if stray := strayUnder(root, f.Path, r.Files); len(stray) > 0 {
				out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "holds entries the receipt does not describe (" + strings.Join(stray, ", ") + ") — undo would destroy them"})
				continue
			}
		default:
			var rerr error
			current, exists, rerr = readFileIfPresent(path)
			if rerr != nil {
				out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "cannot be read: " + rerr.Error()})
				continue
			}
		}
		switch {
		case f.Link != "" && (!isLink || target != f.Link):
			out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "no longer the link fathom wrote (to " + f.Link + ") — undo would remove something else"})
		case f.Link == "" && isLink:
			out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "is a link now (to " + target + "); the receipt describes a file"})
		case f.Link != "":
			plan = append(plan, restore{file: f, path: path, exists: true, link: target})
		case f.AfterExists && !exists:
			out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "the file no longer exists; the receipt describes a file fathom wrote"})
		case !f.AfterExists && exists:
			out.Refused = append(out.Refused, Refusal{Path: f.Path, Reason: "the file exists again; the receipt describes a file that was removed"})
		case f.AfterExists && digest(current) != f.AfterDigest:
			out.Refused = append(out.Refused, Refusal{
				Path:   f.Path,
				Reason: "changed since the receipt was written (" + short(f.AfterDigest) + " then, " + short(digest(current)) + " now) — undo would destroy that change",
			})
		default:
			plan = append(plan, restore{file: f, path: path, current: current, exists: exists})
		}
	}
	if len(out.Refused) > 0 {
		return out, ErrUndoRefused
	}

	// The undo is itself a mutation, so it gets its own receipt — carrying
	// the pre-undo content, which makes the undo undoable in turn.
	back := Receipt{
		Time:   opt.now().UTC().Format(time.RFC3339),
		Root:   root,
		UndoOf: r.ID,
	}
	for _, p := range plan {
		bf := ReceiptFile{
			Path:         p.file.Path,
			Existed:      p.exists,
			Before:       p.current,
			BeforeDigest: digest(p.current),
			AfterExists:  p.file.Existed || p.file.BeforeLink != "",
			AfterDigest:  ternary(p.file.Existed, digest(p.file.Before), ""),
		}
		if p.link != "" {
			bf.BeforeLink, bf.BeforeDigest = p.link, linkDigest(p.link)
		}
		if p.file.BeforeLink != "" {
			bf.Link, bf.AfterDigest = p.file.BeforeLink, linkDigest(p.file.BeforeLink)
		}
		back.Files = append(back.Files, bf)
	}
	written, err := AppendReceipt(root, back, opt.MaxReceipts)
	if err != nil {
		return out, err
	}
	out.Receipted = &written

	// Order is load-bearing: a link fathom wrote goes first, so restoring
	// the files it replaced writes real files and never through the link;
	// a link the undo restores goes last, once the files that stood in its
	// place are gone.
	for _, p := range plan {
		if p.file.Link == "" {
			continue
		}
		if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
			return out, fmt.Errorf("removing the link %s: %w", p.file.Path, err)
		}
		out.Removed = append(out.Removed, p.file.Path)
	}
	for _, p := range plan {
		switch {
		case p.file.Link != "" || p.file.BeforeLink != "":
			continue
		case !p.file.Existed:
			if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
				return out, fmt.Errorf("removing %s: %w", p.file.Path, err)
			}
			out.Removed = append(out.Removed, p.file.Path)
		default:
			if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
				return out, err
			}
			if err := os.WriteFile(p.path, []byte(p.file.Before), 0o644); err != nil {
				return out, fmt.Errorf("restoring %s: %w", p.file.Path, err)
			}
			out.Restored = append(out.Restored, p.file.Path)
		}
	}
	for _, p := range plan {
		if p.file.BeforeLink == "" {
			continue
		}
		if fi, err := os.Lstat(p.path); err == nil && fi.IsDir() {
			if err := removeEmptyTree(p.path); err != nil {
				return out, fmt.Errorf("restoring the link %s: %w", p.file.Path, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			return out, err
		}
		if err := os.Symlink(filepath.FromSlash(p.file.BeforeLink), p.path); err != nil {
			return out, fmt.Errorf("restoring the link %s: %w", p.file.Path, err)
		}
		out.Restored = append(out.Restored, p.file.Path)
	}
	return out, nil
}

func isDir(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir()
}

// strayUnder lists the files under the directory rel that files does not
// remove (an entry with Existed false is one the undo removes).
func strayUnder(root, rel string, files []ReceiptFile) []string {
	removes := map[string]bool{}
	for _, f := range files {
		if !f.Existed && f.Link == "" {
			removes[f.Path] = true
		}
	}
	var stray []string
	_ = filepath.WalkDir(filepath.Join(root, filepath.FromSlash(rel)), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		r, rerr := filepath.Rel(root, p)
		if rerr == nil && !removes[filepath.ToSlash(r)] {
			stray = append(stray, filepath.ToSlash(r))
		}
		return nil
	})
	return stray
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// receiptTarget is the on-disk path a receipt entry names. The ledger's
// entry keeps its .fathom/owned.json spelling as an identifier but lives in
// the state directory.
func receiptTarget(root, rel string) string {
	if rel == ledgerRelPath {
		return ledgerPath(root)
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}
