package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// RT-169 hygiene: the production `fathom init` path (initWiringSpec →
// hookKeyFor → wiring.Key) registers the observe-only Read|Grep|Glob
// observer `"async": true` in the settings file it actually writes, and
// reconcile's desired value for that entry carries the flag too, so an
// async entry written by `plugin render` is not drift.
func TestInitWiringRegistersObserverAsync(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	spec, _ := initWiringSpec(root, "svc", "/opt/fathom")
	if _, err := applyWiringSpec(nil, root, spec, io.Discard); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Hooks map[string][]struct {
			Matcher string           `json:"matcher"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	asyncBy := map[string]bool{}
	for _, hs := range fathomHookSpecs {
		for _, g := range parsed.Hooks[hs.event] {
			if g.Matcher != hs.matcher {
				continue
			}
			for _, h := range g.Hooks {
				asyncBy[hs.event+"|"+hs.matcher], _ = h["async"].(bool)
			}
		}
	}
	sawAsync := false
	for _, hs := range fathomHookSpecs {
		got := asyncBy[hs.event+"|"+hs.matcher]
		if got != hs.async {
			t.Errorf("%s[%s]: settings.json async=%v, the registry says %v", hs.event, hs.matcher, got, hs.async)
		}
		if hs.async {
			sawAsync = true
		}
		if k := hookKeyFor(hs, "fathom "+hs.sub); k.Async != hs.async {
			t.Errorf("%s[%s]: reconcile's desired key async=%v, want %v", hs.event, hs.matcher, k.Async, hs.async)
		}
	}
	if !sawAsync {
		t.Fatal("no spec is async — the Read|Grep|Glob observer must be")
	}
}
