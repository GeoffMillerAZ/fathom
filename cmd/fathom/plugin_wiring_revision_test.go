package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoSettingsHasEveryHookSpecRow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct {
				Command string
				Timeout int
				Async   bool
			}
		}
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	for _, spec := range fathomHookSpecsAt("") {
		found := 0
		for _, group := range settings.Hooks[spec.event] {
			if group.Matcher != spec.matcher {
				continue
			}
			for _, hook := range group.Hooks {
				if strings.Contains(hook.Command, "cloud-session.sh") && spec.event == "SessionStart" && strings.HasSuffix(hook.Command, " fathom "+spec.sub) {
					found++ // The cloud setup wrapper owns its existing timeout.
				} else if hook.Command == hookCommand("fathom", spec.sub) && hook.Timeout == spec.timeout && hook.Async == spec.async {
					found++
				}
			}
		}
		if found != 1 {
			t.Errorf("%s/%s/%s has %d matching rows, want 1", spec.event, spec.matcher, spec.sub, found)
		}
	}
}
