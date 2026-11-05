package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

// writeExport writes a generated export of a made-up provider; none is a committed provider export.
func writeExport(t *testing.T, dir, source, version string, attrs ...string) {
	t.Helper()
	as := map[string]any{}
	for _, a := range attrs {
		as[a] = map[string]any{"type": "string", "optional": true, "description": "drop"}
	}
	schema := map[string]any{"version": 0, "block": map[string]any{"attributes": as}}
	b, err := json.Marshal(map[string]any{"format_version": "1.0", "provider_schemas": map[string]any{
		"registry.opentofu.org/" + source: map[string]any{
			"provider":            schema,
			"resource_schemas":    map[string]any{"widget_thing": schema, "widget_other": map[string]any{"version": 1, "block": map[string]any{}}},
			"data_source_schemas": map[string]any{"widget_thing": schema},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	name := strings.ReplaceAll(source, "/", "-") + "_" + version + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testLine() pinLine {
	return pinLine{
		Line: 1, Source: "example/widget", Registry: "registry.opentofu.org", Platform: "linux_amd64", Licence: "MPL-2.0",
		Releases: []pinRelease{
			{Version: "1.0.0", SHA256: strings.Repeat("a", 64)},
			{Version: "1.1.0", SHA256: strings.Repeat("b", 64)},
		},
	}
}

func fixtureExports(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeExport(t, dir, "example/widget", "1.0.0", "name")
	writeExport(t, dir, "example/widget", "1.1.0", "name", "tags")
	return dir
}

// TestBuildCatalogTwoBuildsAreByteIdentical is IF-4: a catalog and its receipt are the same bytes on a second build,
// from a different scratch directory.
func TestBuildCatalogTwoBuildsAreByteIdentical(t *testing.T) {
	ex := dirExporter{dir: fixtureExports(t), tool: "opentofu test"}
	var cat, rcpt [2][]byte
	for i := range cat {
		out := t.TempDir()
		rec, err := buildCatalog(context.Background(), testLine(), ex, out, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if cat[i], err = os.ReadFile(filepath.Join(out, rec.Catalog)); err != nil {
			t.Fatal(err)
		}
		if rcpt[i], err = os.ReadFile(filepath.Join(out, rec.Catalog+".receipt.json")); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(cat[0], cat[1]) || !bytes.Equal(rcpt[0], rcpt[1]) {
		t.Fatal("two builds of the same pins and exports differ")
	}
}

func TestBuildCatalogReceiptsTheBuild(t *testing.T) {
	out := t.TempDir()
	rec, err := buildCatalog(context.Background(), testLine(), dirExporter{dir: fixtureExports(t), tool: "opentofu test"}, out, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, rec.Catalog)
	m, err := iacfacts.VerifyIaCCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Replacement != iacfacts.ReplacementAbsent || rec.Replacement != iacfacts.ReplacementAbsent || !strings.Contains(rec.ReplacementNote, "IF-13") {
		t.Fatalf("a catalog built before the source scan must say replacement is absent: %+v %+v", m, rec)
	}
	if rec.Source != "registry.opentofu.org/example/widget" || rec.Line != "1" || rec.Tool != "opentofu test" || len(rec.Releases) != 2 || rec.Releases[0].ArchiveSHA256 != strings.Repeat("a", 64) || rec.Releases[0].ExportSHA256 == "" || rec.Releases[1].Resources != 2 {
		t.Fatalf("%+v", rec)
	}
	f, err := iacfacts.ReadIaCCatalog(path, iacfacts.IaCCatalogRequest{Version: "1.1.0", Kind: iacfacts.SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "tags", "type"}})
	if err != nil || string(f.Value) != `"string"` {
		t.Fatalf("%s %v", f.Value, err)
	}
	b, _ := os.ReadFile(path + ".receipt.json")
	var round receipt
	if err := json.Unmarshal(b, &round); err != nil || round.SHA256 != rec.SHA256 {
		t.Fatalf("receipt unreadable: %v", err)
	}
}

func TestBuildCatalogRefusesTheWrongProviderAndLine(t *testing.T) {
	dir := t.TempDir()
	writeExport(t, dir, "example/other", "1.0.0", "x")
	writeExport(t, dir, "example/other", "1.1.0", "x")
	_, err := buildCatalog(context.Background(), testLine(), dirExporter{dir: dir, tool: "t"}, t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("an unreadable export directory must fail")
	}
	// exports named for the pinned provider but holding another.
	dir = t.TempDir()
	writeExport(t, dir, "example/other", "1.0.0", "x")
	writeExport(t, dir, "example/other", "1.1.0", "x")
	for _, v := range []string{"1.0.0", "1.1.0"} {
		os.Rename(filepath.Join(dir, "example-other_"+v+".json"), filepath.Join(dir, "example-widget_"+v+".json"))
	}
	_, err = buildCatalog(context.Background(), testLine(), dirExporter{dir: dir, tool: "t"}, t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not the pinned provider") {
		t.Fatalf("%v", err)
	}
	l := testLine()
	l.Line = 2
	_, err = buildCatalog(context.Background(), l, dirExporter{dir: fixtureExports(t), tool: "t"}, t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "line") {
		t.Fatalf("%v", err)
	}
}

func TestLoadPinsFile(t *testing.T) {
	pf, err := loadPins(filepath.Join("..", "..", "calibration", "iac", "pins.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"hashicorp/aws": false, "hashicorp/azurerm": false, "hashicorp/google": false, "hashicorp/kubernetes": false, "hashicorp/helm": false}
	for _, p := range pf.Providers {
		if _, ok := want[p.Source]; !ok {
			t.Errorf("unexpected provider %s", p.Source)
		}
		want[p.Source] = true
		if len(p.Lines) == 0 {
			t.Errorf("%s pins no line", p.Source)
		}
	}
	for s, seen := range want {
		if !seen {
			t.Errorf("the first five providers include %s", s)
		}
	}
	if got := len(pf.selectLines("hashicorp/aws", 5)); got != 1 {
		t.Fatalf("aws line 5: %d", got)
	}
	if got := pf.selectLines("", 0); len(got) < 10 || got[0].Registry != "registry.opentofu.org" || got[0].Platform != "linux_amd64" {
		t.Fatalf("%+v", got)
	}
}

func TestPinValidationRefusals(t *testing.T) {
	good := func() pinFile {
		return pinFile{Registry: "r", Platform: "linux_amd64", Providers: []pinProvider{{Source: "a/b", Lines: []pinLine{{Line: 1, Releases: []pinRelease{{Version: "1.0.0", SHA256: strings.Repeat("0", 64)}}}}}}}
	}
	if pf := good(); pf.validate() != nil {
		t.Fatal("the base pin file must validate")
	}
	for name, mut := range map[string]func(*pinFile){
		"short sha":      func(p *pinFile) { p.Providers[0].Lines[0].Releases[0].SHA256 = "abc" },
		"wrong line":     func(p *pinFile) { p.Providers[0].Lines[0].Releases[0].Version = "2.0.0" },
		"pre-release":    func(p *pinFile) { p.Providers[0].Lines[0].Releases[0].Version = "1.0.0-rc1" },
		"bad source":     func(p *pinFile) { p.Providers[0].Source = "ab" },
		"duplicate line": func(p *pinFile) { p.Providers[0].Lines = append(p.Providers[0].Lines, p.Providers[0].Lines[0]) },
		"unordered": func(p *pinFile) {
			r := p.Providers[0].Lines[0].Releases
			p.Providers[0].Lines[0].Releases = append(r, r[0])
		},
		"no registry": func(p *pinFile) { p.Registry = "" },
	} {
		pf := good()
		mut(&pf)
		if pf.validate() == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

// fakeTofu is a stand-in executable: init succeeds, `providers schema -json` prints a canned export, and
// `version -json` names a version. It records that it ran in an environment pointed at the mirror.
func fakeTofu(t *testing.T, export string) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  init) test -f \"$TF_CLI_CONFIG_FILE\" && test -n \"$TF_DATA_DIR\" && echo ran > \"$TF_DATA_DIR.ran\" || exit 3 ;;\n" +
		"  providers) cat '" + export + "' ;;\n" +
		"  version) echo '{\"terraform_version\":\"1.13.0\"}' ;;\n" +
		"  *) exit 4 ;;\n" +
		"esac\n"
	p := filepath.Join(dir, "tofu")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTofuExporterHashesTheArchiveAgainstItsPin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-in")
	}
	exports := fixtureExports(t)
	archives := t.TempDir()
	line := pinLine{Line: 1, Source: "example/widget", Registry: "registry.opentofu.org", Platform: archivePlatform(), Releases: []pinRelease{{Version: "1.0.0"}}}
	zipPath := filepath.Join(archives, "terraform-provider-widget_1.0.0_"+archivePlatform()+".zip")
	os.WriteFile(zipPath, []byte("pretend provider archive"), 0o644)
	ex := tofuExporter{tofu: fakeTofu(t, filepath.Join(exports, "example-widget_1.0.0.json")), archives: archives, work: t.TempDir()}

	line.Releases[0].SHA256 = strings.Repeat("0", 64)
	if _, _, err := ex.export(context.Background(), line, line.Releases[0]); err == nil || !strings.Contains(err.Error(), "not its pin") {
		t.Fatalf("an archive that does not hash to its pin must be refused: %v", err)
	}
	// the real digest of the stand-in archive.
	sum := sha256Hex([]byte("pretend provider archive"))
	line.Releases[0].SHA256 = sum
	rc, tool, err := ex.export(context.Background(), line, line.Releases[0])
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if tool != "opentofu 1.13.0" {
		t.Fatalf("tool %q", tool)
	}
	var export map[string]any
	if err := json.NewDecoder(rc).Decode(&export); err != nil || export["provider_schemas"] == nil {
		t.Fatalf("export unreadable: %v", err)
	}
	line.Platform = "plan9_riscv"
	if _, _, err := ex.export(context.Background(), line, line.Releases[0]); err == nil || !strings.Contains(err.Error(), "this host") {
		t.Fatalf("a pin for another platform cannot be hashed here: %v", err)
	}
}

func TestRunBuildsFromPinsAndExports(t *testing.T) {
	exports := t.TempDir()
	writeExport(t, exports, "example/widget", "1.0.0", "a")
	pins := filepath.Join(t.TempDir(), "pins.yaml")
	os.WriteFile(pins, []byte("registry: registry.opentofu.org\nplatform: linux_amd64\nproviders:\n  - source: example/widget\n    lines:\n      - line: 1\n        releases:\n          - {version: 1.0.0, sha256: "+strings.Repeat("c", 64)+"}\n"), 0o644)
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	if rc := run([]string{"-pins", pins, "-out", out, "-exports", exports, "-tool", "opentofu test"}, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc %d %s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "registry.opentofu.org_example_widget.v1.iaccat.zip sha256=") {
		t.Fatalf("%q", stdout.String())
	}
	ents, _ := os.ReadDir(out)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".work") {
			t.Fatalf("scratch left behind: %s", e.Name())
		}
	}
	if rc := run([]string{"-pins", pins, "-out", out}, &stdout, &stderr); rc != 2 {
		t.Fatalf("missing exporter: rc %d", rc)
	}
	if rc := run([]string{"-pins", pins, "-out", out, "-exports", exports}, &stdout, &stderr); rc != 2 {
		t.Fatalf("-exports without -tool: rc %d", rc)
	}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
