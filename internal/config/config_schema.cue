// Package config defines the CUE schema for fathom's runtime configuration
// (docs/plans/config-experience.md CF-1). Humans author TOML
// (~/.fathom/config.toml, project .fathom/config.toml); this schema is the
// validator, never the authoring surface (ADR-018 Dagger lesson) — a parsed
// TOML document is converted to a CUE value and unified against #Config at
// load time, so a typo'd key or an out-of-enum value fails loudly, naming
// the file, the key, and the expected shape.
//
// This file is authoritative AND live: internal/config embeds a
// byte-identical copy (internal/config/config_schema.cue, drift-tested
// against this file — see internal/semantic/manifest.go for the precedent)
// and validates every loaded layer against it.
package config

// #Provider enumerates the six LLM construction lanes cmd/fathom's
// buildLLMClient already supports (docs/plans/provider-matrix.md). A profile
// with any other provider value is rejected at load, not at first use.
#Provider: "anthropic" | "claude-cli" | "openai" | "azure-openai" | "bedrock" | "codex-cli"

// #Profile is the unit of provider configuration. Every field beyond
// `provider` is optional: an unset field means "let the env-overlay or the
// adapter's own default decide" (internal/config.ResolveProfile), so a
// minimal profile can be as small as `provider = "claude-cli"`.
//
// Raw secrets are UNREPRESENTABLE by construction: this struct is closed
// (CUE definitions are closed by default) and has no `api_key` field, so a
// literal secret pasted into a profile fails validation instead of being
// silently accepted and persisted to disk. `api_key_env` names an
// environment variable to read at construction time; `api_key_cmd` is an
// argv-exec'd command (no shell) whose trimmed stdout is the key. Neither
// is ever logged or echoed in an error.
#Profile: {
	provider: #Provider

	// model is an exact pin — it always wins over the adapter's own
	// DefaultModel (see cmd/fathom/compile.go's compileModel doc), but a
	// caller-set FATHOM_COMPILE_MODEL env var (or a future --model flag)
	// still wins over this per the file's stated precedence.
	model?: string

	// endpoint overrides the provider's base URL: OpenAI-compatible base
	// (Ollama/vLLM), Azure OpenAI resource endpoint (including Azure
	// Government's *.openai.azure.us), or a Bedrock BaseURL override (a
	// GovCloud FIPS endpoint or VPC endpoint).
	endpoint?: string

	// region is the AWS region for provider "bedrock", e.g. "us-gov-west-1"
	// for GovCloud. Passed straight to the kit's bedrock.Config.Region.
	region?: string

	// deployment is the Azure OpenAI deployment name (provider
	// "azure-openai" only).
	deployment?: string

	// api_version is the Azure OpenAI API version (provider "azure-openai"
	// only); empty defers to the adapter's pinned GA default.
	api_version?: string

	// bin overrides the resolved CLI binary path for the claude-cli /
	// codex-cli lanes (mirrors FATHOM_CLAUDE_BIN / FATHOM_CODEX_BIN).
	bin?: string

	// timeout bounds ONE model call's subprocess for the claude-cli lane, a
	// Go duration ("20m", "90s"); mirrors FATHOM_CLAUDE_TIMEOUT. Omitted
	// keeps the kit default (5 minutes). A section that exceeds it counts
	// as failed; it never aborts the compile (EW-33).
	timeout?: =~"^([0-9]+(\\.[0-9]+)?(ns|us|ms|s|m|h))+$"

	// api_key_env names an environment variable read at LLM-client
	// construction time (never at config load time, so a rotated
	// credential is picked up without re-parsing the config file).
	api_key_env?: string

	// api_key_cmd is an argv (no shell interpretation) executed with a
	// bounded timeout at construction time; its trimmed stdout is the
	// credential. Must name at least one element (the executable).
	api_key_cmd?: [...string] & [_, ...]
}

// #Daemon configures the serve-http daemon's own startup: the same config
// file that picks a compile provider also picks the daemon's store/addr/
// call-log/auto-refresh defaults, so a launchd plist with no env at all is
// no longer a silent compile_layer outage (docs/plans/config-experience.md's
// grounding gap).
#Daemon: {
	store?:           string
	addr?:            string
	call_log?:        string
	auto_refresh?:    bool
	default_profile?: string

	// state_dir overrides where fathom's runtime state (session logs,
	// observed-discovery logs, retro/narrate artifacts, the receipts log,
	// MCP call logs) is written for this repository (O63 item 5). The
	// default — no state_dir set — is OUTSIDE the repo tree entirely
	// ($XDG_STATE_HOME/fathom/<repo-id>/, internal/statedir.Dir), so a
	// fresh adopter's `git status` after `fathom init` + a session hook +
	// an edit hook never shows hook output. The one recognised value is
	// "workspace": it restores the pre-O63 in-tree convention
	// (<repo>/workspace/...) — for fathom's own repository (whose
	// Taskfile and docs still reference workspace/ paths for its own dev
	// loop) or any repo that wants the old, versionable-by-choice layout
	// back. FATHOM_STATE_DIR (an env var, not a config key — the hooks'
	// fixed-command-line callers cannot set a flag) overrides both.
	state_dir?: "workspace"

	// memory_limit is a SOFT limit (W-N, docs/plans/daemon-footprint.md,
	// O108 reframed as peaks + darwin's lazy reclamation, not a live
	// leak): applied at daemon startup with runtime/debug.SetMemoryLimit,
	// it makes the Go runtime GC and scavenge harder as total memory use
	// approaches it — it never kills the process, unlike a hard (OS or
	// container) limit. Go size syntax ("3GiB", "512MiB", "1.5GiB") or
	// the literal "off" to disable; unset defaults to "3GiB"
	// (internal/config.ParseMemoryLimit does the actual parsing — this
	// pattern only bounds the shape). FATHOM_MEMORY_LIMIT (an env var, not
	// a config key — matches every other daemon key's env-wins-over-config
	// precedence) overrides this.
	memory_limit?: string & =~"^([0-9]+(\\.[0-9]+)?(B|KiB|MiB|GiB|TiB)|off)$"
}

// #Guards is ADR-038's escape hatch: `enforce = false` turns every
// enforce:true guard/layer-rule in .fathom/guard.cue back to advisory,
// repo-wide — FATHOM_ENFORCE=off (an env var, not a config key — the
// hooks' fixed-command-line callers cannot set a flag) is the other half
// of the same switch and always wins over this. Unset means "enforcement
// is live wherever a rule opts into it", the same default-true convention
// #Daemon.auto_refresh's *bool Go type already uses to distinguish "never
// set" from "explicitly false" (internal/config.Guards.Enforce).
#Guards: {
	git_safety?: bool // default on; project or user may opt out of the shipped pack
	enforce?: bool
}

// #Findings configures the findings-loop's governed external import
// (docs/vision/30-findings-loop.md §3.3/§3.7, ADR-040 decision 2): the
// provisioned subject→service mapping `fathom findings import` refuses to
// operate without.
#Findings: {
	// subject is the provisioned "org/repo/component" mapping every
	// imported event's own subject and visibility must equal — no mapping,
	// no import.
	subject?: string

	// outbox is an optional repo-relative path to the review kit's
	// projection outbox file; when set, the session hook imports it once
	// per session on change (§3.7).
	outbox?: string

	// producers is an optional allowlist of catalogued producer ids the
	// outbox sync accepts.
	producers?: [...string]
}

// #Decide configures ADR-042's decision port (docs/vision/32-decision-port.md):
// typed fast decisions (choice/score/noul) behind a provider flag, off by
// default. `provider` is "none" (the default: no call is ever made, every
// site behaves exactly as before), "llm" (any llmclient profile lane, asked
// for a JSON object against the question schema), "typesafe" (the kit's
// TypeSafe/Jev adapter, called directly over TypeSafe's own API or, with
// transport "openrouter", OpenRouter's Decisions router) or "openai" (the
// kit's adapter for OpenAI's Decisions API); model/base_url/transport/
// api_key_file/api_key_env/api_key_cmd configure the two remote lanes. Any
// other value is refused by name, same discipline as #Provider.
// compile_threshold and route_confidence are probabilities and so must lie
// in [0,1] — CUE enforces the range directly, the same way #Profile's
// provider enum makes an out-of-set value a load-time validation error
// rather than a silent no-op. A literal key is refused: `api_key` is an
// error naming api_key_file, and the key-naming fields are shape-checked
// with messages that never repeat the value (a key pasted into the wrong
// field must not reach a log line).
#Decide: {
	provider?: "none" | "llm" | "typesafe" | "openai"

	// profile names the [llm] profile lane provider "llm" uses; empty (the
	// default) means "the compile lane's own default profile resolution".
	profile?: string

	// compile_threshold: a guide section whose "does this state a binding
	// rule" probability is below this is not compiled (counted, disclosed
	// as prefilter_skipped, still served raw by ADR-037's fallback).
	compile_threshold?: number & >=0 & <=1

	// route_confidence: unchanged at 0.9 (2026-09-24, ADR-042 amendment:
	// route's own calibration reading failed the check at every cutoff).
	// The prompt hook injects a HEDGED route line at or above the confirm
	// floor (0.5) — proceed and confirm render identical wording since
	// CV-M11, never a firm instruction — and nothing below it.
	route_confidence?: number & >=0 & <=1

	// route_timeout_ms bounds POST /route's own decider call (O109): the
	// default (3000ms) fits Jev-class latency (70-500ms measured) but
	// starves an LLM lane under ordinary network latency (1.5-2.8s warm,
	// more cold, measured live against OpenRouter) — the design point is a
	// sub-second decision model; the prompt hook's own ~8s shared deadline
	// still bounds the line regardless of this value.
	route_timeout_ms?: int & >=100 & <=30000

	// rank_within_tier: obligations re-ranks eligible rules only inside
	// their governing-scope tier (ADR-035's tier order is never crossed).
	rank_within_tier?: bool

	// rerank_timeout_ms (DP-25, O109) bounds how long the obligations
	// re-rank may wait for the decider across every chunk of one
	// obligations call, in milliseconds. Default 1500 — deliberately kept
	// under the PostToolUse edit hook's own 2000ms HTTP client timeout for
	// its obligations RPC (cmd/fathom/edithook.go), with margin: a rerank
	// that exhausts the whole budget must still return before that
	// client's own deadline gives up and renders nothing at all. The
	// design point is a sub-second decision model (e.g. TypeSafe/Jev); an
	// LLM lane will usually exceed this default, and the envelope then
	// discloses rank_note "decision budget …" while rules are served in
	// lexical order rather than waiting. Raise it only for offline/
	// non-hook use, where nothing is racing a client-side deadline.
	rerank_timeout_ms?: int & >=100 & <=60000

	// max_asks_per_site_per_session (KS-10, ADR-073 decision 7; default
	// 200) caps how many times one site asks the decision port in one
	// session; past it the site answers on its deterministic path with one
	// `capped` line. The minimum across layers: a repository's value may
	// lower the user's (or the default) and never raise it.
	max_asks_per_site_per_session?: int & >=1 & <=100000

	// base_url overrides a remote provider's API origin: provider
	// "typesafe" (default "https://api.typesafe.ai", or
	// "https://openrouter.ai" with transport "openrouter") or "openai"
	// (default "https://api.openai.com/v1"). Unused by "none"/"llm". A key
	// is sent only to the provider's own host, a loopback host, or a host a
	// user-layer (~/.fathom/config.toml) base_url names: a repository's own
	// .fathom/config.toml cannot redirect your key (internal/decide).
	base_url?: string

	// endpoint is base_url's older name (provider "typesafe"), still read
	// when base_url is unset.
	endpoint?: string

	// transport names where a remote provider's wire goes: "typesafe"
	// (TypeSafe's own API, the default for provider "typesafe"),
	// "openrouter" (the same System One wire through OpenRouter's Decisions
	// router, provider "typesafe" only) or "openai" (OpenAI's own API, the
	// only transport for provider "openai").
	transport?: "typesafe" | "openrouter" | "openai"
	if provider != _|_ {
		if provider == "openai" {
			transport?: "openai" | error("[decide] transport for provider \"openai\" is \"openai\" (its own API) or unset")
		}
		if provider == "typesafe" {
			transport?: "typesafe" | "openrouter" | error("[decide] transport for provider \"typesafe\" is \"typesafe\" or \"openrouter\"")
		}
	}

	// model overrides a remote provider's model (defaults: "jev-latest" for
	// typesafe, "typesafe/jev-1.13" over openrouter, "gpt-6-luna" for
	// openai). Unused by "none"/"llm".
	model?: string

	// api_key is never accepted: a literal key does not belong in a config
	// file. The message points at the two ways to name one.
	api_key?: error("[decide] api_key is not accepted: never put a key in config.toml; set api_key_file (an absolute or ~/ path to a chmod 600 file holding the key) or api_key_env (the name of a variable holding it)")

	// api_key_file is the path of a file holding the remote provider's key,
	// absolute or under "~/"; the file must grant no access to group or
	// other users (chmod 600). It is read by the adapter at first use, never
	// at config load, and wins over api_key_env. Use it for the launchd
	// daemon, which does not see your shell's environment.
	api_key_file?: =~"^(/|~/)." | error("[decide] api_key_file must be an absolute or ~/ path to a key file (the value is not repeated here)")

	// api_key_env names an environment variable read at decision-port
	// construction time (never at config load time); unset defers to the
	// transport's well-known variable (TYPESAFE_API_KEY, OPENROUTER_API_KEY
	// or OPENAI_API_KEY).
	api_key_env?: =~"^[A-Za-z_][A-Za-z0-9_]*$" | error("[decide] api_key_env must name an environment variable: letters, digits and underscores (the value is not repeated here)")

	// api_key_cmd is an argv (no shell interpretation) executed with a
	// bounded timeout at construction time; its trimmed stdout is the
	// credential. Wins over api_key_file and api_key_env when set. Must name
	// at least one element (the executable).
	api_key_cmd?: [...string] & [_, ...]

	// sites is the egress allowlist (ADR-044 decision 8, I14): the site
	// names this port is allowed to send a decision.Question off-host from.
	// A site absent here makes NO call at all, whatever provider is set.
	// Unset (the default) means DefaultDecide.Sites in Go: every site that
	// existed when per-site opt-in landed, minus "route" (JF-7, off by
	// default; naming it turns it back on); `sites = []` disables every
	// site. Closed to the six sites that exist today ("assess" is `fathom
	// assess`, ADR-046; "preflight" is task preflight's Score, WI-18) plus
	// the opt-in names (JF-4, judgment-foundations §4, R2): "triage_failure",
	// "ask", "explore", "compact_advice" and one "judge_<name>" per judge
	// site. An opt-in name takes effect only from the repository's own
	// .fathom/config.toml, never from a user-global file. A session site
	// ("readout_mode", "compact_advice") validates here but is dropped and
	// named: only session_sites opts one in (I60 as amended by ADR-073).
	sites?: [...("route" | "compile_prefilter" | "obligations_rerank" | "certify_suggest" | "assess" | "preflight" | "triage_failure" | "ask" | "explore" | "compact_advice" | "readout_mode" | =~"^judge_[a-z][a-z0-9_]*$")]

	// exclude (default empty, I61): repo-relative path globs ("**" crosses
	// directories) whose spans never leave for a decision provider: an ask
	// whose request names a matching path, or a trailing part of one (so a
	// glob without a slash matches a base name at any depth), answers on
	// its deterministic path. A privacy key: every layer's globs union, so
	// a repository adds and never removes the user's.
	exclude?: [...string & !=""]

	// The consent and kill fields (KS-3, ADR-073 decision 5, I66): every one
	// is user-only, read from ~/.fathom/config.toml alone; a repository's
	// .fathom/config.toml that sets one has it dropped and named.
	//
	// enabled (default true): false is a hard off, re-checked before each
	// ask, so a running daemon closes on its next ask without a restart.
	enabled?: bool

	// allow_repo_provider (default false): a provider or transport that only
	// a repository's config names stays closed unless this is true; even
	// then a transport's well-known key variable is read only when this file
	// names the provider.
	allow_repo_provider?: bool

	// session_sites (default empty): the only list that opts in the session
	// sites, decisions about your own sessions.
	session_sites?: [...("readout_mode" | "compact_advice")]

	// deny_sites (default empty): a site named here is refused whatever any
	// list says.
	deny_sites?: [...("route" | "compile_prefilter" | "obligations_rerank" | "certify_suggest" | "assess" | "preflight" | "triage_failure" | "ask" | "explore" | "compact_advice" | "readout_mode" | =~"^judge_[a-z][a-z0-9_]*$")]
}

// #Tests configures vision 33 / ADR-043 stage 0's test-evidence ingest: a
// plain `fathom ingest` also picks up attested #TestResult envelopes when
// results_dir is set, exactly as `fathom tests ingest --results` does
// explicitly.
#Tests: {
	// results_dir is a repo-relative directory `fathom ingest` reads
	// attested #TestResult envelopes from (internal/testevidence.
	// LoadEnvelopes) — unset means "no envelope auto-pickup on a plain
	// ingest"; `fathom tests ingest --results` still works either way.
	results_dir?: string

	// artifacts_dir is a repo-relative directory an envelope's artifact
	// refs (name + sha256 digest) resolve against when the digest
	// matches — unset means artifact refs are recorded but never
	// resolved to bytes.
	artifacts_dir?: string

	// commands (TR-1) lists the repository's own declared test commands
	// ("go test", "bin/itest"), beside those read from its Taskfile,
	// Makefile and package.json; the test-run hook acts only on a Bash
	// command that invokes one of them.
	commands?: [...string]
}

// #Hooks configures opt-in hook behaviours. test_run (TR-9, ADR-056
// decision 6) turns on the test-run hook: one line after a declared test
// command fails, naming the failing tests and the changed symbols they
// reach. Unset means off (default off until the level 10 bench).
// pin_check (PR-6, ADR-057 decision 4) is the commit pin line's mode: off
// (no check), record (check and log a pin beat, output unchanged) or
// feedback (the lost-pin lines also reach the agent). Unset means record
// (until PR-M1 passes). claim_check (RO-38, RO-39) turns on the stop claim
// check and its SubagentStop hook entry; gate_preflight (RO-40) turns on
// the advisory line before a Bash `git commit` or `git push`;
// read_outline_hint (H8) turns on the outline line before a whole Read of a
// long source file and its PreToolUse(Read) hook entry; tests_to_run (RO-43,
// H4) turns on the line after an edit of a Go file naming the tests that
// reach it and their go test recipe; prompt_definitions
// (H10) adds one line to the prompt hook naming each identifier it resolved
// with its definition file:line. Unset means off for all five. "observe"
// computes and logs with emitted=false without injecting the line.
#Hooks: {
	subagent_map_facts?: bool | "observe"
	// Unchanged edit/cwd context is injected once per session (default on).
	dedupe?:             bool
	test_run?:           bool
	pin_check?:          "off" | "record" | "feedback"
	claim_check?:        bool | "observe"
	gate_preflight?:     bool | "observe"
	read_outline_hint?:  bool | "observe"
	tests_to_run?:       bool | "observe"
	prompt_definitions?: bool | "observe"
}

// #Readouts configures the readout hook's modes (RO-15, ADR-071, docs/
// design/readouts.md). Every key is the user's own switch: read from
// ~/.fathom/config.toml only, dropped and named when a repository's
// .fathom/config.toml sets one. Nothing reads it until the readout hook
// lands (RO-16); unset, the mode is off.
#Readouts: {
	// mode: off (default), observe (log only), lean, rich.
	mode?: #ReadoutMode
	// roots: the directory prefixes under which observe, lean and rich
	// apply; every root when unset.
	roots?: [...(string & !="")]
	// quiet_prefixes: extra quiet heads (RO-28), each a command name or its
	// leading words; a command's leading `&&`-joined segments that run one
	// are read past, like `cd` and `export`.
	quiet_prefixes?: [...(string & !="")]
	// rules.<id>.mode overrides mode for one rule.
	rules?: [string]: {
		mode?: #ReadoutMode
	}
	// trust: the sha256 (lowercase hex) of each repository
	// `.fathom/readouts.toml` the user trusts (RO-33). User-only.
	trust?: [...(string & =~"^[0-9a-f]{64}$")]
}

#ReadoutMode: "off" | "observe" | "lean" | "rich"

// #SurfacePattern is one `[[surface.pattern]]` entry (ADR-044 §2.1): a
// repo-declared capability-inventory source `internal/analysis/inventory.
// Extract` cannot derive from a built-in kind (api/json/route/env/rpc/
// cli) — this repository's own MCP tool names and CLI verbs, for instance,
// neither of which goes through a typed tool registry or cobra. kind
// becomes the "<kind>:" prefix on every match; glob (internal/filter's
// gitignore-style syntax, ** included; "" matches every file) scopes which
// files are scanned; regex's first capturing group names the matched
// item.
#SurfacePattern: {
	kind:  string
	glob?: string
	regex: string
}

// #Surface configures ADR-044 §2.1's repo-declared capability-inventory
// patterns: any number of `[[surface.pattern]]` blocks.
#Surface: {
	pattern?: [...#SurfacePattern]
}

// #Connector configures discovery (RT-145, docs/design/responsible-
// tenant.md §4.2). include re-includes directories the default cache list
// (.cache, .tools, target, dist, build, __pycache__, .venv, .gradle, .next,
// .turbo, coverage) prunes: a bare name re-includes every directory with
// that name, an entry with a slash one repo-relative path. Read from the
// corpus's own .fathom/config.toml; never re-includes vendor, .git,
// testdata or the other always-skipped directories. respect_gitignore
// (RT-68, §4.2) discovers a git work tree through `git ls-files -co
// --exclude-standard` — skipDir and the default list still applied after
// it — and stays off by default until RT-M11 has measured every service.
#Connector: {
	include?: [...string]
	respect_gitignore?: bool
}

// #Typed configures the typed Go pass (EW-68). skip_modules lists
// repo-relative directory globs (internal/pathglob syntax, ** included)
// naming the nested Go modules the pass does not load: each keeps its
// syntax-pass edges and is named, with reason skip_modules, in the pass's
// one stderr line; the root module is never skipped. A measurement-only
// module (a calibration oracle with a third-party tree to compile) is the
// case: on a cold build cache its load can cost minutes. Read from the
// corpus's own .fathom/config.toml only; an absolute or escaping glob is
// refused (fathom config names it). A changed list re-runs the pass at the
// next ingest.
#Typed: {
	skip_modules?: [...(string & !="")]
}

// #Gate configures local gate evidence; acceptance defaults off.
#Gate: {
	accept_receipts?: bool
	allow_receipt_acceptance?: bool
	receipts_dir?:    string
	host_label?:      string
}

// #Receipts configures what a portable receipt states about the repository
// it was produced in. repository_uri is the absolute URI (a scheme, then no
// whitespace) a receipt's SARIF names as versionControlProvenance's
// repositoryUri; unset names none. fathom never derives one from a git
// remote (ADR-011): a repository that wants it in its receipts states it.
#Receipts: {
	repository_uri?: =~"^[A-Za-z][A-Za-z0-9+.-]*:[^[:space:]]+$" | error("[receipts] repository_uri must be an absolute URI with a scheme (https://..., ssh://..., file:...): the repositoryUri a SARIF versionControlProvenance carries")
}

// #Tenancy is the responsible-tenant budget (ADR-047,
// docs/design/responsible-tenant.md §1): what the daemon may cost the
// machine it shares. It is a MACHINE contract, read from the user layer
// (~/.fathom/config.toml) and env only: a [tenancy] table in a repository's
// .fathom/config.toml validates here (so a copied block is not a load
// failure) but is ignored and named (internal/config.ResolveTenancyConfig,
// doctor's tenancy_scope) — a cloned repository must not raise a laptop's
// ceiling (P13).
#Tenancy: {
	// budget is phys_footprint: "auto" = clamp(RAM/8, 1GiB, 4GiB).
	budget?: "auto" | #Size
	// The ladder's marks, × budget (§2.1). exit is capped at RAM/4.
	shed_at?:   number & >0 & <=1
	refuse_at?: number & >0 & <=2
	exit_at?:   number & >=1 & <=8
	// The rungs are ordered: shed ≤ refuse (checked here when both are
	// set; against the 0.75 default by ResolveTenancyConfig). An exit
	// mark below refuse is raised to it, named in the startup line.
	if shed_at != _|_ if refuse_at != _|_ {
		refuse_at: >=shed_at
	}

	// false keeps shed/refuse only (disclosed); an in-flight ingest is
	// always halted before any exit.
	exit_enabled?: bool
	// The runtime.SetMemoryLimit target: the exit mark by default.
	gc_hint?:           "exit" | "budget" | "off" | #Size
	admission?:         "observe" | "enforce" | "off"
	gomaxprocs?:        "auto" | (int & >=1)
	child_parallelism?: int & >=1
	notify?:            bool

	host?: {
		shed_on_pressure?:   #Pressure
		refuse_on_pressure?: #Pressure
		// Swap-volume floor applies at or above this pressure; unset = warn.
		floor_on_pressure?:     "normal" | "warn" | "critical"
		refuse_on_swap_growth?: #Size
		swap_growth_window?:    #Duration
		min_swap_volume_free?:  #Size
		min_store_volume_free?: #Size
		// × budget: host pressure moves the ladder only while footprint +
		// children is at or above it. Unset = 0.6 × shed_at (0.45); 0 = at
		// any footprint (the rule before 2026-10-19).
		allowance?: number & >=0 & <=1
	}
	breaker?: {
		safe_after?:  int & >=1
		safe_window?: #Duration
		stop_after?:  int & >=1
		stop_window?: #Duration
		warmup?:      #Duration
	}
	unmanaged?: {
		idle_exit?: #Duration
		ttl?:       #Duration
		budget?:    #Size
	}
	caches?: {
		fraction?:  number & >0 & <=1
		shed_trim?: number & >0 & <=1
		idle_drop?: #Duration
	}
}

// #Size is an IEC byte size ("512MiB", "4GiB"), GOMEMLIMIT's syntax.
#Size: string & =~"^[0-9]+(\\.[0-9]+)?(B|KiB|MiB|GiB|TiB)$"

// #Duration is a Go duration ("30m", "120s", "1h30m").
#Duration: string & =~"^([0-9]+(\\.[0-9]+)?(ns|us|ms|s|m|h))+$"

// #Pressure names a host memory-pressure level; "off" never triggers.
#Pressure: "warn" | "critical" | "off"

// #Config is the top-level shape of one config.toml layer (user or
// project). Both layers validate against the same schema; internal/config
// merges them field-by-field per docs/plans/config-experience.md's stated
// precedence (flags > env > project > user > built-in defaults).
#Config: {
	// default_profile names the profile used when neither --profile nor an
	// env-overlay value selects one explicitly.
	default_profile?: string

	// profiles is an open map of profile name to #Profile — any number of
	// named profiles, keyed however the operator likes.
	profiles?: [string]: #Profile

	daemon?:    #Daemon
	guards?:    #Guards
	findings?:  #Findings
	decide?:    #Decide
	tests?:     #Tests
	hooks?:     #Hooks
	surface?:   #Surface
	connector?: #Connector
	receipts?:  #Receipts
	gate?:      #Gate
	typed?:     #Typed
	tenancy?:   #Tenancy
	readouts?:  #Readouts
	shell?:     #Shell
	console?:   #Console
	work?:      #Work
	risk?:      #Risk
	learning?:  #Learning

	// zones (WI-1, docs/design/work-intelligence.md §2) names coarser
	// work-intelligence zones by repo-relative path globs (internal/
	// pathglob syntax, ** included): zone name to a non-empty glob list.
	// Unset, a zone is a Go package's directory or, for other languages,
	// the deepest directory holding indexed symbols.
	zones?: [string]: [...string] & [_, ...]
}

// #Risk is `[risk]`: the review risk thresholds `.fathom/risk.cue` carried
// (schema/risk). Every field is optional; unset keeps the binary's default.
// A repository's [risk] section wins over a risk.cue beside it, which doctor
// names.
#Risk: {
	blast_radius_high_callers?: int & >0
	zone_interest_medium_rate?: number & >0
}

// #Learning is `[learning]`: the learned-rule thresholds
// `.fathom/learning.cue` carried (schema/learning). Every field is optional;
// unset keeps the binary's default.
#Learning: {
	promote_corroborations?:      int & >0
	promote_min_sessions?:        int & >0
	promote_window_days?:         int & >0
	retire_refutes?:              int & >0
	retire_never_triggered_days?: int & >0
}

// #Work configures the work-intelligence pages (WI-2w, docs/design/
// work-intelligence.md §3 U1).
#Work: {
	// ci_summaries names each CI summary file /work reads CI minutes from:
	// JUnit XML (its `time` totals) or a workflow-run JSON export, by a
	// repo-relative or absolute path, with the commit it built. A
	// workflow run's own head_sha wins; a JUnit file must name the commit.
	ci_summaries?: [...#CISummary]
}

// #CISummary is one `[[work.ci_summaries]]` entry.
#CISummary: {
	path:    string & !=""
	commit?: string & =~"^[0-9a-f]{7,64}$"
}

// #Console bounds the Observatory console's own work (RT-118,
// docs/design/responsible-tenant.md §14).
#Console: {
	// conserve_max_files is the widest range, in changed files, /refactors
	// computes conserve for (default 500); a wider one is refused by name
	// and left to `fathom conserve --since`.
	conserve_max_files?: int & >=1
}

// #ShellWrapper is either a transparent argv prefix or a script argument.
#ShellWrapper: {
	prefix?:     string & =~"[^[:space:]]"
	command?:    string & =~"^[A-Za-z0-9_./-]+$"
	script_arg?: int & >=1
	cwd_arg?:    int & >=1
	if script_arg != _|_ {
		if cwd_arg != _|_ {cwd_arg: !=script_arg}
	}
	if prefix != _|_ {
		command?:    _|_
		script_arg?: _|_
		cwd_arg?:    _|_
	}
	if prefix == _|_ {
		command:    string
		script_arg: int
	}
}
#Shell: wrappers?: [...#ShellWrapper]
