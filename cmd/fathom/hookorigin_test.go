package main

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-146: the daemon marks a hook's request so a whole-tree read under it
// never waits for a slot; agents and the CLI are not hooks.
func TestMarkHookOrigin(t *testing.T) {
	for origin, want := range map[string]bool{
		"edit-hook": true, "session-hook": true, "stop-hook": true, "prompt-hook": true, "prime": true,
		"": false, "verify-cli": false, "doctor": false, "console": false, "init": false,
	} {
		if got := client.IsHookOrigin(markHookOrigin(context.Background(), origin)); got != want {
			t.Errorf("markHookOrigin(%q) hook = %v, want %v", origin, got, want)
		}
	}
}
