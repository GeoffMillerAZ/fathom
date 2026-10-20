package main

// capabilities_config.go — FD-34 (docs/plans/fathom-dir.md, A2): the
// config.toml switches whose field is not user-only surface as capability
// ids, valued by the same booleans. A switch is surfaced only when a layer
// of the file sets it, so an unset key falls through to the capability's
// built-in Default, which holds what the file means by silence.

import (
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// configSwitchKeys pairs a config.toml key with the capability id that
// surfaces it. daemon.auto_refresh is listed so the user-only rule, not
// omission, keeps it out (I64, SEC-2).
func configSwitchKeys() [][2]string {
	return [][2]string{
		{"hooks.test_run", "test-run-hook"},
		{"hooks.pin_check", "pin-check"},
		{"guards.enforce", "guard-enforce"},
		{"daemon.auto_refresh", "auto-refresh"},
	}
}

// configSwitchSettings is the Config layer of a capability Resolution: the
// capability ids whose config.toml key is set, not user-only, and known to
// the registry. Any load problem yields no switches (config errors are the
// config reader's to report).
func configSwitchSettings(root string) capability.Settings {
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	if err != nil {
		return nil
	}
	reg := capability.Default()
	out := capability.Settings{}
	for _, kv := range configSwitchKeys() {
		key, id := kv[0], kv[1]
		if !config.RepoMaySet(key) {
			continue
		}
		if _, ok := reg.Get(id); !ok {
			continue
		}
		if _, set := loaded.Provenance[key]; !set {
			continue
		}
		hooks, guards := loaded.Config.Hooks, loaded.Config.Guards
		switch key {
		case "hooks.test_run":
			out[id] = hooks.TestRun
		case "hooks.pin_check":
			out[id] = hooks.PinCheck != config.PinCheckOff
		case "guards.enforce":
			out[id] = guards.Enforce == nil || *guards.Enforce
		}
	}
	return out
}
