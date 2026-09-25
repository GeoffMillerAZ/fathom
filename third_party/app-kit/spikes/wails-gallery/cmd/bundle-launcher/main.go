// bundle-launcher is a private, native-free entrypoint, not a general process runner.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/macbundle"
)

type planner func(string) (macbundle.LaunchPlan, error)
type execute func(string, []string, []string) error

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Executable, macbundle.Plan, execParent, os.Environ()))
}

func run(args []string, out, diagnostics io.Writer, ownExecutable func() (string, error), plan planner, exec execute, environment []string) int {
	checkOnly := len(args) == 1 && args[0] == "-check"
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(out, "Private Gallery bundle launcher. No arguments: verify the bundle and exec its paired HTTP parent. -check: verify and print the plan without executing anything.")
		return 0
	}
	if len(args) != 0 && !checkOnly {
		fmt.Fprintln(diagnostics, "gallery-launcher: unexpected arguments; nothing was launched")
		return 2
	}
	path, err := ownExecutable()
	if err != nil {
		fmt.Fprintln(diagnostics, "gallery-launcher: could not locate its own executable; nothing was launched")
		return 1
	}
	selected, err := plan(path)
	if err != nil {
		_ = json.NewEncoder(diagnostics).Encode(struct {
			Error string `json:"error"`
		}{"gallery-launcher: bundle validation failed; nothing was launched: " + err.Error()})
		return 1
	}
	if !pairedPlan(path, selected) {
		fmt.Fprintln(diagnostics, "gallery-launcher: invalid parent/companion launch plan; nothing was launched")
		return 1
	}
	if checkOnly {
		if err := json.NewEncoder(out).Encode(struct {
			CheckedOnly bool                 `json:"checked_only"`
			Plan        macbundle.LaunchPlan `json:"plan"`
		}{true, selected}); err != nil {
			fmt.Fprintln(diagnostics, "gallery-launcher: could not write check report; nothing was launched")
			return 1
		}
		return 0
	}
	// Replace this process: the existing HTTP parent retains signal and child ownership.
	argv := append([]string{selected.Executable}, selected.Args...)
	if err := exec(selected.Executable, argv, environment); err != nil {
		fmt.Fprintln(diagnostics, "gallery-launcher: could not execute the verified HTTP parent; no fallback was attempted")
		return 1
	}
	return 0 // injected test seam only; successful exec never returns.
}

// Keep the process boundary closed even if the private planner later regresses.
// This is role/argument validation, not a replacement for Plan's byte checks.
func pairedPlan(own string, selected macbundle.LaunchPlan) bool {
	if !filepath.IsAbs(own) || filepath.Clean(own) != own || filepath.Base(own) != "gallery-launcher" || filepath.Base(filepath.Dir(own)) != "MacOS" {
		return false
	}
	contents := filepath.Dir(filepath.Dir(own))
	if filepath.Base(contents) != "Contents" || filepath.Ext(filepath.Dir(contents)) != ".app" {
		return false
	}
	server := filepath.Join(contents, "Resources", "gallery-server")
	native := filepath.Join(contents, "Resources", "gallery-native")
	if selected.Executable != server || len(selected.Args) != 4 || selected.Args[0] != "-native-host" || selected.Args[1] != native || selected.Args[2] != "-native-host-sha256" || len(selected.Args[3]) != 64 {
		return false
	}
	if selected.Args[3] != selected.Manifest.Files[2].SHA256 {
		return false
	}
	for _, c := range selected.Args[3] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
