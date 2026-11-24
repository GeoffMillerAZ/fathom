package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// outlineVersion is the schema_version of outline's JSON output. Provisional (CH-D6).
const outlineVersion = 1

// outlineFile is one entry of outline's JSON output. Outline is null when the file could not be read.
type outlineFile struct {
	Path    string           `json:"path"`
	Error   string           `json:"error"`
	Outline *view.OutlineDoc `json:"outline"`
}

// cmdOutline prints the shape of each PATH: its symbols nested by span containment, as text or JSON. It reads on
// extract's read path (supervised unless -in-process) and adds no text the read does not carry.
func cmdOutline(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("outline", flag.ContinueOnError)
	root := fs.String("root", ".", "directory the PATH arguments are relative to")
	format := fs.String("format", "text", "output form: text or json")
	maxNodes := fs.Int("max-nodes", 0, "the most nodes one file's outline holds, deepest level dropped first; 0 means no limit")
	deadline := fs.Duration("deadline", lang.DefaultReadDeadline, "the time one file's read may take; it may only be lowered")
	maxMemory := fs.String("max-memory", defaultMaxMemory, "the memory the supervised child may hold, such as 512MiB or 4GiB")
	inProcess := fs.Bool("in-process", false, "read in this process, with no supervised child")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if *format != "text" && *format != "json" {
		return usageError{fmt.Errorf("-format %q: want text or json", *format)}
	}
	if *maxNodes < 0 {
		return usageError{fmt.Errorf("-max-nodes %d must be 0 or more", *maxNodes)}
	}
	if *deadline <= 0 || *deadline > lang.DefaultReadDeadline {
		return usageError{fmt.Errorf("-deadline %v must be above 0 and at most %v", *deadline, lang.DefaultReadDeadline)}
	}
	memory, err := parseSize(*maxMemory)
	if err != nil {
		return usageError{err}
	}
	if fs.NArg() == 0 {
		return usageError{errors.New("outline needs at least one PATH")}
	}
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	cfg := readConfig{deadline: *deadline}
	var files []fileResult
	var unreadable bool
	if *inProcess {
		files, unreadable, err = extractInProcess(ctx, cfg, rootAbs, nil, fs.Args())
	} else {
		files, unreadable, err = extractSupervised(ctx, cfg, rootAbs, "", memory, nil, fs.Args())
	}
	if err != nil {
		return err
	}
	opts := view.OutlineOptions{MaxNodes: *maxNodes}
	entries := make([]outlineFile, 0, len(files))
	for _, f := range files {
		e := outlineFile{Path: f.Path, Error: f.Error}
		if f.Result != nil {
			d := view.Outline(f.Path, *f.Result, opts)
			e.Outline = &d
		} else if e.Error == "" {
			e.Error = "no result"
		}
		entries = append(entries, e)
	}
	if *format == "json" {
		err = writeJSON(out, struct {
			SchemaVersion int           `json:"schema_version"`
			Files         []outlineFile `json:"files"`
		}{outlineVersion, entries})
	} else {
		err = writeOutlineText(out, entries)
	}
	if err != nil {
		return err
	}
	if unreadable {
		return errUnreadable
	}
	return nil
}

// writeOutlineText prints each file's text form in order. A file that could not be read prints one line with its
// error and no nodes.
func writeOutlineText(out io.Writer, entries []outlineFile) error {
	for _, e := range entries {
		var err error
		if e.Outline != nil {
			_, err = io.WriteString(out, e.Outline.Text())
		} else {
			_, err = fmt.Fprintf(out, "# %s error=%s\n", e.Path, e.Error)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
