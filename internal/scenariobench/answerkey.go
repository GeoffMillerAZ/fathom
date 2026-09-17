// answerkey.go is O53 defect 3: tasks copied from
// testdata/contextbench/tasks-home.json ran in worktrees that also
// CONTAINED that file (and its siblings under testdata/scenariobench/ and
// docs/bench/), so the answer key a task's ground truth was copied from
// sat inside the very worktree the agent could Read/Grep/Bash — it
// surfaced in tool results in 41 scored cells of pilot vs-pilot-3. Three
// mechanisms close it: prepare-time removal (removeExcludedPaths,
// subject.go), a load-time requirement that any task derived from the
// home corpus declares the exclusion (corpus.go's validateScenarioTask),
// and this file's post-session scan, which catches the case both of those
// missed — a subject setup command that re-creates an excluded path, a
// symlink, or a corpus that forgot to name a path the removal needed.
//
// O61 (wave N10): removeExcludedPaths used to delete the excluded paths
// outright with os.RemoveAll — indistinguishable, to `git status`, from
// the agent deleting a tracked file itself. vs-pilot-4's first `git
// status` in a cell printed `deleted: docs/bench/README.md`, `deleted:
// testdata/contextbench/tasks-home.json`, … and the leak scanner below
// correctly read those literal names out of the tool_result and voided
// the cell — so the removal MECHANISM lost two of the first ten cells,
// not the tools under test. removeExcludedPaths now applies a git sparse
// checkout in non-cone mode instead: `git sparse-checkout set --no-cone
// '/*' '!<pattern>'…` (verified against a real git — subject_test.go)
// leaves the excluded paths simply absent from the working tree, with
// `git status` clean of them, no synthetic commit, and HEAD/LocalBase
// unchanged. This closes the WORKING-TREE leak; it does not and cannot
// close history mining — `git show HEAD:<excluded>` (or `git log -p`,
// `git cat-file`) still resolves the blob, because checkoutCommitIntoWorktree
// gives every cell worktree a real commit whose tree contains it. That
// mining command still gets caught below (its own text names the excluded
// path, scanned via toolUseInputText), but closing the vector itself would
// need a subject with no history at all for the excluded blobs to hide
// in — a corpus-level decision, not this fix's.
//
// O65 (wave N12): vs-pilot-5 (140 cells, 3 LOST) showed the RESULT-side
// half of this scan was too loose — "the needle appears anywhere in the
// tool_result text" caught two prose mentions, not the answer key itself
// (H1-T05/gopls/t1's Read of an ordinary source file whose own comment
// happened to name an excluded path as documentation; H1-T07/graphify/t2's
// `grep -rl "ADR-020"` listing a file that is not itself excluded, whose
// own prose elsewhere mentions one). resultTextNamesExcludedPath narrows
// the RESULT check to lines actually ROOTED at the excluded path — the
// shape every real grep/ls/find hit has and prose never does. The third
// LOST cell of that run (H1-T05/graphify/t1) was real: a `find` command
// walked OUTSIDE the worktree entirely and its result named the
// operator's own push clone — still a defect, but a sandbox escape, not
// an answer-key leak; SandboxEscapeScan (below) reports that distinctly.
package scenariobench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

// homeCorpusMarker is the exact path a task's derivation names when its
// prompt or ground truth was copied from the home lookup corpus.
const homeCorpusMarker = "testdata/contextbench/tasks-home.json"

// requiredHomeCorpusExcludes are the exclude_paths glob patterns
// validateScenarioTask requires of any task whose derivation names the
// home corpus: the home corpus itself, this corpus (a scenariobench task
// can quote another scenariobench task's own answer, e.g. lookups-vs.json
// quoting a sibling task's gold), and docs/bench/, which narrates both in
// prose.
var requiredHomeCorpusExcludes = []string{
	"testdata/contextbench/**",
	"testdata/scenariobench/**",
	"docs/bench/**",
}

// derivesFromHomeCorpus reports whether t's own Derivation names the home
// lookup corpus as a source for its prompt or ground truth — the
// contamination risk requiredHomeCorpusExcludes guards against.
func derivesFromHomeCorpus(t Task) bool {
	return strings.Contains(t.Derivation, homeCorpusMarker)
}

// excludePathsCoverHomeCorpus reports whether paths names every glob in
// requiredHomeCorpusExcludes, verbatim (the loader refuses a task that
// derives from the home corpus but declares anything less).
func excludePathsCoverHomeCorpus(paths []string) []string {
	have := make(map[string]bool, len(paths))
	for _, p := range paths {
		have[p] = true
	}
	var missing []string
	for _, want := range requiredHomeCorpusExcludes {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}

// fathomSelfHostMarkers are the repo_url spellings this repo's own
// guards-*.json/lookups-vs.json corpora use for their subject (both the
// SSH and HTTPS forms already committed, case-folded before comparison).
// wave N14 item 7 (guards-i28/i31, guards-pilot-6 second reading): a task
// whose subject is fathom's own repository carries this corpus's own
// scoring apparatus (testdata/scenariobench/*.json — a task's own rule,
// prediction, and detector text) and its runbook (docs/bench/) INSIDE the
// checked-out worktree an agent can Read/Grep/Bash into — the same
// answer-key contamination requiredHomeCorpusExcludes guards against for
// the home lookup corpus, from a different source. None of guards-
// fathom.json/guards-m1.json/guards-adr.json ever declared exclude_paths
// at all, which — since AnswerKeyLeakScan itself short-circuits on an
// empty list — disabled BOTH the prepare-time removal AND the post-
// session leak scanner for every task in them at once.
var fathomSelfHostMarkers = []string{"geoffmilleraz/fathom"}

// subjectIsFathomSelfHost reports whether t's subject repo is fathom's
// own, by fathomSelfHostMarkers.
func subjectIsFathomSelfHost(t Task) bool {
	repo := strings.ToLower(t.Subject.RepoURL)
	for _, m := range fathomSelfHostMarkers {
		if strings.Contains(repo, m) {
			return true
		}
	}
	return false
}

// requiredSelfHostExcludes are the exclude_paths glob patterns
// validateScenarioTask requires of any subjectIsFathomSelfHost task: this
// corpus's own scoring apparatus and the runbook narrating it in prose.
// Deliberately NOT testdata/contextbench/** — that one is
// requiredHomeCorpusExcludes' own concern (derivesFromHomeCorpus), a
// distinct condition a self-hosting task may or may not also meet.
var requiredSelfHostExcludes = []string{
	"testdata/scenariobench/**",
	"docs/bench/**",
}

// excludePathsCoverSelfHost mirrors excludePathsCoverHomeCorpus exactly,
// against requiredSelfHostExcludes instead.
func excludePathsCoverSelfHost(paths []string) []string {
	have := make(map[string]bool, len(paths))
	for _, p := range paths {
		have[p] = true
	}
	var missing []string
	for _, want := range requiredSelfHostExcludes {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}

// excludeNeedle turns one exclude_paths glob into the literal substring
// AnswerKeyLeakScan searches transcript text for: a "/**" suffix (the only
// shape requiredHomeCorpusExcludes or a hand-authored corpus is expected
// to use) becomes its directory prefix WITH the trailing slash, so
// "testdata/contextbench/**" catches any mention of a path under that
// directory without also matching an unrelated "testdata/contextbenchmark"
// sibling. A pattern with no "/**" suffix is used as-is — a plain file
// path glob names exactly the string that would leak.
func excludeNeedle(pattern string) string {
	if strings.HasSuffix(pattern, "/**") {
		return strings.TrimSuffix(pattern, "**")
	}
	return pattern
}

// AnswerKeyLeakScan scans session's raw transcript (session.RawPath) for
// any tool_use OR tool_result block naming one of excludePaths' literal
// directory prefixes (excludeNeedle) — proof that an excluded answer-key
// path reached the agent despite prepare-time removal. Both directions
// matter: a Read of an excluded file returns raw file content with no
// path anywhere in the RESULT (fix-first review, wave-n1 — the earlier
// version scanned tool_result text only, so a plain Read of the answer
// key escaped the scan entirely), so the tool_use's own INPUT is scanned
// too (toolUseInputText: Read/read_source's file path, Grep/Glob's own
// path/pattern/glob fields, a Bash command's own text). false whenever
// there is nothing to scan (no raw transcript, no excludePaths) or
// nothing was found — never a false positive from a pattern this
// function cannot confidently turn into a literal needle.
func AnswerKeyLeakScan(session SessionResult, excludePaths []string) bool {
	if session.RawPath == "" || len(excludePaths) == 0 {
		return false
	}
	needles := make([]string, 0, len(excludePaths))
	for _, p := range excludePaths {
		if n := excludeNeedle(p); n != "" {
			needles = append(needles, n)
		}
	}
	if len(needles) == 0 {
		return false
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return false
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamToolEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				text := toolUseInputText(c.Name, c.Input)
				for _, needle := range needles {
					if containsPathNeedle(text, needle) {
						return true
					}
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				text := rawTextBlock(c.Content)
				if resultTextNamesExcludedPath(text, needles) {
					return true
				}
			}
		}
	}
	return false
}

// resultTextNamesExcludedPath reports whether text (one tool_result's own
// content) contains a LINE that is itself rooted at one of needles —
// grep -n's "<path>:<line>:<content>", grep -l's bare "<path>", or an
// ls/find listing of the file (optionally "./"-prefixed, as `find` prints
// relative to its starting directory) — as opposed to the needle merely
// appearing somewhere inside a line of unrelated prose. O65 (wave N12)
// narrows AnswerKeyLeakScan's own RESULT-side check to this shape: see
// this file's own header for the two vs-pilot-5 cells ("(import.go;
// docs/bench/public-loc.md)" in an unrelated source comment;
// docs/plans/archive/falsifiability.md's own prose naming
// testdata/contextbench/tasks.json) that a bare substring search over the
// whole text wrongly caught. A real grep/ls/find hit on an excluded path
// always has the excluded directory as the LINE's own leading text; prose
// introduces the path with other words first. The tool_USE side
// (toolUseInputText, scanned via containsPathNeedle above) is deliberately
// NOT narrowed this way — an input field names a target directly, with no
// prose to hide inside.
func resultTextNamesExcludedPath(text string, needles []string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "./")
		for _, n := range needles {
			if strings.HasPrefix(trimmed, n) {
				return true
			}
		}
	}
	return false
}

// sandboxEscapeMarkers are literal substrings that identify a path as
// belonging to the OPERATOR's own scratch trees — a push clone, another
// wave's clone, or a benchroot evidence tree — never a cell's own
// worktree, whose paths live under
// workspace/artifacts/scenariobench/cells/. O65 (wave N12), item (c):
// H1-T05/graphify/t1 ran `find /private/tmp/claude-501 -maxdepth 8 -iname
// "*fathom*"` — a command that walked OUTSIDE the cell's own worktree
// entirely — and its result listed the operator's push clone
// (".../pushclone/testdata/scenariobench/guards-fathom.json").
// AnswerKeyLeakScan's own rooted-line rule (resultTextNamesExcludedPath)
// correctly does NOT flag that line as an answer-key leak: an absolute
// path with several more directory components before the excluded
// sub-path is not "rooted" at it. But it is still a real defect — a
// sandbox escape, distinct from an answer-key leak — and must still LOST
// the cell. Within this bench's own directory layout there is no OTHER
// tree a stray command could wander into from inside a cell's worktree —
// these three names are a complete proxy for "outside the worktree",
// without needing the worktree's own absolute path threaded through.
//
// wave N18 defect 1 (vs-pilot-6, 105 of 140 cells LOST): every run's bench
// ROOT itself is "<scratchpad>/benchroot2", and "/benchroot" is a
// substring of "/benchroot2" — so the cell's OWN worktree path
// (workspace/artifacts/scenariobench/worktrees/<cell>) and its subject-
// worktree path (workspace/scenariobench/subjects/<sha>-worktrees/<cell>),
// both of which live INSIDE the bench root, tripped this marker on the
// very first `cd`. SandboxEscapeScan now takes the cell's own safe
// directories and excludes any match found inside them — see its own doc
// comment below.
var sandboxEscapeMarkers = []string{"/pushclone/", "/wave-", "/benchroot"}

// sandboxTokenBoundary reports whether r can never appear inside an
// absolute-path token — whitespace, or the quoting/bracketing punctuation
// a shell command or a tool's own JSON text wraps a path in (including the
// ':' that separates a grep -n hit's own path from its line number).
func sandboxTokenBoundary(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '\'', '`', '(', ')', '[', ']', '{', '}', '<', '>', ':', ',', ';':
		return true
	}
	return false
}

// enclosingPathToken returns the maximal substring of text around byte
// offset i (a position inside a sandboxEscapeMarkers match) that contains
// no sandboxTokenBoundary rune — the whole path token spelled at that
// position, so textNamesSandboxEscape can reason about where it STARTS
// (absolute vs. relative — a relative path is never a sandbox escape) and
// what it is a PREFIX of (one of the cell's own safe directories), rather
// than the bare marker substring alone.
func enclosingPathToken(text string, i int) string {
	start := i
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if sandboxTokenBoundary(r) {
			break
		}
		start -= size
	}
	end := i
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if sandboxTokenBoundary(r) {
			break
		}
		end += size
	}
	// wave N19 defect 1: a trailing '.' is never boundary-scanned above (a
	// real path segment can legitimately contain one, e.g.
	// "app_test.go") but prose narrating a path immediately followed by
	// sentence-ending punctuation and no space — Serena's own
	// "<active-project>... at <path>.\n..." session-start message,
	// verbatim in the raw transcript on every serena-arm session — glues
	// that period onto the end of an otherwise-exact safe-dir token,
	// which then fails pathUnderAnyDir's prefix check by exactly one
	// byte. Trimming it here can only ever SHRINK the token (never change
	// where it starts, and the marker match itself always sits well
	// before any trailing punctuation), so it cannot turn a genuine
	// escape into a safe one — only stop a sentence's own full stop from
	// masquerading as part of the path.
	return strings.TrimRight(text[start:end], ".")
}

// pathUnderAnyDir reports whether path (cleaned) is exactly one of dirs or
// nested inside one of them, once each is filepath.Clean-ed too. Empty
// entries in dirs (a caller that does not have a raw dir, say) are simply
// skipped — never treated as "everything is safe".
func pathUnderAnyDir(path string, dirs []string) bool {
	clean := macTmpCanonical(filepath.Clean(path))
	for _, d := range dirs {
		if d == "" {
			continue
		}
		cd := macTmpCanonical(filepath.Clean(d))
		if clean == cd || strings.HasPrefix(clean, cd+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// macTmpCanonical strips the leading "/private" macOS adds when it
// resolves a symlinked top-level directory (/tmp, /var, /etc — the
// FreeBSD-derived layout every stock macOS ships) to its real location: a
// tool call that named a path as "/tmp/…" (the symlink spelling) and this
// driver's own "-root" (always given as the resolved "/private/tmp/…"
// spelling in this fleet's own scratch convention) name the IDENTICAL
// file, but pathUnderAnyDir's bare string-prefix comparison sees two
// different trees unless both sides are put in the same spelling first.
// A path that never had the prefix is returned unchanged.
func macTmpCanonical(path string) string {
	return strings.TrimPrefix(path, "/private")
}

// SandboxEscapeScan scans session's raw transcript for a tool_use INPUT or
// tool_result naming an absolute path that is a genuine sandbox escape:
// one of sandboxEscapeMarkers' own substrings, in a path token that (a)
// starts with "/" — a relative path is never an escape, however it
// happens to spell — and (b) is not itself inside any of safeDirs, the
// caller's own safe directories for this cell (CellLost's own naming: the
// cell's own worktree directory, its subject-worktree directory, and its
// raw transcript directory — pass whichever of these are known; an empty
// or omitted safeDirs behaves exactly as before this fix). false whenever
// there is nothing to scan (no raw transcript) or nothing was found.
func SandboxEscapeScan(session SessionResult, safeDirs ...string) bool {
	if session.RawPath == "" {
		return false
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return false
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamToolEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				if textNamesSandboxEscape(toolUseInputText(c.Name, c.Input), safeDirs) {
					return true
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				if textNamesSandboxEscape(rawTextBlock(c.Content), safeDirs) {
					return true
				}
			}
		}
	}
	return false
}

// ansiOSCRE matches one terminal OSC (Operating System Command) escape
// sequence — ESC ']' … terminated by BEL or ESC '\' (ST) — the shape a
// terminal hyperlink wraps a path in: `ESC]8;;file:///abs/path ESC\
// display-text ESC]8;;ESC\`, e.g. this environment's own `ls -la`
// rendering (the Bash tool's own colorized/hyperlinked listing, byte-
// identical whether the cd is inside the cell's own worktree or the
// shared subjects/ pool one level up from it). The href parameter
// carries the FULL absolute path; the visible display text alongside it
// is normally just a basename. Left unstripped, the href's own path
// characters (including its embedded ':' and ';') corrupt
// enclosingPathToken's boundary walk into a token that starts or ends
// mid-string — wave N19 defect 1's second shape.
var ansiOSCRE = regexp.MustCompile("\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)")

// ansiCSIRE matches one terminal CSI (Control Sequence Introducer) escape
// sequence — ESC '[' parameter/intermediate bytes, one final byte — the
// shape `ls --color`/`grep --color` wraps text in for SGR color codes
// (e.g. `\x1b[36m`).
var ansiCSIRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripTerminalEscapes removes terminal decoration (OSC hyperlinks, CSI
// color codes) from text before it is scanned for a sandbox-escape
// marker. It never removes a marker occurrence that exists anywhere in
// the PLAIN, unescaped text — a colorized `ls`/`eza --hyperlink`
// listing's own href always duplicates a path the tool's structured
// input (the Bash command argument, a Read's file_path) already carries
// unescaped, which SandboxEscapeScan scans separately — so stripping the
// decoration loses no detection power, it only stops the decoration
// itself from being misread as path content.
func stripTerminalEscapes(text string) string {
	text = ansiOSCRE.ReplaceAllString(text, "")
	text = ansiCSIRE.ReplaceAllString(text, "")
	return text
}

// textNamesSandboxEscape reports whether text names a genuine sandbox
// escape: see SandboxEscapeScan's own doc comment for the exact rule.
func textNamesSandboxEscape(text string, safeDirs []string) bool {
	text = stripTerminalEscapes(text)
	for _, m := range sandboxEscapeMarkers {
		start := 0
		for {
			i := strings.Index(text[start:], m)
			if i < 0 {
				break
			}
			pos := start + i
			token := enclosingPathToken(text, pos)
			if strings.HasPrefix(token, "/") && !pathUnderAnyDir(token, safeDirs) {
				return true
			}
			start = pos + len(m)
		}
	}
	return false
}

// toolUseInputText flattens one tool_use's own input into a single string
// AnswerKeyLeakScan can needle-search: Read/read_source's own file path
// (toolInputPath, stream.go — covers "file_path" and "file"), Grep/Glob's
// own "path"/"pattern"/"glob" fields (any of these can name or restrict to
// an excluded directory), and a Bash "command" field verbatim. Every
// field that is absent or not a string is simply "" and contributes
// nothing; unmarshaling into a struct rather than a map keeps this
// tolerant of whatever other fields a tool's own schema carries.
func toolUseInputText(name string, input json.RawMessage) string {
	var b strings.Builder
	if p := toolInputPath(name, input); p != "" {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	var fields struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Glob    string `json:"glob"`
		Command string `json:"command"`
	}
	if json.Unmarshal(input, &fields) == nil {
		for _, s := range []string{fields.Path, fields.Pattern, fields.Glob, fields.Command} {
			if s != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// containsPathNeedle reports whether needle (excludeNeedle's own
// directory-prefix shape, always ending in "/") appears in text at a
// position not immediately preceded by a path/identifier character — so
// "testdata/contextbench/" matches "cat testdata/contextbench/x.json" but
// not a differently-named directory that merely ends the same way
// ("mytestdata/contextbench/x.json"). needle's own trailing "/" is
// already the right-hand boundary — a directory needle is deliberately
// followed by more path text, unlike containsWholeToken's (goldsource.go)
// two-sided identifier check.
func containsPathNeedle(text, needle string) bool {
	if needle == "" {
		return false
	}
	start := 0
	for {
		i := strings.Index(text[start:], needle)
		if i < 0 {
			return false
		}
		i += start
		if i == 0 || !isPathIdentByte(text[i-1]) {
			return true
		}
		start = i + 1
	}
}

func isPathIdentByte(b byte) bool {
	return b == '_' || b == '-' || b == '.' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// removeExcludedPaths hides every path under root matching one of patterns
// (gitignore-style globs, internal/filter) from the git WORKING TREE — via
// a git sparse checkout in non-cone mode, never deletion (O61: deleting a
// tracked path is indistinguishable, to `git status`, from the agent doing
// it, and voided cells on that alone) — PrepareSubject's own call, run
// after checkout and before any ingest, for EVERY arm, so an answer-key
// file a corpus was derived from never sits inside the worktree an agent's
// Read/Grep/Bash tools can reach. root must already be a git repository
// (PrepareSubject's checkoutCommitIntoWorktree/reuse-reset guarantee this
// before removeExcludedPaths ever runs); a nil/empty patterns disables
// sparse-checkout instead of doing nothing — root's OWN worktree directory
// is reused across the tasks and trials that share a (base_commit, arm)
// (subject.go's own comment), so a task declaring no exclusions must still
// see the full tree even when an earlier cell in the same shared worktree
// left sparse-checkout active with a different exclusion set; `git
// sparse-checkout disable` is a verified no-op when nothing was ever
// excluded.
func removeExcludedPaths(ctx context.Context, root string, patterns []string) error {
	if len(patterns) == 0 {
		if out, err := runIn(ctx, root, "git", "sparse-checkout", "disable"); err != nil {
			return fmt.Errorf("removing excluded paths: disabling sparse-checkout: %w (%s)", err, out)
		}
		return nil
	}

	args := append([]string{"sparse-checkout", "set", "--no-cone"}, sparseCheckoutPatternsFor(patterns)...)
	if out, err := runIn(ctx, root, "git", args...); err != nil {
		return fmt.Errorf("removing excluded paths: git sparse-checkout set: %w (%s)", err, out)
	}

	// Belt-and-braces (the fix's own requirement, not a duplicate of the
	// post-session leak scan below): sparse-checkout's own pattern
	// semantics track internal/filter's — verified against a real git — but
	// a subject worktree must never simply trust that. A path still on disk
	// after this call fails the subject preparation loudly, as an
	// instrument error PrepareSubject's own caller surfaces immediately,
	// rather than as a LOST cell discovered only after a session ran.
	if found, err := findExcludedPathStillPresent(root, patterns); err != nil {
		return err
	} else if found != "" {
		return fmt.Errorf("removing excluded paths: %s matches an exclude_paths pattern but is still present after `git sparse-checkout set` — refusing rather than risk it reaching an agent", found)
	}
	return nil
}

// sparseCheckoutPatternsFor turns patterns into `git sparse-checkout set
// --no-cone` arguments: "/*" first — non-cone sparse-checkout patterns are
// a WHITELIST (a bare exclude negation with nothing else would check out
// NOTHING, not "everything except"), verified against a real git
// (subject_test.go) — then one "!"-negated pattern per exclude via
// excludeGlobToSparsePattern.
func sparseCheckoutPatternsFor(patterns []string) []string {
	out := make([]string, 0, len(patterns)+1)
	out = append(out, "/*")
	for _, p := range patterns {
		if sp := excludeGlobToSparsePattern(p); sp != "" {
			out = append(out, sp)
		}
	}
	return out
}

// excludeGlobToSparsePattern translates one exclude_paths glob (relative
// to the worktree root — types.go's own doc comment) into a negated
// sparse-checkout pattern, matching internal/filter's own anchoring rule
// (compile()): a pattern containing "/" anywhere is root-anchored, a bare
// name matches at any depth. "testdata/contextbench/**" becomes
// "!/testdata/contextbench/**" — verified against a real git to remove the
// directory itself, not just its contents, leaving git status clean and
// HEAD unmoved (subject_test.go). A leading "/" the caller already
// supplied is stripped first so this never doubles it. "" (never a valid
// sparse pattern) for a blank/whitespace-only input, which
// sparseCheckoutPatternsFor then simply omits.
func excludeGlobToSparsePattern(pattern string) string {
	p := strings.TrimSpace(pattern)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	if strings.Contains(p, "/") {
		return "!/" + p
	}
	return "!" + p
}

// findExcludedPathStillPresent is removeExcludedPaths' own belt-and-braces
// check: it walks root (never descending into ".git") and returns the
// first path (relative to root) that STILL matches one of patterns after
// the sparse checkout was applied — "" when nothing does. Read-only; unlike
// the deletion-era removeExcludedWalk this never removes anything, so it
// can also be used to prove a match exists without mutating the tree.
func findExcludedPathStillPresent(root string, patterns []string) (string, error) {
	m := filter.New(patterns)
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("removing excluded paths: reading %s: %w", root, err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		found, err := findExcludedWalk(root, e.Name(), m)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
	}
	return "", nil
}

// findExcludedWalk checks rel (a path relative to root) against m and
// returns it when matched; otherwise, for a directory, recurses into it so
// a pattern anchored to a nested path (rather than one of root's own
// top-level entries) is still found.
func findExcludedWalk(root, rel string, m *filter.Matcher) (string, error) {
	full := root + "/" + rel
	info, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("removing excluded paths: statting %s: %w", full, err)
	}
	target := rel
	if info.IsDir() {
		target += "/"
	}
	if m.Match(target) {
		return rel, nil
	}
	if !info.IsDir() {
		return "", nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", fmt.Errorf("removing excluded paths: reading %s: %w", full, err)
	}
	for _, e := range entries {
		found, err := findExcludedWalk(root, rel+"/"+e.Name(), m)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
	}
	return "", nil
}
