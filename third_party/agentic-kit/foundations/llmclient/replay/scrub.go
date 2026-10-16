package llmreplay

import (
	"sync"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	prellmdlpgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/pre-llm-dlp-gate"
)

// Scrubber transforms an Entry before it is appended to the cassette. It runs
// on every entry RecordingClient writes; the returned entry is what gets
// persisted. Scrubbers must be pure functions of their input (no shared
// mutable state) — RecordingClient may call them from its stream-forwarding
// goroutine.
type Scrubber func(Entry) Entry

// defaultScrubber compiles the shipped default gate exactly once, on first
// use.
var defaultScrubber = sync.OnceValue(func() Scrubber {
	return NewGateScrubber(prellmdlpgate.MustDefaultGate())
})

// DefaultScrubber is the scrubber RecordingClient applies unless overridden:
// pre-llm-dlp-gate's MustDefaultGate redaction over all recorded text
// content. It scrubs the request's Prompt and System, every message's text
// and tool_result block content, the response's Text and text-bearing blocks,
// every chunk's TextDelta and block content, and recorded error strings.
//
// Honesty contract (see the package doc, "Cassettes are sensitive
// artifacts"): this is best-effort secret-SHAPE redaction. Per-delta scanning
// misses secrets split across chunk boundaries; tool_use Input, BlockUnknown
// Raw, and ToolDef contents are not scanned at all. Review cassettes before
// sharing them.
func DefaultScrubber(e Entry) Entry {
	return defaultScrubber()(e)
}

// NewGateScrubber returns a Scrubber that applies g's redaction policy to all
// recorded text content — the same coverage as DefaultScrubber, with a
// caller-chosen policy (e.g. prellmdlpgate.ExtendedPolicy for the larger
// shipped catalog, or an org-specific RedactPolicy).
func NewGateScrubber(g *prellmdlpgate.Gate) Scrubber {
	redact := func(s string) string {
		if s == "" {
			return s
		}
		out, _ := g.Scan(s)
		return out
	}
	return func(e Entry) Entry {
		e.Request = scrubRequest(e.Request, redact)
		if e.Response != nil {
			r := scrubResponse(*e.Response, redact)
			e.Response = &r
		}
		if len(e.Chunks) > 0 {
			chunks := make([]Chunk, len(e.Chunks))
			for i, c := range e.Chunks {
				chunks[i] = scrubChunk(c, redact)
			}
			e.Chunks = chunks
		}
		e.Err = redact(e.Err)
		return e
	}
}

// scrubRequest redacts the text-bearing fields of a request: Prompt, System,
// and the text/tool_result content of every message block. Tools (schemas,
// descriptions) and raw-JSON block payloads are NOT scanned — see the package
// doc for why.
func scrubRequest(req llmclient.Request, redact func(string) string) llmclient.Request {
	req.Prompt = redact(req.Prompt)
	req.System = redact(req.System)
	if len(req.Messages) > 0 {
		msgs := make([]llmclient.Message, len(req.Messages))
		for i, m := range req.Messages {
			msgs[i] = llmclient.Message{Role: m.Role, Content: scrubBlocks(m.Content, redact)}
		}
		req.Messages = msgs
	}
	return req
}

// scrubResponse redacts the response's Text and its text-bearing blocks.
func scrubResponse(r llmclient.Response, redact func(string) string) llmclient.Response {
	r.Text = redact(r.Text)
	r.Content = scrubBlocks(r.Content, redact)
	return r
}

// scrubChunk redacts a recorded chunk's text delta, block content, and error
// message. Deltas are scanned individually: secrets split across delta
// boundaries survive — documented loudly in the package doc.
func scrubChunk(c Chunk, redact func(string) string) Chunk {
	c.TextDelta = redact(c.TextDelta)
	if c.Block != nil {
		b := scrubBlock(*c.Block, redact)
		c.Block = &b
	}
	c.Err = redact(c.Err)
	return c
}

// scrubBlocks redacts each block in a fresh slice, leaving the input alone.
func scrubBlocks(blocks []llmclient.ContentBlock, redact func(string) string) []llmclient.ContentBlock {
	if len(blocks) == 0 {
		return blocks
	}
	out := make([]llmclient.ContentBlock, len(blocks))
	for i, b := range blocks {
		out[i] = scrubBlock(b, redact)
	}
	return out
}

// scrubBlock redacts the plain-text fields of a block: Text (BlockText) and
// Content (BlockToolResult). Input and Raw are raw JSON and are left
// untouched — redacting inside structured JSON could corrupt it.
func scrubBlock(b llmclient.ContentBlock, redact func(string) string) llmclient.ContentBlock {
	b.Text = redact(b.Text)
	b.Content = redact(b.Content)
	return b
}
