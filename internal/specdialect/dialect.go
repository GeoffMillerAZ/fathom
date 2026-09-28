// Package specdialect is the spec-dialect port (vision 38, ADR-049,
// docs/design/spec-interop.md §3): every foreign spec format — Spec Kit's
// tasks.md, OpenSpec's change tasks, and later Kiro, BMAD, Task Master and
// GitHub task lists — is read IN PLACE and mapped into the canonical model,
// plan grammar v2's verify.Plan, with each task's position in the foreign
// file kept so a verdict points back at the author's own line.
//
// Read in place, never rewrite: a dialect's Parse is pure over the bytes it
// is handed, and the only writer in this package (WriteFixups) touches
// nothing outside the fathom converge markers (law 15).
//
// Detection is by path and content shape, never by a model, and walks only
// the paths each dialect names (design §7): Detect is handed a file list and
// reads at most the candidates it matches.
package specdialect

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Span is where one task's checkbox sits in its foreign file: the task id
// (the same id its verify.Task carries) and the 1-based line and column of
// the checkbox's opening bracket.
type Span struct {
	TaskID string
	Line   int
	Col    int
}

// SpecDialect is one foreign spec format's adapter (design §3).
type SpecDialect interface {
	// ID names the dialect: "fathom-v2", "speckit", "openspec", ….
	ID() string
	// Detect returns, sorted, the candidate spec files among files
	// (slash-separated, relative to root) this dialect claims. It may read
	// a candidate under root to check its content shape and reads nothing
	// else.
	Detect(root string, files []string) []string
	// Claims reports whether this dialect owns the file at path (any
	// separator, relative or absolute) given its content — the dispatch
	// question `fathom verify --plan <path>` asks.
	Claims(path string, content []byte) bool
	// Parse maps the foreign file into the canonical model. Pure: no I/O.
	// Every returned Task's Line is its line in the foreign file, and the
	// spans list each task's checkbox position in task order.
	Parse(path string, content []byte) (verify.Plan, []Span, error)
}

// NativeID is the canonical dialect's id: plan grammar v2 itself.
const NativeID = "fathom-v2"

// Registry holds the dialects `fathom verify` can dispatch to. The zero
// value is unusable; build one with NewRegistry or Default.
type Registry struct {
	byID map[string]SpecDialect
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{byID: map[string]SpecDialect{}} }

// RegisterSpecDialect adds d to r. An empty id, or an id already
// registered, is refused — dispatch must never depend on registration
// order.
func RegisterSpecDialect(r *Registry, d SpecDialect) error {
	if d == nil || strings.TrimSpace(d.ID()) == "" {
		return fmt.Errorf("specdialect: a dialect needs a non-empty id")
	}
	if _, dup := r.byID[d.ID()]; dup {
		return fmt.Errorf("specdialect: dialect %q is already registered", d.ID())
	}
	r.byID[d.ID()] = d
	return nil
}

// Default is the registry `fathom verify` uses: the native grammar plus
// every shipped foreign dialect (wave 1: Spec Kit and OpenSpec; wave 4:
// Kiro, GitHub task lists). GitHub task lists never auto-claim a file
// (ghTaskListDialect.Detect/Claims): it is registered so `--dialect
// gh-tasklist` resolves, not so ForFile/Detect ever picks it.
func Default() *Registry {
	r := NewRegistry()
	for _, d := range []SpecDialect{Native(), SpecKit(), OpenSpec(), Kiro(), GHTaskList()} {
		if err := RegisterSpecDialect(r, d); err != nil {
			panic(err) // a programming error in the static list above
		}
	}
	return r
}

// IDs returns every registered dialect id, sorted.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Lookup returns the dialect registered under id.
func (r *Registry) Lookup(id string) (SpecDialect, bool) {
	d, ok := r.byID[id]
	return d, ok
}

// foreignFirst is the dispatch order: every foreign dialect in id order,
// then the native grammar last — the native parser reads any Markdown
// checklist, so it is the fallback, never the first claimant.
func (r *Registry) foreignFirst() []SpecDialect {
	var out []SpecDialect
	for _, id := range r.IDs() {
		if id != NativeID {
			out = append(out, r.byID[id])
		}
	}
	if n, ok := r.byID[NativeID]; ok {
		out = append(out, n)
	}
	return out
}

// ForFile returns the dialect that claims path given its content: the
// first foreign dialect (in id order) whose Claims holds, else the native
// grammar when registered. ok is false when nothing claims it.
func (r *Registry) ForFile(path string, content []byte) (SpecDialect, bool) {
	for _, d := range r.foreignFirst() {
		if d.Claims(path, content) {
			return d, true
		}
	}
	return nil, false
}

// Detection is one detected spec file and the dialect that claims it.
type Detection struct {
	Path    string
	Dialect string
}

// Detect runs every dialect's Detect over files (relative to root) and
// returns one detection per claimed file, sorted by path. A file two
// dialects both detect goes to the first in dispatch order (foreign
// dialects by id, native last), so the answer never depends on map order.
func (r *Registry) Detect(root string, files []string) []Detection {
	owner := map[string]string{}
	for _, d := range r.foreignFirst() {
		for _, p := range d.Detect(root, files) {
			if _, taken := owner[p]; !taken {
				owner[p] = d.ID()
			}
		}
	}
	out := make([]Detection, 0, len(owner))
	for p, id := range owner {
		out = append(out, Detection{Path: p, Dialect: id})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// slash normalises a path to forward slashes for shape matching.
func slash(p string) string { return filepath.ToSlash(p) }

// detectBy is the shared Detect body: the files whose slash path matches
// pathOK, read under root only when shapeOK is non-nil, kept when the
// content has the dialect's shape. Sorted, deduplicated.
func detectBy(root string, files []string, pathOK func(string) bool, shapeOK func([]byte) bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		p := slash(f)
		if seen[p] || !pathOK(p) {
			continue
		}
		if shapeOK != nil {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			if err != nil || !shapeOK(b) {
				continue
			}
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// pathSegments splits a slash path into its segments.
func pathSegments(p string) []string { return strings.Split(strings.Trim(slash(p), "/"), "/") }

// hasSegmentsSuffix reports whether p ends with the pattern segments, where
// "*" matches exactly one segment: ("specs","*","tasks.md") matches
// "specs/001-x/tasks.md" and "/abs/repo/specs/001-x/tasks.md".
func hasSegmentsSuffix(p string, pattern ...string) bool {
	segs := pathSegments(p)
	if len(segs) < len(pattern) {
		return false
	}
	tail := segs[len(segs)-len(pattern):]
	for i, want := range pattern {
		if want != "*" && tail[i] != want {
			return false
		}
	}
	return true
}
