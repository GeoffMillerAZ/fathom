package designattempts

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

var errNoStore = errors.New("designattempts: no private attempt store")

// Detail is an integrity-checked syntax attempt with freshly checked original
// membership. Replay is matches-current or stale-transformer; neither certifies
// behavior, visual fidelity, security, performance or consumer integration.
type Detail struct {
	Revision    string                    `json:"revision"`
	Entry       Entry                     `json:"entry"`
	Translation designconvert.Translation `json:"translation"`
	Replay      string                    `json:"replay"`
}

// Load reads bounded discovery metadata only. Missing attempt storage is empty;
// a missing private root or corrupt existing manifest remains an error. It does
// not scan receipt files or original ZIPs; Read verifies those on exact access.
func Load(root string) (*Catalog, error) {
	r, err := openStore(root, false)
	if errors.Is(err, errNoStore) {
		return emptyCatalog()
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return loadIndex(r)
}

func emptyCatalog() (*Catalog, error) {
	body, err := encodeCatalog([]Entry{})
	if err != nil {
		return nil, err
	}
	return decodeCatalog(body)
}

func loadIndex(r *os.Root) (*Catalog, error) {
	body, err := readRegular(r, "catalog.json", MaxCatalogBytes)
	if errors.Is(err, os.ErrNotExist) {
		return emptyCatalog()
	}
	if err != nil {
		return nil, err
	}
	return decodeCatalog(body)
}

// Publish explicitly saves a current, replay-checked attempt under the existing
// private asset root. Originals and their catalog are never changed. Immutable
// receipts are linked without clobbering; the metadata index is replaced last.
// Concurrent writers are refused by an exclusive lock. An abandoned lock needs
// operator review; no automatic stale-lock removal or garbage collection occurs.
// Files are synced and publication is atomic for readers, but this API does not
// promise directory/power-loss durability. A failed index publication may leave
// an unlisted immutable receipt; retry can safely adopt the exact same bytes.
func Publish(root string, result designconvert.Translation) (Entry, error) {
	original, err := verifiedOriginal(root, result.Source)
	if err != nil {
		return Entry{}, err
	}
	if err := designconvert.VerifyReplay(result, []byte(original.Content)); err != nil {
		return Entry{}, fmt.Errorf("designattempts: only current replay-checked syntax attempts can be saved: %w", err)
	}
	body, err := json.Marshal(result)
	if err != nil || len(body) > designconvert.MaxTranslationBytes {
		return Entry{}, fmt.Errorf("designattempts: cannot encode bounded attempt")
	}
	entry := entryFor(result, body)
	r, err := openStore(root, true)
	if err != nil {
		return Entry{}, err
	}
	defer r.Close()
	lock, err := r.OpenFile(".publish.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Entry{}, fmt.Errorf("designattempts: acquire publish lock (another writer or abandoned lock): %w", err)
	}
	if err := lock.Close(); err != nil {
		_ = r.Remove(".publish.lock")
		return Entry{}, err
	}
	defer r.Remove(".publish.lock")
	prior, err := loadIndex(r)
	if err != nil {
		return Entry{}, err
	}
	dir, err := openDirectory(r, "entries", true)
	if err != nil {
		return Entry{}, err
	}
	dir.Close()
	filename := entryPath(entry.ID)
	if known, exists := prior.Get(entry.ID); exists {
		stored, err := readRegular(r, filename, designconvert.MaxTranslationBytes)
		if err != nil || !bytes.Equal(stored, body) || !reflect.DeepEqual(known, entry) {
			return Entry{}, fmt.Errorf("designattempts: existing attempt is missing or inconsistent; refusing replacement")
		}
		return known, nil
	}
	entries := append(prior.List(), entry)
	index, err := encodeCatalog(entries)
	if err != nil {
		return Entry{}, err
	}
	staged, err := stage(r, body)
	if err != nil {
		return Entry{}, err
	}
	defer r.Remove(staged)
	if err := r.Link(staged, filename); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return Entry{}, err
		}
		stored, readErr := readRegular(r, filename, designconvert.MaxTranslationBytes)
		if readErr != nil || !bytes.Equal(stored, body) {
			return Entry{}, fmt.Errorf("designattempts: immutable destination conflicts; refusing replacement")
		}
	}
	stagedIndex, err := stage(r, index)
	if err != nil {
		return Entry{}, err
	}
	defer r.Remove(stagedIndex)
	// A symlink or special manifest cannot be replaced as a repair side effect.
	if info, err := r.Lstat("catalog.json"); err == nil {
		if !info.Mode().IsRegular() {
			return Entry{}, fmt.Errorf("designattempts: catalog must remain a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Entry{}, err
	}
	if err := r.Rename(stagedIndex, "catalog.json"); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Read performs exact receipt/hash/schema/summary checks, verifies the original
// ZIP and member, and reruns a current transformer. Historical transformer
// identities are retained with stale-transformer, never a fabricated replay pass.
func Read(root, id string) (Detail, error) {
	if !validID(id) {
		return Detail{}, fmt.Errorf("designattempts: invalid exact attempt ID")
	}
	catalog, err := Load(root)
	if err != nil {
		return Detail{}, err
	}
	entry, ok := catalog.Get(id)
	if !ok {
		return Detail{}, fmt.Errorf("designattempts: unknown attempt: %w", os.ErrNotExist)
	}
	r, err := openStore(root, false)
	if err != nil {
		return Detail{}, err
	}
	defer r.Close()
	// Check the nested directory as well as the leaf; source content stays inert.
	dir, err := openDirectory(r, "entries", false)
	if err != nil {
		return Detail{}, err
	}
	defer dir.Close()
	body, err := readRegular(dir, strings.TrimPrefix(id, "syntax@")+".json", designconvert.MaxTranslationBytes)
	if err != nil {
		return Detail{}, err
	}
	result, err := designconvert.DecodeCanonical(body)
	if err != nil {
		return Detail{}, fmt.Errorf("designattempts: stored syntax envelope rejected: %w", err)
	}
	if !reflect.DeepEqual(entryFor(result, body), entry) {
		return Detail{}, fmt.Errorf("designattempts: immutable receipt identity or catalog summary mismatch")
	}
	original, err := verifiedOriginal(root, result.Source)
	if err != nil {
		return Detail{}, err
	}
	replay := "stale-transformer"
	if result.Transformer == designconvert.CurrentTransformer() {
		if err := designconvert.VerifyReplay(result, []byte(original.Content)); err != nil {
			return Detail{}, fmt.Errorf("designattempts: current transformer replay failed: %w", err)
		}
		replay = "matches-current"
	}
	return Detail{catalog.Revision, entry, result, replay}, nil
}

func verifiedOriginal(root string, source designconvert.Source) (*designassets.TextFile, error) {
	if len(source.ArchiveID) > 256 || len(source.Path) > designassets.MaxPathBytes {
		return nil, fmt.Errorf("designattempts: original identity exceeds bounds")
	}
	original, err := designassets.ReadFile(root, source.ArchiveID, source.Path)
	if err != nil {
		return nil, fmt.Errorf("designattempts: original verification: %w", err)
	}
	if source.ArchiveID != original.ArchiveID || source.Path != original.Path ||
		source.Size != len(original.Content) || source.SHA256 != original.SHA256 {
		return nil, fmt.Errorf("designattempts: receipt source does not match verified original membership")
	}
	return original, nil
}

func entryPath(id string) string { return "entries/" + strings.TrimPrefix(id, "syntax@") + ".json" }

func openStore(name string, create bool) (*os.Root, error) {
	if name == "" {
		return nil, fmt.Errorf("designattempts: existing private root is required")
	}
	name = filepath.Clean(name)
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("designattempts: private root must be a real directory")
	}
	r, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	child, err := openDirectory(r, "translations", create)
	if !create && errors.Is(err, os.ErrNotExist) {
		return nil, errNoStore
	}
	return child, err
}

func openDirectory(r *os.Root, name string, create bool) (*os.Root, error) {
	if create {
		if err := r.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	info, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("designattempts: %s must be a real directory", name)
	}
	return r.OpenRoot(name)
}

func readRegular(r *os.Root, name string, limit int) ([]byte, error) {
	before, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > int64(limit) {
		return nil, fmt.Errorf("designattempts: expected bounded regular file")
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() > int64(limit) {
		return nil, fmt.Errorf("designattempts: stored file changed or is not bounded/regular")
	}
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(body) > limit || int64(len(body)) != info.Size() {
		return nil, fmt.Errorf("designattempts: stored file read failed or changed")
	}
	return body, nil
}

func stage(r *os.Root, body []byte) (name string, err error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	name = ".write-" + hex.EncodeToString(nonce[:]) + ".tmp"
	f, err := r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = r.Remove(name)
		}
	}()
	if _, err = f.Write(body); err != nil {
		return name, err
	}
	if err = f.Sync(); err != nil {
		return name, err
	}
	err = f.Close()
	return name, err
}
