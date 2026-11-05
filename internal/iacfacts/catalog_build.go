package iacfacts

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// IaCCatalogWriter builds a catalog from `tofu providers schema -json` exports. It is the one deterministic writer
// that `fathom iac import` and cmd/iaccatalog share. It runs no provider: it reads files.
//
// A schema is pruned to names, types and flags (descriptions are dropped), keyed canonically, and stored once
// under its sha256, so a resource that did not change between releases costs one blob. Identity schemas, provider
// functions and descriptions are not carried. Replacement facts are absent until IF-13, and the manifest says so.
type IaCCatalogWriter struct {
	work     string
	own      bool
	lim      IaCLimits
	kind     string
	licence  string
	tool     string
	source   string
	line     string
	schemaFm string
	versions map[string]*iacVersionIndex
	inputs   map[string]string
	blobs    map[string]int64
}

type iacVersionIndex struct {
	Provider  string            `json:"provider,omitempty"`
	Resources map[string]string `json:"resources"`
	Data      map[string]string `json:"data_sources"`
	Ephemeral map[string]string `json:"ephemeral_resources"`
}

// IaCWriterOptions are the manifest fields a caller supplies.
type IaCWriterOptions struct {
	// Licence is the SPDX identifier recorded at the pin; empty when not audited (IF-11).
	Licence string
	// Tool names the tool that produced the exports, for the manifest.
	Tool string
	// WorkDir holds spilled blobs. Empty makes a temp directory under TMPDIR, removed by Close.
	WorkDir string
	// Limits override the documented bounds; the zero value uses them.
	Limits IaCLimits
}

// NewIaCCatalogWriter starts a catalog.
func NewIaCCatalogWriter(opts IaCWriterOptions) (*IaCCatalogWriter, error) {
	w := &IaCCatalogWriter{
		kind: KindTerraformProvider, licence: opts.Licence, tool: opts.Tool, lim: opts.Limits,
		versions: map[string]*iacVersionIndex{}, inputs: map[string]string{}, blobs: map[string]int64{},
	}
	if w.lim == (IaCLimits{}) {
		w.lim = DefaultIaCLimits()
	}
	dir := opts.WorkDir
	if dir == "" {
		d, err := os.MkdirTemp("", "iaccat-*")
		if err != nil {
			return nil, err
		}
		dir, w.own = d, true
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w.work = dir
	return w, nil
}

// Close removes spilled blobs the writer created a directory for.
func (w *IaCCatalogWriter) Close() error {
	if w.own {
		return os.RemoveAll(w.work)
	}
	for h := range w.blobs {
		_ = os.Remove(filepath.Join(w.work, h))
	}
	return nil
}

// IaCExportStats is what AddExport read.
type IaCExportStats struct {
	Source    string
	Resources int
	Data      int
	Ephemeral int
	// InputSHA256 is the digest of the export bytes.
	InputSHA256 string
}

// AddExport reads one `tofu providers schema -json` export as the given version. The export is streamed, one
// schema in memory at a time. A schema over the blob bound or the depth bound refuses the whole export, naming it.
func (w *IaCCatalogWriter) AddExport(version string, export io.Reader) (IaCExportStats, error) {
	var st IaCExportStats
	v, err := ParseIaCVersion(version)
	if err != nil {
		return st, err
	}
	if _, dup := w.versions[version]; dup {
		return st, fmt.Errorf("iac catalog: version %s was added twice", version)
	}
	if v.String() != version {
		return st, fmt.Errorf("iac catalog: write the version as %s, not %q", v.String(), version)
	}
	line := strconv.Itoa(v.Major)
	if w.line != "" && w.line != line {
		return st, fmt.Errorf("iac catalog: version %s is on line %s, but this catalog holds line %s (one catalog holds one major line)", version, line, w.line)
	}
	h := sha256.New()
	tee := io.TeeReader(export, h)
	dec := json.NewDecoder(tee)
	ix := &iacVersionIndex{Resources: map[string]string{}, Data: map[string]string{}, Ephemeral: map[string]string{}}
	var format string
	if err := expectDelim(dec, '{'); err != nil {
		return st, fmt.Errorf("iac catalog: export: %w", err)
	}
	providers := 0
	for dec.More() {
		key, err := readKeyToken(dec)
		if err != nil {
			return st, fmt.Errorf("iac catalog: export: %w", err)
		}
		switch key {
		case "format_version":
			if err := dec.Decode(&format); err != nil {
				return st, fmt.Errorf("iac catalog: export format_version: %w", err)
			}
		case "provider_schemas":
			if err := expectDelim(dec, '{'); err != nil {
				return st, fmt.Errorf("iac catalog: export provider_schemas: %w", err)
			}
			for dec.More() {
				addr, err := readKeyToken(dec)
				if err != nil {
					return st, fmt.Errorf("iac catalog: export: %w", err)
				}
				providers++
				if providers > 1 {
					return st, errors.New("iac catalog: the export holds more than one provider; export one provider at a time")
				}
				if w.source != "" && w.source != addr {
					return st, fmt.Errorf("iac catalog: version %s is %s, but this catalog holds %s", version, addr, w.source)
				}
				st.Source = addr
				if err := w.addProvider(dec, ix, &st); err != nil {
					return st, err
				}
			}
			if err := expectDelim(dec, '}'); err != nil {
				return st, fmt.Errorf("iac catalog: export: %w", err)
			}
		default:
			if err := skipToken(dec); err != nil {
				return st, fmt.Errorf("iac catalog: export %s: %w", key, err)
			}
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return st, fmt.Errorf("iac catalog: export: %w", err)
	}
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return st, err
	}
	if providers != 1 {
		return st, errors.New("iac catalog: the export holds no provider schema")
	}
	if len(ix.Resources)+len(ix.Data)+len(ix.Ephemeral) == 0 && ix.Provider == "" {
		return st, errors.New("iac catalog: the export holds no schemas")
	}
	st.InputSHA256 = hex.EncodeToString(h.Sum(nil))
	w.source, w.line, w.schemaFm = st.Source, line, firstNonEmpty(w.schemaFm, format)
	w.versions[version] = ix
	w.inputs[version] = st.InputSHA256
	st.Resources, st.Data, st.Ephemeral = len(ix.Resources), len(ix.Data), len(ix.Ephemeral)
	return st, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (w *IaCCatalogWriter) addProvider(dec *json.Decoder, ix *iacVersionIndex, st *IaCExportStats) error {
	if err := expectDelim(dec, '{'); err != nil {
		return fmt.Errorf("iac catalog: export provider: %w", err)
	}
	for dec.More() {
		key, err := readKeyToken(dec)
		if err != nil {
			return fmt.Errorf("iac catalog: export provider: %w", err)
		}
		var into map[string]string
		switch key {
		case "provider":
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return fmt.Errorf("iac catalog: export provider block: %w", err)
			}
			if ix.Provider, err = w.putSchema("provider", raw); err != nil {
				return err
			}
			continue
		case "resource_schemas":
			into = ix.Resources
		case "data_source_schemas":
			into = ix.Data
		case "ephemeral_resource_schemas":
			into = ix.Ephemeral
		default:
			if err := skipToken(dec); err != nil {
				return fmt.Errorf("iac catalog: export %s: %w", key, err)
			}
			continue
		}
		t, err := dec.Token()
		if err != nil {
			return fmt.Errorf("iac catalog: export %s: %w", key, err)
		}
		if t == nil {
			continue // a null map is an empty one
		}
		if d, ok := t.(json.Delim); !ok || d != '{' {
			return fmt.Errorf("iac catalog: export %s: expected an object, got %v", key, t)
		}
		for dec.More() {
			typ, err := readKeyToken(dec)
			if err != nil {
				return fmt.Errorf("iac catalog: export %s: %w", key, err)
			}
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return fmt.Errorf("iac catalog: export %s %s: %w", key, typ, err)
			}
			if into[typ], err = w.putSchema(typ, raw); err != nil {
				return err
			}
		}
		if err := expectDelim(dec, '}'); err != nil {
			return fmt.Errorf("iac catalog: export %s: %w", key, err)
		}
	}
	return expectDelim(dec, '}')
}

// putSchema prunes one schema, checks it against the blob and depth bounds, spills it under its address and
// returns the address.
func (w *IaCCatalogWriter) putSchema(name string, raw json.RawMessage) (string, error) {
	pruned, err := pruneIaCSchema(raw)
	if err != nil {
		return "", fmt.Errorf("iac catalog: schema %s: %w", name, err)
	}
	if int64(len(pruned)) > w.lim.BlobBytes {
		return "", refuse(RefuseBlobBytes, "schema %s is %d bytes pruned, over the %d byte blob bound; the whole export is refused", name, len(pruned), w.lim.BlobBytes)
	}
	sc := newCatScan(bytes.NewReader(pruned), w.lim.Depth, 16)
	if err := sc.skipValue(0); err != nil {
		if errors.Is(err, errScanDepth) {
			return "", refuse(RefuseDepth, "schema %s nests deeper than %d; the whole export is refused", name, w.lim.Depth)
		}
		return "", fmt.Errorf("iac catalog: schema %s: %w", name, err)
	}
	sum := sha256.Sum256(pruned)
	addr := hex.EncodeToString(sum[:])
	if _, ok := w.blobs[addr]; !ok {
		if err := os.WriteFile(filepath.Join(w.work, addr), pruned, 0o644); err != nil {
			return "", err
		}
		w.blobs[addr] = int64(len(pruned))
	}
	return addr, nil
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != want {
		return fmt.Errorf("expected %q, got %v", want, t)
	}
	return nil
}

func readKeyToken(dec *json.Decoder) (string, error) {
	t, err := dec.Token()
	if err != nil {
		return "", err
	}
	s, ok := t.(string)
	if !ok {
		return "", fmt.Errorf("expected an object key, got %v", t)
	}
	return s, nil
}

func skipToken(dec *json.Decoder) error {
	var skip json.RawMessage
	return dec.Decode(&skip)
}

// pruneIaCSchema keeps names, types and flags of a {version, block} schema and drops descriptions. Descriptions
// live on a block or an attribute object, never in the attribute map, so an attribute named "description" stays.
func pruneIaCSchema(raw json.RawMessage) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var s map[string]any
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if blk, ok := s["block"].(map[string]any); ok {
		pruneBlock(blk)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

func pruneBlock(b map[string]any) {
	delete(b, "description")
	delete(b, "description_kind")
	if attrs, ok := b["attributes"].(map[string]any); ok {
		for _, a := range attrs {
			if am, ok := a.(map[string]any); ok {
				pruneAttribute(am)
			}
		}
	}
	if bts, ok := b["block_types"].(map[string]any); ok {
		for _, bt := range bts {
			if bm, ok := bt.(map[string]any); ok {
				if inner, ok := bm["block"].(map[string]any); ok {
					pruneBlock(inner)
				}
			}
		}
	}
}

func pruneAttribute(a map[string]any) {
	delete(a, "description")
	delete(a, "description_kind")
	if nt, ok := a["nested_type"].(map[string]any); ok {
		if attrs, ok := nt["attributes"].(map[string]any); ok {
			for _, x := range attrs {
				if am, ok := x.(map[string]any); ok {
					pruneAttribute(am)
				}
			}
		}
	}
}

// IaCBuildResult describes the file WriteFile produced.
type IaCBuildResult struct {
	Path     string
	SHA256   string
	Bytes    int64
	Entries  int
	Blobs    int
	Manifest IaCCatalogManifest
}

// Manifest is the manifest WriteFile would write.
func (w *IaCCatalogWriter) Manifest() (IaCCatalogManifest, error) {
	if len(w.versions) == 0 {
		return IaCCatalogManifest{}, errors.New("iac catalog: no export was added")
	}
	vs := make([]IaCVersion, 0, len(w.versions))
	for s := range w.versions {
		v, _ := ParseIaCVersion(s)
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return CompareIaCVersions(vs[i], vs[j]) < 0 })
	m := IaCCatalogManifest{
		Format: CatalogFormat, Kind: w.kind, Source: w.source, Line: w.line, Tool: w.tool,
		SchemaFormat: w.schemaFm, Inputs: map[string]string{}, Licence: w.licence, Replacement: ReplacementAbsent,
	}
	for _, v := range vs {
		m.Versions = append(m.Versions, v.Raw)
		m.Inputs[v.Raw] = w.inputs[v.Raw]
	}
	return m, nil
}

// CatalogFileName is the name an installed catalog takes: the address and the major line, so two lines of one
// provider are two files and the file says which.
func CatalogFileName(m IaCCatalogManifest) string {
	return strings.ReplaceAll(m.Source, "/", "_") + ".v" + m.Line + ".iaccat.zip"
}

// WriteFile writes the catalog to path as a deterministic zip: entries in a fixed order, zero timestamps, no
// extra fields, one compressor setting. Two writes of the same exports are byte-identical on one toolchain.
// A result over the file or entry bound removes the file and refuses.
func (w *IaCCatalogWriter) WriteFile(path string) (IaCBuildResult, error) {
	var res IaCBuildResult
	m, err := w.Manifest()
	if err != nil {
		return res, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".iaccat-*")
	if err != nil {
		return res, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	zw := zip.NewWriter(tmp)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	put := func(name string) (io.Writer, error) {
		return zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	}
	entries := 0
	mb, err := json.Marshal(m)
	if err != nil {
		return res, err
	}
	if int64(len(mb)) > w.lim.ManifestBytes {
		return res, refuse(RefuseManifestBytes, "the manifest is %d bytes, over the %d byte bound", len(mb), w.lim.ManifestBytes)
	}
	out, err := put("manifest.json")
	if err != nil {
		return res, err
	}
	if _, err := out.Write(mb); err != nil {
		return res, err
	}
	entries++
	for _, v := range m.Versions {
		ib, err := json.Marshal(w.versions[v])
		if err != nil {
			return res, err
		}
		if int64(len(ib)) > w.lim.IndexBytes {
			return res, refuse(RefuseIndexBytes, "the index for %s is %d bytes, over the %d byte bound", v, len(ib), w.lim.IndexBytes)
		}
		if out, err = put("versions/" + v + ".json"); err != nil {
			return res, err
		}
		if _, err := out.Write(ib); err != nil {
			return res, err
		}
		entries++
	}
	addrs := make([]string, 0, len(w.blobs))
	for a := range w.blobs {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	if entries+len(addrs) > w.lim.Entries {
		return res, refuse(RefuseEntries, "the catalog would hold %d entries, over the %d entry bound", entries+len(addrs), w.lim.Entries)
	}
	for _, a := range addrs {
		f, err := os.Open(filepath.Join(w.work, a))
		if err != nil {
			return res, err
		}
		if out, err = put("blobs/" + a); err != nil {
			f.Close()
			return res, err
		}
		_, err = io.Copy(out, f)
		f.Close()
		if err != nil {
			return res, err
		}
		entries++
	}
	if err := zw.Close(); err != nil {
		return res, err
	}
	if err := tmp.Sync(); err != nil {
		return res, err
	}
	st, err := tmp.Stat()
	if err != nil {
		return res, err
	}
	if st.Size() > w.lim.CatalogBytes {
		return res, refuse(RefuseCatalogBytes, "the catalog is %d bytes, over the %d byte bound", st.Size(), w.lim.CatalogBytes)
	}
	if err := tmp.Close(); err != nil {
		return res, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return res, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return res, err
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return res, err
	}
	return IaCBuildResult{Path: path, SHA256: sum, Bytes: st.Size(), Entries: entries, Blobs: len(addrs), Manifest: m}, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
