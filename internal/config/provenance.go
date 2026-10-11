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

	if tm, ok := raw["tests"].(map[string]any); ok {
		mergeTestsFields(&cfg.Tests, tm, source, path, prov)
	}

	if hm, ok := raw["hooks"].(map[string]any); ok {
		if b, ok := hm["test_run"].(bool); ok {
			cfg.Hooks.TestRun = b
			prov["hooks.test_run"] = SourceInfo{source, path}
		}
		if s, ok := hm["pin_check"].(string); ok {
			cfg.Hooks.PinCheck = s
			prov["hooks.pin_check"] = SourceInfo{source, path}
		}
	}

	if cm, ok := raw["connector"].(map[string]any); ok {
		if v, ok := cm["include"]; ok {
			if list, ok := stringSlice(v); ok {
				cfg.Connector.Include = list
				prov["connector.include"] = SourceInfo{source, path}
			}
		}
	}

	if rm, ok := raw["receipts"].(map[string]any); ok {
		if v, ok := stringField(rm, "repository_uri"); ok {
			cfg.Receipts.RepositoryURI = v
			prov["receipts.repository_uri"] = SourceInfo{source, path}
		}
	}

	if sm, ok := raw["surface"].(map[string]any); ok {
		mergeSurfaceFields(&cfg.Surface, sm, source, path, prov)
	}

	if cm, ok := raw["console"].(map[string]any); ok {
		if v, ok := numberField(cm, "conserve_max_files"); ok {
			cfg.Console.ConserveMaxFiles = int(v)
			prov["console.conserve_max_files"] = SourceInfo{source, path}
		}
	}

	// [work] ci_summaries (WI-2w): an array of tables, so the layer that
	// sets it wins outright, like every other slice-valued field.
	if wm, ok := raw["work"].(map[string]any); ok {
		if list, ok := wm["ci_summaries"].([]any); ok {
			sums := make([]CISummary, 0, len(list))
			for _, entry := range list {
				m, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				var c CISummary
				c.Path, _ = stringField(m, "path")
				c.Commit, _ = stringField(m, "commit")
				sums = append(sums, c)
			}
			cfg.Work.CISummaries = sums
			prov["work.ci_summaries"] = SourceInfo{source, path}
		}
	}

	// [zones] (WI-1): like every other slice-valued field, the layer that
	// sets the table wins outright — a zone map has no per-entry merge
	// that would not silently mix two layers' partitions.
	if zm, ok := raw["zones"].(map[string]any); ok {
		zones := make(map[string][]string, len(zm))
		for name, v := range zm {
			arr, _ := v.([]any)
			for _, g := range arr {
				if s, ok := g.(string); ok {
					zones[name] = append(zones[name], s)
				}
			}
		}
		cfg.Zones = zones
		prov["zones"] = SourceInfo{source, path}
	}
}

// mergeSurfaceFields merges the `[surface]` table's `pattern` array-of-
// tables. Like Findings.Producers/Profile.APIKeyCmd, a present `pattern`
// key REPLACES the whole slice rather than merging entry-by-entry — an
// array-of-tables has no natural per-element merge key, so "the layer that
// sets it wins outright" is the same discipline every other slice field in
// this package already follows.
func mergeSurfaceFields(s *Surface, raw map[string]any, source Source, path string, prov Provenance) {
	v, ok := raw["pattern"]
	if !ok {
		return
	}
	list, ok := v.([]any)
	if !ok {
		return
	}
	patterns := make([]SurfacePattern, 0, len(list))
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		var p SurfacePattern
		p.Kind, _ = stringField(m, "kind")
		p.Glob, _ = stringField(m, "glob")
		p.Regex, _ = stringField(m, "regex")
		patterns = append(patterns, p)
	}
	s.Pattern = patterns
	prov["surface.pattern"] = SourceInfo{source, path}
}

func mergeTestsFields(t *Tests, raw map[string]any, source Source, path string, prov Provenance) {
	set := func(key string, dst *string) {
		if v, ok := stringField(raw, key); ok {
			*dst = v
			prov["tests."+key] = SourceInfo{source, path}
		}
	}
	set("results_dir", &t.ResultsDir)
	set("artifacts_dir", &t.ArtifactsDir)
	if v, ok := raw["commands"]; ok {
		if list, ok := stringSlice(v); ok {
			t.Commands = list
			prov["tests.commands"] = SourceInfo{source, path}
		}
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
	set("timeout", &p.Timeout)
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
	set("memory_limit", &d.MemoryLimit)

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
	if v, ok := numberField(raw, "route_timeout_ms"); ok {
		d.RouteTimeoutMS = int(v)
		prov["decide.route_timeout_ms"] = SourceInfo{source, path}
	}
	if v, ok := raw["rank_within_tier"]; ok {
		if b, ok := v.(bool); ok {
			d.RankWithinTier = b
			prov["decide.rank_within_tier"] = SourceInfo{source, path}
		}
	}
	if v, ok := numberField(raw, "rerank_timeout_ms"); ok {
		d.RerankTimeoutMS = int(v)
		prov["decide.rerank_timeout_ms"] = SourceInfo{source, path}
	}
	if v, ok := stringField(raw, "base_url"); ok {
		d.BaseURL = v
		prov["decide.base_url"] = SourceInfo{source, path}
	}
	if v, ok := stringField(raw, "endpoint"); ok {
		d.Endpoint = v
		prov["decide.endpoint"] = SourceInfo{source, path}
	}
	// The layer that last set the base URL in effect (base_url, else the
	// older endpoint): a project-layer one may not redirect a key off the
	// provider's own host (internal/decide).
	if _, ok := raw["base_url"]; ok {
		d.BaseURLSource = source
	} else if _, ok := raw["endpoint"]; ok && d.BaseURL == "" {
		d.BaseURLSource = source
	}
	if v, ok := stringField(raw, "transport"); ok {
		d.Transport = v
		prov["decide.transport"] = SourceInfo{source, path}
	}
	if v, ok := stringField(raw, "model"); ok {
		d.Model = v
		prov["decide.model"] = SourceInfo{source, path}
	}
	// The key source is one setting (KeySource): a layer that names any of
	// api_key_cmd, api_key_file or api_key_env replaces all three from a
	// lower layer, and the transport in effect in this layer is recorded
	// (KeyTransport, at the end), so a key is sent only over the transport
	// it was named for.
	namesKey := false
	for _, k := range []string{"api_key_cmd", "api_key_file", "api_key_env"} {
		if _, ok := raw[k]; ok {
			namesKey = true
		}
	}
	if namesKey {
		d.APIKeyCmd, d.APIKeyFile, d.APIKeyEnv = nil, "", ""
		delete(prov, "decide.api_key_cmd")
		delete(prov, "decide.api_key_file")
		delete(prov, "decide.api_key_env")
	}
	if v, ok := stringField(raw, "api_key_file"); ok {
		d.APIKeyFile = v
		prov["decide.api_key_file"] = SourceInfo{source, path}
	}
	if v, ok := stringField(raw, "api_key_env"); ok {
		d.APIKeyEnv = v
		prov["decide.api_key_env"] = SourceInfo{source, path}
	}
	if v, ok := raw["api_key_cmd"]; ok {
		if list, ok := stringSlice(v); ok {
			d.APIKeyCmd = list
			prov["decide.api_key_cmd"] = SourceInfo{source, path}
		}
	}
	if v, ok := raw["sites"]; ok {
		if list, ok := stringSlice(v); ok {
			d.Sites = list
			prov["decide.sites"] = SourceInfo{source, path}
			// JF-4: only the project layer is the repository's own list,
			// the one that may opt it into an opt-in site.
			if source == SourceProject {
				d.RepoSites = list
			}
		}
	}
	if namesKey {
		d.KeyTransport = d.EffectiveTransport()
		if d.KeyTransport == "" {
			d.KeyTransport = KeyTransportNone
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
