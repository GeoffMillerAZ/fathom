package main

// supervise.go -- the process boundary of fathom-lang extract (ADR-092, the read guarantee).
//
// pkg/lang returns by a deadline, but Go cannot interrupt a goroutine and a fatal stack overflow or a parser that
// allocates without bound ends or starves the process that runs it. So extract reads in a child: this same binary,
// started with a hidden flag. The parent writes one header line (the sibling list) and then one path per line to
// the child's standard input, and the child writes one JSON line per file, after a first line that says it is
// ready. The parent sends a path only after the answer to the last one, so the file in hand is always known.
//
// The parent kills and restarts the child when
//   - a file takes longer than the read deadline plus killMargin (a hang the library's deadline could not end),
//   - the child ends in the middle of a file, by any cause, including a fatal stack overflow,
//   - the child's own watchdog, which polls the Go runtime's memory about every 100 ms, finds it past
//     -max-memory and exits with exitMemory, and
//   - the child answered a file by abandoning its parser at the deadline (the parser still burns a core there).
//
// The file in hand then reads Incomplete with the reason in its notes, the batch goes on with the next file, and
// the output of a batch that faults nowhere is byte-equal to -in-process (TestSupervisedOutputEqualsInProcess).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/readlimit"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

const (
	// defaultMaxMemory is the memory the child may hold.
	defaultMaxMemory = "4GiB"
	// exitMemory is the exit status of a child whose watchdog found it past -max-memory. No other path exits with it.
	exitMemory = 75
	// memoryPoll is how often the child's watchdog reads the runtime's memory.
	memoryPoll = 100 * time.Millisecond
	// killedNotePrefix starts the one note of a file whose read ended the child or passed its time.
	killedNotePrefix = "read-killed: "
)

// The supervisor's timings, variables so a test can shorten them.
var (
	// killMargin is how long past the read deadline the parent waits for a file before it kills the child.
	killMargin = 10 * time.Second
	// startupTimeout is how long the parent waits for a child to build its Reader.
	startupTimeout = 2 * time.Minute
	// childCommand builds the command that starts the child with args, which ctx ends. A test replaces it to
	// re-execute the test binary.
	childCommand = func(ctx context.Context, args []string) (*exec.Cmd, error) {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		return exec.CommandContext(ctx, exe, args...), nil
	}
	// waitGrace is how long a child that is ending, or was told to, has to exit before it is killed.
	waitGrace = 5 * time.Second
)

// parseSize reads a byte count: a number, with an optional K, M or G (or KiB, MiB, GiB) suffix, all powers of 1024.
func parseSize(s string) (uint64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	mult := uint64(1)
	for _, u := range []struct {
		suffix string
		mult   uint64
	}{{"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(t, u.suffix) {
			t, mult = strings.TrimSpace(strings.TrimSuffix(t, u.suffix)), u.mult
			break
		}
	}
	n, err := strconv.ParseUint(t, 10, 64)
	if err != nil || n == 0 || n > (1<<62)/mult {
		return 0, fmt.Errorf("-max-memory %q is not a size such as 512MiB or 4GiB", s)
	}
	return n * mult, nil
}

// childHeader is the first line the parent writes to the child.
type childHeader struct {
	Siblings []string `json:"siblings"`
}

// childHello is the first line the child writes: ready, or why it cannot be.
type childHello struct {
	Ready bool   `json:"ready"`
	Fatal string `json:"fatal"`
}

// runChild is the child's half: build the Reader, say so, and answer one path per line until standard input ends.
func runChild(ctx context.Context, cfg readConfig, root string, memory uint64, in io.Reader, out io.Writer) error {
	watchMemory(memory, func() { os.Exit(exitMemory) })
	br := bufio.NewReaderSize(in, 1<<20)
	bw := bufio.NewWriterSize(out, 1<<20)
	enc := json.NewEncoder(bw)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("supervised child: no header: %w", err)
	}
	var hdr childHeader
	if err := json.Unmarshal(line, &hdr); err != nil {
		return fmt.Errorf("supervised child: bad header: %w", err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	r, err := lang.New(cfg.options()...)
	if err != nil {
		enc.Encode(childHello{Fatal: err.Error()})
		bw.Flush()
		return err
	}
	defer r.Close()
	enc.Encode(childHello{Ready: true})
	if err := bw.Flush(); err != nil {
		return err
	}
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var p string
			if jerr := json.Unmarshal(line, &p); jerr != nil {
				return fmt.Errorf("supervised child: bad path line: %w", jerr)
			}
			fr, rerr := readOne(ctx, r, rootAbs, hdr.Siblings, p)
			if rerr != nil {
				return rerr
			}
			if err := enc.Encode(fr); err != nil {
				return err
			}
			if err := bw.Flush(); err != nil {
				return err
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// watchMemory polls the runtime's memory about every memoryPoll and calls exit once the process holds more than
// limit bytes from the operating system (the runtime's Sys less what it has handed back). It is the child's own
// limit: the parent cannot read a child's memory without a platform call, and a child past its limit has little
// time left to be asked.
func watchMemory(limit uint64, exit func()) {
	var m runtime.MemStats
	var poll func()
	poll = func() {
		runtime.ReadMemStats(&m)
		if m.Sys-m.HeapReleased > limit {
			exit()
			return
		}
		time.AfterFunc(memoryPoll, poll) // a timer callback, so no goroutine of this package outlives the run
	}
	time.AfterFunc(memoryPoll, poll)
}

// extractSupervised reads every path through a child the parent kills and restarts as the file in hand makes it fail.
func extractSupervised(ctx context.Context, cfg readConfig, rootAbs, cache string, memory uint64, sibs, paths []string) ([]fileResult, bool, error) {
	sup := &supervisor{cfg: cfg, root: rootAbs, cache: cache, memory: memory, sibs: sibs}
	defer sup.stop()
	files := []fileResult{}
	unreadable := false
	for _, p := range paths {
		fr, err := sup.read(ctx, p)
		if err != nil {
			return nil, false, err
		}
		unreadable = unreadable || fr.Error != ""
		files = append(files, fr)
	}
	return files, unreadable, nil
}

// supervisor owns the current child, if there is one.
type supervisor struct {
	cfg    readConfig
	root   string
	cache  string
	memory uint64
	sibs   []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    *bufio.Reader
	stderr *headBuffer
}

// line reads the child's next output line, waiting at most d. It is the child that ends a wait that times out:
// the caller kills it, which closes the pipe the read is blocked on.
func (s *supervisor) line(ctx context.Context, d time.Duration) ([]byte, readlimit.Outcome, error) {
	out := s.out
	return readlimit.Run(ctx, nil, d, func() ([]byte, error) { return out.ReadBytes('\n') })
}

// childArgs are the arguments that start a child with the parent's reading flags. Paths go on standard input.
func (s *supervisor) childArgs() []string {
	args := []string{"extract", "-supervised-child", "-root", s.root, "-deadline", s.cfg.deadline.String(), "-max-memory", strconv.FormatUint(s.memory, 10)}
	if s.cfg.pack != "" {
		args = append(args, "-pack", s.cfg.pack)
	}
	if s.cache != "" {
		args = append(args, "-cache", s.cache)
	}
	return args
}

// start launches a child and waits for it to say it is ready.
func (s *supervisor) start(ctx context.Context) error {
	cmd, err := childCommand(ctx, s.childArgs())
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	tail := &headBuffer{max: 8192}
	cmd.Stderr = tail
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting the reader process: %w", err)
	}
	s.cmd, s.stdin, s.out, s.stderr = cmd, stdin, bufio.NewReaderSize(stdout, 1<<20), tail
	hdr, _ := json.Marshal(childHeader{Siblings: s.sibs})
	if _, err := stdin.Write(append(hdr, '\n')); err != nil {
		return fmt.Errorf("the reader process did not start: %s", s.reap())
	}
	b, out, err := s.line(ctx, startupTimeout)
	switch {
	case out == readlimit.Canceled:
		s.kill()
		return err
	case out == readlimit.DeadlineExceeded:
		s.kill()
		return fmt.Errorf("the reader process was not ready in %v", startupTimeout)
	case err != nil:
		return fmt.Errorf("the reader process did not start: %s", s.reap())
	}
	var hello childHello
	if err := json.Unmarshal(b, &hello); err != nil || (!hello.Ready && hello.Fatal == "") {
		s.kill()
		return errors.New("the reader process said something other than ready")
	}
	if hello.Fatal != "" {
		s.kill()
		return errors.New(hello.Fatal)
	}
	return nil
}

// read asks the child for one path, starting or restarting it as needed. A file whose read ends the child, or passes
// its time, is a result that reads Incomplete; the error is for a run that cannot go on (a cancelled context, a
// child that will not start).
func (s *supervisor) read(ctx context.Context, p string) (fileResult, error) {
	if s.cmd == nil {
		if err := s.start(ctx); err != nil {
			return fileResult{}, err
		}
	}
	_, shown, _ := resolvePath(s.root, p)
	path, _ := json.Marshal(p)
	if _, err := s.stdin.Write(append(path, '\n')); err != nil {
		return killedResult(shown, "the reader process ended before it took this file ("+s.reap()+")"), nil
	}
	b, out, err := s.line(ctx, s.cfg.deadline+killMargin)
	switch {
	case out == readlimit.Canceled:
		s.kill()
		return fileResult{}, fmt.Errorf("%s: %w", shown, err)
	case out == readlimit.DeadlineExceeded:
		s.kill()
		return killedResult(shown, fmt.Sprintf("the reader process passed the %v read deadline and a %v margin on this file and was killed", s.cfg.deadline, killMargin)), nil
	case err != nil:
		return killedResult(shown, "the reader process ended while it read this file ("+s.reap()+")"), nil
	}
	var fr fileResult
	if err := json.Unmarshal(b, &fr); err != nil {
		s.kill()
		return killedResult(shown, "the reader process answered with something other than a result"), nil
	}
	if abandoned(fr) {
		s.kill() // the child gave up on a parser that is still running: end it rather than let it burn a core
	}
	return fr, nil
}

// abandoned reports whether the child answered by abandoning a parser, which keeps running in it.
func abandoned(fr fileResult) bool {
	if fr.Result == nil {
		return false
	}
	if fr.Result.Skipped == lang.SkipBusy {
		return true
	}
	for _, n := range fr.Result.Notes {
		if strings.HasPrefix(n, "read-deadline:") {
			return true
		}
	}
	return false
}

// killedResult is the result of a file the child could not finish.
func killedResult(path, why string) fileResult {
	return fileResult{Path: path, Result: &lang.Result{
		Symbols: []lang.Symbol{}, References: []lang.Reference{}, Unresolved: []lang.Unresolved{}, Facts: []lang.Fact{},
		Incomplete: true,
		Notes:      []string{killedNotePrefix + why + ", so none of its facts are returned"},
	}}
}

// kill ends the child and reaps it.
func (s *supervisor) kill() {
	if s.cmd == nil {
		return
	}
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.reap()
}

// reap waits for the child, which is ending or was killed, and says how it ended. A child that has not exited
// after waitGrace is killed. The next read starts a new one.
func (s *supervisor) reap() string {
	cmd, tail := s.cmd, s.stderr
	s.cmd, s.stdin, s.out, s.stderr = nil, nil, nil, nil
	if cmd == nil {
		return "no reader process"
	}
	err := waitChild(cmd)
	var ee *exec.ExitError
	switch {
	case err == nil:
		return "the reader process exited"
	case errors.As(err, &ee) && ee.ExitCode() == exitMemory:
		return fmt.Sprintf("the reader process passed its memory limit of %d bytes and exited", s.memory)
	}
	why := err.Error()
	if f := tail.fatalLine(); f != "" {
		why += ": " + f
	}
	return why
}

// waitChild waits for cmd to exit and kills it when it does not within waitGrace.
func waitChild(cmd *exec.Cmd) error {
	wait := func() (struct{}, error) { return struct{}{}, cmd.Wait() }
	_, out, err := readlimit.Run(context.Background(), nil, waitGrace, wait)
	if out == readlimit.DeadlineExceeded {
		cmd.Process.Kill()
		_, _, err = readlimit.Run(context.Background(), nil, 0, wait)
	}
	return err
}

// stop ends the child at the end of a run: closing its standard input makes it exit, and a child that does not is killed.
func (s *supervisor) stop() {
	if s.cmd == nil {
		return
	}
	s.stdin.Close()
	s.reap()
}

// headBuffer keeps the first max bytes written to it: a child's standard error, of which only a runtime fatal's
// first line is ever shown. The runtime prints that line before the long goroutine trace, so the head holds it.
type headBuffer struct {
	mu  sync.Mutex
	max int
	b   []byte
}

func (t *headBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room := t.max - len(t.b); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		t.b = append(t.b, p[:room]...)
	}
	return len(p), nil
}

// fatalLine is the line of a Go runtime fatal ("fatal error: stack overflow"), and "" when the child wrote none. It is
// the runtime's own text and never the file's.
func (t *headBuffer) fatalLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, l := range strings.Split(string(t.b), "\n") {
		if strings.HasPrefix(l, "fatal error: ") {
			if len(l) > 120 {
				l = l[:120]
			}
			return l
		}
	}
	return ""
}
