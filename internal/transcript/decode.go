// Package transcript decodes local Claude Code JSONL transcripts. Its event
// projection retains metadata and counts only, never prompt, input or output text.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// LineMax bounds a single transcript line, as in readout replay.
const LineMax = 64 << 20

// Line and Block are the transient Claude envelope readout replay already used.
// They are deliberately separate from Event: consumers of the event projection
// cannot recover transcript text. Readouts need the transient view to re-render
// their rules locally; it must never be persisted or sent to a provider.
type Line struct {
	Type       string `json:"type"`
	Subtype    string `json:"subtype"`
	UUID       string `json:"uuid"`
	SessionID  string `json:"sessionId"`
	Timestamp  string `json:"timestamp"`
	Cwd        string `json:"cwd"`
	GitBranch  string `json:"gitBranch"`
	HookEvent  string `json:"hook_event"`
	Output     string `json:"output"`
	Attachment *struct {
		Type      string `json:"type"`
		HookEvent string `json:"hookEvent"`
		ToolUseID string `json:"toolUseID"`
	} `json:"attachment"`
	Message *struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type Block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
	HookEvent string          `json:"hookEventName"`
}

type ToolResult struct {
	Stdout              *string `json:"stdout"`
	Stderr              string  `json:"stderr"`
	Interrupted         bool    `json:"interrupted"`
	PersistedOutputPath string  `json:"persistedOutputPath"`
}

// DecodeLine is the single line/block decoder shared by events and readouts.
// Plain message strings are prompts, not blocks. Invalid envelopes and block
// arrays are errors even when they do not contain a tool marker.
func DecodeLine(raw []byte) (Line, []Block, error) {
	var l Line
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return l, nil, nil
	}
	if raw[0] != '{' {
		return l, nil, errors.New("expected Claude transcript object")
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return Line{}, nil, err
	}
	if l.Message == nil {
		return l, nil, nil
	}
	content := bytes.TrimSpace(l.Message.Content)
	if len(content) == 0 || bytes.Equal(content, []byte("null")) || content[0] == '"' {
		return l, nil, nil
	}
	if content[0] != '[' {
		return l, nil, errors.New("expected Claude content array or string")
	}
	var blocks []Block
	if err := json.Unmarshal(content, &blocks); err != nil {
		return l, nil, err
	}
	return l, blocks, nil
}

// ResultOf is a tool result as the readout hook saw it: structured streams
// only for a single successful result, otherwise content. It never reads a
// persisted file; that remains readout replay's local join.
func ResultOf(l *Line, b *Block, single bool) (stdout, stderr string, tr ToolResult) {
	if tr, ok := StructuredResult(l, single); ok && !b.IsError {
		return *tr.Stdout, tr.Stderr, tr
	}
	return ResultText(b.Content), "", ToolResult{}
}

// StructuredResult decodes the line-level streams only when exactly one result
// can own them. The transient value preserves replay's partial-decode behavior.
func StructuredResult(l *Line, single bool) (tr ToolResult, ok bool) {
	ok = single && len(l.ToolUseResult) > 0 && l.ToolUseResult[0] == '{' &&
		json.Unmarshal(l.ToolUseResult, &tr) == nil && tr.Stdout != nil
	return tr, ok
}

// ResultText is a tool_result's content: a string, or its text blocks.
func ResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if raw[0] == '"' && json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var b strings.Builder
	for _, bl := range blocks {
		if bl.Type == "text" {
			b.WriteString(bl.Text)
		}
	}
	return b.String()
}

func ResultCount(blocks []Block) int {
	n := 0
	for _, b := range blocks {
		if b.Type == "tool_result" {
			n++
		}
	}
	return n
}

// Issue names the input and physical line that could not be decoded. Reasons
// are fixed classes: neither malformed text nor OS errors can leak content.
type Issue struct {
	File string
	Line int
	Kind string
}

type Result struct {
	Events          []Event
	Issues          []Issue
	MalformedLines  int
	OversizeLines   int
	UnreadableFiles int
	Duplicates      int
}

// WalkFile streams the existing readout consumers through the shared decoder.
// It walks one file, so its issues are labelled with file index 0; a caller
// that walks several files and needs to tell their issues apart uses
// DecodeTranscriptEvents, which numbers the files.
// No transient Line or Block is retained in the returned coverage receipt.
// The visitor sees every decoded occurrence: readouts retain their own window
// and result-pairing dedupe semantics, while events deduplicate before returning.
func WalkFile(path string, visit func(int, *Line, []Block)) Result {
	return walkFile(path, 0, visit)
}

// fileLabel identifies only the transcript class and its caller-assigned walk index.
func fileLabel(path string, index int) string {
	class := "session"
	if isSubagent(path) {
		class = "subagent"
	}
	return fmt.Sprintf("%s:%d", class, index)
}

func walkFile(path string, index int, visit func(int, *Line, []Block)) Result {
	label := fileLabel(path, index)
	f, err := os.Open(path)
	if err != nil {
		return Result{UnreadableFiles: 1, Issues: []Issue{{File: label, Kind: "unreadable_file"}}}
	}
	defer f.Close()
	var malformed []Issue
	result := WalkLines(f, func(number int, raw []byte) {
		l, blocks, err := DecodeLine(raw)
		if err != nil {
			malformed = append(malformed, Issue{Line: number, Kind: "malformed_line"})
		} else if visit != nil {
			visit(number, &l, blocks)
		}
	})
	result.MalformedLines = len(malformed)
	result.Issues = append(result.Issues, malformed...)
	for i := range result.Issues {
		result.Issues[i].File = label
	}
	sort.SliceStable(result.Issues, func(i, j int) bool { return result.Issues[i].Line < result.Issues[j].Line })
	return result
}

// WalkLines visits nonblank, bounded physical lines, including a partial line
// returned with a read error. The caller owns the reader and file identity.
// Replay uses this raw view to preserve its own historical parse accounting.
func WalkLines(r io.Reader, visit func(int, []byte)) Result {
	var result Result
	issue := func(line int, kind string) {
		result.Issues = append(result.Issues, Issue{Line: line, Kind: kind})
	}
	br := bufio.NewReaderSize(r, 1<<20)
	var buf []byte
	over, number := false, 0
	for {
		chunk, rerr := br.ReadSlice('\n')
		if len(buf)+len(chunk) <= LineMax {
			buf = append(buf, chunk...)
		} else {
			over = true
		}
		if errors.Is(rerr, bufio.ErrBufferFull) {
			continue
		}
		if len(buf) > 0 || over {
			number++
			if over {
				result.OversizeLines++
				issue(number, "oversize_line")
			} else if len(bytes.TrimSpace(buf)) > 0 && visit != nil {
				visit(number, buf)
			}
		}
		buf, over = buf[:0], false
		if rerr != nil {
			if !errors.Is(rerr, io.EOF) {
				result.UnreadableFiles++
				issue(number+1, "unreadable_file")
			}
			break
		}
	}
	return result
}
