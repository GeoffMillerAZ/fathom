package main

// edithook_pin.go — the commit pin line (ADR-057 decision 4,
// docs/design/served-not-kept.md section 3, plan docs/plans/served-not-kept.md
// PR-3 to PR-6).
//
// On a `git commit` seen by PostToolUse(Bash) through `fathom edit-hook`, the
// sha comes from git's own `[branch sha]` summary line in the tool output (no
// summary line, no check), and conserve.PinLosses runs from the commit's
// first parent to the commit: the CV-7 guarded set with reach off, since a
// hook opens no store. Each lost pin is one line naming the test, the source
// that names it, the loss kind and where to look at head; at most three
// lines, then a count left to pre-push. Nothing when no pin is lost. The line
// is a fact with a location: never the rule text, never a description of the
// detector, never a waiver or bypass hint (PR-X2).
//
// It runs whether or not the rule was served: a lost pin is a fact about the
// diff. It keeps no state. Only a git commit statement reaches it, so the
// observe-only exit (RT-131) still forks nothing for every other Bash call.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

const (
	// pinBeatKind is the hooks.jsonl beat kind of the pin stage (channel
	// edit-hook).
	pinBeatKind = "pin"
	// pinLineCap bounds one line: every line is under 240 bytes.
	pinLineCap = 240
	// pinLinesMax is how many loss lines one commit gets before the rest
	// is a count left to pre-push.
	pinLinesMax = 3
	// pinBlobCap bounds one blob read through cat-file: a larger object is
	// a read error (the stage skips), never an unbounded allocation in a hook.
	pinBlobCap = 16 << 20
)

// gitSummaryRe matches git's commit summary line: "[main 3f9a1c2] msg",
// "[main (root-commit) 3f9a1c2] msg", "[detached HEAD 3f9a1c2] msg". The
// same shape Claude Code reads its gitOperation.commit.sha from (PR-V1).
var gitSummaryRe = regexp.MustCompile(`(?m)^\[(?:detached HEAD|[^\s\]]+)(?: \(root-commit\))? ([0-9a-f]{4,40})\]`)

// commitPinLine is the pin stage of edit-hook's PostToolUse(Bash) path. It
// returns the text to add to the payload: the lost-pin lines, or "" (no git
// commit, no summary line, nothing lost, or a mode or harness that does not
// put the lines in front of the agent).
func commitPinLine(codec harness.Codec, hook harness.HookInput, armOff bool) string {
	if hook.Tool != harness.ToolShell || hook.FilePath != "" {
		return ""
	}
	at, ok := lastGitCommitDir(hook.Command)
	if !ok {
		return ""
	}
	start := hook.Cwd
	if start == "" {
		start = resolveProjDirFor(codec.ID(), "")
	}
	dir := start
	if at.known {
		dir = resolveBashDir(start, at.dir)
	}
	root := gitToplevelByStat(dir)
	if root == "" {
		return ""
	}
	if pinCheckMode(root) != config.PinCheckFeedback {
		return ""
	}
	shas := gitSummaryShas(hook.ToolOutput)
	var lines []string
	for _, sha := range shas {
		commit, losses, err := commitPinLosses(context.Background(), root, sha)
		if err != nil {
			continue
		}
		lines = append(lines, renderPinLines(losses, commit)...)
	}
	if armOff || codec.ID() != harness.ClaudeCode {
		return ""
	}
	return capPinLines(lines)
}

// pinCheckMode reads [hooks] pin_check for the repository at root, cheapest
// first: a layer that never names pin_check cannot change the default, so
// only a layer that does costs a config.Load. A config that does not load
// leaves the default, record.
func pinCheckMode(root string) string {
	userPath, projPath := config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml")
	if !bytes.Contains(readConfigLayer(userPath), []byte("pin_check")) && !bytes.Contains(readConfigLayer(projPath), []byte("pin_check")) {
		return config.PinCheckRecord
	}
	l, err := config.Load(userPath, projPath)
	if err != nil || l.Config.Hooks.PinCheck == "" {
		return config.PinCheckRecord
	}
	return l.Config.Hooks.PinCheck
}

// gitSummaryShas returns the distinct commit shas git's summary lines in out
// name, in order.
func gitSummaryShas(out string) []string {
	var shas []string
	seen := map[string]bool{}
	for _, m := range gitSummaryRe.FindAllStringSubmatch(out, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			shas = append(shas, m[1])
		}
	}
	return shas
}

// gitCommitAt is where one `git commit` statement ran: dir is bash-path
// text relative to the command's start (or absolute), known false when a
// cd the parser could not follow came first.
type gitCommitAt struct {
	dir   string
	known bool
}

// lastGitCommitDir finds the last `git commit` statement in cmd and where it
// ran, threading cd/pushd/popd and `git -C` the way bashwrite.go threads
// them for a write. ok is false when cmd runs no git commit.
func lastGitCommitDir(cmd string) (gitCommitAt, bool) {
	if !strings.Contains(cmd, "git") || !strings.Contains(cmd, "commit") {
		return gitCommitAt{}, false // the cheap gate: almost every Bash call ends here
	}
	cmd = strings.TrimSpace(blankHeredocBodies(cmd))
	var found []gitCommitAt
	walkGitCommits(cmd, &bwState{cwdKnown: true}, &found)
	if len(found) == 0 {
		return gitCommitAt{}, false
	}
	return found[len(found)-1], true
}

func walkGitCommits(cmd string, st *bwState, found *[]gitCommitAt) {
	for _, s := range splitTopLevelStatements(cmd) {
		if s.subshell {
			inner := *st
			inner.stack = append([]bwStackEntry(nil), st.stack...)
			walkGitCommits(s.text, &inner, found)
			continue
		}
		if at, ok := gitCommitStatement(shellClauseFields(s.text), st); ok {
			*found = append(*found, at)
			continue
		}
		// cd, pushd and popd move st exactly as they do for a write.
		processBashWriteStatement(s.text, st, &bwCollector{seen: map[string]bool{}})
	}
}

// gitCommitStatement reports whether fields (one statement, after any
// wrapper words: NAME=value, env, time, command...) is `git [global
// options] commit ...`, and the directory it ran in: st's cwd moved by each
// -C.
func gitCommitStatement(fields []string, st *bwState) (gitCommitAt, bool) {
	fields = stripCommandWrappers(fields)
	i := 0
	if len(fields) == 0 || path.Base(fields[0]) != "git" {
		return gitCommitAt{}, false
	}
	at := gitCommitAt{dir: st.cwd, known: st.cwdKnown}
	for i++; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "-C":
			if i+1 < len(fields) {
				i++
				if strings.ContainsAny(fields[i], "$`") {
					at.known = false
				} else {
					at.dir = joinBashPath(at.dir, strings.Trim(fields[i], "\"'"))
				}
			}
		case f == "-c" || f == "--git-dir" || f == "--work-tree" || f == "--namespace":
			i++ // the option's value
		case strings.HasPrefix(f, "-"):
		default:
			return at, f == "commit"
		}
	}
	return gitCommitAt{}, false
}

// resolveBashDir turns a bash-path dir (relative to start, absolute, or
// home-rooted) into a filesystem path.
func resolveBashDir(start, dir string) string {
	if strings.HasPrefix(dir, "~") {
		if p := expandBashTilde(dir); p != "" {
			return p
		}
		return start
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(start, filepath.FromSlash(dir))
}

// pinSkip is a reason the stage did not check a commit.
type pinSkip string

func (s pinSkip) Error() string { return "pin check skipped: " + string(s) }

const (
	pinSkipNotFound   pinSkip = "not_found"
	pinSkipRootCommit pinSkip = "root_commit"
)

// commitPinLosses runs conserve.PinLosses over sha's first parent to sha in
// the repository at root, reading every blob through one `git cat-file
// --batch` process and the changed files through one diff-tree. It returns
// the commit's full sha.
func commitPinLosses(ctx context.Context, root, sha string) (string, []conserve.PinLoss, error) {
	batch, err := startCatFileBatch(ctx, root)
	if err != nil {
		return sha, nil, err
	}
	defer batch.close()
	c, err := batch.get(sha + "^{commit}")
	if err != nil {
		return sha, nil, err
	}
	if !c.ok || c.typ != "commit" {
		return sha, nil, pinSkipNotFound
	}
	commit := c.name
	parent := firstParent(c.body)
	if parent == "" {
		return commit, nil, pinSkipRootCommit
	}
	raw, err := gitro.Output(ctx, root, "diff-tree", "-r", "-z", "--name-only", "--no-renames", parent, commit)
	if err != nil {
		return commit, nil, err
	}
	var changed []string
	for _, p := range strings.Split(string(raw), "\x00") {
		if p != "" {
			changed = append(changed, p)
		}
	}
	losses, err := conserve.PinLosses(ctx, conserve.PinLossInput{
		Root: root, Base: parent, Head: commit, ChangedFiles: changed,
		Read: func(rev, p string) ([]byte, bool, error) {
			o, err := batch.get(rev + ":" + p)
			if err != nil {
				return nil, false, err
			}
			if !o.ok || o.typ != "blob" {
				return nil, false, nil
			}
			return o.body, true, nil
		},
	})
	return commit, losses, err
}

// firstParent reads the first `parent` header of a raw commit object.
func firstParent(body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" {
			break // the headers end at the first blank line
		}
		if p, ok := strings.CutPrefix(line, "parent "); ok {
			return strings.TrimSpace(p)
		}
	}
	return ""
}

// catFileBatch is one `git cat-file --batch` process used as a
// request/response pipe: conserve.PinLosses asks for blobs lazily (which
// files it reads depends on what it has already read), so each get writes
// one object name and reads one answer, never the whole request up front.
// Answers are memoized; the first error is sticky.
type catFileBatch struct {
	cmd     *exec.Cmd
	cleanup func()
	stdin   io.WriteCloser
	out     *bufio.Reader
	memo    map[string]catFileObject
	err     error
}

type catFileObject struct {
	name string // the full object name git resolved
	typ  string
	body []byte
	ok   bool // false: git answered "missing" or "ambiguous"
}

func startCatFileBatch(ctx context.Context, dir string) (*catFileBatch, error) {
	cmd, cleanup, err := gitro.Command(ctx, dir, "cat-file", "--batch")
	if err != nil {
		cleanup()
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return nil, err
	}
	return &catFileBatch{cmd: cmd, cleanup: cleanup, stdin: stdin, out: bufio.NewReader(stdout), memo: map[string]catFileObject{}}, nil
}

// get asks for one object ("<rev>", "<rev>^{commit}" or "<rev>:<path>").
func (b *catFileBatch) get(name string) (catFileObject, error) {
	if o, ok := b.memo[name]; ok {
		return o, nil
	}
	if b.err != nil {
		return catFileObject{}, b.err
	}
	if strings.ContainsAny(name, "\n\x00") {
		return catFileObject{}, fmt.Errorf("cat-file: object name %q has a newline", name)
	}
	fail := func(err error) (catFileObject, error) {
		b.err = err
		return catFileObject{}, err
	}
	if _, err := io.WriteString(b.stdin, name+"\n"); err != nil {
		return fail(err)
	}
	header, err := b.out.ReadString('\n')
	if err != nil {
		return fail(err)
	}
	header = strings.TrimSuffix(header, "\n")
	if strings.HasSuffix(header, " missing") || strings.HasSuffix(header, " ambiguous") {
		b.memo[name] = catFileObject{}
		return catFileObject{}, nil
	}
	f := strings.Fields(header)
	if len(f) != 3 {
		return fail(fmt.Errorf("cat-file: unexpected header %q", header))
	}
	n, err := strconv.Atoi(f[2])
	if err != nil || n < 0 || n > pinBlobCap {
		return fail(fmt.Errorf("cat-file: object %s size %q out of bounds", f[0], f[2]))
	}
	body := make([]byte, n+1) // the body, then its newline
	if _, err := io.ReadFull(b.out, body); err != nil {
		return fail(err)
	}
	o := catFileObject{name: f[0], typ: f[1], body: body[:n], ok: true}
	b.memo[name] = o
	return o, nil
}

func (b *catFileBatch) close() {
	_ = b.stdin.Close()
	_ = b.cmd.Wait()
	b.cleanup()
}

// renderPinLines is one line per loss, in PinLosses' order.
func renderPinLines(losses []conserve.PinLoss, commit string) []string {
	short := commit
	if len(short) > 7 {
		short = short[:7]
	}
	out := make([]string, 0, len(losses))
	for _, l := range losses {
		out = append(out, renderPinLine(l, short))
	}
	return out
}

// renderPinLine is one lost pin: kind, test, naming source, where to look at
// head, and whether the same commit edited the naming source. Under
// pinLineCap bytes: a longer line drops the -run clause, then the source's
// reason, then is cut.
func renderPinLine(l conserve.PinLoss, short string) string {
	cite, bare := "its governing source", "its governing source"
	if len(l.Sources) > 0 {
		s := l.Sources[0]
		cite, bare = s.Cite(), s.Path
		if s.Line > 0 {
			bare = fmt.Sprintf("%s:%d", s.Path, s.Line)
		}
		if n := len(l.Sources) - 1; n > 0 {
			more := fmt.Sprintf(" +%d", n)
			cite, bare = cite+more, bare+more
		}
	}
	var where, run string
	switch l.Kind {
	case "deleted":
		run = " · a -run on this name now matches no test"
		switch {
		case l.ReplacedBy != "":
			where = fmt.Sprintf("gone in %s, replaced by %s (%s:%d)", short, l.ReplacedBy, l.HeadFile, l.HeadLine)
		case l.HeadFile != "":
			where = fmt.Sprintf("gone from %s in %s", l.HeadFile, short)
		default:
			where = fmt.Sprintf("gone with %s in %s", l.BaseFile, short)
		}
	case "renamed":
		run = " · a -run on this name now matches no test"
		where = fmt.Sprintf("now %s (%s:%d) in %s", l.RenamedTo, l.HeadFile, l.HeadLine, short)
	default:
		where = fmt.Sprintf("%s:%d in %s", l.HeadFile, l.HeadLine, short)
	}
	note := ""
	if l.SourceEdited {
		note = " (its source was edited in this commit)"
	}
	core := func(named string) string {
		return fmt.Sprintf("fathom pin %s · %s (named by %s) · %s", l.Kind, l.Test, named, where)
	}
	for _, cand := range []string{core(cite) + run + note, core(cite) + note, core(bare) + note} {
		if len(cand) < pinLineCap {
			return cand
		}
	}
	return clipBytes(core(bare), pinLineCap-1-len("…")-len(note)) + "…" + note
}

// capPinLines joins at most pinLinesMax lines and counts the rest.
func capPinLines(lines []string) string {
	if len(lines) > pinLinesMax {
		n := len(lines) - pinLinesMax
		lines = append(lines[:pinLinesMax:pinLinesMax], fmt.Sprintf("fathom pin +%d more at pre-push", n))
	}
	return strings.Join(lines, "\n")
}

// emitPinContext prints text alone as the payload (no other stage of this
// call produced one); "" prints nothing.
func emitPinContext(codec harness.Codec, text string, stdout io.Writer) {
	if text == "" {
		return
	}
	out, err := codec.EncodeOutput(harness.EventPostTool, harness.HookOutput{Context: text})
	if err != nil || len(out) == 0 {
		return
	}
	fmt.Fprintln(stdout, string(out))
}

// errIsPinSkip reports err's skip reason, if it is one.
func errIsPinSkip(err error) (pinSkip, bool) {
	var s pinSkip
	if errors.As(err, &s) {
		return s, true
	}
	return "", false
}
