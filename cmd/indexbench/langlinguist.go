package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

var linguistRevArg = regexp.MustCompile(`^[0-9a-f]{40}$`)

// gitHeadTimeout bounds the one `git rev-parse` that checks a checkout's pin.
const gitHeadTimeout = 30 * time.Second

// cmdLangLinguist is `indexbench lang-linguist` (LB-F48, LB-F49, design §5,
// §13.2 step 9): github-linguist's classification data for the queue items,
// and a draft manifest match block from it.
//
//	indexbench lang-linguist -linguist <checkout> -rev <40-hex> [-queue <file>] [-out <file>]
//	indexbench lang-linguist -match <id> [-data <file>]
//
// The first form imports lib/linguist/languages.yml and heuristics.yml from
// a checkout at the pinned revision into the data file, re-reading the
// hand-kept mapping (items.<id>.linguist, items.<id>.allow_filenames) from
// the file it is about to overwrite. The second prints the draft match
// block for one item from the committed data, no checkout needed; what a
// worker must decide about it is written to stderr as warnings. Nothing
// here changes how the connector classifies a file.
func cmdLangLinguist(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-linguist", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	checkout := fs.String("linguist", "", "a github-linguist/linguist checkout to import (its lib/linguist/*.yml)")
	rev := fs.String("rev", "", "the checkout's commit, 40-hex; a git checkout must be at it")
	queue := fs.String("queue", "", "queue file (default <root>/docs/plans/language-breadth.queue.json)")
	out := fs.String("out", "", "data file written, and whose mapping is re-read (default <root>/docs/plans/language-breadth.linguist.json)")
	match := fs.String("match", "", "print the draft match block for this queue id from the committed data")
	data := fs.String("data", "", "with -match, the data file read (default <root>/docs/plans/language-breadth.linguist.json)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *queue == "" {
		*queue = filepath.Join(*root, "docs", "plans", "language-breadth.queue.json")
	}
	if *out == "" {
		*out = langbench.LinguistDataPath(*root)
	}
	if *data == "" {
		*data = langbench.LinguistDataPath(*root)
	}
	q, err := langbench.LoadQueue(*queue)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}

	if *match != "" {
		return linguistMatch(*root, *match, *data, q, stdout, stderr)
	}
	if *checkout == "" || *rev == "" {
		fmt.Fprintln(stderr, "lang-linguist: -linguist <checkout> -rev <40-hex> to import, or -match <id> to draft")
		return 2
	}
	if !linguistRevArg.MatchString(*rev) {
		fmt.Fprintf(stderr, "lang-linguist: -rev %q must be a 40-hex commit\n", *rev)
		return 2
	}
	return linguistImport(*checkout, *rev, *out, q, stdout, stderr)
}

func linguistImport(checkout, rev, out string, q []langbench.QueueEntry, stdout, stderr io.Writer) int {
	if head, ok := gitHead(checkout); ok && head != rev {
		fmt.Fprintf(stderr, "lang-linguist: %s is at %s, not the pinned -rev %s\n", checkout, head, rev)
		return 1
	}
	mapping, err := langbench.LoadLinguistMapping(out)
	if err != nil {
		fmt.Fprintf(stderr, "lang-linguist: the id-to-name mapping is read from %s (items.<id>.linguist): %v\n", out, err)
		return 1
	}
	languages, heuristics, err := langbench.ReadLinguistCheckout(checkout)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	d, err := langbench.ImportLinguist(languages, heuristics, rev, q, mapping)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	b, err := langbench.MarshalLinguist(d)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	var exts, files, interps, blocks int
	for _, it := range d.Items {
		exts += len(it.Extensions)
		files += len(it.Filenames)
		interps += len(it.Interpreters)
		blocks += len(it.Heuristics)
	}
	fmt.Fprintf(stdout, "lang-linguist: %d items from %s@%s: %d extensions, %d filenames, %d interpreters; %d heuristic blocks (%d patterns, %d dropped as not RE2, %d rules dropped)\nlang-linguist: wrote %s\n",
		len(d.Items), d.Source, rev[:12], exts, files, interps, blocks, d.Stats.Patterns, d.Stats.Dropped, d.Stats.DroppedRules, out)
	return 0
}

func linguistMatch(root, id, dataPath string, q []langbench.QueueEntry, stdout, stderr io.Writer) int {
	known := false
	for _, e := range q {
		known = known || e.ID == id
	}
	if !known {
		fmt.Fprintf(stderr, "lang-linguist: %q is not a queue id\n", id)
		return 1
	}
	d, err := langbench.LoadLinguistData(dataPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	registered, err := langbench.RegisteredMatches(root)
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	m, warns, err := langbench.DraftMatchFromLinguist(d, id, langbench.LinguistExtensionClaims(q, d, registered))
	if err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	// An extension the connector already classifies natively is not another
	// dialect's claim, but the worker must know it (lang-disclosure's rule).
	exts := append([]string{}, m.Extensions...)
	if m.Sniff != nil {
		exts = append(exts, m.Sniff.Extensions...)
	}
	for _, x := range exts {
		if lang := connector.LanguageForPath("x" + x); lang != core.LangUnclassified {
			warns = append(warns, fmt.Sprintf("extension %s is already classified by the connector as %s", x, lang))
		}
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "lang-linguist: warning:", w)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintln(stderr, "lang-linguist:", err)
		return 1
	}
	_, _ = stdout.Write(buf.Bytes())
	return 0
}

// gitHead is the HEAD of dir when dir is itself a git checkout. The
// environment's GIT_* variables (a hook's GIT_DIR) are dropped so they can
// never point the question at some other repository.
func gitHead(dir string) (string, bool) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitHeadTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	b, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}
