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
// "typesafe" (ADR-042 amendment, 2026-09-22) and provider "openai" (ADR-042
// amendment, 2026-10-09) go through remoteProviderVerdict: the key's
// destination and source are checked (a source that does not resolve is a
// FAIL naming it — a file's path and mode, a variable's name — never the
// key), then the kit adapter is pinged directly (no profile lane involved)
// under a 5s timeout; a failed ping names the endpoint, never the key. A
// configured provider's passing row says its thresholds still need a
// calibration reading. Any other
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

// blockedSitesSuffix names, in decide.AllSites order, any site cfg.Sites
// excludes (ADR-044 decision 8, I14) — appended to decision_provider's own
// Reason once the provider itself is proven reachable, so an operator sees
// not just "the provider works" but which of its sites are actually
// enabled. "" when every site is allowed (the common, unconfigured case),
// so an unnarrowed [decide] block leaves the Reason line unchanged.
func blockedSitesSuffix(cfg config.Decide) string {
	var blocked []string
	for _, site := range decide.AllSites {
		if !decide.SiteAllowed(cfg, site) {
			blocked = append(blocked, site)
		}
	}
	if len(blocked) == 0 {
		return ""
	}
	return fmt.Sprintf(" (sites off: %s)", strings.Join(blocked, ", "))
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
		reason += blockedSitesSuffix(dc) + uncalibratedNote
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}

	case "typesafe", "openai":
		return remoteProviderVerdict(dc, callLogPath)

	default:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("unknown [decide] provider %q (want \"none\", \"llm\", \"typesafe\", or \"openai\")", dc.Provider),
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

// uncalibratedNote ends a configured provider's passing Reason: a
// reachable provider is not a calibrated one (ADR-056 R3), and the
// thresholds in [decide] mean nothing for it until a reading grades it.
const uncalibratedNote = "; its thresholds need a calibration reading (" + calibrationReadingGlob + ") before they mean anything for this provider"

// remoteProviderVerdict is the row for provider typesafe or openai (DP-34):
// the transport and base URL must keep the key on the provider's own host
// (decide.CheckRemote), the key's source must resolve — checked without
// reading a key out of a file (config.KeySource.Probe) — and a one-question
// ping through the kit adapter must answer. The Reason names provider,
// model, transport, endpoint and the key's source and state, never the key.
func remoteProviderVerdict(dc config.Decide, callLogPath string) doctorCheck {
	const name = "decision_provider"
	transport := dc.EffectiveTransport()
	model := decide.ResolvedModel(dc)
	endpoint := decide.ResolvedEndpoint(dc)
	head := fmt.Sprintf("[decide] provider=%s model=%s transport=%s endpoint=%s", dc.Provider, model, transport, endpoint)

	if err := decide.CheckRemote(dc); err != nil {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("%s: %v", head, err),
			Remedy: "fix [decide] transport/base_url in .fathom/config.toml (a host other than the provider's own belongs in ~/.fathom/config.toml)",
		}
	}
	src := dc.KeySource()
	state, err := src.Probe(context.Background())
	if err != nil {
		remedy := "in ~/.fathom/config.toml (key sources are read from the user config only), set [decide] api_key_file to a chmod 600 file holding the key (the launchd daemon does not see your shell's environment, so a file is the way to hand it a key), or api_key_env"
		if src.File != "" {
			remedy = "make the key file a regular, non-empty file with chmod 600, or point [decide] api_key_file (in ~/.fathom/config.toml) at one"
		}
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("%s: no API key: %v", head, err),
			Remedy: remedy,
		}
	}
	keyLine := fmt.Sprintf("key=%s (%s)", src, state)

	ports, err := decide.Open(dc, nil, "", nil)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("%s %s: %v", head, keyLine, err), Remedy: "fix [decide] in .fathom/config.toml"}
	}
	pinger, ok := ports.Adapter.(interface{ Ping(context.Context) error })
	if ok {
		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pinger.Ping(pingCtx); err != nil {
			return doctorCheck{
				Name: name, Status: doctorFail,
				// Never the key: the kit's adapters redact it from every
				// error (an HTTP error body's Authorization echo included).
				Reason: fmt.Sprintf("%s %s: ping failed: %v", head, keyLine, err),
				Remedy: "check [decide] base_url, the model and the key's validity with the provider",
			}
		}
	}
	reason := fmt.Sprintf("%s %s ping ok", head, keyLine)
	if latencyMS, ok := lastDecisionLatencyMS(callLogPath); ok {
		reason = fmt.Sprintf("%s (last decision latency_ms=%d)", reason, latencyMS)
	}
	reason += blockedSitesSuffix(dc) + uncalibratedNote
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
