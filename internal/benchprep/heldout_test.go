package benchprep

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func TestSplitSHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("a\nb\n"))
	want := hex.EncodeToString(sum[:])

	t.Run("known vector", func(t *testing.T) {
		got, err := SplitSHA256([]string{"a", "b"})
		if err != nil {
			t.Fatalf("SplitSHA256: %v", err)
		}
		if got != want {
			t.Fatalf("SplitSHA256 = %s, want %s", got, want)
		}
		if len(got) != 64 || got != strings.ToLower(got) {
			t.Fatalf("SplitSHA256 = %q, want 64 lowercase hex chars", got)
		}
	})

	t.Run("order stable", func(t *testing.T) {
		got, err := SplitSHA256([]string{"b", "a"})
		if err != nil {
			t.Fatalf("SplitSHA256: %v", err)
		}
		if got != want {
			t.Fatalf("SplitSHA256([b a]) = %s, want %s", got, want)
		}
	})

	t.Run("byte order not locale order", func(t *testing.T) {
		sum := sha256.Sum256([]byte("B\na\n"))
		got, err := SplitSHA256([]string{"a", "B"})
		if err != nil {
			t.Fatalf("SplitSHA256: %v", err)
		}
		if got != hex.EncodeToString(sum[:]) {
			t.Fatalf("SplitSHA256 did not sort in byte order")
		}
	})

	t.Run("caller slice not mutated", func(t *testing.T) {
		in := []string{"c", "a", "b"}
		if _, err := SplitSHA256(in); err != nil {
			t.Fatalf("SplitSHA256: %v", err)
		}
		if !reflect.DeepEqual(in, []string{"c", "a", "b"}) {
			t.Fatalf("caller slice mutated: %v", in)
		}
	})

	refusals := []struct {
		name string
		ids  []string
		want string
	}{
		{"nil list", nil, "empty"},
		{"empty list", []string{}, "empty"},
		{"empty id", []string{"a", ""}, "empty id"},
		{"newline in id", []string{"a", "b\nc"}, "newline"},
		{"carriage return in id", []string{"a", "b\rc"}, "carriage return"},
		{"duplicate", []string{"a", "dup-1", "b", "dup-1"}, "dup-1"},
	}
	for _, tc := range refusals {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			got, err := SplitSHA256(tc.ids)
			if err == nil {
				t.Fatalf("SplitSHA256(%q) = %s, want an error", tc.ids, got)
			}
			if got != "" {
				t.Fatalf("SplitSHA256 returned %q alongside an error", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// homeDevSetPath is the repo-relative path of the home corpus the committed
// record declares a dev set.
const homeDevSetPath = "testdata/contextbench/tasks-home.json"

func homeIDs(t *testing.T) []string {
	t.Helper()
	tasks, err := contextbench.LoadTasks(filepath.Join("..", "..", filepath.FromSlash(homeDevSetPath)))
	if err != nil {
		t.Fatalf("LoadTasks home corpus: %v", err)
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

// TestHeldOutRecord_HomeCorpusIsDevSet pins the committed record to the home
// corpus: exactly one Dev line names it, it carries the corpus's split sha,
// and the sha covers all 13 tasks. A 14th task fails this gate until someone
// updates the line on purpose.
func TestHeldOutRecord_HomeCorpusIsDevSet(t *testing.T) {
	ids := homeIDs(t)
	if len(ids) != 13 {
		t.Fatalf("home corpus has %d tasks, want 13 (update docs/bench/heldout-reads.md on purpose)", len(ids))
	}
	sha, err := SplitSHA256(ids)
	if err != nil {
		t.Fatalf("SplitSHA256: %v", err)
	}
	rec, err := ParseHeldOutRecord("../..", "docs/bench/heldout-reads.md")
	if err != nil {
		t.Fatalf("ParseHeldOutRecord: %v", err)
	}
	var matches []DevSplit
	for _, d := range rec.Dev {
		if d.Path == homeDevSetPath {
			matches = append(matches, d)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("%d Dev lines name %s, want exactly 1", len(matches), homeDevSetPath)
	}
	d := matches[0]
	if d.SHA256 != sha {
		t.Fatalf("Dev sha = %s, want %s", d.SHA256, sha)
	}
	if d.Tasks != 13 {
		t.Fatalf("Dev Tasks = %d, want 13", d.Tasks)
	}
	want := append([]string(nil), ids...)
	sort.Strings(want)
	if !reflect.DeepEqual(d.IDs, want) {
		t.Fatalf("Dev IDs = %v, want %v", d.IDs, want)
	}
}

// TestHeldOutRecord_NoHeldOutLineCommitted pins that the home corpus is never
// a held-out split and that the committed file carries no Held-out line at
// all: BX-11's oracle is that only the controller writes one.
func TestHeldOutRecord_NoHeldOutLineCommitted(t *testing.T) {
	sha, err := SplitSHA256(homeIDs(t))
	if err != nil {
		t.Fatalf("SplitSHA256: %v", err)
	}
	rec, err := ParseHeldOutRecord("../..", "docs/bench/heldout-reads.md")
	if err != nil {
		t.Fatalf("ParseHeldOutRecord: %v", err)
	}
	for _, h := range rec.HeldOut {
		if h.SHA256 == sha {
			t.Fatalf("the home corpus sha %s is on a Held-out line (line %d)", sha, h.Line)
		}
	}
	raw, err := os.ReadFile("../../docs/bench/heldout-reads.md")
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	if loc := regexp.MustCompile(`(?m)^Held-out split sha256: [0-9a-f]{64}`).FindIndex(raw); loc != nil {
		t.Fatalf("committed record carries a Held-out line at byte %d; only the controller writes it (BX-11)", loc[0])
	}
}

// heldoutFixture builds a record fixture in a temp root.
type heldoutFixture struct {
	t    *testing.T
	root string
}

func newHeldoutFixture(t *testing.T) *heldoutFixture {
	t.Helper()
	return &heldoutFixture{t: t, root: t.TempDir()}
}

// tasks writes a tiny valid contextbench tasks file at rel and returns the
// split sha of its ids.
func (f *heldoutFixture) tasks(rel string, ids ...string) string {
	f.t.Helper()
	type task struct {
		ID          string   `json:"id"`
		Prompt      string   `json:"prompt"`
		GroundTruth []string `json:"ground_truth"`
		ScoringRule string   `json:"scoring_rule"`
	}
	var doc struct {
		Version int    `json:"version"`
		Tasks   []task `json:"tasks"`
	}
	doc.Version = 1
	for _, id := range ids {
		doc.Tasks = append(doc.Tasks, task{ID: id, Prompt: "p " + id, GroundTruth: []string{"x"}, ScoringRule: "all-of"})
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		f.t.Fatalf("marshal: %v", err)
	}
	f.write(rel, string(raw))
	sha, err := SplitSHA256(ids)
	if err != nil {
		f.t.Fatalf("SplitSHA256: %v", err)
	}
	return sha
}

func (f *heldoutFixture) write(rel, text string) {
	f.t.Helper()
	p := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", rel, err)
	}
}

// record writes the record file and parses it.
func (f *heldoutFixture) record(lines ...string) (HeldOutRecord, error) {
	f.t.Helper()
	f.write("record.md", strings.Join(lines, "\n")+"\n")
	return ParseHeldOutRecord(f.root, "record.md")
}

func shaOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestParseHeldOutRecord_AbsentRecord(t *testing.T) {
	root := t.TempDir()
	_, err := ParseHeldOutRecord(root, "nope.md")
	if !errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("absent relative record: err = %v, want ErrNoHeldOutRecord", err)
	}
	_, err = ParseHeldOutRecord("/does/not/matter", filepath.Join(root, "nope.md"))
	if !errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("absent absolute record: err = %v, want ErrNoHeldOutRecord", err)
	}
}

func TestParseHeldOutRecord_UnreadableRecordIsNotAbsent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 000 does not make a file unreadable")
	}
	f := newHeldoutFixture(t)
	f.write("record.md", "# nothing\n")
	p := filepath.Join(f.root, "record.md")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	_, err := ParseHeldOutRecord(f.root, "record.md")
	if err == nil {
		t.Fatalf("unreadable record parsed without error")
	}
	if errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("unreadable record reported as absent (I38): %v", err)
	}
}

func TestParseHeldOutRecord_RecordPathIsADirectoryIsNotAbsent(t *testing.T) {
	f := newHeldoutFixture(t)
	if err := os.MkdirAll(filepath.Join(f.root, "record.md"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := ParseHeldOutRecord(f.root, "record.md")
	if err == nil || errors.Is(err, ErrNoHeldOutRecord) {
		t.Fatalf("err = %v, want a non-absent read error", err)
	}
}

func TestParseHeldOutRecord_MalformedLines(t *testing.T) {
	f := newHeldoutFixture(t)
	devSHA := f.tasks("dev.json", "d1", "d2")
	hSHA := shaOf("held")
	cases := []struct {
		name string
		line string
	}{
		{"dev short sha", "Dev split sha256: abc dev.json (2 tasks)"},
		{"dev uppercase sha", "Dev split sha256: " + strings.ToUpper(devSHA) + " dev.json (2 tasks)"},
		{"dev missing tasks", "Dev split sha256: " + devSHA + " dev.json"},
		{"dev double space", "Dev split sha256: " + devSHA + "  dev.json (2 tasks)"},
		{"dev trailing text", "Dev split sha256: " + devSHA + " dev.json (2 tasks) extra"},
		{"heldout bad split", "Held-out split sha256: " + hSHA + " dev (3 tasks)"},
		{"heldout no count", "Held-out split sha256: " + hSHA + " held-out"},
		{"heldout short sha", "Held-out split sha256: abc held-out (3 tasks)"},
		{"read missing run", "Read: " + hSHA + " split held-out prereg P1 " + hSHA + " commit abcdef1 at 2026-10-26T00:00:00Z"},
		{"read short commit", "Read: " + hSHA + " split held-out prereg P1 " + hSHA + " run r1 commit abc123 at 2026-10-26T00:00:00Z"},
		{"read uppercase commit", "Read: " + hSHA + " split held-out prereg P1 " + hSHA + " run r1 commit ABCDEF1 at 2026-10-26T00:00:00Z"},
		{"read offset time", "Read: " + hSHA + " split held-out prereg P1 " + hSHA + " run r1 commit abcdef1 at 2026-10-26T00:00:00+02:00"},
		{"read bad time", "Read: " + hSHA + " split held-out prereg P1 " + hSHA + " run r1 commit abcdef1 at yesterday"},
		{"read no space after colon", "Read: " + hSHA},
		{"demoted short sha", "Demoted: abc because"},
		{"demoted no reason", "Demoted: " + hSHA},
		{"demoted blank reason", "Demoted: " + hSHA + "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.record("# prose", "", "Dev split sha256: "+devSHA+" dev.json (2 tasks)", tc.line)
			if err == nil {
				t.Fatalf("malformed line parsed: %q", tc.line)
			}
			if !strings.Contains(err.Error(), "line 4") {
				t.Fatalf("error %q does not name line 4", err)
			}
			if !strings.Contains(err.Error(), tc.line) && !strings.Contains(err.Error(), strings.TrimRight(tc.line, " ")) {
				t.Fatalf("error %q does not quote the line", err)
			}
		})
	}
}

func TestParseHeldOutRecord_ProsePasses(t *testing.T) {
	f := newHeldoutFixture(t)
	sha := f.tasks("dev.json", "d1")
	rec, err := f.record(
		"# Held-out reads",
		"",
		"- `Dev split sha256: <hex> <repo-relative path> (<n> tasks)`: prose bullet",
		"  Dev split sha256: indented so prose",
		"Dev split sha256:no-space-after-colon is prose",
		" Held-out split sha256: leading space is prose",
		"- Read: <split sha256> split <name> prereg <id> <prereg sha256> run <run id> commit <sha> at <RFC3339 UTC>",
		"> Demoted: <split sha256> <reason>",
		"`Held-out split sha256: <hex> <split> (<n> tasks)`",
		"read: lower-case prefix is prose",
		"Reads are appended at the end.",
		"Dev split sha256: "+sha+" dev.json (1 tasks)",
	)
	if err != nil {
		t.Fatalf("ParseHeldOutRecord: %v", err)
	}
	if len(rec.Dev) != 1 || rec.Dev[0].Line != 12 || len(rec.HeldOut)+len(rec.Reads)+len(rec.Demoted) != 0 {
		t.Fatalf("rec = %+v, want exactly the one Dev line at line 12", rec)
	}
}

func TestParseHeldOutRecord_CarriageReturnsStripped(t *testing.T) {
	f := newHeldoutFixture(t)
	sha := f.tasks("dev.json", "d1")
	f.write("record.md", "# h\r\nDev split sha256: "+sha+" dev.json (1 tasks)\r\n")
	rec, err := ParseHeldOutRecord(f.root, "record.md")
	if err != nil {
		t.Fatalf("ParseHeldOutRecord: %v", err)
	}
	if len(rec.Dev) != 1 || rec.Dev[0].Line != 2 {
		t.Fatalf("rec = %+v", rec)
	}
}

func TestParseHeldOutRecord_DevRefusals(t *testing.T) {
	f := newHeldoutFixture(t)
	sha := f.tasks("dev.json", "d1", "d2")
	f.write("dup.json", `{"version":1,"tasks":[`+
		`{"id":"x","prompt":"p","ground_truth":["g"],"scoring_rule":"all-of"},`+
		`{"id":"x","prompt":"p","ground_truth":["g"],"scoring_rule":"all-of"}]}`)
	f.write("bad.json", "{not json")
	if err := os.MkdirAll(filepath.Join(f.root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	t.Run("missing dev file is not absence", func(t *testing.T) {
		_, err := f.record("Dev split sha256: " + sha + " missing.json (2 tasks)")
		if err == nil {
			t.Fatalf("missing dev file parsed")
		}
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNoHeldOutRecord) {
			t.Fatalf("missing dev file satisfies an absence check: %v", err)
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("error %q does not name line 1", err)
		}
	})
	t.Run("unparseable dev file", func(t *testing.T) {
		_, err := f.record("Dev split sha256: " + sha + " bad.json (2 tasks)")
		if err == nil || errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNoHeldOutRecord) {
			t.Fatalf("err = %v, want a plain refusal", err)
		}
	})
	t.Run("sha drift", func(t *testing.T) {
		other := shaOf("other")
		_, err := f.record("# h", "Dev split sha256: "+other+" dev.json (2 tasks)")
		if err == nil {
			t.Fatalf("drifted dev sha parsed")
		}
		for _, want := range []string{"line 2", other, sha} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not mention %q", err, want)
			}
		}
	})
	t.Run("count mismatch", func(t *testing.T) {
		_, err := f.record("Dev split sha256: " + sha + " dev.json (3 tasks)")
		if err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("err = %v, want a count refusal naming line 1", err)
		}
	})
	t.Run("duplicate task id in the dev file", func(t *testing.T) {
		_, err := f.record("Dev split sha256: " + shaOf("x\nx\n") + " dup.json (2 tasks)")
		if err == nil || !strings.Contains(err.Error(), "x") {
			t.Fatalf("err = %v, want a duplicate-id refusal", err)
		}
	})
	t.Run("absolute path", func(t *testing.T) {
		abs := filepath.ToSlash(filepath.Join(f.root, "dev.json"))
		_, err := f.record("Dev split sha256: " + sha + " " + abs + " (2 tasks)")
		if err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("err = %v, want an absolute-path refusal", err)
		}
	})
	t.Run("dotdot path", func(t *testing.T) {
		_, err := f.record("Dev split sha256: " + sha + " sub/../dev.json (2 tasks)")
		if err == nil {
			t.Fatalf("path with .. element parsed")
		}
		_, err = f.record("Dev split sha256: " + sha + " ../dev.json (2 tasks)")
		if err == nil {
			t.Fatalf("path escaping root parsed")
		}
	})
	t.Run("non-clean path", func(t *testing.T) {
		for _, p := range []string{"./dev.json", "sub//../dev.json", "dev.json/", "sub/./x.json"} {
			if _, err := f.record("Dev split sha256: " + sha + " " + p + " (2 tasks)"); err == nil {
				t.Fatalf("non-clean path %q parsed", p)
			}
		}
	})
	t.Run("duplicate dev sha", func(t *testing.T) {
		f.tasks("copy.json", "d1", "d2")
		_, err := f.record(
			"Dev split sha256: "+sha+" dev.json (2 tasks)",
			"Dev split sha256: "+sha+" copy.json (2 tasks)")
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want a duplicate Dev sha refusal naming line 2", err)
		}
	})
}

func TestParseHeldOutRecord_CrossChecks(t *testing.T) {
	f := newHeldoutFixture(t)
	devSHA := f.tasks("dev.json", "d1", "d2")
	h1, h2 := shaOf("h1"), shaOf("h2")
	read := func(sha, split string) string {
		return "Read: " + sha + " split " + split + " prereg P1 " + shaOf("prereg") + " run r1 commit abcdef1 at 2026-10-26T00:00:00Z"
	}
	dev := "Dev split sha256: " + devSHA + " dev.json (2 tasks)"

	t.Run("duplicate held-out sha", func(t *testing.T) {
		_, err := f.record(
			"Held-out split sha256: "+h1+" held-out (3 tasks)",
			"Held-out split sha256: "+h1+" post-cutoff (3 tasks)")
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want a duplicate Held-out sha refusal naming line 2", err)
		}
	})
	t.Run("sha on both dev and held-out", func(t *testing.T) {
		_, err := f.record(dev, "Held-out split sha256: "+devSHA+" held-out (2 tasks)")
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want a Dev/Held-out overlap refusal", err)
		}
	})
	t.Run("read with unknown split sha", func(t *testing.T) {
		_, err := f.record(
			"Held-out split sha256: "+h1+" held-out (3 tasks)",
			read(h2, "held-out"))
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want an unknown-split refusal naming line 2", err)
		}
	})
	t.Run("read of a dev sha", func(t *testing.T) {
		_, err := f.record(dev, read(devSHA, "held-out"))
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want a refusal: a Dev sha has no Held-out line", err)
		}
	})
	t.Run("read split name mismatch", func(t *testing.T) {
		_, err := f.record(
			"Held-out split sha256: "+h1+" held-out (3 tasks)",
			read(h1, "post-cutoff"))
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want a split-name mismatch refusal naming line 2", err)
		}
	})
	t.Run("demoted with unknown split sha", func(t *testing.T) {
		_, err := f.record(
			"Held-out split sha256: "+h1+" held-out (3 tasks)",
			"Demoted: "+h2+" leaked")
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("err = %v, want an unknown-split refusal naming line 2", err)
		}
	})
	t.Run("read line before its held-out line is fine", func(t *testing.T) {
		if _, err := f.record(read(h1, "held-out"), "Held-out split sha256: "+h1+" held-out (3 tasks)"); err != nil {
			t.Fatalf("cross-checks run after the whole file is read: %v", err)
		}
	})
}

func TestParseHeldOutRecord_FullRecord(t *testing.T) {
	f := newHeldoutFixture(t)
	devSHA := f.tasks("sets/dev.json", "d2", "d1", "d3")
	h1, h2 := shaOf("h1"), shaOf("h2")
	preSHA := shaOf("prereg")
	rec, err := f.record(
		"# Held-out reads",
		"",
		"Dev split sha256: "+devSHA+" sets/dev.json (3 tasks)",
		"prose between",
		"Held-out split sha256: "+h1+" held-out (4 tasks)",
		"Held-out split sha256: "+h2+" post-cutoff (5 tasks)",
		"Read: "+h1+" split held-out prereg P-1 "+preSHA+" run run-1 commit abcdef1 at 2026-10-26T01:02:03Z",
		"Read: "+h2+" split post-cutoff prereg P-2 "+preSHA+" run run-2 commit "+strings.Repeat("a", 40)+" at 2026-10-27T04:05:06Z",
		"Demoted: "+h1+" the split leaked into a prompt",
	)
	if err != nil {
		t.Fatalf("ParseHeldOutRecord: %v", err)
	}
	want := HeldOutRecord{
		Dev: []DevSplit{{SHA256: devSHA, Path: "sets/dev.json", Tasks: 3, IDs: []string{"d1", "d2", "d3"}, Line: 3}},
		HeldOut: []HeldOutSplit{
			{SHA256: h1, Split: "held-out", Tasks: 4, Line: 5},
			{SHA256: h2, Split: "post-cutoff", Tasks: 5, Line: 6},
		},
		Reads: []HeldOutReadEntry{
			{SplitSHA256: h1, Split: "held-out", PreregID: "P-1", PreregSHA256: preSHA, RunID: "run-1", Commit: "abcdef1",
				At: time.Date(2026, 10, 26, 1, 2, 3, 0, time.UTC), Line: 7,
				Text: "Read: " + h1 + " split held-out prereg P-1 " + preSHA + " run run-1 commit abcdef1 at 2026-10-26T01:02:03Z"},
			{SplitSHA256: h2, Split: "post-cutoff", PreregID: "P-2", PreregSHA256: preSHA, RunID: "run-2", Commit: strings.Repeat("a", 40),
				At: time.Date(2026, 10, 27, 4, 5, 6, 0, time.UTC), Line: 8,
				Text: "Read: " + h2 + " split post-cutoff prereg P-2 " + preSHA + " run run-2 commit " + strings.Repeat("a", 40) + " at 2026-10-27T04:05:06Z"},
		},
		Demoted: []SplitDemotion{{SplitSHA256: h1, Reason: "the split leaked into a prompt", Line: 9,
			Text: "Demoted: " + h1 + " the split leaked into a prompt"}},
	}
	if !reflect.DeepEqual(rec, want) {
		t.Fatalf("record mismatch\n got: %+v\nwant: %+v", rec, want)
	}
}

func TestParseHeldOutRecord_PrefixConsts(t *testing.T) {
	for name, got := range map[string]string{
		"Dev split sha256: ":      PrefixDevSplit,
		"Held-out split sha256: ": PrefixHeldOutSplit,
		"Read: ":                  PrefixHeldOutRead,
		"Demoted: ":               PrefixDemoted,
	} {
		if got != name {
			t.Fatalf("prefix const = %q, want %q", got, name)
		}
	}
}
