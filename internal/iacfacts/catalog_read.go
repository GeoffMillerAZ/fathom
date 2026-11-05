package iacfacts

import (
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
	"regexp"
)

var blobHashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReadIaCCatalog reads one fact from the catalog at path: the manifest, one version index and one blob, and
// closes. It keeps nothing between calls (no residency, no memo). The directory is streamed, the index and blob
// are walked by a streaming scanner that decodes only the requested path, and every byte of the blob passes a
// hash that must equal its address, so a fact is either exactly what the builder wrote or refused.
//
// Bounds (design section 9): a catalog file over 256 MiB, over 500,000 entries or with a manifest over 4 MiB is
// refused whole at cap + 1. A version index over 4 MiB, a blob over 4 MiB, nesting over 64 or a fact over I48's
// 8 MiB refuses that one answer, with a reason in the returned *IaCRefusal.
func ReadIaCCatalog(path string, req IaCCatalogRequest) (IaCCatalogFact, error) {
	return readIaCCatalog(path, req, DefaultIaCLimits())
}

func readIaCCatalog(path string, req IaCCatalogRequest, lim IaCLimits) (IaCCatalogFact, error) {
	var zero IaCCatalogFact
	key, ok := indexKey(req.Kind)
	if !ok {
		return zero, fmt.Errorf("iac catalog: unknown fact kind %q", req.Kind)
	}
	if req.Version == "" || (req.Kind != SectionProvider && req.Type == "") {
		return zero, errors.New("iac catalog: a fact request names a version and, except for the provider, a type")
	}
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return zero, err
	}
	z, err := openCatZip(f, st.Size(), lim)
	if err != nil {
		return zero, err
	}
	m, err := readCatHead(z, lim, req.Version)
	if err != nil {
		return zero, err
	}
	idxName := "versions/" + req.Version + ".json"
	idx, found, err := z.find(idxName)
	if err != nil {
		return zero, err
	}
	if found && !m.Versions.found {
		found = false // an index the manifest does not list is not a release of this catalog
	}
	if !found {
		return zero, refuse(RefuseNotFound, "the catalog %s holds no version %s (it holds the %s line: add a catalog that does)", filepath.Base(path), req.Version, m.Line)
	}
	r, err := z.open(idxName, idx, lim.IndexBytes, RefuseIndexBytes)
	if err != nil {
		return zero, err
	}
	sc := newCatScan(r, lim.Depth, 4096)
	ipath := []string{key}
	if req.Kind != SectionProvider {
		ipath = append(ipath, req.Type)
	}
	hit, err := sc.find(ipath, 0)
	if err != nil {
		return zero, scanRefusal(err, "version index "+idxName, lim)
	}
	// Read the rest before answering either way, so a truncated or tampered entry is corrupt and never "not found".
	if err := sc.drain(); err != nil {
		return zero, scanRefusal(err, "version index "+idxName, lim)
	}
	if !hit {
		return zero, refuse(RefuseNotFound, "version %s has no %s %q", req.Version, req.Kind, req.Type)
	}
	var digest string
	if err := json.Unmarshal(sc.capBuf, &digest); err != nil || !blobHashRE.MatchString(digest) {
		return zero, corruptf("version %s names a malformed blob address for %s %q", req.Version, req.Kind, req.Type)
	}

	blobName := "blobs/" + digest
	be, found, err := z.find(blobName)
	if err != nil {
		return zero, err
	}
	if !found {
		return zero, corruptf("version %s names blob %s, which the catalog lacks", req.Version, digest)
	}
	br, err := z.open(blobName, be, lim.BlobBytes, RefuseBlobBytes)
	if err != nil {
		return zero, err
	}
	h := sha256.New()
	bs := newCatScan(io.TeeReader(br, h), lim.Depth, int(lim.FactBytes))
	bs.capHint = int(be.usize)
	hit, err = bs.find(req.Path, 0)
	if err != nil {
		return zero, scanRefusal(err, "blob "+digest, lim)
	}
	// Hash every byte before answering either way: a tampered blob with a missing path is corrupt, not "not found".
	if err := bs.drain(); err != nil {
		return zero, scanRefusal(err, "blob "+digest, lim)
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return zero, corruptf("blob %s does not hash to its address", digest)
	}
	if !hit {
		return zero, refuse(RefuseNotFound, "blob %s has no path %q", digest, joinDots(req.Path))
	}
	val := bs.capBuf
	if !json.Valid(val) {
		return zero, corruptf("blob %s holds malformed JSON at %q", digest, joinDots(req.Path))
	}
	return IaCCatalogFact{
		Catalog: filepath.Base(path), Source: m.Source, Version: req.Version, Kind: req.Kind, Type: req.Type,
		Digest: digest, Path: joinDots(req.Path), Value: val,
	}, nil
}

// scanRefusal turns a scanner or entry fault into the refusal a caller prints.
func scanRefusal(err error, what string, lim IaCLimits) error {
	var ref *IaCRefusal
	var fe flate.CorruptInputError
	switch {
	case errors.As(err, &ref):
		return err
	case errors.As(err, &fe):
		return corruptf("%s does not inflate: %v", what, err)
	case errors.Is(err, errScanDepth):
		return refuse(RefuseDepth, "%s nests deeper than %d", what, lim.Depth)
	case errors.Is(err, errScanCapture):
		return refuse(RefuseFactBytes, "%s: the requested fact is over %d bytes, I48's point ceiling", what, lim.FactBytes)
	case errors.Is(err, errScanSyntax), errors.Is(err, io.ErrUnexpectedEOF):
		return corruptf("%s is malformed JSON", what)
	}
	return err
}

// readManifestBytes reads manifest.json whole and checks its JSON depth.
func readManifestBytes(z *catZip, lim IaCLimits) ([]byte, error) {
	e, ok, err := z.find("manifest.json")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, corruptf("the catalog has no manifest.json")
	}
	r, err := z.open("manifest.json", e, lim.ManifestBytes, RefuseManifestBytes)
	if err != nil {
		return nil, err
	}
	buf, err := readEntryBytes(r, e)
	if err != nil {
		return nil, err
	}
	if err := newCatScan(bytes.NewReader(buf), lim.Depth, 16).skipValue(0); err != nil {
		return nil, scanRefusal(err, "manifest.json", lim)
	}
	return buf, nil
}

func checkManifestHead(format int, kind, source, line string, versions int) error {
	if format != CatalogFormat {
		return refuse(RefuseFormat, "the catalog is format %d, this build reads format %d", format, CatalogFormat)
	}
	if kind != KindTerraformProvider {
		return refuse(RefuseFormat, "the catalog kind %q is not one this build reads", kind)
	}
	if source == "" || line == "" || versions == 0 {
		return corruptf("the manifest names no source, line or versions")
	}
	return nil
}

// readCatManifest reads and checks manifest.json in full.
func readCatManifest(z *catZip, lim IaCLimits) (IaCCatalogManifest, error) {
	var m IaCCatalogManifest
	buf, err := readManifestBytes(z, lim)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return m, corruptf("manifest.json is unreadable: %v", err)
	}
	return m, checkManifestHead(m.Format, m.Kind, m.Source, m.Line, len(m.Versions))
}

// catHead is the part of the manifest a point lookup needs. It decodes the version list without keeping it and
// skips the inputs map, so one lookup against a manifest at its bound allocates little more than the manifest.
type catHead struct {
	Format   int          `json:"format"`
	Kind     string       `json:"kind"`
	Source   string       `json:"source"`
	Line     string       `json:"line"`
	Versions versionProbe `json:"versions"`
}

// versionProbe counts the manifest's versions and notes whether want is one of them.
type versionProbe struct {
	want  string
	count int
	found bool
}

func (p *versionProbe) UnmarshalJSON(b []byte) error {
	i := 0
	skipWS := func() {
		for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
			i++
		}
	}
	skipWS()
	if i >= len(b) || b[i] != '[' {
		return errors.New("versions is not an array")
	}
	i++
	for {
		skipWS()
		if i < len(b) && b[i] == ']' {
			return nil
		}
		if i >= len(b) || b[i] != '"' {
			return errors.New("a version is not a string")
		}
		start := i
		i++
		escaped := false
		for i < len(b) && b[i] != '"' {
			if b[i] == '\\' {
				escaped = true
				i++
			}
			i++
		}
		if i >= len(b) {
			return errors.New("an unterminated version")
		}
		lit := b[start+1 : i]
		i++
		p.count++
		if escaped {
			var s string
			if err := json.Unmarshal(b[start:i], &s); err != nil {
				return err
			}
			p.found = p.found || s == p.want
		} else {
			p.found = p.found || string(lit) == p.want
		}
		skipWS()
		if i < len(b) && b[i] == ',' {
			i++
		}
	}
}

// readCatHead reads the manifest for a point lookup of one version.
func readCatHead(z *catZip, lim IaCLimits, version string) (catHead, error) {
	var h catHead
	buf, err := readManifestBytes(z, lim)
	if err != nil {
		return h, err
	}
	h.Versions.want = version
	if err := json.Unmarshal(buf, &h); err != nil {
		return h, corruptf("manifest.json is unreadable: %v", err)
	}
	return h, checkManifestHead(h.Format, h.Kind, h.Source, h.Line, h.Versions.count)
}

// ReadIaCCatalogManifest returns a catalog's manifest without reading an index or a blob.
func ReadIaCCatalogManifest(path string) (IaCCatalogManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return IaCCatalogManifest{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return IaCCatalogManifest{}, err
	}
	lim := DefaultIaCLimits()
	z, err := openCatZip(f, st.Size(), lim)
	if err != nil {
		return IaCCatalogManifest{}, err
	}
	return readCatManifest(z, lim)
}

// VerifyIaCCatalog checks a whole catalog against its bounds, the check `fathom iac add` runs before it installs
// one. Any bound exceeded refuses the whole catalog. It reads the manifest, every version index and every blob,
// proves each blob hashes to its address, that every address an index names exists, and that every version in the
// manifest has an index. It returns the manifest.
func VerifyIaCCatalog(path string) (IaCCatalogManifest, error) {
	m, err := verifyIaCCatalog(path, DefaultIaCLimits())
	if err != nil {
		return m, err
	}
	// One live lookup per version proves the read path end to end on the installed bytes.
	for _, v := range m.Versions {
		if _, err := ReadIaCCatalog(path, IaCCatalogRequest{Version: v, Kind: SectionProvider}); err != nil {
			var ref *IaCRefusal
			if errors.As(err, &ref) && ref.Code == RefuseNotFound {
				continue // a catalog without a provider block is still a catalog
			}
			return m, wholeCatalog(err)
		}
	}
	return m, nil
}

func verifyIaCCatalog(path string, lim IaCLimits) (IaCCatalogManifest, error) {
	var m IaCCatalogManifest
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return m, err
	}
	z, err := openCatZip(f, st.Size(), lim)
	if err != nil {
		return m, err
	}
	if m, err = readCatManifest(z, lim); err != nil {
		return m, wholeCatalog(err)
	}
	if err := checkManifestVersions(m); err != nil {
		return m, err
	}
	listed := make(map[string]struct{}, len(m.Versions))
	want := map[string]struct{}{}
	for _, v := range m.Versions {
		listed["versions/"+v+".json"] = struct{}{}
		name := "versions/" + v + ".json"
		e, ok, err := z.find(name)
		if err != nil {
			return m, err
		}
		if !ok {
			return m, corruptf("the manifest lists version %s but the catalog has no %s", v, name)
		}
		r, err := z.open(name, e, lim.IndexBytes, RefuseIndexBytes)
		if err != nil {
			return m, wholeCatalog(err)
		}
		buf, err := readEntryBytes(r, e)
		if err != nil {
			return m, wholeCatalog(err)
		}
		// Depth is enforced over the whole index, not only the path a lookup walks (design section 9).
		if err := newCatScan(bytes.NewReader(buf), lim.Depth, 16).skipValue(0); err != nil {
			return m, wholeCatalog(scanRefusal(err, name, lim))
		}
		ix, err := parseVersionIndex(buf)
		if err != nil {
			return m, corruptf("%s: %v", name, err)
		}
		if ix.provider != "" {
			want[ix.provider] = struct{}{}
		}
		for _, sec := range []map[string]string{ix.resources, ix.data, ix.ephemeral} {
			for t, h := range sec {
				if !blobHashRE.MatchString(h) {
					return m, corruptf("version %s names a malformed blob address for %q", v, t)
				}
				want[h] = struct{}{}
			}
		}
	}
	err = z.each(func(name []byte, e catZipEntry) (bool, error) {
		switch {
		case string(name) == "manifest.json":
			return false, nil
		case len(name) > len("versions/") && string(name[:9]) == "versions/":
			if _, ok := listed[string(name)]; !ok {
				return true, corruptf("the catalog holds %q, a version index the manifest does not list", name)
			}
			return false, nil
		case len(name) != len("blobs/")+64 || string(name[:6]) != "blobs/":
			return true, corruptf("the catalog holds an unknown entry %q", name)
		}
		addr := string(name[6:])
		if !blobHashRE.MatchString(addr) {
			return true, corruptf("blob entry %q has a malformed address", addr)
		}
		if _, ok := want[addr]; !ok {
			return true, corruptf("blob %s is not named by any version index, or appears twice", addr)
		}
		r, err := z.open("blobs/"+addr, e, lim.BlobBytes, RefuseBlobBytes)
		if err != nil {
			return true, wholeCatalog(err)
		}
		h := sha256.New()
		sc := newCatScan(io.TeeReader(r, h), lim.Depth, 16)
		if err := sc.skipValue(0); err != nil {
			return true, wholeCatalog(scanRefusal(err, "blob "+addr, lim))
		}
		if err := sc.drain(); err != nil {
			return true, wholeCatalog(err)
		}
		if hex.EncodeToString(h.Sum(nil)) != addr {
			return true, corruptf("blob %s does not hash to its address", addr)
		}
		delete(want, addr)
		return false, nil
	})
	if err != nil {
		return m, err
	}
	if len(want) > 0 {
		return m, corruptf("%d blob addresses named by version indexes are missing from the catalog", len(want))
	}
	return m, nil
}

// versionIndex is a parsed versions/<v>.json.
type versionIndex struct {
	provider                   string
	resources, data, ephemeral map[string]string
}

// parseVersionIndex reads an index strictly: only the four known keys, each once, and each type name once. A decoder
// into maps would keep the last of a duplicate while a lookup takes the first, so two readers could disagree.
func parseVersionIndex(b []byte) (versionIndex, error) {
	var ix versionIndex
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := expectDelim(dec, '{'); err != nil {
		return ix, err
	}
	seen := map[string]bool{}
	for dec.More() {
		key, err := readKeyToken(dec)
		if err != nil {
			return ix, err
		}
		if seen[key] {
			return ix, fmt.Errorf("the key %q appears twice", key)
		}
		seen[key] = true
		if key == "provider" {
			if err := dec.Decode(&ix.provider); err != nil {
				return ix, fmt.Errorf("provider: %w", err)
			}
			continue
		}
		var into *map[string]string
		switch key {
		case "resources":
			into = &ix.resources
		case "data_sources":
			into = &ix.data
		case "ephemeral_resources":
			into = &ix.ephemeral
		default:
			return ix, fmt.Errorf("unknown key %q", key)
		}
		t, err := dec.Token()
		if err != nil {
			return ix, err
		}
		if t == nil {
			continue
		}
		if d, ok := t.(json.Delim); !ok || d != '{' {
			return ix, fmt.Errorf("%s is not an object", key)
		}
		*into = map[string]string{}
		for dec.More() {
			typ, err := readKeyToken(dec)
			if err != nil {
				return ix, err
			}
			if _, dup := (*into)[typ]; dup {
				return ix, fmt.Errorf("%s names %q twice", key, typ)
			}
			var addr string
			if err := dec.Decode(&addr); err != nil {
				return ix, fmt.Errorf("%s %q: %w", key, typ, err)
			}
			(*into)[typ] = addr
		}
		if err := expectDelim(dec, '}'); err != nil {
			return ix, err
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return ix, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ix, errors.New("content follows the index")
	}
	return ix, nil
}

// wholeCatalog widens an answer refusal into a refusal of the whole catalog, which is what a bound breach means
// at `add`.
func wholeCatalog(err error) error {
	var ref *IaCRefusal
	if errors.As(err, &ref) && ref.Scope == ScopeAnswer {
		c := *ref
		c.Scope = ScopeCatalog
		return &c
	}
	return err
}

// checkManifestVersions requires sorted, unique, valid versions that all sit in the manifest's major line.
func checkManifestVersions(m IaCCatalogManifest) error {
	var prev IaCVersion
	for i, s := range m.Versions {
		v, err := ParseIaCVersion(s)
		if err != nil {
			return corruptf("manifest version %q: %v", s, err)
		}
		if fmt.Sprint(v.Major) != m.Line {
			return corruptf("manifest version %s is outside the %s line the catalog is named for", s, m.Line)
		}
		if i > 0 && CompareIaCVersions(prev, v) >= 0 {
			return corruptf("manifest versions are not strictly ascending at %s", s)
		}
		prev = v
	}
	return nil
}

// readEntryBytes reads an opened entry whole. The entry's declared size was checked against its bound at open, so the
// buffer is exact, and the read runs to the end so the entry's size and CRC are checked.
func readEntryBytes(r io.Reader, e catZipEntry) ([]byte, error) {
	buf := make([]byte, e.usize)
	if _, err := io.ReadFull(r, buf); err != nil {
		var ref *IaCRefusal
		if errors.As(err, &ref) {
			return nil, err
		}
		return nil, corruptf("an entry is shorter than its declared %d bytes: %v", e.usize, err)
	}
	var one [1]byte
	if n, err := r.Read(one[:]); n != 0 || !errors.Is(err, io.EOF) {
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, corruptf("an entry is longer than its declared %d bytes", e.usize)
	}
	return buf, nil
}
