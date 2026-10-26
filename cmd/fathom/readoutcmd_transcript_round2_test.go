package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readout"
)

// Synthetic compatibility corpus: invalid non-tool JSON, an invalid marked
// line, invalid blocks, blanks and CRLF, and an object-valued assistant turn.
const replayTranscriptRound2 = "\n \t\r\ninvalid without marker\r\nbroken \"tool_use\"\r\n" +
	`{"type":"assistant","message":{"content":[{"type":"tool_use","id":42}]}}` + "\r\n" +
	`{"type":"assistant","sessionId":"session-example","cwd":"/home/example/work","timestamp":"2026-10-18T10:00:00Z","message":{"id":"m1","content":[{"type":"tool_use","id":"a","name":"Bash","input":{"command":"cat alpha.go"}}]}}` + "\r\n" +
	`{"type":"user","sessionId":"session-example","timestamp":"2026-10-18T10:00:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"a","content":"hello"}]}}` + "\r\n" +
	`{"type":"system","uuid":"boundary-example","subtype":"compact_boundary","timestamp":"2026-10-18T10:01:00Z"}` + "\r\n" +
	`{"type":"assistant","timestamp":"2026-10-18T10:02:00Z","message":{"id":"m2","content":{"note":"tool_use"}}}` + "\r\n" +
	`{"type":"assistant","sessionId":"session-example","cwd":"/home/example/work","timestamp":"2026-10-18T10:03:00Z","message":{"id":"m3","content":[{"type":"tool_use","id":"b","name":"Bash","input":{"command":"cat alpha.go"}}]}}` + "\r\n" +
	`{"type":"user","sessionId":"session-example","timestamp":"2026-10-18T10:03:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"b","content":"hello"}]}}` + "\r\n" +
	`{"type":"assistant","sessionId":"session-example","cwd":"/home/example/work","timestamp":"2026-10-18T10:04:00Z","message":{"id":"m4","content":[{"type":"tool_use","id":"c","name":"Bash","input":{"command":"head alpha.go"}}]}}` + "\r\n" +
	`{"type":"user","sessionId":"session-example","timestamp":"2026-10-18T10:04:01Z","message":{"content":[{"type":"tool_result","tool_use_id":"c","content":"hello"}]}}` + "\r\n"

func checkReplayTranscriptRound2(t *testing.T, rep replayReport) {
	t.Helper()
	if rep.Corpus.Files != 1 || rep.Corpus.Sessions != 1 || rep.Corpus.LinesUnparsed != 2 || rep.Corpus.LinesOversize != 0 {
		t.Errorf("corpus = %+v, want files 1, sessions 1, unparsed 2, oversize 0", rep.Corpus)
	}
	if rep.Bash.Calls != 3 || rep.Bash.Bytes != 15 {
		t.Errorf("Bash = %+v, want calls 3, bytes 15", rep.Bash)
	}
	d := rep.Dedupe
	if d.Bash.Results != 3 || d.Bash.Bytes != 15 || d.Bytes != 15 || d.Repeat.Results != 1 || d.Repeat.Bytes != 5 || d.AcrossTurns.Results != 1 || d.WithinBatch.Results != 0 {
		t.Errorf("dedupe = %+v", d)
	}
	c := rep.Compactions
	if c.Events != 1 || len(c.PerEvent) != 1 {
		t.Fatalf("compactions = %+v", c)
	}
	if row := c.PerEvent[0]; row.PreSet != 1 || row.Reread != 1 || row.PostTurns != 3 || row.Complete {
		t.Errorf("compaction = %+v, want pre/reread 1/1, post turns 3, incomplete", row)
	}
}

func TestCmdReadoutReplayTranscriptCompatibilityCounts(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "projects")
	p := filepath.Join(root, "project-example", "session-example.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(replayTranscriptRound2), 0600); err != nil {
		t.Fatal(err)
	}
	rules := writeReadoutRules(t, dir, readoutTestRules)
	useReadoutReplayEnv(t, root, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	var out, errb bytes.Buffer
	if code := cmdReadoutReplay([]string{"--json", "--rules", rules}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, &errb)
	}
	var rep replayReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	checkReplayTranscriptRound2(t, rep)
}

func TestReplaySizingObjectContentStillCountsTurn(t *testing.T) {
	z := newReplaySizing(30, time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC))
	st := z.open("session-example.jsonl")
	for _, line := range strings.Split(replayTranscriptRound2, "\n") {
		z.line(st, []byte(line))
	}
	if len(st.ring) != 5 {
		t.Errorf("turns = %d, want 5 including invalid-block and object-content assistant messages", len(st.ring))
	}
	if st.open == nil || st.open.post != 3 {
		t.Errorf("post turns = %+v, want 3", st.open)
	}
}

type replayTranscriptReadError struct{}

func (replayTranscriptReadError) Read([]byte) (int, error) {
	return 0, errors.New("synthetic read failure")
}

func TestReplayTranscriptReadErrorKeepsPartialCounts(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	a := newReplayAgg(&readout.RuleSet{}, 30, now.Add(-30*24*time.Hour))
	pending := `{"type":"assistant","message":{"id":"m4","content":[{"type":"tool_use","id":"pending","name":"Bash","input":{"command":"echo pending"}}]}}`
	r := io.MultiReader(strings.NewReader(replayTranscriptRound2+pending), replayTranscriptReadError{})
	if !a.readFile("session-example.jsonl", r) {
		t.Error("partial transcript lost counted calls")
	}
	a.sizing.report(&a.rep)
	a.rep.Corpus.Sessions = len(a.sessions)
	checkReplayTranscriptRound2(t, a.rep)
	if a.rep.Bash.Unpaired != 1 {
		t.Errorf("unpaired = %d, want partial tail's 1", a.rep.Bash.Unpaired)
	}
	// An opened directory has a read error, while an open failure counts no file.
	empty := newReplayAgg(&readout.RuleSet{}, 30, now)
	dir := t.TempDir()
	empty.file(filepath.Join(dir, "missing.jsonl"))
	if empty.rep.Corpus.Files != 0 {
		t.Error("open failure counted as a file")
	}
	empty.file(dir)
	if empty.rep.Corpus.Files != 1 {
		t.Errorf("opened unreadable file count = %d, want 1", empty.rep.Corpus.Files)
	}
}
