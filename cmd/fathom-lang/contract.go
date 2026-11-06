package main

// contract.go -- fathom-lang contract: what a provider catalog an operator built states about a type (pkg/lang/iac,
// IF-C5). A catalog is a local file. Each lookup opens the zip, reads one version index and one blob, and closes it.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// catalogInfo is the output of contract with only -catalog.
type catalogInfo struct {
	Catalog          string   `json:"catalog"`
	Source           string   `json:"source"`
	Line             string   `json:"line"`
	Versions         []string `json:"versions"`
	ReplacementFacts string   `json:"replacement_facts"`
}

// cmdContract prints a catalog's manifest, a type's contract at a version, or one attribute of it.
func cmdContract(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("contract", flag.ContinueOnError)
	catalog := fs.String("catalog", "", "the catalog file (required)")
	source := fs.String("source", "", "the provider address (default: the catalog's own)")
	version := fs.String("version", "", "an exact version the catalog holds (default: the highest)")
	kind := fs.String("kind", "", "resource, data_source or ephemeral_resource (default: the first that holds TYPE, in that order)")
	if err := fs.Parse(args); err != nil {
		return parseError(err)
	}
	if *catalog == "" {
		return usageError{errors.New("contract needs -catalog PATH")}
	}
	if fs.NArg() > 2 {
		return usageError{fmt.Errorf("contract takes TYPE and ATTR, got %d arguments", fs.NArg())}
	}
	cat, err := iac.OpenCatalog(*catalog)
	if err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return writeJSON(out, struct {
			SchemaVersion int         `json:"schema_version"`
			Catalog       catalogInfo `json:"catalog"`
		}{schemaVersion, catalogInfo{Catalog: *catalog, Source: cat.Source(), Line: cat.Line(), Versions: cat.Versions(), ReplacementFacts: cat.ReplacementFacts()}})
	}
	src, ver := *source, *version
	if src == "" {
		src = cat.Source()
	}
	if ver == "" {
		vs := cat.Versions()
		ver = vs[len(vs)-1]
	}
	typ := fs.Arg(0)
	var ct *iac.Contract
	if *kind == "" {
		ct, err = cat.Contract(src, ver, typ)
	} else {
		ct, err = cat.KindContract(src, ver, iac.Kind(*kind), typ)
	}
	if err != nil {
		return err
	}
	if fs.NArg() == 2 {
		attr := fs.Arg(1)
		keep := ct.Attributes[:0:0]
		for _, a := range ct.Attributes {
			if a.Path == attr || strings.HasPrefix(a.Path, attr+".") || strings.HasPrefix(a.Path, attr+"[*]") {
				keep = append(keep, a)
			}
		}
		if len(keep) == 0 {
			return fmt.Errorf("%s %s at %s has no attribute %q", ct.Kind, ct.Type, ct.Version, attr)
		}
		ct.Attributes = keep
	}
	return writeJSON(out, struct {
		SchemaVersion int           `json:"schema_version"`
		Contract      *iac.Contract `json:"contract"`
	}{schemaVersion, ct})
}
