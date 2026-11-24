package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// atVersion is the schema_version of at's output. Provisional (CH-D6).
const atVersion = 1

// atRequest is one PATH:LINE[-END] argument.
type atRequest struct {
	path       string
	start, end int
}

// atAnswer is one entry of at's output. Enclosing is null when there is no
// candidate symbol (none, a skipped file, an unreadable file); Touched is an
// empty array, never null.
type atAnswer struct {
	Path      string        `json:"path"`
	Start     int           `json:"start"`
	End       int           `json:"end"`
	Status    view.Status   `json:"status"`
	Enclosing *lang.Symbol  `json:"enclosing"`
	Touched   []lang.Symbol `json:"touched"`
	Complete  bool          `json:"complete"`
	Error     string        `json:"error"`
}

// parseAtQuery reads PATH:LINE or PATH:LINE-END. The split is at the last colon.
func parseAtQuery(arg string) (atRequest, error) {
	i := strings.LastIndex(arg, ":")
	if i <= 0 || i == len(arg)-1 {
		return atRequest{}, fmt.Errorf("%q is not PATH:LINE or PATH:LINE-END", arg)
	}
	lines := arg[i+1:]
	if lines[0] == '-' {
		return atRequest{}, fmt.Errorf("%q: line must be 1 or more", arg)
	}
	startText, endText, isRange := strings.Cut(lines, "-")
	start, err := strconv.Atoi(startText)
	if err != nil || startText[0] == '+' {
		return atRequest{}, fmt.Errorf("%q: line %q is not a number", arg, startText)
	}
	if start < 1 {
		return atRequest{}, fmt.Errorf("%q: line must be 1 or more", arg)
	}
	end := start
	if isRange {
		end, err = strconv.Atoi(endText)
		if err != nil || endText[0] == '+' || endText[0] == '-' {
			return atRequest{}, fmt.Errorf("%q: end line %q is not a number", arg, endText)
		}
		if end < start {
			return atRequest{}, fmt.Errorf("%q: end line %d is before start line %d", arg, end, start)
		}
	}
	return atRequest{path: arg[:i], start: start, end: end}, nil
}

// cmdAt answers which symbol holds each PATH:LINE[-END] and which symbols it touches. Each distinct path is read
// once, through extract's read path (supervised unless -in-process), and the queries on it share that read.
func cmdAt(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("at", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the PATH parts are relative to")
	deadline := fs.Duration("deadline", lang.DefaultReadDeadline, "the time one file's read may take; it may only be lowered")
	maxMemory := fs.String("max-memory", defaultMaxMemory, "the memory the supervised child may hold, such as 512MiB or 4GiB")
	inProcess := fs.Bool("in-process", false, "read in this process, with no supervised child")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if *deadline <= 0 || *deadline > lang.DefaultReadDeadline {
		return usageError{fmt.Errorf("-deadline %v must be above 0 and at most %v", *deadline, lang.DefaultReadDeadline)}
	}
	memory, err := parseSize(*maxMemory)
	if err != nil {
		return usageError{err}
	}
	if fs.NArg() == 0 {
		return usageError{errors.New("at needs at least one PATH:LINE[-END]")}
	}
	queries := make([]atRequest, 0, fs.NArg())
	for _, a := range fs.Args() {
		q, err := parseAtQuery(a)
		if err != nil {
			return usageError{err}
		}
		queries = append(queries, q)
	}
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	// One read per distinct path, in order of first appearance.
	var paths []string
	seen := map[string]bool{}
	for _, q := range queries {
		_, shown, _ := resolvePath(rootAbs, q.path)
		if !seen[shown] {
			seen[shown] = true
			paths = append(paths, q.path)
		}
	}
	cfg := readConfig{deadline: *deadline}
	var files []fileResult
	var unreadable bool
	if *inProcess {
		files, unreadable, err = extractInProcess(ctx, cfg, rootAbs, nil, paths)
	} else {
		files, unreadable, err = extractSupervised(ctx, cfg, rootAbs, "", memory, nil, paths)
	}
	if err != nil {
		return err
	}
	type read struct {
		ix  *view.Index
		err string
	}
	reads := map[string]read{}
	for _, f := range files {
		r := read{err: f.Error}
		if f.Result != nil {
			r.ix = view.NewIndex(*f.Result)
		}
		reads[f.Path] = r
	}
	answers := make([]atAnswer, 0, len(queries))
	for _, q := range queries {
		_, shown, _ := resolvePath(rootAbs, q.path)
		answers = append(answers, answerAt(shown, q, reads[shown].ix, reads[shown].err))
	}
	if err := writeJSON(out, struct {
		SchemaVersion int        `json:"schema_version"`
		Queries       []atAnswer `json:"queries"`
	}{atVersion, answers}); err != nil {
		return err
	}
	if unreadable {
		return errUnreadable
	}
	return nil
}

// answerAt answers one query from its file's Index. A file with no Index is unknown.
func answerAt(shown string, q atRequest, ix *view.Index, errText string) atAnswer {
	a := atAnswer{Path: shown, Start: q.start, End: q.end, Status: view.StatusUnknown, Touched: []lang.Symbol{}, Error: errText}
	if ix == nil {
		return a
	}
	enc, st := ix.EnclosingRange(q.start, q.end)
	a.Status = st
	if st == view.StatusFound || (st == view.StatusUnknown && enc != (lang.Symbol{})) {
		a.Enclosing = &enc
	}
	touched, complete := ix.Touched(q.start, q.end)
	if touched != nil {
		a.Touched = touched
	}
	a.Complete = complete
	return a
}
