package benchprep

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// ErrDemotedSplit reports that a split has a Demoted line in the held-out
// record: it is a dev set from here on, so a read of it is not a held-out read.
// RecordHeldOutRead wraps it, so a caller can tell a demotion from any other
// refusal with errors.Is.
var ErrDemotedSplit = errors.New("demoted to dev: not a held-out read")

// ErrSecondHeldOutRead reports that a split sha already has a Read line: a
// held-out split is read once (BX-29), and a second read is refused until the
// controller demotes the split. RecordHeldOutRead wraps it.
var ErrSecondHeldOutRead = errors.New("second held-out read refused")

// HeldOutRead is one read of a held-out split, as the caller describes it. It
// carries no scenariobench type, so any harness that reads a held-out split
// (scenariobench run, a labs caller) can record the read.
type HeldOutRead struct {
	// Split is the split name the caller believes it is reading, "held-out" or
	// "post-cutoff". It must equal the name on the split's Held-out line.
	Split string
	// SplitSHA256 is the split's identity (SplitSHA256): 64 lowercase hex.
	SplitSHA256 string
	// PreregID names the signed pre-registration the read is under; no whitespace.
	PreregID string
	// PreregSHA256 is the pre-registration's content digest: 64 lowercase hex.
	PreregSHA256 string
	// PreregSigned is the pre-registration's signed line as the caller read it
	// from the prereg file; it must match `^Prereg \S+ v[0-9]+ signed`.
	PreregSigned string
	// RunID names the run that reads the split; no whitespace.
	RunID string
	// Commit is the commit the run starts from: 7 to 64 lowercase hex.
	Commit string
}

var (
	sha256HexRE    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	commitHexRE    = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	preregSignedRE = regexp.MustCompile(`^Prereg \S+ v[0-9]+ signed`)
)

// validate checks every field of the read and names the first bad one.
func (r HeldOutRead) validate() error {
	token := func(field, v string) error {
		if v == "" {
			return fmt.Errorf("held-out read: %s is empty", field)
		}
		if hasSpace(v) {
			return fmt.Errorf("held-out read: %s %q contains whitespace", field, v)
		}
		return nil
	}
	if err := token("Split", r.Split); err != nil {
		return err
	}
	if !sha256HexRE.MatchString(r.SplitSHA256) {
		return fmt.Errorf("held-out read: SplitSHA256 %q is not 64 lowercase hex characters", r.SplitSHA256)
	}
	if err := token("PreregID", r.PreregID); err != nil {
		return err
	}
	if !sha256HexRE.MatchString(r.PreregSHA256) {
		return fmt.Errorf("held-out read: PreregSHA256 %q is not 64 lowercase hex characters", r.PreregSHA256)
	}
	if !preregSignedRE.MatchString(r.PreregSigned) {
		return fmt.Errorf("held-out read: PreregSigned %q does not match %q: the prereg must carry a signed line", r.PreregSigned, preregSignedRE.String())
	}
	if err := token("RunID", r.RunID); err != nil {
		return err
	}
	if !commitHexRE.MatchString(r.Commit) {
		return fmt.Errorf("held-out read: Commit %q is not 7 to 64 lowercase hex characters", r.Commit)
	}
	return nil
}

// RecordHeldOutRead appends one Read line for read to the held-out record at
// record (absolute, or relative to root; root also resolves the record's
// repo-relative Dev lines) and returns the entry exactly as ParseHeldOutRecord
// would parse it from the file afterwards. now supplies the timestamp (nil
// means time.Now); it is rendered in UTC, RFC 3339, ending in Z.
//
// Every refusal returns an error and appends nothing. The checks run in this
// order:
//
//  1. The read itself: Split non-empty without whitespace; SplitSHA256 and
//     PreregSHA256 64 lowercase hex; PreregID non-empty without whitespace;
//     PreregSigned matching `^Prereg \S+ v[0-9]+ signed`; RunID non-empty
//     without whitespace; Commit 7 to 64 lowercase hex. The error names the
//     bad field.
//  2. The record, through ParseHeldOutRecord. An absent record is refused (a
//     held-out read needs a committed split) with ErrNoHeldOutRecord kept in the
//     error chain; an unreadable or malformed record is refused.
//  3. The split sha must have a Held-out line, else the split was never
//     committed or has drifted (BX-11).
//  4. read.Split must equal that line's split name.
//  5. A Demoted line for the sha: an error wrapping ErrDemotedSplit, naming the
//     Demoted line. A demotion wins over an earlier read.
//  6. An existing Read line for the sha: an error wrapping ErrSecondHeldOutRead,
//     naming the first read's line and text. A second read is refused until the
//     split is demoted (BX-29).
//
// Otherwise exactly one line is appended:
//
//	Read: <split sha> split <name> prereg <id> <prereg sha> run <run id> commit <commit> at <RFC3339 UTC>
//
// The append is one Write call on a file opened os.O_APPEND|os.O_WRONLY, with
// no os.O_CREATE (the record must already exist). When the existing file is
// non-empty and does not end in a newline, a newline is prepended inside that
// same single write. RecordHeldOutRead never rewrites or truncates an earlier
// byte, and a short write, a write error or a close error is a refusal. There
// is no cross-process lock: the controller serialises runs. It runs no git and
// no shell.
//
// The appended line dirties the working tree. A caller therefore runs every
// clean-tree check BEFORE calling it (LoadPrereg's check, the record against
// HEAD, the dirty_tree stamp of NP-B5) and commits the Read line before the next
// prereg run.
func RecordHeldOutRead(root, record string, read HeldOutRead, now func() time.Time) (HeldOutReadEntry, error) {
	if err := read.validate(); err != nil {
		return HeldOutReadEntry{}, err
	}
	rec, err := ParseHeldOutRecord(root, record)
	if err != nil {
		if errors.Is(err, ErrNoHeldOutRecord) {
			return HeldOutReadEntry{}, fmt.Errorf("held-out read: a held-out read needs a committed split: %w", err)
		}
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: %w", err)
	}

	var held *HeldOutSplit
	for i := range rec.HeldOut {
		if rec.HeldOut[i].SHA256 == read.SplitSHA256 {
			held = &rec.HeldOut[i]
			break
		}
	}
	if held == nil {
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: split sha256 %s has no Held-out split sha256 line: the split was never committed, or has drifted (BX-11)", read.SplitSHA256)
	}
	if read.Split != held.Split {
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: split sha256 %s is a %q split on line %d, but the read names %q", read.SplitSHA256, held.Split, held.Line, read.Split)
	}
	for _, d := range rec.Demoted {
		if d.SplitSHA256 == read.SplitSHA256 {
			return HeldOutReadEntry{}, fmt.Errorf("held-out read: split sha256 %s: %w (Demoted line %d: %s)", read.SplitSHA256, ErrDemotedSplit, d.Line, d.Text)
		}
	}
	for _, r := range rec.Reads {
		if r.SplitSHA256 == read.SplitSHA256 {
			return HeldOutReadEntry{}, fmt.Errorf("held-out read: split sha256 %s: %w: it was already read on line %d (%s); a second read is refused until the split is demoted (BX-29)", read.SplitSHA256, ErrSecondHeldOutRead, r.Line, r.Text)
		}
	}

	if now == nil {
		now = time.Now
	}
	line := fmt.Sprintf("%s%s split %s prereg %s %s run %s commit %s at %s",
		PrefixHeldOutRead, read.SplitSHA256, read.Split, read.PreregID, read.PreregSHA256, read.RunID, read.Commit,
		now().UTC().Format(time.RFC3339))

	full := record
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	existing, err := os.ReadFile(full)
	if err != nil {
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: reading %s before the append: %v", full, err)
	}
	payload := line + "\n"
	lineNo := bytes.Count(existing, []byte{'\n'}) + 1
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte{'\n'}) {
		payload = "\n" + payload
		lineNo++
	}
	entry, ok := parseReadLine(line, lineNo)
	if !ok {
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: the line built from the read does not parse: %q", line)
	}

	f, err := os.OpenFile(full, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: opening %s for append: %v", full, err)
	}
	n, werr := f.Write([]byte(payload))
	cerr := f.Close()
	switch {
	case werr != nil:
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: appending to %s: %v", full, werr)
	case n != len(payload):
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: appending to %s: short write (%d of %d bytes)", full, n, len(payload))
	case cerr != nil:
		return HeldOutReadEntry{}, fmt.Errorf("held-out read: closing %s after the append: %v", full, cerr)
	}
	return entry, nil
}
