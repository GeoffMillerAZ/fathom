package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"
)

const usage = `agent-hooks — token-economics hooks for Claude Code and Codex CLI

Usage:
  agent-hooks hook --host claude|codex     handle one hook event (stdin JSON)
  agent-hooks doctor [--host claude|codex] check policy, wiring, ledger and self-test
  agent-hooks policy check [FILE]          validate a policy file (default: the active one)
  agent-hooks policy default               print the built-in policy
  agent-hooks print-hooks --host H [--bin PATH]
                                           print the hook registration
  agent-hooks wire --host H [--bin PATH] [--dry-run]
                                           merge it into the host's hook file
                                           (idempotent, append-only, backed up)
  agent-hooks report [--days N]            summarize the ledger
  agent-hooks evaluate-spawn                evaluate one private spawn request (stdin JSON)
  agent-hooks version                      print the build version

Environment: AGENT_HOOKS_POLICY (policy file), AGENT_HOOKS_STATE (state dir).
Guide: docs/tokenomics.md in the agentic-kit repository.
`

// app carries every dependency on the outside world, so each command is
// testable with in-memory streams, a fake environment and a fixed clock.
type app struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	env            func(string) string
	now            func() time.Time
	home           string
}

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv, os.UserHomeDir))
}

// runMain leaves the pure evaluator outside home discovery. Its dependencies
// are explicit so startup behavior can be tested without changing process state.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, userHome func() (string, error)) int {
	home := ""
	if len(args) == 0 || args[0] != "evaluate-spawn" {
		home, _ = userHome()
	}
	a := &app{stdin: stdin, stdout: stdout, stderr: stderr, env: getenv, now: time.Now, home: home}
	return a.run(args)
}

// run dispatches a command line and returns the process exit code.
func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "hook":
		host, ok := a.hostFlag("hook", rest, true)
		if !ok {
			return 0 // a misconfigured hook command must not block the host
		}
		return a.hook(host)
	case "doctor":
		host, ok := a.hostFlag("doctor", rest, false)
		if !ok {
			return 2
		}
		return a.doctor(host)
	case "policy":
		return a.policyCmd(rest)
	case "print-hooks":
		return a.printHooks(rest)
	case "wire":
		return a.wire(rest)
	case "report":
		return a.report(rest)
	case "evaluate-spawn":
		return a.evaluateSpawn(rest)
	case "version":
		fmt.Fprintln(a.stdout, "agent-hooks", buildVersion())
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(a.stdout, usage)
		return 0
	}
	fmt.Fprintf(a.stderr, "agent-hooks: unknown command %q\n\n%s", cmd, usage)
	return 2
}

// hostFlag parses --host. When required, a missing or unknown host is an
// error.
func (a *app) hostFlag(name string, args []string, required bool) (string, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	host := fs.String("host", "", "claude or codex")
	if fs.Parse(args) != nil {
		return "", false
	}
	switch *host {
	case hostClaude, hostCodex:
		return *host, true
	case "":
		if !required {
			return "", true
		}
	}
	fmt.Fprintf(a.stderr, "agent-hooks %s: --host must be %q or %q\n", name, hostClaude, hostCodex)
	return "", false
}

// policyCmd implements "policy check [FILE]" and "policy default".
func (a *app) policyCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.stderr, usage)
		return 2
	}
	switch args[0] {
	case "default":
		_, _ = a.stdout.Write(defaultPolicyJSON)
		return 0
	case "check":
		path := a.policyPath()
		if len(args) > 1 {
			path = args[1]
		}
		raw, err := readCapped(path, policyMaxBytes)
		if err != nil {
			fmt.Fprintf(a.stderr, "policy %s: %v\n", path, err)
			return 1
		}
		if _, err := parsePolicy(raw); err != nil {
			fmt.Fprintf(a.stderr, "policy %s is invalid:\n%v\n", path, err)
			return 1
		}
		fmt.Fprintf(a.stdout, "policy %s is valid\n", path)
		return 0
	}
	fmt.Fprintf(a.stderr, "agent-hooks policy: unknown subcommand %q\n", args[0])
	return 2
}

// buildRevision is set at link time by installers that build from a source
// export without VCS metadata (go build -ldflags "-X main.buildRevision=…").
var buildRevision string

// buildVersion reports the stamped revision, or else the module version and
// VCS revision baked in by the Go toolchain, so no release step has to stamp
// the binary.
func buildVersion() string {
	if buildRevision != "" {
		return buildRevision
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	version, revision, dirty := info.Main.Version, "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	switch {
	case revision != "" && dirty:
		return version + " (" + revision + ", modified)"
	case revision != "":
		return version + " (" + revision + ")"
	}
	return version
}
