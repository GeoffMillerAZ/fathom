package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// daemon.go — `fathom daemon install|status|uninstall`.
//
// The gap this closes (2026-09-01, found by running the adoption path as a
// new user on a clean machine): every user-facing "the daemon is down"
// message named ONE launchd label — `com.geoffmilleraz.fathom-http` — which
// exists on exactly one machine in the world. The SessionStart hook fires in
// every session of every adopter, and told all of them to run:
//
//	launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http
//
// which fails with "Could not find service" for anyone who is not its
// author. Installing the daemon at all was a documented copy-paste of a
// plist with paths to hand-edit — the single manual step in adoption, and
// the one most likely to be done wrong.
//
// So: fathom writes the plist itself, with the adopter's own paths, and
// every hint names the label ACTUALLY installed on this machine.

// defaultDaemonLabel is the label fathom installs under. Deliberately
// vendor-neutral: the pre-existing com.geoffmilleraz.* label stays valid
// because labelInstalled() discovers whatever is there rather than assuming.
const defaultDaemonLabel = "dev.fathom.serve-http"

// launchAgentsDir is ~/Library/LaunchAgents, or "" when HOME is unresolvable.
func launchAgentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents")
}

// labelInstalled returns the launchd label of an INSTALLED fathom serve-http
// agent on this machine, or "" if there is none. It reads the plists rather
// than assuming a label, which is the whole point: a hint that names someone
// else's service is worse than no hint, because it looks authoritative.
// With several installed, it is the alphabetically first (labelsInstalled).
func labelInstalled() string {
	if found := labelsInstalled(); len(found) > 0 {
		return found[0]
	}
	return ""
}

// labelsInstalled returns the launchd labels of every installed fathom
// serve-http agent on this machine, sorted.
func labelsInstalled() []string {
	dir := launchAgentsDir()
	if dir == "" || runtime.GOOS != "darwin" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}
		var doc struct {
			Keys []string `xml:"dict>key"`
			Strs []string `xml:"dict>string"`
		}
		if xml.Unmarshal(body, &doc) != nil {
			continue
		}
		text := string(body)
		if !strings.Contains(text, "serve-http") || !strings.Contains(text, "fathom") {
			continue
		}
		found = append(found, strings.TrimSuffix(e.Name(), ".plist"))
	}
	sort.Strings(found)
	return found
}

// daemonRestartHint is the ONE place that renders "how do I get the daemon
// back". Every caller (doctor, the session hook, init) uses it, so no
// adopter is ever handed another machine's service name.
func daemonRestartHint(storeHint string) string {
	if storeHint == "" {
		storeHint = "workspace/db/fathom.sqlite"
	}
	if label := labelInstalled(); label != "" {
		return fmt.Sprintf("launchctl kickstart -k gui/$(id -u)/%s", label)
	}
	if runtime.GOOS == "darwin" {
		return fmt.Sprintf("fathom daemon install   # writes the launchd agent and starts it (or run it in the foreground: fathom serve-http --store %s)", storeHint)
	}
	return fmt.Sprintf("fathom serve-http --store %s --addr :7575   # no launchd on this platform; run it under your own supervisor", storeHint)
}

func cmdDaemon(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom daemon install|status|uninstall")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "install":
		return cmdDaemonInstall(rest, stdout, stderr)
	case "status":
		return cmdDaemonStatus(rest, stdout, stderr)
	case "uninstall":
		return cmdDaemonUninstall(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown daemon subcommand %q (install|status|uninstall)\n", sub)
		return 2
	}
}

// plistBody renders the launchd agent. KeepAlive + RunAtLoad because the
// daemon is meant to be always-there; a fathom that needs starting by hand
// is a fathom the agent silently stops using.
func plistBody(label, bin, store, addr, logDir, callLog, pathEnv string, goEnv []string) string {
	var goEnvXML strings.Builder
	for _, kv := range goEnv {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || v == "" {
			continue
		}
		fmt.Fprintf(&goEnvXML, "\n    <key>%s</key><string>%s</string>", k, v)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>serve-http</string>
    <string>--store</string><string>%s</string>
    <string>--addr</string><string>%s</string>
    <string>--call-log</string><string>%s</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <!-- go must be on PATH: daemon-side auto-refresh shells out to "go list"
         for typed-edge enrichment on Go services. Without it refreshes fall
         back to syntax-only edges SILENTLY, which is the kind of quiet
         downgrade fathom exists to refuse. -->
    <key>PATH</key><string>%s</string>
    <!-- BP-37: PATH alone was not enough. launchd does not hand an agent a
         login shell's environment, so GOPATH arrived UNSET and go resolved
         its default to the literal, unexpanded "$HOME/go" — refusing every
         command with "GOPATH entry is relative". Every daemon-side ingest of
         a Go service fell back to syntax-only edges (2026-09-12). Absolute
         values, written at install time from this shell's own go env. -->%s
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s/fathom-http.log</string>
  <key>StandardErrorPath</key><string>%s/fathom-http.err</string>
</dict></plist>
`, label, bin, store, addr, callLog, pathEnv, goEnvXML.String(), logDir, logDir)
}

// daemonPathEnv builds the PATH launchd gives the daemon. launchd starts
// agents with a minimal PATH, so `go` must be named explicitly or typed-edge
// enrichment degrades without saying so. Resolved from THIS shell's go, then
// the usual install locations.
func daemonPathEnv() string {
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	if goBin, err := exec.LookPath("go"); err == nil {
		add(filepath.Dir(goBin))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, "go", "bin"))
		add(filepath.Join(home, ".local", "bin"))
	}
	for _, d := range []string{"/opt/homebrew/bin", "/usr/local/go/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"} {
		add(d)
	}
	return strings.Join(dirs, ":")
}

// daemonGoEnvKeys is the ordered set of Go toolchain variables the plist
// carries beside PATH (BP-37). Pinned as a list so a test can assert the
// plist declares exactly these, and so the status verb prints the same ones.
var daemonGoEnvKeys = []string{"GOPATH", "GOCACHE", "GOMODCACHE"}

// daemonGoEnv resolves absolute values for daemonGoEnvKeys from THIS
// shell's environment, falling back to `go env` and then to home-relative
// defaults — the same repair ladder the typed-edge loader itself walks
// (internal/fusion.TypedEdgeEnv), applied once at install time so the
// daemon never has to.
//
// Returns "KEY=VALUE" entries for every key that resolved, in
// daemonGoEnvKeys order. A key that resolves to nothing is omitted rather
// than written empty: an empty GOPATH in the plist is exactly the state
// this row exists to prevent.
func daemonGoEnv() []string {
	env := fusion.TypedEdgeEnv()
	resolved := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			resolved[k] = v // later entries win, exactly as exec does
		}
	}
	out := make([]string, 0, len(daemonGoEnvKeys))
	for _, k := range daemonGoEnvKeys {
		if v := resolved[k]; v != "" && filepath.IsAbs(v) {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func cmdDaemonInstall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("daemon install", stderr)
	label := fs.String("label", defaultDaemonLabel, "launchd label")
	store := fs.String("store", "", "store path (default: ~/.fathom/db/fathom.sqlite — the shared fleet store)")
	addr := fs.String("addr", ":7575", "listen address")
	binPath := fs.String("bin", "", "fathom binary to run (default: this executable, symlink-resolved)")
	force := fs.Bool("force", false, "overwrite an existing agent with the same label")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if runtime.GOOS != "darwin" {
		fmt.Fprintf(stderr, "fathom daemon install is launchd-only (this is %s).\n", runtime.GOOS)
		fmt.Fprintf(stderr, "Run it under your own supervisor instead:\n  fathom serve-http --store <path> --addr %s\n", *addr)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot resolve home directory: %v\n", err)
		return 1
	}

	// The binary launchd runs must be a STABLE path, not a build output that
	// `go build` will truncate under a running process (O25). Resolve
	// symlinks so the shim in ~/.local/bin does not become the answer.
	exe := *binPath
	if exe == "" {
		self, exeErr := os.Executable()
		if exeErr != nil {
			fmt.Fprintf(stderr, "error: cannot resolve this executable: %v\n", exeErr)
			return 1
		}
		if resolved, linkErr := filepath.EvalSymlinks(self); linkErr == nil {
			self = resolved
		}
		exe = self
	}
	if _, statErr := os.Stat(exe); statErr != nil {
		fmt.Fprintf(stderr, "error: binary %s is not usable: %v\n", exe, statErr)
		return 1
	}

	storePath := *store
	if storePath == "" {
		storePath = filepath.Join(home, ".fathom", "db", "fathom.sqlite")
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o700); err != nil {
		fmt.Fprintf(stderr, "error: cannot create store directory: %v\n", err)
		return 1
	}
	logDir := filepath.Join(home, ".fathom", "logs")
	callLog := filepath.Join(home, ".fathom", "logs", "mcp-calls.jsonl")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "error: cannot create log directory: %v\n", err)
		return 1
	}

	dir := launchAgentsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: cannot create %s: %v\n", dir, err)
		return 1
	}
	plistPath := filepath.Join(dir, *label+".plist")

	// Never silently replace a service someone else (or an earlier fathom)
	// installed — an overwrite here can point launchd at a different binary.
	if _, statErr := os.Stat(plistPath); statErr == nil && !*force {
		fmt.Fprintf(stderr, "fathom daemon install: %s already exists — re-run with --force to replace it, or `fathom daemon status` to see what it runs\n", plistPath)
		return 1
	}
	if existing := labelInstalled(); existing != "" && existing != *label && !*force {
		fmt.Fprintf(stderr, "fathom daemon install: a fathom agent is already installed as %q — installing a second one would leave two daemons fighting for %s.\n", existing, *addr)
		fmt.Fprintf(stderr, "  keep it:    launchctl kickstart -k gui/$(id -u)/%s\n  replace it: fathom daemon uninstall --label %s && fathom daemon install\n", existing, existing)
		return 1
	}

	pathEnv := daemonPathEnv()
	goEnv := daemonGoEnv()
	if err := os.WriteFile(plistPath, []byte(plistBody(*label, exe, storePath, *addr, logDir, callLog, pathEnv, goEnv)), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing %s: %v\n", plistPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s\n  binary   %s\n  store    %s\n  addr     %s\n  call log %s\n  PATH     %s\n", plistPath, exe, storePath, *addr, callLog, pathEnv)
	for _, kv := range goEnv {
		k, v, _ := strings.Cut(kv, "=")
		fmt.Fprintf(stdout, "  %-8s %s\n", k, v)
	}
	if len(goEnv) < len(daemonGoEnvKeys) {
		fmt.Fprintf(stdout, "  NOTE: only %d of %d Go toolchain variables (%s) resolved to absolute paths — typed-edge enrichment may fall back to syntax-only edges\n",
			len(goEnv), len(daemonGoEnvKeys), strings.Join(daemonGoEnvKeys, ", "))
	}
	if !strings.Contains(pathEnv, "go/bin") && !strings.Contains(pathEnv, "/usr/local/go") {
		fmt.Fprintln(stdout, "  NOTE: no Go toolchain found on PATH — typed-edge enrichment for Go services will fall back to syntax-only edges")
	}

	uid := os.Getuid()
	// bootout first so a re-install replaces rather than stacks; a missing
	// service makes bootout fail, which is fine and not worth reporting.
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, *label)).Run()
	out, bootErr := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath).CombinedOutput()
	if bootErr != nil {
		fmt.Fprintf(stderr, "fathom daemon install: wrote the agent but launchctl bootstrap failed: %v\n%s\n", bootErr, strings.TrimSpace(string(out)))
		fmt.Fprintf(stderr, "  start it by hand: launchctl bootstrap gui/%d %s\n", uid, plistPath)
		return 1
	}
	fmt.Fprintf(stdout, "bootstrapped %s\n", *label)
	fmt.Fprintf(stdout, "verify: fathom daemon status   ·   register the MCP server: docs/adoption/claude-code.md §2\n")
	return 0
}

func cmdDaemonStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("daemon status", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	jsonOut := fs.Bool("json", false, "emit label, plist, reachability and the remedy as JSON (docs/usage.md §daemon)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	label := labelInstalled()
	if *jsonOut { // RP-17: daemon_json.go
		st := daemonStatusJSON{Platform: runtime.GOOS, Label: label, URL: *url}
		if label != "" {
			st.Plist = filepath.Join(launchAgentsDir(), label+".plist")
		}
		st.Reachable, st.Detail = daemonHealthy(*url)
		if !st.Reachable {
			st.Remedy = daemonRestartHint("")
		}
		return renderDaemonStatusJSON(stdout, st)
	}
	switch {
	case label != "":
		fmt.Fprintf(stdout, "launchd agent: %s (%s)\n", label, filepath.Join(launchAgentsDir(), label+".plist"))
		// BP-37: what the daemon's Go environment actually is, read back
		// from the installed plist — not what this shell's environment
		// would be. A plist missing these is the 2026-09-12 defect still
		// present, and the reader must be able to see that from here.
		printPlistGoEnv(stdout, filepath.Join(launchAgentsDir(), label+".plist"))
	case runtime.GOOS == "darwin":
		fmt.Fprintln(stdout, "launchd agent: none installed — `fathom daemon install` writes one")
	default:
		fmt.Fprintf(stdout, "launchd agent: n/a on %s — run `fathom serve-http` under your own supervisor\n", runtime.GOOS)
	}

	// Reachability is the fact that matters; an installed agent that is not
	// serving is worse than none, because everything downstream assumes it.
	if healthy, detail := daemonHealthy(*url); healthy {
		fmt.Fprintf(stdout, "daemon:        reachable at %s (%s)\n", *url, detail)
		return 0
	} else {
		fmt.Fprintf(stdout, "daemon:        NOT reachable at %s (%s)\n", *url, detail)
		fmt.Fprintf(stdout, "remedy:        %s\n", daemonRestartHint(""))
		return 1
	}
}

func cmdDaemonUninstall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("daemon uninstall", stderr)
	label := fs.String("label", "", "launchd label (default: the installed fathom agent)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if runtime.GOOS != "darwin" {
		fmt.Fprintf(stderr, "fathom daemon uninstall is launchd-only (this is %s)\n", runtime.GOOS)
		return 2
	}
	target := *label
	if target == "" {
		target = labelInstalled()
	}
	if target == "" {
		fmt.Fprintln(stdout, "no fathom launchd agent installed — nothing to remove")
		return 0
	}
	plistPath := filepath.Join(launchAgentsDir(), target+".plist")
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), target)).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "error: removing %s: %v\n", plistPath, err)
		return 1
	}
	fmt.Fprintf(stdout, "removed %s (%s)\n", target, plistPath)
	fmt.Fprintln(stdout, "the store and logs under ~/.fathom are untouched")
	return 0
}

// printPlistGoEnv reports the Go toolchain variables the installed plist
// declares (BP-37). A key the plist does not carry is named as MISSING with
// the remedy: an absent GOPATH is what made every daemon-side ingest of a
// Go service fall back to syntax-only edges while reporting success.
func printPlistGoEnv(stdout io.Writer, plistPath string) {
	raw, err := os.ReadFile(plistPath)
	if err != nil {
		fmt.Fprintf(stdout, "go env:        unreadable plist (%v)\n", err)
		return
	}
	body := string(raw)
	var missing []string
	for _, k := range daemonGoEnvKeys {
		if strings.Contains(body, "<key>"+k+"</key>") {
			continue
		}
		missing = append(missing, k)
	}
	if len(missing) == 0 {
		fmt.Fprintf(stdout, "go env:        plist declares %s\n", strings.Join(daemonGoEnvKeys, ", "))
		return
	}
	fmt.Fprintf(stdout, "go env:        MISSING %s from the plist environment — typed-edge enrichment for Go services will fall back to syntax-only edges silently\n", strings.Join(missing, ", "))
	fmt.Fprintf(stdout, "remedy:        fathom daemon install --force\n")
}

// daemonHealthy probes /healthz and returns a short human detail either way.
// Short timeout: a status verb that hangs on a wedged daemon teaches people
// not to run it.
func daemonHealthy(baseURL string) (bool, string) {
	hc := &http.Client{Timeout: 2 * time.Second}
	resp, err := hc.Get(strings.TrimRight(baseURL, "/") + "/healthz")
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return true, strings.TrimSpace(string(body))
}
