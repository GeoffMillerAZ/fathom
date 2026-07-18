// Package boundedexec runs subprocesses with hard resource bounds for agent
// tool execution: a wall-clock deadline, byte caps on captured output, an
// explicitly supplied environment, and a fixed working directory.
//
// boundedexec is NOT A SANDBOX. It bounds how long a tool may run and how
// much output it may return — nothing else. Do not reach for this package
// expecting containment of hostile code; it provides none.
//
// # Non-guarantees
//
// The child process runs with the host's FULL AMBIENT AUTHORITY:
//
//   - Filesystem: the child can read and write anything the parent's user
//     can. Config.Dir sets the starting directory only; it is not a chroot.
//   - Network: unrestricted.
//   - Memory and CPU: unrestricted beyond the wall-clock timeout. A child
//     can exhaust RAM or spin every core until the deadline kills it.
//   - Privileges: the child runs as the parent's user with all of that
//     user's capabilities.
//
// Real isolation requires OS primitives — containers, namespaces, seccomp,
// sandbox-exec, jails — and is explicitly out of scope for a stdlib-only
// kit. Layer this package inside such a primitive when running untrusted
// tools.
//
// Environment isolation here prevents accidental secret inheritance (for
// example API keys sitting in the parent's environment) only when the
// caller passes a scrubbed Config.Env. The child's environment is always
// set explicitly from Config.Env and never inherited — but a hostile child
// can still read secrets from disk or the process table; see above. Note
// also that a relative Spec.Path is resolved against the parent process's
// PATH (exec.LookPath), not Config.Env: pass absolute paths to avoid
// ambient lookup.
//
// # Process-tree cleanup
//
// On unix the child is started in its own process group (Setpgid) and, on
// deadline expiry or cancellation, the whole group is signaled: SIGTERM to
// the negative pgid first, SIGKILL after a short grace period. Grandchild
// cleanup is BEST-EFFORT and UNIX-ONLY: a descendant that moves itself
// into a new process group escapes, and on non-unix platforms only the
// direct child is killed.
//
// # Determinism
//
// Result.Duration is measured with an injected clock; deadline and grace
// enforcement necessarily use real runtime timers (context.WithTimeout,
// time.Timer) because the child process lives on the real clock. The tests
// for this package deliberately deviate from the repo's injected-clock /
// no-sleep rule: exercising a subprocess runner honestly requires real
// subprocesses and real (short) timeouts. They skip on platforms without
// /bin/sh.
//
// # Composition
//
// Stdlib only (context, os/exec, syscall behind build tags). Policy about
// whether a tool may run at all belongs to higher layers (for example
// foundations/environment/tool-execution-policy); boundedexec only
// executes an already-approved Spec within bounds.
//
// # Concurrency
//
// A Runner is immutable after New and safe for concurrent use by multiple
// goroutines. Each Run call uses only local state and joins its single
// watcher goroutine before returning; no goroutines outlive Run.
package boundedexec

import "time"

// Config bounds every run made through a Runner.
type Config struct {
	// Timeout is the maximum wall-clock duration a child may run. The
	// effective hard deadline of a run is min(ctx deadline, Timeout).
	// Must be positive.
	Timeout time.Duration

	// MaxStdoutBytes caps captured stdout. The FIRST MaxStdoutBytes bytes
	// are kept; the remainder is drained and discarded so the child never
	// blocks on a full pipe. Must be positive.
	MaxStdoutBytes int

	// MaxStderrBytes caps captured stderr, with the same first-N-and-drain
	// semantics as MaxStdoutBytes. Must be positive.
	MaxStderrBytes int

	// Env is the COMPLETE child environment in "KEY=value" form. It is
	// always set explicitly on the child — the parent's environment is
	// NEVER inherited. A nil or empty slice yields an empty environment.
	// Pass a scrubbed allowlist to keep parent secrets (API keys, tokens)
	// out of the child.
	Env []string

	// Dir is the child's working directory. Empty means the parent's
	// current directory (ambient — prefer setting it explicitly).
	Dir string
}

// Spec describes one subprocess invocation.
type Spec struct {
	// Path is the program to run. Prefer an absolute path: relative paths
	// are resolved against the PARENT's PATH, not Config.Env.
	Path string

	// Args are the arguments passed to the program (argv[1:]).
	Args []string

	// Stdin is written to the child's standard input and then closed.
	// Nil or empty means the child sees immediate EOF.
	Stdin []byte
}

// Result reports what a bounded run produced. It is populated (possibly
// with partial output) even when the run timed out or was canceled.
type Result struct {
	// Stdout holds the first Config.MaxStdoutBytes bytes of the child's
	// standard output.
	Stdout []byte

	// Stderr holds the first Config.MaxStderrBytes bytes of the child's
	// standard error.
	Stderr []byte

	// StdoutTruncated is true when the child wrote more than
	// Config.MaxStdoutBytes bytes; the excess was drained and discarded.
	StdoutTruncated bool

	// StderrTruncated is true when the child wrote more than
	// Config.MaxStderrBytes bytes; the excess was drained and discarded.
	StderrTruncated bool

	// ExitCode is the child's exit status. It is -1 when the child was
	// terminated by a signal — including kills issued by this package on
	// timeout. A signal kill that was NOT caused by the deadline is
	// distinguished from a timeout by TimedOut being false.
	ExitCode int

	// TimedOut is true when the hard deadline (min of ctx deadline and
	// Config.Timeout) expired and the child was killed for it.
	TimedOut bool

	// Duration is the observed wall-clock time of the run, from process
	// start to full reaping.
	Duration time.Duration
}
