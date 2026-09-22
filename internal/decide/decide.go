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

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
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

// Open builds a Ports from cfg: provider "none" (or unset, the default)
// yields the kit's zero-configuration decision.None — no call is ever made.
// provider "llm" resolves an llmclient.LLMClient via lane(cfg.Profile) (the
// caller's own profile-construction seam — internal/decide never
// constructs a client itself) and wraps it in decision.NewLLM(client,
// model). Any other provider value is refused by name — the same discipline
// internal/config's schema validation already applies at load, kept here
// too so a Ports is never silently built for a provider validation should
// have caught.
func Open(cfg config.Decide, lane func(profile string) (llmclient.LLMClient, error), model string, rec *Recorder) (*Ports, error) {
	switch cfg.Provider {
	case "", "none":
		return &Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}, nil
	case "llm":
		client, err := lane(cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("decide: opening llm lane (profile %q): %w", cfg.Profile, err)
		}
		return &Ports{Decider: decision.NewLLM(client, model), Cfg: cfg, Recorder: rec}, nil
	default:
		return nil, fmt.Errorf("decide: unknown [decide] provider %q (want \"none\" or \"llm\")", cfg.Provider)
	}
}
