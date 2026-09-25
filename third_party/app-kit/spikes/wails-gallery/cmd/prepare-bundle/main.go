// prepare-bundle assembles a private experiment. It never launches or installs it.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/macbundle"
)

type assembler func(string, macbundle.Inputs) (macbundle.Manifest, error)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, macbundle.Assemble))
}

func run(args []string, out, diagnostics io.Writer, assemble assembler) int {
	flags := flag.NewFlagSet("prepare-bundle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var destination string
	var input macbundle.Inputs
	flags.StringVar(&destination, "out", "", "new absolute private .app path; never overwrite")
	flags.StringVar(&input.Launcher.Path, "launcher", "", "absolute bundle-launcher executable")
	flags.StringVar(&input.Launcher.SHA256, "launcher-sha256", "", "reviewed launcher SHA256")
	flags.StringVar(&input.Server.Path, "server", "", "absolute cgo-free Gallery HTTP parent")
	flags.StringVar(&input.Server.SHA256, "server-sha256", "", "reviewed server SHA256")
	flags.StringVar(&input.Native.Path, "native", "", "absolute contained Wails native companion")
	flags.StringVar(&input.Native.SHA256, "native-sha256", "", "reviewed native SHA256")
	flags.StringVar(&input.KitRevision, "kit-revision", "", "declared source/candidate revision; not a release claim")
	flags.StringVar(&input.MinMacOS, "min-macos", "", "declared minimum macOS version; verify linked targets separately")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(out, "Prepare a private Gallery bundle without launching, installing or signing it. All flags below are required.")
		flags.SetOutput(out)
		flags.PrintDefaults()
		return 0
	} else if err != nil || flags.NArg() != 0 {
		fmt.Fprintln(diagnostics, "prepare-bundle: invalid arguments; use -h")
		return 2
	}
	for _, value := range []string{destination, input.Launcher.Path, input.Launcher.SHA256, input.Server.Path, input.Server.SHA256, input.Native.Path, input.Native.SHA256, input.KitRevision, input.MinMacOS} {
		if value == "" {
			fmt.Fprintln(diagnostics, "prepare-bundle: every input, reviewed hash and metadata flag is required; use -h")
			return 2
		}
	}
	manifest, err := assemble(destination, input)
	if err != nil {
		// JSON keeps a path or operating-system error from injecting terminal controls.
		_ = json.NewEncoder(diagnostics).Encode(struct {
			Error string `json:"error"`
		}{"prepare-bundle: " + err.Error()})
		return 1
	}
	if err := json.NewEncoder(out).Encode(manifest); err != nil {
		fmt.Fprintln(diagnostics, "prepare-bundle: bundle was prepared but its report could not be written; retain the output and do not retry over it")
		return 1
	}
	return 0
}
