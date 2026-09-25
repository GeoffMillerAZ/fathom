package designassets

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type zipMember struct {
	name, body string
	mode       fs.FileMode
}

func zipBytes(t *testing.T, members ...zipMember) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, m := range members {
		h := &zip.FileHeader{Name: m.name, Method: zip.Store}
		if m.mode != 0 {
			h.SetMode(m.mode)
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func writeFixture(t *testing.T, dir, name string, body []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func fixtureMembers() []zipMember {
	return []zipMember{
		{"project/", "", os.ModeDir | 0755},
		{"project/README.md", "Vendor setup guide.\n", 0644},
		{"project/package.json", `{"name":"private-reference","version":"1.2.3","dependencies":{"react":"^19.0.0"},"devDependencies":{"tailwindcss":"^4.0.0"},"scripts":{"dev":"must never execute"}}`, 0644},
		{"project/src/page.tsx", "export default function Page() { return null }\n", 0644},
		{"project/src/theme.css", ":root { --accent: olive; }\n", 0644},
		{"project/public/logo.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`, 0644},
		{"project/public/photo.png", "\x89PNG\x00binary", 0644},
		{"project/LICENSE", "Private vendor license placeholder.\n", 0644},
		{"project/.env.local", "API_TOKEN=not-a-real-token\n", 0600},
	}
}

func importedFixture(t *testing.T) (string, string, *Catalog, []byte) {
	t.Helper()
	src, root := t.TempDir(), filepath.Join(t.TempDir(), "cache")
	body := zipBytes(t, fixtureMembers()...)
	writeFixture(t, src, "studio.zip", body)
	c, err := Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	return src, root, c, body
}

func TestImportPreservesOriginalAndDeterministicRevisions(t *testing.T) {
	src, root, c, original := importedFixture(t)
	if len(c.List()) != 1 {
		t.Fatal("missing imported archive")
	}
	a := c.List()[0]
	if a.ID != "studio@"+digest(original) || a.SHA256 != digest(original) || len(a.Files) != 8 || len(a.Projects) != 1 || a.Projects[0].Dependencies["react"] != "^19.0.0" || a.SourceURL != "https://tailwindcss.com/plus/templates/studio" {
		t.Fatalf("incorrect metadata: %+v", a)
	}
	for _, name := range []string{filepath.Join(src, "studio.zip"), filepath.Join(root, "archives", a.SHA256+".zip")} {
		b, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(b, original) {
			t.Fatalf("original bytes changed: %s %v", name, err)
		}
	}
	before, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(before, []byte(src)) || bytes.Contains(before, []byte(root)) || bytes.Contains(before, []byte("must never execute")) {
		t.Fatal("metadata leaked absolute paths or manifest scripts")
	}
	c2, err := Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "catalog.json"))
	if c2.Revision != c.Revision || !bytes.Equal(before, after) {
		t.Fatal("repeated import is not byte deterministic")
	}
	changed := zipBytes(t, zipMember{"README.md", "new revision", 0644})
	writeFixture(t, src, "studio.zip", changed)
	c3, err := Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c3.List()) != 2 || c3.Revision == c.Revision {
		t.Fatal("new snapshot replaced history or retained stale revision")
	}
	if _, ok := c3.Get(a.ID); !ok {
		t.Fatal("old immutable ID disappeared")
	}
	if _, err := Verify(root, a.ID); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 || entries[0].Name() != "archives" || entries[1].Name() != "catalog.json" {
		t.Fatalf("unexpected extraction/staging files: %v", entries)
	}
}

func TestDetachedMetadataAndBoundedRead(t *testing.T) {
	_, root, c, _ := importedFixture(t)
	a := c.List()[0]
	id := a.ID
	a.Files[0].Path = "changed"
	a.Projects[0].Dependencies["react"] = "changed"
	c.Archives[0].Files[0].Path = "exported changed"
	c.Archives[0].Projects[0].Dependencies["react"] = "exported changed"
	got, ok := c.Get(id)
	if !ok || got.Files[0].Path == "changed" || got.Files[0].Path == "exported changed" || got.Projects[0].Dependencies["react"] != "^19.0.0" {
		t.Fatal("mutable shared metadata escaped")
	}
	text, err := ReadFile(root, id, "project/src/page.tsx")
	if err != nil || text.Content != fixtureMembers()[3].body || text.SHA256 != digest([]byte(text.Content)) {
		t.Fatalf("read failed: %+v %v", text, err)
	}
	for _, name := range []string{"project/.env.local", "project/public/photo.png", "../catalog.json", "/etc/passwd", "project\\README.md", "missing"} {
		if _, err := ReadFile(root, id, name); err == nil {
			t.Errorf("unsafe/unknown read accepted: %s", name)
		}
	}
	if _, err := ReadFile(root, "missing", "README.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown archive error: %v", err)
	}
	if _, err := Verify(root, "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown Verify error: %v", err)
	}
}

func TestImportFailurePreservesPriorCache(t *testing.T) {
	src, root, _, _ := importedFixture(t)
	before, _ := os.ReadFile(filepath.Join(root, "catalog.json"))
	oldFiles, _ := os.ReadDir(filepath.Join(root, "archives"))
	writeFixture(t, src, "catalyst-ui-kit.zip", zipBytes(t, zipMember{"README.md", "valid new archive", 0644}))
	writeFixture(t, src, "syntax.zip", zipBytes(t, zipMember{"../escape", "invalid", 0644}))
	if _, err := Import(context.Background(), src, root); err == nil {
		t.Fatal("invalid batch accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, "catalog.json"))
	newFiles, _ := os.ReadDir(filepath.Join(root, "archives"))
	if !bytes.Equal(before, after) || len(oldFiles) != len(newFiles) {
		t.Fatal("failed batch changed prior cache")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatal("failed batch left stage or lock")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Import(ctx, src, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestRejectUnsafeZIPMembers(t *testing.T) {
	cases := map[string][]zipMember{
		"traversal": {{"../outside", "x", 0644}}, "absolute": {{"/outside", "x", 0644}},
		"windows": {{"C:/outside", "x", 0644}}, "backslash": {{"a\\outside", "x", 0644}},
		"nul": {{"a\x00b", "x", 0644}}, "control": {{"a\nb", "x", 0644}},
		"double-slash": {{"a//b", "x", 0644}}, "dot": {{"a/./b", "x", 0644}},
		"symlink": {{"link", "/outside", os.ModeSymlink | 0777}}, "device": {{"device", "", os.ModeDevice | 0600}},
		"duplicate":     {{"a", "one", 0644}, {"a", "two", 0644}},
		"case-fold":     {{"A/page.tsx", "one", 0644}, {"a/other.tsx", "two", 0644}},
		"unicode-fold":  {{"σ.tsx", "one", 0644}, {"ς.tsx", "two", 0644}},
		"file-parent":   {{"a", "one", 0644}, {"a/b", "two", 0644}},
		"parent-file":   {{"a/b", "one", 0644}, {"a", "two", 0644}},
		"duplicate-dir": {{"a/", "", os.ModeDir | 0755}, {"a/", "", os.ModeDir | 0755}},
		"long":          {{strings.Repeat("a", MaxPathBytes+1), "x", 0644}},
		"empty":         {},
	}
	for name, members := range cases {
		t.Run(name, func(t *testing.T) {
			src, root := t.TempDir(), t.TempDir()
			writeFixture(t, src, "studio.zip", zipBytes(t, members...))
			if _, err := Import(context.Background(), src, root); err == nil {
				t.Fatal("unsafe ZIP accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "catalog.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("unsafe import published catalog")
			}
		})
	}
}

func TestZIPCRCAndResourceBounds(t *testing.T) {
	base := zipBytes(t, zipMember{"README.md", "unique stored payload", 0644})
	badCRC := bytes.Clone(base)
	offset := bytes.Index(badCRC, []byte("unique stored payload"))
	badCRC[offset] ^= 1
	tooLargeMember := bytes.Clone(base)
	cd := bytes.Index(tooLargeMember, []byte{'P', 'K', 1, 2})
	binary.LittleEndian.PutUint32(tooLargeMember[cd+24:], uint32(MaxMemberBytes+1))
	tooMany := bytes.Clone(base)
	end := len(tooMany) - 22
	binary.LittleEndian.PutUint16(tooMany[end+8:], MaxMembers+1)
	binary.LittleEndian.PutUint16(tooMany[end+10:], MaxMembers+1)
	zip64 := bytes.Clone(base)
	binary.LittleEndian.PutUint16(zip64[end+8:], 65535)
	binary.LittleEndian.PutUint16(zip64[end+10:], 65535)
	for name, body := range map[string][]byte{"crc": badCRC, "size": tooLargeMember, "count": tooMany, "zip64": zip64, "notzip": []byte("not a zip")} {
		t.Run(name, func(t *testing.T) {
			src, root := t.TempDir(), t.TempDir()
			writeFixture(t, src, "studio.zip", body)
			if _, err := Import(context.Background(), src, root); err == nil {
				t.Fatal("invalid ZIP accepted")
			}
		})
	}
	src, root := t.TempDir(), t.TempDir()
	f, err := os.Create(filepath.Join(src, "studio.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxArchiveBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Import(context.Background(), src, root); err == nil {
		t.Fatal("oversized sparse archive accepted")
	}
}

func TestReadRefusesTamperingAndUnsafeText(t *testing.T) {
	_, root, c, _ := importedFixture(t)
	a := c.List()[0]
	name := filepath.Join(root, "archives", a.SHA256+".zip")
	body, _ := os.ReadFile(name)
	body[len(body)/2] ^= 1
	writeFixture(t, filepath.Dir(name), filepath.Base(name), body)
	if _, err := Verify(root, a.ID); err == nil {
		t.Fatal("Verify accepted changed original")
	}
	if _, err := ReadFile(root, a.ID, "project/README.md"); err == nil {
		t.Fatal("read accepted changed original")
	}
	if _, err := Load(root); err != nil {
		t.Fatalf("metadata load unexpectedly claims full ZIP verification: %v", err)
	}
	src, root := t.TempDir(), t.TempDir()
	writeFixture(t, src, "studio.zip", zipBytes(t, zipMember{"big.txt", strings.Repeat("x", int(MaxTextBytes)+1), 0644}, zipMember{"key.txt", "-----BEGIN RSA PRIVATE KEY-----", 0644}, zipMember{"binary.txt", "a\x00b", 0644}))
	c, err := Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"big.txt", "key.txt", "binary.txt"} {
		if _, err := ReadFile(root, c.List()[0].ID, name); err == nil {
			t.Errorf("unsafe text accepted: %s", name)
		}
	}
}

func TestManifestValidationAndMemberIntegrity(t *testing.T) {
	_, root, c, _ := importedFixture(t)
	valid, _ := os.ReadFile(filepath.Join(root, "catalog.json"))
	var doc map[string]any
	if err := json.Unmarshal(valid, &doc); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"revision", "schema_version", "archives"} {
		copy := map[string]any{}
		for k, v := range doc {
			copy[k] = v
		}
		delete(copy, field)
		body, _ := json.Marshal(copy)
		if _, err := decodeCatalog(body); err == nil {
			t.Errorf("missing %s accepted", field)
		}
	}
	bad := bytes.Replace(valid, []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1)
	if _, err := decodeCatalog(bad); err == nil {
		t.Fatal("unknown schema version accepted")
	}
	bad = bytes.Replace(valid, []byte(c.Revision), []byte(strings.Repeat("0", 64)), 1)
	if _, err := decodeCatalog(bad); err == nil {
		t.Fatal("stale revision accepted")
	}
	archives := c.List()
	archives[0].Projects[0].Dependencies["react"] = "^999.0.0"
	forged, err := encodeCatalog(archives)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "catalog.json", forged)
	if _, err := Verify(root, archives[0].ID); err == nil {
		t.Fatal("Verify accepted metadata detached from original manifest")
	}
	archives = c.List()
	archives[0].Files[0].SHA256 = strings.Repeat("0", 64)
	body, err := encodeCatalog(archives)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "catalog.json", body)
	if _, err := ReadFile(root, archives[0].ID, archives[0].Files[0].Path); err == nil {
		t.Fatal("member metadata tampering accepted")
	}
	f, err := os.Create(filepath.Join(root, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxCatalogBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Load(root); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

func TestCacheAndInputSymlinksAndLockRefused(t *testing.T) {
	src, root, c, _ := importedFixture(t)
	writeFixture(t, root, ".import.lock", []byte("existing lock"))
	if _, err := Import(context.Background(), src, root); err == nil {
		t.Fatal("concurrent lock ignored")
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".import.lock")); string(b) != "existing lock" {
		t.Fatal("other import lock removed")
	}
	for _, target := range []string{"root", "root-slash", "root-dot", "catalog", "archives", "archive", "input"} {
		t.Run(target, func(t *testing.T) {
			src, root, c, _ := importedFixture(t)
			outside := t.TempDir()
			switch target {
			case "root", "root-slash", "root-dot":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
				if target == "root-slash" {
					root += "/"
				}
				if target == "root-dot" {
					root += "/."
				}
			case "catalog":
				original, _ := os.ReadFile(filepath.Join(root, "catalog.json"))
				writeFixture(t, outside, "catalog.json", original)
				if err := os.Remove(filepath.Join(root, "catalog.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "catalog.json"), filepath.Join(root, "catalog.json")); err != nil {
					t.Fatal(err)
				}
			case "archives":
				if err := os.Rename(filepath.Join(root, "archives"), filepath.Join(outside, "archives")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "archives"), filepath.Join(root, "archives")); err != nil {
					t.Fatal(err)
				}
			case "archive":
				name := c.List()[0].SHA256 + ".zip"
				if err := os.Rename(filepath.Join(root, "archives", name), filepath.Join(outside, name)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, name), filepath.Join(root, "archives", name)); err != nil {
					t.Fatal(err)
				}
			case "input":
				if err := os.Rename(filepath.Join(src, "studio.zip"), filepath.Join(outside, "studio.zip")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "studio.zip"), filepath.Join(src, "studio.zip")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Import(context.Background(), src, root); err == nil {
				t.Fatal("symlink accepted")
			}
			if target != "input" {
				if _, err := Verify(root, c.List()[0].ID); err == nil {
					t.Fatal("exact verification followed symlink")
				}
			}
		})
	}
	if c.List()[0].ID == "" {
		t.Fatal("fixture missing")
	}
}

func TestProductMappingAndProjectMetadata(t *testing.T) {
	names := []string{"syntax", "protocol", "pocket", "primer", "commit", "studio", "radiant", "keynote", "compass", "transmit", "salient", "spotlight", "oatmeal-olive-instrument", "catalyst-ui-kit", "marketing-v4", "application-ui-v4", "ecommerce-v4"}
	src, root := t.TempDir(), t.TempDir()
	for _, name := range names {
		writeFixture(t, src, name+".zip", zipBytes(t, zipMember{"README.md", "reference for " + name, 0644}))
	}
	c, err := Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.List()) != 17 {
		t.Fatalf("got %d official mappings", len(c.List()))
	}
	writeFixture(t, src, "unknown.zip", zipBytes(t, zipMember{"README.md", "not mapped", 0644}))
	if _, err := Import(context.Background(), src, root); err == nil {
		t.Fatal("unknown filename accepted")
	}
	for _, body := range []string{`{"dependencies":{"pkg":"https://user:password@example.com/pkg"}}`, `{"dependencies":{"pkg":123}}`, `{"scripts":`} {
		if _, err := parseProject("package.json", []byte(body)); err == nil {
			t.Errorf("bad project metadata accepted: %s", body)
		}
	}
	p, err := parseProject("package.json", []byte(`{"private":true}`))
	if err != nil || !reflect.DeepEqual(p.Dependencies, map[string]string{}) {
		t.Fatalf("private unnamed project unsupported: %+v %v", p, err)
	}
}
