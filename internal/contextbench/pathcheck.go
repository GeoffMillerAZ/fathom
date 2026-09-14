package contextbench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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
//     root ('sed -n 1,120p /Volumes/m2ext/.../CLAUDE.md' — `--restricted`
//     confines the Read/Glob/Grep tools to the working directory, but a
//     shell command naming an absolute path escapes it entirely — see
//     DetectOutOfRootReads).
//
// Both readings are pure functions over a `claude -p --output-format
// stream-json --verbose` transcript, exactly like TranscriptMetrics and
// TrustedWithoutRecheck.

// PathLeakResult is DetectPathLeak's reading of one transcript.
type PathLeakResult struct {
	// Leaked is true when the live checkout's absolute path appeared in
	// any served tool_result payload — a fathom/grep answer built from the
	// live tree instead of the export the session ran against.
	Leaked bool `json:"leaked"`
	// Paths lists every distinct occurrence found, sorted.
	Paths []string `json:"paths,omitempty"`
}

// DetectPathLeak scans transcript's tool_result payloads (what a tool call
// SERVED to the model) for liveCheckout — the dev tree's own absolute
// path — and reports whether, and where, it appeared. liveCheckout == ""
// always reports no leak: there is nothing to check a served path against,
// which is a different fact from "checked, none found".
func DetectPathLeak(transcript []byte, liveCheckout string) PathLeakResult {
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
// "/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md"), not merely
// "found somewhere in a payload".
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

// absolutePathInShell matches a `/`-rooted path token inside a shell
// command string — a best-effort scan (a command line is not a schema
// this package owns), same shape discipline as trust.go's
// symbolOrPathToken: at least one interior "/", word characters and the
// usual path punctuation only.
var absolutePathInShell = regexp.MustCompile(`/[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)+`)

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
					paths = []string{in.FilePath}
				}
			case "Bash":
				var in struct {
					Command string `json:"command"`
				}
				if json.Unmarshal(c.Input, &in) == nil && in.Command != "" {
					paths = absolutePathInShell.FindAllString(in.Command, -1)
				}
			default:
				continue
			}
			for _, p := range paths {
				if !filepath.IsAbs(p) {
					continue
				}
				if p == sessionRoot || strings.HasPrefix(p, sessionRoot+"/") {
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
