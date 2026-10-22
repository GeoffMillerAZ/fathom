// Package discoveryptr records content-checked file ranges for handoffs.
package discoveryptr

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Pointer is evidence recorded when a lookup observed a file range.
type Pointer struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Hash  string `json:"hash"`
}

func (p Pointer) Ref() string { return core.CiteRef(p.Path, p.Start, p.End) }

// PathLike rejects shell fragments rather than guessing they are locations.
func PathLike(p string) bool {
	return p != "" && !strings.HasSuffix(p, "/") && !strings.ContainsAny(p, "\x00\r\n<>|;&{}$`,:\"'") && (strings.Contains(p, "/") || filepath.Ext(p) != "")
}

// NativeInstructions are already loaded by the supported harnesses.
func NativeInstructions(p string) bool {
	switch strings.ToUpper(filepath.Base(p)) {
	case "AGENTS.MD", "AGENTS.OVERRIDE.MD", "CLAUDE.MD", "CLAUDE.LOCAL.MD", "GEMINI.MD", "COPILOT-INSTRUCTIONS.MD":
		return true
	}
	return false
}

// Read confines even symlink targets to root and bounds local I/O. End 0
// snapshots through EOF. Returned text follows core's range hash contract.
func Read(root, path string, start, end int) (Pointer, string, bool) {
	if !filepath.IsAbs(root) || !PathLike(path) || start < 1 || end < 0 {
		return Pointer{}, "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Pointer{}, "", false
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Pointer{}, "", false
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Pointer{}, "", false
	}
	f, err := os.Open(real)
	if err != nil {
		return Pointer{}, "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return Pointer{}, "", false
	}
	data, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 2<<20 {
		return Pointer{}, "", false
	}
	lines := len(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
	if end == 0 {
		end = lines
	}
	if end > lines {
		return Pointer{}, "", false
	}
	text, ok := core.LinesText(data, start, end)
	if !ok {
		return Pointer{}, "", false
	}
	hash, ok := core.HashLines(data, start, end)
	if !ok {
		return Pointer{}, "", false
	}
	return Pointer{Path: filepath.ToSlash(rel), Start: start, End: end, Hash: hash}, text, true
}

// Current drops changed or missing ranges, without a stale placeholder.
func (p Pointer) Current(root string) (string, bool) {
	if p.Hash == "" || p.End < p.Start {
		return "", false
	}
	live, text, ok := Read(root, p.Path, p.Start, p.End)
	return text, ok && live.Hash == p.Hash && live.Path == p.Path
}
