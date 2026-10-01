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
	"context"
	"fmt"
	"slices"
	"strings"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// Ports bundles the wired decision.Decider with the config that produced it
// and the Recorder every site's Response must be logged through.
type Ports struct {
	// Decider is what every site asks: decision.None for provider none,
	// else the provider adapter behind the kit's fallback to decision.None
	// (JF-6, degrade.go), whose Name is the provider's.
	Decider  decision.Decider
	Cfg      config.Decide
	Recorder *Recorder
	// Adapter is the bare provider adapter Open built (*llm.LLM,
	// *typesafe.TypeSafe), nil for provider none: for inspection and for
	// Gated, never for a site to ask directly (it would skip the degrade).
	Adapter decision.Decider

	degrade *degrader
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
func SiteAllowed(cfg config.Decide, site string) bool {
	if IsOptInSite(site) && !slices.Contains(cfg.RepoSites, site) {
		return false
	}
	return slices.Contains(cfg.Sites, site)
}

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
// provider "typesafe" resolves cfg.ResolveAPIKey (cmd, then env, then
// TYPESAFE_API_KEY — internal/config/apikey.go) and builds the kit's
// TypeSafe Decider directly over cfg.Endpoint/cfg.Model — no lane, no
// llmclient, the same construction-swap typesafe.NewTypeSafe (decision/
// typesafe) documents. An
// empty key is an error (never a silent None here — the caller,
// cmd/fathom's openDecisionPorts, is what turns this into a warn-and-none,
// per I14) naming which of the three indirections to set. Any other
// provider value is refused by name — the same discipline internal/config's
// schema validation already applies at load, kept here too so a Ports is
// never silently built for a provider validation should have caught.
//
// The API key resolution below runs against context.Background(), not a
// caller-supplied ctx: Open runs once at daemon/CLI startup (the same
// lifecycle stage cmd/fathom/llmprofile.go's buildAnthropicClientProfile/
// buildOpenAIClientProfile already resolve a profile's own credential
// against — see Profile.ResolveAPIKey's call sites there), never per
// request, so there is no request-scoped deadline to thread through three
// call sites and their tests for a bounded, one-time local exec/env read
// (apiKeyCmdTimeout already caps the api_key_cmd case on its own).
func Open(cfg config.Decide, lane func(profile string) (llmclient.LLMClient, error), model string, rec *Recorder, opts ...Option) (*Ports, error) {
	var adapter decision.Decider
	switch cfg.Provider {
	case "", "none":
		return &Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}, nil
	case "llm":
		client, err := lane(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("decide: opening llm lane (profile %q): %w", cfg.Profile, err)
		}
		adapter = llm.NewLLM(client, model)
	case "typesafe":
		key, err := cfg.ResolveAPIKey(context.Background())
		if err != nil {
			return nil, fmt.Errorf("decide: resolving [decide] provider typesafe API key: %w", err)
		}
		if key == "" {
			return nil, fmt.Errorf("decide: [decide] provider typesafe: no API key (set TYPESAFE_API_KEY, or [decide].api_key_env / api_key_cmd)")
		}
		ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: key, BaseURL: cfg.Endpoint, Model: cfg.Model})
		if err != nil {
			return nil, fmt.Errorf("decide: constructing typesafe decider: %w", err)
		}
		adapter = ts
	default:
		return nil, fmt.Errorf("decide: unknown [decide] provider %q (want \"none\", \"llm\", or \"typesafe\")", cfg.Provider)
	}
	// JF-6: the adapter falls back to decision.None, composed from a policy
	// derived from [decide] (degrade.go): a failed call writes one line and
	// leaves the site on its deterministic path.
	obs := newDegrader(opts)
	d, err := compose(cfg, adapter, obs)
	if err != nil {
		return nil, fmt.Errorf("decide: composing the degrade fallback: %w", err)
	}
	return &Ports{Decider: d, Cfg: cfg, Recorder: rec, Adapter: adapter, degrade: obs}, nil
}
