// Command iaccatalog builds IaC provider catalogs from a pin list, deterministically, and receipts each one. It is a
// maintainer tool outside the product binary: it may run OpenTofu against a filesystem mirror of archives that are
// already on disk, which the product never does (I56, docs/design/iac-facts.md section 4). It fetches nothing and
// publishes nothing. Replacement facts come from a later Go source scan (IF-13), so every catalog it writes says
// replacement is absent.
//
//	iaccatalog -pins calibration/iac/pins.yaml -out DIR -exports DIR [-source hashicorp/aws] [-line 5] [-tool NAME]
//	iaccatalog -pins calibration/iac/pins.yaml -out DIR -archives DIR [-tofu tofu]
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("iaccatalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pins := fs.String("pins", "calibration/iac/pins.yaml", "the pin list")
	out := fs.String("out", "", "directory for catalogs and receipts (required)")
	exports := fs.String("exports", "", "directory of exports already made, named <namespace>-<name>_<version>.json")
	archives := fs.String("archives", "", "directory of pinned provider archives; each is hashed against its pin, then exported under OpenTofu")
	tofu := fs.String("tofu", "tofu", "the OpenTofu executable, with -archives")
	tool := fs.String("tool", "", "with -exports: the tool and version that made them, recorded in the manifest")
	source := fs.String("source", "", "build only this provider, namespace/name")
	line := fs.Int("line", 0, "build only this major line")
	work := fs.String("work", "", "scratch directory (default: a temp directory next to -out)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" || (*exports == "") == (*archives == "") {
		fmt.Fprintln(stderr, "usage: iaccatalog -pins FILE -out DIR (-exports DIR [-tool NAME] | -archives DIR [-tofu PATH]) [-source ns/name] [-line N]")
		return 2
	}
	pf, err := loadPins(*pins)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	lines := pf.selectLines(*source, *line)
	if len(lines) == 0 {
		fmt.Fprintln(stderr, "error: no pinned line matches")
		return 1
	}
	scratch := *work
	if scratch == "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		d, err := os.MkdirTemp(*out, ".work-*")
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		defer os.RemoveAll(d)
		scratch = d
	}
	var ex exporter
	if *exports != "" {
		if *tool == "" {
			fmt.Fprintln(stderr, "error: -exports needs -tool, the tool and version that made the exports")
			return 2
		}
		ex = dirExporter{dir: *exports, tool: *tool}
	} else {
		ex = tofuExporter{tofu: *tofu, archives: *archives, work: scratch}
	}
	failed := false
	for _, l := range lines {
		rec, err := buildCatalog(context.Background(), l, ex, *out, filepath.Join(scratch, fmt.Sprintf("%s-%d", filepath.Base(l.Source), l.Line)))
		if err != nil {
			fmt.Fprintf(stderr, "error: %s line %d: %v\n", l.Source, l.Line, err)
			failed = true
			continue
		}
		fmt.Fprintf(stdout, "%s sha256=%s bytes=%d blobs=%d releases=%d\n", rec.Catalog, rec.SHA256, rec.Bytes, rec.Blobs, len(rec.Releases))
	}
	if failed {
		return 1
	}
	return 0
}
