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
// The scan threads an effective working directory across top-level `;`/
// `&&`/`||`/`&`/newline-separated statements — `cd`, `pushd`, `popd` update
// it, everything else's relative write targets join onto it — so `cd
// internal/store && cat > y.go` still resolves `internal/store/y.go`, not
// `y.go` at the root. A `(...)` group is scanned as its own scope: cwd
// changes inside it (`(cd x && cat > y)`) never leak to what follows. A
// `cd`/`pushd` whose target cannot be read with confidence (no argument,
// `cd -`, a `$VAR` or command substitution) marks the cwd unknown from
// that point on — a write after it still counts as write-shaped (doubt),
// but names no target, rather than resolving against a wrong directory.
//
// Quoted text and heredoc bodies are data, never write shapes (EW-1, O122):
// bashWriteTargets blanks every heredoc body up front (blankHeredocBodies)
// and finds each write shape on a copy of the statement whose single- and
// double-quoted spans are masked (maskQuotedSpans), so `echo "a -> b"`
// has no redirect and `git commit -m 'cp x, mv y'` names no writer, while
// a quoted redirect TARGET (`> "docs/a b.md"`) is still read from the real
// text at the masked match's offsets.
//
// Deliberately not a shell parser: false negatives cost one fewer served
// rule (the same cost extractPromptCandidates accepts), and every
// confidently-extracted path is still just a CANDIDATE — it only ever
// produces output once the real index confirms something governs it.

import (
	"path"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
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
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
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
func bashWriteTargets(cmd string, roots ...string) (paths []string, writeShaped bool) {
	cmd = strings.TrimSpace(blankHeredocBodies(cmd))
	if cmd == "" {
		return nil, false
	}
	c := &bwCollector{seen: map[string]bool{}, roots: roots}
	st := &bwState{cwdKnown: true}
	scanBashWriteStatements(cmd, st, c)
	if len(c.paths) > 0 {
		return c.paths, true
	}
	return nil, c.writeShaped
}

// bwState is the effective-cwd state threaded across a sequence of
// top-level statements: cwd (bash-path-relative to wherever the whole
// command started — the same "already repo-relative text" ground truth a
// no-cd command's targets always represented), whether it is still known
// with confidence, and the pushd/popd stack.
type bwState struct {
	cwd      string
	cwdKnown bool
	stack    []bwStackEntry
}

type bwStackEntry struct {
	cwd   string
	known bool
}

// bwCollector accumulates confident targets (deduped, capped) and the
// write-shaped flag across however many statements and subshells a
// command has.
type bwCollector struct {
	roots       []string
	paths       []string
	seen        map[string]bool
	writeShaped bool
}

func (c *bwCollector) add(p string) {
	if len(c.paths) >= maxBashWriteTargets {
		return
	}
	if p == "" || c.seen[p] {
		return
	}
	c.seen[p] = true
	c.paths = append(c.paths, p)
}

// scanBashWriteStatements walks cmd's top-level statements in order,
// threading st's cwd through the plain ones and recursing into each
// subshell with a COPY of st — so a subshell's own cd(s) never leak into
// what follows it, matching real shell scoping.
func scanBashWriteStatements(cmd string, st *bwState, c *bwCollector) {
	for _, s := range splitTopLevelStatements(cmd) {
		if s.subshell {
			inner := *st
			inner.stack = append([]bwStackEntry(nil), st.stack...)
			scanBashWriteStatements(s.text, &inner, c)
			continue
		}
		processBashWriteStatement(s.text, st, c)
	}
}

// processBashWriteStatement handles one already-isolated statement: a
// cd/pushd/popd mutates st, anything else is scanned for the write shapes
// bashWriteTargets recognizes, each target resolved against st.cwd.
func processBashWriteStatement(stmt string, st *bwState, c *bwCollector) {
	parsed := parseShellCommand(stmt, c.roots...)
	if parsed.Refusal == shellcmd.RefusalWrapper {
		c.writeShaped = true // an unreadable declared script is doubt
		return
	}
	wrapped := false
	for _, p := range parsed.Pipelines {
		for _, u := range p.Units {
			if u.DeclaredScript {
				wrapped = true
				inner := *st
				inner.stack = append([]bwStackEntry(nil), st.stack...)
				if u.Cwd != "" {
					applyBashCd(&inner, u.Cwd, true)
				}
				scanBashWriteStatements(u.Script, &inner, c)
			}
		}
	}
	if wrapped {
		// Outer redirects and other pipeline commands act in the caller's cwd.
		detectBashWritesInStatement(stmt, st, c)
		return
	}
	fields, unreadable := lexClauseFields(stmt, c.roots...)
	if unreadable {
		// RO-4: the lexer cannot read this statement (the splitter hands
		// over fragments of compounds and of `2>&1`, so unreadable alone is
		// not a write). It is doubt where it matters: a cd/pushd/popd we
		// cannot read leaves the cwd unknown, and the write-shape detectors
		// below still mark doubt for any write shape they see, naming a
		// target only when they can read it with confidence.
		if f := strings.Fields(stmt); len(f) > 0 && (f[0] == "cd" || f[0] == "pushd" || f[0] == "popd") {
			st.cwdKnown = false
		}
		detectBashWritesInStatement(stmt, st, c)
		return
	}
	if len(fields) == 0 {
		return
	}
	switch fields[0] {
	case "cd":
		target, ok := bashCdTarget(fields[1:])
		applyBashCd(st, target, ok)
		return
	case "pushd":
		target, ok := bashCdTarget(fields[1:])
		st.stack = append(st.stack, bwStackEntry{cwd: st.cwd, known: st.cwdKnown})
		applyBashCd(st, target, ok)
		return
	case "popd":
		if n := len(st.stack); n > 0 {
			top := st.stack[n-1]
			st.stack = st.stack[:n-1]
			st.cwd, st.cwdKnown = top.cwd, top.known
		} else {
			st.cwdKnown = false // popd with an empty stack: bash itself errors: unknown from here
		}
		return
	}
	detectBashWritesInStatement(stmt, st, c)
}

// bashCdTarget picks the directory argument out of a cd/pushd's fields
// (already past the verb itself), skipping -L/-P-style flags. ok is false
// — "not resolvable here" — for a bare `cd -` (previous directory), a
// target naming a shell variable or command substitution, or no target
// at all (bash's own "go to $HOME" default, unknown in this context).
func bashCdTarget(args []string) (target string, ok bool) {
	for _, a := range args {
		if a == "-" {
			return "", false
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		if strings.ContainsAny(a, "$`") {
			return "", false
		}
		return strings.Trim(a, "\"'"), true
	}
	return "", false
}

// applyBashCd updates st.cwd on a resolvable target; on doubt it marks
// the cwd unknown from here on, WITHOUT discarding whatever confident
// value it held — a later cd that IS resolvable (an absolute path, most
// usefully) recovers it.
func applyBashCd(st *bwState, target string, ok bool) {
	if !ok {
		st.cwdKnown = false
		return
	}
	st.cwd = joinBashPath(st.cwd, target)
	st.cwdKnown = true
}

// joinBashPath resolves target against cwd using POSIX (forward-slash)
// join semantics — a shell command's own paths, regardless of the host
// OS fathom itself runs on.
func joinBashPath(cwd, target string) string {
	if path.IsAbs(target) || target == "~" || strings.HasPrefix(target, "~") {
		return path.Clean(target) // a tilde path is home-rooted, never relative to cwd: the caller expands or refuses it
	}
	return path.Join(cwd, target)
}

// detectBashWritesInStatement runs the write-shape detectors against one
// isolated statement and resolves each extracted target through st's
// cwd. When st.cwdKnown is false, a detected write still sets writeShaped
// (doubt) but names no target — better than resolving against a
// directory the parser is no longer sure of.
func detectBashWritesInStatement(stmt string, st *bwState, c *bwCollector) {
	resolve := func(p string) string {
		p = strings.Trim(p, "\"'")
		if p == "" || strings.HasPrefix(p, "$") || strings.HasPrefix(p, "`") {
			return ""
		}
		if !st.cwdKnown {
			return ""
		}
		return joinBashPath(st.cwd, p)
	}

	// Every write shape is FOUND on the masked copy (a quoted `>` or a
	// quoted writer name is text) and READ from the real text at the same
	// offsets (a quoted target, or a script with spaces, is still whole).
	mask := maskQuotedSpans(stmt)

	if gitApplyRe.MatchString(mask) {
		c.writeShaped = true // the patch names its own targets; never guessable here
		return
	}

	sawWrite := false

	for _, m := range redirectRe.FindAllStringSubmatchIndex(mask, -1) {
		target := stmt[m[4]:m[5]]
		if bashWriteDevNull[target] {
			continue // a stream sink, not a new file argument
		}
		sawWrite = true
		c.add(resolve(target))
	}

	if loc := sedInPlaceRe.FindStringIndex(mask); loc != nil {
		sawWrite = true
		fields, _ := lexClauseFields(stmt[loc[1]:]) // unreadable: sawWrite already marks doubt, no target named
		sawScript := false
		for _, f := range fields {
			if strings.HasPrefix(f, "-") {
				continue
			}
			if !sawScript {
				sawScript = true // the sed script/pattern itself, not a target
				continue
			}
			c.add(resolve(f))
		}
	}

	if idx := wordEnd(mask, "tee"); idx >= 0 {
		sawWrite = true
		tfields, _ := lexClauseFields(stmt[idx:]) // unreadable: sawWrite already marks doubt, no target named
		for _, f := range tfields {
			if !strings.HasPrefix(f, "-") {
				c.add(resolve(f))
			}
		}
	}

	for _, verb := range []string{"cp", "mv"} {
		if idx := wordEnd(mask, verb); idx >= 0 {
			sawWrite = true
			var nonFlags []string
			vfields, _ := lexClauseFields(stmt[idx:]) // unreadable: sawWrite already marks doubt, no target named
			for _, f := range vfields {
				if !strings.HasPrefix(f, "-") {
					nonFlags = append(nonFlags, f)
				}
			}
			if len(nonFlags) >= 2 {
				c.add(resolve(nonFlags[len(nonFlags)-1])) // the last argument is the destination
			}
		}
	}

	if sawWrite {
		c.writeShaped = true
	}
}

// maskQuotedSpans returns s with every byte inside a single- or double-
// quoted span replaced by '_' (the quote characters themselves stay, so a
// masked quoted argument is still one non-space token and an empty pair
// of quotes is still empty). The result has s's length, so an offset
// found in the mask indexes s directly. A backslash escapes the next byte
// outside quotes and inside double quotes, never inside single quotes; an
// unterminated quote masks to the end, as the shell itself would reject
// the line.
func maskQuotedSpans(s string) string {
	if !strings.ContainsAny(s, `'"`) {
		return s
	}
	b := []byte(s)
	quote := byte(0)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			} else {
				b[i] = '_'
			}
		case quote == '"':
			switch {
			case c == '\\' && i+1 < len(b):
				b[i], b[i+1] = '_', '_'
				i++
			case c == '"':
				quote = 0
			default:
				b[i] = '_'
			}
		case c == '\\':
			i++ // an escaped byte outside quotes is a literal, never a quote opener
		case c == '\'' || c == '"':
			quote = c
		}
	}
	return string(b)
}

// blankHeredocBodies returns cmd with every heredoc body (the lines after
// a `<<WORD`, `<<-WORD`, `<<'WORD'` or `<<"WORD"` line, through the line
// that is exactly WORD) overwritten with spaces, newlines kept: a body is
// data for the command reading it, so a `>` or a writer's name in it is
// never a write shape, and an apostrophe in its prose never opens a quote
// that swallows the next statement. `<<<` (a here-string) and a shift in
// `$((1<<3))` are not heredocs; a heredoc with no terminator line is left
// as written rather than blanking the rest of the command on a guess.
func blankHeredocBodies(cmd string) string {
	if !strings.Contains(cmd, "<<") {
		return cmd
	}
	type pending struct {
		word      string
		stripTabs bool
	}
	var queue []pending
	b := []byte(cmd)
	quote := byte(0)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
		case quote == '"':
			switch {
			case c == '\\' && i+1 < len(b):
				i++
			case c == '"':
				quote = 0
			}
		case c == '\\':
			i++
		case c == '\'' || c == '"':
			quote = c
		case c == '$' && i+2 < len(b) && b[i+1] == '(' && b[i+2] == '(':
			i = matchParenEnd(cmd, i+1) // arithmetic: its `<<` is a shift
		case c == '\n' && len(queue) > 0:
			pos := i + 1
			for _, q := range queue {
				next, ok := blankHeredocBody(b, pos, q.word, q.stripTabs)
				if !ok {
					break
				}
				pos = next
			}
			queue = nil
			i = pos - 1
		case c == '<' && i+1 < len(b) && b[i+1] == '<' && (i+2 >= len(b) || b[i+2] != '<') && (i == 0 || b[i-1] != '<'):
			if word, strip, end, ok := parseHeredocDelimiter(b, i+2); ok {
				queue = append(queue, pending{word, strip})
				i = end - 1
			}
		}
	}
	return string(b)
}

// parseHeredocDelimiter reads the delimiter after a `<<` at b[from:]: an
// optional `-`, optional blanks, then a quoted or bare word that starts
// with a letter or underscore (so `1<<3` is never one). end is the index
// just past the word.
func parseHeredocDelimiter(b []byte, from int) (word string, stripTabs bool, end int, ok bool) {
	i := from
	if i < len(b) && b[i] == '-' {
		stripTabs = true
		i++
	}
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	if i >= len(b) {
		return "", false, 0, false
	}
	switch q := b[i]; {
	case q == '\'' || q == '"':
		j := i + 1
		for j < len(b) && b[j] != q && b[j] != '\n' {
			j++
		}
		if j >= len(b) || b[j] != q || j == i+1 {
			return "", false, 0, false
		}
		return string(b[i+1 : j]), stripTabs, j + 1, true
	case q == '\\' || q == '_' || (q >= 'a' && q <= 'z') || (q >= 'A' && q <= 'Z'):
		if q == '\\' {
			i++
		}
		j := i
		for j < len(b) && !strings.ContainsRune(" \t\n;|&<>()", rune(b[j])) {
			j++
		}
		if j == i {
			return "", false, 0, false
		}
		return string(b[i:j]), stripTabs, j, true
	}
	return "", false, 0, false
}

// blankHeredocBody overwrites the body lines of one heredoc that starts at
// b[from], through its terminator line, with spaces (newlines stay), and
// returns the index just past the terminator line. ok is false — b
// untouched — when no line is exactly word (with `<<-`, leading tabs on a
// line are ignored).
func blankHeredocBody(b []byte, from int, word string, stripTabs bool) (next int, ok bool) {
	pos := from
	for pos <= len(b) {
		eol := pos
		for eol < len(b) && b[eol] != '\n' {
			eol++
		}
		line := strings.TrimRight(string(b[pos:eol]), "\r")
		if stripTabs {
			line = strings.TrimLeft(line, "\t")
		}
		if line == word {
			for k := from; k < eol; k++ {
				if b[k] != '\n' {
					b[k] = ' '
				}
			}
			if eol < len(b) {
				eol++ // past the terminator's newline
			}
			return eol, true
		}
		if eol >= len(b) {
			return from, false
		}
		pos = eol + 1
	}
	return from, false
}

// bwStatement is one top-level statement splitTopLevelStatements produced
// — either plain text, or (subshell) a `(...)` group's inner text with
// the wrapping parens stripped.
type bwStatement struct {
	text     string
	subshell bool
}

// splitTopLevelStatements splits cmd into an ordered sequence of
// statements at the top level: `;`, `&&`, `||`, a bare `&`, and newlines
// are separators. A `(` that opens with nothing else pending in the
// current statement starts a subshell group, kept whole (its inner text,
// unparsed further here) so the caller can scope cwd changes inside it. A
// `$(...)` command substitution is copied through opaquely — its own
// parens and separators must never be mistaken for the real ones. Quotes
// and paren nesting are respected throughout.
func splitTopLevelStatements(s string) []bwStatement {
	var out []bwStatement
	var cur strings.Builder
	quote := byte(0)
	flush := func() {
		t := strings.TrimSpace(cur.String())
		if t != "" {
			out = append(out, bwStatement{text: t})
		}
		cur.Reset()
	}
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
			i++
		case c == '\'' || c == '"':
			quote = c
			cur.WriteByte(c)
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			j := matchParenEnd(s, i+1)
			cur.WriteString(s[i:min(j+1, len(s))]) // j is len(s) for an unmatched `$(`
			i = j + 1
		case c == '(' && strings.TrimSpace(cur.String()) == "":
			j := matchParenEnd(s, i)
			out = append(out, bwStatement{text: s[i+1 : j], subshell: true})
			cur.Reset()
			i = j + 1
		case c == ';' || c == '\n':
			flush()
			i++
		case c == '&' && i+1 < len(s) && s[i+1] == '&':
			flush()
			i += 2
		case c == '|' && i+1 < len(s) && s[i+1] == '|':
			flush()
			i += 2
		case c == '&':
			flush()
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return out
}

// matchParenEnd returns the index of the ')' matching the '(' at s[open],
// respecting nesting and quotes. An unmatched open (malformed input)
// consumes to the end of the string rather than looping or panicking: it
// returns len(s), one past the last byte, so s[open+1:end] is the whole
// rest and a `(` that ends the command is an empty group, never a slice
// past its own end.
func matchParenEnd(s string, open int) int {
	depth := 0
	quote := byte(0)
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
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

// lexClauseFields reads s with the one command lexer (shellcmd.ParseCommand)
// and returns the argv of its first simple command: the words up to the next
// clause separator (`;`, `&&`, `||`, `|`, a bare `&`, or a newline), with
// quotes removed and redirects, env prefixes and wrappers kept out, so
// scanning a following verb's arguments never slurps the words of a chained
// command as if they were its own. unreadable is true when the lexer could
// not read s (RO-4); fields is then nil and each caller decides what that
// means for itself (the bash-write detector reads it as doubt).
func lexClauseFields(s string, roots ...string) (fields []string, unreadable bool) {
	cmd := parseShellCommand(s, roots...)
	if cmd.Unreadable {
		return nil, true
	}
	if len(cmd.Pipelines) == 0 || len(cmd.Pipelines[0].Units) == 0 {
		return nil, false
	}
	u := cmd.Pipelines[0].Units[0]
	if u.Kind != shellcmd.KindSimple {
		return nil, false
	}
	for _, w := range u.Words {
		if w.Text != "" { // an empty quoted word (sed's `-i ''`) is no field
			fields = append(fields, w.Text)
		}
	}
	return fields, false
}
