package intent

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// util.go — small deterministic helpers. Every list this package returns is
// sorted or in file order; nothing here may depend on map iteration.

func splitLines(s string) []string { return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") }

func itoa(i int) string { return strconv.Itoa(i) }

func sortStrings(s []string) { sort.Strings(s) }

func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

// isTableRule reports whether a table line is the |---|---| separator.
func isTableRule(t string) bool {
	body := strings.Trim(t, "|")
	if strings.TrimSpace(body) == "" {
		return false
	}
	for _, r := range body {
		if r != '-' && r != ':' && r != '|' && r != ' ' {
			return false
		}
	}
	return true
}

// tableCells splits a markdown table row, honouring escaped pipes and pipes
// inside code spans (an invariant statement routinely writes `a|b`).
func tableCells(t string) []string {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	var (
		cells   []string
		cur     strings.Builder
		inCode  bool
		escaped bool
	)
	for _, r := range t {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '`':
			inCode = !inCode
			cur.WriteRune(r)
		case r == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells
}

// backtickTokens returns the backticked tokens of a cell — the router's
// globs are written that way, and only that way, so prose in the same cell
// never becomes a glob.
func backtickTokens(cell string) []string {
	var out []string
	for _, m := range backtickRe.FindAllStringSubmatch(cell, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" || strings.ContainsAny(tok, " ") {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// stripEmphasis removes backticks and bold markers so a rung cell can be
// classified on its words rather than its typography.
func stripEmphasis(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

// uniqueStrings sorts and de-duplicates, so every list is deterministic.
func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

var wordRe = regexp.MustCompile(`[A-Za-z0-9]+`)

// slugWords builds a stable id fragment from a non-goal's title: the first
// n words, lower-cased. A retitled non-goal mints a new id, which is
// honest — the old condition is gone.
func slugWords(s string, n int) string {
	words := wordRe.FindAllString(stripEmphasis(s), -1)
	if len(words) > n {
		words = words[:n]
	}
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "-")
}

// firstExistingPointer returns the first backticked path in a router row
// that resolves against the repo — how the docs-map names a home.
func firstExistingPointer(root, routerRel, row string) string {
	base := filepath.Dir(filepath.Join(root, routerRel))
	for _, m := range backtickRe.FindAllStringSubmatch(row, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" || strings.ContainsAny(tok, " *<") {
			continue
		}
		cand := filepath.Clean(filepath.Join(base, tok))
		if p, err := filepath.Rel(root, cand); err == nil && !strings.HasPrefix(p, "..") {
			if statExists(cand) {
				return filepath.ToSlash(p)
			}
		}
	}
	return ""
}

func statExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// statPresence tells a path that is there from one that genuinely is not
// and from one the reader cannot see (ADR-027, I38): blind is non-nil only
// for the last — EACCES/EPERM on a directory it cannot search — and is the
// reader's own error. statExists folds all three into two, which is how an
// unreadable directory's files became "not in this tree".
func statPresence(p string) (present bool, blind error) {
	_, err := os.Stat(p)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return false, nil
	}
	return false, err
}

// errnoOf is the operating system's wording for a failed read or listing,
// the path wrapper stripped ("permission denied") — the path is named
// beside it.
func errnoOf(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}

// recordBearingPointer picks the pointer in a docs-map row that actually
// holds decision records. Every resolving pointer is scored: a directory of
// record files, or a file that parses into `## ADR-NNN` sections, beats an
// index that merely lists them. Ties fall to document order.
func recordBearingPointer(root, routerRel, row string) string {
	base := filepath.Dir(filepath.Join(root, routerRel))
	var fallback string
	for _, m := range backtickRe.FindAllStringSubmatch(row, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" || strings.ContainsAny(tok, " *<") {
			continue
		}
		cand := filepath.Clean(filepath.Join(base, tok))
		relPath, err := filepath.Rel(root, cand)
		if err != nil || strings.HasPrefix(relPath, "..") {
			continue
		}
		info, err := os.Stat(cand)
		if err != nil {
			continue
		}
		if fallback == "" {
			fallback = filepath.ToSlash(relPath)
		}
		if info.IsDir() {
			if files, _ := filepath.Glob(filepath.Join(cand, "*.md")); len(files) > 1 {
				return filepath.ToSlash(relPath)
			}
			continue
		}
		if body, err := os.ReadFile(cand); err == nil && adrSectionRe.Match(body) {
			return filepath.ToSlash(relPath)
		}
	}
	return fallback
}

// adrSectionRe matches the single-file record heading.
var adrSectionRe = regexp.MustCompile(`(?m)^##\s+ADR-\d+`)
