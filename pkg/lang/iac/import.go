package iac

import (
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// Export is one `tofu providers schema -json` file and the provider version the operator asserts it was made at.
// The version is recorded as given and is not checked against the export.
type Export struct {
	Version string
	Path    string
}

// ImportRequest names what to build a catalog from and where to install it. Exactly one input is set: Exports, or
// SchemaZip with Region. Every path is a local file, and nothing is downloaded.
type ImportRequest struct {
	// Dir is the directory the catalog is installed in, created when it is missing. A catalog's file name comes from
	// its manifest, so one provider line, or one CloudFormation region, has one file in Dir.
	Dir string
	// Exports are the exports of ONE provider and one major line, one per release. A multi-provider export is
	// refused. Make one in a scratch directory whose configuration requires only that provider:
	// `tofu init && tofu providers schema -json > export.json`.
	Exports []Export
	// SchemaZip is a CloudFormation registry schema zip the operator fetched, Region the region it is for
	// (us-east-1). A region holds one snapshot, so a new zip replaces the region's catalog.
	SchemaZip string
	Region    string
	// Licence is the SPDX identifier recorded in the manifest, and Tool names the tool that made the exports.
	Licence string
	Tool    string
	// Replace swaps a release the installed catalog holds with different content. Without it that is refused. It
	// does not apply to a schema zip.
	Replace bool
}

// ImportResult is the catalog an import installed.
type ImportResult struct {
	// Path is the installed file, Name its file name, SHA256 its digest and Bytes its size. Source is the provider
	// address (aws/cloudformation for a schema zip), Line the provider's major version or the region, and Versions the
	// releases it holds, oldest first (one sha256-<12 hex> snapshot label for a schema zip).
	Path, Name, SHA256 string
	Bytes              int64
	Source, Line       string
	Versions           []string
	// ReplacementFacts is the manifest's own claim: "absent" for a catalog built from exports, which carries facts
	// 1 to 4 and 7 and no replacement facts, and "contract" for a schema zip, whose replacement facts are the
	// schema's createOnlyProperties and conditionalCreateOnlyProperties.
	ReplacementFacts string
	// Facts says in a sentence what the catalog carries.
	Facts string

	// Exports only. Merged is true when a catalog of the same provider line was installed and its releases were
	// kept. Added are the releases the import brought, Kept the installed releases it did not name, Unchanged the
	// named releases whose content equals the installed one and Replaced the ones Replace swapped. All are in
	// version order.
	Merged                           bool
	Added, Kept, Unchanged, Replaced []string

	// Schema zip only. Types is the resource types read, SchemaDigest the sha256 over the zip's entry names and
	// bodies (its embedded build time is ignored) and Date the day the zip was built. SnapshotReplaced is true when
	// the region already had a catalog, PreviousDigest is that catalog's digest and SnapshotUnchanged is true when
	// it equals the new one.
	Types              int
	SchemaDigest, Date string
	SnapshotReplaced   bool
	PreviousDigest     string
	SnapshotUnchanged  bool
}

const (
	factsFromExports = "attributes, required, sensitive, deprecated and upgrade impact; no replacement facts from an import"
	factsFromSchemas = "required, create-only, read-only, write-only and deprecated properties, each from the schema field that states it"
)

// ImportCatalog builds a provider contract catalog from local files and installs it in req.Dir. It is the builder
// `fathom iac import` uses, so a library consumer needs no fathom binary: a `tofu providers schema -json` export
// becomes a catalog with attributes, required, sensitive, deprecated and upgrade-impact facts and no replacement
// facts, and says so in its manifest; a CloudFormation registry schema zip becomes a region's catalog whose
// replacement facts are the schemas' own. Open the result with OpenCatalog(res.Path).
//
// An import into a directory that already holds the same provider line merges: every release held stays, the named
// ones are added, and the result lists what was added, kept, unchanged and replaced. Nothing is installed when
// anything is refused. A refusal of a bound or a check is a *CatalogError; a bad request or an unreadable file is a
// plain error. ImportCatalog reads and writes only the paths in req.
func ImportCatalog(req ImportRequest) (*ImportResult, error) {
	switch {
	case req.Dir == "":
		return nil, errors.New("iac: import needs a directory to install the catalog in")
	case req.SchemaZip != "" && len(req.Exports) > 0:
		return nil, errors.New("iac: import takes exports or a schema zip, not both")
	case req.SchemaZip == "" && len(req.Exports) == 0:
		return nil, errors.New("iac: import needs exports or a schema zip")
	}
	opts := iacfacts.IaCWriterOptions{Licence: req.Licence, Tool: req.Tool}
	if req.SchemaZip != "" {
		return importSchemaZip(req, opts)
	}
	if req.Region != "" {
		return nil, errors.New("iac: Region applies to a schema zip, not to exports")
	}
	in := make([]iacfacts.IaCImportInput, 0, len(req.Exports))
	for _, e := range req.Exports {
		if strings.TrimSpace(e.Version) == "" {
			return nil, fmt.Errorf("iac: export %s has no version: say the provider version it was made at", e.Path)
		}
		if strings.Contains(e.Path, "://") {
			return nil, fmt.Errorf("iac: import takes a local export, not %q", e.Path)
		}
		in = append(in, iacfacts.IaCImportInput{Version: e.Version, Path: e.Path})
	}
	got, err := iacfacts.ImportIaCExportsReport(req.Dir, in, opts, req.Replace)
	if err != nil {
		return nil, catalogErr(err)
	}
	res := installed(got.Info)
	res.Facts = factsFromExports
	res.Merged = got.Merged
	res.Added, res.Kept, res.Unchanged, res.Replaced = nonNil(got.Added), nonNil(got.Kept), nonNil(got.Unchanged), nonNil(got.Replaced)
	return res, nil
}

func importSchemaZip(req ImportRequest, opts iacfacts.IaCWriterOptions) (*ImportResult, error) {
	if req.Region == "" {
		return nil, errors.New("iac: a schema zip is for one region: say which (for example us-east-1)")
	}
	if req.Replace {
		return nil, errors.New("iac: Replace does not apply to a schema zip: a region holds one snapshot, and a new zip replaces it")
	}
	got, err := iacfacts.ImportCloudFormationSchema(req.Dir, req.SchemaZip, req.Region, opts)
	if err != nil {
		return nil, catalogErr(err)
	}
	res := installed(got.Info)
	res.Facts = factsFromSchemas
	res.Types, res.SchemaDigest, res.Date = got.Stats.Types, got.Stats.Digest, got.Stats.Date
	res.SnapshotReplaced, res.PreviousDigest, res.SnapshotUnchanged = got.Replaced, got.Previous, got.Unchanged
	return res, nil
}

func nonNil(s []string) []string { return append([]string{}, s...) }

func installed(info iacfacts.IaCCatalogInfo) *ImportResult {
	m := info.Manifest
	return &ImportResult{
		Path: info.Path, Name: info.Name, SHA256: info.SHA256, Bytes: info.Bytes,
		Source: m.Source, Line: m.Line, Versions: append([]string{}, m.Versions...), ReplacementFacts: m.Replacement,
		Added: []string{}, Kept: []string{}, Unchanged: []string{}, Replaced: []string{},
	}
}
