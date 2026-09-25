// Command appkit is the kit's CLI. Subcommands grow with the roadmap; today
// it carries the two commands the CSS/token chain needs:
//
//	appkit css [tailwindcss args...]  — run the checksum-pinned Tailwind
//	                                    standalone binary (D2). All arguments
//	                                    pass through, so build and --watch
//	                                    are the same command.
//	appkit gen                        — regenerate committed artifacts from
//	                                    CUE sources (design tokens → @theme
//	                                    CSS + DTCG JSON + Go constants, D7).
//	appkit vendor -out DIR            — materialise the kit's L0/L1/L2
//	                                    artifacts plus a kit.lock into a
//	                                    non-Go consumer's tree; --check
//	                                    re-hashes and exits 1 on drift
//	                                    (ADR-0017 §4). See vendor.go.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/internal/tailwind"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "appkit: %v\n", err)
		os.Exit(command.Code(err))
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		return fmt.Errorf("usage: appkit <css|gen|vendor|data|adoption> [args]")
	}
	switch os.Args[1] {
	case "css":
		return tailwind.Run(ctx, os.Args[2:]...)
	case "gen":
		return genTokens(os.Args[2:])
	case "vendor":
		return vendorCmd(os.Args[2:])
	case "data":
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "assets-evidence":
				return dataAssetsEvidence(os.Args[3:])
			case "assets-evidence-get":
				return dataAssetsEvidenceGet(os.Args[3:])
			case "assets-evidence-save":
				return dataAssetsEvidenceSave(os.Args[3:])
			}
		}
		return dataCmd(ctx, os.Args[2:])
	case "adoption":
		return adoptionCmd(ctx, os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q (want css, gen, vendor, data, or adoption)", os.Args[1])
	}
}

// genTokens regenerates token artifacts from a CUE token set. With no flags
// it regenerates the kit's own committed artifacts (run from the repo root);
// apps point the flags at their own tokens.cue and output paths. Terminal
// output is implicit only for the default token input; custom inputs must
// explicitly select -terminal to generate a terminal palette.
func genTokens(args []string) error {
	const defaultTokensPath = "cue/tokens/kit.cue"
	fs := flag.NewFlagSet("appkit gen", flag.ContinueOnError)
	var (
		tokensPath = fs.String("tokens", defaultTokensPath, "path to the CUE token set (top-level `tokens:` value)")
		cssOut     = fs.String("css", "appkit/ui/tokens/theme.css", "output path for the Tailwind @theme CSS ('' to skip)")
		dtcgOut    = fs.String("dtcg", "appkit/ui/tokens/tokens.dtcg.json", "output path for the DTCG JSON export ('' to skip)")
		goOut      = fs.String("go", "appkit/ui/tokens/names_gen.go", "output path for generated Go role constants ('' to skip)")
		goPkg      = fs.String("gopkg", "tokens", "package name for the generated Go file")
		termOut    = fs.String("terminal", "appkit/cli/theme/palette_gen.go", "output path for the generated terminal color palette (default only for the default token input; '' to skip)")
		termPkg    = fs.String("terminalpkg", "theme", "package name for the generated terminal palette Go file")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	src, err := os.ReadFile(*tokensPath)
	if err != nil {
		return fmt.Errorf("gen: read token set: %w", err)
	}
	terminalExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "terminal" {
			terminalExplicit = true
		}
	})
	if !terminalExplicit {
		inputInfo, err := os.Stat(*tokensPath)
		if err != nil {
			return fmt.Errorf("gen: inspect token set: %w", err)
		}
		defaultInfo, err := os.Stat(defaultTokensPath)
		// Compare the actual input files, not path strings: absolute paths and
		// aliases of the default input keep its behavior, while a symlink followed
		// by .. can resolve to a different file despite matching filepath.Clean.
		if err != nil || !os.SameFile(inputInfo, defaultInfo) {
			*termOut = ""
		}
	}
	t, err := tokens.Load(src)
	if err != nil {
		return err
	}

	outputs := map[string][]byte{}
	if *cssOut != "" {
		outputs[*cssOut] = []byte(tokens.EmitThemeCSS(t))
	}
	if *dtcgOut != "" {
		outputs[*dtcgOut] = tokens.EmitDTCG(t)
	}
	if *goOut != "" {
		outputs[*goOut] = tokens.EmitGoNames(t, *goPkg)
	}
	if *termOut != "" {
		data, err := tokens.EmitTerminalPalette(t, *termPkg)
		if err != nil {
			return err
		}
		outputs[*termOut] = data
	}
	for path, data := range outputs {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("gen: write %s: %w", path, err)
		}
		fmt.Printf("appkit gen: wrote %s (%d bytes)\n", path, len(data))
	}
	return nil
}
