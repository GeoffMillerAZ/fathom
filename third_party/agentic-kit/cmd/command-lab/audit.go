package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const maxSource = 4 << 20
const maxTotal = 64 << 20
const maxLine = 1 << 20
const maxRecords = 50000

var literalCMD = regexp.MustCompile(`(?:^|[,\s{])cmd\s*:\s*"((?:[^"\\]|\\["\\nrt])*)"`)
var families = []string{"git", "read", "search", "build", "transform", "network", "shell", "other"}
var gitOps = []string{"status", "diff", "log", "show", "add", "commit", "push", "pull", "fetch", "checkout", "other"}

func zeroCounts(keys []string) map[string]int {
	m := map[string]int{}
	for _, k := range keys {
		m[k] = 0
	}
	return m
}
func audit(m SourceManifest) AuditReport {
	r := AuditReport{Version: "command-lab.audit/1", Status: "complete", NoActionAuthority: true, ExpectedSources: len(m.ExpectedSourceIDs), Proposals: zeroCounts(families), Candidates: zeroCounts(families), GitOperations: zeroCounts(gitOps), Failures: zeroCounts([]string{"manifest", "source_unreadable", "source_oversize", "total_oversize", "line_oversize", "record_limit", "malformed", "unsupported"}), Sources: []SourceReport{}}
	if m.Version != "command-lab.manifest/1" || len(m.ExpectedSourceIDs) < 1 || len(m.ExpectedSourceIDs) > 32 || len(m.Sources) > 32 {
		r.Failures["manifest"]++
		r.Status = "incomplete"
		return r
	}
	expected := map[string]bool{}
	for _, x := range m.ExpectedSourceIDs {
		if !id(x) || expected[x] {
			r.Failures["manifest"]++
			r.Status = "incomplete"
			return r
		}
		expected[x] = true
	}
	seen := map[string]bool{}
	for _, s := range m.Sources {
		if !id(s.ID) || seen[s.ID] || !expected[s.ID] || !contains([]string{"codex", "claude", "normalized"}, s.Format) || s.Path == "" {
			r.Failures["manifest"]++
			r.Status = "incomplete"
			return r
		}
		seen[s.ID] = true
	}
	if len(seen) != len(expected) {
		r.Failures["manifest"]++
		r.Status = "incomplete"
	}
	for i, s := range m.Sources {
		sr := SourceReport{Ordinal: i + 1, Status: "complete"}
		if r.Bytes > maxTotal {
			r.Failures["total_oversize"]++
			sr.Status = "incomplete"
			r.Sources = append(r.Sources, sr)
			continue
		}
		info, e := os.Lstat(s.Path)
		if e != nil || !info.Mode().IsRegular() {
			r.Failures["source_unreadable"]++
			sr.Status = "incomplete"
			r.Sources = append(r.Sources, sr)
			continue
		}
		f, e := os.Open(s.Path)
		if e != nil {
			r.Failures["source_unreadable"]++
			sr.Status = "incomplete"
			r.Sources = append(r.Sources, sr)
			continue
		}
		opened, e := f.Stat()
		if e != nil || !opened.Mode().IsRegular() {
			_ = f.Close()
			r.Failures["source_unreadable"]++
			sr.Status = "incomplete"
			r.Sources = append(r.Sources, sr)
			continue
		}
		budget := int64(maxSource + 1)
		if remaining := int64(maxTotal) + 1 - r.Bytes; remaining < budget {
			budget = remaining
		}
		counted := &countingReader{inner: io.LimitReader(f, budget)}
		reader := &lineReader{inner: counted}
		for {
			line, e := reader.readLine()
			if r.Bytes+counted.bytes > maxTotal {
				r.Failures["total_oversize"]++
				sr.Status = "incomplete"
				break
			}
			if counted.bytes > maxSource {
				r.Failures["source_oversize"]++
				sr.Status = "incomplete"
				break
			}
			if errors.Is(e, errLineOverflow) {
				r.Failures["line_oversize"]++
				sr.Status = "incomplete"
				break
			}
			if len(line) == 0 && e == io.EOF {
				break
			}
			if len(line) > 0 {
				if len(line) > maxLine {
					r.Failures["line_oversize"]++
					sr.Status = "incomplete"
					break
				}
				if r.Records >= maxRecords {
					r.Failures["record_limit"]++
					sr.Status = "incomplete"
					break
				}
				r.Records++
				sr.Records++
				line = bytes.TrimSuffix(line, []byte{'\n'})
				if !utf8.Valid(line) {
					r.Failures["malformed"]++
					sr.Status = "incomplete"
				} else {
					auditLine(s.Format, line, &r, &sr)
				}
			}
			if e != nil {
				if !errors.Is(e, io.EOF) {
					r.Failures["source_unreadable"]++
					sr.Status = "incomplete"
				}
				break
			}
		}
		f.Close()
		sr.Bytes = counted.bytes
		r.Bytes += counted.bytes
		if sr.Status == "complete" {
			r.LoadedSources++
		}
		r.Sources = append(r.Sources, sr)
	}
	for _, n := range r.Failures {
		if n > 0 {
			r.Status = "incomplete"
			break
		}
	}
	return r
}

var errLineOverflow = errors.New("line overflow")

type countingReader struct {
	inner io.Reader
	bytes int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, e := c.inner.Read(p)
	c.bytes += int64(n)
	return n, e
}

type lineReader struct {
	inner   io.Reader
	pending []byte
	buffer  [32 << 10]byte
}

func (r *lineReader) readLine() ([]byte, error) {
	line := make([]byte, 0, 4096)
	for {
		if len(r.pending) > 0 {
			take := len(r.pending)
			if end := bytes.IndexByte(r.pending, '\n'); end >= 0 {
				take = end + 1
			}
			if len(line)+take > maxLine {
				take = maxLine + 1 - len(line)
				line = append(line, r.pending[:take]...)
				r.pending = r.pending[take:]
				return line, errLineOverflow
			}
			line = append(line, r.pending[:take]...)
			r.pending = r.pending[take:]
			if len(line) > 0 && line[len(line)-1] == '\n' {
				return line, nil
			}
			continue
		}
		remaining := maxLine + 1 - len(line)
		if remaining <= 0 {
			return line, errLineOverflow
		}
		if remaining > len(r.buffer) {
			remaining = len(r.buffer)
		}
		n, e := r.inner.Read(r.buffer[:remaining])
		if n > 0 {
			r.pending = r.buffer[:n]
			continue
		}
		if e != nil {
			return line, e
		}
	}
}
func auditLine(format string, line []byte, r *AuditReport, sr *SourceReport) {
	if !uniqueJSON(line) {
		r.Failures["malformed"]++
		sr.Status = "incomplete"
		return
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(line, &root) != nil || root == nil {
		r.Failures["malformed"]++
		sr.Status = "incomplete"
		return
	}
	if format == "codex" {
		call := root
		if payload := object(root["payload"]); payload != nil {
			call = payload
		}
		var encoded string
		if json.Unmarshal(call["arguments"], &encoded) == nil && !uniqueJSON([]byte(encoded)) {
			r.Failures["malformed"]++
			sr.Status = "incomplete"
			return
		}
	}
	if format == "claude" {
		msg := object(root["message"])
		var content []map[string]json.RawMessage
		if json.Unmarshal(msg["content"], &content) == nil {
			found := false
			for _, part := range content {
				if getString(part, "type") == "tool_use" {
					found = true
					b, _ := json.Marshal(part)
					auditLine(format, b, r, sr)
				}
			}
			if !found {
				r.Failures["unsupported"]++
				sr.Status = "incomplete"
			}
			return
		}
	}
	if format == "normalized" {
		var tool, command string
		if len(root["tool"]) == 0 || json.Unmarshal(root["tool"], &tool) != nil || len(root["command"]) == 0 || json.Unmarshal(root["command"], &command) != nil {
			r.Failures["malformed"]++
			sr.Status = "incomplete"
			return
		}
	}
	tool, cmd, js, ok := extract(format, root)
	if !ok {
		r.Failures["unsupported"]++
		sr.Status = "incomplete"
		return
	}
	if js {
		matches := literalCMD.FindAllSubmatch([]byte(cmd), -1)
		for _, m := range matches {
			var candidate string
			if json.Unmarshal(append(append([]byte{'"'}, m[1]...), '"'), &candidate) != nil {
				continue
			}
			for _, part := range strings.Split(candidate, "\n") {
				fam, _ := family(part)
				r.Candidates[fam]++
			}
		}
		r.Failures["unsupported"]++
		sr.Status = "incomplete"
		return
	}
	if tool == "" && cmd == "" {
		r.Failures["unsupported"]++
		sr.Status = "incomplete"
		return
	}
	fam, op := family(cmd)
	if cmd == "" {
		fam = toolFamily(tool)
	}
	r.Proposals[fam]++
	if fam == "git" {
		r.GitOperations[op]++
	}
}
func uniqueJSON(b []byte) bool {
	if !jsonwire.ValidUnicode(b) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if _, e := parseValue(d, 0, true); e != nil {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func getString(m map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(m[key], &s)
	return s
}
func requiredString(m map[string]json.RawMessage, key string) (string, bool) {
	var s string
	raw, ok := m[key]
	if !ok || json.Unmarshal(raw, &s) != nil || s == "" {
		return "", false
	}
	return s, true
}
func object(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	return m
}
func extract(format string, m map[string]json.RawMessage) (string, string, bool, bool) {
	if format == "normalized" {
		var t, c string
		if len(m["tool"]) == 0 || json.Unmarshal(m["tool"], &t) != nil || len(m["command"]) == 0 || json.Unmarshal(m["command"], &c) != nil {
			return "", "", false, false
		}
		return t, c, t == "functions.exec", true
	}
	if format == "codex" {
		if getString(m, "type") == "function_call" {
			t := getString(m, "name")
			args := object(m["arguments"])
			if args == nil {
				var encoded string
				if json.Unmarshal(m["arguments"], &encoded) == nil {
					args = object([]byte(encoded))
				}
			}
			if t == "functions.exec" {
				return t, getString(args, "code"), true, true
			}
			if t == "functions.exec_command" || t == "shell_command" {
				cmd, ok := requiredString(args, "cmd")
				return t, cmd, false, ok
			}
			return t, getString(args, "cmd"), false, t != ""
		}
		payload := object(m["payload"])
		if getString(payload, "type") == "function_call" {
			return extract(format, payload)
		}
	}
	if format == "claude" {
		if getString(m, "type") == "tool_use" {
			t := getString(m, "name")
			in := object(m["input"])
			if t == "Bash" {
				cmd, ok := requiredString(in, "command")
				return t, cmd, false, ok
			}
			return t, getString(in, "command"), false, t != ""
		}
		msg := object(m["message"])
		var content []map[string]json.RawMessage
		if json.Unmarshal(msg["content"], &content) == nil {
			for _, c := range content {
				if getString(c, "type") == "tool_use" {
					return extract(format, c)
				}
			}
		}
	}
	return "", "", false, false
}
func toolFamily(t string) string {
	switch t {
	case "Read", "read_file":
		return "read"
	case "Grep", "Glob", "search":
		return "search"
	case "Bash", "shell_command":
		return "shell"
	case "Write", "Edit":
		return "transform"
	default:
		return "other"
	}
}
func family(c string) (string, string) {
	c = strings.TrimSpace(c)
	if c == "" {
		return "other", "other"
	}
	parts := strings.Fields(c)
	if len(parts) == 0 {
		return "other", "other"
	}
	first := parts[0]
	if strings.ContainsAny(c, "$`;&|(){}") || contains([]string{"sh", "bash", "zsh", "python", "node", "ruby", "perl", "eval", "env", "sudo"}, first) {
		return "shell", "other"
	}
	switch first {
	case "git":
		op := "other"
		if len(parts) > 1 && contains(gitOps, parts[1]) {
			op = parts[1]
		}
		return "git", op
	case "cat", "head", "tail", "sed", "less":
		return "read", "other"
	case "rg", "grep", "find":
		return "search", "other"
	case "go", "task", "make", "npm", "cargo":
		return "build", "other"
	case "cp", "mv", "rm", "mkdir", "touch", "gofmt":
		return "transform", "other"
	case "curl", "wget", "ssh", "scp":
		return "network", "other"
	default:
		return "other", "other"
	}
}
