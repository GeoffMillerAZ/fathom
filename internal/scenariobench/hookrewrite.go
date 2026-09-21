package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// rewriteBareFathomHooks rewrites every hook command in settingsPath whose
// first token is exactly "fathom" (a PATH-resolved bare binary) to binPath
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
				if cmd == "fathom" || strings.HasPrefix(cmd, "fathom ") {
					hm["command"] = binPath + strings.TrimPrefix(cmd, "fathom")
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
