package main

// doctor_decide.go — DP-4 (docs/plans/decision-port.md, ADR-042): the
// `decision_provider` row.
//
// [decide] provider = "none" (the default) is reported `skipped`, naming
// why — the same "an honestly-off feature is never a guessed pass"
// discipline every other row in this file follows. provider = "llm" pings
// its configured profile lane exactly the way `fathom providers` does
// (buildLLMClientForProfile + a llmclient.Pinger type-assertion, mirroring
// providers.go's own probeProfiles) and names the last recorded
// decisions.jsonl row's latency_ms when that file exists. provider =
// "typesafe" (ADR-042 amendment, 2026-09-22) resolves the key via
// config.Decide.ResolveAPIKey — an unresolvable/empty key is a FAIL naming
// which indirection was checked, never the key itself — then pings the
// kit's TypeSafe Decider directly (no profile lane involved) under a 5s
// timeout; a failed ping names the endpoint, never the key. Any other
// provider value is a FAIL naming it verbatim — reaching that branch means
// config.Load's own CUE-schema refusal (#Decide's provider enum,
// schema/config/schema.cue) was bypassed: a config.toml hand-edited after
// validation, or a daemon running a binary built against a stale schema.
// That is worth surfacing loudly, not silently skipping.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// checkDecisionProvider loads root's merged [decide] config (user + project
// layers, same precedence every other config-reading row uses) and reports
// its verdict. build is the profileClientBuilder seam (providers.go) so a
// test can drive the "llm" branch without a real credential or network
// call — production passes providerClientBuilder, the same var `fathom
// providers` uses.
func checkDecisionProvider(root string, build profileClientBuilder) doctorCheck {
	const name = "decision_provider"

	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("config did not load: %v", err)}
	}
	return decisionProviderVerdict(loaded, build, resolveCallLogPath())
}

// decisionProviderVerdict is checkDecisionProvider's pure decision logic,
// separated out so a test can drive the "unknown provider" branch directly
// with a hand-built config.Loaded — reaching it through a real
// config.toml is impossible by construction, since #Decide's provider enum
// (schema/config/schema.cue) already refuses that value at Load.
func decisionProviderVerdict(loaded config.Loaded, build profileClientBuilder, callLogPath string) doctorCheck {
	const name = "decision_provider"
	dc := loaded.Config.Decide

	switch dc.Provider {
	case "", "none":
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "off ([decide] provider = none)"}

	case "llm":
		p, _, err := config.ResolveProfile(loaded.Config, loaded.Provenance, dc.Profile)
		if err != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("[decide] profile: %v", err),
				Remedy: "fix [decide].profile in .fathom/config.toml, or leave it empty to use the default profile",
			}
		}
		cl, err := build(p)
		if err != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("[decide] provider=llm profile=%s: %v", displayProfileName(dc.Profile), err),
				Remedy: "fathom providers  (diagnose the same lane's auth health)",
			}
		}
		if pinger, ok := cl.(llmclient.Pinger); ok {
			pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := pinger.Ping(pingCtx); err != nil {
				return doctorCheck{
					Name: name, Status: doctorFail,
					Reason: fmt.Sprintf("[decide] provider=llm profile=%s ping failed: %v", displayProfileName(dc.Profile), err),
					Remedy: "fathom providers",
				}
			}
		}
		reason := fmt.Sprintf("[decide] provider=llm profile=%s lane ok", displayProfileName(dc.Profile))
		if latencyMS, ok := lastDecisionLatencyMS(callLogPath); ok {
			reason = fmt.Sprintf("%s (last decision latency_ms=%d)", reason, latencyMS)
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}

	case "typesafe":
		key, keyErr := dc.ResolveAPIKey(context.Background())
		if keyErr != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("[decide] provider=typesafe: resolving API key: %v", keyErr),
				Remedy: "export TYPESAFE_API_KEY=… or set [decide].api_key_env / api_key_cmd",
			}
		}
		if key == "" {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: "[decide] provider=typesafe: no API key (checked api_key_cmd, api_key_env, TYPESAFE_API_KEY)",
				Remedy: "export TYPESAFE_API_KEY=… or set [decide].api_key_env / api_key_cmd",
			}
		}
		model := dc.Model
		if model == "" {
			model = decision.DefaultTypeSafeModel
		}
		endpoint := dc.Endpoint
		if endpoint == "" {
			endpoint = decision.DefaultTypeSafeBaseURL
		}
		ts, tsErr := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: key, BaseURL: dc.Endpoint, Model: dc.Model})
		if tsErr != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("[decide] provider=typesafe: %v", tsErr),
				Remedy: "fix [decide] in .fathom/config.toml",
			}
		}
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ts.Ping(pingCtx); err != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				// Never the key — err is a *decision.TypeSafeError or a
				// transport error, neither of which ever carries it (kit
				// contract, typesafe.go).
				Reason: fmt.Sprintf("[decide] provider=typesafe endpoint=%s ping failed: %v", endpoint, err),
				Remedy: "check [decide].endpoint and the API key's validity",
			}
		}
		reason := fmt.Sprintf("[decide] provider=typesafe model=%s endpoint=%s ping ok", model, endpoint)
		if latencyMS, ok := lastDecisionLatencyMS(callLogPath); ok {
			reason = fmt.Sprintf("%s (last decision latency_ms=%d)", reason, latencyMS)
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}

	default:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("unknown [decide] provider %q (want \"none\", \"llm\", or \"typesafe\")", dc.Provider),
			Remedy: "fix [decide].provider in .fathom/config.toml",
		}
	}
}

// lastDecisionLatencyMS reads the last row of decisions.jsonl (beside
// callLogPath — internal/decide.DefaultRecorderPath) and returns its
// latency_ms field. ok is false when no call log is configured, no
// decisions.jsonl sits next to it, or the last line does not parse — all
// informational, never a reason to fail this row.
func lastDecisionLatencyMS(callLogPath string) (int64, bool) {
	path := decide.DefaultRecorderPath(callLogPath)
	if path == "" {
		return 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		return 0, false
	}
	var row struct {
		LatencyMS int64 `json:"latency_ms"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		return 0, false
	}
	return row.LatencyMS, true
}
