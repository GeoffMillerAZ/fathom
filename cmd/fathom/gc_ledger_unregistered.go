package main

// gc_ledger_unregistered.go — PE-29 (docs/design/progress-and-economics.md
// §11 rule 3; PE-X5; ADR-047 decision 13): `fathom gc`'s class
// ledger-unregistered. Scratch-clone runs of `fathom verify` and `fathom
// intent audit` appended rows keyed by scratch paths to the user-level
// ledgers; the readers filter them on read (PE-24, PE-25), and this class
// lets the owner move what is already there out of the live files.
//
// The dry run counts rows and bytes per unregistered root in the verify and
// intent ledgers. --apply MOVES those rows under <FATHOM_HOME>/archive in
// the owner's 2026-09-29 layout — <ledger-stem>-unregistered-<date>.jsonl,
// plain JSONL, rows byte-identical and appended when the day's file exists;
// SHA256SUMS updated for the files gc wrote; one README table row per
// archive file — and never deletes one. Restore is `cat <archive> >>
// <ledger>` (readers sort by timestamp).
//
// The class is planned only when gcOptions.LedgerRoots is set, and only
// cmdGC sets it: the daemon's daily pass and doctor's dry run pass no roots,
// so neither plans nor applies it. A row naming no root, and a line that is
// not JSON, stays live: unattributed is not unregistered.
//
// The live ledger's writers (fathom verify, the stop hook, the claim-moment
// edit hook, intent audit) append with O_APPEND and take no lock, so the
// rewrite is a temp file in the same directory and a rename, carrying every
// byte appended after the read before the rename and sweeping the old inode
// once after it. Before the rename every original row is checked to land in
// exactly one of archive or live, by count and by bytes; any failure undoes
// the archive append and the metadata and leaves the live ledger untouched.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

const gcClassLedgerUnregistered = "ledger-unregistered"

// gcLedgerRoots is the class's input: which roots the fleet registered.
// Unread non-empty means the registry could not be read, and why; nothing is
// then planned and no count is guessed.
type gcLedgerRoots struct {
	Unread     string
	Registered func(repoRoot string) bool
}

// gcLedgerRootsOf is PE-23's answer as the class's input. An unread
// registry stays unread with its reason; one that lists no root is treated
// as unread too — an empty registry (a fresh or failing daemon) would
// otherwise make every row "unregistered" and move them all.
func gcLedgerRootsOf(r ledgerroots.Roots) *gcLedgerRoots {
	switch {
	case !r.Read:
		return &gcLedgerRoots{Unread: r.Note}
	case len(r.List()) == 0:
		return &gcLedgerRoots{Unread: "the daemon's service registry lists no service root; nothing is moved while it is empty"}
	}
	return &gcLedgerRoots{Registered: r.Registered}
}

// gcUnregisteredLedgerStems are the ledgers the class reads (§11).
var gcUnregisteredLedgerStems = []string{"verify-ledger", "intent-ledger"}

// gcLedgerArchiveReadmeHeader is the owner's 2026-09-29 table header.
const gcLedgerArchiveReadmeHeader = "| archive | source ledger | rule |"

// gcLedgerCarryTries bounds the re-stat loop that carries a growing tail.
const gcLedgerCarryTries = 8

// ledgerRootTally is one unregistered root's rows in one ledger.
type ledgerRootTally struct {
	Root  string
	Rows  int
	Bytes int64
}

// ledgerUnregisteredLedger is one ledger's part of the plan.
type ledgerUnregisteredLedger struct {
	Path    string // the live ledger
	Stem    string
	Archive string // <archive dir>/<stem>-unregistered-<date>.jsonl
	Tallies []ledgerRootTally
	Done    bool
	Err     error
}

// ledgerUnregisteredPlan is the class's plan over one FATHOM_HOME.
type ledgerUnregisteredPlan struct {
	Home       string // the resolved FATHOM_HOME
	ArchiveDir string
	Unread     string
	Ledgers    []ledgerUnregisteredLedger
	registered func(string) bool
}

// ledgerRowRoot is a row's repo_root; ok is false for a line that is not a
// JSON object or names no root.
func ledgerRowRoot(line []byte) (string, bool) {
	var r struct {
		RepoRoot string `json:"repo_root"`
	}
	if err := json.Unmarshal(bytes.TrimRight(line, "\n"), &r); err != nil || r.RepoRoot == "" {
		return "", false
	}
	return r.RepoRoot, true
}

// splitLedgerRows partitions data's complete rows (through its last newline,
// at cut) into moved (an unregistered root) and kept, each in file order
// and byte-identical; bytes past cut are a writer's partial row.
func splitLedgerRows(data []byte, registered func(string) bool) (moved, kept [][]byte, roots []string, cut int) {
	cut = bytes.LastIndexByte(data, '\n') + 1
	rest := data[:cut]
	verdicts := map[string]bool{} // one registry answer per distinct root, local to this pass
	for len(rest) > 0 {
		i := bytes.IndexByte(rest, '\n')
		line := rest[:i+1]
		rest = rest[i+1:]
		root, ok := ledgerRowRoot(line)
		if ok {
			reg, seen := verdicts[root]
			if !seen {
				reg = registered(root)
				verdicts[root] = reg
			}
			ok = !reg
		}
		if ok {
			moved = append(moved, line)
			roots = append(roots, root)
			continue
		}
		kept = append(kept, line)
	}
	return moved, kept, roots, cut
}

// tallyRoots counts moved rows and bytes per root, sorted by root.
func tallyRoots(moved [][]byte, roots []string) []ledgerRootTally {
	by := map[string]*ledgerRootTally{}
	for i, line := range moved {
		t := by[roots[i]]
		if t == nil {
			t = &ledgerRootTally{Root: roots[i]}
			by[roots[i]] = t
		}
		t.Rows++
		t.Bytes += int64(len(line))
	}
	out := make([]ledgerRootTally, 0, len(by))
	for _, t := range by {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}

// planLedgerUnregistered is the dry run over fathomHome's verify and intent
// ledgers: rows and bytes per unregistered root, and where --apply would
// move them. It reads and changes nothing else.
func planLedgerUnregistered(fathomHome string, roots gcLedgerRoots, now time.Time) ledgerUnregisteredPlan {
	p := ledgerUnregisteredPlan{Home: fathomHome, ArchiveDir: filepath.Join(fathomHome, "archive"), Unread: roots.Unread, registered: roots.Registered}
	if p.Unread == "" && p.registered == nil {
		p.Unread = "no registered-roots source was given"
	}
	if p.Unread != "" {
		return p
	}
	day := now.Format("2006-01-02")
	for _, stem := range gcUnregisteredLedgerStems {
		l := ledgerUnregisteredLedger{
			Path:    filepath.Join(fathomHome, stem+".jsonl"),
			Stem:    stem,
			Archive: filepath.Join(p.ArchiveDir, stem+"-unregistered-"+day+".jsonl"),
		}
		data, err := os.ReadFile(l.Path)
		if err != nil {
			if !os.IsNotExist(err) {
				l.Err = err
				p.Ledgers = append(p.Ledgers, l)
			}
			continue
		}
		moved, _, rs, _ := splitLedgerRows(data, p.registered)
		l.Tallies = tallyRoots(moved, rs)
		if len(l.Tallies) > 0 {
			p.Ledgers = append(p.Ledgers, l)
		}
	}
	return p
}

// items renders the plan as gc items: one archive item per ledger and
// unregistered root, or one report item naming why nothing was planned.
func (p ledgerUnregisteredPlan) items() []gcItem {
	if p.Unread != "" {
		return []gcItem{{Class: gcClassLedgerUnregistered, Path: p.Home, Action: "report",
			Reason: "registered roots unread (" + p.Unread + "): nothing planned, no rows counted; rerun when the fleet index answers"}}
	}
	var out []gcItem
	for _, l := range p.Ledgers {
		if l.Err != nil && len(l.Tallies) == 0 {
			out = append(out, gcItem{Class: gcClassLedgerUnregistered, Path: l.Path, Action: "archive", Reason: "ledger unreadable", Error: l.Err.Error()})
			continue
		}
		for _, t := range l.Tallies {
			it := gcItem{Class: gcClassLedgerUnregistered, Path: l.Path, Action: "archive", Bytes: t.Bytes, Rows: t.Rows, Root: t.Root,
				Reason: fmt.Sprintf("%d row(s) from unregistered root %s: --apply moves them to archive/%s, never deletes them (restore: cat it >> the ledger)", t.Rows, t.Root, filepath.Base(l.Archive)),
				Done:   l.Done}
			if l.Err != nil {
				it.Error = l.Err.Error()
			}
			out = append(out, it)
		}
	}
	return out
}

// applyLedgerUnregistered moves each planned ledger's unregistered rows to
// its archive under one lock beside the ledgers. Each ledger's tallies
// become what was actually moved; a failure is that ledger's Err, and its
// live file is untouched. beforeSwap (tests) runs between the read and the
// swap.
func applyLedgerUnregistered(p *ledgerUnregisteredPlan, beforeSwap func(ledger string)) {
	if p.Unread != "" || len(p.Ledgers) == 0 {
		return
	}
	unlock, err := lockLedgerArchive(filepath.Join(p.Home, "ledger-unregistered.lock"))
	if err != nil {
		for i := range p.Ledgers {
			p.Ledgers[i].Err = fmt.Errorf("lock: %w", err)
		}
		return
	}
	defer unlock()
	for i := range p.Ledgers {
		l := &p.Ledgers[i]
		if l.Err != nil {
			continue
		}
		tallies, err := moveUnregisteredRows(l.Path, l.Archive, p.registered, beforeSwap)
		l.Err = err
		if tallies != nil || err == nil {
			// Swapped (a post-swap sweep error is still named): the items
			// are what actually moved.
			l.Tallies, l.Done = tallies, true
		}
	}
}

// lockLedgerArchive takes the class's exclusive lock (flock on path).
func lockLedgerArchive(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// ledgerMoveUndo restores what a failed move changed outside the ledger.
type ledgerMoveUndo struct {
	steps []func()
}

func (u *ledgerMoveUndo) add(f func()) { u.steps = append(u.steps, f) }

func (u *ledgerMoveUndo) run() {
	for i := len(u.steps) - 1; i >= 0; i-- {
		u.steps[i]()
	}
}

// moveUnregisteredRows moves ledger's unregistered-root rows to archive:
// the archive append and its metadata first (each undone on failure), then
// the live file rewritten by a same-directory temp file and a rename that
// carries every byte appended after the read.
func moveUnregisteredRows(ledger, archive string, registered func(string) bool, beforeSwap func(string)) (tallies []ledgerRootTally, err error) {
	f, err := os.Open(ledger)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st0, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ledger, err)
	}
	moved, kept, roots, cut := splitLedgerRows(data, registered)
	if len(moved) == 0 {
		return nil, nil
	}
	if beforeSwap != nil {
		beforeSwap(ledger)
	}

	var undo ledgerMoveUndo
	committed := false
	defer func() {
		if !committed {
			undo.run()
		}
	}()

	// The kept rows, in order, into a temp file beside the ledger.
	tmp, err := os.CreateTemp(filepath.Dir(ledger), "."+filepath.Base(ledger)+".gc-*")
	if err != nil {
		return nil, fmt.Errorf("temp beside %s: %w", ledger, err)
	}
	undo.add(func() { tmp.Close(); os.Remove(tmp.Name()) })
	if err := tmp.Chmod(st0.Mode().Perm()); err != nil {
		return nil, err
	}
	keptBytes := bytes.Join(kept, nil)
	if _, err := tmp.Write(keptBytes); err != nil {
		return nil, fmt.Errorf("write %s: %w", tmp.Name(), err)
	}

	// The moved rows appended to the day's archive, read back byte-identical.
	movedBytes := bytes.Join(moved, nil)
	if err := appendLedgerArchive(archive, movedBytes, &undo); err != nil {
		return nil, err
	}
	if err := updateLedgerArchiveSums(archive, &undo); err != nil {
		return nil, err
	}
	if err := addLedgerArchiveReadmeRow(archive, filepath.Base(ledger), &undo); err != nil {
		return nil, err
	}

	// Carry the tail: every byte a writer appended since the read (and a
	// partial row past cut), until a re-stat finds no more.
	tail, off, err := carryLedgerTail(ledger, f, st0, int64(cut), tmp)
	if err != nil {
		return nil, err
	}

	// Every original row lands in exactly one of archive or live, by count
	// and by bytes, and the temp file is the kept rows plus the tail.
	lines := bytes.Count(data[:cut], []byte{'\n'})
	if len(moved)+len(kept) != lines || len(movedBytes)+len(keptBytes) != cut {
		return nil, fmt.Errorf("partition check failed: %d moved + %d kept of %d rows, %d + %d of %d bytes", len(moved), len(kept), lines, len(movedBytes), len(keptBytes), cut)
	}
	if err := tmp.Sync(); err != nil {
		return nil, err
	}
	got, err := os.ReadFile(tmp.Name())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(got, append(append([]byte{}, keptBytes...), tail...)) {
		return nil, fmt.Errorf("the rewritten ledger is not the kept rows plus the carried tail (%d bytes, want %d)", len(got), len(keptBytes)+len(tail))
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), ledger); err != nil {
		return nil, fmt.Errorf("swap %s: %w", ledger, err)
	}
	committed = true
	syncDir(filepath.Dir(ledger))

	// A writer that opened the old inode before the rename may append to
	// it after the carry: sweep it once more into the new ledger.
	if late, lerr := readLedgerFrom(f, off); lerr != nil {
		return tallyRoots(moved, roots), fmt.Errorf("rows moved; the post-swap sweep of %s could not read the old file: %w", ledger, lerr)
	} else if len(late) > 0 {
		if werr := appendBytes(ledger, late); werr != nil {
			return tallyRoots(moved, roots), fmt.Errorf("rows moved; %d byte(s) appended during the swap could not be carried to %s: %w", len(late), ledger, werr)
		}
	}
	return tallyRoots(moved, roots), nil
}

// carryLedgerTail copies ledger's bytes past off into tmp until a re-stat
// shows no growth, checking the path still names the file read (writers
// only append; anything else aborts). It returns the bytes carried and the
// offset reached.
func carryLedgerTail(ledger string, f *os.File, st0 os.FileInfo, off int64, tmp *os.File) ([]byte, int64, error) {
	var tail []byte
	for try := 0; ; try++ {
		st, err := os.Stat(ledger)
		if err != nil {
			return nil, 0, fmt.Errorf("re-stat %s: %w", ledger, err)
		}
		if !os.SameFile(st, st0) {
			return nil, 0, fmt.Errorf("%s was replaced during apply", ledger)
		}
		if st.Size() < off {
			return nil, 0, fmt.Errorf("%s shrank during apply (%d < %d bytes)", ledger, st.Size(), off)
		}
		if st.Size() == off {
			return tail, off, nil
		}
		if try == gcLedgerCarryTries {
			return nil, 0, fmt.Errorf("%s kept growing during apply; rerun", ledger)
		}
		b, err := readLedgerFrom(f, off)
		if err != nil {
			return nil, 0, err
		}
		if _, err := tmp.Write(b); err != nil {
			return nil, 0, err
		}
		tail = append(tail, b...)
		off += int64(len(b))
	}
}

// readLedgerFrom reads f from off to its current end.
func readLedgerFrom(f *os.File, off int64) ([]byte, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() <= off {
		return nil, nil
	}
	b := make([]byte, st.Size()-off)
	n, err := f.ReadAt(b, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return b[:n], nil
}

// appendBytes appends b to path with O_APPEND, as the ledgers' writers do.
func appendBytes(path string, b []byte) error {
	w, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// appendLedgerArchive appends rows to archive (created with archive/ when
// absent), syncs it and reads the appended region back; undo truncates it to
// its previous length, or removes a file it created.
func appendLedgerArchive(archive string, rows []byte, undo *ledgerMoveUndo) error {
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		return fmt.Errorf("archive dir: %w", err)
	}
	var prev int64 = -1
	if st, err := os.Stat(archive); err == nil {
		prev = st.Size()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	w, err := os.OpenFile(archive, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	undo.add(func() {
		if prev < 0 {
			_ = os.Remove(archive)
		} else {
			_ = os.Truncate(archive, prev)
		}
	})
	if _, err := w.Write(rows); err != nil {
		w.Close()
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	if err := w.Sync(); err != nil {
		w.Close()
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	all, err := os.ReadFile(archive)
	if err != nil {
		return fmt.Errorf("archive %s: %w", archive, err)
	}
	start := prev
	if start < 0 {
		start = 0
	}
	if int64(len(all)) != start+int64(len(rows)) || !bytes.Equal(all[start:], rows) {
		return fmt.Errorf("archive %s: the appended rows did not read back byte-identical", archive)
	}
	return nil
}

// updateLedgerArchiveSums sets archive's line in SHA256SUMS beside it to the
// file's current digest, keeping every other line as it was.
func updateLedgerArchiveSums(archive string, undo *ledgerMoveUndo) error {
	sumsPath := filepath.Join(filepath.Dir(archive), "SHA256SUMS")
	b, err := os.ReadFile(archive)
	if err != nil {
		return fmt.Errorf("SHA256SUMS: %w", err)
	}
	line := fmt.Sprintf("%x  %s", sha256.Sum256(b), filepath.Base(archive))
	prev, err := os.ReadFile(sumsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("SHA256SUMS: %w", err)
	}
	var lines []string
	replaced := false
	for _, l := range strings.Split(strings.TrimRight(string(prev), "\n"), "\n") {
		if l == "" {
			continue
		}
		if f := strings.Fields(l); len(f) == 2 && strings.TrimPrefix(f[1], "*") == filepath.Base(archive) {
			l, replaced = line, true
		}
		lines = append(lines, l)
	}
	if !replaced {
		lines = append(lines, line)
	}
	if err := replaceFileKeepingUndo(sumsPath, prev, err == nil, []byte(strings.Join(lines, "\n")+"\n"), undo); err != nil {
		return fmt.Errorf("SHA256SUMS: %w", err)
	}
	return nil
}

// addLedgerArchiveReadmeRow adds archive's row to the README's table beside
// it (after the table's last row), creating the README with the owner's
// header when absent; an archive the table already names is left as is.
func addLedgerArchiveReadmeRow(archive, ledgerName string, undo *ledgerMoveUndo) error {
	readme := filepath.Join(filepath.Dir(archive), "README.md")
	name := filepath.Base(archive)
	row := fmt.Sprintf("| %s | %s | repo_root not a registered service root (`fathom gc --apply`, class %s) |", name, ledgerName, gcClassLedgerUnregistered)
	prev, err := os.ReadFile(readme)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("README.md: %w", err)
	}
	existed := err == nil
	var next string
	switch {
	case !existed:
		next = "# Ledger archive\n\n" +
			"Rows moved out of the live ledgers by `fathom gc --apply`, class " + gcClassLedgerUnregistered + " (plan progress-and-economics PE-29). " +
			"Moved, never deleted: every original row is in exactly one of the archive file here or the live ledger. Rows are byte-identical to the originals.\n\n" +
			gcLedgerArchiveReadmeHeader + "\n|---|---|---|\n" + row + "\n\n" +
			"Restore a ledger: append its archive file back (`cat <archive> >> <ledger>`); readers sort by timestamp, so order in the file is not significant. SHA256SUMS here covers the archive files.\n"
	case strings.Contains(string(prev), "| "+name+" |"):
		return nil
	default:
		lines := strings.SplitAfter(string(prev), "\n")
		at := -1
		for i, l := range lines {
			if strings.TrimSpace(l) == gcLedgerArchiveReadmeHeader {
				at = i
				for j := i + 1; j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|"); j++ {
					at = j
				}
				break
			}
		}
		if at < 0 {
			sep := "\n"
			if !strings.HasSuffix(string(prev), "\n") {
				sep = "\n\n"
			}
			next = string(prev) + sep + gcLedgerArchiveReadmeHeader + "\n|---|---|---|\n" + row + "\n"
		} else {
			if !strings.HasSuffix(lines[at], "\n") {
				lines[at] += "\n"
			}
			next = strings.Join(lines[:at+1], "") + row + "\n" + strings.Join(lines[at+1:], "")
		}
	}
	if err := replaceFileKeepingUndo(readme, prev, existed, []byte(next), undo); err != nil {
		return fmt.Errorf("README.md: %w", err)
	}
	return nil
}

// replaceFileKeepingUndo writes content to path by a temp file and a rename;
// undo puts prev back (or removes the file when it did not exist).
func replaceFileKeepingUndo(path string, prev []byte, existed bool, content []byte, undo *ledgerMoveUndo) error {
	if err := writeFileRenamed(path, content); err != nil {
		return err
	}
	undo.add(func() {
		if existed {
			_ = writeFileRenamed(path, prev)
		} else {
			_ = os.Remove(path)
		}
	})
	return nil
}

// writeFileRenamed writes content to a temp file beside path, then renames,
// keeping path's mode when it exists (0644 when new).
func writeFileRenamed(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".gc-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// syncDir fsyncs dir so a rename in it is durable (best-effort).
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}

// ledgerUnregisteredClass plans the class over home's FATHOM_HOME and, with
// apply, moves the rows; it returns the class's items and errors.
func ledgerUnregisteredClass(home string, roots gcLedgerRoots, now time.Time, apply bool, beforeSwap func(string)) ([]gcItem, []string) {
	fh := fathomhome.Under(home)
	if fh == "" {
		return nil, nil
	}
	p := planLedgerUnregistered(fh, roots, now)
	if apply {
		applyLedgerUnregistered(&p, beforeSwap)
	}
	var errs []string
	for _, l := range p.Ledgers {
		if l.Err != nil {
			errs = append(errs, fmt.Sprintf("%s %s: %v", gcClassLedgerUnregistered, l.Path, l.Err))
		}
	}
	return p.items(), errs
}
