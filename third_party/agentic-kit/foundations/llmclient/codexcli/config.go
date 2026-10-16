package codexcli

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
	defaultBin = "codex"

	// defaultTimeout bounds a single Complete call's subprocess lifetime when
	// Config.Timeout is zero or negative. The effective bound is always
	// min(Timeout, the caller's ctx deadline) — see Complete.
	defaultTimeout = 5 * time.Minute

	// maxCaptureBytes caps how much of the child process's stdout/stderr this
	// adapter buffers, mirroring claudecli's guard against unbounded memory
	// growth. A real JSONL event stream for a single-turn completion is small;
	// this is generous headroom, not an expected ceiling.
	maxCaptureBytes = 32 << 20

	// scratchDirPattern names the temp directory this adapter creates (when
	// Config.ScratchDir is empty) and passes to every `codex exec -C` call.
	// See the package doc, "Ambient-context suppression", for why an empty,
	// non-git, purpose-built directory is part of the pinned invocation.
	scratchDirPattern = "codexcli-scratch-*"
)

// Config configures a Client. The zero value resolves "codex" from PATH,
// passes no -m (the CLI uses its own configured default model), creates and
// owns a fresh scratch directory, and bounds each Complete call at
// defaultTimeout.
//
// WARNING — version-pinned against codex-cli 0.144.5; read before
// upgrading the CLI: this adapter's Request.System mapping
// (model_instructions_file) and most of its ambient-context suppression
// (the four include_* keys) ride UNDOCUMENTED config keys that exist only
// as field names inside the 0.144.5 binary — they appear nowhere in
// `codex exec --help` or any CLI documentation surface. A future codex-cli
// version may drop or rename them SILENTLY: an unrecognised `-c key=value`
// is verified to be tolerated with exit 0 and no warning (see
// testdata/PROBES.md, preliminary section). The consequence of that silent
// drop would be severe and invisible: Request.System would be silently
// lost (the model never sees it), the CLI's ~13k-token built-in persona
// would silently return on every call (roughly quadrupling input spend and
// changing model behavior), and no error would ever surface. Call Ping at
// construction/deploy time — it is the liveness tripwire built for exactly
// this dependence — and re-verify per testdata/PROBES.md on any codex-cli
// upgrade.
type Config struct {
	// Bin is the codex executable: a bare name resolved via exec.LookPath
	// against PATH (default "codex" when empty), or an absolute path
	// override — matters for contexts (e.g. a launchd daemon) whose PATH
	// does not include wherever the CLI was installed.
	Bin string

	// Model is the default model passed via -m when Request.Model is empty.
	// Empty means: omit -m entirely and let the CLI use its own configured
	// default (config.toml's `model`, or the CLI's built-in default).
	Model string

	// Timeout bounds a single Complete call's subprocess lifetime. Zero or
	// negative means defaultTimeout (5 minutes). The effective bound is
	// always min(Timeout, the caller's ctx deadline).
	Timeout time.Duration

	// ScratchDir overrides the working root passed to every call via
	// `codex exec -C <ScratchDir>` (see the package doc, "Ambient-context
	// suppression"). Empty means New creates and owns a fresh temp directory
	// (named via scratchDirPattern) for the Client's lifetime; the directory
	// is never removed (see the package doc, "Scratch directory lifecycle").
	// Set this explicitly in tests that need to inspect the directory's
	// contents (e.g. for temp-file cleanup assertions).
	ScratchDir string
}

// Client is a Codex CLI implementation of llmclient.LLMClient: it shells out
// to the `codex` binary rather than calling an HTTP API directly, so
// completions ride the local CLI's login (a ChatGPT Plus/Pro subscription's
// OAuth session) instead of a metered OPENAI_API_KEY. See the package doc for
// the exact invocation shape, what is and is not supported, and the honesty
// notes on ambient context and cost accounting.
//
// Concurrency: Client is immutable after New and safe for concurrent use —
// every Complete/Stream call runs its own subprocess through runner (itself
// documented safe for concurrent use) with per-call temp files under
// scratchDir; no state is shared across concurrent calls beyond that shared
// (but per-call-uniquely-named) directory.
type Client struct {
	bin        string
	model      string
	scratchDir string
	runner     *boundedexec.Runner
}

var _ llmclient.LLMClient = (*Client)(nil)

// New resolves cfg.Bin (default "codex") via exec.LookPath — failing fast
// here, at construction, rather than deferring the failure to the first
// Complete call — creates (or validates) the scratch directory, and returns
// a ready Client.
//
// The subprocess is run through foundations/agent/bounded-exec, reusing the
// kit's existing bounded subprocess runner (timeout enforcement, captured
// output caps, best-effort process-tree cleanup on cancellation/timeout)
// rather than hand-rolling exec.CommandContext a second time — the same
// composition choice claudecli makes. Like claudecli, and for the same
// reason, this Client passes os.Environ() through VERBATIM: the whole point
// of this adapter is letting the `codex` binary find its own
// already-authenticated ChatGPT session (HOME, PATH, and whatever else the
// CLI's credential resolution needs via CODEX_HOME), and the child here is
// the trusted, purpose-built `codex` binary, not an arbitrary or untrusted
// tool. This never reads or logs any credential value — it only forwards the
// parent's environment variables to the child process, exactly as a normal
// (unscrubbed) exec.Command would.
func New(cfg Config) (*Client, error) {
	bin := cfg.Bin
	if bin == "" {
		bin = defaultBin
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("codexcli: resolve %q: %w", bin, err)
	}

	scratchDir := cfg.ScratchDir
	if scratchDir == "" {
		scratchDir, err = os.MkdirTemp("", scratchDirPattern)
		if err != nil {
			return nil, fmt.Errorf("codexcli: create scratch dir: %w", err)
		}
	} else if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return nil, fmt.Errorf("codexcli: create scratch dir %q: %w", scratchDir, err)
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
		return nil, fmt.Errorf("codexcli: %w", err)
	}

	return &Client{
		bin:        resolved,
		model:      cfg.Model,
		scratchDir: scratchDir,
		runner:     runner,
	}, nil
}
