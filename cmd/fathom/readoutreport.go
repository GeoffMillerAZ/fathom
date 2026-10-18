package main

// readoutreport.go — `fathom readout report`: the readout hook's rows joined
// to harm, per rule (design docs/design/readouts.md §10, ADR-071 decision
// 9). Rows carry no output text, so the report finds each row's tool-use id
// in the local transcripts, re-renders the call with the rule its `rule_sha`
// names, recomputes the dropped lines and matches the next five calls
// against them, the way replay does. A row it cannot replay (no transcript,
// a rule whose text changed, a call that renders differently now) is
// counted apart, never as clean (I38). Per rule only: no totals across
// rules, no per-person or per-session view, never command or output text.

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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// roDemoteLower is decision 9's demotion bar: a lean recovery rate whose 95%
// Wilson lower bound exceeds it. Promotion reuses replay's eligibility bar
// (replayMinEvaluated, replayMinCleanLower), the same R3 pruning bar.
const roDemoteLower = 0.05

const (
	roNoLeanRows = "no lean rows"
	roVerdictPro = "promote"
	roVerdictKp  = "keep"
	roVerdictDem = "demote"
	roVerdictIns = "insufficient"
)

// roReport is the report's JSON form.
type roReport struct {
	Window roReportWindow `json:"window"`
	Rules  []roReportRule `json:"rules"`
}

// roReportWindow is the span the rows cover, asserted from the first row.
// With --since, incomplete when the hook log reaches back only to
// covered_from, after the window's start.
type roReportWindow struct {
	From        string  `json:"from,omitempty"`
	To          string  `json:"to,omitempty"`
	Days        float64 `json:"days"`
	Since       string  `json:"since,omitempty"`
	Incomplete  bool    `json:"incomplete"`
	CoveredFrom string  `json:"covered_from,omitempty"`
}

// roReportRule is one rule's reading.
type roReportRule struct {
	Rule          string          `json:"rule"`
	Hits          int             `json:"hits"`
	Served        int             `json:"served"`
	BytesRemoved  int64           `json:"bytes_removed"`
	EstTokens     int64           `json:"est_tokens"`
	Reference     roReference     `json:"reference"`
	NotComputable roNotComputable `json:"not_computable"`
	Recovery      *roRecovery     `json:"recovery"`
	RecoveryNote  string          `json:"recovery_note,omitempty"`
	Verdict       string          `json:"verdict"`
}

// roReference: of the served readouts replayed, how many had a dropped-only
// path or identifier named by one of the next five calls.
type roReference struct {
	Referenced int     `json:"referenced"`
	Of         int     `json:"of"`
	Rate       float64 `json:"rate"`
	CleanLower float64 `json:"clean_lower"`
}

// roNotComputable counts the served readouts the report could not replay.
type roNotComputable struct {
	Total          int `json:"total"`
	TranscriptGone int `json:"transcript_gone"`
	RuleChanged    int `json:"rule_changed"`
	OutputDiffers  int `json:"output_differs"`
}

// roRecovery is the lean rows' recovery: a repeat of the command or a read
// of the same file within five calls; net bytes are the bytes removed less
// the recovery calls' result bytes, over the lean rows replayed.
type roRecovery struct {
	LeanRows  int     `json:"lean_rows"`
	Recovered int     `json:"recovered"`
	Of        int     `json:"of"`
	Rate      float64 `json:"rate"`
	Lower     float64 `json:"lower"`
	NetBytes  int64   `json:"net_bytes"`
}

// roRow is one readout row with a matched rule.
type roRow struct {
	at        time.Time
	rule      string
	ruleSHA   string
	toolUseID string
	session   string
	mode      string
	cmdSHA    string
	raw       bool
	bytesIn   int
	bytesOut  int
}

func cmdReadoutReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("readout report", stderr)
	sinceFlag := fs.String("since", "", "bound the window to the last duration (e.g. 7d, 36h; default: both hook log generations whole)")
	jsonOut := fs.Bool("json", false, "emit the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: readout report takes no arguments (got %q)\n", fs.Arg(0))
		return 1
	}
	var since time.Time
	if *sinceFlag != "" {
		d, err := parseHumaneDuration(*sinceFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: readout report --since: %v\n", err)
			return 1
		}
		since = readoutNow().Add(-d).UTC()
	}
	rs, err := readout.LoadRules(fathomhome.FathomHome())
	if err != nil {
		fmt.Fprintf(stderr, "error: readout report: %v\n", err)
		return 2
	}
	rep := readoutReport(rs, beatLogPath(), readoutProjectsDir(), since)
	rep.Window.Since = *sinceFlag
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 2
		}
		return 0
	}
	printReadoutReport(stdout, rep)
	return 0
}

// readoutReport reads the readout rows of both hook-log generations at or
// after since, replays each served one from the transcripts under projects
// and reads every rule against decision 9's bars.
func readoutReport(rs *readout.RuleSet, logPath, projects string, since time.Time) roReport {
	rows, oldest := readoutReportRows(logPath, since)
	rep := roReport{Rules: []roReportRule{}}
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

	bySHA := map[string]*readout.Rule{}
	for _, r := range rs.Rules {
		if sha := readoutRuleSHA(r); sha != "" {
			bySHA[sha] = r
		}
	}
	want := map[string]bool{}
	sessions := map[string]bool{}
	allSessions := false
	for _, r := range rows {
		if r.raw || bySHA[r.ruleSHA] == nil || r.toolUseID == "" {
			continue
		}
		want[r.toolUseID] = true
		if r.session == "" {
			allSessions = true
		}
		sessions[r.session] = true
	}
	var calls map[string]*roCall
	if len(want) > 0 && projects != "" {
		if allSessions {
			sessions = nil
		}
		calls = readoutReportCalls(projects, from, want, sessions)
	}

	byRule := map[string]*roReportRule{}
	for _, r := range rows {
		rr := byRule[r.rule]
		if rr == nil {
			rr = &roReportRule{Rule: r.rule}
			byRule[r.rule] = rr
		}
		readoutReportRow(rr, r, bySHA[r.ruleSHA], calls[r.toolUseID])
	}
	for _, rr := range byRule {
		rr.EstTokens = rr.BytesRemoved / 4
		ref := &rr.Reference
		ref.Rate = roRound(shareOf(int64(ref.Referenced), int64(ref.Of)))
		lo, _ := ask.Wilson(ref.Of-ref.Referenced, ref.Of)
		ref.CleanLower = roRound(lo)
		if rr.Recovery == nil {
			rr.RecoveryNote = roNoLeanRows
		} else {
			rec := rr.Recovery
			rec.Rate = roRound(shareOf(int64(rec.Recovered), int64(rec.Of)))
			lo, _ := ask.Wilson(rec.Recovered, rec.Of)
			rec.Lower = roRound(lo)
		}
		rr.Verdict = readoutVerdict(ref.Of, ref.Referenced, rr.Recovery)
		rep.Rules = append(rep.Rules, *rr)
	}
	sort.Slice(rep.Rules, func(i, j int) bool { return rep.Rules[i].Rule < rep.Rules[j].Rule })
	return rep
}

// readoutReportRow folds one row into its rule's reading.
func readoutReportRow(rr *roReportRule, r roRow, rule *readout.Rule, call *roCall) {
	rr.Hits++
	if r.raw {
		return
	}
	rr.Served++
	removed := int64(r.bytesIn - r.bytesOut)
	rr.BytesRemoved += removed
	lean := r.mode == readout.ModeLean
	if lean {
		if rr.Recovery == nil {
			rr.Recovery = &roRecovery{}
		}
		rr.Recovery.LeanRows++
	}
	nc := &rr.NotComputable
	switch {
	case rule == nil:
		nc.Total++
		nc.RuleChanged++
		return
	case call == nil:
		nc.Total++
		nc.TranscriptGone++
		return
	}
	referenced, recoveryBytes, recovered, ok := call.replay(rule, r)
	if !ok {
		nc.Total++
		nc.OutputDiffers++
		return
	}
	rr.Reference.Of++
	if referenced {
		rr.Reference.Referenced++
	}
	if lean {
		rec := rr.Recovery
		rec.Of++
		if recovered {
			rec.Recovered++
		}
		rec.NetBytes += removed - recoveryBytes
	}
}

// readoutVerdict reads decision 9's bars: demote when the lean recovery
// rate's 95% Wilson lower bound exceeds 5%; promote when at least 200
// served readouts were replayed (a row the report cannot replay never
// counts toward the bar) and the lower bound that none had a dropped line
// referenced is at least 0.90; insufficient under 200; keep otherwise.
func readoutVerdict(of, referenced int, rec *roRecovery) string {
	if rec != nil && rec.Of > 0 {
		if lo, _ := ask.Wilson(rec.Recovered, rec.Of); lo > roDemoteLower {
			return roVerdictDem
		}
	}
	if of < replayMinEvaluated {
		return roVerdictIns
	}
	if lo, _ := ask.Wilson(of-referenced, of); lo >= replayMinCleanLower {
		return roVerdictPro
	}
	return roVerdictKp
}

func roRound(f float64) float64 { return math.Round(f*1e4) / 1e4 }

// readoutReportRows streams both hook-log generations, the rotated one
// first, keeping the readout rows that name a rule at or after since; oldest
// is the first beat on any channel, what the window is asserted from. It
// decodes only readout lines, so a 64 MB generation of other channels'
// beats is never held in memory (readBeatsSince decodes every beat).
func readoutReportRows(path string, since time.Time) (rows []roRow, oldest time.Time) {
	if path == "" {
		return nil, time.Time{}
	}
	marker := []byte(`"channel":"` + readoutChannel + `"`)
	for _, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		br := bufio.NewReaderSize(f, 1<<20)
		for {
			line, rerr := br.ReadBytes('\n')
			if len(bytes.TrimSpace(line)) > 0 {
				if oldest.IsZero() {
					var b struct {
						At time.Time `json:"at"`
					}
					if json.Unmarshal(line, &b) == nil {
						oldest = b.At
					}
				}
				if bytes.Contains(line, marker) {
					if r, ok := readoutReportRowOf(line, since); ok {
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

func readoutReportRowOf(line []byte, since time.Time) (roRow, bool) {
	var b heartbeat.Beat
	if json.Unmarshal(line, &b) != nil || b.Channel != readoutChannel || b.Meta["rule"] == "" {
		return roRow{}, false
	}
	if !since.IsZero() && b.At.Before(since) {
		return roRow{}, false
	}
	m := b.Meta
	r := roRow{at: b.At, rule: m["rule"], ruleSHA: m["rule_sha"], toolUseID: m["tool_use_id"], session: m["session"],
		mode: m["mode"], cmdSHA: m["cmd_sha"], raw: m["raw"] == "true"}
	r.bytesIn, _ = strconv.Atoi(m["bytes_in"])
	r.bytesOut, _ = strconv.Atoi(m["bytes_out"])
	if r.mode == "" {
		r.mode = readout.ModeObserve
	}
	return r, true
}

// roCall is one wanted Bash call as its transcript holds it: the command,
// its result, and the next five tool calls.
type roCall struct {
	idx         int
	command     string
	stdout      string
	stderr      string
	interrupted bool
	persisted   string
	resulted    bool
	followers   []*roFollower
}

// roFollower is a later call as the report reads it: the paths and
// identifiers its input names (replay's tokens), its command's hash, the
// files it reads, and its result's bytes once that arrives.
type roFollower struct {
	id     string
	tokens map[string]struct{}
	cmdSHA string
	reads  []string
	bytes  int64
}

// replay re-renders the call with the row's rule and mode; ok is false when
// the command or the rendering differs from what the row recorded. It
// returns whether a following call named a dropped-only token and, for the
// recovery, whether one repeated the command or read the same file, with
// those calls' result bytes.
func (c *roCall) replay(rule *readout.Rule, r roRow) (referenced bool, recoveryBytes int64, recovered, ok bool) {
	if r.cmdSHA != "" && sha256Hex(c.command) != r.cmdSHA {
		return false, 0, false, false
	}
	cmd := shellcmd.ParseCommand(c.command)
	ro := readout.Render(&readout.RuleSet{Rules: []*readout.Rule{rule}}, readout.Input{
		Command: &cmd, Stdout: c.stdout, Stderr: c.stderr, Interrupted: c.interrupted,
		Persisted: c.persisted, Mode: r.mode,
	})
	if ro.Rule != r.rule || ro.Raw || ro.BytesIn != r.bytesIn || ro.BytesOut != r.bytesOut {
		return false, 0, false, false
	}
	exact, suffixes := droppedOnlyTokens(c.stdout, ro.Stdout, c.command)
	files := append(wholeFileRead(&cmd), sedReadFiles(&cmd)...)
	for _, f := range c.followers {
		if !referenced && refersToDropped(f.tokens, exact, suffixes) {
			referenced = true
		}
		if (r.cmdSHA != "" && f.cmdSHA == r.cmdSHA) || roReadsAny(f.reads, files) {
			recovered = true
			recoveryBytes += f.bytes
		}
	}
	return referenced, recoveryBytes, recovered, true
}

// roReadsAny: a file of reads names one of files, by the same path or one a
// '/'-suffix of the other (an absolute Read of a relative cat).
func roReadsAny(reads, files []string) bool {
	for _, a := range reads {
		for _, b := range files {
			a, b := filepath.Clean(a), filepath.Clean(b)
			if a == b || strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a) {
				return true
			}
		}
	}
	return false
}

// readoutReportCalls finds the wanted tool-use ids in the transcripts under
// root modified at or after from, reading only files whose path names one
// of sessions (nil: every file). A call is taken from the first file, in
// walk order, that holds both it and its result.
func readoutReportCalls(root string, from time.Time, want, sessions map[string]bool) map[string]*roCall {
	out := map[string]*roCall{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") || len(out) == len(want) {
			return nil
		}
		if info, ierr := d.Info(); ierr != nil || info.ModTime().Before(from) {
			return nil
		}
		if sessions != nil && !roPathNamesSession(p, sessions) {
			return nil
		}
		readoutReportFile(p, want, out)
		return nil
	})
	return out
}

func roPathNamesSession(p string, sessions map[string]bool) bool {
	for s := range sessions {
		if s != "" && strings.Contains(p, s) {
			return true
		}
	}
	return false
}

// readoutReportFile streams one transcript with replay's decoder types.
func readoutReportFile(p string, want map[string]bool, out map[string]*roCall) {
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()
	open := map[string]*roCall{}
	byFollower := map[string]*roFollower{}
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
			seq = readoutReportLine(buf, seq, want, out, open, byFollower)
		}
		buf, over = buf[:0], false
		if rerr != nil {
			break
		}
	}
	for id, c := range open {
		if c.resulted {
			out[id] = c
		}
	}
}

func readoutReportLine(raw []byte, seq int, want map[string]bool, out, open map[string]*roCall, byFollower map[string]*roFollower) int {
	var l replayTranscriptLine
	if json.Unmarshal(raw, &l) != nil || l.Message == nil || len(l.Message.Content) == 0 || l.Message.Content[0] != '[' {
		return seq
	}
	var blocks []replayBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return seq
	}
	results := 0
	for _, b := range blocks {
		if b.Type == "tool_result" {
			results++
		}
	}
	for i := range blocks {
		b := &blocks[i]
		switch b.Type {
		case "tool_use":
			fl, command := roFollowerOf(b)
			for _, c := range open {
				if seq > c.idx && seq <= c.idx+replayFollowers {
					c.followers = append(c.followers, fl)
					if fl.id != "" {
						byFollower[fl.id] = fl
					}
				}
			}
			if b.Name == "Bash" && want[b.ID] && out[b.ID] == nil && open[b.ID] == nil {
				open[b.ID] = &roCall{idx: seq, command: command}
			}
			seq++
		case "tool_result":
			stdout, stderr, tr := roResultOf(&l, b, results == 1)
			if c := open[b.ToolUseID]; c != nil && !c.resulted {
				c.stdout, c.stderr, c.interrupted, c.persisted, c.resulted = stdout, stderr, tr.Interrupted, tr.PersistedOutputPath, true
			}
			if fl := byFollower[b.ToolUseID]; fl != nil {
				fl.bytes = int64(len(stdout) + len(stderr))
			}
		}
	}
	return seq
}

// roFollowerOf reads a tool call's input as replay does, plus its command
// hash and the files it reads (a Read's file_path, a whole-file cat or a
// `sed -n` of a Bash command).
func roFollowerOf(b *replayBlock) (*roFollower, string) {
	fl := &roFollower{id: b.ID, tokens: map[string]struct{}{}}
	if b.Name == "Bash" {
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(b.Input, &in)
		c := shellcmd.ParseCommand(in.Command)
		refTokens(in.Command, fl.tokens)
		fl.cmdSHA = sha256Hex(in.Command)
		fl.reads = append(wholeFileRead(&c), sedReadFiles(&c)...)
		return fl, in.Command
	}
	var in any
	if json.Unmarshal(b.Input, &in) == nil {
		walkJSONStrings(in, func(s string) { refTokens(s, fl.tokens) })
	}
	if b.Name == "Read" {
		var rd struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(b.Input, &rd) == nil && rd.FilePath != "" {
			fl.reads = []string{rd.FilePath}
		}
	}
	return fl, ""
}

// roResultOf is a tool result's text the way the hook saw it: the
// structured stdout and stderr when the line carries one result, else the
// result's content; never the persisted file (the hook got its path only).
func roResultOf(l *replayTranscriptLine, b *replayBlock, single bool) (stdout, stderr string, tr replayToolResult) {
	if single && len(l.ToolUseResult) > 0 && l.ToolUseResult[0] == '{' &&
		json.Unmarshal(l.ToolUseResult, &tr) == nil && tr.Stdout != nil && !b.IsError {
		return *tr.Stdout, tr.Stderr, tr
	}
	return resultText(b.Content), "", replayToolResult{}
}

// printReadoutReport writes the report's text form.
func printReadoutReport(w io.Writer, rep roReport) {
	win := rep.Window
	switch {
	case win.From == "":
		fmt.Fprintln(w, "readout report: no readout rows in the hook log")
	default:
		fmt.Fprintf(w, "readout report: window %s to %s (%.1f days, from the first row)\n", win.From, win.To, win.Days)
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
	for _, r := range rep.Rules {
		fmt.Fprintf(w, "\n%s: verdict %s\n", r.Rule, r.Verdict)
		fmt.Fprintf(w, "  hits %d, served %d\n", r.Hits, r.Served)
		fmt.Fprintf(w, "  bytes removed %d bytes (in observe, what the readout would remove)\n", r.BytesRemoved)
		fmt.Fprintf(w, "  est. tokens %d (an estimate: bytes/4)\n", r.EstTokens)
		ref := r.Reference
		if ref.Of == 0 {
			fmt.Fprintln(w, "  reference rate: no served readout replayed")
		} else {
			fmt.Fprintf(w, "  reference rate %d of %d replayed readouts had a dropped line named within five calls (%.4f; clean share's 95%% Wilson lower bound %.4f)\n",
				ref.Referenced, ref.Of, ref.Rate, ref.CleanLower)
		}
		nc := r.NotComputable
		fmt.Fprintf(w, "  not computable %d (transcript gone %d, rule changed %d, output differs %d), counted apart, never as clean\n",
			nc.Total, nc.TranscriptGone, nc.RuleChanged, nc.OutputDiffers)
		if r.Recovery == nil {
			fmt.Fprintf(w, "  recovery: %s\n  net bytes: %s\n", roNoLeanRows, roNoLeanRows)
			continue
		}
		rec := r.Recovery
		fmt.Fprintf(w, "  recovery %d of %d lean readouts replayed (%.4f; 95%% Wilson lower bound %.4f; %d lean rows)\n",
			rec.Recovered, rec.Of, rec.Rate, rec.Lower, rec.LeanRows)
		fmt.Fprintf(w, "  net bytes %d bytes (removed less the recovery calls' result bytes, over the lean readouts replayed)\n", rec.NetBytes)
	}
}
