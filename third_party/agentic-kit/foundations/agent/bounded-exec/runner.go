package boundedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

const (
	// killGrace is how long killTree waits between the polite signal
	// (SIGTERM on unix) and the forced SIGKILL of the process group.
	killGrace = 100 * time.Millisecond

	// pipeWaitDelay bounds how long Wait may block on the child's I/O
	// pipes after the child itself has exited — e.g. when an escaped
	// grandchild still holds the write end. See exec.Cmd.WaitDelay.
	pipeWaitDelay = 1 * time.Second
)

// Runner executes Specs under the bounds of its Config. Immutable after
// New; safe for concurrent use by multiple goroutines.
type Runner struct {
	cfg Config
	now func() time.Time
}

// New validates cfg and returns a Runner. Timeout, MaxStdoutBytes and
// MaxStderrBytes must all be positive; Env and Dir are taken as given
// (nil Env means an empty child environment, never inheritance).
func New(cfg Config) (*Runner, error) {
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("boundedexec: Config.Timeout must be positive, got %v", cfg.Timeout)
	}
	if cfg.MaxStdoutBytes <= 0 {
		return nil, fmt.Errorf("boundedexec: Config.MaxStdoutBytes must be positive, got %d", cfg.MaxStdoutBytes)
	}
	if cfg.MaxStderrBytes <= 0 {
		return nil, fmt.Errorf("boundedexec: Config.MaxStderrBytes must be positive, got %d", cfg.MaxStderrBytes)
	}
	return &Runner{cfg: cfg, now: time.Now}, nil
}

// Run starts the subprocess described by spec and waits for it within the
// hard deadline min(ctx deadline, Config.Timeout).
//
// Outcomes:
//   - Normal completion: Result.ExitCode carries the child's status —
//     non-zero exits are data, not errors. err is nil.
//   - Deadline expiry: the process tree is killed (best-effort, see the
//     package doc), Result.TimedOut is true, ExitCode is typically -1,
//     partial output is returned, and err is nil.
//   - ctx canceled: the process tree is killed, partial output is
//     returned, and err wraps context.Canceled.
//   - Child killed by a signal not sent by this package: ExitCode is -1
//     with TimedOut false and err nil.
//   - Failure to start, or an I/O failure while waiting: err is non-nil.
//
// Captured stdout/stderr keep the first N bytes per Config and drain the
// rest, so the child is never blocked on a full pipe by the caps.
func (r *Runner) Run(ctx context.Context, spec Spec) (Result, error) {
	if spec.Path == "" {
		return Result{}, errors.New("boundedexec: Spec.Path must not be empty")
	}

	runCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()
	if err := runCtx.Err(); err != nil {
		return Result{}, fmt.Errorf("boundedexec: context done before start: %w", err)
	}

	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = r.cfg.Dir
	cmd.Env = r.cfg.Env
	if cmd.Env == nil {
		// nil cmd.Env would mean "inherit the parent environment"; the
		// contract is that the child environment is ALWAYS explicit.
		cmd.Env = []string{}
	}
	cmd.Stdin = bytes.NewReader(spec.Stdin)
	stdout := &capWriter{max: r.cfg.MaxStdoutBytes}
	stderr := &capWriter{max: r.cfg.MaxStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = pipeWaitDelay
	setProcGroup(cmd)

	start := r.now()
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("boundedexec: start %q: %w", spec.Path, err)
	}

	exited := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-runCtx.Done():
			killTree(cmd, killGrace, exited)
		case <-exited:
		}
	}()

	waitErr := cmd.Wait()
	close(exited)
	<-watcherDone // join the watcher: no goroutine outlives Run.

	res := Result{Duration: r.now().Sub(start)}
	res.Stdout, res.StdoutTruncated = stdout.snapshot()
	res.Stderr, res.StderrTruncated = stderr.snapshot()

	exitCode, waitFailure := classifyWait(waitErr, cmd)
	res.ExitCode = exitCode

	switch runCtx.Err() {
	case context.DeadlineExceeded:
		// The hard deadline (min of ctx deadline and cfg.Timeout) fired.
		// This is a bounded, expected outcome — reported as data.
		res.TimedOut = true
		return res, nil
	case context.Canceled:
		return res, fmt.Errorf("boundedexec: run canceled: %w", context.Canceled)
	}
	if waitFailure != nil {
		return res, waitFailure
	}
	return res, nil
}

// classifyWait turns cmd.Wait's error into an exit code plus an optional
// hard failure. Exit statuses — including -1 for killed-by-signal — are
// data, not failures. exec.ErrWaitDelay means the child exited cleanly but
// an inherited pipe was forcibly closed (e.g. an escaped grandchild held
// it); the run is treated as a success with possibly incomplete output.
func classifyWait(waitErr error, cmd *exec.Cmd) (exitCode int, failure error) {
	if waitErr == nil {
		return 0, nil
	}
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		return cmd.ProcessState.ExitCode(), nil
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		return ee.ExitCode(), nil
	}
	return -1, fmt.Errorf("boundedexec: wait: %w", waitErr)
}

// capWriter keeps the first max bytes written and silently drains the
// rest, flagging truncation. Write never returns an error and never
// blocks, so os/exec's pipe-copy goroutine keeps draining the child's
// output to EOF — a child writing far past the cap is never stalled.
// Mutex-guarded so snapshots are race-free even if the copier is forced
// down by WaitDelay.
type capWriter struct {
	mu        sync.Mutex
	max       int
	buf       []byte
	truncated bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	room := w.max - len(w.buf) // never negative: buf never exceeds max
	if room >= len(p) {
		w.buf = append(w.buf, p...)
		return len(p), nil
	}
	w.buf = append(w.buf, p[:room]...)
	w.truncated = true
	return len(p), nil
}

func (w *capWriter) snapshot() ([]byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]byte, len(w.buf))
	copy(out, w.buf)
	return out, w.truncated
}
