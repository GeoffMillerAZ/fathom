package conversation

import (
	"encoding/json"
	"fmt"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// snapshotVersion is the format version written by Snapshot and the only
// version accepted by Restore.
const snapshotVersion = 1

// snapshot is the JSON snapshot envelope.
type snapshot struct {
	Version  int               `json:"version"`
	Messages []snapshotMessage `json:"messages"`
}

// snapshotMessage mirrors llmclient.Message with stable JSON field names.
type snapshotMessage struct {
	Role    string          `json:"role"`
	Content []snapshotBlock `json:"content"`
}

// snapshotBlock mirrors llmclient.ContentBlock with stable JSON field names
// and omitempty on every variant field, so nil json.RawMessage payloads
// round-trip as nil rather than the JSON literal null.
type snapshotBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	CacheHint bool            `json:"cache_hint,omitempty"`
}

// Snapshot serialises the conversation's messages as JSON
// ({"version":1,"messages":[...]}). Pending-tool state is derived state —
// the unanswered tool_use blocks of the last assistant message — so it
// round-trips through the messages themselves and Restore rebuilds it
// exactly. Options (functions) are not serialised; they belong to the
// receiving Conversation.
func (c *Conversation) Snapshot() ([]byte, error) {
	s := snapshot{Version: snapshotVersion, Messages: make([]snapshotMessage, 0, len(c.messages))}
	for _, m := range c.messages {
		sm := snapshotMessage{Role: string(m.Role), Content: make([]snapshotBlock, 0, len(m.Content))}
		for _, b := range m.Content {
			sm.Content = append(sm.Content, snapshotBlock{
				Type:      string(b.Type),
				Text:      b.Text,
				ID:        b.ID,
				Name:      b.Name,
				Input:     b.Input,
				ToolUseID: b.ToolUseID,
				Content:   b.Content,
				IsError:   b.IsError,
				Raw:       b.Raw,
				CacheHint: b.CacheHint,
			})
		}
		s.Messages = append(s.Messages, sm)
	}
	return json.Marshal(s)
}

// Restore replaces the conversation's messages (and derived pending-tool
// state) with the contents of a Snapshot, keeping the receiver's Options.
// The snapshot is validated by replaying every message through the same
// invariant checks as Append, so a corrupted or hand-forged snapshot that
// encodes an API-invalid sequence is rejected. On any error the receiver is
// left unchanged.
func (c *Conversation) Restore(data []byte) error {
	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("conversation: restore: %w", err)
	}
	if s.Version != snapshotVersion {
		return fmt.Errorf("conversation: restore: unsupported snapshot version %d (want %d)", s.Version, snapshotVersion)
	}
	replay := New(c.opts)
	for i, sm := range s.Messages {
		m := llmclient.Message{Role: llmclient.Role(sm.Role)}
		if len(sm.Content) > 0 {
			m.Content = make([]llmclient.ContentBlock, 0, len(sm.Content))
			for _, sb := range sm.Content {
				m.Content = append(m.Content, llmclient.ContentBlock{
					Type:      llmclient.BlockType(sb.Type),
					Text:      sb.Text,
					ID:        sb.ID,
					Name:      sb.Name,
					Input:     sb.Input,
					ToolUseID: sb.ToolUseID,
					Content:   sb.Content,
					IsError:   sb.IsError,
					Raw:       sb.Raw,
					CacheHint: sb.CacheHint,
				})
			}
		}
		if err := replay.Append(m); err != nil {
			return fmt.Errorf("conversation: restore: message %d: %w", i, err)
		}
	}
	c.messages = replay.messages
	c.pending = replay.pending
	return nil
}
