package transcript

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Event contains only the features labs may retain. A line event preserves
// metadata even for a prompt-only line. Cwd and GitBranch are caller-salted
// HMAC-SHA256 identifiers truncated to 16 hex characters, never raw text.
// Other kinds follow their source block order. Shape describes JSON types, including nested objects and arrays, with
// object members sorted by shape; no argument keys or values survive.
type Event struct {
	Kind      string
	Line      int
	Cwd       string
	GitBranch string
	Timestamp string
	ToolUseID string
	Name      string
	Shape     string
	IsError   bool
	Bytes     int
	HookEvent string
}

// DecodeTranscriptEvents reads one Claude Code transcript in physical line and
// block order. Uses and results deduplicate separately by tool-use id (a result
// must not suppress its use). Empty ids do not alias. All failures are counted
// and named; no prompt, tool input, output or file content leaves this function.
// salt must be the salt supplied by bundle.LoadOrCreateSalt, shared with the
// session hash. fileIndex is the file's ordinal in the caller's deterministic walk.
func DecodeTranscriptEvents(path, salt string, fileIndex int) Result {
	var events []Event
	seenUses, seenResults := map[string]bool{}, map[string]bool{}
	agents := map[string]bool{}
	duplicates := 0
	subagent := isSubagent(path)
	var last Event
	started := false
	var inputIssues []Issue
	malformed := map[int]bool{}
	issue := func(number int, kind string) {
		inputIssues = append(inputIssues, Issue{File: fileLabel(path, fileIndex), Line: number, Kind: kind})
		malformed[number] = true
	}
	r := walkFile(path, fileIndex, func(number int, l *Line, blocks []Block) {
		base := Event{Line: number, Cwd: metadataHash(salt, l.Cwd), GitBranch: metadataHash(salt, l.GitBranch), Timestamp: l.Timestamp}
		add := func(kind string, e Event) { e.Kind = kind; events = append(events, e) }
		if subagent && !started {
			add("subagent_start", base)
			started = true
		}
		last = base
		add("line", base)
		if a := l.Attachment; a != nil && a.Type == "hook_additional_context" {
			e := base
			e.HookEvent, e.ToolUseID = a.HookEvent, a.ToolUseID
			add("hook_additional_context", e)
		}
		if l.Type == "system" && (l.Subtype == "subagent_start" || l.Subtype == "subagent_stop") {
			add(l.Subtype, base)
		}
		if l.Type == "system" && l.Subtype == "hook_response" {
			var envelope struct {
				HookSpecificOutput struct {
					HookEventName     string `json:"hookEventName"`
					AdditionalContext string `json:"additionalContext"`
				} `json:"hookSpecificOutput"`
			}
			if json.Unmarshal([]byte(l.Output), &envelope) == nil && envelope.HookSpecificOutput.AdditionalContext != "" {
				e := base
				e.HookEvent = l.HookEvent
				if e.HookEvent == "" {
					e.HookEvent = envelope.HookSpecificOutput.HookEventName
				}
				add("hook_additional_context", e)
			}
		}
		single := ResultCount(blocks) == 1
		for _, b := range blocks {
			e := base
			switch b.Type {
			case "tool_use":
				if b.ID != "" && seenUses[b.ID] {
					duplicates++
					continue
				}
				if b.ID != "" {
					seenUses[b.ID] = true
				}
				e.ToolUseID, e.Name = b.ID, b.Name
				var input any
				inputErr := json.Unmarshal(b.Input, &input)
				if inputErr != nil {
					issue(number, "invalid_tool_input")
					e.Shape = "invalid"
				} else {
					e.Shape = inputShape(input)
				}
				add("tool_use", e)
				if b.Name == "Skill" {
					var skill struct {
						Skill string `json:"skill"`
					}
					e.Name = "skill"
					if inputErr == nil {
						if err := json.Unmarshal(b.Input, &skill); err != nil {
							issue(number, "invalid_skill_input")
						} else if skillName.MatchString(skill.Skill) {
							e.Name = skill.Skill
						}
					}
					add("skill", e)
				}
				if b.Name == "Task" || b.Name == "Agent" {
					if b.ID != "" {
						agents[b.ID] = true
					}
					add("subagent_start", e)
				}
			case "tool_result":
				if b.ToolUseID != "" && seenResults[b.ToolUseID] {
					duplicates++
					continue
				}
				if b.ToolUseID != "" {
					seenResults[b.ToolUseID] = true
				}
				e.ToolUseID, e.IsError = b.ToolUseID, b.IsError
				stdout, stderr, _ := ResultOf(l, &b, single)
				e.Bytes = len(stdout) + len(stderr)
				add("tool_result", e)
				if agents[b.ToolUseID] {
					add("subagent_stop", e)
					delete(agents, b.ToolUseID)
				}
			case "hook_additional_context":
				e.HookEvent = b.HookEvent
				if e.HookEvent == "" {
					e.HookEvent = l.HookEvent
				}
				add("hook_additional_context", e)
			}
		}
	})
	// An unreadable tail cannot prove the subagent's closing boundary.
	if subagent && started && r.UnreadableFiles == 0 {
		last.Kind = "subagent_stop"
		events = append(events, last)
	}
	r.Issues = append(r.Issues, inputIssues...)
	sort.SliceStable(r.Issues, func(i, j int) bool { return r.Issues[i].Line < r.Issues[j].Line })
	r.MalformedLines += len(malformed)
	r.Events, r.Duplicates = events, duplicates
	return r
}

func isSubagent(path string) bool {
	dir, base := filepath.Split(path)
	if !strings.HasPrefix(base, "agent-") || !strings.HasSuffix(base, ".jsonl") {
		return false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	n := len(parts)
	return parts[n-1] == "subagents" || n >= 3 && parts[n-3] == "subagents" && parts[n-2] == "workflows"
}

func inputShape(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, inputShape(item))
		}
		return "array[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, inputShape(item))
		}
		sort.Strings(parts)
		return "object{" + strings.Join(parts, ",") + "}"
	}
	return "unknown"
}

var skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

func metadataHash(salt, value string) string {
	if value == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(salt))
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil)[:8])
}
