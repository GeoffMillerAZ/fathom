package main

// launcher.go — fathomd, the launchd agent's stable program (O1233).
//
// On macOS, Background Task Management tracks a launchd agent with a launch
// constraint on its program's hash. Every deploy re-signs and swaps the
// fathom binary, so every deploy changed that hash; past BTM's per-item
// notification limit launchd got no updated constraint and refused to start
// the daemon (exit 78). The agent's program is therefore fathomd: a copy of
// the fathom binary that `fathom daemon install` writes beside it only when
// it is absent or speaks another launcher protocol, and that no deploy ever
// rewrites. Its hash never changes, so BTM has nothing to re-evaluate.
//
// The launcher protocol is one rule: a process whose argv[0] base name is
// fathomd execs the sibling fathom in the same directory, before any other
// work, with the same arguments and environment. An old fathomd therefore
// keeps working with every newer fathom.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// launcherName is the launcher's file name; the binary it runs is its
// sibling daemonBinaryName.
const (
	launcherName     = "fathomd"
	daemonBinaryName = "fathom"
)

// launcherProtocolMarker versions the launcher rule. A build carries it as
// literal bytes, so `daemon install` can tell an installed fathomd's
// protocol without running it (running it would exec the sibling). Bump the
// digit only when the rule itself changes.
const launcherProtocolMarker = "fathom-launcher-protocol: 1"

// launcherExitConfig is EX_CONFIG: the launcher cannot run its binary.
const launcherExitConfig = 78

// launcherSys is what the launcher touches, injected so the rule is tested
// without replacing the test process.
type launcherSys struct {
	Stat       func(string) (fs.FileInfo, error)
	Exec       func(argv0 string, argv, envv []string) error
	Executable func() (string, error)
}

// runLauncher applies the launcher rule. handled is false for any argv[0]
// but fathomd, and nothing is touched. Otherwise it execs the sibling
// fathom; the real exec only returns on failure, which (like a missing
// sibling) exits launcherExitConfig with one stderr line.
func runLauncher(argv, env []string, sys launcherSys, stderr io.Writer) (code int, handled bool) {
	if len(argv) == 0 || filepath.Base(argv[0]) != launcherName {
		return 0, false
	}
	self := argv[0]
	if !strings.ContainsRune(self, filepath.Separator) {
		// Run by bare name from PATH: argv[0] carries no directory.
		exe, err := sys.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "fathomd: cannot resolve its own path to find the fathom binary beside it: %v\n", err)
			return launcherExitConfig, true
		}
		self = exe
	}
	sibling := filepath.Join(filepath.Dir(self), daemonBinaryName)
	if _, err := sys.Stat(sibling); err != nil {
		fmt.Fprintf(stderr, "fathomd: the fathom binary it runs is missing: %s\n", sibling)
		return launcherExitConfig, true
	}
	if err := sys.Exec(sibling, append([]string{sibling}, argv[1:]...), env); err != nil {
		fmt.Fprintf(stderr, "fathomd: cannot run %s: %v\n", sibling, err)
		return launcherExitConfig, true
	}
	return 0, true // only an injected exec returns on success
}

// launcherPathFor is the launcher beside bin.
func launcherPathFor(bin string) string {
	return filepath.Join(filepath.Dir(bin), launcherName)
}

// daemonBinaryFor is the binary a plist's ProgramArguments[0] runs: the
// launcher's sibling fathom, or program itself when it is not the launcher.
func daemonBinaryFor(program string) string {
	if filepath.Base(program) == launcherName {
		return filepath.Join(filepath.Dir(program), daemonBinaryName)
	}
	return program
}

// ensureLauncher writes the launcher beside bin, a byte copy of bin (its
// embedded code signature included, so it carries the deploy's signing
// identity), when the launcher is absent or speaks another protocol. A
// current launcher is never rewritten: its unchanged hash is the point.
// The copy goes to a temp file renamed into place, never over a file a
// process may be running.
func ensureLauncher(bin string) (launcher string, wrote bool, err error) {
	launcher = launcherPathFor(bin)
	marker := []byte(launcherProtocolMarker)
	if cur, readErr := os.ReadFile(launcher); readErr == nil && bytes.Contains(cur, marker) {
		return launcher, false, nil
	}
	src, err := os.ReadFile(bin)
	if err != nil {
		return launcher, false, fmt.Errorf("reading %s to copy it to the launcher: %w", bin, err)
	}
	if !bytes.Contains(src, marker) {
		return launcher, false, fmt.Errorf("%s predates the launcher (it does not exec a sibling when run as %s): install a current build first", bin, launcherName)
	}
	tmp := launcher + ".tmp"
	if err := os.WriteFile(tmp, src, 0o755); err != nil {
		return launcher, false, fmt.Errorf("writing the launcher: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return launcher, false, fmt.Errorf("writing the launcher: %w", err)
	}
	if err := os.Rename(tmp, launcher); err != nil {
		_ = os.Remove(tmp)
		return launcher, false, fmt.Errorf("writing the launcher: %w", err)
	}
	return launcher, true, nil
}

// launcherDriftLine is the posture's plain-words reading of a plist whose
// ProgramArguments[0] is the binary itself, or "" for a launcher plist.
func launcherDriftLine(post plistPostureReport) string {
	if !post.Direct || post.Program == "" {
		return ""
	}
	return fmt.Sprintf("ProgramArguments runs the binary itself (%s): each deploy changes the program macOS's Background Task Management tracks for this agent, and after enough deploys launchd stops starting the daemon (want %s, a launcher no deploy rewrites)",
		post.Program, launcherPathFor(post.Program))
}

// postureDrift is every drift line the posture reports: the tenancy keys,
// then the launcher.
func postureDrift(post plistPostureReport) []string {
	out := append([]string(nil), post.Drift...)
	if l := launcherDriftLine(post); l != "" {
		out = append(out, l)
	}
	return out
}

// launcherMigration rewrites a direct plist to run the launcher beside its
// binary, and moves a WorkingDirectory other than workDir to workDir:
// launchd enters it before the launcher runs, so it must be a directory the
// spawn already needs (the fathom home, which holds the logs), never the
// store's volume. The daemon reads no relative path from its working
// directory (the plist passes --store and --call-log absolute), so nothing
// needs a chdir after the exec. Every other byte is kept.
func launcherMigration(raw []byte, workDir string) ([]byte, []string, error) {
	p, err := parsePlist(raw)
	if err != nil {
		return nil, nil, err
	}
	args, ok := p.entry("ProgramArguments")
	if !ok || args.Kind != "array" || len(args.Children) == 0 || args.Children[0].Kind != "string" {
		return nil, nil, fmt.Errorf("ProgramArguments has no program to migrate")
	}
	prog := args.Children[0]
	if filepath.Base(prog.Text) == launcherName {
		return raw, nil, nil
	}
	launcher := launcherPathFor(prog.Text)
	edits := []plistEdit{{prog.Start, prog.End, plistStringXML(launcher)}}
	changes := []string{fmt.Sprintf("ProgramArguments[0]: %s → %s (a launcher no deploy rewrites)", prog.Text, launcher)}
	if wd, ok := p.entry("WorkingDirectory"); ok && wd.Kind == "string" && workDir != "" && filepath.Clean(wd.Text) != filepath.Clean(workDir) {
		edits = append(edits, plistEdit{wd.Start, wd.End, plistStringXML(workDir)})
		changes = append(changes, fmt.Sprintf("WorkingDirectory: %s → %s (launchd enters it before the launcher runs)", wd.Text, workDir))
	}
	// Apply from the end so earlier offsets stay valid.
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), raw...)
	for _, e := range edits {
		out = append(out[:e.start:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out, changes, nil
}

// plistStringXML is s as a plist <string> element.
func plistStringXML(s string) string {
	var b strings.Builder
	b.WriteString("<string>")
	_ = xml.EscapeText(&b, []byte(s))
	b.WriteString("</string>")
	return b.String()
}
