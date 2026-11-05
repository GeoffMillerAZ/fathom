package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// exporter yields the `tofu providers schema -json` export of one pinned release, and names the tool that made it.
type exporter interface {
	export(ctx context.Context, line pinLine, rel pinRelease) (io.ReadCloser, string, error)
}

// receipt is what a build records beside its catalog. It holds no timestamp, host or path, so two builds of the
// same pins and exports receipt byte-identically.
type receipt struct {
	Catalog  string `json:"catalog"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
	Entries  int    `json:"entries"`
	Blobs    int    `json:"blobs"`
	Source   string `json:"source"`
	Line     string `json:"line"`
	Tool     string `json:"tool"`
	Platform string `json:"platform"`
	// Releases records, per release, the archive pin and the export it was built from.
	Releases []receiptRelease `json:"releases"`
	// Replacement says replacement facts are absent: the Go source scan that adds them is IF-13.
	Replacement     string `json:"replacement"`
	ReplacementNote string `json:"replacement_note"`
	Licence         string `json:"licence,omitempty"`
}

type receiptRelease struct {
	Version       string `json:"version"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ExportSHA256  string `json:"export_sha256"`
	Resources     int    `json:"resources"`
	DataSources   int    `json:"data_sources"`
	Ephemeral     int    `json:"ephemeral_resources"`
}

const replacementNote = "no replacement facts: the provider-source scan is a later row (IF-13), so this catalog answers facts 1 to 4 and 7 only"

// buildCatalog builds the catalog for one provider major line from its pinned releases and writes it, with its
// receipt, to outDir. The build is deterministic: the same pins and exports give byte-identical files.
func buildCatalog(ctx context.Context, line pinLine, ex exporter, outDir, workDir string) (*receipt, error) {
	for _, d := range []string{outDir, workDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	var tool string
	rec := &receipt{Replacement: iacfacts.ReplacementAbsent, ReplacementNote: replacementNote, Licence: line.Licence, Platform: line.Platform}
	// The tool is known only after the first export; the writer wants it up front, so exports are read first.
	type fetched struct {
		rel  pinRelease
		path string
	}
	var held []fetched
	spool, err := os.MkdirTemp(workDir, "export-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(spool)
	for _, rel := range line.Releases {
		rc, t, err := ex.export(ctx, line, rel)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", line.Source, rel.Version, err)
		}
		p := filepath.Join(spool, rel.Version+".json")
		f, err := os.Create(p)
		if err == nil {
			_, err = io.Copy(f, rc)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
		rc.Close()
		if err != nil {
			return nil, err
		}
		if tool != "" && t != tool {
			return nil, fmt.Errorf("%s %s was exported with %q, but earlier releases used %q; one catalog uses one tool", line.Source, rel.Version, t, tool)
		}
		tool = t
		held = append(held, fetched{rel, p})
	}
	w, err := iacfacts.NewIaCCatalogWriter(iacfacts.IaCWriterOptions{Licence: line.Licence, Tool: tool, WorkDir: filepath.Join(workDir, "blobs")})
	if err != nil {
		return nil, err
	}
	defer w.Close()
	for _, h := range held {
		f, err := os.Open(h.path)
		if err != nil {
			return nil, err
		}
		st, err := w.AddExport(h.rel.Version, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", line.Source, h.rel.Version, err)
		}
		if !strings.HasSuffix(st.Source, "/"+line.Source) {
			return nil, fmt.Errorf("%s %s: the export is for %s, not the pinned provider", line.Source, h.rel.Version, st.Source)
		}
		rec.Releases = append(rec.Releases, receiptRelease{
			Version: h.rel.Version, ArchiveSHA256: h.rel.SHA256, ExportSHA256: st.InputSHA256,
			Resources: st.Resources, DataSources: st.Data, Ephemeral: st.Ephemeral,
		})
	}
	m, err := w.Manifest()
	if err != nil {
		return nil, err
	}
	if m.Line != fmt.Sprint(line.Line) {
		return nil, fmt.Errorf("%s: the pins say line %d but the releases are line %s", line.Source, line.Line, m.Line)
	}
	path := filepath.Join(outDir, iacfacts.CatalogFileName(m))
	res, err := w.WriteFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := iacfacts.VerifyIaCCatalog(path); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("the built catalog fails verification: %w", err)
	}
	rec.Catalog, rec.SHA256, rec.Bytes, rec.Entries, rec.Blobs = filepath.Base(path), res.SHA256, res.Bytes, res.Entries, res.Blobs
	rec.Source, rec.Line, rec.Tool = m.Source, m.Line, m.Tool
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path+".receipt.json", append(b, '\n'), 0o644); err != nil {
		return nil, err
	}
	return rec, nil
}
