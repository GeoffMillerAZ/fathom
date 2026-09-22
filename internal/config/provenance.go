package config

import "sort"

// Source names which precedence tier supplied a resolved value
// (docs/plans/config-experience.md: flags > env > project > user >
// built-in defaults). `fathom config --explain` renders this per key.
type Source string

const (
	SourceDefault Source = "default"
	SourceUser    Source = "user"
	SourceProject Source = "project"
	SourceEnv     Source = "env"
	SourceFlag    Source = "flag"
)

// SourceInfo names the source and, for file/env sources, the file path or
// environment variable name that produced a value.
type SourceInfo struct {
	Source Source
	Path   string
}

// Provenance maps a dotted key (e.g. "default_profile",
// "profiles.work.provider", "daemon.addr") to the SourceInfo that last set
// it. Keys absent from the map were never set by any layer — the caller's
// own built-in default applies, with Source SourceDefault.
type Provenance map[string]SourceInfo

// clone returns a shallow copy — merges must never mutate a caller's map in
// place, since ResolveProfile hands back a filtered view derived from a
// shared Loaded.Provenance.
func (p Provenance) clone() Provenance {
	out := make(Provenance, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// mergeLayer applies one parsed-and-validated TOML document onto cfg,
// recording provenance for every key raw actually sets. Only keys PRESENT
// in raw are applied — an absent key never overwrites a lower-precedence
// layer's value, which is what makes user+project layering additive rather
// than a full replacement.
func mergeLayer(cfg *Config, prov Provenance, raw map[string]any, source Source, path string) {
	if v, ok := stringField(raw, "default_profile"); ok {
		cfg.DefaultProfile = v
		prov["default_profile"] = SourceInfo{source, path}
	}

	if pm, ok := raw["profiles"].(map[string]any); ok {
		names := make([]string, 0, len(pm))
		for name := range pm {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			entry, _ := pm[name].(map[string]any)
			existing := cfg.Profiles[name]
			mergeProfileFields(&existing, entry, name, source, path, prov)
			cfg.Profiles[name] = existing
		}
	}

	if dm, ok := raw["daemon"].(map[string]any); ok {
		mergeDaemonFields(&cfg.Daemon, dm, source, path, prov)
	}

	if gm, ok := raw["guards"].(map[string]any); ok {
		mergeGuardsFields(&cfg.Guards, gm, source, path, prov)
	}

	if fm, ok := raw["findings"].(map[string]any); ok {
		mergeFindingsFields(&cfg.Findings, fm, source, path, prov)
	}

	if dcm, ok := raw["decide"].(map[string]any); ok {
		mergeDecideFields(&cfg.Decide, dcm, source, path, prov)
	}
}

func mergeProfileFields(p *Profile, raw map[string]any, name string, source Source, path string, prov Provenance) {
	prefix := "profiles." + name + "."
	set := func(key string, dst *string) {
		if v, ok := stringField(raw, key); ok {
			*dst = v
			prov[prefix+key] = SourceInfo{source, path}
		}
	}
	set("provider", &p.Provider)
	set("model", &p.Model)
	set("endpoint", &p.Endpoint)
	set("region", &p.Region)
	set("deployment", &p.Deployment)
	set("api_version", &p.APIVersion)
	set("bin", &p.Bin)
	set("api_key_env", &p.APIKeyEnv)

	if v, ok := raw["api_key_cmd"]; ok {
		if list, ok := stringSlice(v); ok {
			p.APIKeyCmd = list
			prov[prefix+"api_key_cmd"] = SourceInfo{source, path}
		}
	}
}

func mergeDaemonFields(d *Daemon, raw map[string]any, source Source, path string, prov Provenance) {
	set := func(key string, dst *string) {
		if v, ok := stringField(raw, key); ok {
			*dst = v
			prov["daemon."+key] = SourceInfo{source, path}
		}
	}
	set("store", &d.Store)
	set("addr", &d.Addr)
	set("call_log", &d.CallLog)
	set("default_profile", &d.DefaultProfile)
	set("state_dir", &d.StateDir)

	if v, ok := raw["auto_refresh"]; ok {
		if b, ok := v.(bool); ok {
			d.AutoRefresh = &b
			prov["daemon.auto_refresh"] = SourceInfo{source, path}
		}
	}
}

func mergeGuardsFields(g *Guards, raw map[string]any, source Source, path string, prov Provenance) {
	if v, ok := raw["enforce"]; ok {
		if b, ok := v.(bool); ok {
			g.Enforce = &b
			prov["guards.enforce"] = SourceInfo{source, path}
		}
	}
}

func mergeFindingsFields(f *Findings, raw map[string]any, source Source, path string, prov Provenance) {
	set := func(key string, dst *string) {
		if v, ok := stringField(raw, key); ok {
			*dst = v
			prov["findings."+key] = SourceInfo{source, path}
		}
	}
	set("subject", &f.Subject)
	set("outbox", &f.Outbox)

	if v, ok := raw["producers"]; ok {
		if list, ok := stringSlice(v); ok {
			f.Producers = list
			prov["findings.producers"] = SourceInfo{source, path}
		}
	}
}

func mergeDecideFields(d *Decide, raw map[string]any, source Source, path string, prov Provenance) {
	if v, ok := stringField(raw, "provider"); ok {
		d.Provider = v
		prov["decide.provider"] = SourceInfo{source, path}
	}
	if v, ok := stringField(raw, "profile"); ok {
		d.Profile = v
		prov["decide.profile"] = SourceInfo{source, path}
	}
	if v, ok := numberField(raw, "compile_threshold"); ok {
		d.CompileThreshold = v
		prov["decide.compile_threshold"] = SourceInfo{source, path}
	}
	if v, ok := numberField(raw, "route_confidence"); ok {
		d.RouteConfidence = v
		prov["decide.route_confidence"] = SourceInfo{source, path}
	}
	if v, ok := raw["rank_within_tier"]; ok {
		if b, ok := v.(bool); ok {
			d.RankWithinTier = b
			prov["decide.rank_within_tier"] = SourceInfo{source, path}
		}
	}
}

// numberField reads a TOML numeric field as float64 — go-toml decodes a
// bare integer literal (e.g. `route_confidence = 1`) as int64 and anything
// with a decimal point as float64, so both are accepted here.
func numberField(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func stringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func stringSlice(v any) ([]string, bool) {
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
