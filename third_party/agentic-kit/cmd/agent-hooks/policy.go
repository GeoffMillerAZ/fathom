package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Hosts this binary speaks. The values appear in hook commands
// ("agent-hooks hook --host claude") and in ledger records.
const (
	hostClaude = "claude"
	hostCodex  = "codex"
)

// Fork modes for Claude Code forks, which inherit the session model and
// cannot take a model parameter.
const (
	forkAllow  = "allow"  // forks pass silently
	forkMarker = "marker" // forks need Spawn.ForkMarker in the brief
	forkDeny   = "deny"   // forks are always denied
)

// Policy limits. They bound the work a hook does on every spawn and keep a
// malformed file from turning the hot path into a regex or memory problem.
// Text limits count runes, as schema.cue's strings.MaxRunes does.
const (
	policyMaxBytes   = 256 << 10
	maxTiers         = 8
	maxModelsPerTier = 32
	maxAdvice        = 32
	maxTextRunes     = 1024
	maxPatternLen    = 1024
	minLedgerBytes   = 1 << 20
	maxLedgerBytes   = 1 << 30
	minContainChars  = 2000
	maxContainChars  = 150000 // Claude Code's own Bash output ceiling
	maxDenyLoopLimit = 20
)

//go:embed assets/policy.default.json
var defaultPolicyJSON []byte

// Policy is the consumer-owned token-economics policy. The authoring contract
// is schema.cue; parsePolicy enforces the same rules at runtime with the
// stdlib, including exact key spelling and required fields, so a policy that
// `cue vet` rejects is rejected here too and the reverse.
type Policy struct {
	// Version is the policy format version; only 1 exists.
	Version int `json:"version"`
	// Tiers lists model tiers from cheapest to strongest. Order is meaning:
	// class ceilings and escalation compare tier positions.
	Tiers []Tier `json:"tiers"`
	// Classes sets the tier ceiling for worker and critic briefs.
	Classes Classes `json:"classes"`
	// Spawn holds the per-spawn requirements.
	Spawn SpawnRules `json:"spawn"`
	// Advice is optional once-per-session guidance matched against briefs.
	Advice []Advice `json:"advice,omitempty"`
	// Ledger configures the local measurement ledger.
	Ledger LedgerConfig `json:"ledger"`
	// MainThread keeps the orchestrating session's context lean.
	MainThread MainThreadConfig `json:"main_thread"`

	// compiled forms, filled by validate.
	criticRE *regexp.Regexp
	adviceRE []compiledAdvice
}

// Tier is one rung of the model ladder.
type Tier struct {
	// Name identifies the tier in messages and the ledger, e.g. "fast".
	Name string `json:"name"`
	// UseFor says, in a short phrase, which work belongs on this tier. It is
	// printed in deny reasons, so it is the guidance agents actually read.
	UseFor string `json:"use_for"`
	// Claude lists accepted Claude Code model values. A bare family word
	// (e.g. "haiku") also matches any full model id containing that word as a
	// hyphen-separated token, after exact ids in every tier have been tried.
	Claude []string `json:"claude"`
	// Codex lists accepted Codex model values, matched exactly
	// (case-insensitive).
	Codex []string `json:"codex"`
	// Effort is the recommended reasoning effort for this tier.
	Effort string `json:"effort,omitempty"`
	// Marker, when set, lets a brief escalate past its class ceiling to this
	// tier, e.g. "[deep]".
	Marker string `json:"marker,omitempty"`
}

// Classes holds the brief classes and their tier ceilings.
type Classes struct {
	// CriticPattern is a regular expression over the brief; a match makes the
	// spawn a critic unless the brief carries the worker marker. Empty
	// disables the heuristic, leaving only the critic marker.
	CriticPattern string `json:"critic_pattern"`
	// WorkerMarker forces the worker class, e.g. "[worker]".
	WorkerMarker string `json:"worker_marker"`
	// CriticMarker forces the critic class, e.g. "[critic]".
	CriticMarker string `json:"critic_marker"`
	// Worker is the default class: implementation, search, mechanical work.
	Worker ClassRule `json:"worker"`
	// Critic is judgment work: review, verification, synthesis.
	Critic ClassRule `json:"critic"`
}

// ClassRule is the tier ceiling of one class.
type ClassRule struct {
	// MaxTier is the strongest tier the class may use without a marker.
	MaxTier string `json:"max_tier"`
}

// SpawnRules are the requirements every subagent spawn must meet.
type SpawnRules struct {
	// RequireModel denies spawns that leave the model unset or "inherit".
	RequireModel bool `json:"require_model"`
	// RequireEffort lists hosts whose spawns must name a reasoning effort.
	RequireEffort []string `json:"require_effort,omitempty"`
	// Fork is the Claude Code fork mode: allow, marker or deny.
	Fork string `json:"fork"`
	// ForkMarker is the brief marker that admits a fork in marker mode.
	ForkMarker string `json:"fork_marker"`
	// DenyLoopLimit is how many spawns in a row may be denied in one session
	// before the next one is let through with a note. It keeps a session
	// that cannot comply (say, a host that stops exposing the model field)
	// from being blocked forever.
	DenyLoopLimit int `json:"deny_loop_limit"`
}

// Advice is a once-per-session note for briefs that match When and do not
// match Unless.
type Advice struct {
	// ID names the rule; it keys the once-per-session marker and the ledger.
	ID string `json:"id"`
	// When is a regular expression over the brief.
	When string `json:"when"`
	// Unless optionally suppresses the advice when it matches; present means
	// non-empty.
	Unless *string `json:"unless,omitempty"`
	// Classes optionally limits the rule to "worker" or "critic" briefs.
	Classes []string `json:"classes,omitempty"`
	// Message is the note shown to the agent.
	Message string `json:"message"`
}

type compiledAdvice struct {
	Advice
	when, unless *regexp.Regexp
}

// LedgerConfig configures the measurement ledger.
type LedgerConfig struct {
	// Enabled turns ledger writes on.
	Enabled bool `json:"enabled"`
	// MaxBytes rotates the ledger once it grows past this size.
	MaxBytes int64 `json:"max_bytes"`
	// KeepRotated is how many rotated ledger files to keep.
	KeepRotated int `json:"keep_rotated"`
	// RecordToolOutput records the size (never the content) of tool output.
	RecordToolOutput bool `json:"record_tool_output"`
}

// MainThreadConfig contains large tool output in the main session, where it
// would otherwise stay in context for every later turn. Subagent output is
// never touched: a subagent's context ends with its report. Claude Code only:
// Codex hooks cannot replace a tool result, and Codex tool events do not say
// whether the main thread or a subagent made the call.
type MainThreadConfig struct {
	// BashMaxChars is the largest main-thread Bash output passed through
	// whole; larger output keeps its head and tail and the full text is saved
	// to a file the agent can read in ranges. 0 disables containment.
	BashMaxChars int `json:"bash_max_chars"`
	// HeadChars and TailChars are how much of a contained output stays
	// inline. Errors cluster at the end of build and test output, so the tail
	// matters as much as the head.
	HeadChars int `json:"head_chars"`
	TailChars int `json:"tail_chars"`
}

// loadedPolicy is a policy plus where it came from, for messages and doctor.
type loadedPolicy struct {
	*Policy
	// Source is the file path, or "builtin".
	Source string
	// Fallback is set when a policy file existed but was rejected; the
	// built-in policy is in force and Fallback says why.
	Fallback error
}

// policyPath resolves the policy file: $AGENT_HOOKS_POLICY, then
// $XDG_CONFIG_HOME/agent-hooks/policy.json, then
// ~/.config/agent-hooks/policy.json.
func (a *app) policyPath() string {
	if p := a.env("AGENT_HOOKS_POLICY"); p != "" {
		return p
	}
	if x := a.env("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "agent-hooks", "policy.json")
	}
	return filepath.Join(a.home, ".config", "agent-hooks", "policy.json")
}

// loadPolicy returns the effective policy. A missing file means the built-in
// policy. A file that cannot be read or fails validation also yields the
// built-in policy, with Fallback set, so the hook keeps enforcing something
// sane while doctor and a once-per-session note surface the problem.
func (a *app) loadPolicy() loadedPolicy {
	path := a.policyPath()
	raw, err := readCapped(path, policyMaxBytes)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return loadedPolicy{Policy: builtinPolicy(), Source: "builtin"}
	case err != nil:
		return loadedPolicy{Policy: builtinPolicy(), Source: "builtin", Fallback: err}
	}
	p, err := parsePolicy(raw)
	if err != nil {
		return loadedPolicy{Policy: builtinPolicy(), Source: "builtin", Fallback: fmt.Errorf("%s: %w", path, err)}
	}
	return loadedPolicy{Policy: p, Source: path}
}

// builtinPolicy parses the embedded default. The default is covered by tests,
// so a failure here is a build defect, not a runtime condition.
func builtinPolicy() *Policy {
	p, err := parsePolicy(defaultPolicyJSON)
	if err != nil {
		panic("agent-hooks: embedded default policy is invalid: " + err.Error())
	}
	return p
}

// parsePolicy checks key spelling and required fields, decodes strictly
// (unknown fields, trailing data and oversize input are errors) and
// validates.
func parsePolicy(raw []byte) (*Policy, error) {
	if len(raw) > policyMaxBytes {
		return nil, fmt.Errorf("policy exceeds %d bytes", policyMaxBytes)
	}
	if errs := policyShape.check(raw, "policy"); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode policy: %w", err)
	}
	if dec.More() {
		return nil, errors.New("decode policy: trailing data after the policy object")
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// keySpec describes the keys of one JSON object in the policy: which are
// required, which optional, and the shape of nested objects. Go's decoder
// matches keys case-insensitively and leaves missing fields at their zero
// value; this check closes both gaps, matching CUE's closed, required fields.
type keySpec struct {
	required, optional []string
	objects            map[string]keySpec // key → nested object
	arrays             map[string]keySpec // key → array of objects
}

var policyShape = keySpec{
	required: []string{"version", "tiers", "classes", "spawn", "ledger", "main_thread"},
	optional: []string{"advice"},
	arrays: map[string]keySpec{
		"tiers":  {required: []string{"name", "use_for", "claude", "codex"}, optional: []string{"effort", "marker"}},
		"advice": {required: []string{"id", "when", "message"}, optional: []string{"unless", "classes"}},
	},
	objects: map[string]keySpec{
		"classes": {
			required: []string{"critic_pattern", "worker_marker", "critic_marker", "worker", "critic"},
			objects: map[string]keySpec{
				"worker": {required: []string{"max_tier"}},
				"critic": {required: []string{"max_tier"}},
			},
		},
		"spawn":       {required: []string{"require_model", "fork", "fork_marker", "deny_loop_limit"}, optional: []string{"require_effort"}},
		"ledger":      {required: []string{"enabled", "max_bytes", "keep_rotated", "record_tool_output"}},
		"main_thread": {required: []string{"bash_max_chars", "head_chars", "tail_chars"}},
	},
}

func (s keySpec) check(raw json.RawMessage, path string) []error {
	m, err := rawObject(raw)
	if err != nil {
		return []error{fmt.Errorf("%s: must be an object", path)}
	}
	var errs []error
	for _, k := range s.required {
		if _, ok := m[k]; !ok {
			errs = append(errs, fmt.Errorf("%s.%s is required", path, k))
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !contains(s.required, k) && !contains(s.optional, k) {
			errs = append(errs, fmt.Errorf("%s.%s is not a policy field (keys are case-sensitive)", path, k))
			continue
		}
		if sub, ok := s.objects[k]; ok {
			errs = append(errs, sub.check(m[k], path+"."+k)...)
		}
		if sub, ok := s.arrays[k]; ok {
			var items []json.RawMessage
			if json.Unmarshal(m[k], &items) != nil {
				errs = append(errs, fmt.Errorf("%s.%s must be an array", path, k))
				continue
			}
			for i, item := range items {
				errs = append(errs, sub.check(item, fmt.Sprintf("%s.%s[%d]", path, k, i))...)
			}
		}
	}
	return errs
}

// modelRE mirrors schema.cue's #Model: 1..128 non-space characters.
var modelRE = regexp.MustCompile(`^\S{1,128}$`)

// validate enforces the schema.cue contract and compiles the patterns.
func (p *Policy) validate() error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if p.Version != 1 {
		fail("version must be 1, got %d", p.Version)
	}
	if len(p.Tiers) == 0 || len(p.Tiers) > maxTiers {
		fail("tiers: need 1..%d tiers, got %d", maxTiers, len(p.Tiers))
	}
	tierNames := map[string]bool{}
	seenModel := map[string]map[string]string{hostClaude: {}, hostCodex: {}}
	for i, t := range p.Tiers {
		if !isIdent(t.Name) {
			fail("tiers[%d].name %q must be a lowercase identifier", i, t.Name)
		}
		if tierNames[t.Name] {
			fail("tiers[%d].name %q is duplicated", i, t.Name)
		}
		tierNames[t.Name] = true
		checkText(fail, fmt.Sprintf("tiers[%d].use_for", i), t.UseFor, true)
		checkText(fail, fmt.Sprintf("tiers[%d].effort", i), t.Effort, false)
		checkText(fail, fmt.Sprintf("tiers[%d].marker", i), t.Marker, false)
		for _, host := range []string{hostClaude, hostCodex} {
			models := p.hostModels(t, host)
			if len(models) > maxModelsPerTier {
				fail("tiers[%d].%s: at most %d models", i, host, maxModelsPerTier)
			}
			for _, m := range models {
				key := strings.ToLower(m)
				switch {
				case !modelRE.MatchString(m):
					fail("tiers[%d].%s: model %q must be 1..128 characters without whitespace", i, host, m)
				case key == "inherit":
					fail("tiers[%d].%s: %q is not a model", i, host, m)
				case seenModel[host][key] != "":
					fail("tiers[%d].%s: model %q already belongs to tier %q", i, host, m, seenModel[host][key])
				default:
					seenModel[host][key] = t.Name
				}
			}
		}
	}

	var err error
	p.criticRE = nil
	checkText(fail, "classes.critic_pattern", p.Classes.CriticPattern, false)
	if p.Classes.CriticPattern != "" {
		if p.criticRE, err = compilePattern(p.Classes.CriticPattern); err != nil {
			fail("classes.critic_pattern: %v", err)
		}
	}
	checkText(fail, "classes.worker_marker", p.Classes.WorkerMarker, false)
	checkText(fail, "classes.critic_marker", p.Classes.CriticMarker, false)
	for _, class := range []string{classWorker, classCritic} {
		rule := p.Classes.Worker
		if class == classCritic {
			rule = p.Classes.Critic
		}
		if !tierNames[rule.MaxTier] {
			fail("classes.%s.max_tier %q is not a tier", class, rule.MaxTier)
		}
	}

	for _, h := range p.Spawn.RequireEffort {
		if h != hostClaude && h != hostCodex {
			fail("spawn.require_effort: unknown host %q", h)
		}
	}
	if !unique(p.Spawn.RequireEffort) {
		fail("spawn.require_effort: hosts must be unique")
	}
	switch p.Spawn.Fork {
	case forkAllow, forkDeny:
	case forkMarker:
		if strings.TrimSpace(p.Spawn.ForkMarker) == "" {
			fail("spawn.fork_marker is required when spawn.fork is %q", forkMarker)
		}
	default:
		fail("spawn.fork must be %q, %q or %q, got %q", forkAllow, forkMarker, forkDeny, p.Spawn.Fork)
	}
	checkText(fail, "spawn.fork_marker", p.Spawn.ForkMarker, false)
	if p.Spawn.DenyLoopLimit < 1 || p.Spawn.DenyLoopLimit > maxDenyLoopLimit {
		fail("spawn.deny_loop_limit must be within 1..%d", maxDenyLoopLimit)
	}

	if len(p.Advice) > maxAdvice {
		fail("advice: at most %d rules", maxAdvice)
	}
	adviceIDs := map[string]bool{}
	p.adviceRE = nil
	for i, ad := range p.Advice {
		c := compiledAdvice{Advice: ad}
		if !isIdent(ad.ID) || adviceIDs[ad.ID] {
			fail("advice[%d].id %q must be a unique lowercase identifier", i, ad.ID)
		}
		adviceIDs[ad.ID] = true
		checkText(fail, fmt.Sprintf("advice[%d].when", i), ad.When, true)
		if c.when, err = compilePattern(ad.When); err != nil {
			fail("advice[%d].when: %v", i, err)
		}
		if ad.Unless != nil {
			checkText(fail, fmt.Sprintf("advice[%d].unless", i), *ad.Unless, true)
			if c.unless, err = compilePattern(*ad.Unless); err != nil {
				fail("advice[%d].unless: %v", i, err)
			}
		}
		for _, cl := range ad.Classes {
			if cl != classWorker && cl != classCritic {
				fail("advice[%d].classes: unknown class %q", i, cl)
			}
		}
		if !unique(ad.Classes) {
			fail("advice[%d].classes must be unique", i)
		}
		checkText(fail, fmt.Sprintf("advice[%d].message", i), ad.Message, true)
		p.adviceRE = append(p.adviceRE, c)
	}

	if p.Ledger.MaxBytes < minLedgerBytes || p.Ledger.MaxBytes > maxLedgerBytes {
		fail("ledger.max_bytes must be within %d..%d", minLedgerBytes, maxLedgerBytes)
	}
	if p.Ledger.KeepRotated < 0 || p.Ledger.KeepRotated > 20 {
		fail("ledger.keep_rotated must be within 0..20")
	}
	mt := p.MainThread
	switch {
	case mt.HeadChars < 0 || mt.TailChars < 0:
		fail("main_thread.head_chars and tail_chars must not be negative")
	case mt.BashMaxChars == 0:
	case mt.BashMaxChars < minContainChars || mt.BashMaxChars > maxContainChars:
		fail("main_thread.bash_max_chars must be 0 or within %d..%d", minContainChars, maxContainChars)
	case mt.HeadChars+mt.TailChars == 0:
		fail("main_thread.head_chars and tail_chars must not both be 0 when containment is on")
	case mt.HeadChars+mt.TailChars >= mt.BashMaxChars:
		fail("main_thread.head_chars + tail_chars must be below bash_max_chars")
	}
	return errors.Join(errs...)
}

// tierIndex returns the position of the named tier, or -1.
func (p *Policy) tierIndex(name string) int {
	for i, t := range p.Tiers {
		if t.Name == name {
			return i
		}
	}
	return -1
}

// tierFor resolves a model value to its tier index for a host, or -1. Exact
// values in every tier win over family words, so a full id listed in a
// stronger tier is never shadowed by a family word in a cheaper one.
func (p *Policy) tierFor(host, model string) int {
	want := strings.ToLower(strings.TrimSpace(model))
	for i, t := range p.Tiers {
		for _, m := range p.hostModels(t, host) {
			if strings.ToLower(m) == want {
				return i
			}
		}
	}
	if host != hostClaude {
		return -1 // Codex names reuse words across tiers: exact only
	}
	tokens := strings.Split(want, "-")
	for i, t := range p.Tiers {
		for _, m := range t.Claude {
			if m = strings.ToLower(m); !strings.Contains(m, "-") && contains(tokens, m) {
				return i
			}
		}
	}
	return -1
}

// hostMapped reports whether the policy lists any model for the host. An
// unmapped host gets the model and effort requirements but no tier checks.
func (p *Policy) hostMapped(host string) bool {
	for _, t := range p.Tiers {
		if len(p.hostModels(t, host)) > 0 {
			return true
		}
	}
	return false
}

func (p *Policy) hostModels(t Tier, host string) []string {
	if host == hostCodex {
		return t.Codex
	}
	return t.Claude
}

func (p *Policy) requiresEffort(host string) bool { return contains(p.Spawn.RequireEffort, host) }

var identRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func isIdent(s string) bool { return identRE.MatchString(s) }

func checkText(fail func(string, ...any), field, value string, required bool) {
	switch {
	case required && strings.TrimSpace(value) == "":
		fail("%s is required", field)
	case utf8.RuneCountInString(value) > maxTextRunes:
		fail("%s exceeds %d characters", field, maxTextRunes)
	}
}

// compilePattern compiles a policy regular expression. Go's RE2 engine runs
// in linear time, so a hostile pattern cannot stall a hook; the length cap
// bounds compile cost.
func compilePattern(expr string) (*regexp.Regexp, error) {
	if expr == "" || len(expr) > maxPatternLen {
		return nil, fmt.Errorf("pattern must be 1..%d bytes", maxPatternLen)
	}
	return regexp.Compile(expr)
}

// readCapped reads at most limit bytes and reports an error past it.
func readCapped(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return raw, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func unique(list []string) bool {
	seen := map[string]bool{}
	for _, v := range list {
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
