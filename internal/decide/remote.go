package decide

// remote.go — the two remote providers (DP-31..DP-33, docs/plans/
// decision-port.md section H; ADR-042 amendment 2026-10-09): provider
// "typesafe" (Jev over TypeSafe's own API or OpenRouter's Decisions router)
// and provider "openai" (OpenAI's Decisions API), each the kit's own
// adapter. Fathom names the key's source (config.KeySource) and hands the
// adapter a file path or a variable name; the adapter reads the key at
// first use. Before that, Open checks two things the adapter cannot: the
// source resolves (a missing key disables the port at startup with one
// line, JF-6), and the key's destination is the provider's own host.

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/typesafe"
)

// ResolvedModel is the model a remote provider is asked for: [decide]
// model, else the kit adapter's default for the transport. "" for a
// provider with no model of its own here (none, llm).
func ResolvedModel(cfg config.Decide) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	switch cfg.EffectiveTransport() {
	case config.DecideTransportTypeSafe:
		return typesafe.DefaultTypeSafeModel
	case config.DecideTransportOpenRouter:
		return typesafe.DefaultOpenRouterModel
	case config.DecideTransportOpenAI:
		return openai.DefaultOpenAIModel
	}
	return ""
}

// ResolvedBaseURL is the origin a remote provider's requests go to:
// [decide] base_url (or its older name, endpoint), else the kit adapter's
// default for the transport. "" for none and llm.
func ResolvedBaseURL(cfg config.Decide) string {
	if u := cfg.EffectiveBaseURL(); u != "" {
		return u
	}
	return defaultBaseURL(cfg.EffectiveTransport())
}

// ResolvedEndpoint is the full URL a remote provider's requests are posted
// to: ResolvedBaseURL plus the transport's path, with any user information
// in the base URL dropped, since it is for display (fathom doctor).
func ResolvedEndpoint(cfg config.Decide) string {
	base := strings.TrimSuffix(withoutUserinfo(ResolvedBaseURL(cfg)), "/")
	switch cfg.EffectiveTransport() {
	case config.DecideTransportTypeSafe:
		return base + "/v1/systemone"
	case config.DecideTransportOpenRouter:
		return base + typesafe.DefaultOpenRouterPath
	case config.DecideTransportOpenAI:
		return base + "/decisions"
	}
	return base
}

// defaultBaseURL is the kit adapter's own origin for a transport.
func defaultBaseURL(transport string) string {
	switch transport {
	case config.DecideTransportTypeSafe:
		return typesafe.DefaultTypeSafeBaseURL
	case config.DecideTransportOpenRouter:
		return typesafe.DefaultOpenRouterBaseURL
	case config.DecideTransportOpenAI:
		return openai.DefaultOpenAIBaseURL
	}
	return ""
}

// CheckRemote reports why cfg's remote provider must not be opened, or nil:
// a transport the provider does not speak, a key named for another
// transport, or a base URL that would send the key somewhere other than
// the provider's own API. It reads no key.
func CheckRemote(cfg config.Decide) error {
	transport := cfg.EffectiveTransport()
	switch {
	case cfg.Provider == "openai" && transport != config.DecideTransportOpenAI,
		cfg.Provider == "typesafe" && transport != config.DecideTransportTypeSafe && transport != config.DecideTransportOpenRouter:
		return fmt.Errorf("[decide] transport %q is not one provider %s speaks", transport, cfg.Provider)
	}
	if err := checkKeyTransport(cfg, transport); err != nil {
		return err
	}
	if err := checkAmbientKey(cfg); err != nil {
		return err
	}
	return checkKeyDestination(cfg, transport)
}

// checkAmbientKey refuses a transport's well-known key variable
// (OPENROUTER_API_KEY, ...) for a provider the user layer did not name
// (KS-4, I66): a repository's provider the user's allow_repo_provider let
// open still uses only a key the user's own config names, never one the
// user's environment exports for other tools. A Decide built in code (no
// layer recorded) is its caller's own.
func checkAmbientKey(cfg config.Decide) error {
	src := cfg.KeySource()
	if !src.Default || (cfg.ProviderSource == "" && cfg.TransportSource == "") || userNamedProvider(cfg) {
		return nil
	}
	return fmt.Errorf("[decide] provider %s was named by the repository, so %s is not read (a transport's well-known variable counts only when ~/.fathom/config.toml names the provider); name the key with api_key_file or api_key_env, and the transport it is for (transport = %q), in ~/.fathom/config.toml", cfg.Provider, src, cfg.EffectiveTransport())
}

// checkKeyTransport refuses a named key source (api_key_cmd, api_key_file
// or api_key_env) that config.Load recorded for another transport: the
// config layer that named it set another provider or transport, or none,
// and a later layer moved the port without naming its own key. A user
// layer that names a transport beside its key and no provider binds the
// key to that transport, so a repository's provider the user's
// allow_repo_provider lets open uses it there and nowhere else (O1133). A
// transport's well-known variable (OPENAI_API_KEY, ...) is bound to it by
// name, and a Decide built in code (KeyTransport "") is its caller's own.
func checkKeyTransport(cfg config.Decide, transport string) error {
	src := cfg.KeySource()
	if src.Default || cfg.KeyTransport == "" || cfg.KeyTransport == transport {
		return nil
	}
	namedFor, beside := fmt.Sprintf("transport %q", cfg.KeyTransport), ""
	if cfg.KeyTransport == config.KeyTransportNone {
		namedFor = "no remote provider"
		beside = fmt.Sprintf(", or name the transport it is for beside it (transport = %q)", transport)
	}
	return fmt.Errorf("[decide] %s was named for %s, and this port speaks %q: a key goes only to the provider it was named for; name this provider's key (api_key_file or api_key_env) in the same config file that sets its provider and transport%s", src, namedFor, transport, beside)
}

// DisplayBaseURL is the base URL [decide] names (base_url, else endpoint)
// with any user information dropped, for a status surface (fathom decide
// log, the console's /decide page): CheckRemote refuses a base URL that
// carries credentials, and no surface repeats them.
func DisplayBaseURL(cfg config.Decide) string {
	return withoutUserinfo(cfg.EffectiveBaseURL())
}

// checkKeyDestination refuses a base URL that would carry the key off the
// provider's own API: plain http to anything but a loopback host, or a host
// other than the transport's default that the user's own config
// (~/.fathom/config.toml) did not name. A repository's .fathom/config.toml
// is the repository's to write, so it may point a provider at a loopback
// fake or at the provider's own host, never at a third host holding the
// user's key.
func checkKeyDestination(cfg config.Decide, transport string) error {
	raw := cfg.EffectiveBaseURL()
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("[decide] base_url is not an absolute URL (the value is not repeated here)")
	}
	if u.User != nil {
		return fmt.Errorf("[decide] base_url carries credentials (user information before the host); name the key with api_key_file or api_key_env instead")
	}
	host := u.Hostname()
	if isLoopback(host) {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("[decide] base_url host %q would get the API key over plain http; use https", host)
	}
	if d, err := url.Parse(defaultBaseURL(transport)); err == nil && strings.EqualFold(host, d.Hostname()) {
		return nil
	}
	if cfg.BaseURLSource == config.SourceUser {
		return nil
	}
	return fmt.Errorf("[decide] base_url host %q is not provider %s's own API (%s), and a repository's .fathom/config.toml may not send your API key to another host; name it in ~/.fathom/config.toml to allow it", host, cfg.Provider, defaultBaseURL(transport))
}

// withoutUserinfo returns raw with any user information removed, or raw
// unchanged when it does not parse as a URL with a host.
func withoutUserinfo(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// isLoopback reports whether host names this machine.
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// openRemote builds the kit adapter for provider typesafe or openai. Every
// error names the provider and, for the key, its source, never the key.
func openRemote(cfg config.Decide, o openOptions) (decision.Decider, error) {
	if err := CheckRemote(cfg); err != nil {
		return nil, fmt.Errorf("decide: [decide] provider %s: %w", cfg.Provider, err)
	}
	src := cfg.KeySource()
	if _, err := src.Probe(context.Background(), o.keyIO); err != nil {
		return nil, fmt.Errorf("decide: [decide] provider %s: no API key: %w (in ~/.fathom/config.toml, set api_key_file to a chmod 600 file, or api_key_env)", cfg.Provider, err)
	}
	var key, keyFile, keyEnv string
	switch {
	case len(src.Cmd) > 0:
		k, err := cfg.ResolveAPIKey(context.Background(), o.keyIO)
		if err != nil {
			return nil, fmt.Errorf("decide: [decide] provider %s: %w", cfg.Provider, err)
		}
		key = k
	case src.File != "":
		keyFile = src.File
	default:
		keyEnv = src.Env
	}
	switch cfg.Provider {
	case "openai":
		o, err := openai.NewOpenAI(openai.OpenAIConfig{APIKey: key, APIKeyFile: keyFile, APIKeyEnv: keyEnv, BaseURL: cfg.EffectiveBaseURL(), Model: cfg.Model, HTTPClient: o.httpClient})
		if err != nil {
			return nil, fmt.Errorf("decide: constructing openai decider: %w", err)
		}
		return o, nil
	default: // "typesafe": Open dispatches only the two remote providers here
		transport := cfg.EffectiveTransport()
		ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: key, APIKeyFile: keyFile, APIKeyEnv: keyEnv, Transport: transport, BaseURL: cfg.EffectiveBaseURL(), Model: cfg.Model, HTTPClient: o.httpClient})
		if err != nil {
			return nil, fmt.Errorf("decide: constructing typesafe decider: %w", err)
		}
		return ts, nil
	}
}
