package main

import (
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/recordids"
)

func cmdGateRecordIDs(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("gate record-ids", stderr)
	root := fs.String("root", ".", "repository root")
	path := fs.String("config", "", "JSON family/exception config (default .fathom/record-ids.json when present)")
	cross := fs.Bool("cross-plan", false, "also check ids across active plans, honoring exact grandfathered paths")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: record-ids takes no positional arguments")
		return 1
	}
	cfg := recordids.Defaults()
	configPath := *path
	if configPath == "" {
		candidate := projectdir.File("record-ids.json").Path(*root)
		if _, err := os.Stat(candidate); err == nil {
			configPath = candidate
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	}
	if configPath != "" {
		var err error
		cfg, err = recordids.Load(configPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	}
	if *cross {
		cfg.CrossPlan = true
	}
	dups, err := recordids.Check(*root, cfg)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	for _, d := range dups {
		fmt.Fprintln(stderr, d.String())
	}
	if len(dups) != 0 {
		return 1
	}
	fmt.Fprintln(stdout, "record ids: unique")
	return 0
}
