package main

// report_used.go — `fathom report used <dir> [--json]` (SV-7, ADR-060
// decision 6, vision 41 §4.5): a dependency directory's used closure from
// the consumer's product entry points, each unit with its entry point and
// first importer, and the unreached remainder. A sub-report beside `report
// evidence` and `report packs`: no new verb, no MCP tool. It copies, prunes
// and vendors nothing (SV-X4); a pruning script reads its JSON.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/internal/usedset"
)

func cmdReportUsed(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("report used", stderr)
	root := fs.String("root", "", "the consumer module's root, holding the go.mod that replaces <dir> (default: the module enclosing the current directory)")
	var mains, css repeatedFlag
	fs.Var(&mains, "main", "an entry-point main package directory, relative to the root (repeatable; default: every main package outside <dir> and workspace/)")
	fs.Var(&css, "css", "a CSS entry file whose @source and @import declare asset roots, relative to the root (repeatable; default: every .css file declaring one into <dir>)")
	jsonOut := fs.Bool("json", false, "emit JSON")
	// <dir> may come before or after the flags.
	var dir string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		dir, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if dir == "" && fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if dir == "" {
		fmt.Fprintln(stderr, "error: report used needs a dependency directory: fathom report used <dir> [--json]")
		return 1
	}
	cwd, _ := os.Getwd()
	r := *root
	if r == "" {
		r = enclosingModuleRoot(cwd)
		if r == "" {
			fmt.Fprintln(stderr, "error: no go.mod encloses the current directory; pass --root")
			return 1
		}
	}
	if !filepath.IsAbs(dir) {
		if st, err := os.Stat(filepath.Join(cwd, dir)); err == nil && st.IsDir() && *root == "" {
			dir = filepath.Join(cwd, dir)
		}
	}
	opt := usedset.Options{Root: r, Dir: dir}
	if len(mains) > 0 {
		opt.Mains = mains
	}
	if len(css) > 0 {
		opt.CSS = css
	}
	res, err := usedset.UsedUnits(opt)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}
	renderReportUsed(stdout, res)
	return 0
}

// enclosingModuleRoot is the nearest directory at or above dir holding a
// go.mod, preferring the git toplevel's when it has one (a mirror's own
// go.mod under the toplevel is the dependency, not the consumer).
func enclosingModuleRoot(dir string) string {
	if top := gitToplevel(dir); top != "" {
		if _, err := os.Stat(filepath.Join(top, "go.mod")); err == nil {
			return top
		}
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

func renderReportUsed(w io.Writer, res usedset.Result) {
	kinds := map[string]int{}
	for _, u := range res.Used {
		kinds[u.Kind]++
	}
	fmt.Fprintf(w, "fathom report used — %s (module %s) from %d entry %s\n", res.Dir, res.Module, len(res.Entries), pluralWord(len(res.Entries), "point", "points"))
	fmt.Fprintf(w, "used: %d of %d Go packages; %d embedded files, %d css sources, %d css imports, %d TS/JS modules, %d module files\n",
		res.Totals.UsedGoPackages, res.Totals.GoPackages, kinds[usedset.KindEmbed], kinds[usedset.KindCSSSource], kinds[usedset.KindCSSImport], kinds[usedset.KindJSModule], kinds[usedset.KindModuleFile])
	fmt.Fprintf(w, "files: %d of %d used (%s of %s)\n", res.Totals.UsedFiles, res.Totals.Files, tenancy.FormatBytes(res.Totals.UsedBytes), tenancy.FormatBytes(res.Totals.Bytes))
	if len(res.CSS) > 0 {
		fmt.Fprintf(w, "css entries: %s\n", strings.Join(res.CSS, ", "))
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "\nkind\tunit\treason\tfiles")
	for _, u := range res.Used {
		why := "from " + u.Entry
		if u.FirstImporter != "" && u.FirstImporter != u.Entry {
			why += " via " + u.FirstImporter
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", u.Kind, u.ID, why, u.Files)
	}
	_ = tw.Flush()
	if len(res.Unreached) == 0 {
		fmt.Fprintln(w, "\nunreached: none")
		return
	}
	fmt.Fprintf(w, "\nunreached (%d units nothing reaches):\n", len(res.Unreached))
	tw = tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, u := range res.Unreached {
		fmt.Fprintf(tw, "%s\t%s\t%d files\t%s\n", u.Kind, u.ID, u.Files, tenancy.FormatBytes(u.Bytes))
	}
	_ = tw.Flush()
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
