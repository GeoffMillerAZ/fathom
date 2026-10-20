package main

// readoutcmd_replay.go — `fathom readout replay`: the readout rules over the
// local Claude Code transcripts, aggregates only (design
// docs/design/readouts.md §8, §9 and §10; the corpus reading the plan's
// first measurement reads). Every Bash call in the window is paired with
// its result, deduplicated by tool-use id (a resumed session copies its
// history into a new file), lexed, measured by shape and rendered through
// the engine in observe; a dropped line's paths and identifiers are matched
// against the session's next five calls. Every call is read twice: as
// matched (each rule's match.compound, what the hook serves) and as
// projected (the call's last segment after its leading quiet segments, read
// as a simple command over the call's whole output), and is shaped twice, by
// its first unit and by that effective segment. The output carries counts,
// bytes, shares, rule ids, command heads and lexer reason classes: never
// command text, output text, a path, a session id or a project name.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutProjectsDir is Claude Code's transcript root and readoutNow the
// clock the window is cut from; tests replace both.
var (
	readoutProjectsDir = defaultReadoutProjectsDir
	readoutNow         = time.Now
)

// defaultReadoutProjectsDir is $CLAUDE_CONFIG_DIR/projects, else
// ~/.claude/projects (harness.ClaudeConfigRoot); "" when no root resolves.
func defaultReadoutProjectsDir() string {
	root := harness.ClaudeConfigRoot(wiringDriftHome())
	if root == "" {
		return ""
	}
	return filepath.Join(root, "projects")
}

const (
	replayFollowers     = 5        // "within five calls"
	replayRecent        = 32       // calls kept to feed a result that arrives after later calls
	replayLongRead      = 200      // a whole-file read over this many lines is a long read
	replayMinEvaluated  = 200      // lean eligibility: evaluated readouts
	replayMinCleanLower = 0.90     // lean eligibility: the clean share's Wilson lower bound
	replayTopShapes     = 25       // shapes listed before the rest fold into "(other)"
	replayLineMax       = 64 << 20 // a transcript line longer than this is skipped, counted
	replayPersistedMax  = 16 << 20 // the most of a persisted output file read
	replayTokenMax      = 4096     // dropped-only tokens kept per readout
)

type replayReport struct {
	Window          replayWindow      `json:"window"`
	Corpus          replayCorpus      `json:"corpus"`
	Bash            replayBash        `json:"bash"`
	Projection      replayProjection  `json:"projection"`
	WholeFileReads  replayWhole       `json:"whole_file_reads"`
	Rules           []replayRule      `json:"rules"`
	Eligible        replayEligible    `json:"eligible"`
	EffectiveShapes []replayShapeRow  `json:"effective_shapes"`
	Shapes          []replayShapeRow  `json:"shapes"`
	Tccutil         replayTcc         `json:"tccutil_reset"`
	Reread          replayReread      `json:"reread"`
	Unmatched       replayUnmatched   `json:"unmatched"`
	Joins           replayJoins       `json:"joins"`
	RefusedRules    []string          `json:"refused_rules"`
	Dedupe          replayDedupe      `json:"dedupe"`
	Spawns          replaySpawns      `json:"spawns"`
	Compactions     replayCompactions `json:"compactions"`
}

// replayProjection names what the projected figures and the effective
// shapes read: the quiet heads stripped from a call's front.
type replayProjection struct {
	Quiet []string `json:"quiet"`
	Note  string   `json:"note"`
}

const replayProjectionNote = "projected figures read each call as its last segment after its leading quiet segments, " +
	"matched as a simple command over the call's whole output: a projection, not what the hook serves"

// replayUnmatched is the calls no rule matched, by the engine's reason,
// and the unreadable ones by the class of the lexer's reason.
// replayJoins is how replay filled what the built-ins read beyond the
// output: the declared-test answer by where its declarations came from, the
// persisted text, HEAD's stamp (never known for a past call) and the outline.
type replayJoins struct {
	DeclaredTestByRoot    int    `json:"declared_test_by_root"`
	DeclaredTestByBuiltin int    `json:"declared_test_by_builtin"`
	PersistedMissed       int    `json:"persisted_missed"`
	HeadStampMissed       int    `json:"head_stamp_missed"`
	OutlineComputed       int    `json:"outline_computed"`
	OutlineMissed         int    `json:"outline_missed"`
	EditedRaw             int    `json:"edited_raw"`
	Note                  string `json:"note"`
}

const replayJoinsNote = "a call's declared test commands are read from its cwd when that directory exists, else matched against the built-in test runner list alone; " +
	"a past call's HEAD is unknown, so head_stamp is always a missed join; the outline is computed in process with no deadline, so the reading is deterministic; " +
	"a whole-file read of a file an Edit, Write or MultiEdit touched earlier in the session stays raw"

type replayUnmatched struct {
	ByReason           map[string]int      `json:"by_reason"`
	UnreadableByReason []replayReasonCount `json:"unreadable_by_reason"`
}

type replayReasonCount struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type replayWindow struct {
	Days int    `json:"days"`
	From string `json:"from"`
	To   string `json:"to"`
}

type replayCorpus struct {
	Projects      int `json:"projects"`
	Sessions      int `json:"sessions"`
	Files         int `json:"files"`
	LinesUnparsed int `json:"lines_unparsed"`
	LinesOversize int `json:"lines_oversize"`
}

type replayBash struct {
	Calls         int   `json:"calls"`
	Bytes         int64 `json:"bytes"`
	Duplicates    int   `json:"duplicates_skipped"`
	Unpaired      int   `json:"unpaired"`
	Failures      int   `json:"failures"`
	Persisted     int   `json:"persisted"`
	PersistedRead int   `json:"persisted_read"`
}

type replayWhole struct {
	OverLines int     `json:"over_lines"`
	Calls     int     `json:"calls"`
	Bytes     int64   `json:"bytes"`
	Share     float64 `json:"share_of_bash_bytes"`
}

type replayRule struct {
	Rule      string         `json:"rule"`
	Lossiness string         `json:"lossiness"`
	AsMatched replayRuleView `json:"as_matched"`
	Projected replayRuleView `json:"projected"`
}

// replayRuleView is one rule's figures under one reading of the calls.
type replayRuleView struct {
	Hits          int            `json:"hits"`
	BytesIn       int64          `json:"bytes_in"`
	Served        int            `json:"served"`
	WouldRemove   int64          `json:"would_remove_bytes"`
	Share         float64        `json:"share_of_bash_bytes"` // would-remove bytes over Bash bytes
	Raw           map[string]int `json:"raw"`
	Trips         replayTrips    `json:"trips"`
	UnscopedTrips int            `json:"unscoped_trips"`
	Reference     replayRef      `json:"reference"`
	Eligible      bool           `json:"eligible"`
}

func newReplayRule(ref, lossiness string) *replayRule {
	return &replayRule{Rule: ref, Lossiness: lossiness,
		AsMatched: replayRuleView{Raw: map[string]int{}}, Projected: replayRuleView{Raw: map[string]int{}}}
}

type replayTrips struct {
	Content    int `json:"content"`
	Diagnostic int `json:"diagnostic"`
}

type replayRef struct {
	Referenced       int     `json:"referenced"`
	Of               int     `json:"of"`
	CleanWilsonLower float64 `json:"clean_wilson_lower"`
}

type replayEligible struct {
	MinEvaluated  int                `json:"min_evaluated"`
	MinCleanLower float64            `json:"min_clean_lower"`
	AsMatched     replayEligibleView `json:"as_matched"`
	Projected     replayEligibleView `json:"projected"`
}

type replayEligibleView struct {
	Rules       []string `json:"rules"`
	WouldRemove int64    `json:"would_remove_bytes"`
	Share       float64  `json:"share_of_bash_bytes"`
}

type replayShapeRow struct {
	Shape         string  `json:"shape"`
	Calls         int     `json:"calls"`
	Bytes         int64   `json:"bytes"`
	Share         float64 `json:"share_of_bash_bytes"`
	Simple        int     `json:"simple"`
	Piped         int     `json:"piped"`
	List          int     `json:"list"`
	CompoundShare float64 `json:"compound_share"`
}

type replayTcc struct {
	Calls         int     `json:"calls"`
	Compound      int     `json:"compound"`
	CompoundShare float64 `json:"compound_share"`
}

type replayReread struct {
	WholeReads        int `json:"whole_reads"`
	FollowedBySed     int `json:"followed_by_sed"`
	LongWholeReads    int `json:"long_whole_reads"`
	LongFollowedBySed int `json:"long_followed_by_sed"`
}

// cmdReadoutReplay is `fathom readout replay`.
func cmdReadoutReplay(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("readout replay", stderr)
	days := fs.Int("days", 30, "the window: calls from the last N days")
	projects := fs.String("projects", "*", "a glob over the transcript project directory names to read")
	rulesFile := fs.String("rules", "", "a rule file to replay alone (default: the built-ins and the files under the fathom home)")
	jsonOut := fs.Bool("json", false, "emit the aggregates as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: readout replay takes no arguments (got %q)\n", fs.Arg(0))
		return 1
	}
	if *days < 1 {
		fmt.Fprintln(stderr, "error: readout replay --days must be at least 1")
		return 1
	}
	if _, err := filepath.Match(*projects, ""); err != nil {
		fmt.Fprintf(stderr, "error: readout replay --projects: %v\n", err)
		return 1
	}
	rs, err := loadReadoutRules(*rulesFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: readout replay: %v\n", err)
		return 2
	}
	root := readoutProjectsDir()
	if root == "" {
		fmt.Fprintln(stderr, "error: readout replay: no home directory, so no transcripts")
		return 2
	}
	a := newReplayAgg(rs, *days, readoutNow().Add(-time.Duration(*days)*24*time.Hour))
	if cfg, ok := readoutsUserConfig(); ok && len(cfg.QuietPrefixes) > 0 {
		// The hook reads the user's quiet heads; so does its replay.
		a.quiet = cfg.QuietPrefixes
		a.rep.Projection.Quiet = append(a.rep.Projection.Quiet, cfg.QuietPrefixes...)
	}
	if err := a.walk(root, *projects); err != nil {
		fmt.Fprintf(stderr, "error: readout replay: reading the transcripts: %v\n", err)
		return 2
	}
	rep := a.report()
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 2
		}
		return 0
	}
	printReplayReport(stdout, rep)
	return 0
}

// replayAgg accumulates one replay.
type replayAgg struct {
	rep        replayReport
	rs         *readout.RuleSet
	cutoff     time.Time
	rules      map[string]*replayRule
	shapes     map[string]*replayShapeRow // by first unit
	effective  map[string]*replayShapeRow // by the last segment after the quiet ones
	unreadable map[string]int             // by lexer reason class
	seen       map[string]struct{}        // tool-use ids already counted
	sessions   map[string]struct{}
	from, to   time.Time
	// roots are the declarations read per cwd, once each; edited are the
	// files each session's Edit, Write and MultiEdit calls touched.
	roots   map[string]replayRoot
	runners []declaredTestCommand
	edited  map[string]map[string]struct{}
	sizing  *replaySizing // the dedupe, spawns and compactions sections (readoutcmd_sizing.go)
	// quiet are the user's [readouts] quiet_prefixes, read as the hook reads them.
	quiet []string
}

// replayRoot is one cwd's declared test commands, read once.
type replayRoot struct {
	exists   bool
	declared []declaredTestCommand
}

func newReplayAgg(rs *readout.RuleSet, days int, cutoff time.Time) *replayAgg {
	a := &replayAgg{rs: rs, cutoff: cutoff, rules: map[string]*replayRule{}, shapes: map[string]*replayShapeRow{},
		effective: map[string]*replayShapeRow{}, unreadable: map[string]int{},
		seen: map[string]struct{}{}, sessions: map[string]struct{}{},
		roots: map[string]replayRoot{}, edited: map[string]map[string]struct{}{}, sizing: newReplaySizing(days, cutoff)}
	for _, r := range testRunners {
		a.runners = append(a.runners, declaredTestCommand{Invocation: strings.Join(r, " "), Source: "built-in runners"})
	}
	a.rep.Joins.Note = replayJoinsNote
	a.rep.Window.Days = days
	a.rep.Projection = replayProjection{Quiet: readout.QuietHeads(), Note: replayProjectionNote}
	a.rep.Unmatched.ByReason = map[string]int{}
	for _, r := range rs.Rules {
		a.rules[r.Ref()] = newReplayRule(r.Ref(), readout.Lossiness(r.Stages).String())
	}
	for _, ref := range rs.Refused {
		id := ref.ID
		if id == "" {
			id = "(file)"
		}
		a.rep.RefusedRules = append(a.rep.RefusedRules, id)
	}
	sort.Strings(a.rep.RefusedRules)
	return a
}

// walk reads every *.jsonl under each project directory the glob matches,
// in name order, skipping files last written before the window and a
// workflow run's step journal (replayWorkflowJournal), which is no
// transcript.
func (a *replayAgg) walk(root, glob string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if ok, _ := filepath.Match(glob, e.Name()); !ok {
			continue
		}
		counted := false
		werr := filepath.WalkDir(filepath.Join(root, e.Name()), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") || replayWorkflowJournal(p) {
				return nil
			}
			if info, ierr := d.Info(); ierr != nil || info.ModTime().Before(a.cutoff) {
				return nil
			}
			if a.file(p) {
				counted = true
			}
			return nil
		})
		if werr != nil {
			return werr
		}
		if counted {
			a.rep.Corpus.Projects++
		}
	}
	return nil
}

// replayWorkflowJournal reports a workflow run's step journal,
// <session>/subagents/workflows/<run>/journal.jsonl, beside the run's
// agent-*.jsonl transcripts (O1207): it is the workflow's own log, so replay
// neither opens nor counts it.
func replayWorkflowJournal(p string) bool {
	dir, base := filepath.Split(p)
	if base != "journal.jsonl" {
		return false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	n := len(parts)
	return n >= 3 && parts[n-3] == "subagents" && parts[n-2] == "workflows"
}

// replayFile is one transcript's reading state.
type replayFile struct {
	seq     int
	pending map[string]replayPending
	recent  []replayFollower
	open    []*replaySubject
	counted bool
}

type replayPending struct {
	idx      int
	command  string
	cwd      string
	ts       time.Time
	tsRaw    string
	inWindow bool
}

// replayFollower is one tool call as a later call sees it: the paths and
// identifiers its input names, and the files a `sed -n` of it reads.
type replayFollower struct {
	idx      int
	tokens   map[string]struct{}
	sedFiles []string
}

// replaySubject is a counted Bash call waiting for its next five calls: a
// served readout in either reading (is a dropped line referenced?) or a
// whole-file read (is the file then read again with `sed -n`?).
type replaySubject struct {
	idx     int
	watches []*replayWatch
	reads   []string
	long    bool
	reread  bool
}

// replayWatch is one served readout's dropped-only tokens, counted into
// the reference figures of the rule view that served it.
type replayWatch struct {
	ref        *replayRef
	dropped    map[string]struct{}
	suffixes   map[string]struct{}
	referenced bool
}

// file streams one transcript; true when it counted a call.
func (a *replayAgg) file(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	a.rep.Corpus.Files++
	st := &replayFile{pending: map[string]replayPending{}}
	sz := a.sizing.open(p)
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
		switch {
		case over:
			a.rep.Corpus.LinesOversize++
		case len(buf) > 0:
			a.line(st, buf)
			a.sizing.line(sz, buf)
		}
		buf, over = buf[:0], false
		if rerr != nil {
			break
		}
	}
	for _, s := range st.open {
		a.finish(s)
	}
	a.sizing.close(sz)
	for _, pd := range st.pending {
		if pd.inWindow {
			a.rep.Bash.Unpaired++
		}
	}
	return st.counted
}

type replayTranscriptLine struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	Message   *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type replayBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func (a *replayAgg) line(st *replayFile, raw []byte) {
	if !bytes.Contains(raw, []byte(`"tool_`)) {
		return
	}
	var l replayTranscriptLine
	if err := json.Unmarshal(raw, &l); err != nil {
		a.rep.Corpus.LinesUnparsed++
		return
	}
	if l.Message == nil || len(l.Message.Content) == 0 || l.Message.Content[0] != '[' {
		return
	}
	var blocks []replayBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		a.rep.Corpus.LinesUnparsed++
		return
	}
	results := 0
	for _, b := range blocks {
		if b.Type == "tool_result" {
			results++
		}
	}
	for _, b := range blocks {
		switch b.Type {
		case "tool_use":
			a.use(st, &l, &b)
		case "tool_result":
			a.result(st, &l, &b, results == 1)
		}
	}
}

// use records a tool call: a follower of every open subject, and a
// pending Bash call until its result arrives.
func (a *replayAgg) use(st *replayFile, l *replayTranscriptLine, b *replayBlock) {
	f := replayFollower{idx: st.seq, tokens: map[string]struct{}{}}
	st.seq++
	var command string
	if b.Name == "Bash" {
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(b.Input, &in)
		command = in.Command
		c := shellcmd.ParseCommand(command)
		f.sedFiles = sedReadFiles(&c)
		refTokens(command, f.tokens)
	} else {
		var in any
		if json.Unmarshal(b.Input, &in) == nil {
			walkJSONStrings(in, func(s string) { refTokens(s, f.tokens) })
		}
		a.noteEdit(l, b)
	}
	a.feed(st, f)
	st.recent = append(st.recent, f)
	if len(st.recent) > replayRecent {
		st.recent = append(st.recent[:0:0], st.recent[len(st.recent)-replayRecent:]...)
	}
	if b.Name != "Bash" || b.ID == "" {
		return
	}
	pd := replayPending{idx: f.idx, command: command, cwd: l.Cwd, tsRaw: l.Timestamp, inWindow: true}
	if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
		pd.ts = ts
		pd.inWindow = !ts.Before(a.cutoff)
	}
	st.pending[b.ID] = pd
}

// feed hands a new call to the open subjects it follows within five, and
// finishes those it is the fifth call after.
func (a *replayAgg) feed(st *replayFile, f replayFollower) {
	kept := st.open[:0]
	for _, s := range st.open {
		if f.idx > s.idx && f.idx <= s.idx+replayFollowers {
			s.see(f)
		}
		if f.idx >= s.idx+replayFollowers {
			a.finish(s)
			continue
		}
		kept = append(kept, s)
	}
	st.open = kept
}

func (s *replaySubject) see(f replayFollower) {
	for _, w := range s.watches {
		if !w.referenced {
			w.referenced = refersToDropped(f.tokens, w.dropped, w.suffixes)
		}
	}
	if len(s.reads) > 0 && !s.reread {
		for _, sf := range f.sedFiles {
			for _, r := range s.reads {
				if sf == r {
					s.reread = true
				}
			}
		}
	}
}

func (a *replayAgg) finish(s *replaySubject) {
	for _, w := range s.watches {
		w.ref.Of++
		if w.referenced {
			w.ref.Referenced++
		}
	}
	if len(s.reads) > 0 {
		a.rep.Reread.WholeReads++
		if s.reread {
			a.rep.Reread.FollowedBySed++
		}
		if s.long {
			a.rep.Reread.LongWholeReads++
			if s.reread {
				a.rep.Reread.LongFollowedBySed++
			}
		}
	}
}

type replayToolResult struct {
	Stdout              *string `json:"stdout"`
	Stderr              string  `json:"stderr"`
	Interrupted         bool    `json:"interrupted"`
	PersistedOutputPath string  `json:"persistedOutputPath"`
}

// result counts one Bash call once its result arrives.
func (a *replayAgg) result(st *replayFile, l *replayTranscriptLine, b *replayBlock, single bool) {
	pd, ok := st.pending[b.ToolUseID]
	if !ok {
		return
	}
	delete(st.pending, b.ToolUseID)
	if !pd.inWindow {
		return
	}
	if _, dup := a.seen[b.ToolUseID]; dup {
		a.rep.Bash.Duplicates++
		return
	}
	a.seen[b.ToolUseID] = struct{}{}
	st.counted = true
	session := a.sessionKey(l)
	if _, ok := a.sessions[session]; !ok {
		a.sessions[session] = struct{}{}
		a.rep.Corpus.Sessions++
	}
	if !pd.ts.IsZero() {
		if a.from.IsZero() || pd.ts.Before(a.from) {
			a.from, a.rep.Window.From = pd.ts, pd.tsRaw
		}
		if a.to.IsZero() || pd.ts.After(a.to) {
			a.to, a.rep.Window.To = pd.ts, pd.tsRaw
		}
	}

	var tr replayToolResult
	structured := single && len(l.ToolUseResult) > 0 && l.ToolUseResult[0] == '{' &&
		json.Unmarshal(l.ToolUseResult, &tr) == nil && tr.Stdout != nil
	var stdout, persistedText string
	switch {
	case b.IsError:
	case structured:
		stdout = *tr.Stdout
		if tr.PersistedOutputPath != "" {
			a.rep.Bash.Persisted++
			full, ok := readPersistedOutput(tr.PersistedOutputPath)
			if ok {
				stdout = full
				a.rep.Bash.PersistedRead++
			}
			// Counts are totals only over the whole file: one cut at the
			// read's bound is a missed join, as a missing one is.
			if ok && len(full) < replayPersistedMax {
				persistedText = full
			} else {
				a.rep.Joins.PersistedMissed++
			}
		}
	default:
		stdout = resultText(b.Content)
	}
	stderr := tr.Stderr
	n := int64(len(stdout) + len(stderr))
	if b.IsError {
		n = int64(len(resultText(b.Content)))
	}
	a.rep.Bash.Calls++
	a.rep.Bash.Bytes += n

	c := shellcmd.ParseCommand(pd.command)
	a.shape(&c, n)
	if b.IsError {
		a.rep.Bash.Failures++
		a.rep.Unmatched.ByReason["failure"]++
		return
	}
	s := &replaySubject{idx: pd.idx}
	if files := wholeFileRead(&c); len(files) > 0 {
		s.reads = files
		if lineCount(stdout) > replayLongRead {
			s.long = true
			a.rep.WholeFileReads.Calls++
			a.rep.WholeFileReads.Bytes += n
		}
	}
	in := readout.Input{Command: &c, Stdout: stdout, Stderr: stderr, Interrupted: tr.Interrupted,
		Persisted: tr.PersistedOutputPath, PersistedText: persistedText, Mode: readout.ModeObserve, Quiet: a.quiet}
	if declared, byRoot := a.declaredTest(pd.cwd, pd.command); declared {
		in.DeclaredTest = true
		if byRoot {
			a.rep.Joins.DeclaredTestByRoot++
		} else {
			a.rep.Joins.DeclaredTestByBuiltin++
		}
	}
	call := &replayCall{session: session, cwd: pd.cwd, command: &c}
	ro := a.render(in, call)
	if ro.Rule == "" {
		a.rep.Unmatched.ByReason[ro.Reason]++
		if ro.Reason == readout.ReasonUnreadable {
			a.unreadable[replayReasonClass(c.Reason)]++
		}
	} else if w := a.tally(&ro, false, stdout, pd.command); w != nil {
		s.watches = append(s.watches, w)
	}
	pro := ro
	if in.Command = replayProjected(&c, a.quiet); in.Command != &c {
		pro = a.render(in, call)
	}
	call.count(a, ro, pro)
	if pro.Rule != "" {
		if w := a.tally(&pro, true, stdout, pd.command); w != nil {
			s.watches = append(s.watches, w)
		}
	}
	if len(s.watches) == 0 && len(s.reads) == 0 {
		return
	}
	for _, f := range st.recent {
		if f.idx > s.idx && f.idx <= s.idx+replayFollowers {
			s.see(f)
		}
	}
	if st.seq > s.idx+replayFollowers {
		a.finish(s)
		return
	}
	st.open = append(st.open, s)
}

// replayCall is one counted call's join state, shared by its as-matched and
// projected renders: the outline is computed once.
type replayCall struct {
	session, cwd string
	command      *shellcmd.Command // as the transcript carries it
	outline      []string
	computed     bool
	outlineMiss  bool
	edited       bool
}

// render runs the engine, then, for a matched whole-file rule with an
// outline stage, the edit check and the outline join, and runs it again.
func (a *replayAgg) render(in readout.Input, call *replayCall) readout.Readout {
	ro := readout.Render(a.rs, in)
	rule := matchedRule(a.rs, ro.Rule)
	if rule == nil || ro.Reason == readout.ReasonBelowFloor || !rule.Match.WholeRead || !ruleOutlines(rule) {
		return ro
	}
	path, ok := readout.WholeFileRead(in.Command)
	if !ok {
		return ro
	}
	if a.editedIn(call.session, replayReadPath(call.command, call.cwd, path)) {
		in.Edited, call.edited = true, true
		return readout.Render(a.rs, in)
	}
	if !call.computed {
		call.computed = true
		text := in.Stdout
		if in.Persisted != "" {
			text = in.PersistedText
		}
		if text != "" {
			call.outline, call.outlineMiss = readout.Outline(context.Background(), path, text)
		} else {
			call.outlineMiss = true
		}
	}
	joins := map[string][]string{}
	for k, v := range in.Joins {
		joins[k] = v
	}
	in.Joins = joins
	if call.outline != nil {
		in.Joins[readoutJoinOutline] = call.outline
	}
	return readout.Render(a.rs, in)
}

// count adds one call's join figures once, whichever reading needed them.
func (call *replayCall) count(a *replayAgg, views ...readout.Readout) {
	if call.edited {
		a.rep.Joins.EditedRaw++
	}
	if call.computed {
		if call.outlineMiss {
			a.rep.Joins.OutlineMissed++
		} else {
			a.rep.Joins.OutlineComputed++
		}
	}
	for _, v := range views {
		if containsString(v.JoinsMissed, readoutJoinHeadStamp) {
			a.rep.Joins.HeadStampMissed++
			return
		}
	}
}

// sessionKey is a transcript line's session, else its file's.
func (a *replayAgg) sessionKey(l *replayTranscriptLine) string {
	if l.SessionID != "" {
		return l.SessionID
	}
	return fmt.Sprintf("file-%d", a.rep.Corpus.Files)
}

// noteEdit records the file an Edit, Write or MultiEdit call names for its
// session: a later whole-file read of it is verification and stays raw.
func (a *replayAgg) noteEdit(l *replayTranscriptLine, b *replayBlock) {
	switch b.Name {
	case "Edit", "Write", "MultiEdit":
	default:
		return
	}
	var in struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(b.Input, &in) != nil || in.FilePath == "" {
		return
	}
	p := in.FilePath
	if !filepath.IsAbs(p) && l.Cwd != "" {
		p = filepath.Join(l.Cwd, p)
	}
	key := a.sessionKey(l)
	if a.edited[key] == nil {
		a.edited[key] = map[string]struct{}{}
	}
	a.edited[key][filepath.Clean(p)] = struct{}{}
}

// editedIn reports whether session's earlier calls edited path.
func (a *replayAgg) editedIn(session, path string) bool {
	_, ok := a.edited[session][path]
	return ok && path != ""
}

// replayReadPath resolves a read's file against the call's cwd moved by the
// leading `cd <dir>` segments the projection strips; "" when it cannot.
func replayReadPath(c *shellcmd.Command, cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	dir := cwd
	for i := 0; i+1 < len(c.Pipelines); i++ {
		p := c.Pipelines[i]
		if len(p.Units) != 1 || p.Units[0].Name() != "cd" {
			continue
		}
		args := p.Units[0].Words
		if len(args) == 2 && args[1].Literal() {
			if filepath.IsAbs(args[1].Text) {
				dir = args[1].Text
			} else if dir != "" {
				dir = filepath.Join(dir, args[1].Text)
			}
		}
	}
	if dir == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(dir, path))
}

// declaredTest is TR-1's answer for a past call: the declarations under its
// cwd when the directory exists (read once per cwd), else the built-in test
// runner list alone; byRoot says which.
func (a *replayAgg) declaredTest(cwd, command string) (declared, byRoot bool) {
	if cwd != "" {
		r, ok := a.roots[cwd]
		if !ok {
			if fi, err := os.Stat(cwd); err == nil && fi.IsDir() {
				r = replayRoot{exists: true, declared: declaredTestCommands(cwd, nil)}
			}
			a.roots[cwd] = r
		}
		if r.exists {
			_, m := matchDeclaredIn(r.declared, command)
			return m, true
		}
	}
	_, m := matchDeclaredIn(a.runners, command)
	return m, false
}

// tally counts one readout into its rule's as-matched or projected view; a
// served readout returns a watch on its dropped-only tokens.
func (a *replayAgg) tally(ro *readout.Readout, projected bool, stdout, command string) *replayWatch {
	rr := a.rules[ro.Rule]
	if rr == nil {
		rr = newReplayRule(ro.Rule, ro.Lossiness.String())
		a.rules[ro.Rule] = rr
	}
	v := &rr.AsMatched
	if projected {
		v = &rr.Projected
	}
	v.Hits++
	v.BytesIn += int64(ro.BytesIn)
	if ro.Reason == readout.ReasonProtected {
		if ro.Stream == readout.StreamContent {
			v.Trips.Content++
		} else {
			v.Trips.Diagnostic++
		}
	}
	if ro.Reason == readout.ReasonProtected || (ro.Stream == readout.StreamContent && droppedProtectable(ro.Dropped)) {
		v.UnscopedTrips++
	}
	if ro.Raw {
		v.Raw[ro.Reason]++
		return nil
	}
	v.Served++
	v.WouldRemove += int64(ro.BytesIn - ro.BytesOut)
	w := &replayWatch{ref: &v.Reference}
	w.dropped, w.suffixes = droppedOnlyTokens(stdout, ro.Stdout, command)
	return w
}

// droppedProtectable: an error or frame line dropped, which a protected-line
// rule not scoped by stream would have tripped on.
func droppedProtectable(cs []readout.ClassCount) bool {
	for _, c := range cs {
		if (c.Class == "error" || c.Class == "frame") && c.N > 0 {
			return true
		}
	}
	return false
}

// shape counts a call under its command heads: its first unit's command
// (with a subcommand for the multi-verb tools), and its effective head, the
// producer of its last segment after the leading quiet ones (design §1's
// table read the producing command).
func (a *replayAgg) shape(c *shellcmd.Command, n int64) {
	head, kind := replayShape(c)
	countShape(a.shapes, head, kind, n)
	countShape(a.effective, replayEffectiveShape(c, a.quiet), replayEffectiveKind(c, a.quiet), n)
	if c.Unreadable {
		return
	}
	for _, u := range c.Units() {
		if u.Kind == shellcmd.KindSimple && u.Name() == "tccutil" && replaySub(u) == "reset" {
			a.rep.Tccutil.Calls++
			if kind != "simple" {
				a.rep.Tccutil.Compound++
			}
			return
		}
	}
}

func countShape(m map[string]*replayShapeRow, head, kind string, n int64) {
	row := m[head]
	if row == nil {
		row = &replayShapeRow{Shape: head}
		m[head] = row
	}
	row.Calls++
	row.Bytes += n
	switch kind {
	case "piped":
		row.Piped++
	case "list":
		row.List++
	default:
		row.Simple++
	}
}

// replayStrip is a readable command's segments past its leading quiet ones,
// never stripping the last; nil when the command is unreadable or empty.
func replayStrip(c *shellcmd.Command, extra []string) []shellcmd.Pipeline {
	if c.Unreadable || len(c.Pipelines) == 0 || len(c.Pipelines[len(c.Pipelines)-1].Units) == 0 {
		return nil
	}
	ps := c.Pipelines
	for len(ps) > 1 && replayQuiet(&ps[0], extra) {
		ps = ps[1:]
	}
	return ps
}

// replayQuiet: a segment the projection strips from a call's front, one
// quiet unit (readout.IsQuiet, the engine's one quiet list, then extra)
// joined by `&&`, `;` or a newline. The engine reads past `&&` alone
// (stripQuietPrefix in internal/readout, RO-28); the projection's wider join is what
// RO-M0 read, so the readings stay comparable and the gap shows.
func replayQuiet(p *shellcmd.Pipeline, extra []string) bool {
	if len(p.Units) != 1 || p.Bang || !(p.Sep == "&&" || p.Sep == ";" || p.Sep == "\n") {
		return false
	}
	return readout.IsQuiet(&p.Units[0], extra)
}

// replayEffectiveShape is a call's effective head: its last segment after
// the leading quiet ones, read by its first unit (the producer).
func replayEffectiveShape(c *shellcmd.Command, extra []string) string {
	ps := replayStrip(c, extra)
	if ps == nil {
		return "(unreadable)"
	}
	return unitHead(&ps[len(ps)-1].Units[0])
}

// replayEffectiveKind is what remains once the quiet segments are
// stripped: a list when more than one segment does, else piped or simple.
func replayEffectiveKind(c *shellcmd.Command, extra []string) string {
	ps := replayStrip(c, extra)
	switch {
	case len(ps) > 1:
		return "list"
	case len(ps) == 1 && len(ps[0].Units) > 1:
		return "piped"
	}
	return "simple"
}

// replayProjected is the command the projected figures render: the
// producer of the last segment after the quiet ones, alone. An unreadable
// command, one already a lone unit, and one carrying an rtk wrapper or a
// FATHOM_READOUT prefix (which the projection would lose) stay as they are.
func replayProjected(c *shellcmd.Command, extra []string) *shellcmd.Command {
	ps := replayStrip(c, extra)
	if ps == nil || c.Rtk() || c.ReadoutPrefix() != "" || (len(c.Pipelines) == 1 && len(c.Pipelines[0].Units) == 1) {
		return c
	}
	last := ps[len(ps)-1]
	return &shellcmd.Command{Pipelines: []shellcmd.Pipeline{{Units: last.Units[:1:1], Sep: last.Sep, Bang: last.Bang}}}
}

// replayReasonClass is a lexer reason with its command text elided: the
// backticked token a reason may carry is kept only when it is a shell
// operator or reserved word, else written `…`, past the nesting prefixes
// the lexer adds.
func replayReasonClass(reason string) string {
	var b strings.Builder
	for {
		p := ""
		for _, q := range []string{"in $(…): ", "in `…`: ", "in shell -c: "} {
			if strings.HasPrefix(reason, q) {
				p = q
			}
		}
		if p == "" {
			break
		}
		b.WriteString(p)
		reason = reason[len(p):]
	}
	if i, j := strings.IndexByte(reason, '`'), strings.LastIndexByte(reason, '`'); i >= 0 && j > i && !replayShellToken(reason[i+1:j]) {
		reason = reason[:i+1] + "…" + reason[j:]
	}
	b.WriteString(reason)
	return b.String()
}

// replayShellToken: a shell operator or reserved word (or the lexer's name
// for a newline), the only tokens a reason class keeps.
func replayShellToken(t string) bool {
	switch t {
	case "&&", "||", ";", ";;", ";&", ";;&", "&", "|", "|&", "!", "(", ")", "((", "))", "{", "}", "[[", "]]",
		"<", ">", ">>", "<<", "<<-", "<<<", "<&", ">&", "<>", ">|", "&>", "&>>", "newline",
		"if", "then", "elif", "else", "fi", "case", "esac", "for", "select", "while", "until", "do", "done",
		"in", "function", "time", "coproc":
		return true
	}
	return false
}

func replayShape(c *shellcmd.Command) (head, kind string) {
	if c.Unreadable || len(c.Pipelines) == 0 || len(c.Pipelines[0].Units) == 0 {
		return "(unreadable)", "simple"
	}
	kind = "simple"
	if len(c.Pipelines) > 1 {
		kind = "list"
	} else if len(c.Pipelines[0].Units) > 1 {
		kind = "piped"
	}
	return unitHead(&c.Pipelines[0].Units[0]), kind
}

func unitHead(u *shellcmd.Unit) string {
	switch u.Kind {
	case shellcmd.KindSubshell:
		return "(subshell)"
	case shellcmd.KindGroup:
		return "(group)"
	case shellcmd.KindCompound:
		return "(" + u.Keyword + ")"
	}
	if len(u.Words) == 0 {
		return "(assignment)"
	}
	name := u.Name()
	if !headWord(name, 40) {
		return "(other)"
	}
	if u.Kind == shellcmd.KindShell {
		return name + " -c"
	}
	if name == "sed" && sedQuiet(u) {
		return "sed -n"
	}
	if replayMultiVerb(name) {
		if s := replaySub(u); s != "" {
			return name + " " + s
		}
	}
	return name
}

// headWord: a command name short and plain enough to be a shape.
func headWord(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '+') {
			return false
		}
	}
	return true
}

// replayMultiVerb names the tools whose shape carries their subcommand.
func replayMultiVerb(name string) bool {
	switch name {
	case "git", "go", "gh", "npm", "pnpm", "yarn", "cargo", "docker", "kubectl", "task", "fathom", "tccutil", "brew", "uv", "pip", "pip3", "terraform", "helm", "launchctl", "codesign":
		return true
	}
	return false
}

// replaySub is a unit's first plain non-flag word after the command (past
// git's -C and -c values), "" when none looks like a subcommand.
func replaySub(u *shellcmd.Unit) string {
	for i := 1; i < len(u.Words); i++ {
		w := u.Words[i]
		if strings.HasPrefix(w.Text, "-") {
			if u.Name() == "git" && (w.Text == "-C" || w.Text == "-c") {
				i++
			}
			continue
		}
		if !w.Literal() || w.Text == "" || w.Text[0] < 'a' || w.Text[0] > 'z' || !headWord(w.Text, 24) || strings.ContainsAny(w.Text, "./+") {
			return ""
		}
		return w.Text
	}
	return ""
}

func sedQuiet(u *shellcmd.Unit) bool {
	for _, w := range u.Words[1:] {
		t := w.Text
		if t == "-n" || t == "--quiet" || t == "--silent" || (len(t) > 1 && t[0] == '-' && t[1] != '-' && strings.Contains(t[1:], "n")) {
			return true
		}
	}
	return false
}

// sedArgs splits a sed unit's words into its script and file operands.
func sedArgs(u *shellcmd.Unit) (script string, files []string) {
	scripted := false
	for i := 1; i < len(u.Words); i++ {
		t := u.Words[i].Text
		switch {
		case t == "-e" || t == "-f":
			if i+1 < len(u.Words) && !scripted {
				script = u.Words[i+1].Text
			}
			scripted = true
			i++
		case strings.HasPrefix(t, "-") && t != "-":
		case !scripted:
			scripted, script = true, t
		default:
			if u.Words[i].Literal() {
				files = append(files, filepath.Clean(t))
			}
		}
	}
	return script, files
}

// sedReadFiles are the files a command's `sed -n` units read.
func sedReadFiles(c *shellcmd.Command) []string {
	if c.Unreadable {
		return nil
	}
	var out []string
	for _, u := range c.Units() {
		if u.Kind == shellcmd.KindSimple && u.Name() == "sed" && sedQuiet(u) {
			_, files := sedArgs(u)
			out = append(out, files...)
		}
	}
	return out
}

// wholeFileRead returns the source files a command prints whole: its last
// pipeline a lone `cat` of source files or `sed -n 1,$p` of them, with no
// redirect (never head or tail, which print ten lines without a count).
func wholeFileRead(c *shellcmd.Command) []string {
	if c.Unreadable || len(c.Pipelines) == 0 {
		return nil
	}
	p := c.Pipelines[len(c.Pipelines)-1]
	if len(p.Units) != 1 {
		return nil
	}
	u := &p.Units[0]
	if u.Kind != shellcmd.KindSimple || len(u.Words) == 0 || len(u.Redirects) > 0 {
		return nil
	}
	var files []string
	switch u.Name() {
	case "cat":
		for _, w := range u.Words[1:] {
			if strings.HasPrefix(w.Text, "-") {
				continue
			}
			if !w.Literal() {
				return nil
			}
			files = append(files, filepath.Clean(w.Text))
		}
	case "sed":
		script, fs := sedArgs(u)
		if !sedQuiet(u) || script != "1,$p" {
			return nil
		}
		files = fs
	default:
		return nil
	}
	for _, f := range files {
		if connector.LanguageForPath(f) == core.LangUnclassified {
			return nil
		}
	}
	return files
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// readPersistedOutput reads up to replayPersistedMax of the harness's saved
// full output.
func readPersistedOutput(p string) (string, bool) {
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, replayPersistedMax))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// resultText is a tool_result's content: a string, or its text blocks.
func resultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if raw[0] == '"' && json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var b strings.Builder
	for _, bl := range blocks {
		if bl.Type == "text" {
			b.WriteString(bl.Text)
		}
	}
	return b.String()
}

func walkJSONStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case string:
		fn(x)
	case []any:
		for _, e := range x {
			walkJSONStrings(e, fn)
		}
	case map[string]any:
		for _, e := range x {
			walkJSONStrings(e, fn)
		}
	}
}

// refTokens adds the paths and identifiers a text names: what a later call
// could reference. A path has a '/' or a dotted extension; an identifier
// has at least four characters and an upper-case letter, digit or
// underscore past its first, so prose words are not counted.
func refTokens(s string, into map[string]struct{}) {
	start := -1
	for i := 0; i <= len(s); i++ {
		if i < len(s) && refByte(s[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			addRefToken(s[start:i], into)
			start = -1
		}
	}
}

func refByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '/' || c == '-'
}

func addRefToken(t string, into map[string]struct{}) {
	t = strings.TrimLeft(t, "-")
	for strings.HasPrefix(t, "./") {
		t = t[2:]
	}
	t = strings.TrimRight(t, ".-/")
	if len(t) < 3 {
		return
	}
	if pathToken(t) || identToken(t) {
		into[t] = struct{}{}
	}
}

func pathToken(t string) bool {
	letter := false
	for i := 0; i < len(t); i++ {
		if c := t[i]; c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			letter = true
			break
		}
	}
	if !letter {
		return false
	}
	if strings.Contains(t, "/") {
		return true
	}
	dot := strings.LastIndexByte(t, '.')
	if dot <= 0 || dot == len(t)-1 || len(t)-dot-1 > 8 {
		return false
	}
	c := t[dot+1]
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func identToken(t string) bool {
	if len(t) < 4 || t[0] >= '0' && t[0] <= '9' {
		return false
	}
	marked := false
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			if i > 0 {
				marked = true
			}
		default:
			return false
		}
	}
	return marked
}

// droppedOnlyTokens are the paths and identifiers the raw output names and
// neither the view nor the command does (the agent already had the
// command's), with each path's '/'-suffixes beside them so a later call
// naming the same file by a shorter or longer path still matches.
func droppedOnlyTokens(raw, view, command string) (exact, suffixes map[string]struct{}) {
	rawT, keep := map[string]struct{}{}, map[string]struct{}{}
	refTokens(raw, rawT)
	refTokens(view, keep)
	refTokens(command, keep)
	var only []string
	for t := range rawT {
		if _, ok := keep[t]; !ok {
			only = append(only, t)
		}
	}
	sort.Strings(only)
	if len(only) > replayTokenMax {
		only = only[:replayTokenMax]
	}
	exact, suffixes = map[string]struct{}{}, map[string]struct{}{}
	for _, t := range only {
		exact[t] = struct{}{}
		for i := 0; i < len(t); i++ {
			if t[i] == '/' && i+1 < len(t) {
				suffixes[t[i+1:]] = struct{}{}
			}
		}
	}
	return exact, suffixes
}

// refersToDropped reports whether a call's tokens name a dropped-only token:
// the same path or identifier, or the same file by a '/'-suffix.
func refersToDropped(tokens, exact, suffixes map[string]struct{}) bool {
	if len(exact) == 0 {
		return false
	}
	for t := range tokens {
		if _, ok := exact[t]; ok {
			return true
		}
		if _, ok := suffixes[t]; ok && strings.ContainsAny(t, "./") {
			return true
		}
		for i := 0; i < len(t); i++ {
			if t[i] == '/' && i+1 < len(t) {
				if _, ok := exact[t[i+1:]]; ok {
					return true
				}
			}
		}
	}
	return false
}

func shareOf(n, of int64) float64 {
	if of <= 0 {
		return 0
	}
	return math.Round(float64(n)/float64(of)*1e6) / 1e6
}

// report finalizes the aggregates in a fixed order.
func (a *replayAgg) report() replayReport {
	rep := a.rep
	total := rep.Bash.Bytes
	rep.WholeFileReads.OverLines = replayLongRead
	rep.WholeFileReads.Share = shareOf(rep.WholeFileReads.Bytes, total)
	rep.Eligible.MinEvaluated, rep.Eligible.MinCleanLower = replayMinEvaluated, replayMinCleanLower
	refs := make([]string, 0, len(a.rules))
	for ref := range a.rules {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	rep.Rules = []replayRule{}
	rep.Eligible.AsMatched.Rules, rep.Eligible.Projected.Rules = []string{}, []string{}
	for _, ref := range refs {
		r := *a.rules[ref]
		r.AsMatched.finish(r.Rule, total, &rep.Eligible.AsMatched)
		r.Projected.finish(r.Rule, total, &rep.Eligible.Projected)
		rep.Rules = append(rep.Rules, r)
	}
	rep.Eligible.AsMatched.Share = shareOf(rep.Eligible.AsMatched.WouldRemove, total)
	rep.Eligible.Projected.Share = shareOf(rep.Eligible.Projected.WouldRemove, total)
	rep.EffectiveShapes = shapeRows(a.effective, total)
	rep.Shapes = shapeRows(a.shapes, total)
	rep.Tccutil.CompoundShare = shareOf(int64(rep.Tccutil.Compound), int64(rep.Tccutil.Calls))
	rep.Unmatched.UnreadableByReason = make([]replayReasonCount, 0, len(a.unreadable))
	for class, n := range a.unreadable {
		rep.Unmatched.UnreadableByReason = append(rep.Unmatched.UnreadableByReason, replayReasonCount{Class: class, Count: n})
	}
	sort.Slice(rep.Unmatched.UnreadableByReason, func(i, j int) bool {
		x, y := rep.Unmatched.UnreadableByReason[i], rep.Unmatched.UnreadableByReason[j]
		if x.Count != y.Count {
			return x.Count > y.Count
		}
		return x.Class < y.Class
	})
	if rep.RefusedRules == nil {
		rep.RefusedRules = []string{}
	}
	a.sizing.report(&rep)
	return rep
}

// finish computes a rule view's Wilson lower bound, eligibility and share,
// adding an eligible rule to its reading's eligible set.
func (v *replayRuleView) finish(rule string, total int64, el *replayEligibleView) {
	lo, _ := ask.Wilson(v.Reference.Of-v.Reference.Referenced, v.Reference.Of)
	v.Reference.CleanWilsonLower = math.Round(lo*1e6) / 1e6
	v.Eligible = v.Reference.Of >= replayMinEvaluated && lo >= replayMinCleanLower
	v.Share = shareOf(v.WouldRemove, total)
	if v.Eligible {
		el.Rules = append(el.Rules, rule)
		el.WouldRemove += v.WouldRemove
	}
}

// shapeRows sorts shape rows by bytes then name, folds all past the top 25
// into "(other)", and computes each row's shares.
func shapeRows(m map[string]*replayShapeRow, total int64) []replayShapeRow {
	rows := make([]replayShapeRow, 0, len(m))
	for _, r := range m {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bytes != rows[j].Bytes {
			return rows[i].Bytes > rows[j].Bytes
		}
		return rows[i].Shape < rows[j].Shape
	})
	if len(rows) > replayTopShapes {
		other := replayShapeRow{Shape: "(other)"}
		for _, r := range rows[replayTopShapes:] {
			other.Calls += r.Calls
			other.Bytes += r.Bytes
			other.Simple += r.Simple
			other.Piped += r.Piped
			other.List += r.List
		}
		rows = append(rows[:replayTopShapes:replayTopShapes], other)
	}
	for i := range rows {
		rows[i].Share = shareOf(rows[i].Bytes, total)
		rows[i].CompoundShare = shareOf(int64(rows[i].Piped+rows[i].List), int64(rows[i].Calls))
	}
	return rows
}

func printReplayReport(w io.Writer, rep replayReport) {
	pct := func(f float64) string { return fmt.Sprintf("%.2f%%", f*100) }
	shapes := func(title string, rows []replayShapeRow) {
		fmt.Fprintln(w, title+"  calls  bytes  share  compound")
		for _, s := range rows {
			fmt.Fprintf(w, "%s  %d  %d  %s  %s\n", s.Shape, s.Calls, s.Bytes, pct(s.Share), pct(s.CompoundShare))
		}
	}
	fmt.Fprintf(w, "readout replay: %d Bash calls, %d bytes, %s to %s (last %d days); %d projects, %d sessions; %d duplicates skipped, %d unpaired, %d failures\n",
		rep.Bash.Calls, rep.Bash.Bytes, readoutOrNone(rep.Window.From), readoutOrNone(rep.Window.To), rep.Window.Days,
		rep.Corpus.Projects, rep.Corpus.Sessions, rep.Bash.Duplicates, rep.Bash.Unpaired, rep.Bash.Failures)
	fmt.Fprintf(w, "projection: quiet segments %s; %s\n", strings.Join(rep.Projection.Quiet, ", "), rep.Projection.Note)
	shapes("effective shape (last segment after the quiet ones)", rep.EffectiveShapes)
	fmt.Fprintf(w, "whole-file source reads over %d lines: %d calls, %d bytes, %s of Bash bytes\n",
		rep.WholeFileReads.OverLines, rep.WholeFileReads.Calls, rep.WholeFileReads.Bytes, pct(rep.WholeFileReads.Share))
	fmt.Fprintln(w, "rule  view  hits  served  would-remove  share  trips content/diagnostic  unscoped  referenced/of  clean-lower  eligible")
	for _, r := range rep.Rules {
		for _, v := range []struct {
			name string
			v    replayRuleView
		}{{"as-matched", r.AsMatched}, {"projected", r.Projected}} {
			fmt.Fprintf(w, "%s  %s  %d  %d  %d  %s  %d/%d  %d  %d/%d  %.3f  %t\n", r.Rule, v.name, v.v.Hits, v.v.Served, v.v.WouldRemove, pct(v.v.Share),
				v.v.Trips.Content, v.v.Trips.Diagnostic, v.v.UnscopedTrips, v.v.Reference.Referenced, v.v.Reference.Of, v.v.Reference.CleanWilsonLower, v.v.Eligible)
		}
	}
	fmt.Fprintf(w, "eligible classes (%d or more evaluated, clean lower bound %.2f or more): as-matched %d, would remove %d bytes, %s of Bash bytes; projected %d, would remove %d bytes, %s\n",
		rep.Eligible.MinEvaluated, rep.Eligible.MinCleanLower, len(rep.Eligible.AsMatched.Rules), rep.Eligible.AsMatched.WouldRemove, pct(rep.Eligible.AsMatched.Share),
		len(rep.Eligible.Projected.Rules), rep.Eligible.Projected.WouldRemove, pct(rep.Eligible.Projected.Share))
	shapes("first-unit shape", rep.Shapes)
	fmt.Fprintf(w, "tccutil reset: %d calls, %s compound\n", rep.Tccutil.Calls, pct(rep.Tccutil.CompoundShare))
	fmt.Fprintf(w, "re-read proxy: %d of %d whole-file reads (%d of %d over %d lines) followed by a sed -n of the same file within five calls\n",
		rep.Reread.FollowedBySed, rep.Reread.WholeReads, rep.Reread.LongFollowedBySed, rep.Reread.LongWholeReads, rep.WholeFileReads.OverLines)
	if len(rep.Unmatched.UnreadableByReason) > 0 {
		parts := make([]string, 0, len(rep.Unmatched.UnreadableByReason))
		for _, u := range rep.Unmatched.UnreadableByReason {
			parts = append(parts, fmt.Sprintf("%s %d", u.Class, u.Count))
		}
		fmt.Fprintf(w, "unreadable by reason: %s\n", strings.Join(parts, "; "))
	}
	j := rep.Joins
	fmt.Fprintf(w, "joins: declared test commands matched %d by cwd, %d by the built-in runners; persisted text missed %d; head_stamp missed %d; outline computed %d, missed %d; edited whole-file reads raw %d\n",
		j.DeclaredTestByRoot, j.DeclaredTestByBuiltin, j.PersistedMissed, j.HeadStampMissed, j.OutlineComputed, j.OutlineMissed, j.EditedRaw)
	if len(rep.RefusedRules) > 0 {
		fmt.Fprintf(w, "refused rules: %s\n", strings.Join(rep.RefusedRules, ", "))
	}
	printReplaySizing(w, rep)
}
