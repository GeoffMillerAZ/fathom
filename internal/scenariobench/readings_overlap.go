package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path"
	"strings"
)

// readings_overlap.go is SB-23 (vision 27 §5, M5): in `planner-workers`
// mode, how much of what a worker read had the planner already read? RC-11
// (vision 28 §2.4, F6) extended what SessionReadBytes weighs from Read
// alone to Read, read_source, Grep results, and a Bash call recognizable
// as a literal-path file read — see SessionReadBytes' own doc comment for
// the four shapes and grepFileBytes/bashReadFile for how each is
// attributed to a file.

// streamToolEvent is the subset of one `claude -p --output-format
// stream-json` line SessionReadBytes needs: an assistant turn's tool_use
// blocks (name, id, input) and a user turn's tool_result blocks
// (tool_use_id, content) — the same event shape
// internal/contextbench.FirstGoldHit reads, extended with Input (which
// FirstGoldHit never needed).
type streamToolEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type      string          `json:"type"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
		} `json:"content"`
	} `json:"message"`
}

// readToolFile returns the file path a tool_use call named, for the
// single-file-shaped read tools SessionReadBytes weighs directly: the
// harness's own "Read" tool (input {"file_path": "…"}), any MCP server's
// read_source tool (MCP names them "mcp__<server>__read_source"; input
// {"file": "…", …}), and a "Bash" call recognizable as a literal-path file
// read (bashReadFile) — "" for every other tool, including Glob, and for
// "Grep", which SessionReadBytes weighs separately from its RESULT text
// (grepFileBytes) since one Grep call can name many files, never one.
func readToolFile(name string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	if name == "Bash" {
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &p); err != nil {
			return ""
		}
		return bashReadFile(p.Command)
	}
	if name != "Read" && !strings.HasSuffix(name, "read_source") {
		return ""
	}
	var p struct {
		FilePath string `json:"file_path"`
		File     string `json:"file"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return ""
	}
	if p.FilePath != "" {
		return p.FilePath
	}
	return p.File
}

// bashShellMetachars are the characters whose presence makes a Bash
// command's target NOT a "literal path" (RC-11, vision 28 §2.4 F6): a
// pipe or redirection means the tool result is not that one file's raw
// content, a variable/subshell/glob means the path is not written out in
// the transcript at all. Any of these disqualifies the whole command.
const bashShellMetachars = "|&;<>`$*?~\n"

// stripQuoted drops the content of every single- or double-quoted span in
// s (naively — no backslash-escape handling, which this repo's own
// commands never need): an awk/sed script's own '$1' or similar is quoted
// text, not a shell variable, and must not trip bashShellMetachars the way
// an unquoted "cat $FILE" correctly does.
func stripQuoted(s string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// bashReadFiles are the shell commands RC-11 recognizes as reading one
// file's content to stdout when given a literal trailing path: the CLAUDE.md
// non-negotiables' own routing line names cat/sed -n/head/tail/awk/less as
// grep's-job-shaped, file-content-shaped commands, and a worker discovering
// through one of them is exactly what F6 says the old Read-only weighing
// made invisible.
var bashReadFiles = map[string]bool{
	"cat": true, "head": true, "tail": true, "less": true, "sed": true, "awk": true,
}

// bashReadFile returns the literal file path a Bash command reads via
// cat/sed -n/head/tail/awk/less, or "" when command is not recognizable as
// one of those (a pipe, redirection, glob, or variable substitution makes
// it unrecognizable, per bashShellMetachars — reporting a false single-file
// byte weight for a pipeline would misattribute another command's output).
// The path is the command's LAST whitespace-separated token: sed/awk's
// script argument is whatever came before it and is never inspected, only
// skipped over, so a quoted "1,5p" style script needs no special parsing.
func bashReadFile(command string) string {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsAny(stripQuoted(command), bashShellMetachars) {
		return ""
	}
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return ""
	}
	if !bashReadFiles[path.Base(fields[0])] {
		return ""
	}
	last := fields[len(fields)-1]
	if last == "" || strings.HasPrefix(last, "-") {
		return "" // a trailing flag, not a path (e.g. "cat -n" with nothing after)
	}
	// awk always takes a script argument before the path ("awk '{...}'
	// file" is the 3-field minimum); sed's read shape is specifically
	// "sed -n <script> file" (the 4-field minimum) — a bare "sed -n file"
	// has no script and sed would read it as one, not as a path.
	switch path.Base(fields[0]) {
	case "awk":
		if len(fields) < 3 {
			return ""
		}
	case "sed":
		if len(fields) < 4 || fields[1] != "-n" {
			return ""
		}
	}
	return last
}

// grepFileBytes attributes a Grep tool_result's text to the file(s) it
// names, line by line: ripgrep/grep's own "path:line:match" and
// "path:match" content-mode output, and the bare "path" lines
// files_with_matches mode prints. A line with no matched file — prose, a
// summary count, "No matches found" — contributes to no file, so a Grep
// call whose whole result is not a bare list of paths and matches (like a
// tool description or an error string) is correctly weighed as zero, not
// misattributed. Each matched file's weight is that LINE's own byte
// length (the newline the caller's strings.Split ate is not added back),
// so the sum over every file stays close to the result's total bytes
// rather than multiplying the result by however many files matched.
func grepFileBytes(resultText string) map[string]int64 {
	out := map[string]int64{}
	for _, line := range strings.Split(resultText, "\n") {
		if line == "" {
			continue
		}
		var candidate string
		if i := strings.Index(line, ":"); i > 0 {
			candidate = line[:i]
		} else if !strings.ContainsAny(line, " \t") {
			candidate = line // files_with_matches mode: one bare path per line
		}
		if candidate == "" || strings.ContainsAny(candidate, " \t") {
			continue
		}
		out[candidate] += int64(len(line))
	}
	return out
}

// rawResultText flattens a tool_result content field — a plain string or a
// list of {type:"text", text:…} blocks — into text. The same shape
// internal/contextbench's unexported rawText reads; copied rather than
// imported so this package never depends on another bench package's
// private parsing internals.
func rawResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, bl := range blocks {
		b.WriteString(bl.Text)
	}
	return b.String()
}

// SessionReadBytes parses one captured stream-json transcript (a
// SessionResult.RawPath) and totals bytes read per file — RC-11 (vision 28
// §2.4, F6: "the overlap reading weighed only Read, while workers
// discovered through Grep and cat"). Four tool shapes are weighed, all by
// their RESULT length, never the underlying file's real size:
//   - Read: its result's length, keyed by input.file_path.
//   - an MCP server's read_source (name ends "read_source"): its result's
//     length — the bounded-read JSON envelope, not the file's real size,
//     since that envelope is fathom's own weight on the wire — keyed by
//     input.file.
//   - Bash, when bashReadFile recognizes the command as a literal-path
//     read via cat/sed -n/head/tail/awk/less: the command's stdout length,
//     keyed by the path bashReadFile extracted.
//   - Grep: grepFileBytes attributes the result text to every file its
//     ripgrep-shaped output lines name (never Grep's own input — one call
//     can match many files, so this is per-file, not per-call).
//
// Every other tool call (Glob, a plain Bash that is not a recognized
// read, …) is ignored: this is the byte-weighted read set
// ReexplorationOverlap needs, not a general transcript accounting.
//
// A line that is not a decodable stream-json object is skipped — the same
// tolerance internal/contextbench.FirstGoldHit gives a transcript that may
// carry a stray non-JSON line. rawPath == "" returns (nil, nil): the
// caller (ReexplorationOverlap) falls back to counting files instead.
func SessionReadBytes(rawPath string) (map[string]int64, error) {
	if rawPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, err
	}

	type pending struct {
		file string
		grep bool
	}
	byID := map[string]pending{}
	out := map[string]int64{}

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
				if c.Name == "Grep" {
					byID[c.ID] = pending{grep: true}
					continue
				}
				if file := readToolFile(c.Name, c.Input); file != "" {
					byID[c.ID] = pending{file: file}
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				p, ok := byID[c.ToolUseID]
				if !ok {
					continue
				}
				if p.grep {
					for f, b := range grepFileBytes(rawResultText(c.Content)) {
						out[f] += b
					}
					continue
				}
				if p.file == "" {
					continue
				}
				out[p.file] += int64(len(rawResultText(c.Content)))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// sameFile reports whether a and b name the same file across an
// absolute-vs-repo-relative mismatch (SessionReadBytes' keys come from raw
// tool input and may be absolute; SessionResult.FilesRead is documented
// repo-relative): equal, or one a path-suffix of the other.
func sameFile(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}

// ReexplorationOverlap is SB-23 (vision 27 §5, M5): in `planner-workers`
// mode, for each worker, the bytes of files it read that the planning
// session (Role == "planner") had already read, over the bytes it read in
// total. Byte weights come from SessionReadBytes over each session's raw
// transcript; when any session's RawPath is empty (or its transcript
// yields no reads), the reading falls back to counting FILES instead of
// bytes — a coarser ratio, `not_measurable` says so — rather than
// reporting a byte ratio built on a hole.
func ReexplorationOverlap(sessions []SessionResult) OverlapReading {
	var planner *SessionResult
	var workers []SessionResult
	for i := range sessions {
		switch sessions[i].Role {
		case "planner":
			planner = &sessions[i]
		case "worker":
			workers = append(workers, sessions[i])
		}
	}

	reading := OverlapReading{Workers: len(workers)}
	if planner == nil || len(workers) == 0 {
		reading.NotMeasurable = "no planner/worker sessions recorded"
		return reading
	}

	byteMode := planner.RawPath != ""
	for _, w := range workers {
		if w.RawPath == "" {
			byteMode = false
		}
	}

	var reexplore, total int64
	if byteMode {
		plannerBytes, err := SessionReadBytes(planner.RawPath)
		if err != nil || len(plannerBytes) == 0 {
			byteMode = false
		} else {
			for _, w := range workers {
				wb, err := SessionReadBytes(w.RawPath)
				if err != nil {
					byteMode = false
					break
				}
				for f, b := range wb {
					total += b
					for pf := range plannerBytes {
						if sameFile(f, pf) {
							reexplore += b
							break
						}
					}
				}
			}
		}
	}

	if !byteMode {
		reexplore, total = 0, 0
		for _, w := range workers {
			for _, f := range w.FilesRead {
				total++
				for _, pf := range planner.FilesRead {
					if sameFile(f, pf) {
						reexplore++
						break
					}
				}
			}
		}
		reading.NotMeasurable = "byte counts unavailable (empty raw path, or an unreadable/read-free transcript); reporting a file-count ratio instead"
	}

	reading.ReexploreBytes = reexplore
	reading.WorkerBytes = total
	if total > 0 {
		reading.Ratio = float64(reexplore) / float64(total)
	} else if reading.NotMeasurable == "" {
		reading.NotMeasurable = "no session recorded reads"
	}
	return reading
}
