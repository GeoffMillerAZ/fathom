// Package endpoints aggregates routes.Route slices into a deduplicated,
// deterministically ordered Endpoint inventory for a single service.
package endpoints

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/analysis/routes"
)

// Endpoint represents a unique HTTP method+path combination across all files
// in a service, with metadata about where the registration was found.
type Endpoint struct {
	Method   string   // upper-cased HTTP method
	Path     string   // URL path literal
	Handlers []string // deduplicated handler names, sorted; may contain "" for anonymous
	Files    []string // source files where this method+path was seen, sorted and deduplicated
	Lines    []int    // corresponding line numbers (parallel to Files; first occurrence per file)
}

// key is the canonical deduplification key for an endpoint.
func key(r routes.Route) string {
	return r.Method + ":" + r.Path
}

// Inventory aggregates a flat slice of routes into a deduplicated []Endpoint.
// Routes with the same (Method, Path) are merged; Files, Handlers, and Lines
// are collected, deduplicated, and sorted. The returned slice is sorted by
// (Method, Path) for determinism.
func Inventory(rts []routes.Route) []Endpoint {
	type accumulator struct {
		method   string
		path     string
		handlers map[string]bool
		files    map[string]int // file -> first line seen
	}

	order := make([]string, 0)
	byKey := make(map[string]*accumulator)

	for _, r := range rts {
		k := key(r)
		acc, ok := byKey[k]
		if !ok {
			acc = &accumulator{
				method:   strings.ToUpper(r.Method),
				path:     r.Path,
				handlers: make(map[string]bool),
				files:    make(map[string]int),
			}
			byKey[k] = acc
			order = append(order, k)
		}
		// Collect handler (including empty string for anonymous handlers).
		acc.handlers[r.Handler] = true
		// Record first line seen per file.
		if _, seen := acc.files[r.File]; !seen {
			acc.files[r.File] = r.Line
		}
	}

	out := make([]Endpoint, 0, len(byKey))
	for _, k := range order {
		acc := byKey[k]

		// Build sorted handlers slice.
		handlers := make([]string, 0, len(acc.handlers))
		for h := range acc.handlers {
			handlers = append(handlers, h)
		}
		sort.Strings(handlers)

		// Build sorted files + parallel lines slices.
		files := make([]string, 0, len(acc.files))
		for f := range acc.files {
			files = append(files, f)
		}
		sort.Strings(files)
		lines := make([]int, len(files))
		for i, f := range files {
			lines[i] = acc.files[f]
		}

		out = append(out, Endpoint{
			Method:   acc.method,
			Path:     acc.path,
			Handlers: handlers,
			Files:    files,
			Lines:    lines,
		})
	}

	// Final sort by (Method, Path) for full determinism regardless of input order.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Method != out[j].Method {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
}
