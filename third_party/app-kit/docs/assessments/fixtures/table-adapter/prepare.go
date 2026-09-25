// Explicit source-native P04 preparation. No Gallery hook or automatic paid read.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"github.com/geoffmilleraz/app-kit/internal/tailwind"
)

const tableArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const tableMember = "html/lists/tables/07-with-stacked-columns-on-mobile.html"
const tableSHA = "5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288"
const tableFontRoot = "data/design-assets/reference-assets/inter/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441"
const tableFontSHA = "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"
const tableLicenseSHA = "262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a"
const tableMaxDocument = 1 << 20
const tableCSSInput = `@import "tailwindcss" source(none);
@source "./source.html";
@custom-variant dark (&:where(.dark, .dark *));
@theme { --font-sans: "InterVariable", sans-serif; }
:root { color-scheme: light; }
:root.dark { color-scheme: dark; }
body { margin: 0; background: #fff; }
.dark body { background: #111827; }
`

type tableFileIdentity struct {
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}
type tableReference struct {
	Schema          int                          `json:"schema"`
	CaseID          string                       `json:"case_id"`
	Source          designconvert.Source         `json:"source"`
	Documents       map[string]tableFileIdentity `json:"documents"`
	CSSSHA256       string                       `json:"css_sha256"` // Compiled Tailwind CSS, excluding the pinned font/license prefix.
	CSSInputSHA256  string                       `json:"css_input_sha256"`
	FontSHA256      string                       `json:"font_sha256"`
	LicenseSHA256   string                       `json:"license_sha256"`
	TailwindVersion string                       `json:"tailwind_version"`
	TailwindSHA256  string                       `json:"tailwind_sha256"`
	Scope           string                       `json:"scope"`
	Verification    string                       `json:"verification"`
}

func main() {
	root := flag.String("kit-root", "", "absolute existing private app-kit checkout")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "P04 preparation refused: arguments")
		os.Exit(2)
	}
	dir, err := prepareTable(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, tableFailureMessage(dir, err))
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"path": dir, "case_id": "P04-table", "verification": "not-verified"})
}

func tableDigest(b []byte) string    { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func tableRefuse(stage string) error { return fmt.Errorf("P04 preparation refused: %s", stage) }
func tableFailureMessage(dir string, err error) string {
	if dir == "" {
		return err.Error()
	}
	return fmt.Sprintf("%v; partial=%s", err, dir)
}

func prepareTable(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", tableRefuse("absolute-root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", tableRefuse("root")
	}
	root = resolved
	assets := filepath.Join(root, "data/design-assets")
	file, err := designassets.ReadFile(assets, tableArchive, tableMember)
	if err != nil || file.SHA256 != tableSHA || tableDigest([]byte(file.Content)) != tableSHA {
		return "", tableRefuse("frozen-source")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	font, err := tableReadFile(filepath.Join(root, tableFontRoot, "font-files/InterVariable.woff2"), 352240)
	if err != nil || len(font) != 352240 || tableDigest(font) != tableFontSHA {
		return "", tableRefuse("font")
	}
	license, err := tableReadFile(filepath.Join(root, tableFontRoot, "LICENSE.txt"), 4380)
	if err != nil || len(license) != 4380 || tableDigest(license) != tableLicenseSHA {
		return "", tableRefuse("font-license")
	}
	// Every run owns a new directory. Partial failure remains inspectable; no
	// existing private file is overwritten or automatically removed.
	dir, err := os.MkdirTemp("", "appkit-p04-reference-")
	if err != nil {
		return "", tableRefuse("scratch")
	}
	bin, pin, err := tableCachedTailwind(dir)
	if err != nil {
		return dir, err
	}
	write := func(name string, b []byte) error {
		f, e := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return tableRefuse("exclusive-output")
		}
		_, e = f.Write(b)
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			return tableRefuse("output-write")
		}
		return nil
	}
	if write("source.html", []byte(file.Content)) != nil || write("input.css", []byte(tableCSSInput)) != nil {
		return dir, tableRefuse("css-input")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var css []byte
	for _, out := range []string{"one.css", "two.css"} {
		cmd := exec.CommandContext(ctx, bin, "-i", "input.css", "-o", out, "--minify")
		cmd.Dir = dir
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		for _, item := range os.Environ() {
			key, _, _ := strings.Cut(item, "=")
			if key != "NODE_OPTIONS" && key != "NODE_PATH" && key != "TMPDIR" {
				cmd.Env = append(cmd.Env, item)
			}
		}
		cmd.Env = append(cmd.Env, "NODE_OPTIONS=", "NODE_PATH=", "TMPDIR="+dir)
		if cmd.Run() != nil {
			return dir, tableRefuse("cached-css-build")
		}
		b, e := tableReadFile(filepath.Join(dir, out), tableMaxDocument)
		if e != nil || tableSafeCSS(b) != nil {
			return dir, tableRefuse("css-output")
		}
		if css == nil {
			css = b
		} else if !bytes.Equal(css, b) {
			return dir, tableRefuse("css-replay")
		}
	}
	style := "/* Inter v4.1 SIL Open Font License\n" + string(license) + "\n*/\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(data:font/woff2;base64," + base64.StdEncoding.EncodeToString(font) + ") format('woff2')}\n" + string(css)
	ref := tableReference{Schema: 1, CaseID: "P04-table", Source: source, Documents: map[string]tableFileIdentity{}, CSSSHA256: tableDigest(css), CSSInputSHA256: tableDigest([]byte(tableCSSInput)), FontSHA256: tableFontSHA, LicenseSHA256: tableLicenseSHA, TailwindVersion: "v4.3.0", TailwindSHA256: pin, Scope: "controlled-source-reference", Verification: "not-verified"}
	for _, theme := range []string{"light", "dark"} {
		body, e := tableDocument(file.Content, style, theme)
		if e != nil {
			return dir, e
		}
		name := "source-" + theme + ".html"
		if e = write(name, body); e != nil {
			return dir, e
		}
		ref.Documents[name] = tableFileIdentity{SHA256: tableDigest(body), Size: len(body)}
	}
	// Recheck the authoritative source after preparation, before the final record.
	again, e := designassets.ReadFile(assets, tableArchive, tableMember)
	if e != nil || again.Content != file.Content || again.SHA256 != file.SHA256 {
		return dir, tableRefuse("source-changed")
	}
	data, e := json.MarshalIndent(ref, "", "  ")
	if e != nil {
		return dir, tableRefuse("reference-encoding")
	}
	if e = write("reference.json", append(data, '\n')); e != nil {
		return dir, e
	}
	return dir, nil
}

func tableSafeCSS(b []byte) error {
	if len(b) == 0 || len(b) > tableMaxDocument {
		return tableRefuse("css-size")
	}
	for _, s := range []string{"url(", "@import", "<style", "</style", "@source"} {
		if strings.Contains(strings.ToLower(string(b)), s) {
			return tableRefuse("css-resource")
		}
	}
	return nil
}
func tableDocument(body, style, theme string) ([]byte, error) {
	return tableDocumentForSource(body, style, theme, tableSHA)
}

func tableDocumentForSource(body, style, theme, sourceSHA string) ([]byte, error) {
	if theme != "light" && theme != "dark" {
		return nil, tableRefuse("theme")
	}
	if tableDigest([]byte(body)) != sourceSHA {
		return nil, tableRefuse("document-source")
	}
	lowerStyle := strings.ToLower(style)
	if strings.Contains(lowerStyle, "<style") || strings.Contains(lowerStyle, "</style") {
		return nil, tableRefuse("style-boundary")
	}
	doc := []byte("<!doctype html>\n<html lang=\"en\" class=\"" + theme + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>P04 table: controlled source reference</title><style>" + style + "</style></head><body>" + body + "</body></html>\n")
	if len(doc) > tableMaxDocument {
		return nil, tableRefuse("document-bound")
	}
	return doc, nil
}
func tableReadFile(path string, limit int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, tableRefuse("regular-input")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, tableRefuse("input-open")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || info.Mode() != opened.Mode() || info.Size() != opened.Size() {
		return nil, tableRefuse("input-changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(b) > limit || int64(len(b)) != info.Size() {
		return nil, tableRefuse("input-bound")
	}
	final, err := f.Stat()
	if err != nil || !os.SameFile(info, final) || info.Mode() != final.Mode() || info.Size() != final.Size() {
		return nil, tableRefuse("input-changed")
	}
	return b, nil
}
func tableCachedTailwind(dir string) (string, string, error) {
	m, err := tailwind.LoadManifest()
	if err != nil || m.Version != "v4.3.0" {
		return "", "", tableRefuse("tailwind-pin")
	}
	pin, ok := m.Binaries[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return "", "", tableRefuse("tailwind-platform")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", tableRefuse("cache")
	}
	path := filepath.Join(cache, "app-kit/tailwindcss", m.Version, pin.Asset)
	if _, err = os.Lstat(path); os.IsNotExist(err) {
		return "", "", tableRefuse("missing-cached-tailwind")
	}
	snapshot := filepath.Join(dir, "tailwindcss")
	if err = tableSnapshotExecutable(path, snapshot, pin.SHA256, 512<<20); err != nil {
		return "", "", err
	}
	return snapshot, pin.SHA256, nil
}

func tableSnapshotExecutable(source, destination, expectedSHA string, limit int64) error {
	info, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return tableRefuse("missing-cached-tailwind")
		}
		return tableRefuse("cached-tailwind-integrity")
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit || info.Mode().Perm()&0111 == 0 {
		return tableRefuse("cached-tailwind-integrity")
	}
	in, err := os.Open(source)
	if err != nil {
		return tableRefuse("cached-tailwind-integrity")
	}
	defer in.Close()
	opened, err := in.Stat()
	if err != nil || !os.SameFile(info, opened) || info.Mode() != opened.Mode() || info.Size() != opened.Size() {
		return tableRefuse("cached-tailwind-integrity")
	}
	// Keep unverified partial bytes non-executable. Promote the owned snapshot
	// only after its complete streamed digest matches the manifest pin.
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return tableRefuse("tailwind-snapshot")
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, limit+1))
	closeErr := out.Close()
	final, statErr := in.Stat()
	if copyErr != nil || closeErr != nil || statErr != nil || written != info.Size() || written > limit ||
		!os.SameFile(info, final) || info.Mode() != final.Mode() || info.Size() != final.Size() {
		return tableRefuse("tailwind-snapshot")
	}
	if hex.EncodeToString(hash.Sum(nil)) != expectedSHA {
		return tableRefuse("cached-tailwind-integrity")
	}
	if err = os.Chmod(destination, 0700); err != nil {
		return tableRefuse("tailwind-snapshot")
	}
	snapshot, err := os.Lstat(destination)
	if err != nil || !snapshot.Mode().IsRegular() || snapshot.Mode().Perm() != 0700 || snapshot.Size() != written {
		return tableRefuse("tailwind-snapshot")
	}
	return nil
}
