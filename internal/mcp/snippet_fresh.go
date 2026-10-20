package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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
