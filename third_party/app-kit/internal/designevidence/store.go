package designevidence

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

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

// MaxBundleBytes bounds JSON (including base64 expansion) accepted from the
// CLI/MCP boundary. Decoded artifact bytes remain bounded by designreceipts.
const MaxBundleBytes = 92 << 20

var errNoStore = errors.New("designevidence: no private evidence store")

// Detail is a freshly integrity-checked historical bundle. Rechecked means
// bundled bytes and current original membership matched; it is explicitly not
// current transformer, semantic, target, runtime, or compatibility proof.
type Detail struct {
	Revision           string                  `json:"revision"`
	Entry              Entry                   `json:"entry"`
	Contract           designreceipts.Contract `json:"contract"`
	Receipt            designreceipts.Receipt  `json:"receipt"`
	Rechecked          designreceipts.Report   `json:"rechecked"`
	OriginalMembership string                  `json:"original_membership"`
}

type bundle struct {
	SchemaVersion int               `json:"schema_version"`
	SourceMember  string            `json:"source_member"`
	Contract      json.RawMessage   `json:"contract"`
	Receipt       json.RawMessage   `json:"receipt"`
	Content       map[string][]byte `json:"content"`
}

type checkedEvidence struct {
	Bundle  bundle
	Receipt designreceipts.Receipt
}

// EncodeBundle makes the sole canonical storage representation. Content is
// copied, not retained; no ZIP or filesystem protocol is involved.
func EncodeBundle(sourceMember string, contractBody, receiptBody []byte, content map[string][]byte) ([]byte, error) {
	b := bundle{SchemaVersion: 1, SourceMember: sourceMember, Contract: slicesClone(contractBody), Receipt: slicesClone(receiptBody), Content: cloneContent(content)}
	body, err := json.Marshal(b)
	if err != nil || len(body) > MaxBundleBytes {
		return nil, fmt.Errorf("designevidence: cannot encode bounded bundle")
	}
	if _, err := decodeBundle(body); err != nil {
		return nil, err
	}
	return body, nil
}

// Publish verifies independent expected pins and current original membership
// before writing anything. It retains all supplied exact bytes as immutable
// history but does not execute reports or certify target behavior.
func Publish(root, sourceMember string, contractBody, receiptBody []byte, expected designreceipts.Expected, content map[string][]byte) (Entry, error) {
	body, err := EncodeBundle(sourceMember, contractBody, receiptBody, content)
	if err != nil {
		return Entry{}, err
	}
	return publishBundle(root, body, expected)
}

// PublishBundle accepts only the canonical bounded bundle and never derives
// freshness pins from it. Callers must independently choose expected.
func PublishBundle(root string, body []byte, expected designreceipts.Expected) (Entry, error) {
	return publishBundle(root, body, expected)
}

func publishBundle(root string, body []byte, expected designreceipts.Expected) (Entry, error) {
	b, err := decodeBundle(body)
	if err != nil {
		return Entry{}, err
	}
	report, err := designreceipts.Check(b.Contract, b.Receipt, expected, b.Content)
	if err != nil {
		return Entry{}, fmt.Errorf("designevidence: evidence integrity: %w", err)
	}
	if err := verifyOriginal(root, b.SourceMember, report.Subject.SourceArchive, expected.SourceRevision); err != nil {
		return Entry{}, err
	}
	entry := entryFor(body, b.SourceMember, report)
	r, err := openStore(root, true)
	if err != nil {
		return Entry{}, err
	}
	defer r.Close()
	lock, err := r.OpenFile(".publish.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Entry{}, fmt.Errorf("designevidence: acquire publish lock (another writer or abandoned lock): %w", err)
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
	file := strings.TrimPrefix(entry.ID, "evidence@") + ".json"
	if old, ok := prior.Get(entry.ID); ok {
		entryRoot, openErr := openDirectory(r, "entries", false)
		if openErr != nil {
			return Entry{}, openErr
		}
		stored, err := readRegular(entryRoot, file, MaxBundleBytes)
		entryRoot.Close()
		if err != nil || !bytes.Equal(stored, body) || !reflect.DeepEqual(old, entry) {
			return Entry{}, fmt.Errorf("designevidence: existing bundle is missing or inconsistent; refusing replacement")
		}
		return old, nil
	}
	index, err := encodeCatalog(append(prior.List(), entry))
	if err != nil {
		return Entry{}, err
	}
	staged, err := stage(r, body)
	if err != nil {
		return Entry{}, err
	}
	defer r.Remove(staged)
	if err := r.Link(staged, "entries/"+file); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return Entry{}, err
		}
		entryRoot, openErr := openDirectory(r, "entries", false)
		if openErr != nil {
			return Entry{}, openErr
		}
		stored, readErr := readRegular(entryRoot, file, MaxBundleBytes)
		entryRoot.Close()
		if readErr != nil || !bytes.Equal(stored, body) {
			return Entry{}, fmt.Errorf("designevidence: immutable destination conflicts; refusing replacement")
		}
	}
	stagedIndex, err := stage(r, index)
	if err != nil {
		return Entry{}, err
	}
	defer r.Remove(stagedIndex)
	if info, err := r.Lstat("catalog.json"); err == nil && !info.Mode().IsRegular() {
		return Entry{}, fmt.Errorf("designevidence: catalog must remain a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Entry{}, err
	}
	// Entries are linked first and catalog replacement is last, so failures do
	// not overwrite published data. Owner-controlled storage makes no crash or
	// hostile-same-user durability claim.
	if err := r.Rename(stagedIndex, "catalog.json"); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Load reads only bounded immutable metadata. It intentionally does not check
// bundle/original freshness; Read is the exact-verification operation.
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

// Read verifies exact canonical bundle identity, historical receipt integrity,
// current original ZIP/member membership, and the catalog summary. Historical
// expected pins are reconstructed from the stored contract only for integrity,
// not presented as independently trusted current pins.
func Read(root, id string) (Detail, error) {
	detail, _, err := readCheckedBundle(root, id)
	return detail, err
}

// readCheckedBundle is the single exact-read path for metadata and narrowly
// admitted private content readers. Its bundle must never escape this package.
func readCheckedBundle(root, id string) (Detail, checkedEvidence, error) {
	if !validID(id) {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: invalid exact evidence ID")
	}
	catalog, err := Load(root)
	if err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	entry, ok := catalog.Get(id)
	if !ok {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: unknown evidence: %w", os.ErrNotExist)
	}
	r, err := openStore(root, false)
	if err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	defer r.Close()
	entryRoot, err := openDirectory(r, "entries", false)
	if err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	defer entryRoot.Close()
	body, err := readRegular(entryRoot, strings.TrimPrefix(id, "evidence@")+".json", MaxBundleBytes)
	if err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	b, err := decodeBundle(body)
	if err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	if entry.ID != "evidence@"+digest(body) {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: bundle hash mismatch")
	}
	var contract designreceipts.Contract
	if err := json.Unmarshal(b.Contract, &contract); err != nil {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: stored contract rejected")
	}
	var receipt designreceipts.Receipt
	if err := json.Unmarshal(b.Receipt, &receipt); err != nil {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: stored receipt rejected")
	}
	expected := designreceipts.Expected{ContractSHA256: designreceipts.Digest(b.Contract), SourceRevision: contract.Subject.SourceRevision, ArtifactSHA256: contract.Subject.ArtifactSHA256}
	rechecked, err := designreceipts.Check(b.Contract, b.Receipt, expected, b.Content)
	if err != nil {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: historical evidence integrity: %w", err)
	}
	if err := verifyOriginal(root, b.SourceMember, contract.Subject.SourceArchive, expected.SourceRevision); err != nil {
		return Detail{}, checkedEvidence{}, err
	}
	if !reflect.DeepEqual(entryFor(body, b.SourceMember, rechecked), entry) {
		return Detail{}, checkedEvidence{}, fmt.Errorf("designevidence: immutable bundle identity or catalog summary mismatch")
	}
	return Detail{Revision: catalog.Revision, Entry: cloneEntry(entry), Contract: contract, Receipt: receipt, Rechecked: cloneReport(rechecked), OriginalMembership: "matched"}, checkedEvidence{Bundle: b, Receipt: receipt}, nil
}

func entryFor(body []byte, sourceMember string, report designreceipts.Report) Entry {
	report.Verification = "not-verified"
	return Entry{ID: "evidence@" + digest(body), SourceMember: sourceMember, Recorded: cloneReport(report)}
}
func cloneReport(r designreceipts.Report) designreceipts.Report {
	r.ClaimedStatuses = mapsClone(r.ClaimedStatuses)
	r.IncompleteGates = slicesClone(r.IncompleteGates)
	return r
}
func cloneContent(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		out[k] = slicesClone(v)
	}
	return out
}
func slicesClone[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func decodeBundle(body []byte) (bundle, error) {
	if len(body) == 0 || len(body) > MaxBundleBytes || !json.Valid(body) {
		return bundle{}, fmt.Errorf("designevidence: invalid bundle encoding or size")
	}
	var b bundle
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&b); err != nil {
		return bundle{}, fmt.Errorf("designevidence: invalid bundle JSON fields")
	}
	if d.Decode(&struct{}{}) != io.EOF || b.SchemaVersion != 1 || !validMember(b.SourceMember) || b.Content == nil || len(b.Content) > 1024 || len(b.Contract) == 0 || len(b.Receipt) == 0 {
		return bundle{}, fmt.Errorf("designevidence: invalid bundle")
	}
	canonical, err := json.Marshal(b)
	if err != nil || !bytes.Equal(body, canonical) {
		return bundle{}, fmt.Errorf("designevidence: bundle is not canonical JSON")
	}
	// Check independently validates CUE/canonical receipt documents, inventories,
	// hashes, and the 64 MiB decoded aggregate bound before any filesystem write.
	return b, nil
}

func verifyOriginal(root, member, archive, revision string) error {
	original, err := designassets.ReadFile(root, archive, member)
	if err != nil {
		return fmt.Errorf("designevidence: original verification: %w", err)
	}
	if original.ArchiveID != archive || original.Path != member || original.SHA256 != revision {
		return fmt.Errorf("designevidence: original membership does not match expected source revision")
	}
	return nil
}

func openStore(name string, create bool) (*os.Root, error) {
	if name == "" {
		return nil, fmt.Errorf("designevidence: existing private root is required")
	}
	name = filepath.Clean(name)
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("designevidence: private root must be a real directory")
	}
	r, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	child, err := openDirectory(r, "evidence", create)
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
		return nil, fmt.Errorf("designevidence: %s must be a real directory", name)
	}
	return r.OpenRoot(name)
}
func readRegular(r *os.Root, name string, limit int) ([]byte, error) {
	before, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > int64(limit) {
		return nil, fmt.Errorf("designevidence: expected bounded regular file")
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() > int64(limit) {
		return nil, fmt.Errorf("designevidence: stored file changed or is not bounded/regular")
	}
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(body) > limit || int64(len(body)) != info.Size() {
		return nil, fmt.Errorf("designevidence: stored file read failed or changed")
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
