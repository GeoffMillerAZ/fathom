// Package docextract pulls plain text out of attached documents so an agent can
// read them. It normalizes a document (by media type or filename extension) into
// text that can be appended to a prompt, and caps the result so a huge file can't
// blow the context window.
//
// The core package is DEPENDENCY-FREE: it handles text-like formats (txt, csv,
// tsv, md, json, log, and anything valid-UTF8) with the standard library. Heavy
// binary formats live in opt-in subpackages so a consumer only pulls the deps it
// needs:
//
//	import (
//	    dx "github.com/geoffmilleraz/agentic-kit/kits/docextract"
//	    "github.com/geoffmilleraz/agentic-kit/kits/docextract/pdf"
//	    "github.com/geoffmilleraz/agentic-kit/kits/docextract/xlsx"
//	)
//	text, err := dx.Extract(name, mediaType, data, dx.WithHandlers(pdf.Handler, xlsx.Handler))
//
// Images are NOT handled here — they go to a vision-capable model as image
// content blocks, not extracted text.
//
// Concurrency: Extract and all built-in helpers are pure functions with no
// shared state and are safe for concurrent use.
package docextract

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// DefaultMaxChars caps extracted text by default. Override with WithMaxChars.
const DefaultMaxChars = 24000

// Handler extracts text from one family of binary documents. Match decides
// whether this handler applies (given the lower-cased extension WITH its dot,
// e.g. ".pdf", and the media type); Extract returns the plain text. Register
// handlers via WithHandlers; they are tried in order before the built-in
// text/UTF-8 fallback.
type Handler struct {
	Name    string
	Match   func(ext, mediaType string) bool
	Extract func(data []byte) (string, error)
}

// Option configures Extract.
type Option func(*config)

type config struct {
	maxChars int
	handlers []Handler
}

// WithHandlers registers binary-format handlers (e.g. pdf.Handler, xlsx.Handler),
// tried in order before the text fallback.
func WithHandlers(h ...Handler) Option {
	return func(c *config) { c.handlers = append(c.handlers, h...) }
}

// WithMaxChars overrides the truncation cap (<=0 means no cap).
func WithMaxChars(n int) Option {
	return func(c *config) { c.maxChars = n }
}

// Extract returns plain text from a document's bytes, choosing a handler by
// media type or filename extension. Registered handlers are tried first; then
// text-like formats and any valid-UTF8 payload fall through to raw text. The
// result is trimmed and clamped to the configured cap.
func Extract(name, mediaType string, data []byte, opts ...Option) (string, error) {
	c := config{maxChars: DefaultMaxChars}
	for _, o := range opts {
		o(&c)
	}
	ext := strings.ToLower(filepath.Ext(name))
	mt := strings.ToLower(strings.TrimSpace(mediaType))

	for _, h := range c.handlers {
		if h.Match != nil && h.Match(ext, mt) {
			s, err := h.Extract(data)
			if err != nil {
				return "", err
			}
			return clamp(s, c.maxChars), nil
		}
	}

	switch {
	case strings.HasPrefix(mt, "text/"), isTextExt(ext):
		return clamp(string(data), c.maxChars), nil
	default:
		// Last resort: if it looks like text (valid UTF-8, no NULs), treat it so.
		if utf8.Valid(data) && !bytes.ContainsRune(data, 0) {
			return clamp(string(data), c.maxChars), nil
		}
		return "", fmt.Errorf("docextract: unsupported document type (ext %q, media %q)", ext, mt)
	}
}

func isTextExt(ext string) bool {
	switch ext {
	case ".txt", ".csv", ".tsv", ".md", ".json", ".log", ".yaml", ".yml":
		return true
	}
	return false
}

// clamp trims whitespace and truncates to max (<=0 = unbounded).
func clamp(s string, max int) string {
	s = strings.TrimSpace(s)
	if max > 0 && len(s) > max {
		// Cut on a rune boundary so we never emit invalid UTF-8.
		cut := max
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "\n…(truncated)"
	}
	return s
}
