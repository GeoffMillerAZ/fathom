package contextbench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// Transcript parsing (PB-11): tokens-to-first-gold-hit and tool-call
// counts per cell, read from a `claude -p --output-format stream-json
// --verbose` transcript (one JSON event per line). The final-envelope
// figures say what a cell cost in total; these say how far into the
// session the arm first touched the answer, which is the discovery
// question the bench exists to ask.

// GoldHit is FirstGoldHit's reading of one transcript.
type GoldHit struct {
	// Found is false when no gold token ever appeared in an assistant
	// message or a tool result; the token counts then cover the whole
	// transcript.
	Found bool `json:"found"`
	// TokensToFirstHit is the cumulative input+output tokens of every
	// assistant turn up to and including the one whose text, or whose tool
	// results, first contained a gold token.
	TokensToFirstHit int64 `json:"tokens_to_first_hit"`
	// ToolCallsToFirstHit counts tool_use blocks up to and including that
	// turn; ToolCalls counts them over the whole transcript.
	ToolCallsToFirstHit int `json:"tool_calls_to_first_hit"`
	ToolCalls           int `json:"tool_calls"`
	// FirstHitToken is the gold token seen first (the earliest in the
	// transcript; ties broken by ground-truth order).
	FirstHitToken string `json:"first_hit_token,omitempty"`
}

// streamEvent is the subset of a stream-json line this parser reads.
type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			// tool_use carries ID/Name; the matching tool_result carries
			// ToolUseID, which is how a payload is attributed to the tool
			// that produced it (TranscriptMetrics, gold_source_tool).
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"` // tool_result: string or []block
		} `json:"content"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// FirstGoldHit scans transcript (stream-json, one event per line) for the
// first appearance of any gold token in an assistant text block or a tool
// result, accumulating tokens and tool calls as it goes. Lines that are
// not JSON events are skipped: a transcript may carry a stray stderr line
// and still be readable. Output is a pure function of its input.
func FirstGoldHit(transcript []byte, gold []string) GoldHit {
	var hit GoldHit
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
		switch ev.Type {
		case "assistant":
			u := ev.Message.Usage
			turnTokens := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens + u.OutputTokens
			var text strings.Builder
			for _, c := range ev.Message.Content {
				if c.Type == "tool_use" {
					hit.ToolCalls++
				}
				if c.Type == "text" {
					text.WriteString(c.Text)
					text.WriteByte('\n')
				}
			}
			if !hit.Found {
				hit.TokensToFirstHit += turnTokens
				hit.ToolCallsToFirstHit = hit.ToolCalls
				if tok := firstToken(text.String(), gold); tok != "" {
					hit.Found, hit.FirstHitToken = true, tok
				}
			}
		case "user":
			if hit.Found {
				continue
			}
			var text strings.Builder
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				text.WriteString(rawText(c.Content))
				text.WriteByte('\n')
			}
			if tok := firstToken(text.String(), gold); tok != "" {
				hit.Found, hit.FirstHitToken = true, tok
			}
		}
	}
	return hit
}

// firstToken returns the gold token with the earliest index in text, or
// "" when none appears; ties keep gold order.
func firstToken(text string, gold []string) string {
	best, at := "", -1
	for _, g := range gold {
		if g == "" {
			continue
		}
		if i := strings.Index(text, g); i >= 0 && (at < 0 || i < at) {
			best, at = g, i
		}
	}
	return best
}

// rawText flattens a tool_result content field, which is either a string
// or a list of {type:"text", text:…} blocks.
func rawText(raw json.RawMessage) string {
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
		b.WriteByte('\n')
	}
	return b.String()
}
