package config

// scope.go — SEC-2 (ADR-042 amendment 2026-10-09, docs/plans/config-
// experience.md CF-14..CF-17): every config field has a scope class. A
// repository's .fathom/config.toml may choose behaviour (provider, model,
// thresholds, include lists, repository-relative paths); it may never name
// an executable fathom runs, name a credential, name a path outside the
// repository, or switch on automatic execution. Those fields are user-only:
// Load drops them from the project layer and names them
// (Loaded.IgnoredKeySources). TestEveryConfigFieldHasAScopeClass walks the
// embedded #Config schema against this registry, so a new field cannot land
// unclassified.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScopeClass is what a repository's own .fathom/config.toml may do with a
// field.
type ScopeClass string

const (
	// ScopeBehaviour: either layer may set it; the project layer wins.
	ScopeBehaviour ScopeClass = "behaviour"
	// ScopeRepoPath: a repository-relative path; either layer may set it,
	// and every use site resolves it with RepoPath, which refuses an
	// absolute value, a `..` escape and a symlink escape by name.
	ScopeRepoPath ScopeClass = "repo-path"
	// ScopeUserOnly: read from ~/.fathom/config.toml only; Load drops it
	// from the project layer and names file and field, never the value.
	ScopeUserOnly ScopeClass = "user-only"
)

// FieldScope is one config field's class and the evidence for it.
//
// Key is the field's dotted path in the schema: "*" stands for a map key
// the operator names ("profiles.*.bin", "zones.*"), "[]" for an element of
// an array of tables ("work.ci_summaries[].path"), and a trailing ".**"
// covers every key under a table ("tenancy.**").
type FieldScope struct {
	Key   string
	Class ScopeClass
	Why   string
}

// Short reasons, shared by the user-only rows (the warning names the
// reason, never the value).
const (
	whyExecutable = "names an executable fathom runs"
	whyCredential = "names a credential or the command that prints one"
	whyKeyHost    = "names the host a profile sends your API key to"
	whyMachine    = "configures this machine's daemon, not one repository"
	whyAutoExec   = "switches on automatic execution (re-ingest runs the language toolchain in every repository)"
	whyReadouts   = "the user's own switch for rewriting what a tool result shows the agent; a cloned repository must not turn it on"
	whyConsent    = "the user's own consent to or kill of the decision port (ADR-073, I66)"
)

// fieldScopes is the registry, sorted by Key.
var fieldScopes = []FieldScope{
	{"connector.include", ScopeRepoPath, "directory names or repo-relative paths discovery re-includes (internal/connector/defaultignore.go)"},
	{"connector.respect_gitignore", ScopeBehaviour, "discovery reads `git ls-files` (fixed argv, a read)"},
	{"console.conserve_max_files", ScopeBehaviour, "a bound on the console's conserve range"},
	{"daemon.addr", ScopeUserOnly, whyMachine + ": the address serve-http listens on (servehttp_config.go)"},
	{"daemon.auto_refresh", ScopeUserOnly, whyAutoExec},
	{"daemon.call_log", ScopeUserOnly, whyMachine + ": a path the daemon writes, anywhere on disk"},
	{"daemon.default_profile", ScopeBehaviour, "chooses a profile by name; a profile's executable and key host are user-only"},
	{"daemon.memory_limit", ScopeUserOnly, whyMachine + ": a memory ceiling, like [tenancy] (P13)"},
	{"daemon.state_dir", ScopeBehaviour, "the one value, \"workspace\", keeps runtime state inside the repository (statedir.Resolve)"},
	{"daemon.store", ScopeUserOnly, whyMachine + ": the store path fathom writes (the session hook's findings import passes it to --store)"},
	{"decide.allow_repo_provider", ScopeUserOnly, whyConsent + ": lets a provider only the repository names open"},
	{"decide.api_key_cmd", ScopeUserOnly, whyCredential},
	{"decide.api_key_env", ScopeUserOnly, whyCredential},
	{"decide.api_key_file", ScopeUserOnly, whyCredential},
	{"decide.base_url", ScopeBehaviour, "a key goes only to the provider's own host, a loopback one, or one a user-layer base_url names (internal/decide checkKeyDestination)"},
	{"decide.compile_threshold", ScopeBehaviour, "a threshold"},
	{"decide.deny_sites", ScopeUserOnly, whyConsent + ": refuses a site whatever any list says"},
	{"decide.enabled", ScopeUserOnly, whyConsent + ": false closes the port, a running daemon included"},
	{"decide.endpoint", ScopeBehaviour, "base_url's older name, under the same key-host rule"},
	{"decide.exclude", ScopeBehaviour, "privacy path globs whose spans never leave for a decision provider; every layer's globs union, so a repository adds and never removes the user's (I61)"},
	{"decide.max_asks_per_site_per_session", ScopeBehaviour, "a cap, the minimum across layers: a repository may lower the user's and never raise it"},
	{"decide.model", ScopeBehaviour, "chooses a model"},
	{"decide.profile", ScopeBehaviour, "chooses a profile by name"},
	{"decide.provider", ScopeBehaviour, "chooses a provider; one only the repository names opens only with the user's allow_repo_provider (internal/decide, I66)"},
	{"decide.rank_within_tier", ScopeBehaviour, "a ranking choice"},
	{"decide.rerank_timeout_ms", ScopeBehaviour, "a threshold"},
	{"decide.route_confidence", ScopeBehaviour, "a threshold"},
	{"decide.route_timeout_ms", ScopeBehaviour, "a threshold"},
	{"decide.session_sites", ScopeUserOnly, whyConsent + ": the only list that opts in a session site (readout_mode, compact_advice)"},
	{"decide.sites", ScopeBehaviour, "the sites a decision may come from; an opt-in site only from the repository (JF-4), a session site never (I60)"},
	{"decide.transport", ScopeBehaviour, "a key stays with the transport it was named for (DP-39); one only the repository names opens only with the user's allow_repo_provider (I66)"},
	{"default_profile", ScopeBehaviour, "chooses a profile by name"},
	{"findings.outbox", ScopeRepoPath, "a repo-relative file the session hook imports (findings_outbox.go)"},
	{"findings.producers", ScopeBehaviour, "an allow-list of producer ids"},
	{"findings.subject", ScopeBehaviour, "the review-kit subject this repository maps to"},
	{"guards.enforce", ScopeBehaviour, "turns the repository's own guards advisory"},
	{"hooks.claim_check", ScopeBehaviour, "the claim check runs nothing: it reads the final message and the hook log's test-run rows (claimcheck.go)"},
	{"hooks.gate_preflight", ScopeBehaviour, "the gate preflight runs nothing: it lexes the command, reads git and the hook log, and asks the claim-moment verify path (gatepreflight.go)"},
	{"hooks.pin_check", ScopeBehaviour, "the commit pin line's output mode; the check itself is on by default"},
	{"hooks.read_outline_hint", ScopeBehaviour, "the Read outline hint runs nothing: it reads the file the Read names and runs fathom's own extractors over it in process (readouthint.go)"},
	{"hooks.test_run", ScopeBehaviour, "the test-run hook runs nothing: it reads the output of a command the harness ran (TR-X2, testrunhook.go)"},
	{"profiles.*.api_key_cmd", ScopeUserOnly, whyCredential},
	{"profiles.*.api_key_env", ScopeUserOnly, whyCredential},
	{"profiles.*.api_version", ScopeBehaviour, "an Azure API version"},
	{"profiles.*.bin", ScopeUserOnly, whyExecutable + " (the claude-cli and codex-cli lanes exec it; codex-cli at construction)"},
	{"profiles.*.deployment", ScopeBehaviour, "an Azure deployment name"},
	{"profiles.*.endpoint", ScopeUserOnly, whyKeyHost + " (the openai lane defaults its key to OPENAI_API_KEY)"},
	{"profiles.*.model", ScopeBehaviour, "chooses a model"},
	{"profiles.*.provider", ScopeBehaviour, "chooses a provider"},
	{"profiles.*.region", ScopeBehaviour, "an AWS region"},
	{"profiles.*.timeout", ScopeBehaviour, "a bound on one model call"},
	{"readouts.mode", ScopeUserOnly, whyReadouts + ": the readout hook's default mode (RO-15, ADR-071)"},
	{"readouts.roots", ScopeUserOnly, whyReadouts + ": the directory prefixes the modes apply under"},
	{"readouts.rules.*.mode", ScopeUserOnly, whyReadouts + ": one rule's mode"},
	{"receipts.repository_uri", ScopeBehaviour, "a URI SARIF's versionControlProvenance names; fathom never derives one from a git remote (ADR-011)"},
	{"surface.pattern[].glob", ScopeBehaviour, "a glob over the repository's files"},
	{"surface.pattern[].kind", ScopeBehaviour, "a label"},
	{"surface.pattern[].regex", ScopeBehaviour, "an RE2 pattern over the repository's files"},
	{"tenancy.**", ScopeUserOnly, whyMachine + ": the budget is a machine contract (RT-30, P13)"},
	{"tests.artifacts_dir", ScopeRepoPath, "a repo-relative directory of test artifacts (read by no code yet)"},
	{"tests.commands", ScopeBehaviour, "declared test commands the test-run hook matches; no automatic path runs them"},
	{"tests.results_dir", ScopeRepoPath, "a repo-relative directory a plain ingest reads envelopes from (ingest.go, conserve/coverageload.go)"},
	{"typed.skip_modules", ScopeRepoPath, "repo-relative directory globs naming nested Go modules the typed pass does not load (internal/fusion/typed_skipmodules.go)"},
	{"work.ci_summaries[].commit", ScopeBehaviour, "a commit id"},
	{"work.ci_summaries[].path", ScopeRepoPath, "a repo-relative CI summary file the console reads (console_work_signals.go)"},
	{"zones.*", ScopeBehaviour, "repo-relative path globs"},
}

// ScopeOf returns the scope of the field at key, a dotted path in the
// registry's spelling ("profiles.*.bin", "tenancy.budget"): an exact entry,
// else the nearest ".**" entry above it. Matching is exact and
// case-sensitive, like TOML keys.
func ScopeOf(key string) (FieldScope, bool) {
	for _, fs := range fieldScopes {
		if fs.Key == key {
			return fs, true
		}
	}
	for p := key; p != ""; {
		for _, fs := range fieldScopes {
			if fs.Key == p+".**" {
				return fs, true
			}
		}
		i := strings.LastIndex(p, ".")
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return FieldScope{}, false
}

// RepoMaySet reports whether a repository's .fathom/config.toml may set the
// field at key: true for behaviour and repo-path fields, false for
// user-only and unknown ones. A raw-bytes pre-gate (the test-run hook's)
// consults it so a dropped field never opens a gate.
func RepoMaySet(key string) bool {
	fs, ok := ScopeOf(key)
	return ok && fs.Class != ScopeUserOnly
}

// registryChild names the registry spelling of child k under the dotted
// position norm: k itself when the registry names it, else "*" when the
// position is a map the operator keys (profiles, zones), else k.
func registryChild(norm, k string) string {
	exact := joinKey(norm, k)
	wild := joinKey(norm, "*")
	for _, fs := range fieldScopes {
		if fs.Key == exact || strings.HasPrefix(fs.Key, exact+".") || strings.HasPrefix(fs.Key, exact+"[]") {
			return k
		}
	}
	for _, fs := range fieldScopes {
		if fs.Key == wild || strings.HasPrefix(fs.Key, wild+".") {
			return "*"
		}
	}
	return k
}

func joinKey(p, k string) string {
	if p == "" {
		return k
	}
	return p + "." + k
}

// The named refusals RepoPath returns (errors.Is matches each).
var (
	ErrRepoPathEmpty    = errors.New("empty path")
	ErrRepoPathAbsolute = errors.New("absolute path: a repository config names only paths inside the repository")
	ErrRepoPathEscapes  = errors.New("escapes the repository through ..")
	ErrRepoPathSymlink  = errors.New("escapes the repository through a symlink")
)

// RepoPathError is RepoPath's refusal: the value and the named reason.
type RepoPathError struct {
	Value string
	Err   error
}

func (e *RepoPathError) Error() string { return fmt.Sprintf("%q: %v", e.Value, e.Err) }

func (e *RepoPathError) Unwrap() error { return e.Err }

// RepoPath resolves rel, a repo-path field's value, against the repository
// root and refuses it when it leaves the repository: an absolute value
// (ErrRepoPathAbsolute), a `..` escape (ErrRepoPathEscapes), or a symlink
// on the existing part of the path that resolves outside root
// (ErrRepoPathSymlink). It returns root joined with the cleaned rel. The
// symlink check resolves the longest prefix that exists today; a part that
// does not exist yet cannot be a link.
func RepoPath(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", &RepoPathError{Value: rel, Err: ErrRepoPathEmpty}
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || filepath.VolumeName(rel) != "" {
		return "", &RepoPathError{Value: rel, Err: ErrRepoPathAbsolute}
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", &RepoPathError{Value: rel, Err: ErrRepoPathEscapes}
	}
	joined := filepath.Join(root, clean)

	// Compare absolute, symlink-resolved paths: a relative root (".") and
	// a root reached through a link are still the repository.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", &RepoPathError{Value: rel, Err: err}
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		realRoot = absRoot
	}
	existing, rest := filepath.Join(absRoot, clean), ""
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
	real, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", &RepoPathError{Value: rel, Err: err}
	}
	real = filepath.Join(real, rest)
	if real != realRoot && !strings.HasPrefix(real, realRoot+string(filepath.Separator)) {
		return "", &RepoPathError{Value: rel, Err: ErrRepoPathSymlink}
	}
	return joined, nil
}

// RepoPathRefusal is one repo-path value RepoPath refuses: the dotted
// field ("connector.include[0]") and the named refusal.
type RepoPathRefusal struct {
	Field string
	Err   error
}

// RefusedRepoPaths checks every repo-path field cfg sets against the
// repository root and returns the values RepoPath refuses, in field order.
// Each use site refuses the same value on its own; this is the listing
// `fathom config` and doctor's key_source_scope row print.
func RefusedRepoPaths(root string, cfg Config) []RepoPathRefusal {
	var out []RepoPathRefusal
	check := func(field, v string) {
		if v == "" {
			return
		}
		if _, err := RepoPath(root, v); err != nil {
			out = append(out, RepoPathRefusal{Field: field, Err: err})
		}
	}
	for i, e := range cfg.Connector.Include {
		check(fmt.Sprintf("connector.include[%d]", i), e)
	}
	check("findings.outbox", cfg.Findings.Outbox)
	check("tests.artifacts_dir", cfg.Tests.ArtifactsDir)
	check("tests.results_dir", cfg.Tests.ResultsDir)
	for i, g := range cfg.Typed.SkipModules {
		check(fmt.Sprintf("typed.skip_modules[%d]", i), g)
	}
	for i, s := range cfg.Work.CISummaries {
		check(fmt.Sprintf("work.ci_summaries[%d].path", i), s.Path)
	}
	return out
}
