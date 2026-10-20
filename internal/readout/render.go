package readout

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// DefaultCap is the harness's output cap in characters (Claude Code's
// BASH_MAX_OUTPUT_LENGTH default).
const DefaultCap = 30000

// Input is everything Render reads. The caller lexes the command and
// computes the joins; Render does no I/O.
type Input struct {
	Command     *shellcmd.Command
	Stdout      string
	Stderr      string
	Interrupted bool
	// Cap is the harness's output cap in characters; 0 is DefaultCap.
	Cap int
	// Persisted is the harness's persistedOutputPath, when it saved
	// over-cap output.
	Persisted string
	// PersistedText is that file's text, when the caller read it inside the
	// joins budget; counts (grep's matches, git log's commits, a cap's
	// per-group counts) come from it. "" when Persisted is unset or the read
	// missed: counts are then labelled as over the shown text (design §6.7).
	PersistedText string
	// DeclaredTest is the test-run matcher's answer (TR-1,
	// matchDeclaredTestCommand in cmd/fathom): the command runs a declared
	// test command. A rule with match.declared_test accepts it.
	DeclaredTest bool
	// Saved is the hook's saved raw file (RO-22), when one exists.
	Saved string
	// Mode is the configured mode; "" serves each rule in its own mode. A
	// FATHOM_READOUT=lean or rich prefix overrides both.
	Mode string
	// Joins are the built-in joins' lines, by name, computed by the caller.
	Joins map[string][]string
	// Quiet names extra quiet heads ([readouts] quiet_prefixes): a leading
	// `&&`-joined segment one names is read past like the built-in ones.
	Quiet []string
	// Edited is the caller's answer for a whole-file read (match.whole_read):
	// the file it prints was edited this session or differs from HEAD, so
	// the read is verification and stays raw (ReasonEdited).
	Edited bool
}

// Why a readout serves the raw output.
const (
	ReasonUnreadable   = "unreadable"
	ReasonPrefixRaw    = "prefix-raw"
	ReasonRtk          = "rtk"
	ReasonInterrupted  = "interrupted"
	ReasonMachineBound = "machine-bound"
	ReasonNoRule       = "no-rule"
	ReasonBelowFloor   = "below-floor"
	ReasonProtected    = "protected"
	ReasonNoRecovery   = "no-recovery"
	ReasonNeverWorse   = "never-worse"
	// ReasonInverse: a lossless stage did not round-trip on this output, so
	// its view is not served as lossless (design §6.3; group checks itself,
	// so this is a backstop).
	ReasonInverse = "inverse-failed"
	// ReasonEdited: a whole-file read of a file the session edited (or that
	// differs from HEAD) stays raw (Input.Edited).
	ReasonEdited = "edited"
)

// Trip is a protected line a rule's stages would have dropped.
type Trip struct {
	Line  int    // 1-based input line
	Class string // exit, notice, error or frame
}

// ClassCount is one class's dropped-line count.
type ClassCount struct {
	Class string
	N     int
}

// Readout is Render's result: the view the agent would see and what the
// hook row records.
type Readout struct {
	// Rule is the matched rule's id@version, "" when none matched.
	Rule   string
	Mode   string
	Stream Stream
	// Raw is true when the view's body is the raw stdout; Reason and
	// Detail say why.
	Raw    bool
	Reason string
	Detail string
	// Stdout is the view: the readout ending in its receipt, or the raw
	// stdout (followed by the trip's receipt on a protected trip).
	Stdout string
	// Stderr is always the raw stderr: diagnostic, never staged.
	Stderr    string
	Receipt   string
	Lossiness LossClass
	Records   []Record
	Trips     []Trip
	Dropped   []ClassCount
	// Columns counts lines whose columns were dropped; Summarized, lines
	// folded into a summary.
	Columns     int
	Summarized  int
	JoinsMissed []string
	BytesIn     int
	BytesOut    int
	// BaseBytes is the stdout's size after the base stages: a rule's saving
	// is measured from here, never from BytesIn (RO-13).
	BaseBytes int
	LinesIn   int
	LinesOut  int
}

// Render is the engine (RO-5): a pure function over the lexed command,
// stdout, stderr, the interrupted flag and the harness cap. It classifies,
// matches a rule, runs its stages, then the records, then the local joins,
// then the contract (protected lines by stream), then never-worse. The same
// input gives the same bytes.
func Render(rs *RuleSet, in Input) Readout {
	out := Readout{Raw: true, Stdout: in.Stdout, Stderr: in.Stderr, BytesIn: len(in.Stdout)}
	ls, _ := splitLines(in.Stdout)
	out.LinesIn = len(ls)
	out.BytesOut, out.LinesOut = out.BytesIn, out.LinesIn
	raw := func(reason, detail string) Readout {
		out.Reason, out.Detail = reason, detail
		return out
	}
	c := in.Command
	if c == nil || c.Unreadable {
		detail := ""
		if c != nil {
			detail = c.Reason
		}
		return raw(ReasonUnreadable, detail)
	}
	prefix := c.ReadoutPrefix()
	if prefix == "raw" {
		return raw(ReasonPrefixRaw, "")
	}
	if c.Rtk() {
		return raw(ReasonRtk, "")
	}
	if in.Interrupted {
		return raw(ReasonInterrupted, "")
	}
	sh := classifyCommand(c, in.Quiet)
	sh.declared = in.DeclaredTest
	if sh.machine != "" {
		return raw(ReasonMachineBound, sh.machine)
	}
	var rule *Rule
	if rs != nil {
		for i := len(rs.Rules) - 1; i >= 0; i-- {
			r := rs.Rules[i]
			if r.Mode != ModeOff && r.Match.matches(sh) {
				rule = r
				break
			}
		}
	}
	if rule == nil {
		return raw(ReasonNoRule, sh.compound)
	}
	out.Rule = rule.Ref()
	out.Mode = effectiveMode(rule, in.Mode, prefix)
	if rule.untrusted {
		out.Mode = ModeObserve
	}
	out.Stream = streamOf(sh)
	out.Lossiness = rule.loss
	if (rule.When.BytesGT > 0 && len(in.Stdout) <= rule.When.BytesGT) || (rule.When.LinesGT > 0 && len(ls) <= rule.When.LinesGT) {
		return raw(ReasonBelowFloor, "")
	}
	if rule.Match.WholeRead && in.Edited {
		return raw(ReasonEdited, "")
	}
	capChars := in.Cap
	if capChars <= 0 {
		capChars = DefaultCap
	}
	env := runEnv{
		joins:         in.Joins,
		persisted:     in.Persisted,
		persistedText: in.PersistedText,
		saved:         in.Saved,
		truncated:     in.Persisted != "" || utf8.RuneCountInString(in.Stdout) >= capChars,
		capChars:      capChars,
		file:          "",
		offset:        0,
	}
	if files, off, ok := readOperands(sh.unit); ok && len(files) == 1 && files[0].Literal() {
		env.file, env.offset = files[0].Text, off
	}
	// The quiet segments' own lines lead stdout and stay verbatim ahead of
	// the rule's view; the rule reads the producer's part (RO-28).
	head, body := quietOutput(sh.quiet, in.Stdout)
	shift := strings.Count(head, "\n")
	res := run(rule, body, out.Stream, env)
	res.shift(shift)
	out.BaseBytes = len(head) + len(res.baseText())
	out.Records, out.Trips, out.JoinsMissed = res.records, res.trips, res.missed
	out.Dropped, out.Columns, out.Summarized = res.droppedCounts(), res.columns, res.summarized
	if res.truncated {
		env.truncated = true
	}
	if res.invErr != nil {
		return raw(ReasonInverse, res.invErr.Error())
	}
	if len(res.trips) > 0 {
		out.Receipt = tripReceipt(rule, out.Mode, res.trips)
		out.Stdout = in.Stdout + newlineIfNeeded(in.Stdout) + out.Receipt + "\n"
		out.BytesOut = len(out.Stdout)
		return raw(ReasonProtected, "")
	}
	if rule.loss != LossNone && out.Mode != ModeObserve && !recoveryAvailable(rule, env, res) {
		return raw(ReasonNoRecovery, rule.Recovery)
	}
	out.Receipt = receipt(rule, out.Mode, res, env)
	view := head + res.body() + out.Receipt + "\n"
	if out.Mode != ModeRich && len(view) >= out.BaseBytes {
		return raw(ReasonNeverWorse, "")
	}
	out.Raw = false
	out.Stdout = view
	out.BytesOut, out.LinesOut = len(view), shift+len(res.lines)
	return out
}

// shift moves a run's input line numbers (trips, records) down n lines:
// the quiet segments' lines that led the output it read.
func (r *result) shift(n int) {
	if n == 0 {
		return
	}
	for i := range r.trips {
		r.trips[i].Line += n
	}
	for i := range r.records {
		r.records[i].Line += n
	}
}

func newlineIfNeeded(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return ""
	}
	return "\n"
}

func effectiveMode(r *Rule, configured, prefix string) string {
	switch prefix {
	case ModeLean, ModeRich:
		return prefix
	}
	if configured != "" {
		return configured
	}
	if r.Mode != "" {
		return r.Mode
	}
	return ModeObserve
}

// recoveryAvailable reports whether the rule's recovery exists for this call.
func recoveryAvailable(r *Rule, env runEnv, res result) bool {
	switch r.Recovery {
	case RecoveryRepeat:
		// A rule that joins head_stamp repeats at a stamped HEAD (design
		// §6.4): no stamp, no reproducible repeat.
		return !r.joins(joinHeadStamp) || res.stamp != ""
	case RecoverySpan:
		return env.file != "" && env.offset >= 0
	case RecoveryPersisted:
		return env.persisted != ""
	case RecoverySaved:
		return env.saved != ""
	}
	return false
}

// runEnv is the per-call context a rule run reads.
type runEnv struct {
	joins         map[string][]string
	persisted     string
	persistedText string
	saved         string
	truncated     bool
	capChars      int
	file          string // a read's single file operand, for span recipes
	offset        int    // the file line of the first shown line, minus one; -1 unknown
}

// result is one rule run over one output.
type result struct {
	lines      []line
	nl         bool
	in         []string
	trips      []Trip
	dropped    [classCount]int
	dropIdx    []int
	columns    int
	summarized int
	records    []Record
	missed     []string
	truncated  bool
	// base is the lines after the base stages; counts are the receipt's
	// count clauses; stamp is the head_stamp join's line.
	base   []line
	counts []string
	stamp  string
	invErr error
	// contentLost: an error-word or path:line line, unprotected in a
	// content stream, was not kept exact.
	contentLost bool
}

func (r *result) body() string {
	if len(r.lines) == 0 {
		return ""
	}
	return joinLines(texts(r.lines), true)
}

func (r *result) baseText() string {
	if len(r.base) == 0 {
		return ""
	}
	return joinLines(texts(r.base), true)
}

func (r *result) droppedCounts() []ClassCount {
	var out []ClassCount
	for _, c := range classOrder {
		if n := r.dropped[c]; n > 0 {
			out = append(out, ClassCount{Class: c.String(), N: n})
		}
	}
	return out
}

// run applies a rule's stages, then its joins, then checks every input
// line's fate against the contract. Lossless stages are checked against
// their inverse on the way (invErr).
func run(r *Rule, stdout string, stream Stream, env runEnv) result {
	in, nl := splitLines(stdout)
	res := result{nl: nl, in: in}
	shown := make([]string, len(in))
	classes := make([]lineClass, len(in))
	for i, t := range in {
		shown[i] = renderTerminal(t)
		classes[i] = classify(shown[i])
		if classes[i] == classNotice {
			res.truncated = true
		}
	}
	ctx := &runCtx{joins: env.joins, label: "in the shown " + thousands(env.capChars) + " chars"}
	if env.persistedText != "" {
		ps, _ := splitLines(env.persistedText)
		ctx.persisted = make([]string, len(ps))
		for i, t := range ps {
			ctx.persisted[i] = renderTerminal(t)
		}
	} else {
		ctx.cut = env.truncated || res.truncated
	}
	ls := inputLines(in)
	res.base = baseStages(ls)
	res.invErr = baseRoundTrip(ls, res.base)
	ls = res.base
	var joins []*Stage
	for i := range r.Stages {
		s := &r.Stages[i]
		if s.Kind == StageJoin {
			joins = append(joins, s)
			continue
		}
		next := s.apply(ls, ctx)
		if stageLoss[s.Kind] == LossNone && res.invErr == nil {
			res.invErr = s.roundTrip(ls, next)
		}
		ls = next
	}
	for _, s := range joins {
		next := s.apply(ls, ctx)
		if res.invErr == nil {
			res.invErr = s.roundTrip(ls, next)
		}
		ls = next
	}
	res.lines, res.records, res.missed, res.stamp = ls, ctx.records, ctx.missed, ctx.stamp
	best := make([]int, len(in)) // 0 absent, 1 summary, 2 columns, 3 exact
	for _, l := range ls {
		rank := 3 - int(l.fate)
		for _, i := range l.src {
			if rank > best[i] {
				best[i] = rank
			}
		}
	}
	for i, c := range classes {
		if best[i] == 3 {
			continue
		}
		if protected(c, stream) {
			res.trips = append(res.trips, Trip{Line: i + 1, Class: c.String()})
			continue
		}
		if c == classError || c == classFrame {
			res.contentLost = true
		}
		switch best[i] {
		case 0:
			res.dropped[c]++
			res.dropIdx = append(res.dropIdx, i)
		case 1:
			res.summarized++
			res.dropIdx = append(res.dropIdx, i)
		case 2:
			res.columns++
		}
	}
	for i := range r.Stages {
		if s := &r.Stages[i]; s.Kind == StageExtract && s.tmpl != nil {
			if c := countClause(s, shown, best, ctx); c != "" {
				res.counts = append(res.counts, c)
			}
		}
	}
	return res
}

// countClause is an extract stage's count in the receipt: its template
// with {count} the matching lines and {groups} their distinct keys, over
// the persisted output when the caller read it, else over the shown
// output and labelled when that is cut (never a total over cut output),
// then how many of the shown ones the readout dropped. "" when nothing
// matches.
func countClause(s *Stage, shown []string, best []int, ctx *runCtx) string {
	count := func(ls []string) (int, int) {
		n, groups := 0, map[string]bool{}
		for _, t := range ls {
			if !s.re.MatchString(t) {
				continue
			}
			n++
			if s.key != nil {
				groups[s.prefix(t)] = true
			}
		}
		return n, len(groups)
	}
	n, g := count(shown)
	dropped := 0
	for i, t := range shown {
		if best[i] < 2 && s.re.MatchString(t) {
			dropped++
		}
	}
	if ctx.persisted != nil {
		n, g = count(ctx.persisted)
	}
	if n == 0 {
		return ""
	}
	c := renderTemplate(s.tmpl, map[string]string{"count": strconv.Itoa(n), "groups": strconv.Itoa(g)})
	switch {
	case ctx.persisted != nil:
		return fmt.Sprintf("%s in the persisted output; %d of the shown dropped", c, dropped)
	case ctx.cut:
		return fmt.Sprintf("%s %s, %d dropped", c, ctx.label, dropped)
	}
	return fmt.Sprintf("%s, %d dropped", c, dropped)
}

// joins reports whether the rule has a join stage named j.
func (r *Rule) joins(j string) bool {
	for _, s := range r.Stages {
		if s.Kind == StageJoin && s.Join == j {
			return true
		}
	}
	return false
}

// receipt is the readout's last line (design §6): rule@version, mode, line
// counts, what dropped by class, and the recovery.
func receipt(r *Rule, mode string, res result, env runEnv) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d→%d lines", len(res.base), len(res.lines)))
	total := 0
	var by []string
	for _, c := range res.droppedCounts() {
		total += c.N
		by = append(by, fmt.Sprintf("%s %d", c.Class, c.N))
	}
	lossless := total == 0 && res.columns == 0 && res.summarized == 0
	if lossless {
		parts = append(parts, "nothing dropped")
	}
	if total > 0 {
		parts = append(parts, fmt.Sprintf("dropped %d (%s)", total, strings.Join(by, ", ")))
	}
	if res.columns > 0 {
		parts = append(parts, fmt.Sprintf("columns dropped on %d lines", res.columns))
	}
	if res.summarized > 0 {
		parts = append(parts, fmt.Sprintf("%d lines summarized", res.summarized))
	}
	parts = append(parts, res.counts...)
	if (env.truncated || res.truncated) && env.persistedText == "" && len(res.counts) == 0 {
		parts = append(parts, "counts in the shown "+thousands(env.capChars)+" chars")
	}
	for _, j := range res.missed {
		parts = append(parts, "join "+j+": not computed")
	}
	if !lossless {
		parts = append(parts, recoveryClause(r, res, env))
	}
	return fmt.Sprintf("[readout %s %s: %s]", r.Ref(), mode, strings.Join(parts, "; "))
}

func recoveryClause(r *Rule, res result, env runEnv) string {
	switch r.Recovery {
	case RecoveryRepeat:
		if res.stamp != "" {
			return "raw: repeat the command at " + res.stamp
		}
		return "raw: repeat the command"
	case RecoverySpan:
		return "raw: " + spanRecipes(res.dropIdx, env)
	case RecoveryPersisted:
		if env.persisted == "" {
			return "raw: not persisted"
		}
		return "raw: " + env.persisted
	case RecoverySaved:
		if env.saved == "" {
			return "raw: not saved"
		}
		return "raw saved: " + env.saved
	}
	return "raw: no recovery"
}

// spanRecipes renders `sed -n A,Bp <file>` for the dropped ranges, at most
// three, in file line numbers.
func spanRecipes(idx []int, env runEnv) string {
	file := env.file
	if file == "" {
		file = "<file>"
	}
	off := env.offset
	if off < 0 {
		off = 0
	}
	type span struct{ a, b int }
	var spans []span
	for _, i := range idx {
		n := i + 1 + off
		if k := len(spans) - 1; k >= 0 && spans[k].b == n-1 {
			spans[k].b = n
			continue
		}
		spans = append(spans, span{n, n})
	}
	var out []string
	for i, s := range spans {
		if i == 3 {
			out = append(out, fmt.Sprintf("+%d more ranges", len(spans)-3))
			break
		}
		out = append(out, fmt.Sprintf("sed -n %d,%dp %s", s.a, s.b, file))
	}
	if len(out) == 0 {
		return "nothing to recover"
	}
	return strings.Join(out, ", ")
}

func tripReceipt(r *Rule, mode string, trips []Trip) string {
	t := trips[0]
	s := fmt.Sprintf("[readout %s %s: raw served; protected line %d (%s) would drop", r.Ref(), mode, t.Line, t.Class)
	if n := len(trips) - 1; n > 0 {
		s += fmt.Sprintf("; +%d more", n)
	}
	return s + "]"
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}
