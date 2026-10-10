package main

// intent_playbook.go — HP-35: `fathom intent render --playbook`, which
// writes a root `INTENT.md` in the shape Anthropic's AI-Native SDLC
// playbook opens with (Problem / Proposed outcome / Affected users /
// Constraints / Open questions, then intent → spec → plan → diff → PR →
// incident).
//
// Two facts set the whole design of this verb:
//
//  1. INTENT.md is NOT a harness feature. No harness reads it, no hook
//     fires on it, nothing auto-loads it. It is an entry document for a
//     HUMAN arriving at the repo, and for an agent told to read it.
//  2. This repo already has the durable form of everything the playbook
//     asks for, under `docs/intent/`. A second corpus would be a second
//     thing to keep true, and the one lesson this repo has paid for
//     repeatedly is that a copy drifts silently.
//
// So INTENT.md is RENDERED, inside fathom markers, as POINTERS into the
// intent corpus plus the few lines that are genuinely one-screen summary.
// It carries no fact that `docs/intent/` does not already carry, which is
// exactly why it can be regenerated rather than maintained. `config audit`
// polices the pointers (the custodian's pointers corpus), so a pointer
// that rots is a finding rather than a quiet lie.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// playbookIntentFile is the rendered file's repo-relative path. Root, not
// docs/: the playbook's whole point is that it is the first thing you see.
const playbookIntentFile = "INTENT.md"

// The fathom marker pair. Same discipline as the stanza (init.go): fathom
// owns ONLY what sits between the markers, so a human may write anything
// above or below and a re-render will not touch it.
const (
	playbookIntentBegin = "<!-- fathom:intent-playbook begin -->"
	playbookIntentEnd   = "<!-- fathom:intent-playbook end -->"
)

// playbookMaxLines is the one-screen cap on the marked body (K13), markers
// included. The renderer truncates the question list to stay inside it; the
// golden test fails above it.
const playbookMaxLines = 50

// playbookCorpus is the fixed order the digest reads the corpus files in.
var playbookCorpus = []string{
	"docs/intent/README.md",
	"docs/intent/scope.md",
	"docs/intent/invariants.md",
	"docs/intent/questions.md",
}

// renderPlaybookIntent renders the marked body of INTENT.md for the repo
// at root: open goals, unanswered questions, the invariant count, how to
// change intent, and a footer carrying the corpus digest (sha256 over the
// bytes of the corpus files read, in a fixed order; never git HEAD, which
// would stale the file on every commit). Pure apart from reading the
// corpus: same corpus, same bytes. A source that is missing or unreadable
// is NAMED as UNAVAILABLE, never silently dropped (the ADR-027 lesson).
func renderPlaybookIntent(root string) string {
	h := sha256.New()
	read := func(rel string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", false
		}
		h.Write(b)
		return string(b), true
	}
	// Read in the digest's fixed order, once each.
	readme, okReadme := read(playbookCorpus[0])
	scope, okScope := read(playbookCorpus[1])
	invs, okInvs := read(playbookCorpus[2])
	qs, okQs := read(playbookCorpus[3])

	var head []string
	head = append(head, playbookIntentBegin, "# Intent", "",
		"*Generated from `docs/intent/` by `fathom intent render --playbook`: edit the corpus, not this file.*")
	if okReadme {
		if p := firstParagraphAfterHeading(readme); p != "" {
			head = append(head, "", p)
		}
	} else {
		head = append(head, "", unavailable(playbookCorpus[0]))
	}
	head = append(head, "", "Why it exists: `docs/design/product-vision.md`. Vocabulary: `docs/intent/domain.md`.", "")

	var tail []string
	tail = append(tail, "## Constraints", "")
	if okInvs {
		tail = append(tail, fmt.Sprintf("%d invariants must always hold and are binding: `docs/intent/invariants.md`.", len(invariantIDs(invs))))
	} else {
		tail = append(tail, unavailable(playbookCorpus[2]))
	}
	tail = append(tail, "Autonomy: `docs/intent/autonomy.md`. Decisions: `docs/intent/decisions/README.md`.", "",
		"## Changing intent", "",
		"Edit `docs/intent/{README,scope,invariants,questions}.md`, then run",
		"`fathom intent render --playbook` and commit INTENT.md in the same change.",
		"`fathom intent render --playbook --check` (the `intent-render-check` gate)",
		"exits 1 naming the first stale line.", "")

	// Questions are ids on one line (their titles live in questions.md); a
	// question whose text after the lead begins "Answered" is not open.
	qLines := []string{"## Unanswered questions", ""}
	if !okQs {
		qLines = append(qLines, unavailable(playbookCorpus[3]))
	} else {
		var ids []string
		for _, l := range boldLeads(qs) {
			if !strings.HasPrefix(l.Rest, "Answered") {
				ids = append(ids, l.ID)
			}
		}
		if len(ids) == 0 {
			qLines = append(qLines, "None open: `docs/intent/questions.md`.")
		} else {
			qLines = append(qLines, fmt.Sprintf("%d open, titles and blockers in `docs/intent/questions.md`: %s.", len(ids), strings.Join(ids, ", ")))
		}
	}
	qLines = append(qLines, "")

	// Goals take whatever the screen has left, minus the closing lines
	// (digest, end marker) and this section's heading, blank and pointer.
	goalLines := []string{"## Open goals", ""}
	if !okScope {
		goalLines = append(goalLines, unavailable(playbookCorpus[1]), "")
	} else {
		goals := boldLeadIDs(sectionOf(scope, "Current goals"))
		if len(goals) == 0 {
			goalLines = append(goalLines, "None recorded in `docs/intent/scope.md` §Current goals.")
		}
		budget := playbookMaxLines - len(head) - len(qLines) - len(tail) - 2 - len(goalLines) - 3
		shown := goals
		if len(shown) > budget && budget >= 1 {
			shown = append(append([]string{}, goals[:budget-1]...), fmt.Sprintf("...and %d more: `docs/intent/scope.md` §Current goals", len(goals)-(budget-1)))
		}
		for _, g := range shown {
			goalLines = append(goalLines, "- "+g)
		}
		goalLines = append(goalLines, "", "Each goal's `Done when`, and the non-goals: `docs/intent/scope.md`.", "")
	}

	var all []string
	all = append(all, head...)
	all = append(all, goalLines...)
	all = append(all, qLines...)
	all = append(all, tail...)
	all = append(all, "corpus sha256: "+hex.EncodeToString(h.Sum(nil)), playbookIntentEnd)
	return strings.Join(all, "\n") + "\n"
}

func unavailable(rel string) string {
	return fmt.Sprintf("`%s` could not be read — this section is UNAVAILABLE, not empty. Fix the file and re-render.", rel)
}

// firstParagraphAfterHeading returns the first non-heading, non-blank
// paragraph of a markdown body, unwrapped to one line.
func firstParagraphAfterHeading(body string) string {
	var para []string
	started := false
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if !started {
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			started = true
		}
		if t == "" {
			break
		}
		para = append(para, t)
	}
	return strings.Join(para, " ")
}

// sectionOf returns the body of the `## <heading>` section, up to the next
// heading of the same or a higher level.
func sectionOf(body, heading string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## "+heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// boldLeadStartRe matches the start of a `**ID. Title...` lead — the
// corpus's own shape for a goal and for a question. The title may wrap
// across lines, so the closing `**` is found by joining lines, not per line.
var boldLeadStartRe = regexp.MustCompile(`^\*\*([A-Z]+[0-9]+)\.\s+`)

// boldLead is one entry: its id, its title joined onto one line, and the
// text that follows the closing `**` (to the next blank line).
type boldLead struct{ ID, Title, Rest string }

func (l boldLead) String() string { return fmt.Sprintf("**%s** — %s", l.ID, l.Title) }

// boldLeads returns every bold-lead entry in body, in document order.
func boldLeads(body string) []boldLead {
	lines := strings.Split(body, "\n")
	var out []boldLead
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		m := boldLeadStartRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		// Join this entry's lines and find the close. An entry ends at a
		// blank line, after a hard break (a trailing backslash), or before a
		// line that starts the next entry: a split living doc's router lists
		// one routed record per line in ONE paragraph (K8, AG-36).
		j := i
		var para []string
		for ; j < len(lines); j++ {
			lt := strings.TrimSpace(lines[j])
			if lt == "" || (j > i && boldLeadStartRe.MatchString(lt)) {
				break
			}
			hard := strings.HasSuffix(lt, `\`)
			para = append(para, strings.TrimSuffix(lt, `\`))
			if hard {
				j++
				break
			}
		}
		text := strings.Join(para, " ")[len(m[0]):]
		end := strings.Index(text, "**")
		if end < 0 {
			continue
		}
		out = append(out, boldLead{
			ID:    m[1],
			Title: strings.TrimRight(strings.TrimSpace(text[:end]), "."),
			Rest:  strings.TrimSpace(text[end+2:]),
		})
		i = j - 1 // the loop's i++ lands on line j, the next unread line
	}
	return out
}

// boldLeadIDs returns "ID — Title" for every bold-lead entry in body, in
// document order.
func boldLeadIDs(body string) []string {
	var out []string
	for _, l := range boldLeads(body) {
		out = append(out, l.String())
	}
	return out
}

// invariantIDRe matches an invariant's id at the head of its row.
var invariantIDRe = regexp.MustCompile(`^\|?\s*\*{0,2}(I[0-9]+)\*{0,2}[.\s|]`)

// invariantIDs returns every invariant id in invariants.md, in order,
// deduplicated.
func invariantIDs(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		m := invariantIDRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// cmdIntentRender implements `fathom intent render`. --playbook is the
// only mode today and is required rather than defaulted: a verb that
// writes a root file on a bare invocation is a surprise.
func cmdIntentRender(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent render", stderr)
	root := fs.String("root", ".", "repository root")
	playbook := fs.Bool("playbook", false, "render the AI-native SDLC playbook's entry document to INTENT.md from docs/intent/ (HP-35)")
	dryRun := fs.Bool("dry-run", false, "print the rendered body to stdout instead of writing INTENT.md")
	check := fs.Bool("check", false, "render in memory and compare with INTENT.md's marked region: exit 1 naming the first differing line when stale or unmarked; never writes")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*playbook {
		fmt.Fprintln(stderr, "error: intent render needs a mode: --playbook (writes INTENT.md from docs/intent/)")
		return 1
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if _, err := os.Stat(filepath.Join(abs, "docs", "intent", "README.md")); err != nil {
		fmt.Fprintf(stderr, "error: %s has no docs/intent/README.md — there is no intent corpus to render from (bootstrap one with the intent-init skill)\n", abs)
		return 1
	}
	body := renderPlaybookIntent(abs)
	if *check {
		return checkPlaybookIntent(filepath.Join(abs, playbookIntentFile), body, stdout, stderr)
	}
	if *dryRun {
		fmt.Fprint(stdout, body)
		return 0
	}
	path := filepath.Join(abs, playbookIntentFile)
	merged, err := mergePlaybookIntent(path, body)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing %s: %v\n", path, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes between the fathom markers)\n", playbookIntentFile, len(body))
	return 0
}

// mergePlaybookIntent splices body into the existing file's marker region,
// preserving anything a human wrote outside it. A file with a corrupted
// marker pair (one marker, not both) is REFUSED rather than overwritten —
// the same three-case discipline the stanza writer uses, because guessing
// where a half-marked region ends is how hand-written prose gets eaten.
func mergePlaybookIntent(path, body string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return body, nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	old := string(data)
	begin := strings.Index(old, playbookIntentBegin)
	end := strings.Index(old, playbookIntentEnd)
	switch {
	case begin < 0 && end < 0:
		// Never wired: append, keeping whatever is already there.
		if strings.TrimSpace(old) == "" {
			return body, nil
		}
		return strings.TrimRight(old, "\n") + "\n\n" + body, nil
	case begin >= 0 && end > begin:
		// body ends with a newline and the old tail already starts with the
		// one that followed the end marker: keep exactly one, so a re-render
		// is idempotent instead of growing a blank line each time.
		tail := old[end+len(playbookIntentEnd):]
		if strings.HasPrefix(tail, "\n") {
			body = strings.TrimSuffix(body, "\n")
		}
		return old[:begin] + body + tail, nil
	default:
		return "", fmt.Errorf("%s has a corrupted fathom marker pair (begin=%t end=%t) — refusing to overwrite; repair or delete the markers by hand", path, begin >= 0, end >= 0)
	}
}

// checkPlaybookIntent compares body with the marked region of the file at
// path. Current: 0. Stale or markers missing or file unreadable: 1, naming
// the file and the first differing line. It never writes.
func checkPlaybookIntent(path, body string, stdout, stderr io.Writer) int {
	name := playbookIntentFile
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot read (%v); run `fathom intent render --playbook`\n", name, err)
		return 1
	}
	old := string(data)
	begin := strings.Index(old, playbookIntentBegin)
	end := strings.Index(old, playbookIntentEnd)
	if begin < 0 || end <= begin {
		fmt.Fprintf(stderr, "%s: fathom intent-playbook markers missing or corrupted; run `fathom intent render --playbook`\n", name)
		return 1
	}
	got := strings.Split(old[begin:end+len(playbookIntentEnd)]+"\n", "\n")
	want := strings.Split(body, "\n")
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			// Line numbers are relative to the marked region's first line.
			line := strings.Count(old[:begin], "\n") + i + 1
			fmt.Fprintf(stderr, "%s: stale at line %d\n  have: %s\n  want: %s\nrun `fathom intent render --playbook` and commit INTENT.md\n", name, line, g, w)
			return 1
		}
	}
	fmt.Fprintf(stdout, "%s: current\n", name)
	return 0
}
