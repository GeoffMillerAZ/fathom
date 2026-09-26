package custodian

import (
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// Option adjusts one Audit run. The four v1 kinds read only the tree;
// CC-14 asks the daemon which tools it serves and CC-15 reads the user's
// thread log, and both need a clock — these are the seams that let a
// test audit without a daemon, a home directory, or wall time. Defaults
// are the live ones, resolved the way the hooks resolve them.
type Option func(*auditEnv)

// auditEnv is the resolved environment one Audit call runs under.
type auditEnv struct {
	daemonURL   string
	toolsFetch  func(daemonURL string) ([]string, error)
	threadsPath string
	now         func() time.Time
	wiringDrift WiringDriftFunc
}

func newAuditEnv(opts []Option) auditEnv {
	env := auditEnv{
		daemonURL:   DefaultDaemonURL(),
		toolsFetch:  FetchServedTools,
		threadsPath: DefaultThreadsPath(),
		now:         time.Now,
	}
	for _, o := range opts {
		if o != nil {
			o(&env)
		}
	}
	return env
}

// WithDaemonURL names the daemon the stanza check compares against
// (default: DefaultDaemonURL). The URL is reported in every stanza
// finding, so an unable-to-check names what was unreachable.
func WithDaemonURL(u string) Option {
	return func(e *auditEnv) { e.daemonURL = u }
}

// WithToolsFetcher replaces the registry lookup (default:
// FetchServedTools). The fetcher receives the daemon URL and returns the
// served tool names; an error is reported as unable-to-check.
func WithToolsFetcher(f func(daemonURL string) ([]string, error)) Option {
	return func(e *auditEnv) { e.toolsFetch = f }
}

// WithThreadsPath points the handoff check at a thread log (default:
// DefaultThreadsPath — the same resolution `fathom thread` uses).
func WithThreadsPath(p string) Option {
	return func(e *auditEnv) { e.threadsPath = p }
}

// WithNow injects the clock the handoff window is measured from.
func WithNow(f func() time.Time) Option {
	return func(e *auditEnv) { e.now = f }
}

// DefaultDaemonURL is the hooks' resolution: FATHOM_HTTP, else the
// loopback daemon on its conventional port.
func DefaultDaemonURL() string {
	if u := os.Getenv("FATHOM_HTTP"); u != "" {
		return u
	}
	// RT-17/RT-18 (§10.4): a test binary, or a process a test started
	// (FATHOM_TEST=1), never falls back to the operator's daemon.
	return testisolate.DaemonURLBackstop("http://localhost:7575")
}

// DefaultThreadsPath is `fathom thread`'s resolution: FATHOM_THREADS,
// else ~/.fathom/threads.jsonl; "" when no home directory resolves (the
// handoff check then reports unable-to-check, not a silent pass).
func DefaultThreadsPath() string {
	if p := os.Getenv("FATHOM_THREADS"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "threads.jsonl")
}
