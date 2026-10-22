package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/gatereceipt"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
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
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
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
	return gatereceipt.Options{Root: root, Dir: dir, Command: command, HostLabel: gate.HostLabel}, gate.AcceptReceipts && loaded.Provenance["gate.accept_receipts"].Source == config.SourceProject && gate.AllowReceiptAcceptance, nil
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
	o.Log = *log
	o.FathomVersion = BuildRevision()
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
	enabled := fs.Bool("if-enabled", false, "refuse silently unless project acceptance and user consent are enabled (for pre-push)")
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
		return 1
	}
	if err := gateReceiptLocalEnvironment(ctx, &o); err != nil {
		fmt.Fprintln(stderr, "receipt rejected:", err)
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

// The checker reads the local Go toolchain, never the binary's build platform.
func gateReceiptLocalEnvironment(ctx context.Context, o *gatereceipt.Options) error {
	runGo := func(args ...string) ([]byte, error) {
		c := exec.CommandContext(ctx, "go", args...)
		c.Dir = o.Root
		c.Env = append(os.Environ(), "GOTOOLCHAIN=local")
		b, err := c.Output()
		if err != nil {
			return nil, fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
		}
		return b, nil
	}
	b, err := runGo("version")
	if err != nil {
		return err
	}
	o.GoVersion = strings.TrimSpace(string(b))
	b, err = runGo("env", "-json", "GOOS", "GOARCH", "GOWORK")
	if err != nil {
		return err
	}
	var env struct{ GOOS, GOARCH, GOWORK string }
	if err := json.Unmarshal(b, &env); err != nil {
		return fmt.Errorf("go env: %w", err)
	}
	if env.GOOS == "" || env.GOARCH == "" || env.GOWORK != "off" {
		return fmt.Errorf("local go env requires GOOS, GOARCH and GOWORK=off")
	}
	o.OS, o.Arch = env.GOOS, env.GOARCH
	return nil
}

type gateRunner struct{}

func (gateRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdGate(args, stdout, stderr)
}
func init() {
	declareVerb(verb{Name: "gate", Group: "review", Order: 100, Run: gateRunner{}, Summary: "Local landing evidence: receipt write | check; record-ids checks record-id uniqueness"})
}
