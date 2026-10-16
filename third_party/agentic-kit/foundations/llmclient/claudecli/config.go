package claudecli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	boundedexec "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/bounded-exec"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

const (
	// defaultBin is the executable name resolved via PATH when Config.Bin is
	// empty.
	defaultBin = "claude"

	// defaultTimeout bounds a single Complete call's subprocess lifetime when
	// Config.Timeout is zero or negative. Generous enough to cover a slow
	// cold-cache completion; the effective bound is always
	// min(Timeout, the caller's ctx deadline) — see Complete.
	defaultTimeout = 5 * time.Minute

	// maxCaptureBytes caps how much of the child process's stdout/stderr this
	// adapter buffers, mirroring the anthropic adapter's maxBodyBytes guard
	// against unbounded memory growth. A real envelope is small (well under 1
	// MiB); this is generous headroom, not an expected ceiling.
	maxCaptureBytes = 32 << 20
)

// Config configures a Client. The zero value resolves "claude" from PATH,
// passes no --model (the CLI uses its own default), and bounds each Complete
// call at defaultTimeout.
type Config struct {
	// Bin is the claude executable: a bare name resolved via exec.LookPath
	// against PATH (default "claude" when empty), or an absolute path
	// override. The override matters for contexts (e.g. a launchd daemon)
	// whose PATH does not include wherever the CLI was installed —
	// exec.LookPath accepts and verifies an absolute path directly, without
	// needing it to be found via PATH search.
	Bin string

	// Model is the default model passed via --model when Request.Model is
	// empty. Accepts anything the installed CLI accepts: an alias ("sonnet",
	// "haiku", "opus", "fable") or a full model ID. Empty means: omit
	// --model entirely and let the CLI use its own configured default.
	Model string

	// Timeout bounds a single Complete call's subprocess lifetime. Zero or
	// negative means defaultTimeout (5 minutes). The effective bound is
	// always min(Timeout, the caller's ctx deadline).
	Timeout time.Duration
}

// Client is a Claude Code CLI implementation of llmclient.LLMClient: it
// shells out to the `claude` binary rather than calling an HTTP API
// directly, so completions ride the local CLI's login (a Claude Pro/Max
// subscription's OAuth session) instead of a metered ANTHROPIC_API_KEY. See
// the package doc for the exact invocation shape, what is and is not
// supported, and the honesty notes on ambient context and cost accounting.
//
// Concurrency: Client is immutable after New and safe for concurrent use —
// every Complete/Stream call runs its own subprocess through runner (itself
// documented safe for concurrent use); no state is shared across concurrent
// calls.
type Client struct {
	bin    string
	model  string
	runner *boundedexec.Runner
}

var _ llmclient.LLMClient = (*Client)(nil)

// New resolves cfg.Bin (default "claude") via exec.LookPath — failing fast
// here, at construction, rather than deferring the failure to the first
// Complete call — and returns a ready Client.
//
// The subprocess is run through foundations/agent/bounded-exec (reusing the
// kit's existing bounded subprocess runner — timeout enforcement, captured
// output caps, and best-effort process-tree cleanup on cancellation/timeout —
// rather than hand-rolling exec.CommandContext a second time). Unlike
// bounded-exec's typical caller, which passes a scrubbed environment
// allowlist to keep secrets out of a sandboxed tool run, this Client passes
// os.Environ() through VERBATIM: the whole point of this adapter is letting
// the `claude` binary find its own already-authenticated session (HOME,
// PATH, and whatever else the CLI's credential resolution needs), and the
// child here is the trusted, purpose-built `claude` binary, not an arbitrary
// or untrusted tool. This never reads or logs any credential value — it only
// forwards the parent's environment variables to the child process, exactly
// as a normal (unscrubbed) exec.Command would.
func New(cfg Config) (*Client, error) {
	bin := cfg.Bin
	if bin == "" {
		bin = defaultBin
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("claudecli: resolve %q: %w", bin, err)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	runner, err := boundedexec.New(boundedexec.Config{
		Timeout:        timeout,
		MaxStdoutBytes: maxCaptureBytes,
		MaxStderrBytes: maxCaptureBytes,
		Env:            os.Environ(),
	})
	if err != nil {
		return nil, fmt.Errorf("claudecli: %w", err)
	}

	return &Client{
		bin:    resolved,
		model:  cfg.Model,
		runner: runner,
	}, nil
}
