package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// envDefaultAPIKeyVar returns the environment variable an adapter reads for
// its credential when a profile does not name one explicitly — the SAME
// variable name cmd/fathom's pre-profile build*Client functions already
// read (buildOpenAIClient's OPENAI_API_KEY, buildAzureOpenAIClient's
// AZURE_OPENAI_API_KEY) or, for anthropic, the kit adapter's own internal
// default (anthropic.Config's APIKeyEnv default) — named here too so every
// provider's credential resolution goes through the one ResolveAPIKey path
// instead of anthropic being a special case.
func envDefaultAPIKeyVar(provider string) string {
	switch provider {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderOpenAI:
		return "OPENAI_API_KEY"
	case ProviderAzureOpenAI:
		return "AZURE_OPENAI_API_KEY"
	default:
		return "" // claude-cli, bedrock, codex-cli: no api-key concept
	}
}

// ResolveProfile resolves the ACTIVE Profile: flagProfile (from a --profile
// flag), when non-empty, names the profile to use; otherwise cfg's merged
// default_profile applies; otherwise no named profile is selected at all
// and the result is built purely from the env-overlay (reproducing
// pre-profile fathom's behavior byte-for-byte when no config file is
// present, as the file's Design section requires).
//
// Env vars ALWAYS win over a file-sourced profile field — they sit at the
// env precedence tier, above project/user files (flags > env > project >
// user > defaults) — which is what "the env-overlay profile" means: the
// process environment is itself an implicit, unnamed profile that layers on
// top of whichever named profile was selected.
//
// Returns an error naming the known profiles when flagProfile or
// cfg.DefaultProfile names one that is not defined.
func ResolveProfile(cfg Config, prov Provenance, flagProfile string) (Profile, Provenance, error) {
	if prov == nil {
		prov = Provenance{}
	}

	name := flagProfile
	if name == "" {
		name = cfg.DefaultProfile
	}

	var base Profile
	baseProv := Provenance{}
	if name != "" {
		p, ok := cfg.Profiles[name]
		if !ok {
			return Profile{}, nil, fmt.Errorf("config: profile %q not found (known profiles: %s)", name, strings.Join(sortedProfileNames(cfg.Profiles), ", "))
		}
		base = p
		prefix := "profiles." + name + "."
		for k, v := range prov {
			if rest, ok := strings.CutPrefix(k, prefix); ok {
				baseProv[rest] = v
			}
		}
	}

	resolved, resolvedProv := applyEnvOverlay(base, baseProv)
	return resolved, resolvedProv, nil
}

// applyEnvOverlay layers the process environment onto base — the exact
// variables cmd/fathom's pre-profile build*Client functions read, plus the
// provider-appropriate default api_key_env when neither a file profile nor
// the env overlay names one explicitly.
func applyEnvOverlay(base Profile, baseProv Provenance) (Profile, Provenance) {
	out := base
	prov := baseProv.clone()

	setEnv := func(field *string, key, envVar string) {
		if v := os.Getenv(envVar); v != "" {
			*field = v
			prov[key] = SourceInfo{SourceEnv, envVar}
		}
	}

	setEnv(&out.Provider, "provider", "FATHOM_LLM_PROVIDER")
	if out.Provider == "" {
		out.Provider = ProviderAnthropic
		if _, ok := prov["provider"]; !ok {
			prov["provider"] = SourceInfo{SourceDefault, ""}
		}
	}

	setEnv(&out.Model, "model", "FATHOM_COMPILE_MODEL")

	switch out.Provider {
	case ProviderClaudeCLI:
		setEnv(&out.Bin, "bin", "FATHOM_CLAUDE_BIN")
	case ProviderCodexCLI:
		setEnv(&out.Bin, "bin", "FATHOM_CODEX_BIN")
	case ProviderOpenAI:
		setEnv(&out.Endpoint, "endpoint", "FATHOM_OPENAI_BASE_URL")
	case ProviderAzureOpenAI:
		setEnv(&out.Endpoint, "endpoint", "AZURE_OPENAI_ENDPOINT")
		setEnv(&out.Deployment, "deployment", "AZURE_OPENAI_DEPLOYMENT")
		setEnv(&out.APIVersion, "api_version", "AZURE_OPENAI_API_VERSION")
	case ProviderBedrock:
		setEnv(&out.Region, "region", "AWS_REGION")
	}

	if out.APIKeyEnv == "" {
		if v := envDefaultAPIKeyVar(out.Provider); v != "" {
			out.APIKeyEnv = v
			if _, ok := prov["api_key_env"]; !ok {
				prov["api_key_env"] = SourceInfo{SourceDefault, ""}
			}
		}
	}

	return out, prov
}

func sortedProfileNames(profiles map[string]Profile) []string {
	names := make([]string, 0, len(profiles))
	for n := range profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
