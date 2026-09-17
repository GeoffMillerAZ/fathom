package custodian

// handoff.go — CC-15 (docs/plans/custodian.md): the completion-cites-
// instruments concern. The operator contract says a session that claims
// it shipped something cites the instrument that proves it — a commit, a
// plan row `fathom verify` gated, a retro artifact. Here that rule is
// mechanical: a handoff note from this repo, filed in the window, whose
// text claims shipping and cites nothing is a finding listing the item.
//
// Reads only ~/.fathom/threads.jsonl (the continuity store's own log —
// ids, kinds, text; never a transcript). The remediation is the thread's
// own verb: `fathom thread close <id> --why "<sha or row>"` — a closure
// that cites counts as the citation.
//
// What counts as a citation is deliberately narrow, because a citation
// that prose can produce by accident is no citation at all:
//
//   - a commit sha: 7–12 or 40 hex characters with at least one letter
//     AND one digit — `deadbeef` and `1234567` are words and numbers, not
//     commits; the thread store's own `th-xxxxxxxx` ids are excluded;
//   - a plan row id: `CC-14`, `IT-3`, `A1-2` — held to the rows the repo's
//     docs/plans actually carry when that directory exists, and to a
//     denylist of look-alikes (UTF-8, SHA-256, ISO-8601, RFC-…) always;
//   - the literal `fathom verify`;
//   - a retro artifact path ending in artifacts/retros: the legacy
//     in-tree workspace/artifacts/retros, or (O63 item 5) the state-dir
//     path retro.go now writes by default —
//     statedir.Resolve(root)/artifacts/retros, which for most repos lives
//     outside the tree entirely (~/.local/state/fathom/<repo-id>/…).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	// shipClaimRe is the vocabulary of a completion claim.
	shipClaimRe = regexp.MustCompile(`(?i)\b(shipped|landed|done|implemented|fixed|closed)\b`)

	// hexWordRe finds candidate commit shas; citesCommitSha then applies
	// the length and letter-and-digit rules the bare regexp cannot.
	hexWordRe = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

	// planRowIDRe finds candidate plan row ids: an upper-case prefix of
	// up to four characters (the second onward may be a digit — A1-2 is
	// a row shape this house uses) and a number. citesPlanRow decides.
	planRowIDRe = regexp.MustCompile(`\b[A-Z][A-Z0-9]{0,3}-[0-9]+\b`)

	// literalInstrumentRes are the citations that need no further
	// judgement: the verify verb and a retro artifact path.
	literalInstrumentRes = []*regexp.Regexp{
		regexp.MustCompile(`fathom verify`),
		// Matches both the legacy in-tree workspace/artifacts/retros and
		// (O63 item 5) the state-dir path retro.go now writes by default —
		// statedir.Resolve(root)/artifacts/retros, whose prefix varies
		// (~/.local/state/fathom/<repo-id>/… on most machines) but always
		// ends in "artifacts/retros".
		regexp.MustCompile(`artifacts/retros`),
	}

	// planRowDenylist is the prefixes of the hyphen-number tokens prose
	// produces that are NOT plan rows. Applied always — even when the
	// plans directory would have rejected them — so the rule is the same
	// in a repo that keeps no plans.
	planRowDenylist = map[string]struct{}{
		"UTF": {}, "SHA": {}, "ISO": {}, "RFC": {}, "TLS": {}, "IPV": {}, "ECDSA": {}, "ED": {},
	}
)

// threadIDPrefix is how cmd/fathom/thread.go mints ids: "th-" + 8 hex.
// A note that mentions another thread is not citing a commit.
const threadIDPrefix = "th-"

// threadRow is the slice of a ~/.fathom/threads.jsonl row this check
// reads (cmd/fathom/thread.go owns the full shape; the fold below mirrors
// its latest-row-wins carry-forward so a closure row keeps its item's
// text and kind).
type threadRow struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Text      string            `json:"text"`
	Pointers  map[string]string `json:"pointers,omitempty"`
	FiledAt   string            `json:"filed_at"`
	ClosedAt  string            `json:"closed_at,omitempty"`
	ClosedWhy string            `json:"closed_why,omitempty"`
}

func checkHandoffCitesInstruments(root string, c Concern, env auditEnv) Finding {
	f := Finding{Concern: c}
	days := c.Check.Days
	if days <= 0 {
		days = DefaultHandoffDays
	}
	window := fmt.Sprintf("%d day(s)", days)

	if env.threadsPath == "" {
		f.Status = StatusUnable
		f.Detail = "no thread log path resolves (no home directory and FATHOM_THREADS unset)"
		return f
	}
	data, err := os.ReadFile(env.threadsPath)
	if errors.Is(err, fs.ErrNotExist) {
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("no thread log at %s — nothing to audit (no handoff notes have been filed)", env.threadsPath)
		return f
	}
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("%s unreadable: %v", env.threadsPath, err)
		return f
	}

	repo := repoName(root)
	now := env.now().UTC()
	cutoff := now.AddDate(0, 0, -days)
	plans := loadPlanRows(root)

	inspected := 0
	var uncited []threadRow
	var undated []string
	for _, it := range foldThreadRows(data) {
		if it.Kind != "handoff-note" || it.Pointers["repo"] != repo {
			continue
		}
		filed, perr := time.Parse(time.RFC3339, it.FiledAt)
		if perr != nil {
			// An undated row cannot be placed in the window — it is not
			// judged, but it is not hidden either (law 3).
			undated = append(undated, it.ID)
			continue
		}
		if filed.Before(cutoff) || filed.After(now) {
			continue
		}
		inspected++
		if !shipClaimRe.MatchString(it.Text) {
			continue
		}
		if citesInstrument(it.Text, plans) || citesInstrument(it.ClosedWhy, plans) {
			continue
		}
		uncited = append(uncited, it)
	}
	sort.Strings(undated)
	undatedNote := ""
	if len(undated) > 0 {
		undatedNote = fmt.Sprintf("; %d handoff note(s) from this repo skipped — filed_at not RFC3339, so they cannot be placed in the window: %s",
			len(undated), strings.Join(undated, ", "))
	}

	if len(uncited) > 0 {
		ids := make([]string, 0, len(uncited))
		for _, it := range uncited {
			ids = append(ids, it.ID)
			f.Evidence = append(f.Evidence, fmt.Sprintf("%s (%s) %q", it.ID, it.FiledAt[:10], truncateText(it.Text, 72)))
		}
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d of %d handoff note(s) from this repo in the last %s claim shipped work without citing a commit, plan row, `fathom verify`, or retro artifact: %s — close each with the citation (fathom thread close <id> --why <sha|row>)%s",
			len(uncited), inspected, window, strings.Join(ids, ", "), undatedNote)
		return f
	}
	f.Status = StatusHolds
	if inspected == 0 {
		f.Detail = fmt.Sprintf("no handoff notes from this repo (%q) in the last %s%s", repo, window, undatedNote)
		return f
	}
	f.Detail = fmt.Sprintf("%d handoff note(s) from this repo in the last %s; every shipping claim cites an instrument%s", inspected, window, undatedNote)
	return f
}

// repoName is how `fathom thread note` stamps an item's repo: the
// lower-cased basename of the absolute root.
func repoName(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return strings.ToLower(filepath.Base(abs))
}

// planRows is what the audited root's docs/plans carries: whether the
// directory exists at all, and the row ids its plans name. Loaded once
// per check, so a citation is held to the repo's plans rather than to
// the shape of a token.
type planRows struct {
	present bool
	ids     map[string]struct{}
}

// loadPlanRows reads the row ids out of docs/plans/*.md and
// docs/plans/archive/*.md under root. The archive is included on
// purpose: a plan absorbed into the changelog inside the window is still
// the instrument that proved the note's claim, and a false violation on
// a freshly archived plan would teach people to ignore the custodian.
// Absent directory → present=false and the shape+denylist rule stands
// alone; an unreadable plan is skipped rather than guessed at.
func loadPlanRows(root string) planRows {
	dir := filepath.Join(root, "docs", "plans")
	if !dirExists(dir) {
		return planRows{}
	}
	out := planRows{present: true, ids: map[string]struct{}{}}
	var paths []string
	for _, pattern := range []string{filepath.Join(dir, "*.md"), filepath.Join(dir, "archive", "*.md")} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, id := range planRowIDRe.FindAllString(string(data), -1) {
			out.ids[id] = struct{}{}
		}
	}
	return out
}

// citesInstrument reports whether s carries any instrument citation.
func citesInstrument(s string, plans planRows) bool {
	if s == "" {
		return false
	}
	for _, re := range literalInstrumentRes {
		if re.MatchString(s) {
			return true
		}
	}
	return citesCommitSha(s) || citesPlanRow(s, plans)
}

// citesCommitSha finds a token shaped like a git commit reference:
// 7–12 hex characters (an abbreviated sha) or exactly 40 (a full one),
// with at least one letter and one digit — an all-letter hex word
// (deadbeef, decade) or a bare number (1234567, a ticket or a phone
// number) is prose, not a commit. A hex word that is the tail of a
// thread id (th-9cfeffd7) is a thread mention, not a commit either.
func citesCommitSha(s string) bool {
	for _, loc := range hexWordRe.FindAllStringIndex(s, -1) {
		word := s[loc[0]:loc[1]]
		n := len(word)
		if (n < 7 || n > 12) && n != 40 {
			continue
		}
		if loc[0] >= len(threadIDPrefix) && s[loc[0]-len(threadIDPrefix):loc[0]] == threadIDPrefix {
			continue
		}
		letter, digit := false, false
		for i := 0; i < n; i++ {
			if word[i] >= 'a' && word[i] <= 'f' {
				letter = true
			} else {
				digit = true
			}
		}
		if letter && digit {
			return true
		}
	}
	return false
}

// citesPlanRow finds a plan row id. The denylist always applies; when
// the audited root keeps a docs/plans directory the id must also be one
// its plans name — a token nothing in the repo's plans carries is not a
// row of them, whatever its shape.
func citesPlanRow(s string, plans planRows) bool {
	for _, id := range planRowIDRe.FindAllString(s, -1) {
		prefix := id[:strings.IndexByte(id, '-')]
		if _, deny := planRowDenylist[prefix]; deny {
			continue
		}
		if plans.present {
			if _, known := plans.ids[id]; !known {
				continue
			}
		}
		return true
	}
	return false
}

// foldThreadRows applies the thread store's fold: latest row per id wins,
// with a partial (closure) row carrying forward the fields it does not
// restate. Output sorted by filed_at then id.
func foldThreadRows(data []byte) []threadRow {
	latest := map[string]threadRow{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var it threadRow
		if json.Unmarshal([]byte(line), &it) != nil || it.ID == "" {
			continue // a malformed line is skipped, never guessed at
		}
		if prev, seen := latest[it.ID]; seen {
			if it.Text == "" {
				it.Text = prev.Text
			}
			if it.Kind == "" {
				it.Kind = prev.Kind
			}
			if it.FiledAt == "" {
				it.FiledAt = prev.FiledAt
			}
			if it.Pointers == nil {
				it.Pointers = prev.Pointers
			}
			if it.ClosedAt == "" {
				it.ClosedAt = prev.ClosedAt
			}
			if it.ClosedWhy == "" {
				it.ClosedWhy = prev.ClosedWhy
			}
		}
		latest[it.ID] = it
	}
	out := make([]threadRow, 0, len(latest))
	for _, it := range latest {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FiledAt != out[j].FiledAt {
			return out[i].FiledAt < out[j].FiledAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func truncateText(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
