package main

// readoutcmd_replay.go — `fathom readout replay`: the readout rules over the
// local Claude Code transcripts, aggregates only (design
// docs/design/readouts.md §8, §9 and §10; the corpus reading the plan's
// first measurement reads). Every Bash call in the window is paired with
// its result, deduplicated by tool-use id (a resumed session copies its
// history into a new file), lexed, measured by shape and rendered through
// the engine in observe; a dropped line's paths and identifiers are matched
// against the session's next five calls. The output carries counts, bytes,
// shares, rule ids and command heads: never command text, output text, a
// path, a session id or a project name.

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
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
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
// ~/.claude/projects; "" when no home resolves.
func defaultReadoutProjectsDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects")
	}
	home := wiringDriftHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
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
	Window         replayWindow     `json:"window"`
	Corpus         replayCorpus     `json:"corpus"`
	Bash           replayBash       `json:"bash"`
	WholeFileReads replayWhole      `json:"whole_file_reads"`
	Rules          []replayRule     `json:"rules"`
	Eligible       replayEligible   `json:"eligible"`
	Shapes         []replayShapeRow `json:"shapes"`
	Tccutil        replayTcc        `json:"tccutil_reset"`
	Reread         replayReread     `json:"reread"`
	Unmatched      map[string]int   `json:"unmatched"`
	RefusedRules   []string         `json:"refused_rules"`
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
	Rule          string         `json:"rule"`
	Lossiness     string         `json:"lossiness"`
	Hits          int            `json:"hits"`
	BytesIn       int64          `json:"bytes_in"`
	Served        int            `json:"served"`
	WouldRemove   int64          `json:"would_remove_bytes"`
	Raw           map[string]int `json:"raw"`
	Trips         replayTrips    `json:"trips"`
	UnscopedTrips int            `json:"unscoped_trips"`
	Reference     replayRef      `json:"reference"`
	Eligible      bool           `json:"eligible"`
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
	MinEvaluated  int      `json:"min_evaluated"`
	MinCleanLower float64  `json:"min_clean_lower"`
	Rules         []string `json:"rules"`
	WouldRemove   int64    `json:"would_remove_bytes"`
	Share         float64  `json:"share_of_bash_bytes"`
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
	rep      replayReport
	rs       *readout.RuleSet
	cutoff   time.Time
	rules    map[string]*replayRule
	shapes   map[string]*replayShapeRow
	seen     map[string]struct{} // tool-use ids already counted
	sessions map[string]struct{}
	from, to time.Time
}

func newReplayAgg(rs *readout.RuleSet, days int, cutoff time.Time) *replayAgg {
	a := &replayAgg{rs: rs, cutoff: cutoff, rules: map[string]*replayRule{}, shapes: map[string]*replayShapeRow{},
		seen: map[string]struct{}{}, sessions: map[string]struct{}{}}
	a.rep.Window.Days = days
	a.rep.Unmatched = map[string]int{}
	for _, r := range rs.Rules {
		a.rules[r.Ref()] = &replayRule{Rule: r.Ref(), Lossiness: readout.Lossiness(r.Stages).String(), Raw: map[string]int{}}
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
// in name order, skipping files last written before the window.
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
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
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
// served readout (is a dropped line referenced?) or a whole-file read (is
// the file then read again with `sed -n`?).
type replaySubject struct {
	idx        int
	rule       *replayRule
	dropped    map[string]struct{}
	suffixes   map[string]struct{}
	referenced bool
	reads      []string
	long       bool
	reread     bool
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
		}
		buf, over = buf[:0], false
		if rerr != nil {
			break
		}
	}
	for _, s := range st.open {
		a.finish(s)
	}
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
	}
	a.feed(st, f)
	st.recent = append(st.recent, f)
	if len(st.recent) > replayRecent {
		st.recent = append(st.recent[:0:0], st.recent[len(st.recent)-replayRecent:]...)
	}
	if b.Name != "Bash" || b.ID == "" {
		return
	}
	pd := replayPending{idx: f.idx, command: command, tsRaw: l.Timestamp, inWindow: true}
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
	if s.rule != nil && !s.referenced {
		s.referenced = refersToDropped(f.tokens, s.dropped, s.suffixes)
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
	if s.rule != nil {
		s.rule.Reference.Of++
		if s.referenced {
			s.rule.Reference.Referenced++
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
	session := l.SessionID
	if session == "" {
		session = fmt.Sprintf("file-%d", a.rep.Corpus.Files)
	}
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
	var stdout string
	switch {
	case b.IsError:
	case structured:
		stdout = *tr.Stdout
		if tr.PersistedOutputPath != "" {
			a.rep.Bash.Persisted++
			if full, ok := readPersistedOutput(tr.PersistedOutputPath); ok {
				stdout = full
				a.rep.Bash.PersistedRead++
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
		a.rep.Unmatched["failure"]++
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
	ro := readout.Render(a.rs, readout.Input{Command: &c, Stdout: stdout, Stderr: stderr, Interrupted: tr.Interrupted,
		Persisted: tr.PersistedOutputPath, Mode: readout.ModeObserve})
	if ro.Rule == "" {
		a.rep.Unmatched[ro.Reason]++
	} else {
		rr := a.rules[ro.Rule]
		if rr == nil {
			rr = &replayRule{Rule: ro.Rule, Lossiness: ro.Lossiness.String(), Raw: map[string]int{}}
			a.rules[ro.Rule] = rr
		}
		rr.Hits++
		rr.BytesIn += int64(ro.BytesIn)
		if ro.Reason == readout.ReasonProtected {
			if ro.Stream == readout.StreamContent {
				rr.Trips.Content++
			} else {
				rr.Trips.Diagnostic++
			}
		}
		if ro.Reason == readout.ReasonProtected || (ro.Stream == readout.StreamContent && droppedProtectable(ro.Dropped)) {
			rr.UnscopedTrips++
		}
		if ro.Raw {
			rr.Raw[ro.Reason]++
		} else {
			rr.Served++
			rr.WouldRemove += int64(ro.BytesIn - ro.BytesOut)
			s.rule = rr
			s.dropped, s.suffixes = droppedOnlyTokens(stdout, ro.Stdout, pd.command)
		}
	}
	if s.rule == nil && len(s.reads) == 0 {
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

// shape counts a call under its command head (design §1's table): the
// first unit's command, with a subcommand for the multi-verb tools.
func (a *replayAgg) shape(c *shellcmd.Command, n int64) {
	head, kind := replayShape(c)
	row := a.shapes[head]
	if row == nil {
		row = &replayShapeRow{Shape: head}
		a.shapes[head] = row
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
	rep.Eligible.Rules = []string{}
	for _, ref := range refs {
		r := *a.rules[ref]
		lo, _ := ask.Wilson(r.Reference.Of-r.Reference.Referenced, r.Reference.Of)
		r.Reference.CleanWilsonLower = math.Round(lo*1e6) / 1e6
		r.Eligible = r.Reference.Of >= replayMinEvaluated && lo >= replayMinCleanLower
		if r.Eligible {
			rep.Eligible.Rules = append(rep.Eligible.Rules, r.Rule)
			rep.Eligible.WouldRemove += r.WouldRemove
		}
		rep.Rules = append(rep.Rules, r)
	}
	rep.Eligible.Share = shareOf(rep.Eligible.WouldRemove, total)
	rows := make([]replayShapeRow, 0, len(a.shapes))
	for _, r := range a.shapes {
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
	rep.Shapes = rows
	rep.Tccutil.CompoundShare = shareOf(int64(rep.Tccutil.Compound), int64(rep.Tccutil.Calls))
	if rep.RefusedRules == nil {
		rep.RefusedRules = []string{}
	}
	return rep
}

func printReplayReport(w io.Writer, rep replayReport) {
	pct := func(f float64) string { return fmt.Sprintf("%.2f%%", f*100) }
	fmt.Fprintf(w, "readout replay: %d Bash calls, %d bytes, %s to %s (last %d days); %d projects, %d sessions; %d duplicates skipped, %d unpaired, %d failures\n",
		rep.Bash.Calls, rep.Bash.Bytes, readoutOrNone(rep.Window.From), readoutOrNone(rep.Window.To), rep.Window.Days,
		rep.Corpus.Projects, rep.Corpus.Sessions, rep.Bash.Duplicates, rep.Bash.Unpaired, rep.Bash.Failures)
	fmt.Fprintf(w, "whole-file source reads over %d lines: %d calls, %d bytes, %s of Bash bytes\n",
		rep.WholeFileReads.OverLines, rep.WholeFileReads.Calls, rep.WholeFileReads.Bytes, pct(rep.WholeFileReads.Share))
	fmt.Fprintln(w, "rule  hits  served  would-remove  trips content/diagnostic  unscoped  referenced/of  clean-lower  eligible")
	for _, r := range rep.Rules {
		fmt.Fprintf(w, "%s  %d  %d  %d  %d/%d  %d  %d/%d  %.3f  %t\n", r.Rule, r.Hits, r.Served, r.WouldRemove,
			r.Trips.Content, r.Trips.Diagnostic, r.UnscopedTrips, r.Reference.Referenced, r.Reference.Of, r.Reference.CleanWilsonLower, r.Eligible)
	}
	fmt.Fprintf(w, "eligible classes (%d or more evaluated, clean lower bound %.2f or more): %d, would remove %d bytes, %s of Bash bytes\n",
		rep.Eligible.MinEvaluated, rep.Eligible.MinCleanLower, len(rep.Eligible.Rules), rep.Eligible.WouldRemove, pct(rep.Eligible.Share))
	fmt.Fprintln(w, "shape  calls  bytes  share  compound")
	for _, s := range rep.Shapes {
		fmt.Fprintf(w, "%s  %d  %d  %s  %s\n", s.Shape, s.Calls, s.Bytes, pct(s.Share), pct(s.CompoundShare))
	}
	fmt.Fprintf(w, "tccutil reset: %d calls, %s compound\n", rep.Tccutil.Calls, pct(rep.Tccutil.CompoundShare))
	fmt.Fprintf(w, "re-read proxy: %d of %d whole-file reads (%d of %d over %d lines) followed by a sed -n of the same file within five calls\n",
		rep.Reread.FollowedBySed, rep.Reread.WholeReads, rep.Reread.LongFollowedBySed, rep.Reread.LongWholeReads, rep.WholeFileReads.OverLines)
	if len(rep.RefusedRules) > 0 {
		fmt.Fprintf(w, "refused rules: %s\n", strings.Join(rep.RefusedRules, ", "))
	}
}
