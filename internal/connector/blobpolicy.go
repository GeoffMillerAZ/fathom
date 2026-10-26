package connector

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
	toml "github.com/pelletier/go-toml/v2"
)

// BlobCorpusPolicy applies the connector's path gates to a committed tree,
// without reading a worktree or caching policy from another commit.
type BlobCorpusPolicy struct {
	ignore  *filter.Matcher
	include map[string]bool
}

// NewBlobCorpusPolicy is pure. Project rules come from the blob's commit;
// scopedPatterns comes from ScopedIgnorePatterns and retains ingest's ordering.
func NewBlobCorpusPolicy(projectIgnore, projectConfig []byte, scopedPatterns []string) BlobCorpusPolicy {
	patterns := append([]string(nil), scopedPatterns...)
	patterns = append(patterns, strings.Split(string(projectIgnore), "\n")...)
	p := BlobCorpusPolicy{ignore: filter.New(patterns), include: map[string]bool{}}
	var doc struct {
		Connector struct {
			Include []string `toml:"include"`
		} `toml:"connector"`
	}
	if len(projectConfig) <= 1<<20 && toml.Unmarshal(projectConfig, &doc) == nil {
		for _, entry := range doc.Connector.Include {
			// Committed includes are repo-relative paths, as in loadConnectorConfig.
			entry = path.Clean(entry)
			if entry != "." && !path.IsAbs(entry) && entry != ".." && !strings.HasPrefix(entry, "../") {
				p.include[entry] = true
			}
		}
	}
	return p
}

// Exclusion returns Discover's reason for excluding rel, or an empty string.
func (p BlobCorpusPolicy) Exclusion(rel string) string {
	return CorpusPathExclusion(rel, p.ignore, p.include)
}
