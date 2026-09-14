package contextbench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The scratch-daemon-default safety checks (VB-36, O47). O47 caught two
// distinct escapes in the home bench's wired arm, both from the same root
// cause — a served path or an ad hoc shell command reaching outside the
// export the session was supposed to be confined to:
//
//   - a served tool_result payload naming the LIVE checkout's absolute
//     path (the daemon was pointed at the real repo, not an export of it,
//     so every answer effectively cited the wrong tree — see DetectPathLeak).
//   - a Bash call reading a file by an absolute path outside the session
//     root ('sed -n 1,120p /srv/acme/repo/CLAUDE.md' — `--restricted`
//     confines the Read/Glob/Grep tools to the working directory, but a
//     shell command naming an absolute path escapes it entirely — see
//     DetectOutOfRootReads).
//
// Both readings are pure functions over a `claude -p --output-format
// stream-json --verbose` transcript, exactly like TranscriptMetrics and
// TrustedWithoutRecheck — EXCEPT that both also canonicalize their two
// caller-supplied root paths via the filesystem (canonicalizePath,
// EvalSymlinks) so a symlinked temp root (macOS's /tmp -> /private/tmp)
// or a symlinked checkout compares equal; the transcript bytes themselves
// are never touched by I/O.
//
// A first cut of DetectPathLeak took only liveCheckout and flagged ANY
// occurrence of it in a served payload. That false-positived on every
// honest answer once the session root nested under the live checkout
// (liveCheckout + "/workspace/tmp/bench-subject-<sha>", the layout this
// package's own first VB-36 pass used): a served path INSIDE the
// legitimate export still contains the live checkout as a string prefix.
// DetectPathLeak now also takes sessionRoot and excludes a hit that falls
// under it — an honest reference to the export, not a leak — regardless
// of whether the caller's session root happens to nest under the live
// checkout (the old layout) or sits outside it entirely (the current
// one, scripts/context-bench.sh's `${TMPDIR:-/tmp}/fathom-context-bench/`).

// canonicalizePath resolves symlinks and cleans path so two spellings of
// the same directory compare equal — macOS's /tmp -> /private/tmp, or a
// symlinked checkout. Falls back to filepath.Clean when the path cannot
// be resolved (it does not exist — a test fixture's synthetic path, or a
// token pulled out of a transcript that is prose, not a real path on this
// machine): a root/path comparison must degrade to a plain string
// comparison, never panic or silently drop the check.
func canonicalizePath(p string) string {
	if p == "" {
		return ""
	}
	p = strings.TrimRight(p, "/")
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(p)
}

// isUnderRoot reports whether path falls under root — either by literal
// prefix (the common case, and the only one that matters for a path that
// does not exist on this machine, e.g. one pulled out of a test fixture)
// or, when that fails, by canonicalized prefix (EvalSymlinks on both
// sides), so a symlinked temp root or checkout is not a false miss. root
// == "" never contains anything.
func isUnderRoot(path, root string) bool {
	if root == "" {
		return false
	}
	root = strings.TrimRight(root, "/")
	if path == root || strings.HasPrefix(path, root+"/") {
		return true
	}
	cp, cr := canonicalizePath(path), canonicalizePath(root)
	if cr == "" {
		return false
	}
	return cp == cr || strings.HasPrefix(cp, cr+"/")
}

// PathLeakResult is DetectPathLeak's reading of one transcript.
type PathLeakResult struct {
	// Leaked is true when the live checkout's absolute path appeared in
	// any served tool_result payload, OUTSIDE the session's own root — a
	// fathom/grep answer built from the live tree instead of the export
	// the session ran against.
	Leaked bool `json:"leaked"`
	// Paths lists every distinct occurrence found, sorted.
	Paths []string `json:"paths,omitempty"`
}

// DetectPathLeak scans transcript's tool_result payloads (what a tool call
// SERVED to the model) for liveCheckout — the dev tree's own absolute
// path — and reports whether, and where, it appeared OUTSIDE sessionRoot.
// A hit whose path falls under sessionRoot (isUnderRoot) is an honest
// reference into the export, not a leak, REGARDLESS of whether sessionRoot
// happens to nest under liveCheckout (the layout this package's own first
// pass used) or sits outside it (the current one) — see the package
// comment. liveCheckout == "" always reports no leak: there is nothing to
// check a served path against, which is a different fact from "checked,
// none found".
func DetectPathLeak(transcript []byte, liveCheckout, sessionRoot string) PathLeakResult {
	var res PathLeakResult
	liveCheckout = strings.TrimRight(liveCheckout, "/")
	if liveCheckout == "" {
		return res
	}
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "user" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_result" {
				continue
			}
			body := rawText(c.Content)
			if !strings.Contains(body, liveCheckout) {
				continue
			}
			for _, hit := range tokensContaining(body, liveCheckout) {
				if isUnderRoot(hit, sessionRoot) {
					continue
				}
				if !seen[hit] {
					seen[hit] = true
					res.Paths = append(res.Paths, hit)
				}
			}
		}
	}
	if len(res.Paths) > 0 {
		res.Leaked = true
		sort.Strings(res.Paths)
	}
	return res
}

// tokensContaining returns every whitespace/quote/bracket-delimited token
// in body that contains needle, deduplicated in first-seen order — so a
// leak is reported as the actual path served (e.g.
// "/srv/acme/repo/CLAUDE.md"), not merely "found somewhere in a payload".
func tokensContaining(body, needle string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.FieldsFunc(body, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '\'', '"', '`', '(', ')', '[', ']', '{', '}', ',', ';', ':':
			return true
		}
		return false
	}) {
		if strings.Contains(tok, needle) && !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

// OutOfRootRead is one Bash/Read tool call whose input named an absolute
// path outside the session root.
type OutOfRootRead struct {
	Tool string `json:"tool"`
	Path string `json:"path"`
}

// shellWordRE splits a shell command string into candidate "words":
// maximal runs of characters that are neither whitespace nor a shell
// metacharacter that can glue two arguments together with no space
// between them (`;&|()<>`). This is a best-effort tokenizer, not a shell
// parser — good enough to isolate one argument from the rest of the
// command without letting a later fix (matching a bare "/" or a
// single-segment path) turn into matching a SUBSTRING of some longer,
// unrelated token — see absolutePathWordRE.
var shellWordRE = regexp.MustCompile(`[^\s;&|()<>]+`)

// absolutePathWordRE matches a whole WORD (already isolated by
// shellWordRE) that is itself an absolute path: "/" alone (the lone root
// — `ls /` must count), or "/" followed by one or more "/"-separated
// segments, each at least one character (never a zero-length segment,
// which is what would let the "//" inside a URL's "scheme://" match).
// Anchored at both ends, so it can only match a word that IS an absolute
// path, never a word merely containing one as a substring — a URL word
// ("https://example.com/foo") starts with "https:", not "/", and is
// rejected outright by the anchor, before the zero-length-segment rule
// would even matter.
var absolutePathWordRE = regexp.MustCompile(`^/(?:[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)*)?$`)

// absolutePathsInShellCommand extracts every whole-word absolute path in
// cmd (tilde-expanded first — see expandHomeDir), tokenizing on
// shellWordRE and matching each token in full against absolutePathWordRE.
// A single-segment path ("/tmp"), the lone root ("/"), and a multi-segment
// path ("/etc/hosts") all match; a URL ("https://host/path") does not,
// because the word itself does not start with "/".
func absolutePathsInShellCommand(cmd string) []string {
	cmd = expandHomeDir(cmd)
	var out []string
	for _, word := range shellWordRE.FindAllString(cmd, -1) {
		word = strings.Trim(word, `'"`)
		if absolutePathWordRE.MatchString(word) {
			out = append(out, word)
		}
	}
	return out
}

// tildeWordRE matches a `~/` that begins a shell word (at the start of
// the string, or right after whitespace) — never a bare `~` mid-word
// (e.g. a version-string tilde), and never `~user/`, which expandHomeDir
// does not attempt to resolve.
var tildeWordRE = regexp.MustCompile(`(^|\s)~/`)

// expandHomeDir replaces a leading `~/` on any shell word in cmd with the
// real, resolved home directory — so a read of `~/.claude/x` is recorded
// as the real path it names ($HOME/.claude/x), never the wrong, tilde-
// stripped `/.claude/x` absolutePathWordRE would otherwise match (the "/"
// right after a dropped "~" looks exactly like a root-relative path).
// Returns cmd unchanged when the home directory cannot be resolved.
func expandHomeDir(cmd string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return cmd
	}
	home = strings.TrimRight(home, "/")
	return tildeWordRE.ReplaceAllStringFunc(cmd, func(m string) string {
		return m[:len(m)-2] + home + "/"
	})
}

// DetectOutOfRootReads scans transcript's Bash/Read tool_use inputs for
// absolute paths outside sessionRoot — the O47 shape: `--restricted`
// confines the Read/Glob/Grep tools to the session root, but a Bash
// command naming an absolute path escapes it completely. sessionRoot ==
// "" reports nothing: there is no root to check a path against.
func DetectOutOfRootReads(transcript []byte, sessionRoot string) []OutOfRootRead {
	sessionRoot = strings.TrimRight(sessionRoot, "/")
	if sessionRoot == "" {
		return nil
	}
	var out []OutOfRootRead
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "assistant" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_use" {
				continue
			}
			var paths []string
			switch c.Name {
			case "Read":
				var in struct {
					FilePath string `json:"file_path"`
				}
				if json.Unmarshal(c.Input, &in) == nil && in.FilePath != "" {
					paths = []string{expandHomeDir(in.FilePath)}
				}
			case "Bash":
				var in struct {
					Command string `json:"command"`
				}
				if json.Unmarshal(c.Input, &in) == nil && in.Command != "" {
					paths = absolutePathsInShellCommand(in.Command)
				}
			default:
				continue
			}
			for _, p := range paths {
				if !filepath.IsAbs(p) {
					continue
				}
				if isUnderRoot(p, sessionRoot) {
					continue
				}
				key := c.Name + "\x00" + p
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, OutOfRootRead{Tool: c.Name, Path: p})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tool != out[j].Tool {
			return out[i].Tool < out[j].Tool
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// SumOutOfRootReads totals OutOfRootReads across every cell in results —
// what the ledger row's out_of_root_reads count reads (VB-36).
func SumOutOfRootReads(results []ArmResult) int {
	var n int
	for _, r := range results {
		n += r.OutOfRootReads
	}
	return n
}

// FlaggedCells lists, sorted, the task/arm/trial identifiers of every cell
// that carries a path_leak or an out_of_root_reads > 0 — surfaced on the
// ledger row so a leak or an escape is never silently averaged into a
// verdict (VB-36): "a cell with any is flagged, not silently scored".
func FlaggedCells(results []ArmResult) []string {
	var out []string
	for _, r := range results {
		if !r.PathLeak && r.OutOfRootReads == 0 {
			continue
		}
		trial := r.Trial
		if trial < 1 {
			trial = 1
		}
		out = append(out, fmt.Sprintf("%s/%s/t%d", r.TaskID, r.Arm, trial))
	}
	sort.Strings(out)
	return out
}
