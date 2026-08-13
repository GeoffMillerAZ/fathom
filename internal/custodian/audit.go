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
)

// Audit evaluates every concern against the repository at root. One
// Finding per concern, in input (ID-sorted) order; a disabled concern is
// reported as off — named, never silently absent.
func Audit(root string, concerns []Concern) []Finding {
	files, filesErr := AgentConfigFiles(root)
	out := make([]Finding, 0, len(concerns))
	for _, c := range concerns {
		if c.Off {
			out = append(out, Finding{Concern: c, Status: StatusOff, Detail: offDetail(c)})
			continue
		}
		out = append(out, checkConcern(root, files, filesErr, c))
	}
	return out
}

func offDetail(c Concern) string {
	if c.Scope == ScopeProject {
		return "off (this repo)"
	}
	return "off (user scope)"
}

// checkConcern evaluates one active concern. Every path ends in a status
// with a detail — an unevaluable check is unable-to-check with the reason,
// never a silent skip and never a fake pass (law 3).
func checkConcern(root string, files []string, filesErr error, c Concern) Finding {
	f := Finding{Concern: c}
	needsCorpus := c.Check.Kind == KindForbiddenPattern ||
		c.Check.Kind == KindCommandDocumentedAndExists ||
		c.Check.Kind == KindPointersResolve
	if needsCorpus && filesErr != nil {
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("agent-config discovery failed: %v", filesErr)
		return f
	}

	switch c.Check.Kind {
	case KindForbiddenPattern:
		return checkForbiddenPattern(root, files, c)
	case KindCommandDocumentedAndExists:
		return checkCommandDocumented(root, files, c)
	case KindMaxLines:
		return checkMaxLines(root, c)
	case KindPointersResolve:
		return checkPointersResolve(root, files, c)
	default:
		// The loader refuses unknown kinds; reaching here means a caller
		// constructed a Check by hand. Same honesty either way.
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("unknown check kind %q", c.Check.Kind)
		return f
	}
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
	f.Detail = fmt.Sprintf("no match across %s", countNoun(len(files), "agent-config file"))
	return f
}

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
		if bytes.Contains(data, []byte(cmdStr)) {
			documentedIn++
		}
	}
	if documentedIn == 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%q is not documented in any of %s", cmdStr, countNoun(len(files), "agent-config file"))
		return f
	}

	argv := strings.Fields(cmdStr)
	if argv[0] == "task" && len(argv) > 1 {
		data, path, err := readTaskfile(root)
		if err != nil {
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("documented in %d file(s) but there is no Taskfile.yml at the repo root to define %q", documentedIn, cmdStr)
			return f
		}
		target := regexp.MustCompile(`(?m)^[ \t]+` + regexp.QuoteMeta(argv[1]) + `:`)
		if !target.Match(data) {
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("documented in %d file(s) but %s has no %q target — the instruction outlived the command (the O25 shape)", documentedIn, filepath.Base(path), argv[1])
			return f
		}
		f.Status = StatusHolds
		f.Detail = fmt.Sprintf("documented in %d file(s); `%s` target exists in %s", documentedIn, argv[1], filepath.Base(path))
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

func readTaskfile(root string) ([]byte, string, error) {
	for _, name := range []string{"Taskfile.yml", "Taskfile.yaml"} {
		p := filepath.Join(root, name)
		data, err := os.ReadFile(p)
		if err == nil {
			return data, p, nil
		}
	}
	return nil, "", errors.New("no Taskfile.yml or Taskfile.yaml at the repo root")
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

// markdownLinkRe captures relative markdown-link targets. Deliberately
// conservative: only `[text](target)` links — backtick paths and bare
// mentions are too noisy to police without false positives, and a false
// violation teaches people to ignore the custodian.
var markdownLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)

func checkPointersResolve(root string, files []string, c Concern) Finding {
	f := Finding{Concern: c}
	resolved := 0
	for _, rel := range files {
		lines, err := readFileLines(root, rel)
		if err != nil {
			f.Status = StatusUnable
			f.Detail = fmt.Sprintf("%s unreadable: %v", rel, err)
			return f
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
				abs := filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(target))
				if _, err := os.Stat(abs); err != nil {
					f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d → %s", rel, i+1, target))
				} else {
					resolved++
				}
			}
		}
	}
	if len(f.Evidence) > 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d dead pointer(s) out of %d", len(f.Evidence), resolved+len(f.Evidence))
		return f
	}
	f.Status = StatusHolds
	if resolved == 0 {
		f.Detail = fmt.Sprintf("no relative pointers found across %s", countNoun(len(files), "agent-config file"))
		return f
	}
	f.Detail = fmt.Sprintf("%d pointer(s) resolve across %s", resolved, countNoun(len(files), "agent-config file"))
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
		if _, dup := seen[ln]; dup {
			continue
		}
		seen[ln] = struct{}{}
		out = append(out, ln)
	}
	sort.Strings(out)
	return out, nil
}

// walkSkipDirs is the non-git fallback's junk list — the conventional
// build/clone/scratch dirs whose CLAUDE.md files belong to OTHER repos or
// to generated trees, not to this one's agent configuration.
var walkSkipDirs = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "workspace": {},
	"ref": {}, "tmp": {}, "bin": {}, "testdata": {}, ".idea": {}, ".vscode": {},
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
