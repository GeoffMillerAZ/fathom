// Package config loads fathom's runtime configuration — named LLM-provider
// profiles plus daemon defaults — from TOML files, validated against a CUE
// schema (docs/plans/config-experience.md CF-1/CF-2).
//
// File format is TOML (humans author it); validation is CUE
// (schema/config/schema.cue, embedded byte-identical into config_schema.cue
// — see config_schema_test.go's drift gate and internal/semantic/manifest.go
// for the precedent this follows). A parsed TOML document is converted to a
// generic map, unified against the embedded #Config schema, and only then
// decoded into the typed Config below — so an unknown key (including a
// literal `api_key`, which the schema has no field for) or an out-of-enum
// value fails at load, naming the file and the key, never silently ignored.
//
// Precedence across layers is flags > env > project > user > built-in
// defaults (docs/plans/config-experience.md's Design section). This package
// owns the file layers (project, user) and the env-overlay; flags are the
// caller's concern (cmd/fathom threads a --profile flag through
// ResolveProfile). Load merges project over user field-by-field, tracking
// per-field Provenance so `fathom config --explain` can name, for every key,
// which file (or none) supplied it.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	toml "github.com/pelletier/go-toml/v2"
)

// configSchema is byte-identical to schema/config/schema.cue — kept in sync
// by TestConfigSchemaMatchesSource (schema_test.go).
//
//go:embed config_schema.cue
var configSchema []byte

// Well-known provider lanes (mirrors cmd/fathom's buildLLMClient switch,
// docs/plans/provider-matrix.md).
const (
	ProviderAnthropic   = "anthropic"
	ProviderClaudeCLI   = "claude-cli"
	ProviderOpenAI      = "openai"
	ProviderAzureOpenAI = "azure-openai"
	ProviderBedrock     = "bedrock"
	ProviderCodexCLI    = "codex-cli"
)

// Profile is one named provider configuration (schema.cue #Profile).
// APIKeyEnv/APIKeyCmd are indirection, never a literal secret — see
// ResolveAPIKey.
type Profile struct {
	Provider   string
	Model      string
	Endpoint   string
	Region     string
	Deployment string
	APIVersion string
	Bin        string
	APIKeyEnv  string
	APIKeyCmd  []string
}

// Daemon configures serve-http's own startup defaults (schema.cue #Daemon).
type Daemon struct {
	Store          string
	Addr           string
	CallLog        string
	AutoRefresh    *bool // nil = unset at every layer; caller applies its own default
	DefaultProfile string
	// StateDir is "" (default: state lives outside the repo tree,
	// internal/statedir.Dir) or "workspace" (O63 item 5's in-tree opt-out
	// — see schema.cue #Daemon.state_dir).
	StateDir string
	// MemoryLimit is Go size syntax ("3GiB") or "off" (W-N,
	// docs/plans/daemon-footprint.md): the soft limit serve-http applies
	// with runtime/debug.SetMemoryLimit at startup. "" = unset at every
	// layer; the caller's own default ("3GiB") applies — see
	// ParseMemoryLimit and schema.cue #Daemon.memory_limit's pattern.
	MemoryLimit string
}

// Guards configures ADR-038's enforcement escape hatch (schema.cue
// #Guards): `[guards] enforce = false` turns every enforce:true guard/
// layer-rule in .fathom/guard.cue back to advisory, repo-wide.
type Guards struct {
	Enforce *bool // nil = unset at every layer; caller applies its own default (true)
}

// Findings configures the findings-loop's governed external import
// (docs/vision/30-findings-loop.md §3.3/§3.7, schema.cue #Findings):
// `fathom findings import` refuses to operate without a provisioned
// Subject.
type Findings struct {
	Subject   string   `toml:"subject"`
	Outbox    string   `toml:"outbox"`
	Producers []string `toml:"producers"`
}

// Decide configures ADR-042's decision port (docs/vision/32-decision-port.md,
// schema.cue #Decide): typed fast decisions (choice/score/noul) behind a
// provider flag, off by default. `provider = "none"` (the default) makes no
// call and changes nothing; `provider = "llm"` routes through `profile`
// (empty = the compile lane's own default profile resolution); `provider =
// "typesafe"` routes through the kit's TypeSafe (Jev) adapter directly —
// Endpoint/Model/APIKeyEnv/APIKeyCmd configure that lane and are otherwise
// unused. CompileThreshold/RouteConfidence are always in [0,1] (enforced by
// the CUE schema); RankWithinTier gates ADR-035's obligations re-rank, and
// RerankTimeoutMS (DP-25) bounds how long that re-rank may wait for it.
type Decide struct {
	Provider         string  `toml:"provider"`
	Profile          string  `toml:"profile"`
	CompileThreshold float64 `toml:"compile_threshold"`
	RouteConfidence  float64 `toml:"route_confidence"`
	// RouteTimeoutMS (O109) bounds POST /route's own decider call in
	// milliseconds. Default 3000 fits Jev-class latency (70-500ms
	// measured) but starves an LLM lane under ordinary network latency
	// (1.5-2.8s warm, more cold, measured live against OpenRouter) — the
	// design point is a sub-second decision model; the prompt hook's own
	// shared deadline (~8s) still bounds the line regardless of this
	// value. CUE-enforced to [100,30000].
	RouteTimeoutMS int  `toml:"route_timeout_ms"`
	RankWithinTier bool `toml:"rank_within_tier"`
	// RerankTimeoutMS (DP-25, O109) bounds how long the obligations
	// re-rank (rerankWithinTiers, internal/semantic) may wait for the
	// Decider across EVERY chunk of one obligations call, in milliseconds
	// — a hard ceiling, not a per-chunk one: the whole call shares this
	// one budget. Default 1500 — deliberately BELOW route_timeout_ms's own
	// 3000ms default, and below even the more common "one sub-second
	// decision model" design point this whole port is built for, because
	// obligations (unlike /route) is on the PostToolUse edit hook's own
	// critical path: cmd/fathom/edithook.go's fetchObligations call gives
	// that hook a flat 2000ms HTTP client timeout for the whole RPC, and a
	// server that spends its entire budget on the rerank alone (with zero
	// left for store I/O, JSON encode/decode and the network round trip)
	// risks losing the race against that client deadline — which renders
	// NOTHING at all (edithook.go: "daemon down or error → silent"), a
	// strictly worse outcome than the rerank giving up early and serving
	// the untouched lexical order. 1500ms leaves 500ms of margin. An LLM
	// lane will usually exceed this default entirely (O109 measured a
	// single 20-question chunk at 8-18s against OpenRouter) — the design
	// point is a sub-second decision model (e.g. TypeSafe/Jev); when the
	// budget is exceeded the envelope discloses rank_note "decision budget
	// …" and serves rules in lexical order rather than waiting. Raise it
	// only for offline/non-hook use (`fathom obligations` from a terminal,
	// a bench run) where nothing is racing a client-side deadline.
	// CUE-enforced to [100,60000].
	RerankTimeoutMS int `toml:"rerank_timeout_ms"`

	// Endpoint overrides provider "typesafe"'s base URL (default
	// decision.DefaultTypeSafeBaseURL). Unused by "none"/"llm".
	Endpoint string `toml:"endpoint"`
	// Model overrides provider "typesafe"'s model (default
	// decision.DefaultTypeSafeModel, "jev-latest"). Unused by "none"/"llm".
	Model string `toml:"model"`
	// APIKeyEnv names an environment variable ResolveAPIKey reads for
	// provider "typesafe", at call time — never cached, never resolved
	// during Load (same discipline as Profile.APIKeyEnv).
	APIKeyEnv string `toml:"api_key_env"`
	// APIKeyCmd is an argv (no shell) ResolveAPIKey execs for provider
	// "typesafe"; wins over APIKeyEnv when both are set (same discipline as
	// Profile.APIKeyCmd).
	APIKeyCmd []string `toml:"api_key_cmd"`
}

// DefaultDecide is the zero-configuration [decide] block: provider "none"
// (no call ever made), the vision-32-documented thresholds, and
// rank_within_tier on. Load seeds Config.Decide with this — the one place
// in this package where a block's default is a non-zero Go value rather
// than "absent means the zero value" (Findings, Guards, Daemon).
var DefaultDecide = Decide{
	Provider:         "none",
	CompileThreshold: 0.5,
	RouteConfidence:  0.9,
	RouteTimeoutMS:   3000,
	RankWithinTier:   true,
	RerankTimeoutMS:  1500,
}

// Config is the merged, in-memory form of one or more config.toml layers.
type Config struct {
	DefaultProfile string
	Profiles       map[string]Profile
	Daemon         Daemon
	Guards         Guards
	Findings       Findings
	Decide         Decide
}

// LoadError names the file and the underlying cause — parse, validate, or
// decode — so an operator can act on it directly instead of a bare CUE
// diagnostic.
type LoadError struct {
	Path string
	Err  error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("config: %s: %v", e.Path, e.Err)
}

func (e *LoadError) Unwrap() error { return e.Err }

// DefaultUserPath returns ~/.fathom/config.toml, or "" if the user's home
// directory cannot be resolved.
func DefaultUserPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".fathom", "config.toml")
}

// DefaultProjectPath returns "<cwd>/.fathom/config.toml", or "" if the
// current working directory cannot be resolved.
func DefaultProjectPath() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return ""
	}
	return filepath.Join(wd, ".fathom", "config.toml")
}

// Loaded is the result of loading the user and project layers: the merged
// Config, per-field Provenance, and which layers actually existed on disk
// (an absent file is not an error — every layer is optional).
type Loaded struct {
	Config       Config
	Provenance   Provenance
	UserPath     string
	UserFound    bool
	ProjectPath  string
	ProjectFound bool
}

// Load reads the user config layer then the project config layer (either
// path may be "" to skip that layer) and merges them field-by-field —
// project wins over user, matching the file's stated precedence tier
// (flags > env > project > user > defaults). A layer that does not exist on
// disk is skipped, not an error; a layer that exists but fails to parse or
// validate is a *LoadError naming that file.
func Load(userPath, projectPath string) (Loaded, error) {
	out := Loaded{
		Config:      Config{Profiles: map[string]Profile{}, Decide: DefaultDecide},
		Provenance:  Provenance{},
		UserPath:    userPath,
		ProjectPath: projectPath,
	}

	if userPath != "" {
		raw, found, err := readLayer(userPath)
		if err != nil {
			return Loaded{}, err
		}
		out.UserFound = found
		if found {
			mergeLayer(&out.Config, out.Provenance, raw, SourceUser, userPath)
		}
	}

	if projectPath != "" {
		raw, found, err := readLayer(projectPath)
		if err != nil {
			return Loaded{}, err
		}
		out.ProjectFound = found
		if found {
			mergeLayer(&out.Config, out.Provenance, raw, SourceProject, projectPath)
		}
	}

	return out, nil
}

// readLayer reads and validates one config.toml file. found is false (with
// a nil error) when the file does not exist — that is not a load failure,
// every layer is optional.
func readLayer(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, &LoadError{Path: path, Err: err}
	}

	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		// go-toml's *DecodeError carries the position but its Error() text
		// drops it — surface line/column explicitly so a parse failure is
		// actionable (the plan's "errors name file, key, and expected
		// shape" contract, N4).
		var derr *toml.DecodeError
		if errors.As(err, &derr) {
			row, col := derr.Position()
			return nil, false, &LoadError{Path: path, Err: fmt.Errorf("parse TOML: line %d, column %d: %w", row, col, err)}
		}
		return nil, false, &LoadError{Path: path, Err: fmt.Errorf("parse TOML: %w", err)}
	}

	if err := validateAgainstSchema(raw); err != nil {
		return nil, false, &LoadError{Path: path, Err: err}
	}

	return raw, true, nil
}

// validateAgainstSchema unifies raw (a generic TOML-decoded document)
// against the embedded #Config definition and validates it concretely. A
// closed #Profile with no api_key field is what makes a literal secret in
// the file a validation error here, not a silently-dropped key.
func validateAgainstSchema(raw map[string]any) error {
	cctx := cuecontext.New()
	schema := cctx.CompileBytes(configSchema)
	if err := schema.Err(); err != nil {
		return fmt.Errorf("compile embedded schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Config"))
	if err := def.Err(); err != nil {
		return fmt.Errorf("look up #Config in embedded schema: %w", err)
	}

	dataVal := cctx.Encode(raw)
	if err := dataVal.Err(); err != nil {
		return fmt.Errorf("encode document: %w", err)
	}

	unified := def.Unify(dataVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return formatCUEError(err)
	}
	return nil
}

// formatCUEError renders a CUE validation error compactly, grouped by
// offending path, so a load failure prints each actual offending key ONCE
// instead of CUE's default multi-paragraph diagnostic. Two N5 fixes live
// here: the message body comes from Msg() (the bare format+args), not
// Error() — Error() already prepends the path, which produced
// "path: path: message" doubling — and errors are grouped per path,
// because a failed disjunction (e.g. an out-of-enum provider) yields one
// sub-error per disjunct, all on the same path.
func formatCUEError(err error) error {
	errs := cueerrors.Errors(err)
	if len(errs) == 0 {
		return err
	}
	byPath := map[string][]string{}
	var order []string
	for _, e := range errs {
		key := strings.Join(e.Path(), ".")
		format, args := e.Msg()
		msg := fmt.Sprintf(format, args...)
		if key == "" {
			msg = e.Error() // pathless: keep the full text
		}
		if _, seen := byPath[key]; !seen {
			order = append(order, key)
		}
		byPath[key] = append(byPath[key], msg)
	}
	sort.Strings(order)
	lines := make([]string, 0, len(order))
	for _, key := range order {
		msgs := strings.Join(byPath[key], "; ")
		if key == "" {
			lines = append(lines, msgs)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", key, msgs))
	}
	return fmt.Errorf("%s", strings.Join(lines, " | "))
}
