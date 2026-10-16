package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/adoption"
)

// This is the MCP/Gallery exact-lookup projection, not a second catalog or pin
// format. Keep discovery independent of consumer files and host configuration.
type adoptionCapabilityOutput struct {
	Revision           string                      `json:"revision"`
	Entry              catalog.Entry               `json:"entry"`
	CapabilityRevision adoption.CapabilityRevision `json:"capability_revision"`
}

func runAdoptionCatalog(ctx context.Context, mode string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("appkit adoption "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: appkit adoption %s [flags]\n", mode)
		fmt.Fprintln(stderr, "Print first-party metadata as JSON; no host, consumer files, network or licensed corpus.")
		fmt.Fprintln(stderr, "Retrieval is not adoption proof. Review exact metadata and gaps before drafting; rebuild to refresh the embedded catalog.")
		fs.PrintDefaults()
	}
	var query catalog.Query
	var id, expected string
	if mode == "search" {
		fs.StringVar(&query.Text, "query", "", "task phrase; same search semantics as MCP/Gallery")
		fs.StringVar(&query.Layer, "layer", "", "exact layer: L0, L1, L2, L3a, L3b, L4")
		fs.StringVar(&query.Profile, "profile", "", "exact profile: portable, go-web, static-web, react-web, cli")
		fs.StringVar(&query.Kind, "kind", "", "exact kind: token, primitive, recipe, reference")
		fs.StringVar(&query.Maturity, "maturity", "", "exact maturity: experimental, available, proven; not consumer acceptance")
		fs.IntVar(&query.Limit, "limit", 10, "page size (1..50); zero also selects the shared default of 10")
		fs.StringVar(&query.Cursor, "cursor", "", "prior next_cursor; retain query, filters and limit")
	} else {
		fs.StringVar(&id, "id", "", "exact capability ID from search (required)")
		fs.StringVar(&expected, "expected-catalog-revision", "", "optional whole-catalog search revision; not a capability pin")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return command.Exit(2, err)
	}
	if fs.NArg() != 0 || mode == "capability" && id == "" {
		return command.Exit(2, errors.New("adoption discovery accepts flags only; capability requires --id from search"))
	}
	if err := ctx.Err(); err != nil {
		return command.Exit(2, err)
	}
	c, err := catalog.Default()
	if err != nil {
		return command.Exit(2, err)
	}
	var value any
	if mode == "search" {
		value, err = c.Search(query)
	} else {
		if expected != "" {
			if len(expected) != 64 {
				return command.Exit(2, errors.New("invalid --expected-catalog-revision: must be exactly 64 lowercase hexadecimal characters"))
			}
			_, decodeErr := hex.DecodeString(expected)
			if decodeErr != nil || expected != strings.ToLower(expected) {
				return command.Exit(2, errors.New("invalid --expected-catalog-revision: must be exactly 64 lowercase hexadecimal characters"))
			}
			if expected != c.Revision {
				return command.Exit(2, fmt.Errorf("catalog revision mismatch: expected %s, current %s; restart adoption search and review the selection", expected, c.Revision))
			}
		}
		entry, ok := c.Get(id)
		if !ok {
			return command.Exit(2, fmt.Errorf("unknown capability %q; use adoption search for exact IDs", id))
		}
		var pin adoption.CapabilityRevision
		pin, err = adoption.PinCapability(c.SchemaVersion, entry)
		value = adoptionCapabilityOutput{Revision: c.Revision, Entry: entry, CapabilityRevision: pin}
	}
	if err != nil {
		return command.Exit(2, err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return command.Exit(2, err)
	}
	body = append(body, '\n')
	if len(body) > 128*1024 {
		return command.Exit(2, errors.New("adoption discovery exceeds the 128KiB response limit; no metadata returned"))
	}
	if err := ctx.Err(); err != nil {
		return command.Exit(2, err)
	}
	n, err := stdout.Write(body)
	if err == nil && n != len(body) {
		err = io.ErrShortWrite
	}
	return command.Exit(2, err)
}
