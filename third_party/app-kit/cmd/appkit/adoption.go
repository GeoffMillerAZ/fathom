package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/appkit/cli/output"
	"github.com/geoffmilleraz/app-kit/internal/adoption"
)

func adoptionCmd(ctx context.Context, args []string) error {
	return runAdoption(ctx, args, os.Stdout, os.Stderr)
}

func runAdoption(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	usage := "usage: appkit adoption <inspect|check|pins|router> [flags]"
	if len(args) == 0 {
		return command.Exit(2, errors.New(usage))
	}
	if args[0] == "-h" || args[0] == "--help" {
		_, err := fmt.Fprintln(stdout, usage+"\nRead-only static evidence. Does not prove runtime adoption or host invocation.")
		return command.Exit(2, err)
	}
	mode := args[0]
	if mode != "inspect" && mode != "check" && mode != "pins" && mode != "router" {
		return command.Exit(2, errors.New(usage))
	}
	fs := flag.NewFlagSet("appkit adoption "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	if mode == "router" {
		contract := fs.String("contract", "docs/app-kit.md", "consumer-owned relative documentation path; prints only, never installs")
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return command.Exit(2, err)
		}
		if fs.NArg() != 0 {
			return command.Exit(2, errors.New("adoption router accepts flags only"))
		}
		block, err := adoption.Router(*contract)
		if err != nil {
			return command.Exit(2, err)
		}
		_, err = io.WriteString(stdout, block)
		return command.Exit(2, err)
	}
	root := fs.String("root", ".", "explicit consumer directory; no recursive fleet discovery")
	manifest := fs.String("manifest", adoption.DefaultManifest, "root-relative consumer-owned adoption JSON")
	machine := fs.Bool("json", false, "one versioned JSON report; no progress decorations on stdout")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return command.Exit(2, err)
	}
	if fs.NArg() != 0 || *root == "" || !adoption.SafePath(*manifest) {
		return command.Exit(2, errors.New("adoption requires a nonempty root, canonical relative manifest path and no positional arguments"))
	}
	var report adoption.Report
	var pins adoption.PinReport
	if mode == "check" {
		report = adoption.Check(ctx, *root, *manifest)
	} else if mode == "pins" {
		pins = adoption.Pins(ctx, *root, *manifest)
		report = pins.Report
	} else {
		report = adoption.Inspect(ctx, *root, *manifest)
	}
	var err error
	if *machine {
		if mode == "pins" {
			err = json.NewEncoder(stdout).Encode(pins)
		} else {
			err = json.NewEncoder(stdout).Encode(report)
		}
	} else {
		// RenderStatusTable has a legacy no-error signature; buffer its bounded
		// output so writer failures still become could-not-report exit 2.
		var b bytes.Buffer
		fmt.Fprintf(&b, "app-kit adoption %s: static evidence only\n", mode)
		fmt.Fprintf(&b, "catalog %s\n", report.CatalogRevision)
		fmt.Fprintf(&b, "checker %q; Go %q; %s/%s; VCS %q\n", report.CheckerBuild.MainVersion, report.CheckerBuild.GoVersion, report.CheckerBuild.GOOS, report.CheckerBuild.GOARCH, report.CheckerBuild.VCSRevision)
		if report.CheckerBuild.VCSModified == nil {
			fmt.Fprintln(&b, "checker VCS modified: unknown (not consumer revision proof)")
		} else {
			fmt.Fprintf(&b, "checker VCS modified: %t (not a working-tree content hash)\n", *report.CheckerBuild.VCSModified)
		}
		if report.Selection != nil {
			fmt.Fprintf(&b, "consumer %q; profile %q; layers %q\n", report.Selection.Consumer, report.Selection.Profile, report.Selection.Layers)
		}
		if report.Module != nil {
			fmt.Fprintf(&b, "declared kit version %q; resolution %s\n", report.Module.DeclaredVersion, report.Module.Resolution)
			if report.Module.Replacement != nil {
				fmt.Fprintf(&b, "declared replacement path %q version %q (not followed)\n", report.Module.Replacement.Path, report.Module.Replacement.Version)
			}
		}
		rows := make([]output.StatusRow, 0, len(report.Checks))
		for _, finding := range report.Checks {
			status := map[string]string{"pass": "ok", "fail": "fail", "unknown": "unknown", "not-applicable": "n/a"}[finding.Status]
			detail := finding.Detail
			if finding.Path != "" {
				detail = fmt.Sprintf("%q: %s", finding.Path, detail)
			}
			rows = append(rows, output.StatusRow{ID: finding.ID, Status: status, Detail: detail, Remedy: finding.Remedy})
		}
		output.RenderStatusTable(&b, rows, output.Detect(stdout, os.Environ()))
		if mode == "pins" {
			fmt.Fprintln(&b, "\nSelected capability pins (metadata identity only):")
			for _, pin := range pins.CapabilityRevisions {
				fmt.Fprintf(&b, "%s  %s\n", pin.ID, pin.SHA256)
			}
			if len(pins.Entries) > 0 {
				fmt.Fprintf(&b, "Selected metadata entries: %d (full entries are present in JSON output).\n", len(pins.Entries))
			}
		}
		fmt.Fprintf(&b, "\nOutcome: %s (static %s only).\n", report.Outcome, mode)
		for _, limitation := range report.Limitations {
			fmt.Fprintln(&b, limitation)
		}
		_, err = stdout.Write(b.Bytes())
	}
	if err != nil {
		return command.Exit(2, fmt.Errorf("adoption: write report: %w", err))
	}
	if report.ExitCode != 0 {
		return command.Exit(report.ExitCode, fmt.Errorf("adoption %s: %s; see report", mode, report.Outcome))
	}
	return nil
}
