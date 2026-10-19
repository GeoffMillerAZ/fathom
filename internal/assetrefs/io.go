package assetrefs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const artifactLimit = 16 << 20
const inputLimit = 8 << 20
const sourceLimit = 64 << 20
const memberLimit = 50000
const entityLimit = 100000
const referenceLimit = 250000

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var guidPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var metaGUIDPattern = regexp.MustCompile(`(?m)^guid: ([0-9a-f]{32})$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func refused(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, args...))
}
func digest(raw []byte) string { hash := sha256.Sum256(raw); return hex.EncodeToString(hash[:]) }
func safeText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func relative(s string) bool {
	return safeText(s) && fs.ValidPath(s) && s != "." && !strings.Contains(s, "\\") && !strings.Contains(s, ":")
}
func skippedPart(p string) bool {
	for _, part := range strings.Split(p, "/") {
		switch part {
		case ".git", ".cache", "Library", "Temp", "Obj", "Logs", "Builds", "Vendor", "ThirdParty", "Packages":
			return true
		}
	}
	return false
}
func assetPath(p string) bool {
	return relative(p) && (p == "Assets" || strings.HasPrefix(p, "Assets/")) && !skippedPart(p)
}
func projectInput(p string) bool {
	return assetPath(p) || p == "ProjectSettings/ProjectVersion.txt" || p == "Packages/manifest.json" || p == "Packages/packages-lock.json"
}
func artifactPath(p string) bool {
	return relative(p) && strings.HasSuffix(p, ".json") && (strings.HasPrefix(p, ".artifacts/") || strings.HasPrefix(p, ".cache/"))
}

// readRegular never follows a symlink in the supplied root or relative path.
// Paths from artifacts cannot escape into arbitrary host files.
func readRegular(ctx context.Context, root, rel string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !relative(rel) {
		return nil, refused("noncanonical local path")
	}
	p := filepath.Join(root, filepath.FromSlash(rel))
	for at := p; ; at = filepath.Dir(at) {
		info, err := os.Lstat(at)
		if err != nil {
			return nil, refused("read %s: %v", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, refused("symlink input %s", rel)
		}
		if at == p && (!info.Mode().IsRegular() || info.Size() > limit) {
			return nil, refused("nonregular/oversize input %s", rel)
		}
		if parent := filepath.Dir(at); parent == at {
			break
		}
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, refused("open %s: %v", rel, err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, refused("read %s: %v", rel, err)
	}
	if int64(len(raw)) > limit {
		return nil, refused("input grew beyond bound %s", rel)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func readHash(ctx context.Context, root string, file fileHash, limit int64) ([]byte, error) {
	if !shaPattern.MatchString(file.SHA256) {
		return nil, refused("invalid SHA256")
	}
	raw, err := readRegular(ctx, root, file.Path, limit)
	if err != nil {
		return nil, err
	}
	if digest(raw) != file.SHA256 {
		return nil, refused("stale/mixed bytes %s", file.Path)
	}
	return raw, nil
}

// Standard decoding silently overwrites duplicate JSON keys. Walk the bounded
// tokens first, refusing duplicates and deep/invalid input before typed decode.
func strictJSON(raw []byte, value any) error {
	if len(raw) == 0 || len(raw) > artifactLimit || !utf8.Valid(raw) {
		return refused("empty/oversize/nonUTF8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := uniqueJSON(d, 0, reflect.TypeOf(value), map[reflect.Type]map[string]reflect.Type{}); err != nil {
		return refused("JSON: %v", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return refused("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return refused("JSON shape: %v", err)
	}
	return nil
}
func uniqueJSON(d *json.Decoder, depth int, expected reflect.Type, fields map[reflect.Type]map[string]reflect.Type) error {
	if depth > 64 {
		return fmt.Errorf("nesting exceeds64")
	}
	for expected != nil && expected.Kind() == reflect.Pointer {
		expected = expected.Elem()
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		var tags map[string]reflect.Type
		if expected != nil && expected.Kind() == reflect.Struct {
			tags = fields[expected]
			if tags == nil {
				tags = map[string]reflect.Type{}
				for index := 0; index < expected.NumField(); index++ {
					field := expected.Field(index)
					if field.PkgPath != "" {
						continue
					}
					tag := strings.Split(field.Tag.Get("json"), ",")[0]
					if tag != "" && tag != "-" {
						tags[tag] = field.Type
					}
				}
				fields[expected] = tags
			}
		} else if expected != nil && expected.Kind() != reflect.Map && expected.Kind() != reflect.Interface {
			return fmt.Errorf("unexpected object shape")
		}
		keys := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || keys[key] {
				return fmt.Errorf("duplicate/invalid object key")
			}
			keys[key] = true
			var child reflect.Type
			if tags != nil {
				var known bool
				child, known = tags[key]
				if !known {
					return fmt.Errorf("unknown/noncanonical JSON field %q", key)
				}
			} else if expected != nil && expected.Kind() == reflect.Map {
				child = expected.Elem()
			}
			if err := uniqueJSON(d, depth+1, child, fields); err != nil {
				return err
			}
		}
	case '[':
		var child reflect.Type
		if expected != nil {
			switch expected.Kind() {
			case reflect.Array, reflect.Slice:
				child = expected.Elem()
			case reflect.Interface:
			default:
				return fmt.Errorf("unexpected array shape")
			}
		}
		for d.More() {
			if err := uniqueJSON(d, depth+1, child, fields); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	_, err = d.Token()
	return err
}

func metadata(ctx context.Context, projectRoot string) ([]string, error) {
	var members []string
	governor := tenancy.NewRowGovernor(ctx, tenancy.ClassTree)
	err := filepath.WalkDir(filepath.Join(projectRoot, "Assets"), func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := governor.Row(); err != nil {
			return err
		}
		rel, err := filepath.Rel(projectRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if skippedPart(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(rel, ".meta") {
			if !entry.Type().IsRegular() {
				return refused("nonregular metadata")
			}
			members = append(members, rel)
			if len(members) > memberLimit {
				return refused("metadata membership exceeds bound")
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: metadata membership: %w", ErrRefused, err)
	}
	slices.Sort(members)
	return members, nil
}

func inputMap(files []fileHash) (map[string]string, error) {
	if len(files) == 0 || len(files) > memberLimit {
		return nil, refused("empty/overbound source inventory")
	}
	m := make(map[string]string, len(files))
	last := ""
	for _, file := range files {
		if !projectInput(file.Path) || !shaPattern.MatchString(file.SHA256) || file.Path <= last {
			return nil, refused("duplicate/unsorted/noncanonical input")
		}
		m[file.Path] = file.SHA256
		last = file.Path
	}
	return m, nil
}

func sourceBytes(r receipt, i inventory) []byte {
	var b strings.Builder
	b.WriteString("fathom.unity.asset-inputs/v1\nproject\x00" + r.Project + "\n")
	b.WriteString("producer\x00" + r.Producer.Path + "\x00" + r.Producer.SHA256 + "\n")
	for _, file := range r.Producer.Inputs {
		b.WriteString("tool\x00" + file.Path + "\x00" + file.SHA256 + "\n")
	}
	if r.ContextInventory != nil {
		b.WriteString("context\x00" + r.ContextInventory.Before + "\n")
	}
	for _, scope := range i.Scopes {
		b.WriteString("scope\x00" + scope + "\n")
	}
	metadataMembers := append([]string(nil), i.Metadata...)
	slices.Sort(metadataMembers)
	for _, meta := range metadataMembers {
		b.WriteString("metadata\x00" + meta + "\n")
	}
	for _, file := range i.Inputs {
		b.WriteString("file\x00" + file.Path + "\x00" + file.SHA256 + "\n")
	}
	return []byte(b.String())
}

func validateSource(ctx context.Context, opts Options, r receipt, i inventory) error {
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
		return err
	}
	if r.ContextInventory == nil {
		return refused("missing authored semantic context inventory")
	}
	if err := checkSourceMemberCount(len(i.Inputs), len(r.ContextInventory.AssetsCode), len(r.ContextInventory.ProjectSettings)); err != nil {
		return err
	}
	projectRoot := filepath.Join(opts.Root, filepath.FromSlash(opts.Project))
	if !shaPattern.MatchString(r.Source.Before) || r.Source.Before != r.Source.After || digest(sourceBytes(r, i)) != r.Source.Before {
		return refused("incomplete/mixed canonical source identity")
	}
	producerBytes, err := readHash(ctx, opts.Root, fileHash{r.Producer.Path, r.Producer.SHA256}, inputLimit)
	if err != nil {
		return err
	}
	toolBytes := len(producerBytes)
	for _, file := range r.Producer.Inputs {
		raw, err := readHash(ctx, opts.Root, file, inputLimit)
		if err != nil {
			return err
		}
		toolBytes += len(raw)
		if toolBytes > inputLimit {
			return refused("producer closure exceeds8MiB")
		}
	}
	members, err := metadata(ctx, projectRoot)
	if err != nil {
		return err
	}
	expectedMembers := append([]string(nil), i.Metadata...)
	slices.Sort(expectedMembers)
	for index, member := range expectedMembers {
		if index > 0 && member == expectedMembers[index-1] {
			return refused("duplicate metadata membership")
		}
	}
	if !slices.Equal(members, expectedMembers) {
		return refused("stale metadata membership")
	}
	total := toolBytes
	governor := tenancy.NewRowGovernor(ctx, tenancy.ClassTree)
	for _, file := range i.Inputs {
		if err := governor.Row(); err != nil {
			return err
		}
		raw, err := readHash(ctx, projectRoot, file, inputLimit)
		if err != nil {
			return err
		}
		total += len(raw)
		if total > sourceLimit {
			return refused("source closure exceeds64MiB")
		}
	}
	if err := validateContext(ctx, projectRoot, r.ContextInventory, total); err != nil {
		return err
	}
	return tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree)
}

func checkScopes(scopes []string) error {
	if len(scopes) == 0 || len(scopes) > 32 {
		return refused("empty/overbound scopes")
	}
	last := ""
	for _, scope := range scopes {
		if !assetPath(scope) || scope <= last {
			return refused("duplicate/unsorted/unsafe scope")
		}
		last = scope
	}
	return nil
}
func inScope(p string, scopes []string) bool {
	for _, scope := range scopes {
		if p == scope || strings.HasPrefix(p, scope+"/") {
			return true
		}
	}
	return false
}
func entitySourcePath(p string) bool {
	switch path.Ext(p) {
	case ".prefab", ".mat", ".asset":
		return assetPath(p)
	}
	return false
}
