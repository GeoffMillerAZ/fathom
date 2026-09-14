package custodian

// classify.go — the code-fact-sentence classifier (VB-28). Precision
// first: a sentence needs a CONCRETE anchor — a repo path or a declared
// Go symbol — to be classified anchored (fresh or drifted); everything
// else that merely looks code-shaped is unanchorable-code-claim, and
// anything that isn't code-shaped at all is preference. When a sentence
// is an instruction (an imperative lead — "Keep…", "Always…", "Never…")
// it is a preference regardless of what it mentions: "Keep CLAUDE.md a
// thin router" names a real path but asserts no fact about the code's
// current state.
//
// An anchor check has THREE outcomes, not two: it holds, it definitely
// fails (the path is gone, the symbol is not declared anywhere reachable),
// or it is unverifiable (a file or directory this check needed to read
// could not be read). Only a definite failure may produce
// anchored-drifted; an unverifiable check downgrades the sentence to
// unanchorable-code-claim instead — an unreadable .go file must never
// manufacture a false drift.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// imperativeLeadRe matches the instructional lead words that make a
// sentence a preference no matter what it goes on to mention.
var imperativeLeadRe = regexp.MustCompile(`^(?:Keep|Prefer|Always|Never|Ask|Write|Run|Use|Avoid|Do|Don't|Consider|Reach|Route|Treat|Build|Update|Read|Load|Check|Verify|Follow|Ensure|Make|Commit|Tell|Add|Remove|Delegate|Try|Stay|Put|Pass|Refresh|File|Record|Regenerate)\b`)

var (
	factNumberUnitRe  = regexp.MustCompile(`(?i)\b\d[\d,]*\s+(?:lines?|files?|symbols?|tools?|tests?|packages?|commits?|sentences?|edges?|rules?|repos?|repositories?)\b`)
	triggerPhraseRe   = regexp.MustCompile(`(?i)\b(?:returns|lives at|is defined in|defined in|located at|calls)\b`)
	bareFileRe        = regexp.MustCompile(`^[A-Za-z0-9_-]+\.(?:go|md|cue|ya?ml|json|sh|toml|cfg|txt)$`)
	identRe           = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	hasUpperRe        = regexp.MustCompile(`[A-Z]`)
	dotPairRe         = regexp.MustCompile(`^([A-Z][A-Za-z0-9]*)\.([A-Za-z_][A-Za-z0-9_]*)$`)
	proseDotPairRe    = regexp.MustCompile(`\b([A-Z][A-Za-z0-9]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	rootFileMentionRe = regexp.MustCompile(`\b(CLAUDE\.md|AGENTS\.md)\b`)
	lineCountClaimRe  = regexp.MustCompile("`?([A-Za-z0-9_./-]+\\.(?:go|md|cue|ya?ml|json|sh))`?[^.\\n]{0,60}?\\b(\\d[\\d,]*)\\s+lines?\\b")
)

// anchorOutcome is a check's tri-state verdict.
type anchorOutcome int

const (
	anchorHolds anchorOutcome = iota
	anchorFailed
	anchorUnverifiable
)

// anchorResult is one concrete anchor's resolution.
type anchorResult struct {
	outcome anchorOutcome
	receipt string
}

// classifySentence is the detector: given the repo root (the "index at
// HEAD" for this row — see codefacts.go's doc comment), the repo-relative
// path the sentence came from (sourceFile; "" when unknown, e.g. a
// hand-supplied test sentence, or when the source lives outside root such
// as Claude Code auto-memory), and the sentence's text, return its class
// and, for anchored-drifted or unanchorable-code-claim, the receipt.
func classifySentence(root, sourceFile, text string) (CodeFactClass, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ClassPreference, ""
	}
	if imperativeLeadRe.MatchString(text) {
		return ClassPreference, ""
	}

	shaped := false
	var checks []anchorResult
	add := func(r *anchorResult) {
		// r == nil means "path-shaped but names a repo this token's first
		// segment does not belong to" (a sibling module, an external Go
		// import path) — still code-shaped, just not ours to check, so it
		// falls through to unanchorable rather than a false drift
		// (audit.go's PointersResolve check draws the identical line).
		if r != nil {
			checks = append(checks, *r)
		}
	}

	// Path-shaped and symbol-shaped backticked tokens.
	for _, m := range backtickTokenRe.FindAllStringSubmatch(text, -1) {
		token := m[1]
		switch {
		case repoPathRe.MatchString(token), bareFileRe.MatchString(token):
			shaped = true
			add(checkPathAnchor(root, sourceFile, token))
		case dotPairRe.MatchString(token):
			shaped = true
			sub := dotPairRe.FindStringSubmatch(token)
			checks = append(checks, checkSymbolAnchor(root, sub[2]))
		case identRe.MatchString(token) && len(token) > 1 && hasUpperRe.MatchString(token):
			// A bare, all-lowercase backticked token (`references`,
			// `find_symbol`, `search`) is code-shaped but reads as a
			// tool/command/config name, not a Go identifier — Go
			// convention capitalizes exported symbols, and even an
			// unexported multi-word one carries internal caps
			// (`auditCodeFactSentences`). Requiring an uppercase letter
			// is what keeps this branch off the fathom stanza's own tool
			// names (precision first: in doubt, fall through to shaped-
			// with-no-check, i.e. unanchorable, never a false drift).
			shaped = true
			checks = append(checks, checkSymbolAnchor(root, token))
		}
	}

	// The remaining cues look only at text OUTSIDE backtick spans — a
	// backticked `ADR.md` is already resolved as a path above, and
	// without this exclusion its own "ADR.md" would ALSO match the bare
	// Type.Method scan below (ADR / md), adding a spurious failing
	// symbol check for a sentence that should classify fresh.
	bareText := backtickTokenRe.ReplaceAllString(text, "")

	// Bare (non-backticked) root file mentions.
	for _, m := range rootFileMentionRe.FindAllString(bareText, -1) {
		shaped = true
		add(checkPathAnchor(root, sourceFile, m))
	}

	// Bare (non-backticked) Type.Method mentions — but a match shaped
	// like "Word.knownext" (Config.go, Notes.md) is a filename, not a
	// method, and is checked as a path instead.
	for _, sub := range proseDotPairRe.FindAllStringSubmatch(bareText, -1) {
		shaped = true
		if bareFileRe.MatchString(sub[0]) {
			add(checkPathAnchor(root, sourceFile, sub[0]))
			continue
		}
		checks = append(checks, checkSymbolAnchor(root, sub[2]))
	}

	// A file+line-count claim is the strongest anchor: check it directly
	// and skip re-deriving a weaker path-only anchor for the same file.
	if sub := lineCountClaimRe.FindStringSubmatch(text); sub != nil {
		shaped = true
		add(checkLineCountAnchor(root, sourceFile, sub[1], sub[2]))
	} else if factNumberUnitRe.MatchString(text) {
		shaped = true
	}

	if triggerPhraseRe.MatchString(text) {
		shaped = true
	}

	if !shaped {
		return ClassPreference, ""
	}
	if len(checks) == 0 {
		return ClassUnanchorable, ""
	}

	var failReceipt, unverifiableReceipt string
	anyFailed := false
	anyUnverifiable := false
	for _, c := range checks {
		switch c.outcome {
		case anchorFailed:
			anyFailed = true
			if failReceipt == "" {
				failReceipt = c.receipt
			}
		case anchorUnverifiable:
			anyUnverifiable = true
			if unverifiableReceipt == "" {
				unverifiableReceipt = c.receipt
			}
		}
	}
	// A definite failure always wins: it is more certain and more
	// actionable than "could not fully check". But when NOTHING failed
	// outright and at least one check could not complete, the honest
	// verdict is "cannot confirm this is still true" — never a claimed
	// drift and never a claimed fresh.
	if anyFailed {
		return ClassAnchoredDrifted, failReceipt
	}
	if anyUnverifiable {
		return ClassUnanchorable, unverifiableReceipt
	}
	return ClassAnchoredFresh, checks[0].receipt
}

// resolveRepoPath tries relRaw's possible locations and reports which one
// (if any) exists, whether relRaw is foreign (its first path segment
// belongs to neither candidate root — a sibling module, an external Go
// import path — rather than a stale pointer into THIS repo), and any
// non-not-exist stat error encountered along the way (permission denied
// on an intermediate directory) — a signal that the check is
// UNVERIFIABLE, not that the path is gone.
//
// Two candidate locations, tried in different orders depending on shape:
// repo-root-relative (the house style for backticked pointers with a
// slash) and relative to sourceFile's own directory (the style
// PointersResolve also accepts). For a BARE single-segment filename
// (`CLAUDE.md`, no slash) the source file's own directory is tried FIRST
// — a nested CLAUDE.md saying "CLAUDE.md is 32 lines" means itself, not
// the root file of the same name.
func resolveRepoPath(root, sourceFile, relRaw string) (abs string, foreign bool, blocked error) {
	rel := strings.TrimSuffix(relRaw, "/")
	fileDir := ""
	// Claude Code auto-memory's display path is "~/…", outside root — its
	// claims are about the repo, not relative to itself, so it gets only
	// the root-relative resolution below, same as sourceFile == "".
	if sourceFile != "" && !filepath.IsAbs(sourceFile) && !strings.HasPrefix(sourceFile, "~") {
		fileDir = filepath.Dir(sourceFile)
	}

	bareName := !strings.Contains(rel, "/")
	if !bareName {
		first := rel[:strings.IndexByte(rel, '/')]
		if first == "." || first == ".." {
			return "", true, nil
		}
		rootFirst := dirExists(filepath.Join(root, first))
		fileFirst := fileDir != "" && dirExists(filepath.Join(root, fileDir, first))
		if !rootFirst && !fileFirst {
			return "", true, nil
		}
	}

	fromRoot := filepath.Join(root, filepath.FromSlash(rel))
	candidates := []string{fromRoot}
	if fileDir != "" {
		fromFile := filepath.Join(root, fileDir, filepath.FromSlash(rel))
		if bareName {
			candidates = []string{fromFile, fromRoot} // itself before the root's same-named file
		} else {
			candidates = append(candidates, fromFile)
		}
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, false, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			if blocked == nil {
				blocked = err
			}
		}
	}
	return "", false, blocked
}

// checkPathAnchor reports relRaw's existence, or nil when relRaw is
// foreign (see resolveRepoPath) — a token this audit should not judge at
// all, since it does not name anything under this repo.
func checkPathAnchor(root, sourceFile, relRaw string) *anchorResult {
	rel := strings.TrimSuffix(relRaw, "/")
	abs, foreign, blocked := resolveRepoPath(root, sourceFile, relRaw)
	if foreign {
		return nil
	}
	if blocked != nil {
		return &anchorResult{outcome: anchorUnverifiable, receipt: rel + ": unverifiable (" + blocked.Error() + ")"}
	}
	if abs != "" {
		return &anchorResult{outcome: anchorHolds, receipt: rel + " resolves under " + root}
	}
	return &anchorResult{outcome: anchorFailed, receipt: rel + ": no such file or directory under " + root}
}

func checkLineCountAnchor(root, sourceFile, path, claimedRaw string) *anchorResult {
	claimed := strings.ReplaceAll(claimedRaw, ",", "")
	abs, foreign, blocked := resolveRepoPath(root, sourceFile, path)
	if foreign {
		return nil
	}
	if blocked != nil {
		return &anchorResult{outcome: anchorUnverifiable, receipt: path + ": unverifiable (" + blocked.Error() + ")"}
	}
	if abs == "" {
		return &anchorResult{outcome: anchorFailed, receipt: path + ": no such file under " + root}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return &anchorResult{outcome: anchorUnverifiable, receipt: path + ": unverifiable (" + err.Error() + ")"}
	}
	actual := countLines(data)
	if itoa(actual) != claimed {
		return &anchorResult{outcome: anchorFailed, receipt: path + " is " + itoa(actual) + " lines now, not " + claimedRaw}
	}
	return &anchorResult{outcome: anchorHolds, receipt: path + " is still " + claimedRaw + " lines"}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// checkSymbolAnchor reports whether a Go declaration for name exists
// anywhere under root — a whole-repo heuristic search (func, method,
// type, const, var), not a type-checked resolution. Good enough to
// distinguish "still declared somewhere" from "renamed or deleted",
// which is the drift this row exists to catch; the receipt says so
// plainly rather than overclaiming precision it does not have.
//
// A .go file (or directory) this walk cannot read is NOT treated as
// "no declaration there": it is recorded as the reason the search is
// unverifiable, and reported as such only when nothing else in the
// tree resolved the name — a positive match elsewhere still holds
// regardless of what else was unreadable.
func checkSymbolAnchor(root, name string) anchorResult {
	if !identRe.MatchString(name) {
		return anchorResult{outcome: anchorFailed, receipt: name + ": not a checkable identifier"}
	}
	q := regexp.QuoteMeta(name)
	pats := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^func\s+` + q + `\s*[(\[]`),
		regexp.MustCompile(`(?m)^func\s*\([^)]*\)\s*` + q + `\s*\(`),
		regexp.MustCompile(`(?m)^type\s+` + q + `\b`),
		regexp.MustCompile(`(?m)^const\s+` + q + `\b`),
		regexp.MustCompile(`(?m)^var\s+` + q + `\b`),
	}
	found := false
	var blockedPath string
	var blockedErr error
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if found {
			return nil
		}
		if err != nil {
			if blockedPath == "" {
				blockedPath, blockedErr = p, err
			}
			return nil
		}
		if d.IsDir() {
			if _, skip := walkSkipDirs[d.Name()]; skip && p != root {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			if blockedPath == "" {
				blockedPath, blockedErr = p, rerr
			}
			return nil
		}
		for _, re := range pats {
			if re.Match(data) {
				found = true
				return nil
			}
		}
		return nil
	})
	if found {
		return anchorResult{outcome: anchorHolds, receipt: "`" + name + "` is declared under " + root}
	}
	if blockedPath != "" {
		return anchorResult{
			outcome: anchorUnverifiable,
			receipt: "verification of `" + name + "` is incomplete: " + blockedPath + " is unreadable (" + blockedErr.Error() + ")",
		}
	}
	return anchorResult{outcome: anchorFailed, receipt: "no declaration of `" + name + "` found under " + root}
}
