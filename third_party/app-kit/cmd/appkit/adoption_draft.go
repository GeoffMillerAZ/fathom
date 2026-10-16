package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/adoption"
)

func runAdoptionDraft(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("appkit adoption draft", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: appkit adoption draft --root DIR --consumer SLUG --profile PROFILE --contract PATH [flags]")
		fmt.Fprintln(stderr, "Print a reviewable version-2 JSON draft, not an adoption check or upgrade approval.")
		fmt.Fprintln(stderr, "Drafting does not prove runtime adoption, artifact origin or host activation.")
		fmt.Fprintln(stderr, "No writes or existing-manifest reads. Review selected metadata and files before saving; then run adoption check.")
		fs.PrintDefaults()
	}
	var opts adoption.DraftOptions
	root := fs.String("root", "", "explicit consumer directory (required)")
	fs.StringVar(&opts.Consumer, "consumer", "", "consumer slug (required)")
	fs.StringVar(&opts.Profile, "profile", "", "explicit catalog profile (required)")
	fs.StringVar(&opts.Contract, "contract", "", "root-relative human contract path (required; not read)")
	fs.StringVar(&opts.ExpectedCatalogRevision, "expected-catalog-revision", "", "optional whole-catalog revision from reviewed discovery; mismatch refuses")
	addValues := func(name, help string, dst *[]string, limit int) {
		fs.Func(name, help+"; repeat once per value", func(value string) error {
			if len(*dst) >= limit {
				return fmt.Errorf("too many --%s values (maximum %d)", name, limit)
			}
			*dst = append(*dst, value)
			return nil
		})
	}
	addValues("layer", "selected catalog layer", &opts.Layers, 6)
	addValues("capability", "selected exact capability ID", &opts.Capabilities, 64)
	addValues("artifact", "existing root-relative regular file to hash", &opts.Artifacts, 64)
	fs.BoolVar(&opts.Agents, "agents", false, "require the canonical router in AGENTS.md when checked (a CLAUDE.md, if any, must import or carry it); does not install it")
	moduleFile := fs.String("module-file", "", "optional root-relative module declaration file (requires --module-version; not read)")
	moduleVersion := fs.String("module-version", "", "explicit exact kit version (requires --module-file; never inferred)")
	allowLocal := fs.Bool("allow-local-replace", false, "explicitly allow development replacement for the selected module only")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return command.Exit(2, err)
	}
	if fs.NArg() != 0 || (*moduleFile == "") != (*moduleVersion == "") || *allowLocal && *moduleFile == "" {
		return command.Exit(2, errors.New("adoption draft accepts flags only; module-file and module-version must be selected together, and allow-local-replace requires that module"))
	}
	if *moduleFile != "" {
		opts.Module = &adoption.ModuleRequirement{File: *moduleFile, Version: *moduleVersion, AllowLocalReplace: *allowLocal}
	}
	s, err := adoption.Draft(ctx, *root, opts)
	if err != nil {
		return command.Exit(2, err)
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		body = append(body, '\n')
		var n int
		n, err = stdout.Write(body)
		if err == nil && n != len(body) {
			err = io.ErrShortWrite
		}
	}
	if err != nil {
		return command.Exit(2, fmt.Errorf("adoption draft: write JSON: %w", err))
	}
	return nil
}
