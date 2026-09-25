package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"charm.land/fang/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/appkit/cli/output"
	"github.com/geoffmilleraz/app-kit/appkit/cli/tui"
	"github.com/geoffmilleraz/app-kit/apps/terminal"
)

type runner func(context.Context, *terminal.Model) error

func main() {
	root := newCommand(func() bool { return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) }, func(ctx context.Context, m *terminal.Model) error {
		// Mouse mode belongs to the model so F2 can release capture live. Do not
		// pass WithMouse, which deliberately forces capture on every frame.
		_, err := tui.Run(ctx, m, tui.WithInput(os.Stdin), tui.WithOutput(os.Stdout), tui.WithColorProfile(output.Detect(os.Stdout, os.Environ())))
		return err
	})
	// Bubble Tea owns this interactive command's SIGINT/SIGTERM lifecycle.
	// Fang's concurrent SIGINT context cancellation can strand Tea v2.0.9's
	// signal sender during shutdown. Override only here, not the shared CLI default.
	os.Exit(command.Execute(context.Background(), root, fang.WithNotifySignal()))
}

type report struct {
	SchemaVersion int           `json:"schema_version"`
	Outcome       string        `json:"outcome"`
	ExitCode      int           `json:"exit_code"`
	Page          *catalog.Page `json:"page,omitempty"`
	Error         string        `json:"error,omitempty"`
}

func newCommand(terminalReady func() bool, run runner) *cobra.Command {
	root := command.New(command.App{Name: "appkit-terminal", Short: "Inspect the shared capability catalog in a terminal.", Version: "development"})
	var query, profile string
	var mouse, machine bool
	root.Flags().StringVar(&query, "query", "", "initial catalog text filter")
	root.Flags().StringVar(&profile, "profile", "", "exact capability runtime profile (not the terminal host)")
	root.Flags().BoolVar(&mouse, "mouse", false, "start with mouse capture; F2 toggles it during the session")
	root.Flags().BoolVar(&machine, "json", false, "one bounded catalog-search JSON report; no terminal or input required")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return command.Exit(2, err) })
	root.Args = func(cmd *cobra.Command, args []string) error { return command.Exit(2, cobra.NoArgs(cmd, args)) }
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		c, err := catalog.Default()
		if err == nil {
			err = cmd.Context().Err()
		}
		if machine {
			r := report{SchemaVersion: 1, Outcome: "observed"}
			if err == nil {
				page, queryErr := c.Search(catalog.Query{Text: query, Profile: profile, Limit: 50})
				err = queryErr
				if err == nil {
					r.Page = &page
				}
			}
			if err != nil {
				r.Outcome, r.ExitCode, r.Error = "could-not-inspect", 2, err.Error()
			}
			if writeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(r); writeErr != nil {
				return command.Exit(2, writeErr)
			}
			return command.Exit(2, err)
		}
		if err != nil {
			return command.Exit(2, err)
		}
		if !terminalReady() {
			return command.Exit(2, errors.New("interactive catalog requires terminal input and output; use --json for pipes"))
		}
		m, err := terminal.New(c, terminal.Options{Mouse: mouse, Query: query, Profile: profile})
		if err != nil {
			return command.Exit(2, err)
		}
		return command.Exit(2, run(cmd.Context(), m))
	}
	return root
}
