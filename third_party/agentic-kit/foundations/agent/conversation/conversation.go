// Package conversation maintains multi-turn LLM conversation state and owns
// the message-sequence invariants that every Anthropic-style Messages API
// consumer otherwise re-learns the hard way:
//
//   - The first message must have role user, and roles must alternate
//     strictly between user and assistant.
//   - Every tool_use block in an assistant message must be answered by
//     tool_result blocks carried in the SINGLE immediately-following user
//     message: the result set must exactly cover the tool_use IDs — no
//     missing, unknown, or duplicated IDs — and must never be split across
//     several user messages (the API rejects split or mispaired results
//     with a 400).
//   - tool_result blocks must come first in that user message, before any
//     text or other blocks.
//   - tool_use blocks appear only in assistant messages; tool_result blocks
//     only in user messages; no message (and no text block) may be empty.
//
// The append paths make those mistakes unrepresentable: a sequence of
// appends that all succeed yields a conversation the API will accept.
// Beyond appending, the package provides token estimation (TokenEstimate),
// suffix windowing with optional head preservation and compaction
// (WindowCtx/Window), and JSON snapshotting (Snapshot/Restore).
//
// Composition: imports only the standard library and the sibling foundation
// foundations/llmclient (Message/ContentBlock vocabulary). It performs no
// I/O, holds no clock, and is fully deterministic.
//
// Concurrency contract: a *Conversation is NOT safe for concurrent use —
// it is single-agent-loop state. Guard it with external synchronisation if
// shared across goroutines. All returned slices are copies (see Messages
// for the exact copy depth), so values returned before a mutation remain
// stable after it.
package conversation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// Options configures a Conversation. The zero value is usable: the default
// estimator applies, no head is preserved, and no compactor runs.
type Options struct {
	// Estimator maps a piece of text to an approximate token count. Nil
	// selects the default chars/4 heuristic (ceil(len(s)/4)).
	//
	// ACCURACY WARNING: chars/4 is calibrated for English prose. On
	// code-heavy or symbol-dense text the error band is roughly ±30% —
	// dense code, JSON, and non-Latin scripts tokenize at closer to 2–3
	// chars/token, so the heuristic UNDER-estimates them. Any windowing
	// budget derived from an estimate must carry margin: budget at no more
	// than ~70% of the true context limit, and verify against
	// llmclient.Usage.InputTokens (the ground truth) when responses come
	// back.
	Estimator func(string) int

	// PreserveHead keeps the first N messages of the conversation
	// unconditionally in every Window result — typically a primer exchange
	// that must survive truncation.
	//
	// Trade-offs (see Window for the full seam rules):
	//   - The head's token cost counts against the Window budget, so a
	//     large head squeezes the tail; a head at or beyond the budget
	//     still returns at least the head plus the most recent exchange,
	//     over budget.
	//   - If message N-1 is an assistant message containing tool_use
	//     blocks, the head is extended by one message to include the
	//     paired tool_result message — a pair is never split.
	//   - A head that ends with a USER message creates a user→user seam
	//     when messages are dropped; Window repairs it (Compactor summary
	//     inserted as an assistant message, or the two user messages
	//     merged into one). Heads that end on an assistant message (even
	//     lengths, e.g. a user/assistant primer pair) avoid any merging.
	//
	// Values <= 0 preserve nothing; values >= the conversation length
	// disable windowing (everything is preserved).
	PreserveHead int

	// Compactor, when non-nil, is invoked by WindowCtx whenever messages are
	// dropped. It receives the caller's context and copies of the dropped
	// messages (oldest first), and may return a summary message and ok=true
	// to splice a recap into the result at the cut point.
	//
	// Role rule: only the returned message's Content is used — its Role is
	// ignored and the package places the summary role-safely (see WindowCtx).
	// The content must be role-neutral: if it contains tool_use or
	// tool_result blocks, or an empty text block, the summary is discarded
	// as if ok were false. The summary's token cost is NOT counted against
	// the Window/WindowCtx budget — one more reason budgets must carry margin.
	//
	// Error semantics: a non-nil error means compaction FAILED. The package
	// never silently drops messages on a compactor error — instead compaction
	// is aborted (as if ok==false) and the error is surfaced to the
	// WindowCtx caller. The window is returned intact (over budget but lossless)
	// so the caller can decide how to proceed. A failing compactor must never
	// cause data loss; the dropped messages remain in the over-budget window.
	//
	// The legacy Window method (no context) calls WindowCtx with
	// context.Background(), so Compactors that honour context cancellation
	// should prefer WindowCtx at the call site.
	Compactor func(ctx context.Context, dropped []llmclient.Message) (llmclient.Message, bool, error)
}

// Conversation is append-only multi-turn message state. Create with New.
// Not safe for concurrent use; see the package doc.
type Conversation struct {
	opts     Options
	est      func(string) int
	messages []llmclient.Message
	// pending holds the tool_use blocks of the last assistant message that
	// have not yet been answered. Non-empty iff the last message is an
	// assistant message containing tool_use blocks.
	pending []llmclient.ContentBlock
}

// New returns an empty Conversation configured by opts. A nil
// Options.Estimator selects the default chars/4 heuristic; a negative
// PreserveHead is treated as zero.
func New(opts Options) *Conversation {
	if opts.PreserveHead < 0 {
		opts.PreserveHead = 0
	}
	est := opts.Estimator
	if est == nil {
		est = defaultEstimator
	}
	return &Conversation{opts: opts, est: est}
}

// defaultEstimator is the chars/4 heuristic: ceil(len(s)/4). See
// Options.Estimator for the error band.
func defaultEstimator(s string) int { return (len(s) + 3) / 4 }

// AppendUserText appends a user message containing the single text block s.
// It fails if s is empty (the API rejects empty text blocks), if the last
// message is already a user message, or if tool results are pending — in
// that case the next user message must carry them; use AppendToolResults
// (or Append with a message whose tool_result blocks cover the pending IDs,
// followed by text).
func (c *Conversation) AppendUserText(s string) error {
	if s == "" {
		return errors.New("conversation: empty user text (the API rejects empty text blocks)")
	}
	return c.Append(llmclient.Message{
		Role:    llmclient.RoleUser,
		Content: []llmclient.ContentBlock{llmclient.TextBlock(s)},
	})
}

// AppendAssistant appends resp as an assistant message via resp.AsMessage()
// and records its tool_use blocks as pending — until they are answered by
// AppendToolResults, no plain user text can be appended. For v1-style
// responses that carry Text but no Content blocks, a single text block is
// synthesised from resp.Text. An entirely empty response is rejected (the
// API rejects empty messages), as is appending an assistant message first
// or after another assistant message.
func (c *Conversation) AppendAssistant(resp llmclient.Response) error {
	m := resp.AsMessage()
	if len(m.Content) == 0 && resp.Text != "" {
		m.Content = []llmclient.ContentBlock{llmclient.TextBlock(resp.Text)}
	}
	return c.Append(m)
}

// AppendToolResults appends all results as ONE user message answering the
// pending tool_use blocks of the last assistant message. Every block must
// be a tool_result, and the set must EXACTLY cover the pending tool_use
// IDs: a missing, unknown, or duplicated ToolUseID fails with an error
// naming the offending IDs, and nothing is appended. Results are kept in
// caller order. Appending them as a single message is what makes the
// API-400 class "results split across user messages / mispaired results"
// unrepresentable.
func (c *Conversation) AppendToolResults(results ...llmclient.ContentBlock) error {
	if len(c.pending) == 0 {
		return errors.New("conversation: no pending tool uses to answer")
	}
	if len(results) == 0 {
		return fmt.Errorf("conversation: tool_result set must exactly cover the pending tool uses: missing tool_result for tool_use IDs %v", pendingIDs(c.pending))
	}
	for i, b := range results {
		if b.Type != llmclient.BlockToolResult {
			return fmt.Errorf("conversation: AppendToolResults block %d has type %q, want %q", i, b.Type, llmclient.BlockToolResult)
		}
	}
	return c.Append(llmclient.Message{
		Role:    llmclient.RoleUser,
		Content: append([]llmclient.ContentBlock(nil), results...),
	})
}

// Append validates and appends an arbitrary message — the advanced path for
// callers composing messages by hand (e.g. tool results followed by user
// text in the same message). It enforces every invariant in the package
// doc: role alternation, first-message-is-user, non-empty content, tool
// block placement, tool_result-first ordering, and exact tool pairing
// against the pending tool_use IDs. On error nothing is appended and the
// conversation is unchanged. Content blocks with types this package does
// not model (BlockUnknown and future inventions) pass through untouched.
func (c *Conversation) Append(m llmclient.Message) error {
	if err := c.validate(m); err != nil {
		return err
	}
	c.messages = append(c.messages, copyMessage(m))
	if m.Role == llmclient.RoleAssistant {
		c.pending = toolUseBlocks(m)
	} else {
		c.pending = nil
	}
	return nil
}

// validate checks m against the package invariants without mutating state.
func (c *Conversation) validate(m llmclient.Message) error {
	switch m.Role {
	case llmclient.RoleUser, llmclient.RoleAssistant:
	default:
		return fmt.Errorf("conversation: invalid role %q (want %q or %q)", m.Role, llmclient.RoleUser, llmclient.RoleAssistant)
	}
	if len(c.messages) == 0 {
		if m.Role != llmclient.RoleUser {
			return errors.New("conversation: first message must have role user")
		}
	} else if last := c.messages[len(c.messages)-1].Role; last == m.Role {
		return fmt.Errorf("conversation: roles must alternate: %q message cannot follow %q", m.Role, last)
	}
	if len(m.Content) == 0 {
		return errors.New("conversation: message content must be non-empty (the API rejects empty messages)")
	}
	if m.Role == llmclient.RoleAssistant {
		return c.validateAssistantBlocks(m)
	}
	return c.validateUserBlocks(m)
}

// validateAssistantBlocks checks block placement inside an assistant
// message: no tool_result blocks, no empty text blocks, and tool_use blocks
// carrying unique non-empty IDs.
func (c *Conversation) validateAssistantBlocks(m llmclient.Message) error {
	seen := make(map[string]bool)
	for i, b := range m.Content {
		switch b.Type {
		case llmclient.BlockText:
			if b.Text == "" {
				return fmt.Errorf("conversation: block %d: empty text block", i)
			}
		case llmclient.BlockToolUse:
			if b.ID == "" {
				return fmt.Errorf("conversation: block %d: tool_use block has empty ID", i)
			}
			if seen[b.ID] {
				return fmt.Errorf("conversation: block %d: duplicate tool_use ID %q", i, b.ID)
			}
			seen[b.ID] = true
		case llmclient.BlockToolResult:
			return fmt.Errorf("conversation: block %d: tool_result blocks are only valid in user messages", i)
		}
	}
	return nil
}

// validateUserBlocks checks block placement inside a user message and the
// tool pairing contract: tool_result blocks first, exactly covering the
// pending tool_use IDs (or absent when nothing is pending).
func (c *Conversation) validateUserBlocks(m llmclient.Message) error {
	var results []llmclient.ContentBlock
	sawNonResult := false
	for i, b := range m.Content {
		switch b.Type {
		case llmclient.BlockText:
			if b.Text == "" {
				return fmt.Errorf("conversation: block %d: empty text block", i)
			}
			sawNonResult = true
		case llmclient.BlockToolUse:
			return fmt.Errorf("conversation: block %d: tool_use blocks are only valid in assistant messages", i)
		case llmclient.BlockToolResult:
			if b.ToolUseID == "" {
				return fmt.Errorf("conversation: block %d: tool_result block has empty ToolUseID", i)
			}
			if sawNonResult {
				return fmt.Errorf("conversation: block %d: tool_result blocks must come first in user message content", i)
			}
			results = append(results, b)
		default:
			sawNonResult = true
		}
	}

	if len(c.pending) == 0 {
		if len(results) > 0 {
			ids := make([]string, 0, len(results))
			for _, r := range results {
				ids = append(ids, r.ToolUseID)
			}
			return fmt.Errorf("conversation: orphaned tool_result blocks (no pending tool uses) for ToolUseIDs %v", ids)
		}
		return nil
	}

	// Exact coverage of the pending tool_use IDs.
	answered := make(map[string]bool, len(c.pending))
	for _, p := range c.pending {
		answered[p.ID] = false
	}
	var unknown, dup []string
	for _, r := range results {
		done, ok := answered[r.ToolUseID]
		switch {
		case !ok:
			unknown = append(unknown, r.ToolUseID)
		case done:
			dup = append(dup, r.ToolUseID)
		default:
			answered[r.ToolUseID] = true
		}
	}
	var missing []string
	for id, done := range answered {
		if !done {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)
	if len(missing)+len(unknown)+len(dup) == 0 {
		return nil
	}
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing tool_result for tool_use IDs %v", missing))
	}
	if len(unknown) > 0 {
		parts = append(parts, fmt.Sprintf("unknown ToolUseIDs %v", unknown))
	}
	if len(dup) > 0 {
		parts = append(parts, fmt.Sprintf("duplicate tool_result for ToolUseIDs %v", dup))
	}
	return fmt.Errorf("conversation: tool_result set must exactly cover the pending tool uses: %s", strings.Join(parts, "; "))
}

// PendingToolUses returns the tool_use blocks of the last assistant message
// that have not yet been answered, in message order, or nil when none are
// pending. The returned slice is fresh; the blocks are value copies (their
// json.RawMessage Input shares bytes — treat as read-only, per Messages).
func (c *Conversation) PendingToolUses() []llmclient.ContentBlock {
	if len(c.pending) == 0 {
		return nil
	}
	out := make([]llmclient.ContentBlock, len(c.pending))
	copy(out, c.pending)
	return out
}

// HasPendingTools reports whether the last assistant message contains
// tool_use blocks that have not yet been answered.
func (c *Conversation) HasPendingTools() bool { return len(c.pending) > 0 }

// Messages returns the conversation, oldest first, safe to assign straight
// to llmclient.Request.Messages.
//
// Copy depth: the returned slice and each message's Content slice are fresh
// allocations — appending to or mutating them (or appending further turns
// to the Conversation) never affects the other side. The ContentBlock
// elements are value copies, so their string fields are independent too;
// only the json.RawMessage payloads (ContentBlock.Input and .Raw) share
// underlying bytes with the conversation. The llmclient contract already
// requires raw JSON to be echoed verbatim, so treat those bytes as
// immutable and the sharing is unobservable.
func (c *Conversation) Messages() []llmclient.Message {
	return copyMessages(c.messages)
}

// TokenEstimate returns the estimated token count of the whole
// conversation: the estimator is applied to every non-empty textual payload
// — text blocks, tool_use names and JSON inputs, tool_result contents, and
// the raw JSON of unknown blocks — and summed. It is an ESTIMATE with the
// error band documented on Options.Estimator; budget decisions made from it
// must carry margin.
func (c *Conversation) TokenEstimate() int {
	total := 0
	for _, m := range c.messages {
		total += c.messageCost(m)
	}
	return total
}

// messageCost estimates one message: the estimator applied to each
// non-empty textual payload of each block, summed. Blocks of unmodelled
// types contribute every populated textual field.
func (c *Conversation) messageCost(m llmclient.Message) int {
	total := 0
	add := func(s string) {
		if s != "" {
			total += c.est(s)
		}
	}
	for _, b := range m.Content {
		switch b.Type {
		case llmclient.BlockText:
			add(b.Text)
		case llmclient.BlockToolUse:
			add(b.Name)
			add(string(b.Input))
		case llmclient.BlockToolResult:
			add(b.Content)
		default: // BlockUnknown and future types
			add(b.Text)
			add(b.Name)
			add(string(b.Input))
			add(b.Content)
			add(string(b.Raw))
		}
	}
	return total
}

// toolUseBlocks returns copies of the tool_use blocks of m, in order.
func toolUseBlocks(m llmclient.Message) []llmclient.ContentBlock {
	var uses []llmclient.ContentBlock
	for _, b := range m.Content {
		if b.Type == llmclient.BlockToolUse {
			uses = append(uses, b)
		}
	}
	return uses
}

// pendingIDs returns the sorted IDs of pending tool_use blocks.
func pendingIDs(pending []llmclient.ContentBlock) []string {
	ids := make([]string, 0, len(pending))
	for _, b := range pending {
		ids = append(ids, b.ID)
	}
	slices.Sort(ids)
	return ids
}

// copyMessage returns a copy of m with a fresh Content slice. Blocks are
// value copies; json.RawMessage payloads share bytes (see Messages).
func copyMessage(m llmclient.Message) llmclient.Message {
	out := llmclient.Message{Role: m.Role}
	if len(m.Content) > 0 {
		out.Content = make([]llmclient.ContentBlock, len(m.Content))
		copy(out.Content, m.Content)
	}
	return out
}

// copyMessages copies msgs at the depth documented on Messages.
func copyMessages(msgs []llmclient.Message) []llmclient.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]llmclient.Message, len(msgs))
	for i, m := range msgs {
		out[i] = copyMessage(m)
	}
	return out
}
