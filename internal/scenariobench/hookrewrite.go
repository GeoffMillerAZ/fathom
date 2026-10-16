package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// hookGuardSep joins the guard `fathom init` puts before every hook command
// (`command -v <bin> >/dev/null 2>&1 || exit 0; <bin> <sub>`: a silent no-op
// when the binary is absent) to the command it guards.
const hookGuardSep = " >/dev/null 2>&1 || exit 0; "

// unguardHook returns the command a guarded hook runs and whether cmd had
// the guard; an unguarded command comes back unchanged.
func unguardHook(cmd string) (string, bool) {
	rest, ok := strings.CutPrefix(cmd, "command -v ")
	if !ok {
		return cmd, false
	}
	if _, run, ok := strings.Cut(rest, hookGuardSep); ok {
		return run, true
	}
	return cmd, false
}

// rewriteBareFathomHooks rewrites every hook command in settingsPath whose
// first token (after the guard, which names the same binary) is exactly "fathom" (a PATH-resolved bare binary) to binPath
// followed by the same arguments, and reports how many it changed. Commands
// already absolute, or naming any other binary, are left byte-identical; a
// missing file is not an error (0 rewrites). The file is rewritten only
// when something changed. See subject.go (O103) for why this exists.
func rewriteBareFathomHooks(settingsPath, binPath string) (int, error) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return 0, fmt.Errorf("%s: parsing: %w", settingsPath, err)
	}
	hooksObj, _ := settings["hooks"].(map[string]any)
	n := 0
	for _, groups := range hooksObj {
		arr, _ := groups.([]any)
		for _, g := range arr {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			hs, _ := gm["hooks"].([]any)
			for _, h := range hs {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmd, _ := hm["command"].(string)
				run, guarded := unguardHook(cmd)
				if run == "fathom" || strings.HasPrefix(run, "fathom ") {
					run = binPath + strings.TrimPrefix(run, "fathom")
					if guarded {
						run = "command -v " + binPath + hookGuardSep + run
					}
					hm["command"] = run
					n++
				}
			}
		}
	}
	if n == 0 {
		return 0, nil
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return 0, err
	}
	return n, nil
}
