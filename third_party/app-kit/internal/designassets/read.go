package designassets

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Verify returns exact metadata only after checking the entire original ZIP's
// size and SHA256 and re-deriving its file/project inventory. This proves local
// byte integrity and metadata binding, not vendor authenticity.
func Verify(root, id string) (Archive, error) {
	r, err := openCache(root, false)
	if err != nil {
		return Archive{}, err
	}
	defer r.Close()
	c, err := loadCatalog(r)
	if err != nil {
		return Archive{}, err
	}
	a, ok := c.Get(id)
	if !ok {
		return Archive{}, fmt.Errorf("designassets: unknown archive %q: %w", id, os.ErrNotExist)
	}
	f, err := verifiedArchive(context.Background(), r, a)
	if err != nil {
		return Archive{}, err
	}
	defer f.Close()
	if err := verifyMetadata(context.Background(), f, a); err != nil {
		return Archive{}, err
	}
	return a, nil
}

func verifyMetadata(ctx context.Context, f *os.File, a Archive) error {
	actual, _, err := inspect(ctx, f, a.Size, a.Filename, a.SHA256)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, a) {
		return fmt.Errorf("designassets: catalog metadata does not match original archive %s", a.ID)
	}
	return nil
}

// ReadFile verifies the complete ZIP, then CRC/size/SHA256 of one inventoried
// member. Only bounded UTF-8 text is returned; sensitive filenames are denied.
func ReadFile(root, id, name string) (*TextFile, error) {
	if err := validPath(name); err != nil {
		return nil, err
	}
	r, err := openCache(root, false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	c, err := loadCatalog(r)
	if err != nil {
		return nil, err
	}
	a, ok := c.Get(id)
	if !ok {
		return nil, fmt.Errorf("designassets: unknown archive %q: %w", id, os.ErrNotExist)
	}
	i, ok := slices.BinarySearchFunc(a.Files, name, func(f File, s string) int { return strings.Compare(f.Path, s) })
	if !ok {
		return nil, fmt.Errorf("designassets: unknown member %q: %w", name, os.ErrNotExist)
	}
	metadata := a.Files[i]
	if metadata.Size > MaxTextBytes || metadata.Kind == "sensitive" || metadata.Kind == "asset" && !strings.HasSuffix(strings.ToLower(name), ".svg") {
		return nil, fmt.Errorf("designassets: member is sensitive, binary, or exceeds text limit (%d bytes)", MaxTextBytes)
	}
	f, err := verifiedArchive(context.Background(), r, a)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := boundedZIP(f, a.Size)
	if err != nil {
		return nil, err
	}
	var body []byte
	found := false
	for _, member := range z.File {
		if member.Name != name {
			continue
		}
		if found || !member.Mode().IsRegular() {
			return nil, fmt.Errorf("designassets: conflicting member")
		}
		found = true
		reader, err := member.Open()
		if err != nil {
			return nil, err
		}
		body, err = io.ReadAll(io.LimitReader(reader, MaxTextBytes+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if !found || int64(len(body)) != metadata.Size || digest(body) != metadata.SHA256 {
		return nil, fmt.Errorf("designassets: member integrity mismatch")
	}
	if !utf8.Valid(body) || !safeText(string(body)) {
		return nil, fmt.Errorf("designassets: binary or sensitive text refused")
	}
	return &TextFile{id, name, metadata.SHA256, metadata.Size, string(body)}, nil
}

func safeText(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' || r == 127 {
			return false
		}
	}
	upper := strings.ToUpper(s)
	return !strings.Contains(upper, "PRIVATE KEY-----") && !strings.Contains(upper, "BEGIN PGP PRIVATE KEY") && !credentialText.MatchString(s)
}

// This is a conservative disclosure guard, not a universal secret detector.
// Sensitive filenames are refused independently, even when their content is a
// harmless example. Credential-shaped source must be inspected outside this API.
var credentialText = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://[^\s/"'<>]+:[^\s/"'<>]+@|(?:AKIA|ASIA)[A-Z0-9]{16}|gh[pousr]_[A-Z0-9]{20,}|github_pat_[A-Z0-9_]{20,}|sk-(?:proj-)?[A-Z0-9_-]{20,}|[?&](?:access_token|api_key|token|password|secret)=[^\s&"'<>]+)`)
