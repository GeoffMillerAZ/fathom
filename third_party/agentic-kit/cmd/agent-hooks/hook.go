package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxHookInput caps the payload read from the host. Tool responses can be
// large (Claude Code allows up to 150k characters of Bash output); anything
// beyond the cap is treated as unreadable and the hook fails open.
const maxHookInput = 8 << 20

// codeDenyLoopReleased marks a spawn let through by the deny-loop breaker.
const codeDenyLoopReleased = "deny_loop_released"

// hook handles one host hook invocation. It always exits 0: a deny is a
// decision written to stdout, not an exit status, and every fault — panic
// included — fails open with no output and a fault record in the ledger.
func (a *app) hook(host string) (code int) {
	var ledger LedgerConfig // zero until the policy loads: no writes before then
	defer func() {
		if r := recover(); r != nil {
			a.appendLedger(ledger, ledgerRecord{Host: host, Kind: kindFault, Fault: fmt.Sprintf("panic: %v", r)})
			code = 0
		}
	}()
	lp := a.loadPolicy()
	ledger = lp.Ledger

	raw, err := io.ReadAll(io.LimitReader(a.stdin, maxHookInput+1))
	if err != nil || len(raw) > maxHookInput {
		a.appendLedger(ledger, ledgerRecord{Host: host, Kind: kindFault, Fault: "hook input unreadable or oversized"})
		return 0
	}
	in, err := decodeHookInput(raw)
	if err != nil {
		a.appendLedger(ledger, ledgerRecord{Host: host, Kind: kindFault, Fault: "hook input is not a JSON object"})
		return 0
	}

	switch {
	case in.Event == eventPreToolUse && isSpawnTool(host, in.ToolName):
		a.onSpawn(host, lp, in)
	case in.Event == eventPostToolUse && isSpawnTool(host, in.ToolName):
		a.onSpawnResult(host, lp, in)
	case in.Event == eventPostToolUse:
		a.onToolOutput(host, lp, in)
	case in.Event == eventSubagentStop:
		a.onSubagentStop(host, lp, in)
	}
	return 0
}

// onSpawn decides a subagent spawn and records the decision.
func (a *app) onSpawn(host string, lp loadedPolicy, in hookInput) {
	req, ok := parseSpawn(host, in.ToolInput)
	if !ok {
		a.appendLedger(lp.Ledger, ledgerRecord{Host: host, Kind: kindFault, Session: in.SessionID, Tool: in.ToolName, Fault: "spawn input is not a JSON object"})
		return
	}
	if req.Model == "" && req.AgentType != "" {
		req.DefinedModel = a.definedModel(host, in.CWD, req.AgentType)
	}
	v := decideSpawn(lp.Policy, req)
	if lp.Fallback != nil {
		v.Notes = append([]note{{
			Key:  "policy-fallback",
			Text: fmt.Sprintf("agent-hooks: the policy file was rejected and the built-in policy is in force (%v). Run `agent-hooks doctor`.", lp.Fallback),
		}}, v.Notes...)
	}

	rec := ledgerRecord{
		Host: host, Kind: kindSpawn, Session: in.SessionID, AgentID: in.AgentID, Tool: in.ToolName,
		AgentType: req.AgentType, Model: req.Model, Effort: req.Effort, Tier: v.Tier, Class: v.Class,
		Decision: "allow", Codes: v.Codes, Fork: req.Fork, ContextFork: req.ContextFork,
		BriefChars: len([]rune(req.Brief)), ParentModel: in.Model, Policy: lp.Source,
	}
	if v.Deny {
		if a.denyStreak(in.SessionID) < lp.Spawn.DenyLoopLimit {
			a.recordDeny(in.SessionID)
			rec.Decision = "deny"
			a.appendLedger(lp.Ledger, rec)
			writeJSON(a.stdout, preToolUseDeny(v.Reason))
			return
		}
		// The breaker: this session has been denied spawn after spawn. Let
		// this one through, say why, and keep measuring.
		rec.Codes = append(rec.Codes, codeDenyLoopReleased)
		v.Notes = append(v.Notes, note{
			Key: "deny-loop-released",
			Text: fmt.Sprintf("agent-hooks: %d spawns in a row were denied, so this one was let through to keep the session moving. "+
				"Fix the spawn call (%s); if the spawn tool cannot take these fields, relax the policy.", lp.Spawn.DenyLoopLimit, strings.Join(v.Codes, ", ")),
		})
	} else {
		a.clearDenies(in.SessionID)
	}

	var texts []string
	for _, n := range v.Notes {
		if a.once(in.SessionID, n.Key) {
			texts = append(texts, n.Text)
			rec.Notes = append(rec.Notes, n.Key)
		}
	}
	a.appendLedger(lp.Ledger, rec)
	if len(texts) > 0 {
		writeJSON(a.stdout, preToolUseContext(strings.Join(texts, "\n\n")))
	}
}

// definedModel reads the model an agent definition pins, looking in the
// project's and the user's agent directories. It reads at most a small,
// bounded prefix of one file and returns "" for anything unexpected.
func (a *app) definedModel(host, cwd, agentType string) string {
	if safeName(agentType) != agentType {
		return "" // never build a path from an unsafe name
	}
	var candidates []string
	if host == hostCodex {
		if cwd != "" {
			candidates = append(candidates, filepath.Join(cwd, ".codex", "agents", agentType+".toml"))
		}
		candidates = append(candidates, filepath.Join(a.codexHome(), "agents", agentType+".toml"))
	} else {
		if cwd != "" {
			candidates = append(candidates, filepath.Join(cwd, ".claude", "agents", agentType+".md"))
		}
		candidates = append(candidates, filepath.Join(a.claudeHome(), "agents", agentType+".md"))
	}
	for _, path := range candidates {
		raw, err := readCapped(path, 64<<10)
		if err != nil {
			continue
		}
		return agentFileModel(host, string(raw))
	}
	return ""
}

// agentFileModel extracts the model from a Claude Code agent's frontmatter
// ("model: haiku") or a Codex agent file ("model = \"x\""). "inherit" and
// anything that is not a plain model value read as no model.
func agentFileModel(host, text string) string {
	lines := strings.Split(text, "\n")
	if host == hostClaude {
		if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
			return ""
		}
		for _, l := range lines[1:] {
			if strings.TrimSpace(l) == "---" {
				break
			}
			if k, v, ok := strings.Cut(l, ":"); ok && strings.TrimSpace(k) == "model" {
				return plainModel(strings.Trim(strings.TrimSpace(v), `"'`))
			}
		}
		return ""
	}
	for _, l := range lines {
		if k, v, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == "model" {
			if s, err := strconv.Unquote(strings.TrimSpace(v)); err == nil {
				return plainModel(s)
			}
		}
	}
	return ""
}

func plainModel(s string) string {
	if !modelRE.MatchString(s) || strings.EqualFold(s, "inherit") {
		return ""
	}
	return s
}

// The deny streak is the number of spawns denied in a row in one session,
// kept as O_EXCL marker files deny-1, deny-2, … in the session's state
// directory. An allowed spawn clears it.

func (a *app) denyStreak(session string) int {
	dir := filepath.Join(a.stateDir(), "sessions", sessionKey(session))
	n := 0
	for n < maxDenyLoopLimit {
		if _, err := os.Stat(filepath.Join(dir, "deny-"+strconv.Itoa(n+1))); err != nil {
			break
		}
		n++
	}
	return n
}

func (a *app) recordDeny(session string) {
	for i := 1; i <= maxDenyLoopLimit; i++ {
		if a.once(session, "deny-"+strconv.Itoa(i)) {
			return
		}
	}
}

func (a *app) clearDenies(session string) {
	dir := filepath.Join(a.stateDir(), "sessions", sessionKey(session))
	for i := 1; i <= maxDenyLoopLimit; i++ {
		if os.Remove(filepath.Join(dir, "deny-"+strconv.Itoa(i))) != nil {
			return
		}
	}
}

// onSpawnResult records a finished Claude Code spawn. Codex's spawn_agent
// returns only a handle, and an async Claude Code spawn returns at launch, so
// for those the spend is measured at SubagentStop instead.
func (a *app) onSpawnResult(host string, lp loadedPolicy, in hookInput) {
	req, _ := parseSpawn(host, in.ToolInput)
	rec := ledgerRecord{
		Host: host, Kind: kindSpawnResult, Session: in.SessionID, AgentID: in.AgentID, Tool: in.ToolName,
		AgentType: req.AgentType, Model: req.Model, ParentModel: in.Model, Policy: lp.Source,
	}
	var res agentResult
	if host == hostClaude && json.Unmarshal(in.ToolResponse, &res) == nil {
		rec.Status, rec.Resolved, rec.SpawnedAgent = res.Status, res.ResolvedModel, res.AgentID
		rec.TotalTokens, rec.DurationMs, rec.ToolUses = res.TotalTokens, res.TotalDurationMs, res.ToolUseCount
		rec.ReportChars = textChars(res.Content)
	}
	a.appendLedger(lp.Ledger, rec)
}

// onSubagentStop records a finished subagent's final message size and its
// transcript path. Tokens are read from the transcript at report time, not
// here: Claude Code fires SubagentStop before it flushes the final message,
// so a read now would undercount, and the hot path stays cheap.
func (a *app) onSubagentStop(host string, lp loadedPolicy, in hookInput) {
	a.appendLedger(lp.Ledger, ledgerRecord{
		Host: host, Kind: kindSubagentStop, Session: in.SessionID, AgentID: in.AgentID,
		AgentType: in.AgentType, ParentModel: in.Model, ReportChars: len([]rune(in.LastMessage)),
		Transcript: in.AgentTranscriptPath, Policy: lp.Source,
	})
}

// onToolOutput measures a tool result and, for oversized main-thread Bash
// output in Claude Code, contains it. Claude Code sets agent_id only inside
// subagents, so its absence identifies the main thread.
func (a *app) onToolOutput(host string, lp loadedPolicy, in hookInput) {
	rec := ledgerRecord{
		Host: host, Kind: kindToolOutput, Session: in.SessionID, AgentID: in.AgentID, Tool: in.ToolName,
		OutputChars: textChars(in.ToolResponse), Policy: lp.Source,
	}
	var persisted struct {
		Size int `json:"persistedOutputSize"`
	}
	if json.Unmarshal(in.ToolResponse, &persisted) == nil && persisted.Size > 0 {
		rec.OutputChars = persisted.Size // the response carries only a preview
	}
	if isShellTool(host, in.ToolName) {
		rec.Command = commandHead(in.ToolInput)
	}
	if host == hostClaude && in.ToolName == "Bash" && in.AgentID == "" {
		if c, ok := a.containBash(lp.MainThread, in); ok {
			rec.Contained, rec.OutputChars = true, c.Chars
			writeJSON(a.stdout, postToolUseReplace(c.Response))
		}
	}
	if lp.Ledger.RecordToolOutput {
		a.appendLedger(lp.Ledger, rec)
	}
}

// Host output envelopes. Both hosts accept these hookSpecificOutput shapes on
// exit 0. A deny never sets continue:false: Codex treats unsupported fields
// as a hook failure and fails open.

func preToolUseDeny(reason string) any {
	return map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": eventPreToolUse, "permissionDecision": "deny", "permissionDecisionReason": reason,
	}}
}

func preToolUseContext(text string) any {
	return map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": eventPreToolUse, "additionalContext": text,
	}}
}

func postToolUseReplace(resp json.RawMessage) any {
	return map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": eventPostToolUse, "updatedToolOutput": resp,
	}}
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
