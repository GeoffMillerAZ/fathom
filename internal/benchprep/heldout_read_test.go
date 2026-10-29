package benchprep

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readFixture is a record with one committed Held-out split and a valid read
// for it.
type readFixture struct {
	*heldoutFixture
	splitSHA string
	heldLine string
}

func newReadFixture(t *testing.T) *readFixture {
	t.Helper()
	return &readFixture{
		heldoutFixture: newHeldoutFixture(t),
		splitSHA:       shaOf("split-a"),
		heldLine:       "Held-out split sha256: " + shaOf("split-a") + " held-out (5 tasks)",
	}
}

func (f *readFixture) path() string { return filepath.Join(f.root, "record.md") }

func (f *readFixture) raw() string {
	f.t.Helper()
	b, err := os.ReadFile(f.path())
	if err != nil {
		f.t.Fatalf("read record: %v", err)
	}
	return string(b)
}

func (f *readFixture) validRead() HeldOutRead {
	return HeldOutRead{
		Split:        "held-out",
		SplitSHA256:  f.splitSHA,
		PreregID:     "H4",
		PreregSHA256: shaOf("prereg-h4"),
		PreregSigned: "Prereg H4 v1 signed by the controller",
		RunID:        "run-001",
		Commit:       "abc1234",
	}
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 26, 8, 30, 0, 0, time.UTC) }

func (f *readFixture) mustParse() HeldOutRecord {
	f.t.Helper()
	rec, err := ParseHeldOutRecord(f.root, "record.md")
	if err != nil {
		f.t.Fatalf("parse record: %v", err)
	}
	return rec
}

func TestRecordHeldOutRead_FirstReadAppendsOneLine(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", "# Record\n\nprose line\n"+f.heldLine+"\n\n## Read log\n")
	before := f.raw()

	entry, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err != nil {
		t.Fatalf("RecordHeldOutRead: %v", err)
	}
	after := f.raw()
	if !strings.HasPrefix(after, before) {
		t.Fatalf("earlier bytes changed:\nbefore %q\nafter  %q", before, after)
	}
	added := strings.TrimPrefix(after, before)
	want := "Read: " + f.splitSHA + " split held-out prereg H4 " + shaOf("prereg-h4") + " run run-001 commit abc1234 at 2026-10-26T08:30:00Z\n"
	if added != want {
		t.Fatalf("appended bytes = %q, want %q", added, want)
	}
	rec := f.mustParse()
	if len(rec.Reads) != 1 {
		t.Fatalf("re-parse: %d reads, want 1", len(rec.Reads))
	}
	if rec.Reads[0] != entry {
		t.Fatalf("returned entry and re-parse disagree:\nreturned %+v\nparsed   %+v", entry, rec.Reads[0])
	}
	if entry.Line != strings.Count(before, "\n")+1 {
		t.Fatalf("entry.Line = %d, want %d", entry.Line, strings.Count(before, "\n")+1)
	}
	if entry.Text != strings.TrimSuffix(want, "\n") {
		t.Fatalf("entry.Text = %q", entry.Text)
	}
}

func TestRecordHeldOutRead_AbsoluteRecordPath(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	entry, err := RecordHeldOutRead("/does/not/matter", f.path(), f.validRead(), fixedNow)
	if err != nil {
		t.Fatalf("RecordHeldOutRead: %v", err)
	}
	if entry.Line != 2 {
		t.Fatalf("entry.Line = %d, want 2", entry.Line)
	}
}

func TestRecordHeldOutRead_SecondReadRefused(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	first, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	before := f.raw()

	second := f.validRead()
	second.PreregID = "H5"
	second.RunID = "run-002"
	_, err = RecordHeldOutRead(f.root, "record.md", second, fixedNow)
	if !errors.Is(err, ErrSecondHeldOutRead) {
		t.Fatalf("err = %v, want ErrSecondHeldOutRead", err)
	}
	for _, want := range []string{"line 2", first.Text, "demoted", "BX-29"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err %q does not contain %q", err, want)
		}
	}
	if f.raw() != before {
		t.Fatalf("file changed on a refused second read")
	}
}

func TestRecordHeldOutRead_DemotedRefused(t *testing.T) {
	demoted := "Demoted: " + shaOf("split-a") + " tuned on it"
	t.Run("no read yet", func(t *testing.T) {
		f := newReadFixture(t)
		f.write("record.md", f.heldLine+"\n"+demoted+"\n")
		before := f.raw()
		_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
		if !errors.Is(err, ErrDemotedSplit) {
			t.Fatalf("err = %v, want ErrDemotedSplit", err)
		}
		for _, want := range []string{"demoted to dev: not a held-out read", "line 2"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("err %q does not contain %q", err, want)
			}
		}
		if f.raw() != before {
			t.Fatalf("file changed on a refused read of a demoted split")
		}
	})
	t.Run("already read", func(t *testing.T) {
		f := newReadFixture(t)
		readLine := "Read: " + f.splitSHA + " split held-out prereg H4 " + shaOf("prereg-h4") + " run run-000 commit abc1234 at 2026-10-25T00:00:00Z"
		f.write("record.md", f.heldLine+"\n"+readLine+"\n"+demoted+"\n")
		before := f.raw()
		_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
		if !errors.Is(err, ErrDemotedSplit) {
			t.Fatalf("err = %v, want ErrDemotedSplit", err)
		}
		if errors.Is(err, ErrSecondHeldOutRead) {
			t.Fatalf("err = %v matches ErrSecondHeldOutRead; demotion wins", err)
		}
		if !strings.Contains(err.Error(), "line 3") {
			t.Fatalf("err %q does not name the Demoted line 3", err)
		}
		if f.raw() != before {
			t.Fatalf("file changed on a refused read of a demoted split")
		}
	})
}

func TestRecordHeldOutRead_NoHeldOutLineRefused(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", "Held-out split sha256: "+shaOf("other")+" held-out (5 tasks)\n")
	before := f.raw()
	_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err == nil {
		t.Fatalf("a sha with no Held-out line was accepted")
	}
	if errors.Is(err, ErrDemotedSplit) || errors.Is(err, ErrSecondHeldOutRead) || errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("err = %v matches the wrong sentinel", err)
	}
	if !strings.Contains(err.Error(), "no Held-out split sha256 line") || !strings.Contains(err.Error(), "BX-11") {
		t.Fatalf("err = %v", err)
	}
	if f.raw() != before {
		t.Fatalf("file changed on a refusal")
	}
}

func TestRecordHeldOutRead_AbsentRecordRefusedAndNotCreated(t *testing.T) {
	f := newReadFixture(t)
	_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if !errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("err = %v, want ErrNoHeldOutRecord", err)
	}
	if _, statErr := os.Stat(f.path()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("record was created: stat err = %v", statErr)
	}
}

func TestRecordHeldOutRead_UnreadableRecordRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 does not make a file unreadable")
	}
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	if err := os.Chmod(f.path(), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.path(), 0o644) })
	_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err == nil {
		t.Fatalf("an unreadable record was accepted")
	}
	if errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("an unreadable record matched ErrNoHeldOutRecord: %v", err)
	}
	_ = os.Chmod(f.path(), 0o644)
	if f.raw() != f.heldLine+"\n" {
		t.Fatalf("file changed on a refusal")
	}
}

func TestRecordHeldOutRead_InvalidReadRefused(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(r *HeldOutRead)
		field  string
	}{
		{"empty split", func(r *HeldOutRead) { r.Split = "" }, "Split"},
		{"split with space", func(r *HeldOutRead) { r.Split = "held out" }, "Split"},
		{"split sha short", func(r *HeldOutRead) { r.SplitSHA256 = "abc" }, "SplitSHA256"},
		{"split sha uppercase", func(r *HeldOutRead) { r.SplitSHA256 = strings.ToUpper(shaOf("split-a")) }, "SplitSHA256"},
		{"empty prereg id", func(r *HeldOutRead) { r.PreregID = "" }, "PreregID"},
		{"prereg id with space", func(r *HeldOutRead) { r.PreregID = "H 4" }, "PreregID"},
		{"prereg sha not 64 hex", func(r *HeldOutRead) { r.PreregSHA256 = "deadbeef" }, "PreregSHA256"},
		{"prereg sha non-hex", func(r *HeldOutRead) { r.PreregSHA256 = strings.Repeat("g", 64) }, "PreregSHA256"},
		{"signed missing version", func(r *HeldOutRead) { r.PreregSigned = "Prereg H4 signed" }, "PreregSigned"},
		{"signed lowercase", func(r *HeldOutRead) { r.PreregSigned = "prereg H4 v1 signed" }, "PreregSigned"},
		{"signed empty", func(r *HeldOutRead) { r.PreregSigned = "" }, "PreregSigned"},
		{"signed unsigned", func(r *HeldOutRead) { r.PreregSigned = "Prereg H4 v1 unsigned" }, "PreregSigned"},
		{"empty run id", func(r *HeldOutRead) { r.RunID = "" }, "RunID"},
		{"run id with space", func(r *HeldOutRead) { r.RunID = "run 1" }, "RunID"},
		{"commit too short", func(r *HeldOutRead) { r.Commit = "abc123" }, "Commit"},
		{"commit too long", func(r *HeldOutRead) { r.Commit = strings.Repeat("a", 65) }, "Commit"},
		{"commit uppercase", func(r *HeldOutRead) { r.Commit = "ABC1234" }, "Commit"},
		{"commit empty", func(r *HeldOutRead) { r.Commit = "" }, "Commit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReadFixture(t)
			f.write("record.md", f.heldLine+"\n")
			before := f.raw()
			read := f.validRead()
			tc.mutate(&read)
			_, err := RecordHeldOutRead(f.root, "record.md", read, fixedNow)
			if err == nil {
				t.Fatalf("invalid read accepted")
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("err %q does not name field %s", err, tc.field)
			}
			if f.raw() != before {
				t.Fatalf("file changed on a refusal")
			}
		})
	}
}

func TestRecordHeldOutRead_ValidationPrecedesRecordAccess(t *testing.T) {
	f := newReadFixture(t)
	read := f.validRead()
	read.PreregID = ""
	_, err := RecordHeldOutRead(f.root, "record.md", read, fixedNow)
	if err == nil || errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("err = %v, want a validation error before the record is touched", err)
	}
}

func TestRecordHeldOutRead_SplitNameMismatchRefused(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	before := f.raw()
	read := f.validRead()
	read.Split = "post-cutoff"
	_, err := RecordHeldOutRead(f.root, "record.md", read, fixedNow)
	if err == nil {
		t.Fatalf("a split-name mismatch was accepted")
	}
	if !strings.Contains(err.Error(), "post-cutoff") || !strings.Contains(err.Error(), "held-out") {
		t.Fatalf("err %q does not name both split names", err)
	}
	if f.raw() != before {
		t.Fatalf("file changed on a refusal")
	}
}

func TestRecordHeldOutRead_ClockRenderedAsUTC(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	zone := time.FixedZone("x", -7*3600)
	now := func() time.Time { return time.Date(2026, 10, 26, 1, 2, 3, 0, zone) }
	entry, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), now)
	if err != nil {
		t.Fatalf("RecordHeldOutRead: %v", err)
	}
	if !strings.HasSuffix(entry.Text, " at 2026-10-26T08:02:03Z") {
		t.Fatalf("entry.Text = %q, want UTC form ending in Z", entry.Text)
	}
	if !strings.Contains(f.raw(), " at 2026-10-26T08:02:03Z\n") {
		t.Fatalf("file lacks the UTC time: %q", f.raw())
	}
	if want := time.Date(2026, 10, 26, 8, 2, 3, 0, time.UTC); !entry.At.Equal(want) || entry.At.Location() != time.UTC {
		t.Fatalf("entry.At = %v, want %v UTC", entry.At, want)
	}
}

func TestRecordHeldOutRead_NilClockUsesNow(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\n")
	lo := time.Now().UTC().Add(-2 * time.Second)
	entry, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), nil)
	if err != nil {
		t.Fatalf("RecordHeldOutRead: %v", err)
	}
	hi := time.Now().UTC().Add(2 * time.Second)
	if entry.At.Before(lo) || entry.At.After(hi) {
		t.Fatalf("entry.At = %v, want within [%v, %v]", entry.At, lo, hi)
	}
}

func TestRecordHeldOutRead_NoTrailingNewline(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine) // no trailing newline
	before := f.raw()
	entry, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err != nil {
		t.Fatalf("RecordHeldOutRead: %v", err)
	}
	after := f.raw()
	if !strings.HasPrefix(after, before+"\n"+PrefixHeldOutRead) {
		t.Fatalf("Read line is not on its own new line: %q", after)
	}
	if !strings.HasSuffix(after, "\n") {
		t.Fatalf("appended line lacks a trailing newline: %q", after)
	}
	if entry.Line != 2 {
		t.Fatalf("entry.Line = %d, want 2", entry.Line)
	}
	if rec := f.mustParse(); len(rec.Reads) != 1 || rec.Reads[0] != entry {
		t.Fatalf("re-parse disagrees: %+v vs %+v", rec.Reads, entry)
	}
}

func TestRecordHeldOutRead_MalformedRecordRefused(t *testing.T) {
	f := newReadFixture(t)
	f.write("record.md", f.heldLine+"\nRead: not a read line\n")
	before := f.raw()
	_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err == nil {
		t.Fatalf("a malformed record was accepted")
	}
	if errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("a malformed record matched ErrNoHeldOutRecord")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err %q does not name line 2", err)
	}
	if f.raw() != before {
		t.Fatalf("file changed on a refusal")
	}
}

func TestRecordHeldOutRead_OtherSplitReadDoesNotBlock(t *testing.T) {
	f := newReadFixture(t)
	other := shaOf("split-b")
	otherRead := "Read: " + other + " split post-cutoff prereg H3 " + shaOf("prereg-h3") + " run run-000 commit abc1234 at 2026-10-25T00:00:00Z"
	f.write("record.md", f.heldLine+"\nHeld-out split sha256: "+other+" post-cutoff (3 tasks)\n"+otherRead+"\n")
	entry, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err != nil {
		t.Fatalf("a read of another split blocked this one: %v", err)
	}
	if entry.Line != 4 {
		t.Fatalf("entry.Line = %d, want 4", entry.Line)
	}
}

func TestRecordHeldOutRead_RecordIsADirectory(t *testing.T) {
	f := newReadFixture(t)
	if err := os.Mkdir(f.path(), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := RecordHeldOutRead(f.root, "record.md", f.validRead(), fixedNow)
	if err == nil || errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("err = %v, want a non-absent refusal", err)
	}
}
