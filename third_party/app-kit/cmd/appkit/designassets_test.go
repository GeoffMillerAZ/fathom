package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

// These tests deliberately do not run in parallel: dataCmd writes os.Stdout.
// Drain the pipe concurrently so a bounded source response cannot deadlock on
// the pipe buffer, and restore stdout even if the command panics.
func captureAssetsCLI(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		writer.Close()
	}()
	type readResult struct {
		body []byte
		err  error
	}
	done := make(chan readResult, 1)
	go func() {
		body, err := io.ReadAll(reader)
		done <- readResult{body, err}
	}()
	commandErr := dataCmd(context.Background(), args)
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	return result.body, commandErr
}

func assetsCLIJSON(t *testing.T, target any, args ...string) []byte {
	t.Helper()
	body, err := captureAssetsCLI(t, args...)
	if err != nil {
		t.Fatalf("data %v: %v; stdout=%q", args, err, body)
	}
	// Unmarshal rejects banners, progress logs, and multiple JSON documents.
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("data %v did not emit exactly one JSON value: %v; stdout=%q", args, err, body)
	}
	return body
}

func assetsCLIDigest(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

type assetsCLIFixture struct {
	source, root, revision string
	ids                    map[string]string
	originals              map[string][]byte
	members                map[string][]byte
}

type assetsCLIImportResult struct {
	Revision string `json:"revision"`
	Archives int    `json:"archives"`
	Root     string `json:"root"`
}

func newAssetsCLIFixture(t *testing.T) assetsCLIFixture {
	t.Helper()
	w := assetsCLIFixture{
		source: t.TempDir(), root: filepath.Join(t.TempDir(), "private-cache"),
		ids: make(map[string]string), originals: make(map[string][]byte),
		members: map[string][]byte{
			"project/README.md":        []byte("Synthetic CLI reference; never a vendor download.\n"),
			"project/src/page.tsx":     []byte("// ONLY_IN_SOURCE_body_token café <>& \u2028\r\nexport const Page = () => \"</script>\";\r\n\t// trailing whitespace \t\n" + strings.Repeat("// bounded source padding\n", 4096)),
			"project/src/sidebar.tsx":  []byte("export const Sidebar = () => null;\n"),
			"project/styles/theme.css": []byte(":root { --accent: olive; }\n"),
		},
	}
	for _, fixture := range []struct{ filename, family string }{
		{"catalyst-ui-kit.zip", "catalyst"}, {"marketing-v4.zip", "marketing"}, {"studio.zip", "studio"},
	} {
		members := make(map[string][]byte)
		for path, body := range w.members {
			members[path] = body
		}
		members["project/package.json"] = fmt.Appendf(nil, `{"name":%q,"version":"1.2.3","dependencies":{"react":"^19.0.0"},"devDependencies":{"tailwindcss":"^4.0.0"},"scripts":{"dev":"must never execute"}}`, fixture.family+"-fixture")
		paths := make([]string, 0, len(members))
		for path := range members {
			paths = append(paths, path)
		}
		slices.Sort(paths)
		var body bytes.Buffer
		z := zip.NewWriter(&body)
		for _, path := range paths {
			header := &zip.FileHeader{Name: path, Method: zip.Store}
			header.SetMode(0o644)
			member, err := z.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := member.Write(members[path]); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		w.originals[fixture.filename] = bytes.Clone(body.Bytes())
		w.ids[fixture.family] = fixture.family + "@" + assetsCLIDigest(body.Bytes())
		if err := os.WriteFile(filepath.Join(w.source, fixture.filename), body.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var imported assetsCLIImportResult
	assetsCLIJSON(t, &imported, "assets-import", "-src", w.source, "-root", w.root)
	if imported.Archives != 3 || imported.Root != w.root || len(imported.Revision) != 64 {
		t.Fatalf("incorrect import result: %+v", imported)
	}
	w.revision = imported.Revision
	return w
}

func assertAssetsMetadataOnly(t *testing.T, body []byte) {
	t.Helper()
	for _, forbidden := range []string{`"content":`, `"files":`, `"scripts":`, "ONLY_IN_SOURCE_body_token", "must never execute"} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Errorf("metadata response exposed %q", forbidden)
		}
	}
}

func TestDesignAssetsCLIJSONRoundTrip(t *testing.T) {
	w := newAssetsCLIFixture(t)
	var families []string
	cursor := ""
	for pageNumber := 0; pageNumber < 3; pageNumber++ {
		var page designassets.Page
		body := assetsCLIJSON(t, &page, "assets-list", "-root", w.root, "-limit", "1", "-cursor", cursor)
		assertAssetsMetadataOnly(t, body)
		if page.Revision != w.revision || page.Total != 3 || len(page.Items) != 1 {
			t.Fatalf("incorrect archive page: %+v", page)
		}
		asset := page.Items[0]
		if asset.ID != w.ids[asset.Family] || asset.Vendor != "tailwindplus" || asset.FileCount != 5 || asset.Projects != 1 || asset.SHA256 != assetsCLIDigest(w.originals[asset.Filename]) || asset.Size != int64(len(w.originals[asset.Filename])) {
			t.Fatalf("incorrect archive summary: %+v", asset)
		}
		if asset.Kind != map[string]string{"catalyst": "ui-kit", "marketing": "ui-blocks", "studio": "template"}[asset.Family] || asset.SourceURL == "" {
			t.Fatalf("archive summary lost its kind or source reference: %+v", asset)
		}
		families = append(families, asset.Family)
		if (page.NextCursor == "") != (pageNumber == 2) {
			t.Fatalf("incorrect continuation on archive page %d", pageNumber)
		}
		cursor = page.NextCursor
	}
	if !slices.Equal(families, []string{"catalyst", "marketing", "studio"}) {
		t.Fatalf("archive pages lost, duplicated, or reordered results: %v", families)
	}
	for _, filter := range []struct {
		args   []string
		family string
	}{
		{[]string{"-kind", "ui-kit"}, "catalyst"},
		{[]string{"-kind", "ui-blocks"}, "marketing"},
		{[]string{"-kind", "template", "-family", "studio", "-query", "TAILWINDPLUS PROJECT/src"}, "studio"},
		{[]string{"-query", "ONLY_IN_SOURCE_body_token"}, ""},
	} {
		var page designassets.Page
		assetsCLIJSON(t, &page, append([]string{"assets-list", "-root", w.root}, filter.args...)...)
		if filter.family == "" {
			if page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
				t.Fatalf("source-body search must return an empty JSON array: %+v", page)
			}
		} else if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Family != filter.family {
			t.Fatalf("filter %v returned %+v", filter.args, page)
		}
	}

	id := w.ids["studio"]
	cursor = ""
	for index, path := range []string{"project/src/page.tsx", "project/src/sidebar.tsx"} {
		var page designassets.FilePage
		body := assetsCLIJSON(t, &page, "assets-files", "-root", w.root, "-id", id, "-prefix", "project/src/", "-kind", "source", "-limit", "1", "-cursor", cursor)
		assertAssetsMetadataOnly(t, body)
		if page.Revision != w.revision || page.ArchiveID != id || page.Total != 2 || len(page.Items) != 1 {
			t.Fatalf("incorrect member page: %+v", page)
		}
		file := page.Items[0]
		if file.Path != path || file.Kind != "source" || file.Size != int64(len(w.members[path])) || file.SHA256 != assetsCLIDigest(w.members[path]) || (page.NextCursor == "") != (index == 1) {
			t.Fatalf("incorrect member metadata/pagination: %+v", page)
		}
		cursor = page.NextCursor
	}
	var metadata struct {
		Asset    designassets.Summary   `json:"asset"`
		Projects []designassets.Project `json:"projects"`
	}
	body := assetsCLIJSON(t, &metadata, "assets-get", "-root", w.root, "-id", id)
	assertAssetsMetadataOnly(t, body)
	if metadata.Asset.ID != id || len(metadata.Projects) != 1 || metadata.Projects[0].Name != "studio-fixture" || metadata.Projects[0].Dependencies["react"] != "^19.0.0" || metadata.Projects[0].DevDependencies["tailwindcss"] != "^4.0.0" {
		t.Fatalf("incorrect verified project metadata: %+v", metadata)
	}
	var source designassets.TextFile
	path := "project/src/page.tsx"
	assetsCLIJSON(t, &source, "assets-get", "-root", w.root, "-id", id, "-path", path)
	if source.ArchiveID != id || source.Path != path || !bytes.Equal([]byte(source.Content), w.members[path]) || source.Size != int64(len(w.members[path])) || source.SHA256 != assetsCLIDigest(w.members[path]) {
		t.Fatalf("source bytes/metadata did not round-trip exactly: id=%q path=%q size=%d sha256=%q content_bytes=%d", source.ArchiveID, source.Path, source.Size, source.SHA256, len(source.Content))
	}
	for filename, original := range w.originals {
		for _, path := range []string{filepath.Join(w.source, filename), filepath.Join(w.root, "archives", assetsCLIDigest(original)+".zip")} {
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("CLI altered original archive %s: %v", path, err)
			}
		}
	}
}

func TestDesignAssetsCLIRejectsInvalidRequestsWithoutStdout(t *testing.T) {
	w := newAssetsCLIFixture(t)
	id := w.ids["studio"]
	missing := filepath.Join(t.TempDir(), "missing-cache")
	var archives designassets.Page
	assetsCLIJSON(t, &archives, "assets-list", "-root", w.root, "-limit", "1")
	var files designassets.FilePage
	assetsCLIJSON(t, &files, "assets-files", "-root", w.root, "-id", id, "-limit", "1")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing-command", nil, "usage:"},
		{"missing-source", []string{"assets-import"}, "usage:"},
		{"missing-files-id", []string{"assets-files"}, "usage:"},
		{"missing-get-id", []string{"assets-get"}, "usage:"},
		{"extra-import", []string{"assets-import", "-src", w.source, "-root", w.root, "extra"}, "usage:"},
		{"extra-list", []string{"assets-list", "-root", w.root, "extra"}, "accepts flags only"},
		{"extra-files", []string{"assets-files", "-root", w.root, "-id", id, "extra"}, "usage:"},
		{"extra-get", []string{"assets-get", "-root", w.root, "-id", id, "extra"}, "usage:"},
		{"missing-import-flag-value", []string{"assets-import", "-src"}, "flag needs an argument"},
		{"missing-list-flag-value", []string{"assets-list", "-root"}, "flag needs an argument"},
		{"missing-files-flag-value", []string{"assets-files", "-id"}, "flag needs an argument"},
		{"missing-get-flag-value", []string{"assets-get", "-path"}, "flag needs an argument"},
		{"empty-import-root", []string{"assets-import", "-src", w.source, "-root", ""}, "cache root is required"},
		{"empty-list-root", []string{"assets-list", "-root", ""}, "cache root is required"},
		{"empty-files-root", []string{"assets-files", "-root", "", "-id", id}, "cache root is required"},
		{"empty-get-root", []string{"assets-get", "-root", "", "-id", id}, "cache root is required"},
		{"absent-source", []string{"assets-import", "-src", missing, "-root", w.root}, missing},
		{"absent-list-root", []string{"assets-list", "-root", missing}, missing},
		{"absent-files-root", []string{"assets-files", "-root", missing, "-id", id}, missing},
		{"absent-get-root", []string{"assets-get", "-root", missing, "-id", id}, missing},
		{"bad-list-cursor", []string{"assets-list", "-root", w.root, "-cursor", "bad"}, "cursor"},
		{"bad-files-cursor", []string{"assets-files", "-root", w.root, "-id", id, "-cursor", "bad"}, "cursor"},
		{"changed-list-query", []string{"assets-list", "-root", w.root, "-limit", "1", "-kind", "template", "-cursor", archives.NextCursor}, "cursor"},
		{"changed-files-query", []string{"assets-files", "-root", w.root, "-id", w.ids["catalyst"], "-limit", "1", "-cursor", files.NextCursor}, "cursor"},
		{"unknown-files-id", []string{"assets-files", "-root", w.root, "-id", "missing"}, "unknown design asset"},
		{"unknown-get-id", []string{"assets-get", "-root", w.root, "-id", "missing"}, "unknown archive"},
		{"unknown-get-member", []string{"assets-get", "-root", w.root, "-id", id, "-path", "missing.tsx"}, "unknown member"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body, err := captureAssetsCLI(t, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want actionable %q", err, test.want)
			}
			if len(body) != 0 {
				t.Errorf("failed command polluted stdout: %q", body)
			}
		})
	}
}

func TestDesignAssetsCLIDefaultRootIsRelativeToWorkingDirectory(t *testing.T) {
	w := newAssetsCLIFixture(t)
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"assets-list"}, {"assets-files", "-id", w.ids["studio"]}, {"assets-get", "-id", w.ids["studio"]}} {
		body, err := captureAssetsCLI(t, args...)
		if err == nil || !strings.Contains(err.Error(), defaultDesignAssetsRoot) || len(body) != 0 {
			t.Fatalf("missing default cache did not fail clearly: stdout=%q error=%v", body, err)
		}
	}
	var imported assetsCLIImportResult
	assetsCLIJSON(t, &imported, "assets-import", "-src", w.source)
	if imported.Root != defaultDesignAssetsRoot || imported.Revision != w.revision || imported.Archives != 3 {
		t.Fatalf("incorrect default-root import: %+v", imported)
	}
	var page designassets.Page
	assetsCLIJSON(t, &page, "assets-list")
	if page.Total != 3 || page.Revision != imported.Revision {
		t.Fatalf("listing did not use the imported default root: %+v", page)
	}
	var files designassets.FilePage
	assetsCLIJSON(t, &files, "assets-files", "-id", w.ids["studio"])
	if files.Total != 5 || files.Revision != imported.Revision {
		t.Fatalf("member listing did not use the imported default root: %+v", files)
	}
	var source designassets.TextFile
	assetsCLIJSON(t, &source, "assets-get", "-id", w.ids["studio"], "-path", "project/src/page.tsx")
	if source.ArchiveID != w.ids["studio"] || source.Path != "project/src/page.tsx" || !bytes.Equal([]byte(source.Content), w.members["project/src/page.tsx"]) {
		t.Fatal("source retrieval did not use the imported default root")
	}
}
