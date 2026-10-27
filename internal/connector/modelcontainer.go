package connector

import (
	"bytes"
	"path"
	"strings"
)

// ClaimedModelContainer reports whether data is a model container a dialect reads
// despite IsBinary (ADR-021 decision (1), amended 2026-10-24): an OPC zip package (its
// local-file signature first) whose extension a rule lists in its Containers. Every
// other file IsBinary flags stays excluded.
func ClaimedModelContainer(rules []DialectRule, relPath string, data []byte) bool {
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return false
	}
	ext := strings.ToLower(path.Ext(relPath))
	for _, r := range rules {
		if containsFold(r.Containers, ext) {
			return true
		}
	}
	return false
}
