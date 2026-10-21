package scip

// Source receipts bind an externally produced index to the local inputs that
// produced it. They are freshness declarations, not signatures or execution
// attestations. Existing producers remain compatible without a receipt;
// scip-dotnet requires one because its generated projects/references are mutable.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const SourceReceiptSchema = "fathom.scip.source/v1"

// ErrSourceReceipt marks an operational refusal, rather than a legacy optional
// enrichment warning. Callers must not report the typed import as successful.
var ErrSourceReceipt = errors.New("SCIP source receipt refused")

const sourceReceiptLimit = 8 << 20
const sourceInputLimit = 20000
const sourceInputBytesLimit int64 = 1 << 30
const inventoryEntryLimit = 100000

type SourceReceipt struct {
	Schema      string            `json:"schema"`
	Status      string            `json:"status"`
	Root        string            `json:"root"`
	IndexSHA256 string            `json:"index_sha256"`
	Producer    SourceProducer    `json:"producer"`
	Inputs      []SourceInput     `json:"inputs"`
	Inventories []SourceInventory `json:"inventories"`
}
type SourceProducer struct {
	Name                string `json:"name"`
	Version             string `json:"version"`
	DistributionVersion string `json:"distribution_version,omitempty"`
	PackageSHA256       string `json:"package_sha256,omitempty"`
	PackagePath         string `json:"package_path,omitempty"`
}
type SourceInput struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type SourceInventory struct {
	Root    string   `json:"root"`
	Pattern string   `json:"pattern"`
	Paths   []string `json:"paths"`
}

// ValidateSourceReceipt returns bound=true when this index must carry a fresh
// sidecar. A failure never authorizes use of a previous import. The client owns
// clearing that index's namespace and evicting graph memos before returning it.
func ValidateSourceReceipt(ctx context.Context, root, path string, raw []byte, required ...bool) (bound bool, err error) {
	var index scippb.Index
	decodeErr := proto.Unmarshal(raw, &index)
	_, statErr := os.Stat(path + ".receipt.json")
	bound = statErr == nil || !os.IsNotExist(statErr) || index.GetMetadata().GetToolInfo().GetName() == "scip-dotnet" || len(required) > 0 && required[0]
	if !bound {
		return false, nil
	}
	refuse := func(format string, args ...any) (bool, error) {
		return true, fmt.Errorf("%w: %s", ErrSourceReceipt, fmt.Sprintf(format, args...))
	}
	if decodeErr != nil {
		return refuse("index decode: %v", decodeErr)
	}
	file, openErr := os.Open(path + ".receipt.json")
	if openErr != nil {
		return refuse("receipt: %v", openErr)
	}
	defer file.Close()
	data, readErr := io.ReadAll(io.LimitReader(file, sourceReceiptLimit+1))
	if readErr != nil {
		return refuse("receipt: %v", readErr)
	}
	if len(data) > sourceReceiptLimit {
		return refuse("receipt exceeds %d bytes", sourceReceiptLimit)
	}
	var receipt SourceReceipt
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decodeErr := decoder.Decode(&receipt); decodeErr != nil {
		return refuse("receipt JSON: %v", decodeErr)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return refuse("receipt has trailing JSON")
	}
	if receipt.Schema != SourceReceiptSchema {
		return refuse("schema %q is unsupported", receipt.Schema)
	}
	if receipt.Status != "complete" {
		return refuse("status is %q, not complete", receipt.Status)
	}
	if !filepath.IsAbs(receipt.Root) {
		return refuse("root must be absolute")
	}
	actualRoot, rootErr := sourceCanonical(root)
	if rootErr != nil {
		return refuse("service root: %v", rootErr)
	}
	receiptRoot, rootErr := sourceCanonical(receipt.Root)
	if rootErr != nil || actualRoot != receiptRoot {
		return refuse("root differs from the service root")
	}
	uri, uriErr := url.Parse(index.GetMetadata().GetProjectRoot())
	if uriErr != nil || uri.Scheme != "file" || uri.Host != "" || !filepath.IsAbs(filepath.FromSlash(uri.Path)) {
		return refuse("index project root is not a local file URI")
	}
	indexRoot, rootErr := sourceCanonical(filepath.FromSlash(uri.Path))
	if rootErr != nil || indexRoot != actualRoot {
		return refuse("index project root differs from the service root")
	}
	sum := sha256.Sum256(raw)
	if receipt.IndexSHA256 != hex.EncodeToString(sum[:]) {
		return refuse("index hash differs")
	}
	buffer := make([]byte, 32<<10)
	currentIndexHash, hashErr := sourceHash(ctx, path, int64(len(raw)), buffer)
	if hashErr != nil || currentIndexHash != receipt.IndexSHA256 {
		return refuse("index changed since it was read")
	}
	producer := index.GetMetadata().GetToolInfo()
	if receipt.Producer.Name != producer.GetName() || receipt.Producer.Version != producer.GetVersion() {
		return refuse("producer differs from the index metadata")
	}
	dotnet := producer.GetName() == "scip-dotnet"
	if dotnet && (receipt.Producer.DistributionVersion == "" || receipt.Producer.PackagePath == "" || !sourceDigest(receipt.Producer.PackageSHA256)) {
		return refuse("producer distribution/package identity is missing")
	}

	if len(receipt.Inputs) == 0 || len(receipt.Inputs) > sourceInputLimit {
		return refuse("input count is outside 1..%d", sourceInputLimit)
	}
	inputs := map[string]string{}
	var inputBytes int64
	for _, input := range receipt.Inputs {
		if err := ctx.Err(); err != nil {
			return refuse("input validation: %v", err)
		}
		absolute, pathErr := sourcePath(actualRoot, input.Path)
		if pathErr != nil {
			return refuse("input %q: %v", input.Path, pathErr)
		}
		if _, duplicate := inputs[absolute]; duplicate {
			return refuse("duplicate input %q", input.Path)
		}
		if !sourceDigest(input.SHA256) {
			return refuse("input %q has an invalid SHA256", input.Path)
		}
		info, statErr := os.Stat(absolute)
		if statErr != nil {
			return refuse("input %q: %v", input.Path, statErr)
		}
		if info.Size() > sourceInputBytesLimit-inputBytes {
			return refuse("input byte budget exceeded at %q", input.Path)
		}
		inputBytes += info.Size()
		actual, hashErr := sourceHash(ctx, absolute, info.Size(), buffer)
		if hashErr != nil {
			return refuse("input %q: %v", input.Path, hashErr)
		}
		if actual != input.SHA256 {
			return refuse("input hash changed: %s", input.Path)
		}
		inputs[absolute] = input.SHA256
	}

	if dotnet {
		packagePath, err := sourcePath(actualRoot, receipt.Producer.PackagePath)
		if err != nil || inputs[packagePath] != receipt.Producer.PackageSHA256 {
			return refuse("producer package is not bound by an input hash")
		}
		project, reference := false, false
		for path := range inputs {
			project = project || strings.EqualFold(filepath.Ext(path), ".csproj")
			reference = reference || strings.EqualFold(filepath.Ext(path), ".dll")
		}
		if !project || !reference {
			return refuse("producer inputs require real project and reference assembly hashes")
		}
	}
	if len(receipt.Inventories) == 0 {
		return refuse("source inventory is missing")
	}
	covered := map[string]bool{}
	for _, inventory := range receipt.Inventories {
		scope, pathErr := sourcePath(actualRoot, inventory.Root)
		if pathErr != nil || !sourceWithin(actualRoot, scope) {
			return refuse("inventory root must be within the service")
		}
		if !sourcePattern(inventory.Pattern) {
			return refuse("inventory pattern %q must be a simple *.extension", inventory.Pattern)
		}
		var actual []string
		seen := 0
		walkErr := filepath.WalkDir(scope, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			seen++
			if seen > inventoryEntryLimit {
				return fmt.Errorf("inventory entry limit exceeded")
			}
			if entry.IsDir() {
				return nil
			}
			match, _ := filepath.Match(inventory.Pattern, entry.Name())
			if !match {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("inventory source is not a regular file: %s", path)
			}
			canonical, err := sourceCanonical(path)
			if err != nil {
				return err
			}
			actual = append(actual, canonical)
			return nil
		})
		if walkErr != nil {
			return refuse("inventory %q: %v", inventory.Root, walkErr)
		}
		expected := make([]string, 0, len(inventory.Paths))
		for _, name := range inventory.Paths {
			absolute, pathErr := sourcePath(actualRoot, name)
			if pathErr != nil || !sourceWithin(scope, absolute) {
				return refuse("inventory path %q is outside its scope", name)
			}
			if _, ok := inputs[absolute]; !ok {
				return refuse("inventory path %q has no input hash", name)
			}
			expected = append(expected, absolute)
			covered[absolute] = true
		}
		slices.Sort(actual)
		slices.Sort(expected)
		if !slices.Equal(actual, expected) {
			return refuse("inventory changed: %s/%s", inventory.Root, inventory.Pattern)
		}
	}
	for _, document := range index.GetDocuments() {
		relative := filepath.FromSlash(document.GetRelativePath())
		if filepath.IsAbs(relative) || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return refuse("document path is not root-relative")
		}
		absolute, pathErr := sourcePath(actualRoot, relative)
		if pathErr != nil {
			return refuse("document %q: %v", relative, pathErr)
		}
		if _, ok := inputs[absolute]; !ok {
			return refuse("document %q has no source hash", relative)
		}
		if !covered[absolute] {
			return refuse("document %q is outside the declared inventories", relative)
		}
	}
	if len(index.GetDocuments()) == 0 {
		return refuse("index has no documents")
	}
	return true, nil
}

func sourceCanonical(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}
func sourcePath(root, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	return sourceCanonical(path)
}
func sourceWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
func sourceDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && strings.ToLower(value) == value
}
func sourcePattern(value string) bool {
	return regexp.MustCompile(`^\*\.[A-Za-z0-9_-]+$`).MatchString(value)
}
func sourceHash(ctx context.Context, path string, expectedSize int64, buffer []byte) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file")
	}
	if info.Size() != expectedSize {
		return "", fmt.Errorf("file size changed during validation")
	}
	hash := sha256.New()
	n, err := io.CopyBuffer(hash, sourceContextReader{ctx: ctx, reader: io.LimitReader(file, expectedSize+1)}, buffer)
	if err != nil {
		return "", err
	}
	if n != expectedSize {
		return "", fmt.Errorf("file size changed during hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SourceReceiptExpected only reads the bounded metadata prefix of a SCIP file.
// Metadata must be first under the SCIP streaming contract. It never decodes a
// large legacy index merely to decide whether automatic source validation applies.
func SourceReceiptExpected(path string) bool {
	if _, err := os.Stat(path + ".receipt.json"); err == nil || !os.IsNotExist(err) {
		return true
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return false
	}
	number, wire, n := protowire.ConsumeTag(raw)
	if n < 0 || number != 1 || wire != protowire.BytesType {
		return false
	}
	value, n := protowire.ConsumeBytes(raw[n:])
	if n < 0 {
		return false
	}
	var metadata scippb.Metadata
	if proto.Unmarshal(value, &metadata) != nil {
		return false
	}
	return metadata.GetToolInfo().GetName() == "scip-dotnet"
}

type sourceContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r sourceContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
