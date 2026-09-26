package custodian

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Audit evaluates every concern against the repository at root. One
// Finding per concern, in input (ID-sorted) order; a disabled concern is
// reported as off — named, never silently absent.
//
// Each corpus a concern names is resolved once per call through the
// corpus registry (corpus.go, CC-11) — agent-config, lefthook, taskfile,
// ci — and every corpus-reading kind runs over whatever its `where:`
// resolves to: one engine, many corpora.
//
// Options (options.go) inject the two things the CC-14/CC-15 kinds read
// beyond the tree — the daemon's tool registry and the user's thread log
// — plus the clock; every default is the live one, so callers with no
// opinion pass nothing.
func Audit(root string, concerns []Concern, opts ...Option) []Finding {
	env := newAuditEnv(opts)
	cache := corpusCache{root: root, got: map[string]corpusResult{}}
	out := make([]Finding, 0, len(concerns))
	for _, c := range concerns {
		if c.Off {
			out = append(out, Finding{Concern: c, Status: StatusOff, Detail: offDetail(c)})
			continue
		}
		out = append(out, checkConcern(root, &cache, c, env))
	}
	return out
}

// corpusCache resolves each named corpus at most once per Audit call.
type corpusCache struct {
	root string
	got  map[string]corpusResult
}

type corpusResult struct {
	files []string
	err   error
}

func (cc *corpusCache) files(name string) ([]string, error) {
	if r, ok := cc.got[name]; ok {
		return r.files, r.err
	}
	files, err := ResolveCorpus(cc.root, name)
	cc.got[name] = corpusResult{files: files, err: err}
	return files, err
}

func offDetail(c Concern) string {
	switch c.Scope {
	case ScopeProject:
		return "off (this repo)"
	case ScopePack:
		return "off (pack " + c.Pack + ")"
	}
	return "off (user scope)"
}

// checkConcern evaluates one active concern. Every path ends in a status
// with a detail — an unevaluable check is unable-to-check with the reason,
// never a silent skip and never a fake pass (law 3).
func checkConcern(root string, cache *corpusCache, c Concern, env auditEnv) Finding {
	var files []string
	if corporaFor(c.Check.Kind) != nil {
		if c.Check.Where == "" {
			c.Check.Where = CorpusAgentConfig // a hand-built Check; the loader defaults it too
		}
		var err error
		files, err = cache.files(c.Check.Where)
		if err != nil {
			return Finding{Concern: c, Status: StatusUnable, Detail: fmt.Sprintf("%s discovery failed: %v", c.Check.Where, err)}
		}
	}

	var f Finding
	switch c.Check.Kind {
	case KindForbiddenPattern:
		f = checkForbiddenPattern(root, files, c)
	case KindRequiredPattern:
		f = checkRequiredPattern(root, files, c)
	case KindCommandDocumentedAndExists:
		f = checkCommandDocumented(root, files, c)
	case KindCommandsResolve:
		f = checkCommandsResolve(root, files, c)
	case KindMaxLines:
		return checkMaxLines(root, c)
	case KindMaxBytes:
		return checkMaxBytes(root, c)
	case KindStanzaPresent:
		return checkStanzaPresent(root, c)
	case KindPointersResolve:
		f = checkPointersResolve(root, files, c)
	case KindStanzaToolsServed:
		f = checkStanzaToolsServed(root, files, c, env)
	case KindHandoffCitesInstruments:
		return checkHandoffCitesInstruments(root, c, env)
	case KindADRReferencesResolve:
		return checkADRReferencesResolve(root, c)
	case KindWiringCurrent:
		return checkWiringCurrent(root, c, env)
	default:
		// The loader refuses unknown kinds; reaching here means a caller
		// constructed a Check by hand. Same honesty either way.
		return Finding{Concern: c, Status: StatusUnable, Detail: fmt.Sprintf("unknown check kind %q", c.Check.Kind)}
	}
	if f.Files == nil {
		f.Files = files
	}
	return f
}

func checkForbiddenPattern(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c}
	re, err := regexp.Compile(c.Check.Pattern)
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("pattern does not compile: %v", err)
		return f
	}
	for _, rel := range files {
		lines, err := readFileLines(root, rel)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s unreadable: %v", rel, err)
			return f
		}
		for i, ln := range lines {
			if re.MatchString(ln) {
				f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
	}
	if len(f.Evidence) > 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d match(es) of forbidden pattern %q", len(f.Evidence), c.Check.Pattern)
		return f
	}
	f.Status = StatusHolds
	f.Detail = fmt.Sprintf("no match across %s", countNoun(len(files), corpusNoun(c.Check.Where)))
	return f
}

// checkRequiredPattern is forbidden_pattern's dual: at least one line of
// the corpus must match. The matching lines are the evidence either way —
// where the required thing lives is worth knowing when it holds.
//
// Over a YAML corpus (lefthook, taskfile, ci) a comment never satisfies
// the pattern — a commented-out step runs nothing — and `under` holds the
// match to the lines under one top-level key (the lefthook pack's
// pre-push, I17: a verify in pre-commit gates no push).
func checkRequiredPattern(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c}
	re, err := regexp.Compile(c.Check.Pattern)
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("pattern does not compile: %v", err)
		return f
	}
	if len(files) == 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("no %s(s) at all — the corpus is %s, and nothing there can satisfy `%s`", corpusNoun(c.Check.Where), CorpusAbout(c.Check.Where), c.Check.Pattern)
		return f
	}
	yamlText := yamlCorpus(c.Check.Where)
	under := c.Check.Under
	var keyless []string // files with no `under` key
	for _, rel := range files {
		lines, err := readFileLines(root, rel)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s unreadable: %v", rel, err)
			f.Evidence = nil
			return f
		}
		lo, hi := 1, len(lines)
		if under != "" {
			var found bool
			lo, hi, found, err = topLevelKeySpan(lines, under)
			if err != nil {
				f.Status = StatusUnable
				f.Detail = fmt.Sprintf("%s does not parse as YAML, so what runs under `%s` cannot be read: %v", rel, under, err)
				f.Evidence = nil
				return f
			}
			if !found {
				keyless = append(keyless, rel)
				continue
			}
		}
		for i := lo - 1; i < hi; i++ {
			ln := lines[i]
			if yamlText {
				ln = codeText(ln)
			}
			if re.MatchString(ln) {
				f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
	}
	scope := ""
	if under != "" {
		scope = fmt.Sprintf(" under `%s`", under)
	}
	switch {
	case len(f.Evidence) > 0:
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("%d line(s)%s match `%s` across %s", len(f.Evidence), scope, c.Check.Pattern, countNoun(len(files), corpusNoun(c.Check.Where)))
	case len(keyless) == len(files):
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("no `%s` key in %s — nothing runs under it, so nothing there can satisfy `%s`", under, strings.Join(keyless, ", "), c.Check.Pattern)
	case under != "":
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("no line under `%s` matches required pattern `%s` across %s (comments are not commands)", under, c.Check.Pattern, countNoun(len(files), corpusNoun(c.Check.Where)))
	default:
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("no line matches required pattern `%s` across %s", c.Check.Pattern, countNoun(len(files), corpusNoun(c.Check.Where)))
	}
	return f
}

// yamlCorpus reports a corpus whose files are YAML (their comments are
// not commands, and `under` names a top-level key of them).
func yamlCorpus(where string) bool {
	return where == CorpusLefthook || where == CorpusTaskfile || where == CorpusCI
}

// codeText returns a YAML-corpus line with its comment cut. A line whose
// first non-blank character is '#' is all comment — a YAML comment, or a
// shell comment inside a block scalar; either way it runs nothing — and an
// unquoted '#' after a blank starts a trailing one.
func codeText(line string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch ch := line[i]; {
		case ch == '\\' && inDouble:
			i++
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '#' && !inSingle && !inDouble && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			return line[:i]
		}
	}
	return line
}

// topLevelKeySpan finds a top-level key of a YAML file and returns the
// 1-based line span from the key to the line before the next top-level key
// (or the end of the file).
func topLevelKeySpan(lines []string, key string) (lo, hi int, found bool, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &doc); err != nil {
		return 0, 0, false, err
	}
	top := documentMapping(&doc)
	if top == nil {
		return 0, 0, false, nil
	}
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value != key {
			continue
		}
		lo, hi = top.Content[i].Line, len(lines)
		if i+2 < len(top.Content) {
			hi = top.Content[i+2].Line - 1
		}
		return lo, hi, true, nil
	}
	return 0, 0, false, nil
}

// checkCommandDocumented holds a command to both halves: it is named in
// the corpus (over a YAML corpus, on a line that is not a comment — a
// commented-out step runs nothing), and it exists — a `task X` target in
// the parsed Taskfile graph commands_resolve uses (includes, aliases,
// flattening; never a grep for an indented key, which read a `vars:` entry
// named like the target as the target), else the binary on PATH.
func checkCommandDocumented(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c}
	cmdStr := c.Check.Command

	documentedIn := 0
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s unreadable: %v", rel, err)
			return f
		}
		if documents(data, cmdStr, yamlCorpus(c.Check.Where)) {
			documentedIn++
		}
	}
	if documentedIn == 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%q is not documented in any of %s", cmdStr, countNoun(len(files), corpusNoun(c.Check.Where)))
		return f
	}

	argv := strings.Fields(cmdStr)
	if argv[0] == "task" && len(argv) > 1 {
		name := argv[1]
		if strings.HasPrefix(name, "-") || !taskNameRe.MatchString(name) {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("documented in %d file(s), but %q is not a literal target (a flag or template before it changes what resolves)", documentedIn, name)
			return f
		}
		first, err := rootTaskfile(root)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("the root Taskfile cannot be checked: %v", err)
			return f
		}
		if first == "" {
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("documented in %d file(s) but there is no Taskfile at the repo root to define %q", documentedIn, cmdStr)
			return f
		}
		g, err := loadTaskGraph(root, first)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = err.Error()
			return f
		}
		name = strings.TrimPrefix(name, ":")
		missing, uncheckable := g.resolveTarget(name)
		switch {
		case uncheckable != "":
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("documented in %d file(s); whether %q exists cannot be checked: %s", documentedIn, name, uncheckable)
		case missing == "no such target":
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("documented in %d file(s) but %s has no %q target — the instruction outlived the command (the O25 shape)", documentedIn, first, name)
		case missing != "":
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("documented in %d file(s) but %q does not resolve: %s", documentedIn, name, missing)
		default:
			f.Status = StatusHolds
			f.Detail = fmt.Sprintf("documented in %d file(s); `%s` target exists in %s", documentedIn, name, first)
		}
		return f
	}

	if _, err := exec.LookPath(argv[0]); err != nil {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("documented in %d file(s) but %q is not on PATH", documentedIn, argv[0])
		return f
	}
	f.Status = StatusHolds
	f.Detail = fmt.Sprintf("documented in %d file(s); %q is on PATH", documentedIn, argv[0])
	return f
}

// documents reports whether a corpus file names cmd — anywhere in
// markdown, on a non-comment line of a YAML corpus.
func documents(data []byte, cmd string, yamlText bool) bool {
	if !yamlText {
		return bytes.Contains(data, []byte(cmd))
	}
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.Contains(codeText(ln), cmd) {
			return true
		}
	}
	return false
}

func checkMaxLines(root string, c Concern) Finding {
	f := Finding{Concern: c}
	p := filepath.Join(root, filepath.FromSlash(c.Check.File))
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s is absent — the concern expects it to exist", c.Check.File)
		return f
	}
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("%s unreadable: %v", c.Check.File, err)
		return f
	}
	n := countLines(data)
	if n > c.Check.Lines {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s is %d lines (max %d)", c.Check.File, n, c.Check.Lines)
		return f
	}
	f.Status = StatusHolds
	f.Detail = fmt.Sprintf("%s is %d lines (max %d)", c.Check.File, n, c.Check.Lines)
	return f
}

// checkMaxBytes holds a file to a byte budget — the size the harness
// counts, not the lines a human counts. A missing file is a violation
// (the concern expects it); one that exists and cannot be stat'd is
// unable-to-check, never "absent" (I38).
func checkMaxBytes(root string, c Concern) Finding {
	f := Finding{Concern: c}
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(c.Check.File)))
	if errors.Is(err, fs.ErrNotExist) {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s is absent — the concern expects it to exist", c.Check.File)
		return f
	}
	if err != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("%s unreadable: %v", c.Check.File, err)
		return f
	}
	n := fi.Size()
	f.Detail = fmt.Sprintf("%s is %d bytes (max %d)", c.Check.File, n, c.Check.Bytes)
	if n > int64(c.Check.Bytes) {
		f.Status = StatusViolated
		f.Evidence = []string{c.Check.File}
		return f
	}
	f.Status = StatusHolds
	return f
}

// checkStanzaPresent is the three-case stanza detection fathom init writes
// against (absent / well-formed / corrupted), read-only. Its details are
// file-prefixed sentences because doctor's stanza_present row renders
// them verbatim (CC-13): the row joins the non-holding details of the
// claude-code and codex entries, exactly as it used to join its own.
func checkStanzaPresent(root string, c Concern) Finding {
	f := Finding{Concern: c}
	file := c.Check.File
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		f.Detail = fmt.Sprintf("%s: %v", file, err)
		if errors.Is(err, fs.ErrNotExist) {
			f.Status = StatusViolated
		} else {
			f.Status = StatusUnable // exists and unreadable is never "absent" (I38)
		}
		return f
	}
	content := string(data)
	begin := strings.Index(content, stanzaBeginPrefix)
	end := strings.Index(content, stanzaEnd)
	lineOf := func(idx int) string { return fmt.Sprintf("%s:%d", file, strings.Count(content[:idx], "\n")+1) }
	switch {
	case begin >= 0 && end > begin:
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("%s: fathom stanza well-formed", file)
	case begin < 0 && end < 0:
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s: never wired (no fathom markers)", file)
		f.Evidence = []string{file}
	default:
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s: corrupted marker pair (begin=%t end=%t)", file, begin >= 0, end >= 0)
		if begin >= 0 {
			f.Evidence = append(f.Evidence, lineOf(begin))
		}
		if end >= 0 {
			f.Evidence = append(f.Evidence, lineOf(end))
		}
	}
	return f
}

// markdownLinkRe captures relative markdown-link targets; backtick
// pointer coverage lives beside it (backtickTokenRe + repoPathRe) because
// this house style points with `docs/foo.md`, not links — a check blind
// to that holds vacuously, which the audit's own first live run
// demonstrated. Bare mentions stay unpoliced: a false violation teaches
// people to ignore the custodian.
var markdownLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)

// backtickTokenRe finds inline-code tokens; repoPathRe then decides which
// look like repo paths worth holding to existence: ≥2 non-empty segments
// of path-safe characters (globs, spaces, shell fragments, and prose
// ellipses all fail the charset).
var (
	backtickTokenRe = regexp.MustCompile("`([^`]+)`")
	repoPathRe      = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)+/?$`)
)

// pointerRuntimeDirs are first segments whose contents are runtime or
// vendored state, not documentation contracts — a gitignored artifact
// path is allowed to not exist yet.
var pointerRuntimeDirs = map[string]struct{}{
	"workspace": {}, "tmp": {}, "ref": {}, "bin": {},
}

// PointersCorpus is the file set the pointers check holds to existence:
// the agent-config corpus PLUS the root INTENT.md when one exists (HP-35).
// A wider corpus than the other checks use, deliberately: INTENT.md is
// rendered from `docs/intent/` and is almost entirely pointers, so a
// corpus file that moves breaks it first and loudest — but it carries no
// stanza and no commands, so subjecting it to the stanza and command
// checks would only invite false violations, and a false violation
// teaches people to ignore the custodian.
func PointersCorpus(root string, files []string) []string {
	const intentFile = "INTENT.md"
	for _, rel := range files {
		if rel == intentFile {
			return files
		}
	}
	if _, err := os.Stat(filepath.Join(root, intentFile)); err != nil {
		return files
	}
	return append(append([]string{}, files...), intentFile)
}

// pointerRef is one pointer in an agent-config file: a markdown link
// target (fragment stripped) or a backticked repo path.
type pointerRef struct {
	File   string
	Line   int // 1-based
	Target string
	Link   bool // markdown link (file-relative) vs backtick path (root- or file-relative)
}

// scanPointers walks the pointers corpus and returns the pointers that do
// not resolve plus how many did. An unreadable file is an error (I38:
// unable-to-check upstream, never a smaller count). config propose
// (propose.go) reads the same scan to offer relocations.
func scanPointers(root string, files []string) (dead []pointerRef, resolved int, err error) {
	for _, rel := range PointersCorpus(root, files) {
		lines, err := readFileLines(root, rel)
		if err != nil {
			return nil, 0, fmt.Errorf("%s unreadable: %v", rel, err)
		}
		for i, ln := range lines {
			for _, m := range markdownLinkRe.FindAllStringSubmatch(ln, -1) {
				target := m[1]
				if skipPointerTarget(target) {
					continue
				}
				if idx := strings.IndexByte(target, '#'); idx >= 0 {
					target = target[:idx]
				}
				if target == "" {
					continue
				}
				// Markdown links resolve file-relative (the markdown
				// contract).
				abs := filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(target))
				if _, err := os.Stat(abs); err != nil {
					dead = append(dead, pointerRef{File: rel, Line: i + 1, Target: target, Link: true})
				} else {
					resolved++
				}
			}
			for _, m := range backtickTokenRe.FindAllStringSubmatch(ln, -1) {
				target := m[1]
				if !repoPathRe.MatchString(target) {
					continue
				}
				first := target[:strings.IndexByte(target, '/')]
				if first == "." || first == ".." {
					continue // out-of-repo escapes are not this repo's pointers to hold
				}
				if _, runtime := pointerRuntimeDirs[first]; runtime {
					continue
				}
				// A token whose FIRST segment is not a directory this repo
				// (or the citing file's dir) even has is a foreign name — a
				// Go module path, a kit package — not a pointer into this
				// repo. Policing those would flood the audit with false
				// violations, and a false violation teaches people to
				// ignore the custodian.
				fileBase := filepath.Dir(filepath.FromSlash(rel))
				rootFirst := dirExists(filepath.Join(root, first))
				fileFirst := dirExists(filepath.Join(root, fileBase, first))
				if !rootFirst && !fileFirst {
					continue
				}
				// Backtick paths cite from the repo root in this style;
				// accept a file-relative resolution too, so either
				// convention passes and neither is falsely flagged.
				fromRoot := filepath.Join(root, filepath.FromSlash(target))
				fromFile := filepath.Join(root, fileBase, filepath.FromSlash(target))
				if _, err := os.Stat(fromRoot); err == nil {
					resolved++
					continue
				}
				if _, err := os.Stat(fromFile); err == nil {
					resolved++
					continue
				}
				dead = append(dead, pointerRef{File: rel, Line: i + 1, Target: target})
			}
		}
	}
	return dead, resolved, nil
}

func checkPointersResolve(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c, Files: PointersCorpus(root, files)}
	dead, resolved, err := scanPointers(root, files)
	if err != nil {
		f.Status = StatusUnable
		f.Detail = err.Error()
		return f
	}
	for _, d := range dead {
		f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d → %s", d.File, d.Line, d.Target))
	}
	if len(f.Evidence) > 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d dead pointer(s) out of %d", len(f.Evidence), resolved+len(f.Evidence))
		return f
	}
	f.Status = StatusHolds
	if resolved == 0 {
		f.Detail = fmt.Sprintf("no relative pointers found across %s", countNoun(len(PointersCorpus(root, files)), "agent-config file"))
		return f
	}
	f.Detail = fmt.Sprintf("%d pointer(s) resolve across %s", resolved, countNoun(len(PointersCorpus(root, files)), "agent-config file"))
	return f
}

func skipPointerTarget(target string) bool {
	return strings.HasPrefix(target, "#") ||
		strings.Contains(target, "://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "/") ||
		strings.HasPrefix(target, "~")
}

// AgentConfigFiles discovers the agent-config corpus (CLAUDE.md and
// AGENTS.md at any depth), repo-relative and sorted.
//
// Inside a git work tree (a .git at root), git's ignore rules are the
// authority on what belongs to the repo — that is what keeps `ref/` clones
// and `workspace/` out without this package growing its own ignore
// engine. Elsewhere (tests, bare directories) a walk with a fixed skip
// list stands in.
func AgentConfigFiles(root string) ([]string, error) {
	if fi, err := os.Stat(filepath.Join(root, ".git")); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
		if out, err := gitAgentConfigFiles(root); err == nil {
			return out, nil
		}
		// A git tree whose git invocation failed still deserves an answer:
		// fall through to the walk rather than reporting nothing.
	}
	return walkAgentConfigFiles(root)
}

func gitAgentConfigFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "--",
		"CLAUDE.md", "*/CLAUDE.md", "AGENTS.md", "*/AGENTS.md")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if vendoredPath(ln) {
			continue
		}
		if _, dup := seen[ln]; dup {
			continue
		}
		seen[ln] = struct{}{}
		out = append(out, ln)
	}
	sort.Strings(out)
	return out, nil
}

// vendoredPath excludes tracked-but-vendored trees from the corpus: a
// third-party snapshot's CLAUDE.md is that project's instruction surface,
// not this repo's — auditing it produces violations nobody here can fix.
func vendoredPath(rel string) bool {
	first := rel
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		first = rel[:i]
	}
	switch first {
	case "third_party", "vendor", "node_modules":
		return true
	}
	return false
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// walkSkipDirs is the non-git fallback's junk list — the conventional
// build/clone/scratch dirs whose CLAUDE.md files belong to OTHER repos or
// to generated trees, not to this one's agent configuration.
var walkSkipDirs = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "third_party": {},
	"workspace": {}, "ref": {}, "tmp": {}, "bin": {}, "testdata": {},
	".idea": {}, ".vscode": {},
}

func walkAgentConfigFiles(root string) ([]string, error) {
	var out []string
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := walkSkipDirs[d.Name()]; skip && p != "." {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == "CLAUDE.md" || d.Name() == "AGENTS.md" {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func readFileLines(root, rel string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func countLines(data []byte) int {
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func countNoun(n int, noun string) string {
	return fmt.Sprintf("%d %s(s)", n, noun)
}
