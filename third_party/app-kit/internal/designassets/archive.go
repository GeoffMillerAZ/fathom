package designassets

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type product struct{ family, kind, url string }

func productFor(filename string) (product, bool) {
	name, ok := strings.CutSuffix(filename, ".zip")
	if !ok {
		return product{}, false
	}
	switch name {
	case "syntax", "protocol", "pocket", "primer", "commit", "studio", "radiant", "keynote", "compass", "transmit", "salient", "spotlight":
		return product{name, "template", "https://tailwindcss.com/plus/templates/" + name}, true
	case "oatmeal-olive-instrument":
		return product{"oatmeal", "template", "https://tailwindcss.com/plus/kits/oatmeal"}, true
	case "catalyst-ui-kit":
		return product{"catalyst", "ui-kit", "https://tailwindcss.com/plus/ui-kit"}, true
	case "marketing-v4", "application-ui-v4", "ecommerce-v4":
		family := strings.TrimSuffix(name, "-v4")
		return product{family, "ui-blocks", "https://tailwindcss.com/plus/ui-blocks/" + family}, true
	}
	return product{}, false
}

func validPath(name string) error {
	if name == "" || len(name) > MaxPathBytes || !utf8.ValidString(name) || strings.ContainsAny(name, "\\\x00:") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("designassets: unsafe member path %q", name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("designassets: control character in path")
		}
	}
	return nil
}

// Unicode simple-fold equivalence also catches names such as sigma/final-sigma.
func folded(s string) string {
	return strings.Map(func(r rune) rune {
		min := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < min {
				min = next
			}
		}
		return min
	}, s)
}

type pathNode struct {
	name          string
	dir, explicit bool
}
type pathSet map[string]pathNode

func (seen pathSet) add(name string, dir bool) error {
	if err := validPath(name); err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		isDir, explicit := i < len(parts)-1 || dir, i == len(parts)-1
		key := folded(current)
		if old, ok := seen[key]; ok {
			if old.name != current || old.dir != isDir || (old.explicit && explicit) {
				return fmt.Errorf("designassets: duplicate, case-colliding, or conflicting path %q", name)
			}
			if explicit {
				old.explicit = true
				seen[key] = old
			}
		} else {
			seen[key] = pathNode{current, isDir, explicit}
		}
	}
	return nil
}

func classify(name string) string {
	base := strings.ToLower(path.Base(name))
	ext := strings.ToLower(path.Ext(name))
	if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".npmrc" || base == ".netrc" || base == "id_rsa" || base == "id_ed25519" || ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" {
		return "sensitive"
	}
	if base == "package.json" || strings.Contains(base, "lock") && (ext == ".json" || ext == ".yaml" || ext == ".lock") {
		return "manifest"
	}
	if strings.HasPrefix(base, "readme") {
		return "readme"
	}
	if strings.HasPrefix(base, "license") || strings.HasPrefix(base, "licence") || strings.HasPrefix(base, "copying") {
		return "license"
	}
	if ext == ".css" || ext == ".scss" || ext == ".sass" || ext == ".less" || strings.HasPrefix(base, "tailwind.config.") {
		return "theme"
	}
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".html", ".vue", ".svelte", ".mjs", ".cjs":
		return "source"
	case ".md", ".mdx", ".rst", ".txt":
		return "documentation"
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".ico", ".svg", ".avif", ".woff", ".woff2", ".ttf", ".otf", ".mp4", ".mp3", ".pdf", ".gz", ".zip":
		return "asset"
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".config", ".gitignore", ".prettierrc", ".editorconfig":
		return "config"
	}
	if strings.HasPrefix(base, ".") {
		return "config"
	}
	return "other"
}

// Inspect the fixed-size ZIP trailer before archive/zip allocates its file list.
// ZIP64/multi-disk downloads are deliberately outside this bounded format.
func boundedZIP(r io.ReaderAt, size int64) (*zip.Reader, error) {
	if size <= 0 || size > MaxArchiveBytes {
		return nil, fmt.Errorf("designassets: archive size out of bounds")
	}
	n := min(size, int64(65557))
	tail := make([]byte, n)
	if _, err := r.ReadAt(tail, size-n); err != nil {
		return nil, err
	}
	for i := len(tail) - 22; i >= 0; i-- {
		if !bytes.Equal(tail[i:i+4], []byte{'P', 'K', 5, 6}) {
			continue
		}
		// Go selects the last footer signature even when trailing bytes remain
		// after its comment (and stops scanning on a truncated comment). Never
		// skip that candidate to validate an earlier footer: NewReader could
		// then select a different, unchecked directory inside the comment.
		if i+22+int(binary.LittleEndian.Uint16(tail[i+20:])) != len(tail) {
			return nil, fmt.Errorf("designassets: ambiguous ZIP footer or trailing/truncated comment data")
		}
		count := binary.LittleEndian.Uint16(tail[i+10:])
		cdSize, cdOff := binary.LittleEndian.Uint32(tail[i+12:]), binary.LittleEndian.Uint32(tail[i+16:])
		// Go also probes a ZIP64 locator for its directory-size sentinel 0xffff;
		// reject it rather than allow another footer to override this preflight.
		if binary.LittleEndian.Uint16(tail[i+4:]) != 0 || binary.LittleEndian.Uint16(tail[i+6:]) != 0 || binary.LittleEndian.Uint16(tail[i+8:]) != count || count == 65535 || int(count) > MaxMembers || cdSize == 0xffff || cdSize > 32<<20 || int64(cdOff)+int64(cdSize) != size-n+int64(i) {
			return nil, fmt.Errorf("designassets: ZIP directory exceeds bounds or uses unsupported ZIP64/multi-disk format")
		}
		// archive/zip accepts wrapped EOCD counts and reads until the next
		// signature, not the advertised count. Walk the exact bounded extent
		// ourselves before letting it allocate even one File per record.
		offset, end := int64(cdOff), int64(cdOff)+int64(cdSize)
		var header [46]byte
		for entry := 0; entry < int(count); entry++ {
			if offset+int64(len(header)) > end {
				return nil, fmt.Errorf("designassets: truncated central directory")
			}
			if _, err := r.ReadAt(header[:], offset); err != nil {
				return nil, err
			}
			if !bytes.Equal(header[:4], []byte{'P', 'K', 1, 2}) {
				return nil, fmt.Errorf("designassets: invalid central directory record")
			}
			offset += int64(len(header)) + int64(binary.LittleEndian.Uint16(header[28:])) + int64(binary.LittleEndian.Uint16(header[30:])) + int64(binary.LittleEndian.Uint16(header[32:]))
			if offset > end {
				return nil, fmt.Errorf("designassets: central directory record exceeds extent")
			}
		}
		if offset != end {
			return nil, fmt.Errorf("designassets: central directory count/extent mismatch")
		}
		z, err := zip.NewReader(r, size)
		if err != nil {
			return nil, fmt.Errorf("designassets: ZIP: %w", err)
		}
		if len(z.File) != int(count) || len(z.File) > MaxMembers {
			return nil, fmt.Errorf("designassets: ZIP member count mismatch")
		}
		return z, nil
	}
	return nil, fmt.Errorf("designassets: missing ZIP directory")
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func inspect(ctx context.Context, r io.ReaderAt, size int64, filename, hash string) (Archive, int64, error) {
	p, ok := productFor(filename)
	if !ok {
		return Archive{}, 0, fmt.Errorf("designassets: unsupported download filename %q", filename)
	}
	a := Archive{ID: p.family + "@" + hash, Family: p.family, Filename: filename, SHA256: hash, Size: size, Kind: p.kind, Vendor: "tailwindplus", SourceURL: p.url, Files: []File{}, Projects: []Project{}}
	z, err := boundedZIP(r, size)
	if err != nil {
		return Archive{}, 0, err
	}
	seen, total := pathSet{}, int64(0)
	for _, f := range z.File {
		dir := f.FileInfo().IsDir()
		name := strings.TrimSuffix(f.Name, "/")
		if (!dir && !f.Mode().IsRegular()) || dir != strings.HasSuffix(f.Name, "/") {
			return Archive{}, 0, fmt.Errorf("designassets: non-regular member %q", f.Name)
		}
		if err := seen.add(name, dir); err != nil {
			return Archive{}, 0, err
		}
		if f.UncompressedSize64 > uint64(MaxMemberBytes) || (dir && f.UncompressedSize64 != 0) {
			return Archive{}, 0, fmt.Errorf("designassets: member size exceeds bounds: %q", f.Name)
		}
		if dir {
			// archive/zip's directory reader bypasses CRC and local-header
			// validation. Official download directories carry no data.
			if f.CRC32 != 0 || f.CompressedSize64 != 0 {
				return Archive{}, 0, fmt.Errorf("designassets: directory contains data or invalid CRC")
			}
			offset, err := f.DataOffset()
			if err != nil || offset < 0 || offset > size {
				return Archive{}, 0, fmt.Errorf("designassets: directory local header invalid")
			}
		}
		total += int64(f.UncompressedSize64)
		if total > MaxArchiveExpanded {
			return Archive{}, 0, fmt.Errorf("designassets: expanded archive exceeds bounds")
		}
	}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		isProject := path.Base(f.Name) == "package.json"
		if isProject && (f.UncompressedSize64 > uint64(MaxProjectBytes) || len(a.Projects) >= 128) {
			return Archive{}, 0, fmt.Errorf("designassets: project metadata exceeds bounds")
		}
		rc, err := f.Open()
		if err != nil {
			return Archive{}, 0, err
		}
		h := sha256.New()
		var body bytes.Buffer
		var dst io.Writer = h
		if isProject {
			dst = io.MultiWriter(h, &body)
		}
		n, err := io.Copy(dst, io.LimitReader(contextReader{ctx, rc}, MaxMemberBytes+1))
		closeErr := rc.Close()
		if err != nil {
			return Archive{}, 0, fmt.Errorf("designassets: read %q: %w", f.Name, err)
		}
		if closeErr != nil {
			return Archive{}, 0, closeErr
		}
		if n != int64(f.UncompressedSize64) || n > MaxMemberBytes {
			return Archive{}, 0, fmt.Errorf("designassets: member size mismatch: %q", f.Name)
		}
		a.Files = append(a.Files, File{f.Name, n, hex.EncodeToString(h.Sum(nil)), classify(f.Name)})
		if isProject {
			project, err := parseProject(f.Name, body.Bytes())
			if err != nil {
				return Archive{}, 0, err
			}
			a.Projects = append(a.Projects, project)
		}
	}
	if len(a.Files) == 0 {
		return Archive{}, 0, fmt.Errorf("designassets: archive has no regular files")
	}
	slices.SortFunc(a.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(a.Projects, func(a, b Project) int { return strings.Compare(a.Path, b.Path) })
	return a, total, nil
}

func parseProject(name string, body []byte) (Project, error) {
	var raw struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Dependencies map[string]string `json:"dependencies"`
		Dev          map[string]string `json:"devDependencies"`
		Peer         map[string]string `json:"peerDependencies"`
		Optional     map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Project{}, fmt.Errorf("designassets: invalid project manifest %q: %w", name, err)
	}
	p := Project{name, raw.Name, raw.Version, raw.Dependencies, raw.Dev, raw.Peer, raw.Optional}
	for _, deps := range []*map[string]string{&p.Dependencies, &p.DevDependencies, &p.PeerDependencies, &p.OptionalDependencies} {
		if *deps == nil {
			*deps = map[string]string{}
		}
	}
	return p, validateProject(p)
}

func validateProject(p Project) error {
	if path.Base(p.Path) != "package.json" || len(p.Name) > 512 || len(p.Version) > 512 || len(p.Name+p.Version) != len(strings.TrimSpace(p.Name)+strings.TrimSpace(p.Version)) {
		return fmt.Errorf("designassets: invalid project metadata")
	}
	for _, deps := range []map[string]string{p.Dependencies, p.DevDependencies, p.PeerDependencies, p.OptionalDependencies} {
		if len(deps) > 2048 {
			return fmt.Errorf("designassets: too many dependencies")
		}
		for k, v := range deps {
			if k == "" || len(k) > 512 || len(v) > 2048 {
				return fmt.Errorf("designassets: dependency metadata exceeds bounds")
			}
			if !safeText(k) || !safeText(v) {
				return fmt.Errorf("designassets: sensitive or non-text dependency metadata refused")
			}
			if u, err := url.Parse(v); err == nil && (u.User != nil || u.RawQuery != "") {
				return fmt.Errorf("designassets: credential-bearing dependency URL refused")
			}
		}
	}
	return nil
}

func validateRecords(archives []Archive) error {
	if len(archives) > MaxArchives {
		return fmt.Errorf("designassets: too many archive revisions")
	}
	previous := ""
	for _, a := range archives {
		p, ok := productFor(a.Filename)
		if !ok || a.Family != p.family || a.Kind != p.kind || a.SourceURL != p.url || a.Vendor != "tailwindplus" || a.ID != a.Family+"@"+a.SHA256 || a.ID <= previous {
			return fmt.Errorf("designassets: inconsistent or unordered archive metadata")
		}
		previous = a.ID
		if len(a.Files) == 0 || len(a.Files) > MaxMembers || len(a.Projects) > 128 {
			return fmt.Errorf("designassets: inventory exceeds bounds")
		}
		seen, prior, total := pathSet{}, "", int64(0)
		files := make(map[string]File, len(a.Files))
		for _, f := range a.Files {
			if err := seen.add(f.Path, false); err != nil {
				return err
			}
			if f.Path <= prior || f.Kind != classify(f.Path) {
				return fmt.Errorf("designassets: inconsistent or unordered file inventory")
			}
			prior = f.Path
			total += f.Size
			files[f.Path] = f
		}
		if total > MaxArchiveExpanded {
			return fmt.Errorf("designassets: expanded inventory exceeds bounds")
		}
		prior = ""
		for _, p := range a.Projects {
			f, ok := files[p.Path]
			if !ok || f.Size > MaxProjectBytes || p.Path <= prior {
				return fmt.Errorf("designassets: inconsistent project inventory")
			}
			if err := validateProject(p); err != nil {
				return err
			}
			prior = p.Path
		}
		for _, f := range a.Files {
			if path.Base(f.Path) == "package.json" {
				if _, ok := slices.BinarySearchFunc(a.Projects, f.Path, func(p Project, s string) int { return strings.Compare(p.Path, s) }); !ok {
					return fmt.Errorf("designassets: project metadata missing")
				}
			}
		}
	}
	return nil
}
