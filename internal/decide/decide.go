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

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// Ports bundles the wired decision.Decider with the config that produced it
// and the Recorder every site's Response must be logged through.
type Ports struct {
	Decider  decision.Decider
	Cfg      config.Decide
	Recorder *Recorder
}

// Site name constants for [decide] sites (ADR-044 decision 8, design doc
// §7, I14): the only sites a decision.Decider may ever be asked from. These
// literals are exactly what config.DefaultDecide.Sites lists — a
// config.Decide loaded through config.Load and never touching [decide]
// sites carries all of them, so SiteAllowed(cfg, <any of these>) is true by
// default and every existing call site's behaviour is unchanged.
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

// AllSites lists every known site name, in the same order config.toml's
// documentation and DefaultDecide.Sites present them — for iteration (e.g.
// fathom doctor's decision_provider row naming every site a narrowed
// allowlist excludes).
var AllSites = []string{SiteRoute, SiteCompilePrefilter, SiteObligationsRerank, SiteCertifySuggest, SiteAssess, SitePreflight}

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
func SiteAllowed(cfg config.Decide, site string) bool {
	for _, s := range cfg.Sites {
		if s == site {
			return true
		}
	}
	return false
}

// Open builds a Ports from cfg: provider "none" (or unset, the default)
// yields the kit's zero-configuration decision.None — no call is ever made.
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
func Open(cfg config.Decide, lane func(profile string) (llmclient.LLMClient, error), model string, rec *Recorder) (*Ports, error) {
	switch cfg.Provider {
	case "", "none":
		return &Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}, nil
	case "llm":
		client, err := lane(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("decide: opening llm lane (profile %q): %w", cfg.Profile, err)
		}
		return &Ports{Decider: llm.NewLLM(client, model), Cfg: cfg, Recorder: rec}, nil
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
		return &Ports{Decider: ts, Cfg: cfg, Recorder: rec}, nil
	default:
		return nil, fmt.Errorf("decide: unknown [decide] provider %q (want \"none\", \"llm\", or \"typesafe\")", cfg.Provider)
	}
}
