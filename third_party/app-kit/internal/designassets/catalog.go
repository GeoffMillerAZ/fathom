// Package designassets preserves private vendor downloads as immutable ZIPs.
// Metadata and bounded text inspection never extract or execute their contents.
package designassets

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const (
	MaxArchiveBytes    int64 = 256 << 20
	MaxMemberBytes     int64 = 32 << 20
	MaxArchiveExpanded int64 = 512 << 20
	MaxImportBytes     int64 = 1 << 30
	MaxImportExpanded  int64 = 2 << 30
	MaxCatalogBytes    int64 = 64 << 20
	MaxTextBytes       int64 = 256 << 10
	MaxProjectBytes    int64 = 1 << 20
	MaxMembers               = 20000
	MaxArchives              = 256
	MaxInputArchives         = 64
	MaxPathBytes             = 1024
)

//go:embed schema.cue
var schemaSource []byte

// Catalog is a versioned metadata snapshot. Its exported fields and returned
// records are detached from the private state used by List and Get.
type Catalog struct {
	SchemaVersion int       `json:"schema_version"`
	Revision      string    `json:"revision"`
	Archives      []Archive `json:"archives"`
	archives      []Archive
}

// Archive identifies one exact original download, including its complete file
// inventory. SourceURL identifies a product, not proof of an authenticated fetch.
type Archive struct {
	ID        string    `json:"id"`
	Family    string    `json:"family"`
	Filename  string    `json:"filename"`
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size"`
	Kind      string    `json:"kind"`
	Vendor    string    `json:"vendor"`
	SourceURL string    `json:"source_url"`
	Files     []File    `json:"files"`
	Projects  []Project `json:"projects"`
}

// File records a regular ZIP member. Directories remain in the original ZIP.
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Kind   string `json:"kind"`
}

// Project is metadata from a package.json member. Original manifests, scripts,
// lockfiles and supporting assets remain byte-for-byte in the ZIP.
type Project struct {
	Path                 string            `json:"path"`
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"dev_dependencies"`
	PeerDependencies     map[string]string `json:"peer_dependencies"`
	OptionalDependencies map[string]string `json:"optional_dependencies"`
}

// TextFile is verified, bounded UTF-8 content. Callers must render it as text,
// not execute scripts or interpret HTML/SVG as trusted application markup.
type TextFile struct {
	ArchiveID string `json:"archive_id"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
}

// List returns detached metadata. It does not scan archive contents for drift;
// Verify and ReadFile perform full-archive integrity verification on exact use.
func (c *Catalog) List() []Archive {
	if c == nil {
		return []Archive{}
	}
	return cloneArchives(c.archives)
}

// Get returns detached metadata by exact immutable ID, without reading the ZIP.
func (c *Catalog) Get(id string) (Archive, bool) {
	if c == nil {
		return Archive{}, false
	}
	i, ok := slices.BinarySearchFunc(c.archives, id, func(a Archive, id string) int { return strings.Compare(a.ID, id) })
	if !ok {
		return Archive{}, false
	}
	return cloneArchive(c.archives[i]), true
}

// Load validates a bounded metadata manifest, including its canonical revision.
// It does not assert that any archive has been installed, run, or freshly fetched.
func Load(root string) (*Catalog, error) {
	r, err := openCache(root, false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return loadCatalog(r)
}

func loadCatalog(root *os.Root) (*Catalog, error) {
	f, err := openRegular(root, "catalog.json")
	if err != nil {
		return nil, fmt.Errorf("designassets: open catalog: %w", err)
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, MaxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > MaxCatalogBytes {
		return nil, fmt.Errorf("designassets: catalog exceeds %d bytes", MaxCatalogBytes)
	}
	return decodeCatalog(body)
}

func decodeCatalog(body []byte) (*Catalog, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(schemaSource)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("designassets: embedded schema: %w", err)
	}
	// The stored artifact is JSON, not executable CUE authored by the importer.
	if !json.Valid(body) {
		return nil, fmt.Errorf("designassets: catalog is not valid JSON")
	}
	value := ctx.CompileBytes(body).Unify(schema.LookupPath(cue.ParsePath("#Catalog")))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("designassets: catalog schema: %w", err)
	}
	var c Catalog
	if err := value.Decode(&c); err != nil {
		return nil, fmt.Errorf("designassets: decode catalog: %w", err)
	}
	if err := validateRecords(c.Archives); err != nil {
		return nil, err
	}
	want, err := catalogRevision(c.Archives)
	if err != nil {
		return nil, err
	}
	if c.Revision != want {
		return nil, fmt.Errorf("designassets: catalog revision mismatch")
	}
	c.archives = cloneArchives(c.Archives)
	return &c, nil
}

func catalogRevision(archives []Archive) (string, error) {
	body, err := json.Marshal(struct {
		SchemaVersion int       `json:"schema_version"`
		Archives      []Archive `json:"archives"`
	}{1, archives})
	if err != nil {
		return "", err
	}
	return digest(body), nil
}

func encodeCatalog(archives []Archive) ([]byte, error) {
	slices.SortFunc(archives, func(a, b Archive) int { return strings.Compare(a.ID, b.ID) })
	revision, err := catalogRevision(archives)
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(Catalog{SchemaVersion: 1, Revision: revision, Archives: archives}, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if int64(len(body)) > MaxCatalogBytes {
		return nil, fmt.Errorf("designassets: catalog exceeds %d bytes", MaxCatalogBytes)
	}
	if _, err := decodeCatalog(body); err != nil {
		return nil, err
	}
	return body, nil
}

func cloneArchives(in []Archive) []Archive {
	out := make([]Archive, len(in))
	for i, a := range in {
		out[i] = cloneArchive(a)
	}
	return out
}

func cloneArchive(a Archive) Archive {
	a.Files = slices.Clone(a.Files)
	a.Projects = slices.Clone(a.Projects)
	for i := range a.Projects {
		p := &a.Projects[i]
		p.Dependencies = maps.Clone(p.Dependencies)
		p.DevDependencies = maps.Clone(p.DevDependencies)
		p.PeerDependencies = maps.Clone(p.PeerDependencies)
		p.OptionalDependencies = maps.Clone(p.OptionalDependencies)
	}
	return a
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
