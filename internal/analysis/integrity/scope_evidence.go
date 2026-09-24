package integrity

// scope_evidence.go — the "compatibility-surface Evidence" half of CV-7's
// GuardReason (design §1.2). docs/intent/scope.md's own "## Compatibility
// surface" section is a bullet list, each item ending in a free-text
// "Evidence: …" clause that may name one or more Test* functions in
// backticks — a different shape from internal/intent.Corpus.Surfaces
// (which parses the ROUTER's "sensitive surface" TABLE, docs/intent/
// README.md, not scope.md's bullet list), so intent.Corpus does not
// already expose this as structured data. Rather than teach internal/
// intent a second, bullet-list surface shape for one caller, this package
// reads the section directly — the same "leaf package, caller resolves
// its own I/O" discipline every other flattened GuardedInput field
// follows.

import (
	"regexp"
	"sort"
	"strings"
)

// compatSurfaceHeadingRe matches the "## Compatibility surface" heading
// (case-insensitive, tolerant of the exact wording scope.md uses).
var compatSurfaceHeadingRe = regexp.MustCompile(`(?i)^##\s+Compatibility surface\s*$`)

// anyHeadingRe matches any "## " heading, used to find where the
// Compatibility surface section ENDS (the next heading at the same or a
// higher level).
var anyHeadingRe = regexp.MustCompile(`^#{1,6}\s+\S`)

// testNameInEvidenceRe pulls every Test\w+ token out of an "Evidence: …"
// clause — the exact `\bTest\w+\*?` shape internal/intent's own
// testNameRe uses (ParseAnnotation's tokenizer sibling, package-private
// there), kept in sync by convention rather than a shared symbol since
// importing internal/intent for one unexported regexp would cost this
// leaf package its independence for a two-line pattern.
var testNameInEvidenceRe = regexp.MustCompile(`\bTest\w+\b`)

// ExtractScopeEvidenceTests returns every Test* function named in an
// "Evidence: …" clause within docs/intent/scope.md's "## Compatibility
// surface" section (scopeMD is that file's raw content — any point in
// time the caller chooses, base or head). A file with no such section, or
// no Evidence clause naming a test, returns nil — never an error: an
// absent or test-free compatibility surface is an honest input, not a
// parse failure (only a caller's own missing-file handling, e.g. a
// conserve.Input.BaseFile returning ok=false, decides whether the
// SECTION'S ABSENCE itself is worth reporting elsewhere).
//
// A bullet routinely word-wraps across several physical lines (scope.md's
// own convention — a two-space-indented continuation, no leading "- "),
// and "Evidence:" may open on one physical line while the test name it
// names sits on the next; every bullet is joined into one logical line
// before Evidence clauses are searched, the same wrapped-line discipline
// internal/intent/parse.go's parseConditionsIn already applies to this
// exact document family.
//
// Sorted and deduplicated.
func ExtractScopeEvidenceTests(scopeMD []byte) []string {
	lines := strings.Split(string(scopeMD), "\n")
	inSection := false
	seen := map[string]bool{}
	var bullet strings.Builder
	flush := func() {
		if bullet.Len() == 0 {
			return
		}
		text := bullet.String()
		bullet.Reset()
		if !strings.Contains(text, "Evidence:") {
			return
		}
		evidence := text[strings.Index(text, "Evidence:"):]
		for _, m := range testNameInEvidenceRe.FindAllString(evidence, -1) {
			seen[m] = true
		}
	}
	for _, ln := range lines {
		t := strings.TrimRight(ln, "\r")
		trimmed := strings.TrimSpace(t)
		if compatSurfaceHeadingRe.MatchString(trimmed) {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if anyHeadingRe.MatchString(trimmed) {
			break // the next heading closes the section
		}
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "- "):
			flush()
			bullet.WriteString(trimmed)
		case bullet.Len() > 0:
			// A continuation line of the bullet in progress.
			bullet.WriteByte(' ')
			bullet.WriteString(trimmed)
		}
	}
	flush()
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
