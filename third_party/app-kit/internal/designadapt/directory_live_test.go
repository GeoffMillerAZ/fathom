package designadapt

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Explicit opt-in purchase access. No paid fixture, derivative or diagnostics
// enter tracked files; retained outputs always get a new private temp directory.
func TestAdaptDirectoryLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT for the private P04 probe")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private kit root must be absolute")
	}
	retain := os.Getenv("APPKIT_PROBE_DIRECTORY_RETAIN")
	if retain != "" && retain != "1" {
		t.Fatal("retain must be unset or 1")
	}
	assets := filepath.Join(root, "data/design-assets")
	archiveSHA := strings.Split(DirectoryArchive, "@")[1]
	identities := map[string]string{}
	for _, name := range []string{"catalog.json", "archives/" + archiveSHA + ".zip"} {
		file := filepath.Join(assets, name)
		hash, err := pricingFileSHA256(file)
		if err != nil {
			t.Fatal("private source identity unavailable")
		}
		identities[file] = hash
		if strings.HasSuffix(name, ".zip") && hash != archiveSHA {
			t.Fatal("archive hash differs")
		}
	}
	t.Cleanup(func() {
		for file, before := range identities {
			after, err := pricingFileSHA256(file)
			if err != nil || after != before {
				t.Error("read-only source or catalog changed")
			}
		}
	})
	file, err := designassets.ReadFile(assets, DirectoryArchive, DirectoryMember)
	if err != nil || file.SHA256 != DirectorySourceSHA {
		t.Fatal("frozen P04 unavailable; source/diagnostics withheld")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	first, err := AdaptDirectory(source, []byte(file.Content))
	if err != nil {
		t.Fatalf("P04 adaptation failed: %v", err)
	}
	second, err := AdaptDirectory(source, []byte(file.Content))
	if err != nil || !reflect.DeepEqual(first, second) || VerifyDirectoryReplay(first, []byte(file.Content)) != nil {
		t.Fatal("P04 replay differs")
	}
	for _, name := range []string{"Templ", "Model", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "DefaultsSHA256", "Verification", "Adapter"} {
		t.Run("replay-refuses-"+name, func(t *testing.T) {
			changed := first
			f := reflect.ValueOf(&changed).Elem().FieldByName(name)
			f.SetString(f.String() + "x")
			if VerifyDirectoryReplay(changed, []byte(file.Content)) == nil {
				t.Fatal("tamper passed")
			}
		})
	}
	changed := first
	changed.Defaults = append(append(json.RawMessage(nil), first.Defaults...), ' ')
	if VerifyDirectoryReplay(changed, []byte(file.Content)) == nil {
		t.Fatal("defaults tamper passed")
	}
	var defaults map[string]any
	if json.Unmarshal(first.Defaults, &defaults) != nil || defaults["AddURL"] != "" {
		t.Fatal("invented Add destination")
	}
	defaults["AddURL"] = "/directory/new"
	rows, ok := defaults["Rows"].([]any)
	if !ok || len(rows) != 6 {
		t.Fatal("invalid private defaults")
	}
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || row["EditURL"] != "" {
			t.Fatal("invented row destination")
		}
		row["EditURL"] = "/directory/edit/" + strconv.Itoa(i+1)
	}
	fixtureDefaults, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal("fixture encoding")
	}
	dir := t.TempDir()
	if retain == "1" {
		dir, err = os.MkdirTemp("", "appkit-p04-typed-")
		if err != nil {
			t.Fatal("private output directory")
		}
		t.Logf("private P04 output: %s", dir)
	}
	artifact, err := json.Marshal(first)
	if err != nil {
		t.Fatal("artifact encoding")
	}
	fixture := strings.NewReplacer("SOURCE_LITERAL", strconv.Quote(file.Content), "DEFAULTS_LITERAL", strconv.Quote(string(fixtureDefaults))).Replace(directoryLiveRuntimeTest)
	for name, data := range map[string][]byte{
		"source.html": []byte(file.Content), "artifact.json": artifact, "defaults.json": fixtureDefaults,
		"directory.templ": []byte(first.Templ), "directory_model.go": []byte(first.Model), "directory_test.go": []byte(fixture),
	} {
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("private output creation")
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal("private output write")
		}
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("kit root")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, phase := range []string{"unformatted", "formatted"} {
		marker := filepath.Join(dir, phase+".pass")
		run := func(step string, args ...string) {
			t.Helper()
			cmd := exec.CommandContext(ctx, "go", args...)
			cmd.Dir = repo
			cmd.Env = append(pricingOfflineGoEnv(), "APPKIT_DIRECTORY_PHASE="+phase, "APPKIT_DIRECTORY_OUTPUT="+dir)
			if cmd.Run() != nil {
				t.Fatalf("P04 %s %s failed; private diagnostics withheld", phase, step)
			}
		}
		if phase == "formatted" {
			run("format", "tool", "templ", "fmt", filepath.Join(dir, "directory.templ"))
		}
		run("generate", "tool", "templ", "generate", "-f", filepath.Join(dir, "directory.templ"), "-include-version=false")
		run("compile-render", "test", "-count=1", "-run", "^TestDirectoryLiveExactDOM$", filepath.Join(dir, "directory_model.go"), filepath.Join(dir, "directory_templ.go"), filepath.Join(dir, "directory_test.go"))
		if b, err := os.ReadFile(marker); err != nil || string(b) != "passed\n" {
			t.Fatal("generated test completion missing")
		}
	}
}

const directoryLiveRuntimeTest = `package converted
import("bytes";"context";"encoding/json";"os";"path/filepath";"reflect";"strings";"testing";"golang.org/x/net/html")
const sourceOriginal = SOURCE_LITERAL
const defaultsLiteral = DEFAULTS_LITERAL
func TestDirectoryLiveExactDOM(t *testing.T) {
 var data Directory; if json.Unmarshal([]byte(defaultsLiteral),&data)!=nil {t.Fatal("fixture")}
 c,e:=NewDirectoryNavigation(data);if e!=nil {t.Fatal("constructor")};var out bytes.Buffer;if c.Render(context.Background(),&out)!=nil {t.Fatal("render")}
 want,e:=html.Parse(strings.NewReader(sourceOriginal));if e!=nil {t.Fatal("reference parse")}
 added,edited:=0,0
 var mapActions func(*html.Node);mapActions=func(n *html.Node){if n.Type==html.ElementNode {switch n.Data {case "button":n.Data="a";n.DataAtom=0;added++;found:=false;for i,a:=range n.Attr {if a.Key=="type"&&a.Val=="button" {n.Attr[i].Key="href";n.Attr[i].Val=data.AddURL;found=true}};if !found {t.Fatal("source button")};case "a": if edited>=len(data.Rows){t.Fatal("source links")};found:=false;for i,a:=range n.Attr {if a.Key=="href"&&a.Val=="#" {n.Attr[i].Val=data.Rows[edited].EditURL;found=true}};if !found {t.Fatal("source link")};edited++}};for c:=n.FirstChild;c!=nil;c=c.NextSibling {mapActions(c)}};mapActions(want)
 if added!=1||edited!=6 {t.Fatal("declared action count")}
 // Reparse mapped reference solely to populate tag atoms; no source text,
 // attribute, class, ordering or whitespace is normalized by the comparison.
 var mapped bytes.Buffer;if html.Render(&mapped,want)!=nil {t.Fatal("mapped reference")};want,e=html.Parse(strings.NewReader(mapped.String()));if e!=nil {t.Fatal("mapped parse")}
 same:=func(s string)bool {got,e:=html.Parse(strings.NewReader(s));return e==nil&&reflect.DeepEqual(got,want)}
 if !same(out.String()) {t.Fatal("exact mapped DOM differs")}
 if same(strings.Replace(out.String(),"\n","\n ",1)) {t.Fatal("whitespace canary missed")}
 if same(strings.Replace(out.String(),data.Rows[0].Title,"wrong repeated value",1)) {t.Fatal("mobile repeat canary missed")}
 if same(strings.Replace(out.String(),"scope=\"col\"","scope=\"row\"",1)) {t.Fatal("attribute canary missed")}
 dir,phase:=os.Getenv("APPKIT_DIRECTORY_OUTPUT"),os.Getenv("APPKIT_DIRECTORY_PHASE");if dir==""||(phase!="formatted"&&phase!="unformatted"){t.Fatal("output contract")}
 for name,b:=range map[string][]byte{phase+".html":out.Bytes(),phase+".pass":[]byte("passed\n")} {f,e:=os.OpenFile(filepath.Join(dir,name),os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600);if e!=nil {t.Fatal("output exists")};_,we:=f.Write(b);ce:=f.Close();if we!=nil||ce!=nil {t.Fatal("write")}}
}
`
