package main

// decideports.go — DP-9/DP-10 (docs/plans/decision-port.md, ADR-042): the
// one place every fathom entry point (the HTTP daemon, the stdio daemon,
// and a direct --store compile) opens the decision port. Wave A's
// internal/decide.Open already does the provider switch; this file is
// purely the fail-open glue around it — I14: a misconfigured or
// unreachable [decide] provider must NEVER stop the daemon from starting or
// the CLI from compiling. Every failure here is a warn line and a
// decision.None-backed *decide.Ports, never a returned error. A provider
// that opens but then fails a call (a timeout, a 429 or 529, the daemon's
// refuse rung or safe mode) degrades per call instead (JF-6): one "warn:
// decision port degraded" line to the same stderr, and the site's
// deterministic path (internal/decide/degrade.go).
import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// openDecisionPorts builds the decision port from loaded.Config.Decide.
//
// provider "" or "none" (config.DefaultDecide's own default) opens
// decide.None via decide.Open with no lane at all — never a warn, since
// "off" is the ordinary, unconfigured state every fathom install starts in.
//
// provider "llm" resolves [decide].profile the SAME fallback chain the
// compile lane already uses (servehttp.go's daemonProfileName,
// compileViaStore's --profile flag): an explicit [decide].profile wins,
// empty falls back to daemonProfileName, which config.ResolveProfile
// resolves the rest of the way (its own default_profile fallback, then the
// env overlay). Any failure resolving the profile, building the client, or
// decide.Open itself prints one "warn: decision port disabled: ..." line to
// stderr and returns None ports instead — the port never fails startup
// (I14).
//
// label prefixes the one startup line printed when the port comes up as
// "llm" (e.g. "serve-http", "serve-mcp", "fathom compile") — mirroring the
// existing "serve-http: compile_layer enabled (model %s)" convention
// (servehttp.go).
//
// The decisions.jsonl recorder is opened independently of the provider:
// decide.DefaultRecorderPath(callLogPath) beside the call log, in append
// mode, creating parent directories as needed. "" (no call log configured)
// or an open failure both yield a nil Recorder plus a warn — decide.Recorder
// is a documented safe no-op on a nil receiver, so every call site that logs
// through it degrades silently from there.
//
// KS-1 to KS-6 (ADR-073 decision 5, I66): decide.Open closes the port
// first when FATHOM_DECIDE=off, the kill file, the user's enabled = false
// or a refused consent says so; a configured provider then prints one
// "decision port closed" line naming the source, and provider none prints
// nothing (I42). An open port re-checks the kill file and loaded.UserPath's
// enabled before each ask, so the daemon that opens it once here closes
// without a restart.
func openDecisionPorts(loaded config.Loaded, daemonProfileName, callLogPath, label string, stderr io.Writer) *decide.Ports {
	cfg := loaded.Config.Decide
	rec := openDecisionRecorder(callLogPath, stderr)
	opts := []decide.Option{decide.WithDisclose(stderr), decide.WithUserConfig(loaded.UserPath)}

	if cfg.Provider != "llm" {
		ports, err := decide.Open(cfg, nil, "", rec, opts...)
		if err != nil {
			// Reachable for an unknown provider value (config.Load's own
			// CUE schema already refuses that at load time — #Decide's
			// provider enum, schema/config/schema.cue — but a hand-edited
			// config.toml or a stale schema could still reach here) OR,
			// for provider "typesafe" or "openai", a key source that does
			// not resolve or a base URL off the provider's own host —
			// decide.Open's own error names the source and the fix.
			// Either way: fail open (I14) rather than refuse to start.
			fmt.Fprintf(stderr, "warn: decision port disabled: %v\n", err)
			return &decide.Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}
		}
		if closedPortLine(cfg, ports.Gate, label, stderr) {
			return ports
		}
		switch cfg.Provider {
		case "typesafe", "openai":
			// The transport is named only off the provider's own API, so a
			// typesafe port that opened before transports existed prints
			// the line it always printed.
			transport := ""
			if t := cfg.EffectiveTransport(); t != cfg.Provider {
				transport = ", transport " + t
			}
			fmt.Fprintf(stderr, "%s: decision port %s (model %s%s)\n", label, cfg.Provider, decide.ResolvedModel(cfg), transport)
		}
		return ports
	}

	// The gate before the profile: a closed port resolves no profile and
	// builds no lane (KS-1).
	if gate := decide.Check(cfg, ""); gate.Closed {
		closedPortLine(cfg, gate, label, stderr)
		return &decide.Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec, Gate: gate}
	}
	profileName := cfg.Profile
	if profileName == "" {
		profileName = daemonProfileName
	}
	p, _, profErr := config.ResolveProfile(loaded.Config, loaded.Provenance, profileName)
	if profErr != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: %v\n", profErr)
		return &decide.Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}
	}
	model := compileModelForProfile(p)
	lane := func(string) (llmclient.LLMClient, error) { return buildLLMClientForProfile(p) }

	ports, err := decide.Open(cfg, lane, model, rec, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: %v\n", err)
		return &decide.Ports{Decider: decision.None{}, Cfg: cfg, Recorder: rec}
	}
	if closedPortLine(cfg, ports.Gate, label, stderr) {
		return ports
	}
	fmt.Fprintf(stderr, "%s: decision port llm (model %s)\n", label, model)
	return ports
}

// closedPortLine prints the one startup line for a configured provider
// whose port the gate closed (KS-1 to KS-6) and reports whether it did;
// provider none prints nothing, whatever the gate says (I42).
func closedPortLine(cfg config.Decide, gate decide.Gate, label string, stderr io.Writer) bool {
	if !gate.Closed {
		return false
	}
	switch cfg.Provider {
	case "", "none":
		return true
	}
	fmt.Fprintf(stderr, "warn: %s: decision port %s closed (kill_source %s, %s): %s\n", label, cfg.Provider, gate.Source, gate.Layer, gate.Reason)
	return true
}

// openDecisionRecorder opens (creating parent directories as needed) the
// decisions.jsonl log beside callLogPath in append mode. "" (no call log
// configured at all) or any open failure returns nil — decide.Recorder's
// documented safe no-op — plus a warn line for the latter (a "" path is the
// ordinary unconfigured state, never worth a warn).
func openDecisionRecorder(callLogPath string, stderr io.Writer) *decide.Recorder {
	path := decide.DefaultRecorderPath(callLogPath)
	if path == "" {
		return nil
	}
	f, err := openCallLog(path)
	if err != nil {
		fmt.Fprintf(stderr, "warn: decision log disabled: %v\n", err)
		return nil
	}
	rec := decide.NewRecorder(f, time.Now, buildinfo.Version())
	seedDecisionRecorder(rec, path)
	return rec
}

// seedDecisionRecorder hands rec the last decide.SeedTailBytes of the
// existing log (JF-10), so a model move across a restart still marks the
// site's first new row "recalibration_due". Best effort: an unreadable log
// seeds nothing.
func seedDecisionRecorder(rec *decide.Recorder, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > decide.SeedTailBytes {
		if _, err := f.Seek(st.Size()-decide.SeedTailBytes, io.SeekStart); err != nil {
			return
		}
	}
	rec.Seed(f) // a partial first line fails to parse and is skipped
}
