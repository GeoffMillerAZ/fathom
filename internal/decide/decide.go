// Package decide wires ADR-042's decision port
// (docs/vision/32-decision-port.md) into fathom: it builds the kit's
// decision.Decider from a [decide] config block and records every answered
// question to an append-only JSONL log.
//
// Never a global: Open returns a *Ports the caller injects into every site
// that asks a fast decision (the compile pre-filter, obligations re-rank,
// prompt routing) — mirroring how internal/config threads a Profile through
// call sites instead of reading process-wide state.
package decide

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// Ports bundles the wired decision.Decider with the config that produced it
// and the Recorder every site's Response must be logged through.
type Ports struct {
	// Decider is what every site asks: decision.None for provider none,
	// else the provider adapter behind the kit's fallback to decision.None
	// (JF-6, degrade.go), whose Name is the provider's; for a port only a
	// live kill closed at Open, the reopening port (reopen.go, O1132).
	Decider  decision.Decider
	Cfg      config.Decide
	Recorder *Recorder
	// Adapter is the bare provider adapter Open built (*llm.LLM,
	// *typesafe.TypeSafe, *openai.OpenAI), nil for provider none: for
	// inspection and for Gated, never for a site to ask directly (it would
	// skip the degrade).
	Adapter decision.Decider
	// Gate is the consent and kill verdict Open reached (KS-1 to KS-6): a
	// closed Gate names the source that left the port decision.None, and
	// an open one the layer whose consent opened it.
	Gate Gate

	degrade *degrader
	// live re-checks the kill file and the user's enabled before each ask
	// (KS-2); nil for a port that never opened.
	live *liveGate
	// reopen finishes a port a live kill held closed at Open on the first
	// ask after the kill lifts (O1132); nil otherwise.
	reopen *reopener
}

// Site name constants for [decide] sites (ADR-044 decision 8, design doc
// §7, I14): the only sites a decision.Decider may ever be asked from. A
// config.Decide loaded through config.Load and never touching [decide]
// sites carries config.DefaultDecide.Sites: every one of these except
// SiteRoute (DefaultOffSites, JF-7), so SiteAllowed is true by default for
// every other existing site and its behaviour is unchanged (I42, JF-5).
const (
	// SiteRoute is POST /route (DP-9, cmd/fathom/route.go): one
	// decision.KindChoice question asking which verb a prompt fits.
	SiteRoute = "route"
	// SiteCompilePrefilter is the compile pre-filter (DP-9,
	// internal/semantic.PrefilterSections): one decision.KindPredicate
	// question per ordered section asking whether it states a binding rule.
	SiteCompilePrefilter = "compile_prefilter"
	// SiteObligationsRerank is the obligations within-tier re-rank (DP-7/
	// DP-8, internal/semantic.rerankWithinTiers): one decision.KindPredicate
	// question per eligible rule asking whether it governs the edit. The
	// decisions.jsonl row this site produces is still logged under Site
	// "obligations" (internal/semantic/decisionrank.go) — that wire value
	// predates Sites and stays as-is; SiteObligationsRerank is only the
	// [decide] sites config vocabulary's name for the same call site.
	SiteObligationsRerank = "obligations_rerank"
	// SiteCertifySuggest is `fathom certify suggest` (TC-24,
	// cmd/fathom/certify_suggest.go): one batched request per content-sized
	// chunk of ungoverned Test* symbols.
	SiteCertifySuggest = "certify_suggest"
	// SiteAssess is `fathom assess` (DO-3/IR-3, ADR-046,
	// docs/design/assess-lanes.md): one batched request per explicitly
	// invoked run, typed predicate/choice questions over a unit's on-disk work
	// products. Also gated by the default-off `assess` capability, so
	// listing it here is never, by itself, consent to spend.
	SiteAssess = "assess"
	// SitePreflight is task preflight's Score (WI-18, ADR-051 decision 6,
	// docs/design/work-intelligence.md U6, the Jev site): one KindScore
	// question over a plan row's measured features (never its text, symbol
	// or paths), asked only when a caller requests it (`fathom plans context
	// --preflight --score`). Advisory: it starts, assigns and routes nothing.
	SitePreflight = "preflight"
)

// AllSites lists every site that has a call site, in the same order
// config.toml's documentation presents them — for iteration (e.g. fathom
// doctor's decision_provider row naming every site a narrowed allowlist
// excludes). It never lists an opt-in name: those have no call site yet.
var AllSites = []string{SiteRoute, SiteCompilePrefilter, SiteObligationsRerank, SiteCertifySuggest, SiteAssess, SitePreflight}

// DefaultOffSites are the sites in AllSites that config.DefaultDecide.Sites
// leaves out although they are not opt-in: route only (JF-7,
// judgment-foundations §6). route failed calibration, so it is off by
// default, but it is a narrowing, not a new judgment: its code stays,
// SR-44's deterministic precheck stays, and naming "route" in any layer's
// [decide] sites turns it back on. It is not classed opt-in because route
// is served by the daemon, whose project layer is its own working
// directory's config, not the asking repository's (decision recorded in
// docs/design/judgment-foundations.md §4 and §6).
var DefaultOffSites = []string{SiteRoute}

// Opt-in site names (JF-4, judgment-foundations §4, R2): judgment sites
// that do not exist yet. Names only: no call site asks any of them today.
// They never enter config.DefaultDecide.Sites; SiteAllowed refuses each one
// unless the repository's own [decide] sites names it.
const (
	SiteTriageFailure = "triage_failure"
	SiteAsk           = "ask"
	SiteExplore       = "explore"
	SiteCompactAdvice = "compact_advice"
	// SiteJudgeFamily stands for every "judge_<name>" site (judged
	// telemetry, e.g. judge_failure); a repository names each one, never
	// the family.
	SiteJudgeFamily = "judge_*"
)

// OptInSites lists the opt-in names; an entry ending in "*" is a family
// matched by prefix (IsOptInSite).
var OptInSites = []string{SiteTriageFailure, SiteAsk, SiteExplore, SiteCompactAdvice, SiteJudgeFamily}

// IsOptInSite reports whether site is an opt-in name: listed in OptInSites,
// or a member "judge_<name>" of a family entry (the bare prefix is not).
func IsOptInSite(site string) bool {
	for _, s := range OptInSites {
		if prefix, family := strings.CutSuffix(s, "*"); family {
			if len(site) > len(prefix) && strings.HasPrefix(site, prefix) {
				return true
			}
			continue
		}
		if s == site {
			return true
		}
	}
	return false
}

// SiteAllowed reports whether cfg's [decide] sites allowlist lets site send
// a decision.Question off-host (ADR-044 decision 8, I14: "the only off-host
// egress is LLM compilation and … the decision port"). A site absent from
// cfg.Sites — including every site, when cfg.Sites is nil or empty — makes
// NO call at all, regardless of cfg.Provider: this is a second, independent
// gate a real provider must also clear, never a substitute for Provider
// "none" already making no call. Fail-closed by construction: a
// zero-value config.Decide{} (Sites never set) allows nothing, the same
// direction every other egress control in this repository defaults to,
// never fail-open. Every call site (route.go's routePortActive,
// certify_suggest.go, pkg/client.WithDecision's decisionOpts/CompileLayer
// wiring) consults this one function rather than re-deriving the check.
//
// An opt-in site (IsOptInSite, JF-4) is held to more: it must also be named
// in cfg.RepoSites, the list the repository's own .fathom/config.toml
// wrote, so neither the default nor a user-global config.toml can turn it
// on, whatever cfg.Provider is.
//
// KS-5 (ADR-073 decision 5, I60 as amended): a site in the user-only
// cfg.DenySites is refused whatever any list says, and a session site
// (IsSessionSite) is allowed only when the user-only cfg.SessionSites names
// it, never from any layer's sites (config.Load drops it there). It is
// portGate's site half (siteGate), so the closing source has one name.
func SiteAllowed(cfg config.Decide, site string) bool {
	return !siteGate(cfg, site).Closed
}

// SiteReadoutMode is readouts' readout_mode decision (ADR-071 decision 8):
// a session site, named for the gate before its call site lands.
const SiteReadoutMode = "readout_mode"

// SessionSites are the sites about the user's own sessions (ADR-073
// decision 5): opted in only by the user-only [decide] session_sites.
var SessionSites = config.SessionSites

// IsSessionSite reports whether site is a session site.
func IsSessionSite(site string) bool { return slices.Contains(SessionSites, site) }

// Open builds a Ports from cfg: provider "none" (or unset, the default)
// yields the kit's zero-configuration decision.None — no call is ever made.
// A real provider's adapter is wrapped in the kit's fallback to
// decision.None (JF-6, degrade.go): when it fails a call, one line goes to
// the WithDisclose writer (default os.Stderr) and the site gets a
// *DegradedError that is decision.ErrNoProvider. Ports.Adapter keeps the
// bare adapter.
// provider "llm" resolves an llmclient.LLMClient via lane(cfg.Profile) (the
// caller's own profile-construction seam — internal/decide never
// constructs a client itself) and wraps it in llm.NewLLM(client, model),
// the adapter in the kit's own decision/llm package (agentic-kit DP-14).
// provider "typesafe" (Jev over TypeSafe's own API, or [decide] transport
// "openrouter") and provider "openai" (OpenAI's Decisions API) build the
// kit's own adapter directly — no lane, no llmclient (remote.go): the
// transport must fit the provider and the base URL must keep the key on
// the provider's own host (CheckRemote), and the key's source
// (config.KeySource: api_key_cmd, api_key_file, api_key_env, then the
// transport's well-known variable) must resolve, checked without reading a
// key out of a file. A source that does not resolve is an error (never a
// silent None here — the caller, cmd/fathom's openDecisionPorts, is what
// turns this into a warn-and-none, per I14) naming the source and the fix.
// Any other provider value is refused by name — the same discipline
// internal/config's schema validation already applies at load, kept here
// too so a Ports is never silently built for a provider validation should
// have caught.
//
// The key source check runs against context.Background(), not a
// caller-supplied ctx: Open runs once at daemon/CLI startup (the same
// lifecycle stage cmd/fathom/llmprofile.go's buildAnthropicClientProfile/
// buildOpenAIClientProfile already resolve a profile's own credential
// against — see Profile.ResolveAPIKey's call sites there), never per
// request, so there is no request-scoped deadline to thread through three
// call sites and their tests for a bounded, one-time local exec/env read
// (apiKeyCmdTimeout already caps the api_key_cmd case on its own).
//
// KS-1 to KS-6 (ADR-073 decision 5, I66): before anything else Open reads
// FATHOM_DECIDE (once, so it is per process), stats the kill file and reads
// cfg's enabled and consent (killSource); when any closes the port it
// returns decision.None with that Gate, having read no key variable or key
// file, run no api_key_cmd, built no lane and dialed nothing. A port that
// opens re-checks the kill file and the user's enabled before each ask
// (liveGate), so a running daemon closes without a restart. A port only a
// live kill closed at Open (the kill file, the user's enabled) re-checks it
// the same way and finishes opening on the first ask after it lifts
// (reopening, O1132), so a daemon started while killed reopens without a
// restart; FATHOM_DECIDE=off and a refused consent stay decision.None.
func Open(cfg config.Decide, lane func(profile string) (llmclient.LLMClient, error), model string, rec *Recorder, opts ...Option) (*Ports, error) {
	o := resolveOptions(opts)
	env := os.Getenv(KillEnv)
	gate := killSource(cfg, env, o)
	switch cfg.Provider {
	case "", "none":
		return &Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec, Gate: gate}, nil
	}
	if gate.Closed {
		// O1132: a live kill (the kill file, the user's enabled) is
		// re-checked before each ask, so a port it alone closed reopens on
		// the first ask after it lifts; the environment and a refused
		// consent are settled for the process.
		if isLiveKill(gate.Source) {
			if settled := portGate(gateInput{env: env, cfg: cfg}); !settled.Closed {
				live := newLiveGate(cfg, env, o)
				r := &reopener{cfg: cfg, lane: lane, model: model, rec: rec, o: o, live: live, consentLayer: settled.ConsentLayer}
				return &Ports{Decider: &reopening{r: r}, Cfg: cfg, Recorder: rec, Gate: gate, live: live, reopen: r}, nil
			}
		}
		return &Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec, Gate: gate}, nil
	}
	adapter, err := buildAdapter(cfg, lane, model, o)
	if err != nil {
		return nil, err
	}
	// JF-6: the adapter falls back to decision.None, composed from a policy
	// derived from [decide] (degrade.go): a failed call writes one line and
	// leaves the site on its deterministic path.
	obs := &degrader{w: o.disclose}
	d, err := compose(cfg, adapter, obs)
	if err != nil {
		return nil, fmt.Errorf("decide: composing the degrade fallback: %w", err)
	}
	rec.setConsentLayer(gate.ConsentLayer)
	live := newLiveGate(cfg, env, o)
	return &Ports{Decider: killable{inner: d, gate: live}, Cfg: cfg, Recorder: rec, Adapter: adapter, Gate: gate, degrade: obs, live: live}, nil
}

// buildAdapter builds cfg's provider adapter: the llm lane (its model from
// WithLaneModel when set), or a remote provider's adapter after its key
// source resolves (openRemote).
func buildAdapter(cfg config.Decide, lane func(profile string) (llmclient.LLMClient, error), model string, o openOptions) (decision.Decider, error) {
	switch cfg.Provider {
	case "llm":
		client, err := lane(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("decide: opening llm lane (profile %q): %w", cfg.Profile, err)
		}
		if o.laneModel != nil {
			model = o.laneModel()
		}
		return llm.NewLLM(client, model), nil
	case "typesafe", "openai":
		return openRemote(cfg, o)
	}
	return nil, fmt.Errorf("decide: unknown [decide] provider %q (want \"none\", \"llm\", \"typesafe\", or \"openai\")", cfg.Provider)
}

// killSource is Open's gate (KS-1, KS-6): FATHOM_DECIDE first, then the
// kill file (stat-ed, never read), cfg's enabled, then consent (KS-4). It
// reads no key, execs nothing and dials nothing.
func killSource(cfg config.Decide, env string, o openOptions) Gate {
	return portGate(gateInput{
		env:             env,
		killFile:        o.killFile,
		killFilePresent: killFilePresent(o.stat, o.killFile),
		disabled:        cfg.Disabled,
		cfg:             cfg,
	})
}
