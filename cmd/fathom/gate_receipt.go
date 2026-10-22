package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/gatereceipt"
)

func cmdGate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: gate requires receipt or record-ids")
		return 1
	}
	switch args[0] {
	case "receipt":
		return cmdGateReceipt(args[1:], stdout, stderr)
	case "record-ids":
		return cmdGateRecordIDs(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown gate verb %q\n", args[0])
		return 1
	}
}

func cmdGateReceipt(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: gate receipt requires write or check")
		return 1
	}
	switch args[0] {
	case "write":
		return cmdGateReceiptWrite(args[1:], stdout, stderr)
	case "check":
		return cmdGateReceiptCheck(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown gate receipt verb %q\n", args[0])
		return 1
	}
}

func gateReceiptOptions(ctx context.Context, root, command string) (gatereceipt.Options, bool, error) {
	c := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	c.Dir = root
	b, err := c.Output()
	if err != nil {
		return gatereceipt.Options{}, false, fmt.Errorf("git repository root: %w", err)
	}
	root = strings.TrimSpace(string(b))
	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		return gatereceipt.Options{}, false, err
	}
	gate := loaded.Config.Gate
	dir := gate.ReceiptsDir
	if dir == "" {
		dir = fathomhome.Path("receipts")
		if dir == "" {
			return gatereceipt.Options{}, false, fmt.Errorf("cannot resolve fathom home")
		}
	} else if loaded.Provenance["gate.receipts_dir"].Source == config.SourceProject {
		dir, err = config.RepoPath(root, dir)
		if err != nil {
			return gatereceipt.Options{}, false, fmt.Errorf("gate.receipts_dir: %w", err)
		}
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return gatereceipt.Options{Root: root, Dir: dir, Command: command, HostLabel: gate.HostLabel}, gate.AcceptReceipts, nil
}

func cmdGateReceiptWrite(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("gate receipt write", stderr)
	log := fs.String("log", "", "finished gate log to copy beside the receipt (required)")
	command := fs.String("command", "validate", "gate task name")
	root := fs.String("root", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *log == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: receipt write requires --log and no positional arguments")
		return 1
	}
	ctx := context.Background()
	o, _, err := gateReceiptOptions(ctx, *root, *command)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	c := exec.CommandContext(ctx, "go", "version")
	// Reading toolchain identity must never trigger Go's automatic download.
	c.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	b, err := c.Output()
	if err != nil {
		fmt.Fprintln(stderr, "error: go version:", err)
		return 1
	}
	o.Log = *log
	o.GoVersion = strings.TrimSpace(string(b))
	o.FathomVersion = BuildRevision()
	o.OS = runtime.GOOS
	o.Arch = runtime.GOARCH
	_, id, err := gatereceipt.Write(ctx, o)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintln(stdout, id)
	return 0
}

func cmdGateReceiptCheck(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("gate receipt check", stderr)
	tree := fs.String("tree", "HEAD", "tree or commit whose gate evidence must match")
	command := fs.String("command", "validate", "gate task name")
	root := fs.String("root", ".", "repository directory")
	enabled := fs.Bool("if-enabled", false, "refuse unless [gate] accept_receipts is true (for pre-push)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: receipt check takes no positional arguments")
		return 1
	}
	ctx := context.Background()
	o, accept, err := gateReceiptOptions(ctx, *root, *command)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if *enabled && !accept {
		fmt.Fprintln(stderr, "receipt acceptance disabled ([gate] accept_receipts is false)")
		return 1
	}
	id, err := gatereceipt.Check(ctx, o, *tree)
	if err != nil {
		fmt.Fprintln(stderr, "receipt rejected:", err)
		return 1
	}
	fmt.Fprintln(stdout, id)
	return 0
}

type gateRunner struct{}

func (gateRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdGate(args, stdout, stderr)
}
func init() {
	declareVerb(verb{Name: "gate", Group: "review", Order: 100, Run: gateRunner{}, Summary: "Local landing evidence: receipt write | check; record-ids checks record-id uniqueness"})
}
