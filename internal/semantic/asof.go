package semantic

// asof.go — SU-19 (ADR-041 decision 7, docs/vision/31-substrate.md §3 D6):
// `as_of` accepts either an RFC3339 timestamp or a commit sha, resolved
// against the service's own commits table. One parser shared by the
// obligations MCP tool (internal/mcp/semantic_tools.go) and the `findings
// near --as-of` CLI flag (cmd/fathom/findings.go) so the two surfaces never
// grow divergent accepted forms.

import (
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// ResolveAsOf parses raw into the instant it names: an RFC3339 timestamp
// tried first, then a commit sha (full or an unambiguous >=7-char prefix)
// looked up by Hash in commits, resolved to that commit's own Date. Returns
// an error naming why neither form matched — never a silent zero time.Time.
func ResolveAsOf(raw string, commits []store.CommitInfo) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("semantic: as_of is empty")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if len(raw) < 7 {
		return time.Time{}, fmt.Errorf("semantic: as_of %q is not an RFC3339 timestamp, and too short to be a commit sha", raw)
	}
	var match *store.CommitInfo
	for i := range commits {
		if commits[i].Hash == raw || strings.HasPrefix(commits[i].Hash, raw) {
			if match != nil {
				return time.Time{}, fmt.Errorf("semantic: as_of %q matches more than one indexed commit", raw)
			}
			m := commits[i]
			match = &m
		}
	}
	if match == nil {
		return time.Time{}, fmt.Errorf("semantic: as_of %q is not an RFC3339 timestamp or a known commit sha", raw)
	}
	t, terr := time.Parse(time.RFC3339, match.Date)
	if terr != nil {
		return time.Time{}, fmt.Errorf("semantic: as_of commit %s has an unparseable date %q: %w", match.Hash, match.Date, terr)
	}
	return t.UTC(), nil
}
