package connector

// serviceignore.go — SERVICE-SCOPED ingest-time exclusions (the V2/V4
// rework of docs/design/scope-and-purge.md §3.5).
//
// The first build of purge appended its pattern to the shared
// ~/.fathom/ignore, which LoadIgnoreScoped unions into EVERY repo at
// Discover — so `purge --service S --all` wrote a bare `**` that destroyed
// the whole fleet index on the next refresh, and even a narrow glob
// silently stopped indexing that path in every OTHER repo too. The
// correction is structural, not a validation: an ingest-time exclusion
// written by purge lives in a PER-SERVICE file, ~/.fathom/ignore.d/<service>
// (override the directory with $FATHOM_IGNORE_D), which is loaded only when
// THAT service is discovered. A pattern in it cannot reach another service
// by construction, and the writer refuses a fleet-wide request loudly
// rather than accepting a pattern that would union everywhere.
//
// Law 13 still applies: these are user-scope machine-local files, they may
// only ever ADD exclusions (a re-include is refused exactly as it is for
// ~/.fathom/ignore), and where both scopes speak the project's own
// .fathomignore wins last.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

// ServiceIgnoreDirEnvVar overrides the per-service ignore directory — the
// FATHOM_SESSION_LEDGER pattern, so tests and sandboxes never touch the
// real ~/.fathom.
const ServiceIgnoreDirEnvVar = "FATHOM_IGNORE_D"

// ServiceIgnoreDir returns the per-service ignore directory:
// $FATHOM_IGNORE_D when set, else ~/.fathom/ignore.d, else "" when $HOME
// cannot be resolved (treated as "no service-scoped exclusions").
func ServiceIgnoreDir() string {
	if p := os.Getenv(ServiceIgnoreDirEnvVar); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".fathom", "ignore.d")
}

// validServiceIgnoreID refuses a service id that cannot safely name a file
// under the ignore directory. The empty id is the FLEET-WIDE case and gets
// its own, louder refusal in AppendServiceIgnore.
func validServiceIgnoreID(serviceID string) error {
	if strings.ContainsAny(serviceID, `/\`) || serviceID == "." || serviceID == ".." || strings.Contains(serviceID, "..") {
		return fmt.Errorf("connector: service id %q cannot name a scoped ignore file (path separators and dot segments are refused)", serviceID)
	}
	return nil
}

// ServiceIgnorePath returns the scoped ignore file for serviceID, or an
// error when the id is unsafe or no directory can be resolved.
func ServiceIgnorePath(serviceID string) (string, error) {
	if strings.TrimSpace(serviceID) == "" {
		return "", fmt.Errorf("connector: a service id is required for a scoped ignore path")
	}
	if err := validServiceIgnoreID(serviceID); err != nil {
		return "", err
	}
	dir := ServiceIgnoreDir()
	if dir == "" {
		return "", fmt.Errorf("connector: cannot resolve the service ignore directory (set %s or $HOME)", ServiceIgnoreDirEnvVar)
	}
	return filepath.Join(dir, serviceID), nil
}

// AppendServiceIgnore records ingest-time exclusions for exactly ONE
// service, deduplicating against lines already present (purge runs
// repeatedly and the file is user-visible). It returns whether anything
// was written and the file it wrote to.
//
// It REFUSES, loudly and by design:
//   - an empty service id — that would be a fleet-wide exclusion, and a
//     fleet-wide pattern written by an operation scoped to one service is
//     exactly the accident that once wiped the whole index (V2);
//   - a re-include ("!pattern") — user-scope files only ever ADD
//     exclusions (law 13);
//   - an empty pattern list.
func AppendServiceIgnore(serviceID string, patterns []string, comment string) (bool, string, error) {
	if strings.TrimSpace(serviceID) == "" {
		return false, "", fmt.Errorf(
			"connector: REFUSING a fleet-wide ingest exclusion: no service id was given, and a bare pattern in a shared ignore file " +
				"unions into every repository at Discover — scope the exclusion to one service",
		)
	}
	path, err := ServiceIgnorePath(serviceID)
	if err != nil {
		return false, "", err
	}
	var clean []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "!") {
			return false, path, fmt.Errorf("connector: %q is a re-include pattern — a scoped ignore file may only ADD exclusions (law 13)", p)
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return false, path, fmt.Errorf("connector: no patterns to record")
	}

	existing := map[string]bool{}
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			existing[strings.TrimSpace(line)] = true
		}
	} else if !os.IsNotExist(err) {
		return false, path, err
	}

	var add []string
	for _, p := range clean {
		if existing[p] {
			continue
		}
		add = append(add, p)
		existing[p] = true
	}
	if len(add) == 0 {
		return false, path, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, path, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return false, path, err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "# %s: %s\n", time.Now().UTC().Format(time.RFC3339), sanitizeIgnoreComment(comment))
	for _, p := range add {
		fmt.Fprintln(w, p)
	}
	if err := w.Flush(); err != nil {
		return false, path, err
	}
	return true, path, nil
}

// sanitizeIgnoreComment keeps a comment on one line so it cannot turn into
// an accidental pattern.
func sanitizeIgnoreComment(comment string) string {
	return strings.Join(strings.Fields(comment), " ")
}

// serviceIgnoreLines reads serviceID's scoped ignore file — nil lines and
// zero count for an empty id, an unsafe id, or a missing file.
func serviceIgnoreLines(serviceID string) ([]string, int, string) {
	if strings.TrimSpace(serviceID) == "" || validServiceIgnoreID(serviceID) != nil {
		return nil, 0, ""
	}
	path, err := ServiceIgnorePath(serviceID)
	if err != nil {
		return nil, 0, ""
	}
	lines, n := ignoreLines(path)
	return lines, n, path
}

// LoadIgnoreScopedFor unions root's project-scope .fathomignore with the
// user-scope ~/.fathom/ignore AND serviceID's own scoped exclusions
// (~/.fathom/ignore.d/<serviceID>), returning the merged Matcher (nil when
// nothing contributes a pattern) and the per-scope pattern-count split for
// disclosure. serviceID "" loads no scoped file — the LoadIgnoreScoped
// compatibility path for callers with no service in hand.
//
// A re-include in EITHER user-scope file is refused loudly
// (*ErrUserScopeReinclude): both are machine-local files that may only ever
// ADD exclusions (law 13). Project patterns are appended last, so a
// project's own negation still overrides any user-scope exclusion on
// conflict — where both scopes speak, the project wins.
func LoadIgnoreScopedFor(root, serviceID string) (*filter.Matcher, IgnoreCounts, error) {
	projectLines, projectN := ignoreLines(filepath.Join(root, IgnoreFileName))

	userPath := UserIgnorePath()
	var userLines []string
	var userN int
	if userPath != "" {
		userLines, userN = ignoreLines(userPath)
		if err := checkNoReinclude(userPath, userLines); err != nil {
			return nil, IgnoreCounts{}, err
		}
	}

	svcLines, svcN, svcPath := serviceIgnoreLines(serviceID)
	if svcN > 0 {
		if err := checkNoReinclude(svcPath, svcLines); err != nil {
			return nil, IgnoreCounts{}, err
		}
	}

	if projectN == 0 && userN == 0 && svcN == 0 {
		return nil, IgnoreCounts{}, nil
	}
	combined := make([]string, 0, len(userLines)+len(svcLines)+len(projectLines))
	combined = append(combined, userLines...)
	combined = append(combined, svcLines...)
	combined = append(combined, projectLines...)
	return filter.New(combined), IgnoreCounts{Project: projectN, User: userN, Service: svcN}, nil
}
