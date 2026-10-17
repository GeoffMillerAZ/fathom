//go:build decideexp

package decide_test

// exp_e2items_test.go — E2a's item builder (docs/bench/readings/decide/exp-preregistration.md
// section 8). It is a decideexp test, not a command: the tag and
// FATHOM_DECIDE_EXP=1 keep it out of every `go test ./...` and every binary,
// so stage 0 adds no product surface to keep wired, and it shares the
// harness's store client, sampler stream and vet line. It asks no model
// and opens no decider.
//
//	FATHOM_DECIDE_EXP=1 FATHOM_DECIDE_EXP_E2A_OUT=<scratch>/e2a-items.jsonl \
//	FATHOM_DECIDE_EXP_SUBJECTS='fathom|<checkout holding fd091c4f>|<store ingested there>' \
//	  go test -tags decideexp -run TestDecideExpE2aItems -count=1 -timeout 30m -v ./internal/decide
//
// The draw, fixed here before any E2 call (K19):
//   - Pool: every GoFunc or GoMethod in a .go file that is not a _test.go
//     file with at least one typed-edge caller in the store: an edge of
//     kind direct or iface-dispatch (a call, not a func-value reference)
//     from another Go function or method; a function is not its own caller.
//   - Order: the pool sorted by (subject, file, start line, name) and
//     shuffled as section 3 shuffles, on the stream "e2-caller/fathom"
//     seeded from ask.SampleSeed (ask.Stream).
//   - Each function in that order: its declaration at fd091c4f (read with
//     git show, found by ask.FuncSource), then `git log -L :<re>:<file>
//     -n 1 fd091c4f`, where <re> is the declaration's first line from
//     `func` through its name and the next ( or [, anchored and with each
//     punctuation character bracketed (git's -L regex is POSIX basic), git
//     run with no system or global config. Kept when that commit is among
//     the 300 non-merge commits before fd091c4f (`git rev-list --no-merges
//     --max-count=300 fd091c4f^@`, fd091c4f itself not counted) and its
//     diff changes a non-comment line: a + or - line in a hunk whose text,
//     trimmed, is non-empty and starts with neither // nor /*.
//   - For each kept function, one caller: the same stream goes on and
//     draws Intn(n) over its callers sorted by key; the caller's text at
//     fd091c4f is ask.FuncSource's, as E1 frames a span.
//   - The diff shown is git's output after the commit line, at most 80
//     lines: a longer one keeps its first 79 and a last line naming how
//     many more there were. The first 50 kept are written, ids e2a-001 on.
//
// Deterministic: the same store and checkout write a byte-identical file.
// The item text stays in the output file, which belongs in scratch and is
// never committed; the log holds counts only.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/ask"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// The pre-registered pins of section 8's E2a draw.
const (
	expE2Base       = "fd091c4f0d99ae13b355cdb914ae6e0bca0681b5"
	expE2Window     = 300
	expE2aWant      = 50
	expE2aStream    = "e2-caller"
	expE2aStreamFor = "fathom"
)

// expGitEnv is the environment every builder git runs in: this process's,
// less any GIT_* variable (a hook's GIT_DIR must not redirect it), with no
// system or global config, so the output depends on the repository alone.
func expGitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_PAGER=cat", "LC_ALL=C")
}

func expGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "color.ui=never", "-c", "core.quotePath=true"}, args...)...)
	cmd.Env = expGitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// expCommitWindow is the n non-merge commits before rev (rev itself not
// counted), the window E2a and E2b draw from.
func expCommitWindow(ctx context.Context, dir, rev string, n int) (map[string]bool, error) {
	out, err := expGit(ctx, dir, "rev-list", "--no-merges", fmt.Sprintf("--max-count=%d", n), rev+"^@")
	if err != nil {
		return nil, err
	}
	w := map[string]bool{}
	for _, h := range strings.Fields(out) {
		w[h] = true
	}
	if len(w) != n {
		return nil, fmt.Errorf("window before %s holds %d commits, want %d", rev, len(w), n)
	}
	return w, nil
}

// expFuncnameRegex is the -L regex that finds name's declaration: decl's
// first line from "func" through the name and the ( or [ after it,
// anchored, each punctuation character bracketed (POSIX basic syntax,
// which git's -L uses) and :, ^, \ and - read as any character. ok is false
// when the first line does not hold the name.
func expFuncnameRegex(decl, name string) (string, bool) {
	first, _, _ := strings.Cut(decl, "\n")
	if !strings.HasPrefix(first, "func ") {
		return "", false
	}
	bare := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		bare = name[i+1:]
	}
	from := 0
	if strings.HasPrefix(first, "func (") {
		from = strings.Index(first, ")")
		if from < 0 {
			return "", false
		}
	}
	end := -1
	for _, next := range []string{"(", "["} {
		if i := strings.Index(first[from:], " "+bare+next); i >= 0 && (end < 0 || from+i < end) {
			end = from + i
		}
	}
	if end < 0 {
		return "", false
	}
	prefix := first[:end+1+len(bare)]
	var b strings.Builder
	b.WriteString("^")
	for _, r := range prefix {
		switch {
		case r == ' ' || r == '\t' || r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r > 0x7f:
			b.WriteRune(r)
		case r == ']':
			b.WriteString("[]]")
		case r == ':' || r == '^' || r == '\\' || r == '-':
			b.WriteString(".")
		default:
			b.WriteString("[" + string(r) + "]")
		}
	}
	b.WriteString("[[(]")
	return b.String(), true
}

// expDiffChangesCode is section 8's "its diff changes a non-comment line".
func expDiffChangesCode(diff string) bool {
	inHunk := false
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			inHunk = false
			continue
		case strings.HasPrefix(l, "@@"):
			inHunk = true
			continue
		}
		if !inHunk || l == "" || (l[0] != '+' && l[0] != '-') {
			continue
		}
		s := strings.TrimSpace(l[1:])
		if s != "" && !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, "/*") {
			return true
		}
	}
	return false
}

// expCapLines keeps s to at most n lines: a longer s keeps its first n-1
// and a last line naming how many more there were.
func expCapLines(s string, n int) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n-1], "\n") + fmt.Sprintf("\n[... %d more lines of this diff not shown]", len(lines)-(n-1))
}

// expSplitLogL splits `git log -L ... --format=%H` output into the commit
// and the diff after it.
func expSplitLogL(out string) (hash, diff string) {
	hash, rest, _ := strings.Cut(out, "\n")
	return strings.TrimSpace(hash), strings.TrimRight(strings.TrimLeft(rest, "\n"), "\n")
}

// expE2aCounts is the builder's log line: why each examined function was
// kept or passed over. Counts only.
type expE2aCounts struct {
	Pool, Examined, Kept int
	Skipped              map[string]int
}

// expBuildE2aItems draws E2a's items from sub (the fathom subject).
func expBuildE2aItems(ctx context.Context, sub expSubject) ([]expE2Item, expE2aCounts, error) {
	counts := expE2aCounts{Skipped: map[string]int{}}
	c, err := client.NewConstantClient(client.ModeLocal, sub.Store)
	if err != nil {
		return nil, counts, fmt.Errorf("open store %s: %w", sub.Name, err)
	}
	defer func() { _ = c.Close() }()
	nodes, err := c.ListAllSymbols(ctx, sub.Name)
	if err != nil {
		return nil, counts, err
	}
	edges, err := c.TypedEdges(ctx, sub.Name)
	if err != nil {
		return nil, counts, err
	}
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	funcs := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if (n.Kind != core.KindGoFunc && n.Kind != core.KindGoMethod) || !strings.HasSuffix(n.FilePath, ".go") {
			continue
		}
		if _, dup := funcs[n.Key()]; !dup {
			funcs[n.Key()] = n
		}
	}
	callerSet := map[string]map[string]bool{}
	for _, e := range edges {
		if (e.Kind != core.EdgeDirect && e.Kind != core.EdgeIfaceDispatch) || e.FromSymbol == e.ToSymbol {
			continue
		}
		if _, ok := funcs[e.FromSymbol]; !ok {
			continue
		}
		to, ok := funcs[e.ToSymbol]
		if !ok || strings.HasSuffix(to.FilePath, "_test.go") {
			continue
		}
		if callerSet[e.ToSymbol] == nil {
			callerSet[e.ToSymbol] = map[string]bool{}
		}
		callerSet[e.ToSymbol][e.FromSymbol] = true
	}
	type cand struct {
		span ask.Span
		key  string
	}
	var pool []cand
	for k := range callerSet {
		n := funcs[k]
		pool = append(pool, cand{span: ask.Span{Repo: sub.Name, File: n.FilePath, Name: n.Name, Start: n.Line, End: n.EndLine}, key: k})
	}
	sort.Slice(pool, func(i, j int) bool {
		a, b := pool[i].span, pool[j].span
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return pool[i].key < pool[j].key
	})
	counts.Pool = len(pool)
	stream := ask.NewStream(ask.SampleSeed, expE2aStream, expE2aStreamFor)
	stream.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	window, err := expCommitWindow(ctx, sub.Root, expE2Base, expE2Window)
	if err != nil {
		return nil, counts, err
	}
	files := map[string][]byte{}
	fileAt := func(path string) ([]byte, error) {
		if b, ok := files[path]; ok {
			return b, nil
		}
		out, err := expGit(ctx, sub.Root, "show", expE2Base+":"+path)
		if err != nil {
			return nil, err
		}
		files[path] = []byte(out)
		return files[path], nil
	}
	var items []expE2Item
	for _, p := range pool {
		if len(items) == expE2aWant {
			break
		}
		counts.Examined++
		src, err := fileAt(p.span.File)
		if err != nil {
			counts.Skipped["file_unreadable"]++
			continue
		}
		_, decl, ok := ask.FuncSource(p.span.File, src, p.span)
		if !ok {
			counts.Skipped["no_decl"]++
			continue
		}
		re, ok := expFuncnameRegex(decl, p.span.Name)
		if !ok {
			counts.Skipped["no_funcname_line"]++
			continue
		}
		out, err := expGit(ctx, sub.Root, "log", "-L", ":"+re+":"+p.span.File, "-n", "1", "--format=%H", "--no-color", "--no-ext-diff", expE2Base)
		if err != nil {
			counts.Skipped["log_failed"]++
			continue
		}
		hash, diff := expSplitLogL(out)
		switch {
		case hash == "":
			counts.Skipped["no_commit"]++
			continue
		case !window[hash]:
			counts.Skipped["outside_window"]++
			continue
		case !expDiffChangesCode(diff):
			counts.Skipped["comment_only"]++
			continue
		}
		callers := make([]string, 0, len(callerSet[p.key]))
		for k := range callerSet[p.key] {
			callers = append(callers, k)
		}
		sort.Strings(callers)
		cn := funcs[callers[stream.Intn(len(callers))]]
		csrc, err := fileAt(cn.FilePath)
		if err != nil {
			counts.Skipped["caller_unreadable"]++
			continue
		}
		_, ctext, ok := ask.FuncSource(cn.FilePath, csrc, ask.Span{Repo: sub.Name, File: cn.FilePath, Name: cn.Name, Start: cn.Line, End: cn.EndLine})
		if !ok {
			counts.Skipped["caller_no_decl"]++
			continue
		}
		items = append(items, expE2Item{
			ID:         fmt.Sprintf("e2a-%03d", len(items)+1),
			Function:   p.span.Name,
			File:       p.span.File,
			Caller:     cn.Name,
			CallerFile: cn.FilePath,
			Diff:       expCapLines(diff, expE2aMaxDiffLines),
			CallerSrc:  ctext,
		})
	}
	counts.Kept = len(items)
	return items, counts, nil
}

func TestDecideExpE2aItems(t *testing.T) {
	if os.Getenv("FATHOM_DECIDE_EXP") != "1" {
		t.Skip("E2a item builder: set FATHOM_DECIDE_EXP=1 and FATHOM_DECIDE_EXP_E2A_OUT (asks no model)")
	}
	out := os.Getenv("FATHOM_DECIDE_EXP_E2A_OUT")
	if out == "" {
		t.Fatal("FATHOM_DECIDE_EXP_E2A_OUT must name the JSONL file the items go to (scratch, never committed)")
	}
	var sub *expSubject
	for _, s := range expSubjects(t) {
		if s.Name == expE2aStreamFor {
			sub = &s
			break
		}
	}
	if sub == nil {
		t.Fatalf("FATHOM_DECIDE_EXP_SUBJECTS names no %q subject", expE2aStreamFor)
	}
	items, counts, err := expBuildE2aItems(context.Background(), *sub)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	for _, it := range items {
		if err := it.check(); err != nil {
			t.Fatalf("the builder wrote an item E2 would refuse: %v", err)
		}
		line, err := json.Marshal(it)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("E2a: pool %d, examined %d, kept %d, passed over %v; %d items written to %s", counts.Pool, counts.Examined, counts.Kept, counts.Skipped, len(items), out)
	if len(items) < expE2aWant {
		t.Errorf("only %d of %d items: the pool ran out", len(items), expE2aWant)
	}
}
