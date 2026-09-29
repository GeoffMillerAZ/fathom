package connector

// userignore.go — user-scope ignore defaults (docs/design/scoped-config.md
// deliverable 1, law 13): ~/.fathom/ignore UNIONs additively with every
// repo's project-scope .fathomignore. A user-scope re-include ("!pattern")
// is refused LOUDLY — it may only ever ADD exclusions, never re-include
// something a project (or fathom's own defaults) already excluded, so no
// global machine-wide file can silently pull something back into an index.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// UserIgnoreFileName is the user-scope ignore file's basename under
// ~/.fathom/.
const UserIgnoreFileName = "ignore"

// UserIgnorePath returns ~/.fathom/ignore, or "" when $HOME cannot be
// resolved (best-effort — callers treat "" as "no user-scope file").
func UserIgnorePath() string {
	return fathomhome.Path(UserIgnoreFileName)
}

// ErrUserScopeReinclude is returned when a user-scope ignore file contains
// a gitignore-style re-include pattern ("!pattern") — the one thing law 13
// (docs/design/scoped-config.md §2) forbids: "a user-scope pattern can only
// ever exclude (never re-include, so no global file can silently pull
// something back into an index)."
type ErrUserScopeReinclude struct {
	Path    string
	Line    int
	Pattern string
}

func (e *ErrUserScopeReinclude) Error() string {
	return fmt.Sprintf(
		"connector: %s:%d: %q is a re-include pattern (leading '!') — user-scope ignore files may only ADD exclusions, never re-include (docs/design/scoped-config.md law 13); remove this line or move it to the project's own .fathomignore",
		e.Path, e.Line, e.Pattern,
	)
}

// IgnoreCounts is the project/user pattern-count split for disclosure
// (docs/design/scoped-config.md §4: "excluded: 14 patterns (9 project, 5
// user-scope ~/.fathom/ignore)"). Counts are of non-blank, non-comment
// PATTERN LINES, not matched files.
type IgnoreCounts struct {
	Project int
	User    int
	// Service counts patterns from the SERVICE-SCOPED file
	// ~/.fathom/ignore.d/<service> (serviceignore.go) — purge's write-side
	// chokepoint. Zero whenever the load had no service in hand.
	Service int
}

// ignoreLines returns path's raw lines (nil when absent/unreadable) and the
// count of non-blank, non-comment pattern lines within them.
func ignoreLines(path string) ([]string, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	lines := strings.Split(string(data), "\n")
	n := 0
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		n++
	}
	return lines, n
}

// checkNoReinclude scans lines for a leading '!' pattern and returns
// *ErrUserScopeReinclude naming the first one found (1-indexed line
// number) — nil when every non-blank, non-comment line is a plain
// exclude.
func checkNoReinclude(path string, lines []string) error {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "!") {
			return &ErrUserScopeReinclude{Path: path, Line: i + 1, Pattern: t}
		}
	}
	return nil
}

// LoadIgnoreScoped unions root's project-scope .fathomignore with the
// user-scope ~/.fathom/ignore, returning the merged Matcher (nil when
// neither file contributes any pattern) and the project/user pattern-count
// split for disclosure (§4).
//
// A user-scope re-include is refused LOUDLY (*ErrUserScopeReinclude) —
// this is the enforcement point Discover uses, so a violating user-scope
// file blocks ingest outright until it is fixed, rather than being
// silently dropped or silently honored.
//
// Project patterns are appended AFTER user patterns: filter.Matcher is
// last-match-wins, so a project's own negation always overrides a
// user-scope exclusion on conflict — law 13's "where both scopes speak,
// the project wins."
func LoadIgnoreScoped(root string) (*filter.Matcher, IgnoreCounts, error) {
	return LoadIgnoreScopedFor(root, "")
}

// UserAndServiceOnlyIgnore compiles serviceID's scoped exclusions —
// ~/.fathom/ignore (user-scope) UNION ~/.fathom/ignore.d/<serviceID>
// (service-scope; purge's write-side chokepoint, serviceignore.go) —
// WITHOUT any project .fathomignore. The mirror image of ProjectOnlyIgnore:
// isolates whether a path is excluded by a scoped-ignore/purge rule
// INDEPENDENT of the project's own .fathomignore, so a caller can
// distinguish "this file is purged or user-excluded" (must still refuse)
// from "this file is merely outside the project's own .fathomignore-
// defined corpus" (a narrower refusal a caller may legitimately skip —
// e.g. review_diff's coverage_profiles carve-out for build artifacts
// under workspace/, internal/mcp/review_tool.go's applyCoverageProfiles).
//
// A re-include in either scoped file is refused loudly (*ErrUserScopeReinclude),
// exactly as LoadIgnoreScopedFor enforces it.
func UserAndServiceOnlyIgnore(serviceID string) (*filter.Matcher, error) {
	userPath := UserIgnorePath()
	var userLines []string
	if userPath != "" {
		userLines, _ = ignoreLines(userPath)
		if err := checkNoReinclude(userPath, userLines); err != nil {
			return nil, err
		}
	}
	svcLines, svcN, svcPath := serviceIgnoreLines(serviceID)
	if svcN > 0 {
		if err := checkNoReinclude(svcPath, svcLines); err != nil {
			return nil, err
		}
	}
	if len(userLines) == 0 && len(svcLines) == 0 {
		return nil, nil
	}
	combined := make([]string, 0, len(userLines)+len(svcLines))
	combined = append(combined, userLines...)
	combined = append(combined, svcLines...)
	return filter.New(combined), nil
}

// ProjectOnlyIgnore compiles root/.fathomignore alone (no user-scope
// union) — used by doctor's "why can't fathom see my code" check
// (docs/design/scoped-config.md §4) to isolate whether a USER-scope
// pattern, specifically, is what excludes an otherwise-indexable path:
// compare its Match against LoadIgnore's (union) Match for the same path.
func ProjectOnlyIgnore(root string) *filter.Matcher {
	lines, n := ignoreLines(filepath.Join(root, IgnoreFileName))
	if n == 0 {
		return nil
	}
	return filter.New(lines)
}

// DefaultUserIgnoreSeed is the machine-junk default list `fathom init
// --seed-user-ignore` offers to write (docs/design/scoped-config.md
// deliverable 4) — never auto-written, only ever printed/written when the
// flag is explicitly passed (init.go).
var DefaultUserIgnoreSeed = []string{
	".DS_Store",
	"*.wasm",
	".playwright-mcp/",
	"*.tmp",
	"*.swp",
	".idea/",
	".vscode/",
}
