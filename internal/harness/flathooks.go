package harness

// flathooks.go — GL-7/GL-8: the hook-file entries for the two FLAT shapes
// the json-hook editor does not address — Cursor's `.cursor/hooks.json`
// ("cursor-hooks") and Copilot's `.github/hooks/*.json` ("copilot-hooks").
// Both are {version:1, hooks:{<event>:[entry…]}} with one flat entry per
// binding, no nested "hooks" group. `fathom harness hooks --harness ID`
// prints them for the user to merge; fathom never writes either file.

import (
	"encoding/json"
)

// FlatHookShape returns the descriptor's flat hook-file shape and the
// project file it names ("cursor-hooks" / "copilot-hooks"), ok=false for a
// descriptor whose hooks are not flat.
func FlatHookShape(d Descriptor) (HookFile, bool) {
	for _, f := range d.HookFiles {
		if f.Scope == "project" && (f.Shape == "cursor-hooks" || f.Shape == "copilot-hooks") {
			return f, true
		}
	}
	return HookFile{}, false
}

// cursorHookEntry is one `.cursor/hooks.json` entry (timeout in seconds).
type cursorHookEntry struct {
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
	Timeout int    `json:"timeout"`
}

// copilotHookEntry is one `.github/hooks/*.json` entry. The matcher is the
// Claude-format one Copilot accepts beside its PascalCase events.
type copilotHookEntry struct {
	Type       string `json:"type"`
	Bash       string `json:"bash"`
	Matcher    string `json:"matcher,omitempty"`
	TimeoutSec int    `json:"timeoutSec"`
}

// RenderFlatHooks renders the descriptor's bindings as its flat hook file,
// every entry calling bin with the binding's verb; ok=false when the
// descriptor's hooks are not flat. Deterministic: encoding/json sorts the
// event keys, entries keep binding order.
func RenderFlatHooks(d Descriptor, bin string) ([]byte, bool, error) {
	f, ok := FlatHookShape(d)
	if !ok {
		return nil, false, nil
	}
	hooks := map[string][]any{}
	for _, b := range d.Hooks {
		cmd := bin + " " + b.Verb
		switch f.Shape {
		case "cursor-hooks":
			hooks[b.Harness] = append(hooks[b.Harness], cursorHookEntry{Command: cmd, Matcher: b.Matcher, Timeout: b.TimeoutSec})
		default:
			hooks[b.Harness] = append(hooks[b.Harness], copilotHookEntry{Type: "command", Bash: cmd, Matcher: b.Matcher, TimeoutSec: b.TimeoutSec})
		}
	}
	raw, err := json.MarshalIndent(map[string]any{"version": 1, "hooks": hooks}, "", "  ")
	return raw, true, err
}
