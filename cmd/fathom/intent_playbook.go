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

// renderPlaybookIntent renders the marked body of INTENT.md for the repo
// at root. Pure apart from reading the corpus: same corpus, same bytes.
// The corpus files the body reads: docs/intent/{README,scope,invariants,
// questions}.md. A source that is missing or unreadable is NAMED in the
// output as UNAVAILABLE — never silently dropped, which is the ADR-027
// lesson (an unreadable file once made a whole audit report zero findings,
// and its most broken state was its healthiest-looking one).
func renderPlaybookIntent(root string) string {
	read := func(rel string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", false
		}
		return string(b), true
	}
	var sb strings.Builder
	sb.WriteString(playbookIntentBegin + "\n")
	sb.WriteString("# Intent\n\n")
	sb.WriteString("*Rendered by `fathom intent render --playbook` from `docs/intent/`. Everything\n" +
		"between the fathom markers is generated: edit the corpus, not this file. The\n" +
		"durable records are the corpus; this is its one-screen entry document, in the\n" +
		"shape the AI-native SDLC playbook opens with (intent → spec → plan → diff → PR\n" +
		"→ incident).*\n\n")

	// Problem ← the router's opening paragraph, plus the vision pointer.
	sb.WriteString("## Problem\n\n")
	if body, ok := read("docs/intent/README.md"); ok {
		if p := firstParagraphAfterHeading(body); p != "" {
			sb.WriteString(p + "\n\n")
		}
	} else {
		sb.WriteString(unavailable("docs/intent/README.md") + "\n\n")
	}
	sb.WriteString("Why it exists and what it should feel like: `docs/design/product-vision.md`.\n\n")

	// Proposed outcome ← scope.md's current goals, by id and title only.
	sb.WriteString("## Proposed outcome\n\n")
	if body, ok := read("docs/intent/scope.md"); ok {
		goals := boldLeadIDs(sectionOf(body, "Current goals"))
		if len(goals) == 0 {
			sb.WriteString("No current goals are recorded in `docs/intent/scope.md` §Current goals.\n\n")
		} else {
			for _, g := range goals {
				sb.WriteString("- " + g + "\n")
			}
			sb.WriteString("\nEach goal's `Done when` test, and the boundaries around them: `docs/intent/scope.md`.\n\n")
		}
	} else {
		sb.WriteString(unavailable("docs/intent/scope.md") + "\n\n")
	}

	// Affected users ← a pointer, deliberately. domain.md records the
	// vocabulary and the platform facts; who the users are is stated in
	// the vision, and restating it here would be the copy that drifts.
	sb.WriteString("## Affected users\n\n")
	sb.WriteString("Coding agents and the people running them, across one or many repos: the\n" +
		"surfaces are the Go client (`pkg/client`), the CLI (`cmd/fathom`) and the MCP\n" +
		"server. The domain vocabulary every one of them shares — what an observation,\n" +
		"an obligation, a layer and a claim ARE — is `docs/intent/domain.md`; who fathom\n" +
		"is for and what it should feel like is `docs/design/product-vision.md`.\n\n")

	// Constraints ← the invariant ids (never their prose: the prose is the
	// invariant and it lives in exactly one place) plus the non-goals.
	sb.WriteString("## Constraints\n\n")
	if body, ok := read("docs/intent/invariants.md"); ok {
		ids := invariantIDs(body)
		if len(ids) == 0 {
			sb.WriteString("No invariants are recorded in `docs/intent/invariants.md`.\n\n")
		} else {
			// The COUNT and the pointer, not the list: 38 ids on one screen
			// is noise, and an id without its statement tells nobody
			// anything. The count is the part that changes visibly when
			// someone adds or drops a constraint.
			fmt.Fprintf(&sb, "%d invariants must always hold. Each one's statement and its enforcement\nmechanism: `docs/intent/invariants.md`. An invariant is binding, not advisory;\n`fathom intent audit` reports the ones whose enforcement claim no longer holds.\n\n", len(ids))
		}
	} else {
		sb.WriteString(unavailable("docs/intent/invariants.md") + "\n\n")
	}
	sb.WriteString("What fathom deliberately does NOT do: `docs/intent/scope.md` §Non-goals.\n" +
		"What an agent may do unasked, must ask first, and never does:\n" +
		"`docs/intent/autonomy.md`. Decisions and their revisit triggers:\n" +
		"`docs/intent/decisions/README.md` (records are the `## ADR-NNN` sections of `ADR.md`).\n\n")

	// Open questions ← ids and one-line titles; the detail stays in one place.
	sb.WriteString("## Open questions\n\n")
	if body, ok := read("docs/intent/questions.md"); ok {
		qs := boldLeadIDs(body)
		if len(qs) == 0 {
			sb.WriteString("None open — `docs/intent/questions.md` is empty.\n\n")
		} else {
			for _, q := range qs {
				sb.WriteString("- " + q + "\n")
			}
			sb.WriteString("\nWhat each blocks and who can answer it: `docs/intent/questions.md`.\n\n")
		}
	} else {
		sb.WriteString(unavailable("docs/intent/questions.md") + "\n\n")
	}

	sb.WriteString("## The rest of the loop\n\n")
	sb.WriteString("- **spec / plan** — `docs/plans/*.md`, gated: a box flips to `[x]` only when\n" +
		"  `fathom verify --service <svc> --plan <plan>` proves it (`docs/plans/README.md`).\n" +
		"- **diff / PR** — a conventional commit with a `Verified:` line; observable\n" +
		"  behavior also gets a `CHANGELOG.md` line.\n" +
		"- **incident** — `docs/intent/incidents.md`: what happened, the evidence, and\n" +
		"  what would have caught it, driven to an invariant, a test or a decision.\n\n")
	sb.WriteString("Re-render this file with `fathom intent render --playbook`.\n")
	sb.WriteString(playbookIntentEnd + "\n")
	return sb.String()
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

// boldLeadRe matches a `**ID. Title.**` or `**ID. Title**` lead — the
// corpus's own shape for a goal and for a question.
var boldLeadRe = regexp.MustCompile(`^\*\*([A-Z]+[0-9]+)\.\s+(.+?)\*\*`)

// boldLeadIDs returns "ID — Title" for every bold-lead entry in body, in
// document order.
func boldLeadIDs(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		m := boldLeadRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		out = append(out, fmt.Sprintf("**%s** — %s", m[1], strings.TrimRight(m[2], ".")))
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
		return old[:begin] + body + old[end+len(playbookIntentEnd):], nil
	default:
		return "", fmt.Errorf("%s has a corrupted fathom marker pair (begin=%t end=%t) — refusing to overwrite; repair or delete the markers by hand", path, begin >= 0, end >= 0)
	}
}
