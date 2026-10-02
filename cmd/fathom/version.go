package main

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// Version is the current fathom CLI version.
const Version = "fathom v0.9.0"

// BuildRevision returns a one-line build identity: the version plus the VCS
// revision, commit time, and dirty flag that the Go toolchain embeds via
// -buildvcs (default on). It needs no ldflags. This is the staleness signal:
// compare it against `git rev-parse HEAD` to tell whether a long-running
// serve-mcp process is serving code older than the repo.
func BuildRevision() string {
	rev, when, dirty := vcsInfo()
	if rev == "" {
		return Version + " (revision unknown)"
	}
	short := rev
	if len(short) > 12 {
		short = short[:12]
	}
	tag := ""
	if dirty {
		tag = " (dirty)"
	}
	if when != "" {
		return fmt.Sprintf("%s @ %s%s [%s]", Version, short, tag, when)
	}
	return fmt.Sprintf("%s @ %s%s", Version, short, tag)
}

// vcsInfo extracts the embedded VCS settings from the build info.
func vcsInfo() (revision, time string, modified bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", false
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			time = s.Value
		case "vcs.modified":
			modified = strings.EqualFold(s.Value, "true")
		}
	}
	return revision, time, modified
}

// shortRevision returns the embedded VCS revision truncated to 12 chars (BuildRevision's
// own truncation length), or "" when no VCS info is embedded — used by
// `fathom research bundle` to identify the fathom BINARY, never the
// adopting repo's own commit.
func shortRevision() string {
	rev, _, _ := vcsInfo()
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev
}

// revisionDirty reports whether the embedded VCS state was modified
// (uncommitted changes) at build time.
func revisionDirty() bool {
	_, _, dirty := vcsInfo()
	return dirty
}

// cmdVersion prints the build identity; `--json` adds the responsible-tenant
// contract this binary speaks (tenancy_contract, design §8.3), which
// `daemon install --reconcile` reads from the INSTALLED binary before it
// writes a schema-2 plist.
func cmdVersion(args []string, stdout, _ io.Writer) int {
	if hasFlag(args, "json") {
		rev, when, dirty := vcsInfo()
		enc := json.NewEncoder(stdout)
		_ = enc.Encode(map[string]any{
			"version": Version, "revision": rev, "revision_time": when, "dirty": dirty,
			"tenancy_contract": tenancyContract,
		})
		return 0
	}
	fmt.Fprintln(stdout, BuildRevision())
	return 0
}

func cmdHelp(_ []string, stdout, _ io.Writer) int {
	printUsage(stdout)
	return 0
}

// versionRunner runs `fathom version` (registered below; EW-45).
type versionRunner struct{}

func (versionRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdVersion(args, stdout, stderr)
}

// helpRunner runs `fathom help` (registered below; EW-45).
type helpRunner struct{}

func (helpRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdHelp(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "version", Group: "meta", Order: 10, Run: versionRunner{},
		Summary: "Print version information"})
	declareVerb(verb{Name: "help", Group: "meta", Order: 20, Run: helpRunner{},
		Summary: "Show this help message"})
}
