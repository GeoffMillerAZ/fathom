package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// fileStatBackend is the optional backend capability of reading a service's
// indexed file_stats rows (the rows the ingest stat gate reads).
type fileStatBackend interface {
	FileStats(ctx context.Context, serviceID string) ([]store.FileStat, bool, error)
}

// snippetFileChanged reports whether file now differs from what the index
// read (O1450): size and mtime first, then the content fingerprint when
// those moved (a touched-but-identical file is not a change). It answers
// false, serving the snippet as before, whenever it cannot tell: a backend
// without file_stats, no row for the file, an unreadable file.
func snippetFileChanged(ctx context.Context, backend QueryInterface, service, file string) bool {
	fb, ok := backend.(fileStatBackend)
	if !ok {
		return false
	}
	rows, ok, err := fb.FileStats(ctx, service)
	if err != nil || !ok {
		return false
	}
	file = strings.TrimPrefix(filepath.ToSlash(file), "./")
	var row *store.FileStat
	for i := range rows {
		if rows[i].Path == file {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		return false
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return false
	}
	full := filepath.Join(root, filepath.FromSlash(file))
	fi, err := os.Stat(full)
	if err != nil {
		return false
	}
	if fi.Size() == row.Size && fi.ModTime().UnixNano() == row.MtimeNS {
		return false
	}
	// Stat moved: only a matching fingerprint clears the file. The row
	// carries "<epoch>:<flags>:<hash>" or nothing usable.
	parts := strings.Split(row.ContentHash, ":")
	if len(parts) != 3 || parts[2] == "" || parts[0] != fusion.ExtractorEpoch {
		return true
	}
	data, ex, err := corpus.ReadFileCapped(ctx, full, sourceReadMaxFileBytes)
	if err != nil || ex != nil {
		return true
	}
	return fusion.FingerprintHash(data) != parts[2]
}

// serveDefinitionSnippet sets result["snippet"] to def's source lines, or
// result["snippet_withheld"] when def's file changed since the index
// (O1450). symbol_context's warm answer and its cold answer (O1446) both
// call it, so a check added here reaches both (O1466). The caller has
// already applied its own withholding (snippetWithheld).
func serveDefinitionSnippet(ctx context.Context, backend QueryInterface, service string, def core.SymbolNode, result map[string]any) {
	if def.FilePath == "" || def.Line <= 0 {
		return
	}
	_, pointIdentity := backend.(sourceFingerprintBackend)
	if !pointIdentity && snippetFileChanged(ctx, backend, service, def.FilePath) {
		// The definition is the index's, the text would be the working
		// tree's; never pair them.
		why := "file changed since the index"
		if ic, _ := result["index_commit"].(string); ic != "" {
			why += " (" + ic + ")"
		}
		result["snippet_withheld"] = why + "; read_source serves the current text"
		return
	}
	rng, err := readSourceRange(ctx, backend, service, def.FilePath, def.Line, 0, symbolContextSnippetMaxLines, sourceReadMaxBytes)
	if err != nil || len(rng.Lines) == 0 {
		return
	}
	if rng.Citation.State == "stale" || rng.Citation.State == "refreshing" {
		result["snippet_withheld"] = rng.Citation.Reason
		return
	}
	snippet := make([]any, len(rng.Lines))
	for i, ln := range rng.Lines {
		snippet[i] = map[string]any{"line": ln.Line, "text": ln.Text}
	}
	result["snippet"] = snippet
	if rng.Note != "" {
		result["snippet_truncated"] = true
	}
}
