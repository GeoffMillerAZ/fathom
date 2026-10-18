package main

// readoutreport_baseline.go -- `fathom readout report --baseline`: the share
// of the test-run hook's failing-run rows whose named file or test is
// touched by one of the next three calls. It is the baseline a rich
// readout's acted-on share is read against, computed the same way: the
// rows' failing ids name the tests, their packages' directories name the
// files. Rows carry no command or output text, so it finds each row's call
// in the local transcripts (by the row's tool-use id, or for a row without
// one by its session, runner, scope and time), reads the three calls after
// it, and counts apart every row it cannot place (I38). Aggregates only.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	"github.com/geoffmilleraz/fathom/internal/testrun"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

const (
	blFollowers = 3                // "within three calls"
	blMatchSkew = 30 * time.Second // a row without an id matches a result this close to its time
	blAmbiguous = time.Second      // a second candidate this close to the best makes the match ambiguous
	blMaxCmd    = 8 << 10          // a longer command is no test invocation
	blMaxWords  = 64
)

// roBaseline is the baseline's JSON form.
type roBaseline struct {
	Window        roReportWindow  `json:"window"`
	Rows          int             `json:"rows"`
	Located       int             `json:"located"`
	NotComputable roBaselineNoCmp `json:"not_computable"`
	Touched       int             `json:"touched"`
	Share         float64         `json:"share"`
	Lower         float64         `json:"lower"`
	Upper         float64         `json:"upper"`
}

type roBaselineNoCmp struct {
	Total          int `json:"total"`
	TranscriptGone int `json:"transcript_gone"`
	IDAbsent       int `json:"tool_use_id_absent"`
	NamesNothing   int `json:"names_nothing"`
}

// blRow is one failing-run row of the test-run hook.
type blRow struct {
	at         time.Time
	session    string
	toolUseID  string
	invocation string
	scope      string
	failing    []string
}

// blCall is one Bash call that may be a row's run, with the next calls.
type blCall struct {
	path      string
	id        string
	idx       int
	command   string
	resultAt  time.Time
	followers []*blFollower
}

// blFollower is a later call: the files it reads or edits and the words of
// its command.
type blFollower struct {
	files []string
	words []string
}

func runReadoutBaseline(sinceFlag string, since time.Time, jsonOut bool, stdout io.Writer) int {
	rep := readoutBaselineReport(beatLogPath(), readoutProjectsDir(), since)
	rep.Window.Since = sinceFlag
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 2
		}
		return 0
	}
	printReadoutBaseline(stdout, rep)
	return 0
}

// readoutBaselineReport reads the failing-run rows of both hook-log
// generations at or after since and counts those touched within three calls.
func readoutBaselineReport(logPath, projects string, since time.Time) roBaseline {
	rows, oldest := blReadRows(logPath, since)
	rep := roBaseline{}
	if !since.IsZero() && (oldest.IsZero() || oldest.After(since)) {
		rep.Window.Incomplete = true
		if !oldest.IsZero() {
			rep.Window.CoveredFrom = oldest.UTC().Format(time.RFC3339)
		}
	}
	if len(rows) == 0 {
		return rep
	}
	from, to := rows[0].at, rows[0].at
	for _, r := range rows {
		if r.at.Before(from) {
			from = r.at
		}
		if r.at.After(to) {
			to = r.at
		}
	}
	rep.Window.From, rep.Window.To = from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)
	rep.Window.Days = math.Round(to.Sub(from).Hours()/24*10) / 10
	rep.Rows = len(rows)

	wantID := map[string]bool{}
	var invs [][]string
	seenInv := map[string]bool{}
	for _, r := range rows {
		if r.toolUseID != "" {
			wantID[r.toolUseID] = true
		}
		if !seenInv[r.invocation] && strings.TrimSpace(r.invocation) != "" {
			seenInv[r.invocation] = true
			invs = append(invs, strings.Fields(r.invocation))
		}
	}
	var calls []*blCall
	if projects != "" {
		calls = blReadCalls(projects, from, wantID, invs)
	}
	byID := map[string]*blCall{}
	for _, c := range calls {
		if c.id != "" && wantID[c.id] {
			byID[c.id] = c
		}
	}
	for _, r := range rows {
		targets := blTargets(r.failing)
		if len(targets) == 0 {
			rep.NotComputable.NamesNothing++
			continue
		}
		var call *blCall
		if r.toolUseID != "" {
			if call = byID[r.toolUseID]; call == nil {
				rep.NotComputable.TranscriptGone++
				continue
			}
		} else if call = blMatch(calls, r); call == nil {
			rep.NotComputable.IDAbsent++
			continue
		}
		rep.Located++
		for _, f := range call.followers {
			if blTouches(f, targets) {
				rep.Touched++
				break
			}
		}
	}
	nc := &rep.NotComputable
	nc.Total = nc.TranscriptGone + nc.IDAbsent + nc.NamesNothing
	rep.Share = roRound(shareOf(int64(rep.Touched), int64(rep.Located)))
	lo, hi := ask.Wilson(rep.Touched, rep.Located)
	rep.Lower, rep.Upper = roRound(lo), roRound(hi)
	return rep
}

// blReadRows streams both hook-log generations, the rotated one first,
// keeping the test-run hook's failing-run rows at or after since; oldest is
// the first beat on any channel.
func blReadRows(logPath string, since time.Time) (rows []blRow, oldest time.Time) {
	if logPath == "" {
		return nil, time.Time{}
	}
	marker := []byte(`"channel":"` + testRunChannel + `"`)
	for _, p := range []string{logPath + ".1", logPath} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		br := bufio.NewReaderSize(f, 1<<20)
		for {
			line, rerr := br.ReadBytes('\n')
			if len(bytes.TrimSpace(line)) > 0 {
				var b heartbeat.Beat
				if json.Unmarshal(line, &b) == nil {
					if oldest.IsZero() {
						oldest = b.At
					}
					if r, ok := blRowOf(b, marker, line, since); ok {
						rows = append(rows, r)
					}
				}
			}
			if rerr != nil {
				break
			}
		}
		f.Close()
	}
	return rows, oldest
}

func blRowOf(b heartbeat.Beat, marker, line []byte, since time.Time) (blRow, bool) {
	if !bytes.Contains(line, marker) || b.Channel != testRunChannel {
		return blRow{}, false
	}
	if b.Kind != string(testrun.Fail) && b.Meta["result"] != string(testrun.Fail) {
		return blRow{}, false
	}
	if !since.IsZero() && b.At.Before(since) {
		return blRow{}, false
	}
	m := b.Meta
	r := blRow{at: b.At, session: m["session"], toolUseID: m["tool_use_id"], invocation: m["invocation"], scope: m["scope"]}
	if m["failing"] != "" {
		r.failing = strings.Split(m["failing"], ",")
	}
	return r, true
}

// blReadCalls streams the transcripts under root modified at or after from
// and returns every Bash call that is a wanted id or runs a row's
// invocation, with its result time and the three calls after it.
func blReadCalls(root string, from time.Time, wantID map[string]bool, invs [][]string) []*blCall {
	var out []*blCall
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		if info, ierr := d.Info(); ierr != nil || info.ModTime().Before(from) {
			return nil
		}
		out = append(out, blReadFile(p, wantID, invs)...)
		return nil
	})
	return out
}

func blReadFile(p string, wantID map[string]bool, invs [][]string) []*blCall {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	var all, active []*blCall
	byID := map[string]*blCall{}
	seq := 0
	br := bufio.NewReaderSize(f, 1<<20)
	var buf []byte
	over := false
	for {
		chunk, rerr := br.ReadSlice('\n')
		if len(buf)+len(chunk) <= replayLineMax {
			buf = append(buf, chunk...)
		} else {
			over = true
		}
		if errors.Is(rerr, bufio.ErrBufferFull) {
			continue
		}
		if !over && bytes.Contains(buf, []byte(`"tool_`)) {
			seq = blLine(buf, p, seq, wantID, invs, &all, &active, byID)
		}
		buf, over = buf[:0], false
		if rerr != nil {
			break
		}
	}
	kept := all[:0]
	for _, c := range all {
		if !c.resultAt.IsZero() {
			kept = append(kept, c)
		}
	}
	return kept
}

func blLine(raw []byte, p string, seq int, wantID map[string]bool, invs [][]string, all, active *[]*blCall, byID map[string]*blCall) int {
	var l replayTranscriptLine
	if json.Unmarshal(raw, &l) != nil || l.Message == nil || len(l.Message.Content) == 0 || l.Message.Content[0] != '[' {
		return seq
	}
	var blocks []replayBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return seq
	}
	for i := range blocks {
		b := &blocks[i]
		switch b.Type {
		case "tool_use":
			for len(*active) > 0 && seq > (*active)[0].idx+blFollowers {
				*active = (*active)[1:]
			}
			if len(*active) > 0 {
				fl := blFollowerOf(b)
				for _, c := range *active {
					if seq > c.idx && seq <= c.idx+blFollowers {
						c.followers = append(c.followers, fl)
					}
				}
			}
			if b.Name == "Bash" {
				var in struct {
					Command string `json:"command"`
				}
				if json.Unmarshal(b.Input, &in) == nil && (wantID[b.ID] || blRunsInvocation(in.Command, invs)) {
					c := &blCall{path: p, id: b.ID, idx: seq, command: in.Command}
					*all = append(*all, c)
					*active = append(*active, c)
					byID[b.ID] = c
				}
			}
			seq++
		case "tool_result":
			if c := byID[b.ToolUseID]; c != nil && c.resultAt.IsZero() {
				if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
					c.resultAt = ts
				}
			}
		}
	}
	return seq
}

// blRunsInvocation: the command lexes to a simple command that is one of
// the rows' declared invocations.
func blRunsInvocation(command string, invs [][]string) bool {
	if len(command) > blMaxCmd {
		return false
	}
	segs, unreadable := lexShellSegments(command)
	if unreadable {
		return false
	}
	for _, s := range segs {
		words := stripCommandWrappers(s.words)
		for _, want := range invs {
			if invocationMatches(words, want) {
				return true
			}
		}
	}
	return false
}

// blFollowerOf reads a call as a toucher: a Read, Edit, Write or MultiEdit's
// file, a shell read of a file through the lexer, and a Bash command's words.
func blFollowerOf(b *replayBlock) *blFollower {
	fl := &blFollower{}
	switch b.Name {
	case "Read", "Edit", "Write", "MultiEdit":
		var in struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(b.Input, &in) == nil && in.FilePath != "" {
			fl.files = []string{in.FilePath}
		}
	case "Bash":
		var in struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(b.Input, &in) != nil || len(in.Command) > blMaxCmd {
			return fl
		}
		c := shellcmd.ParseCommand(in.Command)
		fl.files = append(wholeFileRead(&c), sedReadFiles(&c)...)
		segs, _ := lexShellSegments(in.Command)
		for _, s := range segs {
			for _, w := range s.words {
				if len(fl.words) < blMaxWords {
					fl.words = append(fl.words, w)
				}
			}
		}
	}
	return fl
}

// blMatch finds the call a row without a tool-use id ran: in a transcript
// whose path names the row's session, the call running the row's invocation
// (and, when the row has one, its scope) whose result is nearest the row's
// time, within blMatchSkew. A second call as near is ambiguous: nil.
func blMatch(calls []*blCall, r blRow) *blCall {
	if r.invocation == "" {
		return nil
	}
	var best *blCall
	var bestD, second time.Duration = blMatchSkew + 1, blMatchSkew + 1
	for _, c := range calls {
		if r.session != "" && !strings.Contains(c.path, r.session) {
			continue
		}
		d := c.resultAt.Sub(r.at)
		if d < 0 {
			d = -d
		}
		if d > blMatchSkew {
			continue
		}
		if r.scope != "" && testRunScope(c.command, r.invocation) != r.scope {
			continue
		}
		if !blRunsInvocation(c.command, [][]string{strings.Fields(r.invocation)}) {
			continue
		}
		switch {
		case d < bestD:
			best, second, bestD = c, bestD, d
		case d < second:
			second = d
		}
	}
	if best == nil || second-bestD < blAmbiguous {
		return nil
	}
	return best
}

// blTarget is what one failing id names: a package (whose directory holds
// the files) or a test file, and the top-level test name.
type blTarget struct {
	pkg, file, test string
}

func blTargets(ids []string) []blTarget {
	var out []blTarget
	for _, id := range ids {
		if id == "" {
			continue
		}
		var t blTarget
		if file, test, ok := strings.Cut(id, "::"); ok {
			t.file = file
			t.test = test[strings.LastIndex(test, "::")+1:]
			if i := strings.IndexAny(t.test, "[/"); i >= 0 {
				t.test = t.test[:i]
			}
			out = append(out, t)
			continue
		}
		i := blTestStart(id)
		if i > 0 {
			t.pkg = id[:i]
		}
		t.test = id[i+boolInt(i > 0):]
		if j := strings.IndexByte(t.test, '/'); j >= 0 {
			t.test = t.test[:j]
		}
		out = append(out, t)
	}
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// blTestStart is the index of the '.' joining a Go package path to its test
// name ("pkg.TestX"), or 0 when the id carries no package.
func blTestStart(id string) int {
	for i := 1; i < len(id); i++ {
		if id[i-1] != '.' && id[i] == '.' {
			rest := id[i+1:]
			for _, p := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
				if strings.HasPrefix(rest, p) {
					return i
				}
			}
		}
	}
	return 0
}

// blTouches: the call reads or edits a file of a target's package (or the
// target's test file), or a command word names the target's test.
func blTouches(f *blFollower, targets []blTarget) bool {
	for _, t := range targets {
		for _, p := range f.files {
			if t.pkg != "" && blPkgHoldsFile(t.pkg, p) {
				return true
			}
			if t.file != "" && roReadsAny([]string{p}, []string{t.file}) {
				return true
			}
		}
		if t.test != "" {
			for _, w := range f.words {
				if blNamesIdent(w, t.test) {
					return true
				}
			}
		}
	}
	return false
}

// blPkgHoldsFile: file's directory is the package's. The module prefix of an
// import path is unknown here, so the directory matches when the trailing
// path components agree, at least two (one for a one-component path).
func blPkgHoldsFile(pkg, file string) bool {
	if path.Ext(file) == "" {
		return false
	}
	pc := strings.Split(strings.Trim(pkg, "/"), "/")
	dc := strings.Split(strings.Trim(filepath.ToSlash(path.Dir(filepath.ToSlash(file))), "/"), "/")
	k := 0
	for k < len(pc) && k < len(dc) && pc[len(pc)-1-k] == dc[len(dc)-1-k] {
		k++
	}
	need := 2
	if len(pc) < need {
		need = len(pc)
	}
	return k >= need
}

// blNamesIdent: word holds name as a whole identifier.
func blNamesIdent(word, name string) bool {
	for off := 0; ; {
		i := strings.Index(word[off:], name)
		if i < 0 {
			return false
		}
		s, e := off+i, off+i+len(name)
		if (s == 0 || !blIdentByte(word[s-1])) && (e == len(word) || !blIdentByte(word[e])) {
			return true
		}
		off = s + 1
	}
}

func blIdentByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func printReadoutBaseline(w io.Writer, rep roBaseline) {
	win := rep.Window
	if win.From == "" {
		fmt.Fprintln(w, "readout baseline: no failing-run rows in the hook log")
	} else {
		fmt.Fprintf(w, "readout baseline: window %s to %s (%.1f days, from the first row)\n", win.From, win.To, win.Days)
	}
	if win.Since != "" {
		if win.Incomplete {
			reach := "holds no beats"
			if win.CoveredFrom != "" {
				reach = "reaches back only to " + win.CoveredFrom
			}
			fmt.Fprintf(w, "--since %s: INCOMPLETE, the hook log %s\n", win.Since, reach)
		} else {
			fmt.Fprintf(w, "--since %s: the hook log covers the window\n", win.Since)
		}
	}
	nc := rep.NotComputable
	fmt.Fprintf(w, "  rows %d, located %d\n", rep.Rows, rep.Located)
	fmt.Fprintf(w, "  not computable %d (transcript gone %d, tool-use id absent %d, names nothing %d), counted apart, never as untouched\n",
		nc.Total, nc.TranscriptGone, nc.IDAbsent, nc.NamesNothing)
	if rep.Located == 0 {
		fmt.Fprintln(w, "  share: no rows located")
		return
	}
	fmt.Fprintf(w, "  touched %d of %d located within three calls (%.4f; 95%% Wilson interval %.4f to %.4f)\n",
		rep.Touched, rep.Located, rep.Share, rep.Lower, rep.Upper)
}
