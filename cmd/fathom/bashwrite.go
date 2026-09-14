package main

// bashwrite.go — O57 item 4/5(b): a Bash command that WRITES a file never
// matches Edit|Write|MultiEdit, so it never got obligations at all.
// bashWriteTargets parses tool_input.command for the write shapes the
// defect named — heredoc/plain redirection (`>`/`>>`), `sed -i`, `tee`,
// `cp`/`mv` into the tree, `git apply` — and reports EITHER confident
// written paths OR "doubt": something write-shaped happened but the exact
// target could not be extracted with confidence, and the caller falls
// back to serving the cwd's nearest guide rather than staying silent or
// guessing at a path.
//
// Deliberately not a shell parser: false negatives cost one fewer served
// rule (the same cost extractPromptCandidates accepts), and every
// confidently-extracted path is still just a CANDIDATE — it only ever
// produces output once the real index confirms something governs it.

import (
	"regexp"
	"strings"
)

// redirectRe matches a `>` or `>>` redirection target, consuming the one
// character before the operator so a digit (`2>`) or `&` (`&>`) — which
// redirect a STREAM, not a new file argument — can be excluded from the
// match. RE2 has no lookbehind, hence the consumed-prefix trick.
var redirectRe = regexp.MustCompile(`(?:^|[^0-9&])(>{1,2})\s*([^\s|;&<>]+)`)

// sedInPlaceRe finds `sed ... -i` (with or without a suffix like -i.bak,
// or BSD sed's quoted-empty extension-suffix form) anywhere in the
// command.
var sedInPlaceRe = regexp.MustCompile(`\bsed\b.*?\s-i(?:[\w.]*|\s+'')?\b`)

// gitApplyRe matches `git apply` / `git am` — both rewrite whatever the
// patch names, which is never recoverable from the command line alone.
var gitApplyRe = regexp.MustCompile(`\bgit\s+(?:apply|am)\b`)

// bashWriteDevNull are redirection targets that are never a repo file.
var bashWriteDevNull = map[string]bool{
	"/dev/null": true, "/dev/stderr": true, "/dev/stdout": true, "&1": true, "&2": true,
}

// maxBashWriteTargets bounds how many distinct targets one command
// resolves — a heredoc body that happens to contain its own `>` text
// must not turn one Bash call into an unbounded run of obligations RPCs
// (the same discipline maxPromptCandidates applies to prompt-hook).
const maxBashWriteTargets = 5

// bashWriteTargets classifies cmd. paths is nil unless at least one
// confident target was extracted; writeShaped is true whenever the
// command looks like it wrote SOMETHING, confident or not — the caller's
// three-way branch (silent / confident paths / doubt) is
// `!writeShaped` / `len(paths)>0` / `writeShaped && len(paths)==0`.
func bashWriteTargets(cmd string) (paths []string, writeShaped bool) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, false
	}
	seen := map[string]bool{}
	add := func(p string) {
		if len(paths) >= maxBashWriteTargets {
			return
		}
		p = strings.Trim(p, "\"'")
		if p == "" || seen[p] || bashWriteDevNull[p] || strings.HasPrefix(p, "$") || strings.HasPrefix(p, "`") {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	if gitApplyRe.MatchString(cmd) {
		return nil, true // the patch names its own targets; never guessable here
	}

	sawWriteShape := false

	for _, m := range redirectRe.FindAllStringSubmatch(cmd, -1) {
		target := m[2]
		if bashWriteDevNull[target] {
			continue // a stream sink, not a new file argument
		}
		sawWriteShape = true
		add(target)
	}

	if loc := sedInPlaceRe.FindStringIndex(cmd); loc != nil {
		sawWriteShape = true
		fields := shellClauseFields(cmd[loc[1]:])
		sawScript := false
		for _, f := range fields {
			if strings.HasPrefix(f, "-") {
				continue
			}
			if !sawScript {
				sawScript = true // the sed script/pattern itself, not a target
				continue
			}
			add(f)
		}
	}

	if idx := wordEnd(cmd, "tee"); idx >= 0 {
		sawWriteShape = true
		for _, f := range shellClauseFields(cmd[idx:]) {
			if !strings.HasPrefix(f, "-") {
				add(f)
			}
		}
	}

	for _, verb := range []string{"cp", "mv"} {
		if idx := wordEnd(cmd, verb); idx >= 0 {
			sawWriteShape = true
			var nonFlags []string
			for _, f := range shellClauseFields(cmd[idx:]) {
				if !strings.HasPrefix(f, "-") {
					nonFlags = append(nonFlags, f)
				}
			}
			if len(nonFlags) >= 2 {
				add(nonFlags[len(nonFlags)-1]) // the last argument is the destination
			}
		}
	}

	if len(paths) > 0 {
		return paths, true
	}
	return nil, sawWriteShape
}

// wordEnd returns the index just past the first standalone occurrence of
// word in cmd (a real token, not a substring of a longer identifier), or
// -1 if word never occurs as one.
func wordEnd(cmd, word string) int {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	loc := re.FindStringIndex(cmd)
	if loc == nil {
		return -1
	}
	return loc[1]
}

// shellClauseFields tokenizes s (minimally quote-aware) up to the next
// shell clause separator (`;`, `&&`, `||`, `|`, a bare `&`, or a newline)
// — so scanning a following verb's arguments never slurps the words of a
// chained command as if they were its own.
func shellClauseFields(s string) []string {
	var out []string
	var cur strings.Builder
	quote := byte(0)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		case c == ';' || c == '&' || c == '|':
			flush()
			return out
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}
