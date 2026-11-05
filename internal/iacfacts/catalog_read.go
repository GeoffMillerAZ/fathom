package iacfacts

import (
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
	m, err := readCatManifest(z, lim)
	if err != nil {
		return zero, err
	}
	idxName := "versions/" + req.Version + ".json"
	idx, found, err := z.find(idxName)
	if err != nil {
		return zero, err
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
	if !hit {
		return zero, refuse(RefuseNotFound, "version %s has no %s %q", req.Version, req.Kind, req.Type)
	}
	if err := sc.drain(); err != nil {
		return zero, scanRefusal(err, "version index "+idxName, lim)
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
	if !hit {
		return zero, refuse(RefuseNotFound, "blob %s has no path %q", digest, joinPath(req.Path))
	}
	if err := bs.drain(); err != nil {
		return zero, scanRefusal(err, "blob "+digest, lim)
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return zero, corruptf("blob %s does not hash to its address", digest)
	}
	val := bs.capBuf
	if !json.Valid(val) {
		return zero, corruptf("blob %s holds malformed JSON at %q", digest, joinPath(req.Path))
	}
	return IaCCatalogFact{
		Catalog: filepath.Base(path), Source: m.Source, Version: req.Version, Kind: req.Kind, Type: req.Type,
		Digest: digest, Path: joinPath(req.Path), Value: val,
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

// readCatManifest reads and checks manifest.json.
func readCatManifest(z *catZip, lim IaCLimits) (IaCCatalogManifest, error) {
	var m IaCCatalogManifest
	e, ok, err := z.find("manifest.json")
	if err != nil {
		return m, err
	}
	if !ok {
		return m, corruptf("the catalog has no manifest.json")
	}
	r, err := z.open("manifest.json", e, lim.ManifestBytes, RefuseManifestBytes)
	if err != nil {
		return m, err
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&m); err != nil {
		var ref *IaCRefusal
		if errors.As(err, &ref) {
			return m, err
		}
		return m, corruptf("manifest.json is unreadable: %v", err)
	}
	if _, err := io.Copy(io.Discard, r); err != nil {
		return m, err
	}
	if m.Format != CatalogFormat {
		return m, refuse(RefuseFormat, "the catalog is format %d, this build reads format %d", m.Format, CatalogFormat)
	}
	if m.Kind != KindTerraformProvider {
		return m, refuse(RefuseFormat, "the catalog kind %q is not one this build reads", m.Kind)
	}
	if m.Source == "" || m.Line == "" || len(m.Versions) == 0 {
		return m, corruptf("the manifest names no source, line or versions")
	}
	return m, nil
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
	return verifyIaCCatalog(path, DefaultIaCLimits())
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
		return m, err
	}
	if err := checkManifestVersions(m); err != nil {
		return m, err
	}
	want := map[string]struct{}{}
	for _, v := range m.Versions {
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
		var ix struct {
			Provider  string            `json:"provider"`
			Resources map[string]string `json:"resources"`
			Data      map[string]string `json:"data_sources"`
			Ephemeral map[string]string `json:"ephemeral_resources"`
		}
		if err := json.NewDecoder(r).Decode(&ix); err != nil {
			var ref *IaCRefusal
			if errors.As(err, &ref) {
				return m, wholeCatalog(err)
			}
			return m, corruptf("%s is unreadable: %v", name, err)
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return m, wholeCatalog(err)
		}
		if ix.Provider != "" {
			want[ix.Provider] = struct{}{}
		}
		for _, sec := range []map[string]string{ix.Resources, ix.Data, ix.Ephemeral} {
			for t, h := range sec {
				if !blobHashRE.MatchString(h) {
					return m, corruptf("version %s names a malformed blob address for %q", v, t)
				}
				want[h] = struct{}{}
			}
		}
	}
	seen := 0
	err = z.each(func(name []byte, e catZipEntry) (bool, error) {
		if len(name) != len("blobs/")+64 || string(name[:6]) != "blobs/" {
			return false, nil
		}
		addr := string(name[6:])
		if !blobHashRE.MatchString(addr) {
			return false, corruptf("blob entry %q has a malformed address", addr)
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
		seen++
		return false, nil
	})
	if err != nil {
		return m, err
	}
	if len(want) > 0 {
		return m, corruptf("%d blob addresses named by version indexes are missing from the catalog", len(want))
	}
	// One live lookup per version proves the read path end to end on the installed bytes.
	for _, v := range m.Versions {
		if _, err := readIaCCatalog(path, IaCCatalogRequest{Version: v, Kind: SectionProvider}, lim); err != nil {
			var ref *IaCRefusal
			if errors.As(err, &ref) && ref.Code == RefuseNotFound {
				continue // a catalog without a provider block is still a catalog
			}
			return m, wholeCatalog(err)
		}
	}
	return m, nil
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
