package main

// Change-note fragments: the fold (-roll, Unreleased) and the lint (-check).
//
// A repository adopts fragments by making the only line of its "## [Unreleased]"
// section a pointer: "Unreleased changes wait as fragments under <dir>/; ...".
// The pointer line is the per-repo configuration (no flag): a changelog without
// it gets exactly the verbatim-section behaviour of ExtractSection, and -roll
// and -check refuse or no-op there. <dir> is relative to -root.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// fragmentCategories is the fixed fold order, Breaking first.
var fragmentCategories = []string{"Breaking", "Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

var (
	pointerRe = regexp.MustCompile(`^Unreleased changes wait as fragments under ([^\s;]+)`)
	dateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// fragment is one parsed fragment file. entries are its "- " lines (with any
// indented continuation lines attached), in file order.
type fragment struct {
	name     string
	category string
	entries  []string
}

// fragmentsDir returns the fragments directory named by the pointer line in
// the [Unreleased] section, or "" when the changelog has no pointer.
func fragmentsDir(changelog string) string {
	sec, err := ExtractSection(changelog, "Unreleased")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(sec, "\n") {
		if m := pointerRe.FindStringSubmatch(line); m != nil {
			return strings.TrimRight(m[1], "/.")
		}
	}
	return ""
}

// parseFragment parses one fragment's text; the error names the refusal.
func parseFragment(name, text string) (fragment, error) {
	f := fragment{name: name}
	known := map[string]bool{}
	for _, c := range fragmentCategories {
		known[c] = true
	}
	headings := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case strings.HasPrefix(line, "### "):
			headings++
			if headings > 1 {
				return f, fmt.Errorf("%s: more than one heading", name)
			}
			f.category = strings.TrimPrefix(line, "### ")
			if !known[f.category] {
				return f, fmt.Errorf("%s: unknown heading %q", name, line)
			}
		case strings.HasPrefix(line, "- "):
			f.entries = append(f.entries, line)
		case line == "":
		case (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(f.entries) > 0:
			f.entries[len(f.entries)-1] += "\n" + line
		default:
			return f, fmt.Errorf("%s: text outside a heading or a %q line: %q", name, "- ", line)
		}
	}
	if headings == 0 {
		return f, fmt.Errorf("%s: no heading", name)
	}
	if len(f.entries) == 0 {
		return f, fmt.Errorf("%s: no %q line", name, "- ")
	}
	return f, nil
}

// readFragments parses every *.md file of dir in bytewise filename order.
func readFragments(dir string) ([]fragment, []error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	var out []fragment
	var errs []error
	for _, e := range ents { // ReadDir sorts by filename, bytewise
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		f, err := parseFragment(e.Name(), string(b))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, f)
	}
	return out, errs
}

// preOneZero reports whether version has major component 0 (or is unparsable
// as a number, which is treated as released-stable).
func preOneZero(version string) bool {
	v := strings.TrimPrefix(version, "v")
	major, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n == 0
}

// releaseImpact applies the kit's rule (changes.md section 4): Breaking is
// major, Added or Deprecated minor, otherwise patch; pre-1.0 states that
// breaking bumps minor on the same line.
func releaseImpact(frags []fragment, pre10 bool) string {
	impact := "patch"
	for _, f := range frags {
		switch f.category {
		case "Breaking":
			if pre10 {
				return "major (pre-1.0: breaking bumps minor)"
			}
			return "major"
		case "Added", "Deprecated":
			impact = "minor"
		}
	}
	return impact
}

// foldFragments renders the released-block text for frags: the heading, the
// computed Release impact line, then each present category in the fixed order
// with its lines in fragment-file order then line order. heading is the full
// heading line without a newline.
func foldFragments(frags []fragment, heading string, pre10 bool) string {
	var b strings.Builder
	b.WriteString(heading + "\n\nRelease impact: " + releaseImpact(frags, pre10) + "\n")
	for _, cat := range fragmentCategories {
		var lines []string
		for _, f := range frags { // frags is in filename order
			if f.category == cat {
				lines = append(lines, f.entries...)
			}
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString("\n### " + cat + "\n\n" + strings.Join(lines, "\n") + "\n")
	}
	return b.String()
}

// latestReleased returns the first released version in the changelog ("" if none).
func latestReleased(changelog string) string {
	for _, line := range strings.Split(changelog, "\n") {
		if rest, ok := strings.CutPrefix(line, "## ["); ok {
			if v, _, ok := strings.Cut(rest, "]"); ok && v != "Unreleased" {
				return v
			}
		}
	}
	return ""
}

// checkFragments lints the fragments and the changelog: one message per
// refusal, each naming the file. Empty means clean.
func checkFragments(changelog, dir string) []string {
	var msgs []string
	frags, errs := readFragments(dir)
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	if sec, err := ExtractSection(changelog, "Unreleased"); err == nil {
		for i, line := range strings.Split(sec, "\n") {
			if i == 0 || strings.TrimSpace(line) == "" || pointerRe.MatchString(line) {
				continue
			}
			msgs = append(msgs, fmt.Sprintf("CHANGELOG.md: [Unreleased] holds %q besides the pointer line", line))
		}
	}
	released := map[string]bool{}
	inUnreleased := false
	for _, line := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(line, "## [") {
			inUnreleased = strings.HasPrefix(line, "## [Unreleased]")
			continue
		}
		if !inUnreleased {
			released[strings.TrimSpace(line)] = true
		}
	}
	for _, f := range frags {
		for _, e := range f.entries {
			first, _, _ := strings.Cut(e, "\n")
			if released[strings.TrimSpace(first)] {
				msgs = append(msgs, fmt.Sprintf("%s: line already in a released block: %q", f.name, first))
			}
		}
	}
	return msgs
}

// rollChangelog returns changelog with block inserted under the Unreleased
// pointer, ahead of the first released heading.
func rollChangelog(changelog, block string) (string, error) {
	sec, err := ExtractSection(changelog, "Unreleased")
	if err != nil {
		return "", err
	}
	at := strings.Index(changelog, sec) + len(sec)
	head := strings.TrimRight(changelog[:at], "\n") + "\n\n"
	return head + block + "\n" + changelog[at:], nil
}
