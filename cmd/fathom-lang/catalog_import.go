package main

// catalog_import.go -- fathom-lang catalog import: build a provider contract catalog from local files with the code
// `fathom iac import` uses (pkg/lang/iac, IF-C8), so a tool that has no fathom binary can make a catalog for
// `contract`. Nothing is downloaded.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// catalogImportJSON is the output of catalog import. The export fields appear for exports and the schema zip fields
// for a schema zip.
type catalogImportJSON struct {
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	SHA256           string   `json:"sha256"`
	Bytes            int64    `json:"bytes"`
	Source           string   `json:"source"`
	Line             string   `json:"line"`
	Versions         []string `json:"versions"`
	ReplacementFacts string   `json:"replacement_facts"`
	Facts            string   `json:"facts"`

	Merged    *bool    `json:"merged,omitempty"`
	Added     []string `json:"added,omitempty"`
	Kept      []string `json:"kept,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
	Replaced  []string `json:"replaced,omitempty"`

	Types             *int   `json:"types,omitempty"`
	SchemaDigest      string `json:"schema_digest,omitempty"`
	Date              string `json:"date,omitempty"`
	SnapshotReplaced  *bool  `json:"snapshot_replaced,omitempty"`
	PreviousDigest    string `json:"previous_digest,omitempty"`
	SnapshotUnchanged *bool  `json:"snapshot_unchanged,omitempty"`
}

func cmdCatalog(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "import" {
		return usageError{errors.New("catalog takes the subcommand import")}
	}
	return cmdCatalogImport(args[1:], out)
}

func cmdCatalogImport(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("catalog import", flag.ContinueOnError)
	dir := fs.String("dir", "", "the directory to install the catalog in (required)")
	version := fs.String("version", "", "the provider version of a single export file (or write each input as VERSION=export.json)")
	region := fs.String("region", "", "import a CloudFormation registry schema zip (one input) as the catalog of this region, for example us-east-1")
	licence := fs.String("licence", "", "SPDX licence identifier to record in the manifest")
	tool := fs.String("tool", "", "the tool and version that produced the exports, recorded in the manifest")
	replace := fs.Bool("replace", false, "replace a release the installed catalog holds with different content (default: refuse)")
	// Flags may follow the inputs, as in `fathom iac import`: take each positional and parse the rest again.
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	var inputs []string
	for fs.NArg() > 0 {
		inputs = append(inputs, fs.Arg(0))
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return parseError(err)
		}
	}
	if *dir == "" {
		return usageError{errors.New("catalog import needs -dir DIR")}
	}
	if len(inputs) == 0 {
		return usageError{errors.New("catalog import needs an export file (or, with -region, a schema zip)")}
	}
	req := iac.ImportRequest{Dir: *dir, Licence: *licence, Tool: *tool, Replace: *replace}
	if *region != "" {
		if *version != "" {
			return usageError{errors.New("-version does not apply to a CloudFormation schema zip: the catalog is keyed by region and schema digest")}
		}
		if len(inputs) != 1 {
			return usageError{errors.New("catalog import -region takes one registry schema zip")}
		}
		req.SchemaZip, req.Region = inputs[0], *region
	} else {
		for _, a := range inputs {
			v, p := *version, a
			if i := strings.IndexByte(a, '='); i > 0 {
				v, p = a[:i], a[i+1:]
			}
			if v == "" {
				return usageError{fmt.Errorf("%s has no version: pass -version, or write the input as VERSION=%s", a, a)}
			}
			req.Exports = append(req.Exports, iac.Export{Version: v, Path: p})
		}
	}
	res, err := iac.ImportCatalog(req)
	if err != nil {
		return err
	}
	j := catalogImportJSON{Path: res.Path, Name: res.Name, SHA256: res.SHA256, Bytes: res.Bytes, Source: res.Source, Line: res.Line,
		Versions: res.Versions, ReplacementFacts: res.ReplacementFacts, Facts: res.Facts}
	if req.SchemaZip != "" {
		j.Types, j.SchemaDigest, j.Date = &res.Types, res.SchemaDigest, res.Date
		j.SnapshotReplaced, j.PreviousDigest, j.SnapshotUnchanged = &res.SnapshotReplaced, res.PreviousDigest, &res.SnapshotUnchanged
	} else {
		j.Merged = &res.Merged
		j.Added, j.Kept, j.Unchanged, j.Replaced = res.Added, res.Kept, res.Unchanged, res.Replaced
	}
	return writeJSON(out, struct {
		SchemaVersion int               `json:"schema_version"`
		Catalog       catalogImportJSON `json:"catalog"`
	}{schemaVersion, j})
}
