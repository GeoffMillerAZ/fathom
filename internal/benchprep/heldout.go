package benchprep

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// The four record-line prefixes of docs/bench/heldout-reads.md. A line that
// starts at column 0 with one of them is a record line and must parse; every
// other line is prose.
const (
	// PrefixDevSplit opens a Dev line: a task set declared a dev (tuning) set.
	PrefixDevSplit = "Dev split sha256: "
	// PrefixHeldOutSplit opens a Held-out line: a committed held-out split.
	PrefixHeldOutSplit = "Held-out split sha256: "
	// PrefixHeldOutRead opens a Read line: one read of a held-out split.
	PrefixHeldOutRead = "Read: "
	// PrefixDemoted opens a Demoted line: a split that is a dev set from here on.
	PrefixDemoted = "Demoted: "
)

// ErrNoHeldOutRecord reports that the record file itself is absent: the read of
// the record path failed with os.ErrNotExist. It is never returned for any
// other failure (an unreadable record, a missing dev file named inside the
// record), so a caller can treat it as "no record yet" without ever mistaking
// unreadable input for absence (I38).
var ErrNoHeldOutRecord = errors.New("held-out record: record file is absent")

// HeldOutRecord is the parsed docs/bench/heldout-reads.md. Every slice is in
// file (line) order.
type HeldOutRecord struct {
	Dev     []DevSplit
	HeldOut []HeldOutSplit
	Reads   []HeldOutReadEntry
	Demoted []SplitDemotion
}

// DevSplit is one Dev line: a task set declared a dev set. The parser loaded
// Path through contextbench.LoadTasks and verified Tasks and SHA256 against it.
type DevSplit struct {
	SHA256 string
	Path   string
	Tasks  int
	IDs    []string // the file's task ids, sorted ascending
	Line   int      // 1-based
}

// HeldOutSplit is one Held-out line: a committed held-out split.
type HeldOutSplit struct {
	SHA256 string
	Split  string // "held-out" or "post-cutoff"
	Tasks  int
	Line   int // 1-based
}

// HeldOutReadEntry is one Read line: one read of a held-out split.
type HeldOutReadEntry struct {
	SplitSHA256  string
	Split        string
	PreregID     string
	PreregSHA256 string
	RunID        string
	Commit       string
	At           time.Time // UTC
	Line         int       // 1-based
	Text         string    // the line, without its line ending
}

// SplitDemotion is one Demoted line: the split is a dev set from here on.
type SplitDemotion struct {
	SplitSHA256 string
	Reason      string
	Line        int    // 1-based
	Text        string // the line, without its line ending
}

var (
	devLineRE     = regexp.MustCompile(`^Dev split sha256: ([0-9a-f]{64}) (\S+) \((\d+) tasks\)$`)
	heldOutLineRE = regexp.MustCompile(`^Held-out split sha256: ([0-9a-f]{64}) (held-out|post-cutoff) \((\d+) tasks\)$`)
	readLineRE    = regexp.MustCompile(`^Read: ([0-9a-f]{64}) split (\S+) prereg (\S+) ([0-9a-f]{64}) run (\S+) commit ([0-9a-f]{7,64}) at (\S+)$`)
	demotedLineRE = regexp.MustCompile(`^Demoted: ([0-9a-f]{64}) (\S.*)$`)
)

// SplitSHA256 returns the canonical identity of a task split: the lowercase
// sha256 hex (64 chars) of the task ids sorted ascending in byte order
// (sort.Strings on a copy, so the caller's slice is never mutated), each id
// immediately followed by a single "\n" byte, with no other separator, header
// or trailing bytes. The digest is therefore stable under input order.
//
// It refuses an empty list, an empty id, an id containing "\n" or "\r" (either
// would make the framing ambiguous), and a duplicate id (the error names it).
func SplitSHA256(ids []string) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("split sha256: empty id list")
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	h := sha256.New()
	for i, id := range sorted {
		switch {
		case id == "":
			return "", errors.New("split sha256: empty id")
		case strings.Contains(id, "\n"):
			return "", fmt.Errorf("split sha256: id %q contains a newline", id)
		case strings.Contains(id, "\r"):
			return "", fmt.Errorf("split sha256: id %q contains a carriage return", id)
		case i > 0 && sorted[i-1] == id:
			return "", fmt.Errorf("split sha256: duplicate id %q", id)
		}
		h.Write([]byte(id))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ParseHeldOutRecord reads and validates the held-out record at path (absolute,
// or relative to root), the file docs/bench/heldout-reads.md.
//
// A line (trailing "\r" stripped) that starts at column 0 with PrefixDevSplit,
// PrefixHeldOutSplit, PrefixHeldOutRead or PrefixDemoted is a record line and
// must match its format exactly, else the whole record is refused with an error
// naming the 1-based line number and the line. Every other line is prose.
//
//	Dev split sha256: <64 hex> <repo-relative slash path> (<n> tasks)
//	Held-out split sha256: <64 hex> <held-out|post-cutoff> (<n> tasks)
//	Read: <64 hex split sha> split <name> prereg <id> <64 hex prereg sha> run <run id> commit <7-64 hex> at <RFC3339 UTC, ending in Z>
//	Demoted: <64 hex> <reason>
//
// Each Dev line's path is resolved against root (an absolute path, a ".."
// element or a non-clean path is refused) and loaded through
// contextbench.LoadTasks; the line is refused when the file cannot be read or
// loaded, when its task count differs from (<n> tasks), or when SplitSHA256 of
// its ids differs from the recorded sha (drift: the error names the line, the
// recorded and the computed sha). After the whole file is read: a duplicate Dev
// sha or Held-out sha, a sha on both a Dev and a Held-out line, a Read or
// Demoted line whose sha has no Held-out line, and a Read whose split name
// differs from its Held-out line's are all refused.
//
// An absent record file returns an error wrapping ErrNoHeldOutRecord. Any other
// failure to read it returns a different error that does not match it, and the
// dev-file failure is formatted with %v so a missing dev file can never satisfy
// errors.Is(err, os.ErrNotExist) or ErrNoHeldOutRecord: unreadable input is
// never "absent" (I38).
func ParseHeldOutRecord(root, path string) (HeldOutRecord, error) {
	var rec HeldOutRecord
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rec, fmt.Errorf("%w: %s", ErrNoHeldOutRecord, full)
		}
		return rec, fmt.Errorf("held-out record: reading %s: %v", full, err)
	}

	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSuffix(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, PrefixDevSplit):
			dev, err := parseDevLine(root, line, lineNo)
			if err != nil {
				return HeldOutRecord{}, err
			}
			rec.Dev = append(rec.Dev, dev)
		case strings.HasPrefix(line, PrefixHeldOutSplit):
			m := heldOutLineRE.FindStringSubmatch(line)
			n, ok := atoiCount(m)
			if !ok {
				return HeldOutRecord{}, malformedLine(lineNo, line)
			}
			rec.HeldOut = append(rec.HeldOut, HeldOutSplit{SHA256: m[1], Split: m[2], Tasks: n, Line: lineNo})
		case strings.HasPrefix(line, PrefixHeldOutRead):
			entry, ok := parseReadLine(line, lineNo)
			if !ok {
				return HeldOutRecord{}, malformedLine(lineNo, line)
			}
			rec.Reads = append(rec.Reads, entry)
		case strings.HasPrefix(line, PrefixDemoted):
			m := demotedLineRE.FindStringSubmatch(line)
			if m == nil {
				return HeldOutRecord{}, malformedLine(lineNo, line)
			}
			rec.Demoted = append(rec.Demoted, SplitDemotion{SplitSHA256: m[1], Reason: m[2], Line: lineNo, Text: line})
		default:
			// A record line behind indentation or a list marker would otherwise read as
			// prose and vanish: a hidden Demoted or Read line must refuse, not pass.
			if hasRecordPrefix(strings.TrimLeft(line, " \t-*+>")) {
				return HeldOutRecord{}, fmt.Errorf("held-out record: line %d hides a record prefix behind indentation or a list marker; start it at column 0: %q", lineNo, line)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return HeldOutRecord{}, fmt.Errorf("held-out record: reading %s: %v", full, err)
	}
	if err := crossCheckHeldOut(rec); err != nil {
		return HeldOutRecord{}, err
	}
	return rec, nil
}

func malformedLine(lineNo int, line string) error {
	return fmt.Errorf("held-out record: line %d is malformed: %q", lineNo, line)
}

// hasRecordPrefix reports whether s opens with one of the record's line prefixes.
func hasRecordPrefix(s string) bool {
	for _, p := range []string{PrefixDevSplit, PrefixHeldOutSplit, PrefixHeldOutRead, PrefixDemoted} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// atoiCount reads the task count (the third submatch) of a Dev or Held-out
// line; ok is false when the line did not match or the count overflows.
func atoiCount(m []string) (int, bool) {
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[3])
	return n, err == nil
}

func hasSpace(s string) bool {
	return strings.IndexFunc(s, unicode.IsSpace) >= 0
}

func parseReadLine(line string, lineNo int) (HeldOutReadEntry, bool) {
	m := readLineRE.FindStringSubmatch(line)
	if m == nil {
		return HeldOutReadEntry{}, false
	}
	for _, field := range []string{m[2], m[3], m[5]} {
		if hasSpace(field) {
			return HeldOutReadEntry{}, false
		}
	}
	if !strings.HasSuffix(m[7], "Z") {
		return HeldOutReadEntry{}, false
	}
	at, err := time.Parse(time.RFC3339, m[7])
	if err != nil {
		return HeldOutReadEntry{}, false
	}
	return HeldOutReadEntry{
		SplitSHA256: m[1], Split: m[2], PreregID: m[3], PreregSHA256: m[4], RunID: m[5], Commit: m[6],
		At: at.UTC(), Line: lineNo, Text: line,
	}, true
}

func parseDevLine(root, line string, lineNo int) (DevSplit, error) {
	m := devLineRE.FindStringSubmatch(line)
	n, ok := atoiCount(m)
	if !ok {
		return DevSplit{}, malformedLine(lineNo, line)
	}
	sha, rel := m[1], m[2]
	if hasSpace(rel) {
		return DevSplit{}, malformedLine(lineNo, line)
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: dev path %q is absolute; want a repo-relative slash path", lineNo, rel)
	}
	for _, elem := range strings.Split(rel, "/") {
		if elem == ".." {
			return DevSplit{}, fmt.Errorf("held-out record: line %d: dev path %q has a .. element", lineNo, rel)
		}
	}
	if path.Clean(rel) != rel {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: dev path %q is not clean (want %q)", lineNo, rel, path.Clean(rel))
	}
	tasks, err := contextbench.LoadTasks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: dev file %s cannot be loaded: %v", lineNo, rel, err)
	}
	if len(tasks) != n {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: records %d tasks but %s has %d", lineNo, n, rel, len(tasks))
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	computed, err := SplitSHA256(ids)
	if err != nil {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: dev file %s: %v", lineNo, rel, err)
	}
	if computed != sha {
		return DevSplit{}, fmt.Errorf("held-out record: line %d: dev file %s has drifted: recorded sha256 %s, computed %s", lineNo, rel, sha, computed)
	}
	sort.Strings(ids)
	return DevSplit{SHA256: sha, Path: rel, Tasks: n, IDs: ids, Line: lineNo}, nil
}

// crossCheckHeldOut applies the whole-file checks, after every line parsed.
func crossCheckHeldOut(rec HeldOutRecord) error {
	dev := map[string]int{}
	for _, d := range rec.Dev {
		if first, dup := dev[d.SHA256]; dup {
			return fmt.Errorf("held-out record: line %d: duplicate Dev split sha256 %s (first on line %d)", d.Line, d.SHA256, first)
		}
		dev[d.SHA256] = d.Line
	}
	held := map[string]HeldOutSplit{}
	for _, h := range rec.HeldOut {
		if first, dup := held[h.SHA256]; dup {
			return fmt.Errorf("held-out record: line %d: duplicate Held-out split sha256 %s (first on line %d)", h.Line, h.SHA256, first.Line)
		}
		if devLine, both := dev[h.SHA256]; both {
			return fmt.Errorf("held-out record: line %d: split sha256 %s is also a Dev split on line %d", h.Line, h.SHA256, devLine)
		}
		held[h.SHA256] = h
	}
	for _, r := range rec.Reads {
		h, ok := held[r.SplitSHA256]
		if !ok {
			return fmt.Errorf("held-out record: line %d: Read of split sha256 %s, which has no Held-out line", r.Line, r.SplitSHA256)
		}
		if r.Split != h.Split {
			return fmt.Errorf("held-out record: line %d: Read names split %q but the Held-out line %d says %q", r.Line, r.Split, h.Line, h.Split)
		}
	}
	for _, d := range rec.Demoted {
		if _, ok := held[d.SplitSHA256]; !ok {
			return fmt.Errorf("held-out record: line %d: Demoted split sha256 %s has no Held-out line", d.Line, d.SplitSHA256)
		}
	}
	return nil
}
