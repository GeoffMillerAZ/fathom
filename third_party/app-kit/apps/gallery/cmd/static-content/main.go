// Command static-content builds the bounded reading reference as plain files.
// It never starts Gallery or opens a browser; see ADR-0071.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/app-kit/apps/gallery"
)

const maxCSSBytes = 128 << 10

var staticBasePath = regexp.MustCompile(`^/(?:[A-Za-z0-9._~-]+/)*$`)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("static-content", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "", "fresh absolute output directory (never overwritten)")
	cssPath := flags.String("css", "", "absolute path to trusted compiled Gallery CSS")
	basePath := flags.String("base-path", "/", "hosting path prefix, ending in / (binds nested 404 recovery links)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !filepath.IsAbs(*out) || !filepath.IsAbs(*cssPath) {
		fmt.Fprintln(stderr, "static-content: supply -out and -css as absolute paths, with no positional arguments")
		return 2
	}
	if !staticBasePath.MatchString(*basePath) || strings.Contains(*basePath, "/./") || strings.Contains(*basePath, "/../") {
		fmt.Fprintln(stderr, "static-content: -base-path must be / or an absolute ASCII-unreserved path ending in / without dot segments")
		return 2
	}
	css, err := readCSS(*cssPath)
	if err == nil {
		err = gallery.ExportStaticContent(*out, css, *basePath)
	}
	if err != nil {
		fmt.Fprintf(stderr, "static-content: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "Static reading files: %s\n", *out); err != nil {
		fmt.Fprintln(stderr, "static-content: files were exported but the result could not be written to stdout")
		return 1
	}
	return 0
}

func readCSS(name string) ([]byte, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("CSS input: %w", err)
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > maxCSSBytes {
		return nil, errors.New("CSS input must be a regular, non-symlink file containing 1–131072 bytes")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("CSS input: %w", err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("CSS input changed while opening it")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCSSBytes+1))
	if err != nil {
		return nil, fmt.Errorf("CSS input: %w", err)
	}
	if len(data) == 0 || len(data) > maxCSSBytes {
		return nil, errors.New("CSS input changed size or exceeds its bound")
	}
	return data, nil
}
