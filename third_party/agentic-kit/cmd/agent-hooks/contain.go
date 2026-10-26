package main

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// containment is the result of containing one oversized main-thread output.
type containment struct {
	// Response is the replacement tool_response, same shape as the original.
	Response json.RawMessage
	// Chars is the original stdout plus stderr size; Kept is what stays inline.
	Chars, Kept int
	// Path is where the full output was saved.
	Path string
}

// containBash replaces an oversized main-thread Bash result with its head and
// tail plus the path of the saved full output. The replacement keeps every
// field of Claude Code's Bash tool_response; only stdout and stderr shrink.
// ok is false when the output is within budget, is an image, was already
// persisted by Claude Code, has a shape this code does not understand, or
// could not be saved: in every such case the original passes through
// untouched, because containment without the saved evidence would sever the
// link to the ground truth.
func (a *app) containBash(cfg MainThreadConfig, in hookInput) (containment, bool) {
	if cfg.BashMaxChars == 0 {
		return containment{}, false
	}
	fields, err := rawObject(in.ToolResponse)
	if err != nil {
		return containment{}, false
	}
	if _, persisted := fields["persistedOutputPath"]; persisted {
		// Claude Code already saved this output and shows the model a short
		// preview (its own containment, above about 30k characters); a
		// replacement would be ignored and a second copy is waste.
		return containment{}, false
	}
	var isImage bool
	if json.Unmarshal(fields["isImage"], &isImage) == nil && isImage {
		return containment{}, false // clipping base64 image data corrupts it
	}
	stdout, okOut := jsonString(fields["stdout"])
	stderr, okErr := jsonString(fields["stderr"])
	if !okOut && !okErr {
		return containment{}, false
	}
	outN, errN := utf8.RuneCountInString(stdout), utf8.RuneCountInString(stderr)
	total := outN + errN
	if total <= cfg.BashMaxChars {
		return containment{}, false
	}

	full := stdout
	if stderr != "" {
		full += "\n----- stderr -----\n" + stderr
	}
	path, err := a.saveOutput(in.SessionID, toolUseID(in), full)
	if err != nil {
		return containment{}, false
	}

	// Inline budget: stderr gets up to a quarter (it is usually short, and
	// when long its tail carries the error), stdout the rest, and whatever
	// one stream does not need goes to the other.
	budget := cfg.HeadChars + cfg.TailChars
	errBudget := min(errN, budget/4)
	outBudget := min(outN, budget-errBudget)
	errBudget = min(errN, budget-outBudget)
	kept := outBudget + errBudget
	notice := fmt.Sprintf("\n\n[agent-hooks: main-thread output contained: %d of %d characters kept inline. "+
		"Full output: %s . Read it in ranges, or hand the deep read to a subagent so it stays out of this context.]\n\n",
		kept, total, path)

	// Every output over budget has at least one stream over its share, so the
	// notice always lands exactly once: in stdout when stdout was clipped,
	// otherwise in stderr.
	newOut, outClipped := clipShare(stdout, outBudget, cfg, notice)
	errNotice := ""
	if !outClipped {
		errNotice = notice
	}
	newErr, errClipped := clipShare(stderr, errBudget, cfg, errNotice)
	if !outClipped && !errClipped { // unreachable by the arithmetic; never lose the pointer
		newOut = notice + newOut
	}
	if okOut {
		fields["stdout"], _ = marshalNoEscape(newOut)
	}
	if okErr {
		fields["stderr"], _ = marshalNoEscape(newErr)
	}
	resp, err := marshalNoEscape(fields)
	if err != nil {
		return containment{}, false
	}
	return containment{Response: resp, Chars: total, Kept: kept, Path: path}, true
}

// clipShare keeps share runes of s, split between head and tail in the
// configured ratio, with the notice between them. It reports whether s was
// clipped; a string within its share is returned unchanged.
func clipShare(s string, share int, cfg MainThreadConfig, notice string) (string, bool) {
	if utf8.RuneCountInString(s) <= share {
		return s, false
	}
	head := share * cfg.HeadChars / (cfg.HeadChars + cfg.TailChars)
	return clip(s, head, share-head, notice), true
}

// clip keeps the first head and last tail runes of s around the notice. A
// string already within head+tail is returned unchanged.
func clip(s string, head, tail int, notice string) string {
	r := []rune(s)
	if len(r) <= head+tail {
		return s
	}
	return string(r[:head]) + notice + string(r[len(r)-tail:])
}

func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// toolUseID extracts tool_use_id when the host sends it.
func toolUseID(in hookInput) string {
	if in.ToolUseID != "" {
		return in.ToolUseID
	}
	return "call"
}
