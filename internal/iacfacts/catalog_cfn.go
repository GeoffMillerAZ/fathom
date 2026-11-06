package iacfacts

// catalog_cfn.go builds a CloudFormation catalog (IF-C1, ADR-091 amendment) from a registry schema zip the operator
// fetched. The catalog is the IF-1 format: kind cloudformation-registry, source aws/cloudformation, the region as the
// line, and one snapshot labelled by a digest over the zip's entry names and bodies. The zip embeds its build time, so
// the digest ignores it and the manifest keeps the newest entry day as its date.
//
// Nothing here dials, and nothing derived from registry schemas is committed or published (IF-P7): an operator builds
// the catalog locally, and the tests use synthetic schemas.

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// maxCFNEntryBytes bounds one schema file in the zip. The largest registry type is 2.43 MiB (IF-M0).
	maxCFNEntryBytes int64 = 16 << 20
	// maxCFNTotalBytes bounds the zip's files together, uncompressed.
	maxCFNTotalBytes int64 = 1 << 30
)

// cfnRegionRE is a region name, safe as a file name part: us-east-1, us-gov-west-1, cn-north-1.
var cfnRegionRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)

var cfnLabelRE = regexp.MustCompile(`^sha256-[0-9a-f]{12}$`)

// cfnLists are the registry schema fields that state facts, in the order a blob carries them.
var cfnLists = []string{"createOnlyProperties", "conditionalCreateOnlyProperties", "readOnlyProperties", "writeOnlyProperties", "deprecatedProperties"}

// CFNImportStats is what reading a schema zip found.
type CFNImportStats struct {
	// Types is the resource types read. Skipped counts JSON files without a typeName. Other counts files that are not
	// JSON.
	Types, Skipped, Other int
	// Digest is the sha256 over the entry names and bodies, in name order, ignoring every timestamp.
	Digest string
	// Date is the zip's build day, empty when it carries none.
	Date string
}

// checkCFNVersion holds a CloudFormation manifest's one snapshot to its label and its digest.
func checkCFNVersion(m IaCCatalogManifest) error {
	if len(m.Versions) != 1 || !cfnLabelRE.MatchString(m.Versions[0]) {
		return corruptf("a CloudFormation catalog names its snapshot sha256-<12 hex>, not %v", m.Versions)
	}
	d := m.Inputs[m.Versions[0]]
	if len(d) != 64 || "sha256-"+d[:12] != m.Versions[0] || !blobHashRE.MatchString(d) {
		return corruptf("the snapshot %s does not match its recorded digest", m.Versions[0])
	}
	return nil
}

func (w *IaCCatalogWriter) cfnManifest() IaCCatalogManifest {
	var label string
	for l := range w.versions {
		label = l
	}
	m := IaCCatalogManifest{
		Format: CatalogFormat, Kind: w.kind, Source: w.source, Line: w.line, Tool: w.tool,
		Versions: []string{label}, Inputs: map[string]string{label: w.inputs[label]}, Licence: w.licence,
		Replacement: ReplacementContract, Date: w.date,
	}
	return m
}

// cfnDigest hashes names and bodies with length prefixes, so no two different entry lists share a stream.
func cfnDigestEntry(h io.Writer, name string, body []byte) {
	var n [binary.MaxVarintLen64]byte
	h.Write(n[:binary.PutUvarint(n[:], uint64(len(name)))])
	io.WriteString(h, name)
	h.Write(n[:binary.PutUvarint(n[:], uint64(len(body)))])
	h.Write(body)
}

// addCloudFormation reads a registry schema zip as the writer's one snapshot. Any refusal leaves no blob behind.
func (w *IaCCatalogWriter) addCloudFormation(region string, zr *zip.Reader) (st CFNImportStats, err error) {
	w.fresh = nil
	defer func() {
		if err != nil {
			w.dropFresh()
		}
		w.fresh = nil
	}()
	if len(w.versions) != 0 {
		return st, errors.New("iac catalog: a CloudFormation catalog holds one schema zip")
	}
	if !cfnRegionRE.MatchString(region) {
		return st, fmt.Errorf("iac catalog: %q is not a region name (for example us-east-1)", region)
	}
	if len(zr.File) > w.lim.Entries {
		return st, refuse(RefuseEntries, "the schema zip holds %d entries, over the %d entry bound", len(zr.File), w.lim.Entries)
	}
	files := make([]*zip.File, 0, len(zr.File))
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for i := 1; i < len(files); i++ {
		if files[i].Name == files[i-1].Name {
			return st, fmt.Errorf("iac catalog: the schema zip holds the entry %q twice", files[i].Name)
		}
	}
	ix := &iacVersionIndex{Resources: map[string]string{}, Data: map[string]string{}, Ephemeral: map[string]string{}}
	h := sha256.New()
	var total int64
	var newest time.Time
	for _, f := range files {
		if f.UncompressedSize64 > uint64(maxCFNEntryBytes) {
			return st, refuse(RefuseBlobBytes, "the schema file %s declares %d bytes, over the %d byte bound", f.Name, f.UncompressedSize64, maxCFNEntryBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return st, fmt.Errorf("iac catalog: schema file %s: %w", f.Name, err)
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxCFNEntryBytes+1))
		rc.Close()
		if err != nil {
			return st, fmt.Errorf("iac catalog: schema file %s: %w", f.Name, err)
		}
		if int64(len(body)) > maxCFNEntryBytes {
			return st, refuse(RefuseBlobBytes, "the schema file %s is over the %d byte bound", f.Name, maxCFNEntryBytes)
		}
		if total += int64(len(body)); total > maxCFNTotalBytes {
			return st, refuse(RefuseCatalogBytes, "the schema files together are over the %d byte bound", maxCFNTotalBytes)
		}
		cfnDigestEntry(h, f.Name, body)
		if f.Modified.After(newest) {
			newest = f.Modified
		}
		if !strings.HasSuffix(strings.ToLower(f.Name), ".json") {
			st.Other++
			continue
		}
		typ, pruned, err := pruneCFNSchema(body)
		if err != nil {
			return st, fmt.Errorf("iac catalog: schema file %s: %w", f.Name, err)
		}
		if typ == "" {
			st.Skipped++
			continue
		}
		if _, dup := ix.Resources[typ]; dup {
			return st, fmt.Errorf("iac catalog: the schema zip names the type %s twice", typ)
		}
		if ix.Resources[typ], err = w.putPruned(typ, pruned); err != nil {
			return st, err
		}
	}
	if len(ix.Resources) == 0 {
		return st, errors.New("iac catalog: the schema zip holds no resource type (a JSON file with a typeName)")
	}
	st.Types, st.Digest = len(ix.Resources), hex.EncodeToString(h.Sum(nil))
	if newest.Year() >= 1990 {
		st.Date = newest.UTC().Format("2006-01-02")
	}
	label := "sha256-" + st.Digest[:12]
	w.kind, w.source, w.line, w.date, w.replacement = KindCloudFormation, CFNSource, region, st.Date, ReplacementContract
	w.versions[label] = ix
	w.inputs[label] = st.Digest
	return st, nil
}

// pruneCFNSchema keeps what a check reads from one registry schema: the type name, the top-level property names with
// their JSON types, the required list, whether other properties are allowed, and the five fact lists. A nested create-only
// property whose parent object the schema resolves also carries whether that object requires it ("nested").
// Descriptions, definitions and every other field are dropped. A file without a string typeName returns an empty type name.
func pruneCFNSchema(raw []byte) (string, []byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var s map[string]any
	if err := dec.Decode(&s); err != nil {
		return "", nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "", nil, errors.New("content follows the schema")
	}
	typ, _ := s["typeName"].(string)
	if typ == "" {
		return "", nil, nil
	}
	out := map[string]any{"typeName": typ, "required": cfnStrings(s["required"])}
	props := map[string]any{}
	if pm, ok := s["properties"].(map[string]any); ok {
		for name, p := range pm {
			entry := map[string]any{}
			if pp, ok := p.(map[string]any); ok {
				switch t := pp["type"].(type) {
				case string:
					entry["type"] = t
				case []any:
					entry["type"] = cfnStrings(t)
				}
			}
			props[name] = entry
		}
	}
	out["properties"] = props
	if b, ok := s["additionalProperties"].(bool); ok {
		out["additionalProperties"] = b
	}
	for _, l := range cfnLists {
		out[l] = cfnStrings(s[l])
	}
	if nested := cfnNestedRequired(s); len(nested) > 0 {
		out["nested"] = nested
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return "", nil, err
	}
	return typ, bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// cfnStrings is the sorted, unique strings of a JSON array. Anything else reads as an empty list.
func cfnStrings(v any) []string {
	arr, _ := v.([]any)
	seen := map[string]bool{}
	out := []string{}
	for _, e := range arr {
		if s, ok := e.(string); ok && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// buildCloudFormationCatalog reads a registry schema zip into a catalog and writes it at dst (IF-C1). The caller owns
// dst's directory. A refusal leaves no file at dst.
func buildCloudFormationCatalog(zr *zip.Reader, region, dst string, opts IaCWriterOptions) (IaCBuildResult, CFNImportStats, error) {
	var res IaCBuildResult
	w, err := NewIaCCatalogWriter(opts)
	if err != nil {
		return res, CFNImportStats{}, err
	}
	defer w.Close()
	st, err := w.addCloudFormation(region, zr)
	if err != nil {
		return res, st, err
	}
	res, err = w.WriteFile(dst)
	return res, st, err
}

// CFNImportResult is what ImportCloudFormationSchema installed.
type CFNImportResult struct {
	Info  IaCCatalogInfo
	Stats CFNImportStats
	// Replaced is true when the region already had a catalog, which the import replaced: a region holds one snapshot.
	// Previous is that catalog's digest and Unchanged is true when it equals the new one.
	Replaced  bool
	Previous  string
	Unchanged bool
}

// ImportCloudFormationSchema builds a catalog from a CloudFormation registry schema zip the operator fetched and
// installs it in dir as that region's catalog. It is how `fathom iac import --region` works. The zip is a local file:
// a URL is refused, and nothing is downloaded. Nothing is installed when anything is refused.
func ImportCloudFormationSchema(dir, zipPath, region string, opts IaCWriterOptions) (CFNImportResult, error) {
	var res CFNImportResult
	if strings.Contains(zipPath, "://") {
		return res, fmt.Errorf("iac import takes a local schema zip, not %q: fetch it with your own tools first", zipPath)
	}
	if !cfnRegionRE.MatchString(region) {
		return res, fmt.Errorf("iac import: %q is not a region name (for example us-east-1)", region)
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		return res, err
	}
	if !st.Mode().IsRegular() {
		return res, fmt.Errorf("iac import: %s is not a regular file", zipPath)
	}
	if st.Size() > MaxCatalogBytes {
		return res, refuse(RefuseCatalogBytes, "%s is %d bytes, over the %d byte bound", zipPath, st.Size(), MaxCatalogBytes)
	}
	f, err := os.Open(zipPath)
	if err != nil {
		return res, err
	}
	defer f.Close()
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return res, fmt.Errorf("iac import: %s is not a zip: %w", zipPath, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}
	tmp, err := os.CreateTemp(dir, ".import-*")
	if err != nil {
		return res, err
	}
	tmp.Close()
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, res.Stats, err = buildCloudFormationCatalog(zr, region, tmp.Name(), opts)
	if err != nil {
		return res, fmt.Errorf("%s: %w", filepath.Base(zipPath), err)
	}
	info, prev, err := installVerified(dir, tmp.Name())
	if err != nil {
		return res, err
	}
	res.Info = info
	if prev != nil {
		res.Replaced = true
		if len(prev.Versions) == 1 {
			res.Previous = prev.Inputs[prev.Versions[0]]
		}
		res.Unchanged = res.Previous == res.Stats.Digest
	}
	return res, nil
}

// maxCFNRefDepth bounds the $ref hops of one nested lookup, so a schema whose definition refers to itself reads
// unresolved and never loops.
const maxCFNRefDepth = 16

// cfnNestedRequired states, for each nested create-only pointer whose parent object the schema resolves, whether the
// nested property is in that object's required list. The pruned schema keeps no definitions, so the importer reads
// them here, once, from the raw schema. A pointer it cannot resolve is left out, and a catalog entry for it states
// neither required nor optional.
//
// A pointer is walked from the root: a name steps into the property of the object so far, "*" into the items of an
// array, and a "#/definitions/..." $ref is followed at every step. The leaf is resolved when the object holding it
// declares properties and declares the leaf among them. A segment, the leaf included, that its object does not have
// leaves the pointer unresolved.
func cfnNestedRequired(s map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, l := range []string{"createOnlyProperties", "conditionalCreateOnlyProperties"} {
		for _, ptr := range cfnStrings(s[l]) {
			path, ok := CFNPointerPath(ptr)
			if !ok || !strings.ContainsAny(path, ".[") {
				continue
			}
			if req, ok := cfnNestedLeaf(s, ptr); ok {
				out[path] = req
			}
		}
	}
	return out
}

func cfnNestedLeaf(root map[string]any, ptr string) (required, resolved bool) {
	rest, _ := strings.CutPrefix(ptr, "/properties/")
	unescape := strings.NewReplacer("~1", "/", "~0", "~")
	segs := strings.Split(rest, "/")
	node := root
	for i, seg := range segs {
		seg = unescape.Replace(seg)
		obj := cfnResolve(root, node)
		if obj == nil {
			return false, false
		}
		last := i == len(segs)-1
		if seg == "*" {
			if last {
				return false, false
			}
			items, _ := obj["items"].(map[string]any)
			if items == nil {
				return false, false
			}
			node = items
			continue
		}
		props, ok := obj["properties"].(map[string]any)
		if !ok {
			return false, false
		}
		if last {
			if _, has := props[seg]; !has {
				return false, false
			}
			for _, r := range cfnStrings(obj["required"]) {
				if r == seg {
					return true, true
				}
			}
			return false, true
		}
		child, _ := props[seg].(map[string]any)
		if child == nil {
			return false, false
		}
		node = child
	}
	return false, false
}

// cfnResolve follows "#/definitions/Name" references from node and returns the schema object they end at, or nil
// when a reference is of another shape, dangles, or runs past maxCFNRefDepth hops.
func cfnResolve(root, node map[string]any) map[string]any {
	for hops := 0; node != nil; hops++ {
		ref, ok := node["$ref"].(string)
		if !ok {
			return node
		}
		name, ok := strings.CutPrefix(ref, "#/definitions/")
		if !ok || hops >= maxCFNRefDepth {
			return nil
		}
		defs, _ := root["definitions"].(map[string]any)
		node, _ = defs[name].(map[string]any)
	}
	return nil
}
