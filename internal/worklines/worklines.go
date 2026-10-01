// Package worklines classifies one repository's lines of work (its local
// branches, linked worktrees and stashes) against the integration branch and
// the open rows of the plans on it (ADR-060 decision 2, vision 41 §4.1).
//
// A line is one of:
//
//   - landed: an ancestor of the integration branch, patch-equivalent to it
//     (every commit has an equivalent there, `git cherry`), or content-equal
//     (a squash merge: every path the line changed reads the same on the
//     integration branch), with a clean worktree;
//   - claims rows: its plan edits flip rows that are open on the integration
//     branch (the row ids are named);
//   - touches rows: its diff touches an open row's anchors, at the lexical tier
//     (a changed line under the row's package names its symbol, or the diff
//     changes the row's oracle path);
//   - at risk: a dirty worktree, a stash, or commits on no remote;
//   - idle: none of these, and no commit for 14 days;
//   - active: none of these, and a recent commit.
//
// A line can carry more than one fact; its class is the first that holds in
// that order, and the at-risk reasons are listed whatever the class.
//
// The package only reads (I63, P15). Every git command goes through the
// injected Runner, and for a landed line it names the command that would
// remove it instead of running it.
package worklines

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Runner runs one git command in dir and returns its standard output. The
// product passes gitro.Output, which never writes the repository it reads.
type Runner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// Class is a line's classification.
type Class string

const (
	Landed      Class = "landed"
	ClaimsRows  Class = "claims-rows"
	TouchesRows Class = "touches-rows"
	AtRisk      Class = "at-risk"
	Idle        Class = "idle"
	Active      Class = "active"
)

// Classes lists every class in precedence order.
var Classes = []Class{Landed, ClaimsRows, TouchesRows, AtRisk, Idle, Active}

// Line kinds.
const (
	KindBranch   = "branch"
	KindWorktree = "worktree" // a linked worktree on a detached HEAD, or a dirty integration checkout
	KindStash    = "stash"
)

// DefaultIdleAfter is how long a line with no other fact goes without a
// commit before it reads idle (vision 41 §4.1).
const DefaultIdleAfter = 14 * 24 * time.Hour

// Options configures ClassifyLines.
type Options struct {
	// Root is a working tree of the repository (any of its worktrees).
	Root string
	// Integration is the integration branch; "" is "main".
	Integration string
	// Run runs every git command; required.
	Run Runner
	// ReadFile reads a dirty worktree's own copy of a plan; nil is os.ReadFile.
	ReadFile func(string) ([]byte, error)
	// Now is the clock ages are measured against; nil is time.Now.
	Now func() time.Time
	// IdleAfter overrides DefaultIdleAfter when positive.
	IdleAfter time.Duration
}

// Line is one line of work and what is known about it.
type Line struct {
	Kind string `json:"kind"`
	// Name is the branch name, the stash selector (stash@{n}), or a detached
	// worktree's path.
	Name string `json:"name"`
	Tip  string `json:"tip"`
	// Worktree is the worktree the line is checked out in, if any.
	Worktree string `json:"worktree,omitempty"`
	Class    Class  `json:"class"`
	// Landed says how a landed line landed: ancestor, patch-equivalent or
	// content-equal.
	Landed string `json:"landed,omitempty"`
	// Claims are the open rows ("<plan>.md#<ID>") the line's plan edits flip.
	Claims []string `json:"claims,omitempty"`
	// Touches are the open rows whose anchors the line's diff touches.
	Touches []string `json:"touches,omitempty"`
	// Tier names how Touches was matched; always "lexical" today.
	Tier string `json:"tier,omitempty"`
	// AtRisk lists why the line could be lost: a dirty worktree, a stash,
	// commits on no remote.
	AtRisk []string `json:"at_risk,omitempty"`
	// Ahead counts the line's commits that are not on the integration branch.
	Ahead int `json:"ahead"`
	// CommittedAt is the tip commit's committer date.
	CommittedAt time.Time `json:"committed_at"`
	AgeDays     int       `json:"age_days"`
	// Removal is the command that would remove a landed line. Fathom never
	// runs it (I63).
	Removal string `json:"removal,omitempty"`
}

// Report is one repository's lines.
type Report struct {
	Root           string        `json:"root"`
	Integration    string        `json:"integration"`
	IntegrationTip string        `json:"integration_tip"`
	OpenRows       int           `json:"open_rows"`
	Lines          []Line        `json:"lines"`
	Counts         map[Class]int `json:"counts"`
}

// openRow is one row open on the integration branch.
type openRow struct {
	key     string // "<plan>.md#<ID>"
	symbol  string // last identifier of Expect.Symbol
	pkg     string // Expect.Package, slash-separated, no trailing slash
	oracle  string // OraclePath
	planRel string
	id      string
}

type classifier struct {
	ctx       context.Context
	opt       Options
	root      string
	integ     string
	tip       string
	now       time.Time
	idleAfter time.Duration
	open      []openRow
	openByKey map[string]map[string]bool // plan path → open ids
	mainWT    string                     // the main worktree's path
}

// ClassifyLines classifies every local branch, linked worktree and stash of
// the repository at opt.Root.
func ClassifyLines(ctx context.Context, opt Options) (Report, error) {
	if opt.Run == nil {
		return Report{}, errors.New("worklines: no git runner")
	}
	if opt.ReadFile == nil {
		opt.ReadFile = os.ReadFile
	}
	c := &classifier{ctx: ctx, opt: opt, integ: opt.Integration, idleAfter: opt.IdleAfter}
	if c.integ == "" {
		c.integ = "main"
	}
	if c.idleAfter <= 0 {
		c.idleAfter = DefaultIdleAfter
	}
	c.now = time.Now()
	if opt.Now != nil {
		c.now = opt.Now()
	}
	top, err := c.git(opt.Root, "rev-parse", "--show-toplevel")
	if err != nil {
		return Report{}, fmt.Errorf("worklines: %s is not a git working tree: %w", opt.Root, err)
	}
	c.root = top
	tip, err := c.resolveIntegration()
	if err != nil {
		return Report{}, err
	}
	c.tip = tip
	if err := c.loadOpenRows(); err != nil {
		return Report{}, err
	}

	wts, err := c.worktrees()
	if err != nil {
		return Report{}, err
	}
	byBranch := map[string]worktree{}
	for _, w := range wts {
		if w.branch != "" {
			byBranch[w.branch] = w
		}
	}

	var lines []Line
	branches, err := c.branches()
	if err != nil {
		return Report{}, err
	}
	for _, b := range branches {
		if b.name == c.integ {
			continue
		}
		l := Line{Kind: KindBranch, Name: b.name, Tip: b.sha}
		var wt *worktree
		if w, ok := byBranch[b.name]; ok {
			wt = &w
			l.Worktree = w.path
		}
		if err := c.classifyCommitLine(&l, wt, ""); err != nil {
			return Report{}, err
		}
		lines = append(lines, l)
	}
	if len(wts) > 0 {
		c.mainWT = wts[0].path
	}
	for _, w := range wts {
		switch {
		case w.branch == "" && !w.bare && w.head != "":
			l := Line{Kind: KindWorktree, Name: w.path, Tip: w.head, Worktree: w.path}
			ww := w
			if err := c.classifyCommitLine(&l, &ww, ""); err != nil {
				return Report{}, err
			}
			lines = append(lines, l)
		case w.branch == c.integ:
			// The integration checkout is a line only while it holds
			// uncommitted work.
			ww := w
			dirty, missing := c.dirty(ww)
			if dirty == 0 || missing {
				continue
			}
			l := Line{Kind: KindWorktree, Name: w.path, Tip: w.head, Worktree: w.path}
			if err := c.classifyCommitLine(&l, &ww, ""); err != nil {
				return Report{}, err
			}
			lines = append(lines, l)
		}
	}
	stashes, err := c.stashes()
	if err != nil {
		return Report{}, err
	}
	for _, s := range stashes {
		l := Line{Kind: KindStash, Name: s.selector, Tip: s.sha}
		if err := c.classifyCommitLine(&l, nil, s.base); err != nil {
			return Report{}, err
		}
		lines = append(lines, l)
	}

	kindOrder := map[string]int{KindBranch: 0, KindWorktree: 1, KindStash: 2}
	sort.SliceStable(lines, func(i, j int) bool {
		if kindOrder[lines[i].Kind] != kindOrder[lines[j].Kind] {
			return kindOrder[lines[i].Kind] < kindOrder[lines[j].Kind]
		}
		if lines[i].Kind == KindStash {
			return stashIndex(lines[i].Name) < stashIndex(lines[j].Name)
		}
		return lines[i].Name < lines[j].Name
	})
	counts := map[Class]int{}
	for _, l := range lines {
		counts[l.Class]++
	}
	return Report{Root: c.root, Integration: c.integ, IntegrationTip: c.tip, OpenRows: len(c.open), Lines: lines, Counts: counts}, nil
}

func (c *classifier) git(dir string, args ...string) (string, error) {
	out, err := c.opt.Run(c.ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (c *classifier) resolveIntegration() (string, error) {
	for _, ref := range []string{"refs/heads/" + c.integ, "refs/remotes/origin/" + c.integ} {
		out, err := c.git(c.root, "for-each-ref", "--format=%(objectname)", ref)
		if err == nil && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out), nil
		}
	}
	return "", fmt.Errorf("worklines: integration branch %q not found in %s", c.integ, c.root)
}

// loadOpenRows parses every active plan on the integration branch (the
// non-recursive docs/plans/*.md, README.md excluded, as the verify gate
// reads them) and keeps the rows that are neither claimed nor dispositioned.
func (c *classifier) loadOpenRows() error {
	c.openByKey = map[string]map[string]bool{}
	for _, rel := range c.planFilesAt(c.tip) {
		raw, err := c.git(c.root, "show", c.tip+":"+rel)
		if err != nil {
			return err
		}
		plan := verify.ParseMarkdownPlan("", []byte(raw))
		for _, t := range plan.Tasks {
			if t.Claimed || t.Skipped || t.ID == "" {
				continue
			}
			r := openRow{
				key:     path.Base(rel) + "#" + t.ID,
				symbol:  lastIdent(t.Expect.Symbol),
				pkg:     strings.TrimSuffix(filepath.ToSlash(t.Expect.Package), "/"),
				oracle:  filepath.ToSlash(t.OraclePath),
				planRel: rel,
				id:      t.ID,
			}
			c.open = append(c.open, r)
			if c.openByKey[rel] == nil {
				c.openByKey[rel] = map[string]bool{}
			}
			c.openByKey[rel][t.ID] = true
		}
	}
	return nil
}

func (c *classifier) planFilesAt(commit string) []string {
	out, err := c.git(c.root, "ls-tree", "--name-only", commit, "docs/plans/")
	if err != nil {
		return nil
	}
	var files []string
	for _, f := range strings.Split(out, "\n") {
		if isPlanFile(f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

func isPlanFile(rel string) bool {
	if path.Dir(rel) != "docs/plans" || !strings.HasSuffix(rel, ".md") {
		return false
	}
	return !strings.EqualFold(path.Base(rel), "README.md")
}

type branch struct {
	name, sha string
}

func (c *classifier) branches() ([]branch, error) {
	out, err := c.git(c.root, "for-each-ref", "--format=%(refname:short)%00%(objectname)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	var bs []branch
	for _, ln := range strings.Split(out, "\n") {
		parts := strings.SplitN(ln, "\x00", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		bs = append(bs, branch{name: parts[0], sha: parts[1]})
	}
	sort.Slice(bs, func(i, j int) bool { return bs[i].name < bs[j].name })
	return bs, nil
}

type worktree struct {
	path, head, branch string
	bare, prunable     bool
}

func (c *classifier) worktrees() ([]worktree, error) {
	out, err := c.git(c.root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var wts []worktree
	var cur *worktree
	flush := func() {
		if cur != nil {
			wts = append(wts, *cur)
		}
		cur = nil
	}
	for _, ln := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(ln, "worktree "):
			flush()
			cur = &worktree{path: strings.TrimPrefix(ln, "worktree ")}
		case cur == nil:
		case strings.HasPrefix(ln, "HEAD "):
			cur.head = strings.TrimPrefix(ln, "HEAD ")
		case strings.HasPrefix(ln, "branch "):
			cur.branch = strings.TrimPrefix(strings.TrimPrefix(ln, "branch "), "refs/heads/")
		case ln == "bare":
			cur.bare = true
		case strings.HasPrefix(ln, "prunable"):
			cur.prunable = true
		}
	}
	flush()
	return wts, nil
}

type stash struct {
	selector, sha, base string
}

func (c *classifier) stashes() ([]stash, error) {
	has, err := c.git(c.root, "for-each-ref", "--format=%(objectname)", "refs/stash")
	if err != nil || strings.TrimSpace(has) == "" {
		return nil, err
	}
	out, err := c.git(c.root, "log", "--walk-reflogs", "--format=%gd%x00%H%x00%P", "refs/stash")
	if err != nil {
		return nil, err
	}
	var ss []stash
	for _, ln := range strings.Split(out, "\n") {
		parts := strings.SplitN(ln, "\x00", 3)
		if len(parts) != 3 || parts[1] == "" {
			continue
		}
		sel := strings.TrimPrefix(parts[0], "refs/")
		base := strings.Fields(parts[2])
		s := stash{selector: sel, sha: parts[1]}
		if len(base) > 0 {
			s.base = base[0]
		}
		ss = append(ss, s)
	}
	return ss, nil
}

func stashIndex(sel string) int {
	i := strings.Index(sel, "{")
	j := strings.Index(sel, "}")
	if i < 0 || j < i {
		return 0
	}
	n, _ := strconv.Atoi(sel[i+1 : j])
	return n
}

// dirty counts a worktree's uncommitted paths (tracked changes and
// untracked, non-ignored files). missing reports a worktree whose directory
// is gone.
func (c *classifier) dirty(w worktree) (n int, missing bool) {
	if w.prunable {
		return 0, true
	}
	if _, err := os.Stat(w.path); err != nil {
		return 0, true
	}
	out, err := c.git(w.path, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return 0, true
	}
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n, false
}

// dirtyPaths lists a dirty worktree's changed tracked paths.
func (c *classifier) dirtyPaths(w worktree) []string {
	out, err := c.git(w.path, "diff", "--name-only", "--no-renames", "HEAD")
	if err != nil {
		return nil
	}
	return splitLines(out)
}

// classifyCommitLine fills l's facts. base, when set (a stash), is the
// commit the line's changes are measured from; otherwise the merge base with
// the integration branch.
func (c *classifier) classifyCommitLine(l *Line, wt *worktree, base string) error {
	if ct, err := c.git(c.root, "log", "-1", "--format=%ct", l.Tip); err == nil {
		if sec, perr := strconv.ParseInt(strings.TrimSpace(ct), 10, 64); perr == nil {
			l.CommittedAt = time.Unix(sec, 0).UTC()
			l.AgeDays = int(c.now.Sub(l.CommittedAt) / (24 * time.Hour))
		}
	}
	isStash := l.Kind == KindStash
	if base == "" {
		if mb, err := c.git(c.root, "merge-base", c.tip, l.Tip); err == nil {
			base = strings.TrimSpace(mb)
		}
	}

	// Landed: ancestor, patch-equivalent, or content-equal.
	landed := ""
	var changed []string
	if base != "" {
		if out, err := c.git(c.root, "diff", "--name-only", "--no-renames", base, l.Tip); err == nil {
			changed = splitLines(out)
		}
	}
	if !isStash {
		cherry, err := c.git(c.root, "cherry", c.tip, l.Tip)
		if err != nil {
			return err
		}
		plus := 0
		for _, ln := range splitLines(cherry) {
			l.Ahead++
			if strings.HasPrefix(ln, "+") {
				plus++
			}
		}
		switch {
		case l.Ahead == 0:
			landed = "ancestor"
		case plus == 0:
			landed = "patch-equivalent"
		}
	} else {
		l.Ahead = 1
	}
	if landed == "" && base != "" && len(changed) > 0 && c.contentEqual(l.Tip, changed) {
		landed = "content-equal"
	}

	// At risk.
	var dirtyPlans []string
	if wt != nil {
		n, missing := c.dirty(*wt)
		switch {
		case missing:
			l.AtRisk = append(l.AtRisk, "worktree missing (prunable)")
		case n > 0:
			l.AtRisk = append(l.AtRisk, fmt.Sprintf("dirty worktree (%d %s)", n, plural(n, "path", "paths")))
			for _, p := range c.dirtyPaths(*wt) {
				if isPlanFile(p) {
					dirtyPlans = append(dirtyPlans, p)
				}
			}
		}
	}
	dirty := wt != nil && len(l.AtRisk) > 0 && strings.HasPrefix(l.AtRisk[0], "dirty")
	if landed != "" && !dirty {
		l.Class = Landed
		l.Landed = landed
		l.Removal = removal(*l, landed, c.integ, l.Worktree != "" && l.Worktree == c.mainWT)
		return nil
	}
	if isStash {
		l.AtRisk = append(l.AtRisk, "stash")
	} else if landed == "" {
		if out, err := c.git(c.root, "rev-list", "--count", l.Tip, "--not", c.tip, "--remotes"); err == nil {
			if n, _ := strconv.Atoi(strings.TrimSpace(out)); n > 0 {
				l.AtRisk = append(l.AtRisk, fmt.Sprintf("%d %s on no remote", n, plural(n, "commit", "commits")))
			}
		}
	}

	// Claims and touches read only what the line itself changed.
	if landed == "" {
		l.Claims = c.claims(l.Tip, changed, wt, dirtyPlans)
		diff := ""
		if base != "" {
			diff, _ = c.git(c.root, "diff", "--no-color", "--no-ext-diff", "--no-renames", "-U0", base, l.Tip)
		}
		if dirty {
			if wd, err := c.git(wt.path, "diff", "--no-color", "--no-ext-diff", "--no-renames", "-U0", "HEAD"); err == nil {
				diff += "\n" + wd
			}
		}
		l.Touches = c.touches(diff, l.Claims)
	} else {
		// A landed line kept only for its dirty worktree: its working-tree
		// edits are the unlanded part.
		l.Claims = c.claims(l.Tip, nil, wt, dirtyPlans)
		if wd, err := c.git(wt.path, "diff", "--no-color", "--no-ext-diff", "--no-renames", "-U0", "HEAD"); err == nil {
			l.Touches = c.touches(wd, l.Claims)
		}
	}
	if len(l.Touches) > 0 {
		l.Tier = "lexical"
	}
	switch {
	case len(l.Claims) > 0:
		l.Class = ClaimsRows
	case len(l.Touches) > 0:
		l.Class = TouchesRows
	case len(l.AtRisk) > 0:
		l.Class = AtRisk
	case c.now.Sub(l.CommittedAt) >= c.idleAfter:
		l.Class = Idle
	default:
		l.Class = Active
	}
	return nil
}

// contentEqual reports whether every path in changed reads the same at tip
// and on the integration branch: the line's whole change is there, whatever
// shape it landed in (a squash merge).
func (c *classifier) contentEqual(tip string, changed []string) bool {
	out, err := c.git(c.root, "diff", "--name-only", "--no-renames", tip, c.tip)
	if err != nil {
		return false
	}
	differ := map[string]bool{}
	for _, f := range splitLines(out) {
		differ[f] = true
	}
	for _, f := range changed {
		if differ[f] {
			return false
		}
	}
	return true
}

// claims names the open rows the line's own plan edits flip to claimed.
func (c *classifier) claims(tip string, changed []string, wt *worktree, dirtyPlans []string) []string {
	plans := map[string]bool{}
	for _, f := range changed {
		if isPlanFile(f) && c.openByKey[f] != nil {
			plans[f] = false
		}
	}
	for _, f := range dirtyPlans {
		if c.openByKey[f] != nil {
			plans[f] = true
		}
	}
	set := map[string]bool{}
	for rel, fromWorktree := range plans {
		var raw []byte
		if fromWorktree && wt != nil {
			b, err := c.opt.ReadFile(filepath.Join(wt.path, filepath.FromSlash(rel)))
			if err != nil {
				continue
			}
			raw = b
		} else {
			s, err := c.git(c.root, "show", tip+":"+rel)
			if err != nil {
				continue
			}
			raw = []byte(s)
		}
		for _, t := range verify.ParseMarkdownPlan("", raw).Tasks {
			if t.Claimed && c.openByKey[rel][t.ID] {
				set[path.Base(rel)+"#"+t.ID] = true
			}
		}
	}
	return sortedKeys(set)
}

var identRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// touches names the open rows whose anchors the diff touches, at the lexical
// tier: a changed line in a file under the row's package (anywhere, when the
// row names none) carries the row's symbol as a whole identifier, or the
// diff changes the row's oracle path. Plan files are skipped (a row's own
// text names its symbol), and rows the line claims are not repeated.
func (c *classifier) touches(diff string, claimed []string) []string {
	if strings.TrimSpace(diff) == "" || len(c.open) == 0 {
		return nil
	}
	files := map[string]bool{}
	idents := map[string]map[string]bool{} // file → identifiers on changed lines
	cur := ""
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			cur = ""
		case strings.HasPrefix(ln, "+++ "), strings.HasPrefix(ln, "--- "):
			p := strings.TrimSpace(ln[4:])
			if p == "/dev/null" {
				continue
			}
			p = strings.TrimPrefix(strings.TrimPrefix(p, "a/"), "b/")
			p = strings.Trim(p, `"`)
			cur = p
			files[p] = true
		case cur == "" || isPlanFile(cur):
		case strings.HasPrefix(ln, "+"), strings.HasPrefix(ln, "-"):
			if idents[cur] == nil {
				idents[cur] = map[string]bool{}
			}
			for _, id := range identRE.FindAllString(ln[1:], -1) {
				idents[cur][id] = true
			}
		}
	}
	skip := map[string]bool{}
	for _, k := range claimed {
		skip[k] = true
	}
	set := map[string]bool{}
	for _, r := range c.open {
		if skip[r.key] {
			continue
		}
		if r.oracle != "" && files[r.oracle] {
			set[r.key] = true
			continue
		}
		if r.symbol == "" {
			continue
		}
		for f, ids := range idents {
			if r.pkg != "" && path.Dir(f) != r.pkg && !strings.HasPrefix(f, r.pkg+"/") {
				continue
			}
			if ids[r.symbol] {
				set[r.key] = true
				break
			}
		}
	}
	return sortedKeys(set)
}

// removal names the command that would remove a landed line. It is printed,
// never run (I63).
func removal(l Line, landed, integ string, inMainWorktree bool) string {
	switch l.Kind {
	case KindStash:
		return "git stash drop " + l.Name
	case KindWorktree:
		return "git worktree remove " + shellQuote(l.Worktree)
	}
	del := "git branch -D " + shellQuote(l.Name)
	if landed == "ancestor" {
		del = "git branch -d " + shellQuote(l.Name)
	}
	switch {
	case inMainWorktree:
		return "git switch " + shellQuote(integ) + " && " + del
	case l.Worktree != "":
		return "git worktree remove " + shellQuote(l.Worktree) + " && " + del
	}
	return del
}

var safeShell = regexp.MustCompile(`^[A-Za-z0-9_./@:%+=,{}-]+$`)

func shellQuote(s string) string {
	if safeShell.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func lastIdent(sym string) string {
	sym = strings.TrimSpace(sym)
	if i := strings.LastIndexAny(sym, ".:/"); i >= 0 {
		sym = sym[i+1:]
	}
	if m := identRE.FindString(sym); m != "" {
		return m
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
