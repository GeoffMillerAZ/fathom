// Package chat renders the kit's conversational UI — a transcript of message
// bubbles and a composer — as deterministic, token-themed, server-rendered HTML,
// enhanced by a dependency-free vanilla script (chat.js, served via Script/Handler
// like appkit/ui/overlay and router). It is loopback-safe (D5): the markup is
// meaningful without JS, the app owns every endpoint (POST + SSE), and chat.js
// only adds compose comforts (auto-grow, Enter-to-send, draft persistence,
// autoscroll, copy) and a small streaming API the app drives from its own SSE.
//
// "Under the hood" lives in the transcript: an assistant Message can carry
// hitl.ToolCall blocks (tool use inline), Citations (sources, with a provenance
// click hook the app wires to its own drawer), and SafetyFlags (injection
// findings) — the trust surface a serious agent chat needs.
//
// The kit imports no agent framework types — callers pass plain view structs and
// pre-render rich bodies (markdown→HTML) themselves; chat escapes everything else.
//
// Files: message.go (bubbles), transcript.go (the list), composer.go (input),
// citation.go (sources + safety), chat.js (client comforts + streaming API).
package chat

import (
	"html"
	"net/http"

	_ "embed"
)

//go:embed chat.js
var chatJS string

// Script returns chat.js for inlining into a page (<script> tag).
func Script() string { return chatJS }

// Handler serves chat.js with a long-lived immutable cache header. Mount at a
// stable path, e.g. mux.Handle("GET /static/chat.js", chat.Handler()).
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(chatJS))
	})
}

func esc(s string) string { return html.EscapeString(s) }

// Role classifies a message's author for layout + styling.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)
