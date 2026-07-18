// Command release-notes prints one version's section of CHANGELOG.md
// verbatim, for pasting into a GitHub Release body or a release
// announcement.
//
// It parses CHANGELOG.md's "## [<version>] — <date>" headings (see
// CHANGELOG.md's own header for the convention; "## [Unreleased]" carries no
// date) and extracts everything from the matching heading line up to (not
// including) the next "## [" heading, or end of file if none follows.
//
// Usage:
//
//	go run ./cmd/release-notes [-root .] <version>
//
// <version> accepts both "v0.6.0" and bare "0.6.0" (a leading "v" is
// stripped before matching; CHANGELOG.md headings never carry one) as well
// as "Unreleased". -root defaults to "." and names the directory containing
// CHANGELOG.md (mirrors cmd/constitution-lint and cmd/kit-catalog's -root
// convention).
//
// Exit codes: 0 success, 1 CHANGELOG.md unreadable or version not found, 2
// usage error (bad flag or missing/extra positional argument).
//
// Concurrency: this is a single-shot, process-lifetime CLI — it is not safe
// for concurrent use, and carries no concurrency contract beyond "run it
// once and let it exit."
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractSection returns the verbatim slice of changelog running from the
// "## [" heading whose bracketed text matches version up to (not including)
// the next "## [" heading, or to end of file if no later heading exists.
// The returned string preserves the original bytes exactly, including the
// heading line itself.
//
// version accepts both "v0.6.0" and bare "0.6.0": a leading "v" is stripped
// before matching. The match is against the bracketed header text only
// (e.g. "0.6.0" or "Unreleased"), not the whole heading line — a heading may
// carry trailing " — <date>" text after the closing bracket.
//
// ExtractSection returns an error if no "## [" heading's bracketed text
// equals the normalized version.
func ExtractSection(changelog string, version string) (string, error) {
	normalized := strings.TrimPrefix(version, "v")

	type heading struct {
		start   int // byte offset, in changelog, of the start of this heading's line
		bracket string
	}

	var headings []heading
	offset := 0
	// SplitAfter keeps each line's trailing "\n" attached, so summing line
	// lengths reproduces exact byte offsets into the original string — the
	// slicing below never touches a reconstructed (and possibly
	// re-normalized) copy of the text.
	for _, line := range strings.SplitAfter(changelog, "\n") {
		if strings.HasPrefix(line, "## [") {
			rest := line[len("## ["):]
			if idx := strings.Index(rest, "]"); idx >= 0 {
				headings = append(headings, heading{start: offset, bracket: rest[:idx]})
			}
		}
		offset += len(line)
	}

	for i, h := range headings {
		if h.bracket != normalized {
			continue
		}
		end := len(changelog)
		if i+1 < len(headings) {
			end = headings[i+1].start
		}
		return changelog[h.start:end], nil
	}
	return "", fmt.Errorf("version %q not found in changelog", version)
}

// run is the testable entry point. args is os.Args[1:]; stdout/stderr are
// the CLI's output streams.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("release-notes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "directory containing CHANGELOG.md")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: release-notes [-root .] <version>")
		return 2
	}
	version := rest[0]

	data, err := os.ReadFile(filepath.Join(*root, "CHANGELOG.md"))
	if err != nil {
		fmt.Fprintf(stderr, "release-notes: %v\n", err)
		return 1
	}

	section, err := ExtractSection(string(data), version)
	if err != nil {
		fmt.Fprintf(stderr, "release-notes: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, section)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
