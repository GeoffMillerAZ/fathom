package fusion

// summary.go — RT-76 (docs/design/responsible-tenant.md §6): what an
// ingest's final transaction writes to file_stats and service_summary.
// Everything comes from the ingest itself — the discovered refs, the
// sizes and hashes learned while fetching — plus one lstat per file for
// mtime and inode; no file is read again.

import (
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ingestSummary builds the summary facts and file_stats rows for run.
// root is the absolute corpus root ("" for a remote corpus: no stat).
// The symbol count and source kinds are counted by the store inside the
// write transaction.
func ingestSummary(root string, run *ingestRun) (store.ServiceSummary, []store.FileStat) {
	sum := store.ServiceSummary{Bytes: 0}
	if run == nil {
		return store.ServiceSummary{Bytes: -1}, nil
	}
	files := make([]store.FileStat, 0, len(run.refs))
	excluded := map[string]int{}
	if run.defaultIgnore > 0 {
		excluded["default_ignored_dirs"] = run.defaultIgnore
	}
	for _, ref := range run.refs {
		f := store.FileStat{Path: ref.Path, Lang: string(ref.Kind)}
		if lang, ok := connector.UnsupportedLanguageForPath(ref.Path); ok {
			f.Lang = store.UnsupportedLangPrefix + lang
			if sum.UnsupportedLangs == nil {
				sum.UnsupportedLangs = map[string]int{}
			}
			sum.UnsupportedLangs[lang]++
		}
		fact, known := run.facts[ref.Path]
		if known {
			f.Size, f.ContentHash, f.Excluded = fact.size, fact.hash, fact.excluded
			if fact.excluded != "" {
				excluded[fact.excluded]++
			}
			if fact.sniffed {
				excluded[CountClassifiedBySniff]++
			}
		}
		if root != "" && !filepath.IsAbs(ref.Path) {
			if fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil {
				f.MtimeNS = fi.ModTime().UnixNano()
				f.Ino = inodeOf(fi)
				if !known || f.Size < 0 {
					f.Size = fi.Size()
				}
			}
		}
		if f.Size > 0 {
			sum.Bytes += f.Size
		}
		files = append(files, f)
	}
	sum.FileCount = len(files)
	if len(excluded) > 0 {
		sum.ExcludedCounts = excluded
	}
	return sum, files
}
