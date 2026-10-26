package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// assetFS holds the host hook registrations and the agent definitions this
// binary ships, so print-hooks and doctor compare against exactly the bytes
// that were reviewed with the code.
//
//go:embed assets/hooks/*.json assets/agents/claude/*.md assets/agents/codex/*.toml
var assetFS embed.FS

// hookTemplate returns a host's hook registration with the binary path
// substituted.
func hookTemplate(host, bin string) (string, error) {
	raw, err := assetFS.ReadFile("assets/hooks/" + host + ".json")
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(raw), "{{BIN}}", jsonEscape(commandWord(bin))), nil
}

// portableHome is the prefix for a registration that resolves the binary
// under each machine's own home directory.
const portableHome = "$HOME/"

// commandWord renders the binary path as the first word of a hook command.
// "$HOME/rest" stays expandable (and safe for a home path with spaces) as
// "$HOME"/rest; anything else is quoted when it needs to be.
func commandWord(bin string) string {
	if rest, ok := strings.CutPrefix(bin, portableHome); ok && shellSafe.MatchString(rest) {
		return `"$HOME"/` + rest
	}
	return shellQuote(bin)
}

// agentAsset is one shipped agent definition and where its host loads it.
type agentAsset struct {
	Host, Name, Embedded string
	Content              []byte
}

// agentAssets lists the shipped agent definitions.
func agentAssets() []agentAsset {
	var out []agentAsset
	for _, spec := range []struct{ host, glob string }{
		{hostClaude, "assets/agents/claude/*.md"},
		{hostCodex, "assets/agents/codex/*.toml"},
	} {
		names, _ := fsGlob(spec.glob)
		for _, n := range names {
			content, err := assetFS.ReadFile(n)
			if err != nil {
				continue
			}
			out = append(out, agentAsset{Host: spec.host, Name: filepath.Base(n), Embedded: n, Content: content})
		}
	}
	return out
}

// agentInstallPath is where a host loads user-level agents.
func (a *app) agentInstallPath(ag agentAsset) string {
	if ag.Host == hostCodex {
		return filepath.Join(a.codexHome(), "agents", ag.Name)
	}
	return filepath.Join(a.claudeHome(), "agents", ag.Name)
}

// printHooks implements "print-hooks --host H [--bin PATH]".
func (a *app) printHooks(args []string) int {
	flags := flag.NewFlagSet("print-hooks", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	host := flags.String("host", "", "claude or codex")
	bin := flags.String("bin", "", "agent-hooks path: absolute, or $HOME/... to resolve per machine (default: this binary)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *host != hostClaude && *host != hostCodex {
		fmt.Fprintln(a.stderr, "agent-hooks print-hooks: --host must be claude or codex")
		return 2
	}
	path := *bin
	if path == "" {
		path = selfPath()
	}
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, portableHome) {
		fmt.Fprintln(a.stderr, "agent-hooks print-hooks: --bin must be an absolute path or start with $HOME/")
		return 2
	}
	out, err := hookTemplate(*host, path)
	if err != nil {
		fmt.Fprintln(a.stderr, "agent-hooks print-hooks:", err)
		return 1
	}
	fmt.Fprint(a.stdout, out)
	return 0
}

// selfPath is this binary's resolved absolute path, or "" when unknown.
func selfPath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func (a *app) claudeHome() string {
	if d := a.env("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(a.home, ".claude")
}

func (a *app) codexHome() string {
	if d := a.env("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(a.home, ".codex")
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./@%+=:,-]+$`)

// shellQuote quotes a path for a POSIX shell command line when it needs it.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// jsonEscape escapes a string for insertion between JSON quotes.
func jsonEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return r.Replace(s)
}

// fsGlob globs the embedded asset tree.
func fsGlob(pattern string) ([]string, error) { return fs.Glob(assetFS, pattern) }
