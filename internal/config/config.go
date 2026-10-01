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
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
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
	RouteTimeoutMS int `toml:"route_timeout_ms"`
	// RankWithinTier gates ADR-035's obligations within-tier re-rank.
	// Default false (amended under ADR-042, 2026-09-23): the decision-port
	// A/B measured it net negative on governed edits once a real provider
	// is configured (docs/bench/readings/decision-port-ab-2026-09-23.md) —
	// set true explicitly to opt back in.
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

	// Sites is the egress allowlist for ADR-044 decision 8 (I14: "the only
	// off-host egress is LLM compilation and, since ADR-042, the decision
	// port"): the site names allowed to send a decision.Question off-host
	// through this Decide port — "route" (POST /route), "compile_prefilter"
	// (the compile pre-filter, DP-9), "obligations_rerank" (the obligations
	// within-tier re-rank, DP-7/DP-8), "certify_suggest" (`fathom certify
	// suggest`, TC-24), "assess" (`fathom assess`, ADR-046 — also gated
	// by the default-off assess capability) and "preflight" (`fathom plans
	// context --preflight --score`, WI-18 — asked only on request). A site not listed here makes NO call at all,
	// regardless of Provider — internal/decide.SiteAllowed is the single
	// gate every call site consults. A present-but-empty list (`sites = []`)
	// disables every site; an absent key (the common case) leaves the
	// loaded default in place (DefaultDecide.Sites: every current site, so
	// unconfigured behaviour is unchanged) exactly like every other
	// present-key-replaces-default list field in this struct (APIKeyCmd
	// above, Findings.Producers).
	Sites []string `toml:"sites"`
}

// DefaultDecide is the zero-configuration [decide] block: provider "none"
// (no call ever made), the vision-32-documented thresholds, and
// rank_within_tier OFF. Load seeds Config.Decide with this — the one place
// in this package where a block's default is a non-zero Go value rather
// than "absent means the zero value" (Findings, Guards, Daemon).
//
// rank_within_tier defaulted true through ADR-042's original landing; the
// 2026-09-23 decision-port A/B (docs/bench/readings/decision-port-ab-
// 2026-09-23.md) measured it net NEGATIVE on governed edits once a real
// decision provider is configured — guards-jev-r1's paired mean was
// -0.19 [-0.375,-0.062], 3 tasks worse and 0 better, the LLM re-rank
// sometimes pushing the one governing rule to the last visible slot on a
// later touch of the same file — while routing and the compile
// pre-filter (unchanged here) showed a real, if small, positive. Amended
// under ADR-042 (ADR.md); a repo that wants the re-rank back sets
// `rank_within_tier = true` explicitly.
var DefaultDecide = Decide{
	Provider:         "none",
	CompileThreshold: 0.5,
	RouteConfidence:  0.9,
	RouteTimeoutMS:   3000,
	RankWithinTier:   false,
	RerankTimeoutMS:  1500,
	// Every current site (ADR-044 decision 8): the default allowlist keeps
	// every existing site's behaviour unchanged — Provider "none" still
	// makes no call either way, and a real provider calls every site it
	// always called before Sites existed. A repo narrows this explicitly.
	// "assess" (ADR-046) joins the default like every other site: the
	// provider is the egress arm, and assess additionally needs its own
	// default-off capability and an explicit invocation before it spends.
	Sites: []string{"route", "compile_prefilter", "obligations_rerank", "certify_suggest", "assess", "preflight"},
}

// Tests configures vision 33 / ADR-043 stage 0's test-evidence ingest
// (schema.cue #Tests): `[tests] results_dir` makes a plain `fathom ingest`
// also pick up attested #TestResult envelopes, exactly as `fathom tests
// ingest --results` does explicitly.
type Tests struct {
	ResultsDir   string `toml:"results_dir"`
	ArtifactsDir string `toml:"artifacts_dir"`
	// Commands (TR-1, docs/plans/test-run-hook.md) is the repository's own
	// list of declared test commands ("go test", "bin/itest"), beside the
	// ones read from its Taskfile, Makefile and package.json: the test-run
	// hook acts only on a Bash command that invokes one of them. The layer
	// that sets it replaces the whole list.
	Commands []string `toml:"commands"`
}

// Hooks configures opt-in hook behaviours (schema.cue #Hooks).
type Hooks struct {
	// TestRun (TR-9, ADR-056 decision 6) turns on the test-run hook: after
	// a Bash command that runs a declared test command, one line naming
	// the failing tests and the changed symbols they reach. Default false
	// until the level 10 bench (TR-M1) reads PASS.
	TestRun bool
	// PinCheck (PR-6, ADR-057 decision 4) is the commit pin line's mode:
	// PinCheckOff (no check, no beat), PinCheckRecord (check and beat,
	// stdout unchanged) or PinCheckFeedback (the lost-pin lines reach the
	// agent too). Load sets PinCheckRecord when no layer names it: record
	// mode until PR-M1 passes.
	PinCheck string
}

// The [hooks] pin_check values (schema.cue #Hooks).
const (
	PinCheckOff      = "off"
	PinCheckRecord   = "record"
	PinCheckFeedback = "feedback"
)

// Console configures the Observatory console's bounds (schema.cue
// #Console; RT-118, docs/design/responsible-tenant.md §14):
// conserve_max_files is the widest range, in changed files, /refactors
// computes conserve for (0: the built-in 500); a wider one is refused by
// name and left to the CLI verb.
type Console struct {
	ConserveMaxFiles int `toml:"conserve_max_files"`
}

// Work configures the work-intelligence pages (schema.cue #Work; WI-2w,
// docs/design/work-intelligence.md §3 U1): the CI summary files /work
// reads CI minutes from.
type Work struct {
	CISummaries []CISummary
}

// CISummary is one `[[work.ci_summaries]]` entry: a JUnit XML file or a
// workflow-run JSON export (Path, repo-relative or absolute) and the
// commit it built. Commit may be "" only when the file names its own (a
// workflow run's head_sha); a JUnit file without one is refused by name.
type CISummary struct {
	Path   string
	Commit string
}

// SurfacePattern is one `[[surface.pattern]]` entry (ADR-044 §2.1): a
// repo-declared capability-inventory source `internal/analysis/inventory.
// Extract` cannot derive from a built-in kind (api/json/route/env/rpc/
// cli) — this repository's own MCP tool names and CLI verbs, for
// instance, since it dispatches neither through a typed tool registry nor
// cobra. Kind becomes the "<kind>:" prefix on every match; Glob (internal/
// filter's gitignore-style syntax, ** included; "" matches every file)
// scopes which files are scanned; Regex's first capturing group names the
// matched item.
type SurfacePattern struct {
	Kind  string `toml:"kind"`
	Glob  string `toml:"glob"`
	Regex string `toml:"regex"`
}

// Surface configures ADR-044 §2.1's repo-declared capability-inventory
// patterns (schema.cue #Surface): `[[surface.pattern]]` blocks.
type Surface struct {
	Pattern []SurfacePattern
}

// Connector configures discovery (schema.cue #Connector, RT-145):
// `[connector] include` re-includes directories the default cache list
// (.cache, .tools, target, dist, build, __pycache__, .venv, .gradle, .next,
// .turbo, coverage) would prune — a bare name re-includes every directory
// with that name, an entry with a slash one repo-relative path. The
// connector reads it from the corpus's own .fathom/config.toml.
type Connector struct {
	Include []string
	// RespectGitignore discovers a git work tree through `git ls-files`
	// (RT-68); off by default until RT-M11.
	RespectGitignore bool
}

// Config is the merged, in-memory form of one or more config.toml layers.
type Config struct {
	Connector      Connector
	DefaultProfile string
	Profiles       map[string]Profile
	Daemon         Daemon
	Guards         Guards
	Findings       Findings
	Decide         Decide
	Tests          Tests
	Hooks          Hooks
	Surface        Surface
	Console        Console
	Work           Work
	// Zones is `[zones]` (WI-1): zone name to its repo-relative path
	// globs. A layer that sets it replaces the whole table.
	Zones map[string][]string
}

// SurfacePatterns returns cfg's configured `[[surface.pattern]]` entries
// (CV-17, ADR-044 §2.1) — a named accessor so a caller (internal/conserve's
// surface plane, internal/certify's behaviour-target resolver) need not
// know the field lives nested under Surface.
func SurfacePatterns(cfg Config) []SurfacePattern {
	return cfg.Surface.Pattern
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
	return fathomhome.Path("config.toml")
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

	// ProjectTenancyIgnored is true when the project layer carries a
	// [tenancy] table (RT-30, design §1): budgets are machine contracts,
	// read at user scope only, so it is ignored and doctor's tenancy_scope
	// names it.
	ProjectTenancyIgnored bool

	// userTenancy is the user layer's raw [tenancy] table, the only one
	// ResolveTenancyConfig reads.
	userTenancy map[string]any
}

// Load reads the user config layer then the project config layer (either
// path may be "" to skip that layer) and merges them field-by-field —
// project wins over user, matching the file's stated precedence tier
// (flags > env > project > user > defaults). A layer that does not exist on
// disk is skipped, not an error; a layer that exists but fails to parse or
// validate is a *LoadError naming that file.
func Load(userPath, projectPath string) (Loaded, error) {
	out := Loaded{
		Config:      Config{Profiles: map[string]Profile{}, Decide: DefaultDecide, Hooks: Hooks{PinCheck: PinCheckRecord}},
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
			// RT-145: `[connector] include` is a property of the corpus;
			// the connector reads it only from the corpus's own
			// .fathom/config.toml. In the user layer it would validate,
			// show in `fathom config --explain`, and change nothing.
			if cm, ok := raw["connector"].(map[string]any); ok {
				if _, ok := cm["include"]; ok {
					return Loaded{}, &LoadError{Path: userPath, Err: fmt.Errorf("connector.include is read only from a corpus's own .fathom/config.toml; move it there")}
				}
			}
			mergeLayer(&out.Config, out.Provenance, raw, SourceUser, userPath)
			if tm, ok := raw["tenancy"].(map[string]any); ok {
				out.userTenancy = tm
				recordTenancyProvenance(out.Provenance, tm, "tenancy", userPath)
			}
		}
	}

	// F8 (the wave-B live reading): run from $HOME, the "project" layer is
	// the user's own ~/.fathom/config.toml. It is read once, as the user
	// layer — never again as a repository whose [tenancy] is ignored.
	if projectPath != "" && userPath != "" && sameFile(userPath, projectPath) {
		projectPath = ""
		out.ProjectPath = ""
	}

	if projectPath != "" {
		raw, found, err := readLayer(projectPath)
		if err != nil {
			return Loaded{}, err
		}
		out.ProjectFound = found
		if found {
			mergeLayer(&out.Config, out.Provenance, raw, SourceProject, projectPath)
			// RT-30: a repository's [tenancy] validates but is never merged.
			if _, ok := raw["tenancy"]; ok {
				out.ProjectTenancyIgnored = true
			}
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

// sameFile reports whether a and b name one existing file (any spelling:
// a symlinked home, a relative path).
func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}
