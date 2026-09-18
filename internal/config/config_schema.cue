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

	daemon?:   #Daemon
	guards?:   #Guards
	findings?: #Findings
}
