package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// readings_overlap.go is SB-23 (vision 27 §5, M5): in `planner-workers`
// mode, how much of what a worker read had the planner already read?

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

// readToolFile returns the file path a tool_use call named, for the two
// read-shaped tools SessionReadBytes weighs: the harness's own "Read" tool
// (input {"file_path": "…"}) and any MCP server's read_source tool (MCP
// names them "mcp__<server>__read_source"; input {"file": "…", …}) — ""
// for every other tool, including Grep/Glob/Bash, which do not name one
// file's content the way these two do.
func readToolFile(name string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
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
// SessionResult.RawPath) and totals bytes read per file: a Read tool
// result's length, or a read_source result's length — its bounded-read
// JSON envelope, not the underlying file's real size, since that envelope
// is fathom's own weight on the wire. Every other tool call is ignored:
// this is the byte-weighted read set ReexplorationOverlap needs, not a
// general transcript accounting.
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

	type pending struct{ file string }
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
				if !ok || p.file == "" {
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
