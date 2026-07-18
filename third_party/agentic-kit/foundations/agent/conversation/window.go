package conversation

import (
	"context"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// WindowCtx returns a suffix of the conversation whose estimated token cost
// fits budget, plus the number of original messages dropped, and any error
// returned by Options.Compactor. The result is always a sequence the API
// will accept:
//
//   - It is a true suffix of the conversation (optionally preceded by the
//     PreserveHead head and a compaction summary; see below) — never a
//     re-ordering.
//   - An assistant tool_use message and its paired user tool_result message
//     are never split: cut points are only ever "clean" user messages that
//     carry no tool_result blocks, so the result never starts with (or
//     orphans) a tool_result message.
//   - At least the most recent exchange — from the last clean user message
//     to the end — is always kept, even when it alone exceeds budget.
//     Budgets <= 0 therefore yield exactly that minimum window.
//
// Selection: per-message costs come from the estimator (see TokenEstimate;
// the estimate carries the Options.Estimator error band, so budget must be
// set with margin). If the whole conversation fits, everything is returned
// and dropped is 0. Otherwise WindowCtx keeps the EARLIEST clean cut that
// fits — it drops as little as possible — falling back to the latest clean
// cut when nothing fits. If no clean cut exists after the head, everything
// is returned rather than producing an invalid sequence.
//
// PreserveHead: the first Options.PreserveHead messages are always kept and
// their cost counts against budget. If the head boundary would split an
// assistant tool_use from its tool_result message, the head is extended by
// one message to keep the pair. A head covering the whole conversation
// disables windowing.
//
// Compaction and the seam rule: when messages are dropped and
// Options.Compactor returns a summary (ok=true, role-neutral content — see
// Options.Compactor), the summary is spliced in at the cut point with its
// role chosen by the package so that strict alternation survives:
//
//   - Head ends with a USER message: the summary becomes a standalone
//     ASSISTANT message between head and suffix (user → assistant(summary)
//     → user is the only standalone slot that alternates).
//   - No head, or head ends with an ASSISTANT message: there is no
//     standalone slot (the suffix starts with user, and a leading or
//     assistant-adjacent summary message would break alternation or the
//     first-message-is-user rule), so the summary's content blocks are
//     PREPENDED to the suffix's first user message.
//
// Compactor error: if the Compactor returns a non-nil error, compaction is
// aborted — the window is returned over-budget but intact (no messages are
// lost) and the error is surfaced to the caller. The caller must handle this
// explicitly: the conversation is safe, just larger than requested.
//
// Without a usable summary, a head ending in a user message would sit
// directly next to the suffix's first user message; WindowCtx repairs that
// seam by merging the two user messages into one (head content first, so
// any tool_result blocks keep their results-first position). The summary's
// own cost is NOT counted against budget.
//
// The returned messages are copies at the depth documented on Messages;
// dropped counts original messages removed (the summary is not counted).
func (c *Conversation) WindowCtx(ctx context.Context, budget int) (msgs []llmclient.Message, dropped int, err error) {
	n := len(c.messages)
	if n == 0 {
		return nil, 0, nil
	}

	head := c.opts.PreserveHead
	if head < 0 {
		head = 0
	}
	if head > n {
		head = n
	}
	// Never split an assistant tool_use from its tool_result message: if
	// the head ends on an assistant message carrying tool_use blocks, pull
	// the paired user message into the head.
	if head > 0 && head < n && hasToolUse(c.messages[head-1]) {
		head++
	}

	costs := make([]int, n)
	total := 0
	for i, m := range c.messages {
		costs[i] = c.messageCost(m)
		total += costs[i]
	}
	if total <= budget || head >= n {
		return copyMessages(c.messages), 0, nil
	}

	headCost := 0
	for _, ct := range costs[:head] {
		headCost += ct
	}
	// suffixCost[i] = cost of messages[i:].
	suffixCost := make([]int, n+1)
	for i := n - 1; i >= 0; i-- {
		suffixCost[i] = suffixCost[i+1] + costs[i]
	}

	// Candidate cut points: clean user messages strictly after the head.
	// (Cutting at the head itself drops nothing, which the full-fit check
	// above already covered.)
	var candidates []int
	for s := head + 1; s < n; s++ {
		m := c.messages[s]
		if m.Role == llmclient.RoleUser && !hasToolResult(m) {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		// No valid place to cut; return everything rather than an invalid
		// sequence.
		return copyMessages(c.messages), 0, nil
	}

	chosen := candidates[len(candidates)-1] // fallback: most recent exchange
	for _, s := range candidates {
		if headCost+suffixCost[s] <= budget {
			chosen = s // earliest fitting cut drops the least
			break
		}
	}
	dropped = chosen - head

	result := make([]llmclient.Message, 0, head+1+(n-chosen))
	for _, m := range c.messages[:head] {
		result = append(result, copyMessage(m))
	}
	tail := copyMessages(c.messages[chosen:])

	summary, ok, compactErr := c.summarize(ctx, c.messages[head:chosen])
	if compactErr != nil {
		// Compactor failed: return over-budget but intact window — never lose messages.
		// Rebuild the full window (head + all dropped messages + tail) so nothing is lost.
		full := make([]llmclient.Message, 0, n)
		for _, m := range c.messages[:head] {
			full = append(full, copyMessage(m))
		}
		full = append(full, copyMessages(c.messages[head:chosen])...)
		full = append(full, tail...)
		return full, 0, compactErr
	}
	headEndsUser := head > 0 && c.messages[head-1].Role == llmclient.RoleUser
	switch {
	case ok && headEndsUser:
		// user(head end) → assistant(summary) → user(suffix start).
		result = append(result, llmclient.Message{Role: llmclient.RoleAssistant, Content: summary})
		result = append(result, tail...)
	case ok:
		// No standalone slot: ride the summary at the top of the suffix's
		// first user message.
		tail[0].Content = append(summary, tail[0].Content...)
		result = append(result, tail...)
	case headEndsUser:
		// user→user seam with no summary to bridge it: merge the two user
		// messages, head content first.
		last := len(result) - 1
		merged := make([]llmclient.ContentBlock, 0, len(result[last].Content)+len(tail[0].Content))
		merged = append(merged, result[last].Content...)
		merged = append(merged, tail[0].Content...)
		result[last].Content = merged
		result = append(result, tail[1:]...)
	default:
		result = append(result, tail...)
	}
	return result, dropped, nil
}

// Window is the legacy context-free entry point. It delegates to WindowCtx
// with context.Background(). Callers whose Compactor honours cancellation
// should call WindowCtx directly and pass their own ctx.
//
// Window panics if the Compactor returns an error (the legacy signature had
// no error return; callers that can fail must use WindowCtx). If
// Options.Compactor is nil, or the Compactor always returns (_, false, nil),
// Window never panics.
func (c *Conversation) Window(budget int) (msgs []llmclient.Message, dropped int) {
	msgs, dropped, err := c.WindowCtx(context.Background(), budget)
	if err != nil {
		panic("conversation.Window: Compactor returned error; use WindowCtx to handle compactor errors: " + err.Error())
	}
	return msgs, dropped
}

// summarize runs the Compactor over copies of the dropped messages and
// returns a fresh, role-neutral content slice for the summary. It returns
// (nil, false, nil) when no compactor is set or the compactor declines.
// It returns (nil, false, err) when the compactor signals failure — the
// caller must treat this as a lossless abort (never drop messages on error).
// Role-unsafe summaries (tool_use / tool_result / empty text blocks) are
// silently discarded as (nil, false, nil).
func (c *Conversation) summarize(ctx context.Context, droppedMsgs []llmclient.Message) ([]llmclient.ContentBlock, bool, error) {
	if c.opts.Compactor == nil || len(droppedMsgs) == 0 {
		return nil, false, nil
	}
	m, ok, err := c.opts.Compactor(ctx, copyMessages(droppedMsgs))
	if err != nil {
		return nil, false, err
	}
	if !ok || len(m.Content) == 0 {
		return nil, false, nil
	}
	for _, b := range m.Content {
		switch b.Type {
		case llmclient.BlockToolUse, llmclient.BlockToolResult:
			return nil, false, nil
		case llmclient.BlockText:
			if b.Text == "" {
				return nil, false, nil
			}
		}
	}
	content := make([]llmclient.ContentBlock, len(m.Content))
	copy(content, m.Content)
	return content, true, nil
}

// hasToolUse reports whether m contains any tool_use block.
func hasToolUse(m llmclient.Message) bool {
	for _, b := range m.Content {
		if b.Type == llmclient.BlockToolUse {
			return true
		}
	}
	return false
}

// hasToolResult reports whether m contains any tool_result block.
func hasToolResult(m llmclient.Message) bool {
	for _, b := range m.Content {
		if b.Type == llmclient.BlockToolResult {
			return true
		}
	}
	return false
}
