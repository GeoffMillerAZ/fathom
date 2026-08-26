package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cmdPlansArchive implements "plans archive <plan>" (E4-1,
// docs/plans/new-use-cases.md Phase E §E4): the governed artifact lifecycle
// — a plan may only be archived once its proof survives the cleanup. It
// refuses unless ALL THREE hold, checked in this order (the first violated
// criterion is the one reported):
//
//  1. Every row is either [x]-claimed-and-currently-verifying-Pass, or
//     [-] skipped (any disposition, or none — "op-dispositioned" reads
//     broadly: a human decided its fate one way or another). An open ([ ])
//     or in-progress ([~]) row always refuses.
//  2. Every claimed row verifies GREEN RIGHT NOW — the engine is run fresh,
//     never trusting a prior run.
//  3. Lexical absorption: every row's task ID appears somewhere in
//     CHANGELOG.md, ADR.md, or docs/changelog/*.md (the per-release
//     narrative) at a token boundary (never a bare substring — "E3-10" must
//     not absorb "E3-1"; see idAbsorbed), where a range "SH-1..6" or a
//     slash-list "IT-14/15" names every id it denotes (expandIDNotation) —
//     the plan's claims must already be recorded in the permanent history
//     before the working file that carried them disappears from the active set.
//
// Known limitation of the absorption check: it is lexical, so it cannot
// distinguish the SAME id used by two different plans — a CHANGELOG mention
// of one plan's "K-1" absolves any other plan's "K-1" too. Per-plan unique
// ID prefixes (the repo's existing convention: E3-*, B1-*, LC-*, …) are
// what keep the check meaningful; reusing a prefix across plans weakens it.
//
// On success the plan file is `git mv`d to docs/plans/archive/ and a
// tombstone line (date, HEAD sha, verify summary) is written into its
// header, then re-staged.
func cmdPlansArchive(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans archive", stderr)
	root := fs.String("root", ".", "repo root (git mv + CHANGELOG.md/ADR.md checks run relative to this)")
	service := fs.String("service", "", "service ID (required)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL (HTTP mode)")
	storePath := fs.String("store", "", "store path, .sqlite/.db (direct mode; bypasses daemon)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: plans archive requires exactly one plan argument")
		fs.Usage()
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}
	msg, err := runPlansArchive(*root, fs.Arg(0), *service, *storePath, *url)
	if err != nil {
		fmt.Fprintf(stderr, "error: refused to archive: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, msg)
	return 0
}

// runPlansArchive is cmdPlansArchive's testable core.
func runPlansArchive(root, planArg, service, storePath, urlStr string) (string, error) {
	planPath, err := resolveActivePlanPath(root, planArg)
	if err != nil {
		return "", err
	}
	relPlan := filepath.ToSlash(mustRel(root, planPath))

	// Preflight: the plan must be clean in git BEFORE anything else runs.
	// Archiving a dirty plan would sweep uncommitted edits into the index
	// via git mv and stamp the tombstone with a HEAD sha whose committed
	// content never contained them — and the tombstone IS the evidence
	// record (critic finding 5).
	if dirty, derr := gitPathDirty(root, relPlan); derr != nil {
		return "", fmt.Errorf("git status for %q: %w", relPlan, derr)
	} else if dirty {
		return "", fmt.Errorf("plan %q has uncommitted changes — commit them first so the tombstone's HEAD sha matches the archived content", relPlan)
	}

	rowSets, err := computePlanRows([]string{planPath}, service, storePath, urlStr)
	if err != nil {
		return "", fmt.Errorf("verify: %w", err)
	}
	rows := rowSets[0]

	// Criterion 1: every row claimed-and-verifying or skipped — an open or
	// in-progress row always refuses.
	var incomplete []string
	for _, row := range rows {
		if row.Task.Skipped || row.Task.Claimed {
			continue
		}
		incomplete = append(incomplete, row.Task.ID)
	}
	if len(incomplete) > 0 {
		sort.Strings(incomplete)
		return "", fmt.Errorf("not every row is [x]-claimed or [-] skipped — open/in-progress: %s",
			strings.Join(incomplete, ", "))
	}

	// Criterion 2: every claimed row verifies Pass right now.
	var failing []string
	for _, row := range rows {
		if row.Task.Skipped || !row.Task.Claimed {
			continue
		}
		if row.Verdict.Status != verify.Pass {
			failing = append(failing, row.Task.ID)
		}
	}
	if len(failing) > 0 {
		sort.Strings(failing)
		return "", fmt.Errorf("claimed row(s) do not verify green right now: %s", strings.Join(failing, ", "))
	}

	// Criterion 3: lexical absorption into CHANGELOG.md, ADR.md, or the
	// per-release narrative under docs/changelog/.
	absorbedText, err := readAbsorptionCorpus(root)
	if err != nil {
		return "", fmt.Errorf("reading CHANGELOG.md/ADR.md/docs/changelog: %w", err)
	}
	var unabsorbed []string
	for _, row := range rows {
		if row.Task.ID == "" || idAbsorbed(absorbedText, row.Task.ID) {
			continue
		}
		unabsorbed = append(unabsorbed, row.Task.ID)
	}
	if len(unabsorbed) > 0 {
		sort.Strings(unabsorbed)
		return "", fmt.Errorf("row ID(s) not lexically absorbed into CHANGELOG.md, ADR.md, or docs/changelog/*.md: %s",
			strings.Join(unabsorbed, ", "))
	}

	// All three criteria hold — move the file and write the tombstone.
	sha, shaErr := gitHEAD(root)
	if shaErr != nil {
		sha = "unknown"
	}
	summary := verifySummaryLine(rows)

	archiveDir := filepath.Join(root, "docs", "plans", "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %q: %w", archiveDir, err)
	}
	newPath := filepath.Join(archiveDir, filepath.Base(planPath))
	relNew := filepath.ToSlash(mustRel(root, newPath))

	if err := gitMv(root, relPlan, relNew); err != nil {
		return "", fmt.Errorf("git mv %q %q: %w", relPlan, relNew, err)
	}
	if err := prependTombstone(newPath, "Archived", sha, summary); err != nil {
		return "", fmt.Errorf("writing tombstone: %w", err)
	}
	// Re-stage the tombstone edit. A failure here leaves the move done but
	// the tombstone unstaged — surface it, never swallow it (the caller must
	// know the index and worktree disagree).
	if err := gitAdd(root, relNew); err != nil {
		return "", fmt.Errorf("staging tombstone edit for %q: %w", relNew, err)
	}

	return fmt.Sprintf("archived %s -> %s (%s)", relPlan, relNew, summary), nil
}

// cmdPlansRestore implements "plans restore <plan>": the E4-1 round-trip —
// moves an archived plan back to docs/plans/, unconditionally (restore is a
// recovery/audit action; the tombstone stays in the file as the historical
// record of the archival that is being reversed — vision 11's own E4 kill
// criterion, "first restored-from-archive plan reveals a criterion
// false-positive", depends on that evidence surviving the round trip).
func cmdPlansRestore(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans restore", stderr)
	root := fs.String("root", ".", "repo root (git mv runs relative to this)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: plans restore requires exactly one plan argument")
		fs.Usage()
		return 1
	}
	msg, err := runPlansRestore(*root, fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, msg)
	return 0
}

// runPlansRestore is cmdPlansRestore's testable core.
func runPlansRestore(root, planArg string) (string, error) {
	archivePath, err := resolveArchivedPlanPath(root, planArg)
	if err != nil {
		return "", err
	}
	relArchive := filepath.ToSlash(mustRel(root, archivePath))
	dst := filepath.Join(root, "docs", "plans", filepath.Base(archivePath))
	relDst := filepath.ToSlash(mustRel(root, dst))
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("restore target %q already exists", relDst)
	}
	if err := gitMv(root, relArchive, relDst); err != nil {
		return "", fmt.Errorf("git mv %q %q: %w", relArchive, relDst, err)
	}
	return fmt.Sprintf("restored %s -> %s", relArchive, relDst), nil
}

// resolveActivePlanPath resolves a user-given plan argument (bare filename,
// "docs/plans/x.md", or an absolute/CWD-relative path) against root's
// docs/plans/ directory.
func resolveActivePlanPath(root, planArg string) (string, error) {
	for _, c := range []string{
		planArg,
		filepath.Join(root, planArg),
		filepath.Join(root, "docs", "plans", planArg),
		filepath.Join(root, "docs", "plans", filepath.Base(planArg)),
	} {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("plan %q not found under %q", planArg, filepath.Join(root, "docs", "plans"))
}

// resolveArchivedPlanPath is resolveActivePlanPath's counterpart for
// docs/plans/archive/.
func resolveArchivedPlanPath(root, planArg string) (string, error) {
	for _, c := range []string{
		planArg,
		filepath.Join(root, planArg),
		filepath.Join(root, "docs", "plans", "archive", planArg),
		filepath.Join(root, "docs", "plans", "archive", filepath.Base(planArg)),
	} {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("archived plan %q not found under %q", planArg, filepath.Join(root, "docs", "plans", "archive"))
}

// idAbsorbed reports whether id occurs in text at a token boundary: the
// match may not be immediately preceded or followed by an alphanumeric
// character, so "E3-1" is not absorbed by "E3-10" (suffix collision) or
// "XE3-1" (prefix collision). The fleet's active plans already hold
// CF-1/CF-10, LC-1/LC-10, TI-1/TI-10 side by side — a bare
// strings.Contains would let any "-10" row silently absolve its "-1"
// sibling (critic finding 2).
func idAbsorbed(text, id string) bool {
	if id == "" {
		return false
	}
	for from := 0; ; {
		j := strings.Index(text[from:], id)
		if j < 0 {
			return false
		}
		start := from + j
		end := start + len(id)
		beforeOK := start == 0 || !isWordByte(text[start-1])
		afterOK := end == len(text) || !isWordByte(text[end])
		if beforeOK && afterOK {
			return true
		}
		from = start + 1
	}
}

// isWordByte reports whether c continues an identifier token ([0-9A-Za-z]).
// '-' is deliberately NOT a word byte: task IDs contain it internally
// ("E3-1"), so treating it as one would make an ID a non-boundary inside
// itself.
func isWordByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// readAbsorptionCorpus concatenates CHANGELOG.md, ADR.md, and every
// docs/changelog/*.md (the per-release narrative record the 2026-08-24
// condensation moved row ids into — CHANGELOG.md keeps the two-sentence
// lines, docs/changelog/<version>.md keeps the ids as originally written).
// Missing files are tolerated as empty — a repo without one yet just fails
// the absorption check honestly rather than erroring operationally. The
// narrative files are read in sorted order so the corpus is deterministic.
func readAbsorptionCorpus(root string) (string, error) {
	names := []string{"CHANGELOG.md", "ADR.md"}
	narrative, err := filepath.Glob(filepath.Join(root, "docs", "changelog", "*.md"))
	if err != nil {
		return "", err
	}
	sort.Strings(narrative)
	for _, p := range narrative {
		names = append(names, filepath.ToSlash(mustRel(root, p)))
	}
	var b strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	corpus := b.String()
	return corpus + expandIDNotation(corpus), nil
}

// The changelog names plan rows as durable reference points in range and
// slash-list notation ("SH-1..6", "VS-5..10", "IT-14/15", "GKE-W1-1..14")
// — CHANGELOG.md's own header promises that archive checks against them.
// A lexical match on "VS-2" never finds "VS-1..4", so the corpus is extended
// with every id such a token denotes; idAbsorbed then matches at the usual
// token boundaries. A range wider than idRangeMax is ignored rather than
// expanded (a typo like "SH-1..6000" must not absorb the world).
var (
	idRangeRe = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-)(\d+)\.\.(\d+)\b`)
	idListRe  = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-)(\d+)((?:/\d+)+)\b`)
)

const idRangeMax = 500

// expandIDNotation returns a space-separated list of every id denoted by a
// range or slash-list token in text, in document order (deterministic).
func expandIDNotation(text string) string {
	var b strings.Builder
	for _, m := range idRangeRe.FindAllStringSubmatch(text, -1) {
		lo, err1 := strconv.Atoi(m[2])
		hi, err2 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil || hi < lo || hi-lo > idRangeMax {
			continue
		}
		for i := lo; i <= hi; i++ {
			b.WriteString(" " + m[1] + strconv.Itoa(i))
		}
	}
	for _, m := range idListRe.FindAllStringSubmatch(text, -1) {
		b.WriteString(" " + m[1] + m[2])
		for _, n := range strings.Split(strings.TrimPrefix(m[3], "/"), "/") {
			b.WriteString(" " + m[1] + n)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n" + b.String() + "\n"
}

// verifySummaryLine renders the same shape of summary cmdVerify prints,
// for the tombstone.
func verifySummaryLine(rows []planRow) string {
	total, claimed, passed, failed, skipped := len(rows), 0, 0, 0, 0
	for _, row := range rows {
		if row.Task.Skipped {
			skipped++
			continue
		}
		if row.Task.Claimed {
			claimed++
			if row.Verdict.Status == verify.Pass {
				passed++
			} else {
				failed++
			}
		}
	}
	return fmt.Sprintf("%d tasks total; %d claimed (%d verified, %d failing); %d skipped",
		total, claimed, passed, failed, skipped)
}

// prependTombstone inserts a tombstone line into path's header, directly
// after its first line (conventionally the "# Title" heading — the parser
// is line-based, so inserting a non-checklist line here never affects
// verify's task count).
func prependTombstone(path, label, sha, summary string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	parts := strings.SplitN(string(data), "\n", 2)
	header := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}
	tombstone := fmt.Sprintf("> **%s** %s · HEAD `%s` · %s",
		label, time.Now().UTC().Format(time.RFC3339), sha, summary)
	newContent := header + "\n\n" + tombstone + "\n" + rest
	return os.WriteFile(path, []byte(newContent), 0o644)
}

// mustRel returns path relative to root, or path unchanged if it cannot be
// made relative (never errors — a best-effort display/git-argument helper).
func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

// gitPathDirty reports whether rel has any uncommitted state (staged,
// unstaged, or untracked — any `git status --porcelain` line at all).
func gitPathDirty(root, rel string) (bool, error) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--", rel).Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func gitHEAD(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitMv(root, src, dst string) error {
	out, err := exec.Command("git", "-C", root, "mv", src, dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func gitAdd(root, path string) error {
	out, err := exec.Command("git", "-C", root, "add", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
