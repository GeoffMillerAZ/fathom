package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	harnessregistry "github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/provenance"
)

func cmdProvenance(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom provenance note [<commit>] [--harness ID] [--model ID] [--host-label LABEL] [--session ID] [--lane ID]")
		return 1
	}
	switch args[0] {
	case "note":
		return cmdProvenanceNote(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown provenance verb %q (want note)\n", args[0])
		return 1
	}
}

func cmdProvenanceNote(args []string, stdout, stderr io.Writer) int {
	// Like the other positional CLI verbs, accept the commit before flags too.
	var commit string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		commit, args = args[0], args[1:]
	}
	fs := newFlagSet("provenance note", stderr)
	harness := fs.String("harness", "", "declared harness (default FATHOM_HARNESS, then harness session environment)")
	model := fs.String("model", "", "declared agent model (default FATHOM_MODEL)")
	host := fs.String("host-label", "", "operator-supplied host label (default gate.host_label, then unknown; never inferred)")
	session := fs.String("session", "", "declared session (default FATHOM_SESSION_ID, then harness session environment)")
	lane := fs.String("lane", "", "declared lane (default FATHOM_LANE)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if commit == "" && fs.NArg() == 1 {
		commit = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: provenance note accepts at most one commit")
		return 1
	}
	if commit == "" {
		commit = "HEAD"
	}
	if *harness == "" {
		*harness = os.Getenv("FATHOM_HARNESS")
	}
	if *model == "" {
		*model = os.Getenv("FATHOM_MODEL")
	}
	// Resolve the label and session together, including aliases, so the label
	// describes the harness whose session variable supplied the fallback.
	var detectedHarness, detectedSession string
	for _, d := range harnessregistry.All() {
		for _, key := range append(append([]string{}, d.Env.SessionIDAliases...), d.Env.SessionID) {
			if key != "" {
				if value := os.Getenv(key); value != "" {
					detectedHarness, detectedSession = d.Label, value
					break
				}
			}
		}
		if detectedSession != "" {
			break
		}
	}
	if *harness == "" {
		*harness = detectedHarness
	}
	if *session == "" {
		*session = os.Getenv("FATHOM_SESSION_ID")
		if *session == "" {
			*session = detectedSession
		}
	}
	if *lane == "" {
		*lane = os.Getenv("FATHOM_LANE")
	}
	ctx := context.Background()
	if *host == "" {
		root, err := gitro.Output(ctx, ".", "rev-parse", "--show-toplevel")
		if err != nil {
			fmt.Fprintf(stderr, "error: git repository root: %v\n", err)
			return 1
		}
		loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(strings.TrimSpace(string(root))))
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		*host = loaded.Config.Gate.HostLabel
	}
	fields := provenance.Fields{Harness: *harness, Model: *model, HostLabel: *host, Session: *session, Lane: *lane}
	sha, err := provenance.ProvenanceNote(ctx, ".", commit, fields)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Fprintf(stdout, "provenance note: recorded for %s under refs/notes/fathom/provenance\n", sha)
	return 0
}

type provenanceRunner struct{}

func (provenanceRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdProvenance(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "provenance", Group: "review", Order: 110, Run: provenanceRunner{},
		Summary: "provenance note [<commit>]: record declared harness, model, operator host label, session and lane in a local git note (default HEAD); replaces the note without changing the target commit or pushing notes"})
}
