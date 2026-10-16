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

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

// Purchase access is explicit. All generated sources, user-shaped example copy,
// screenshots and diagnostics stay private. Existing base receipts are untouched.
func TestAdaptDirectoryEnhancedLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT for the private P04 enhancement probe")
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
		if err != nil || (strings.HasSuffix(name, ".zip") && hash != archiveSHA) {
			t.Fatal("private source identity unavailable or changed")
		}
		identities[file] = hash
	}
	t.Cleanup(func() {
		for file, before := range identities {
			if after, err := pricingFileSHA256(file); err != nil || after != before {
				t.Error("read-only source or catalog changed")
			}
		}
	})
	file, err := designassets.ReadFile(assets, DirectoryArchive, DirectoryMember)
	if err != nil || file.SHA256 != DirectorySourceSHA {
		t.Fatal("frozen P04 unavailable; source/diagnostics withheld")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	first, err := AdaptDirectoryEnhanced(source, []byte(file.Content))
	if err != nil {
		t.Fatal("P04 enhancement failed")
	}
	second, err := AdaptDirectoryEnhanced(source, []byte(file.Content))
	base, baseErr := AdaptDirectory(source, []byte(file.Content))
	if err != nil || baseErr != nil || !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first.Base, base) || VerifyDirectoryEnhancedReplay(first, []byte(file.Content)) != nil {
		t.Fatal("P04 enhancement replay or base lineage differs")
	}
	for _, name := range []string{"Templ", "Model", "CSS", "RuleSHA256", "TemplSHA256", "ModelSHA256", "CSSSHA256", "Verification", "Adapter"} {
		t.Run("replay-refuses-"+name, func(t *testing.T) {
			changed := first
			f := reflect.ValueOf(&changed).Elem().FieldByName(name)
			f.SetString(f.String() + "x")
			if VerifyDirectoryEnhancedReplay(changed, []byte(file.Content)) == nil {
				t.Fatal("companion tamper passed")
			}
		})
	}
	changed := first
	changed.Base.Defaults = append(append(json.RawMessage(nil), first.Base.Defaults...), ' ')
	if VerifyDirectoryEnhancedReplay(changed, []byte(file.Content)) == nil {
		t.Fatal("base defaults tamper passed")
	}
	encode := func(v any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal("fixture encoding")
		}
		return b
	}
	var defaults map[string]any
	if json.Unmarshal(first.Base.Defaults, &defaults) != nil || defaults["AddURL"] != "" {
		t.Fatal("invented default destination")
	}
	defaults["AddURL"] = "/directory/new"
	rows, ok := defaults["Rows"].([]any)
	if !ok || len(rows) != 6 {
		t.Fatal("invalid private defaults")
	}
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || row["EditURL"] != "" {
			t.Fatal("invented default row destination")
		}
		row["EditURL"] = "/directory/edit/" + strconv.Itoa(i+1)
	}
	clone := func() map[string]any {
		t.Helper()
		var result map[string]any
		if json.Unmarshal(encode(defaults), &result) != nil {
			t.Fatal("fixture clone")
		}
		return result
	}
	scenarios := []string{"baseline6", "empty0", "partial3", "long6"}
	fixtures := map[string]map[string]any{"baseline6": defaults, "empty0": clone(), "partial3": clone(), "long6": clone()}
	fixtures["empty0"]["Rows"] = []any{}
	fixtures["empty0"]["Introduction"] = "No people yet. Add a user to begin."
	fixtures["partial3"]["Rows"] = fixtures["partial3"]["Rows"].([]any)[:3]
	fixtures["partial3"]["Introduction"] = "Showing 3 available records from a partial result. Other records are not included."
	long := fixtures["long6"]
	long["Heading"] = strings.Repeat("Directory", 17)
	long["Introduction"] = strings.Repeat("Detailed role and contact information for this synthetic directory. ", 16) + "<literal & text>"
	for _, name := range []string{"AddLabel", "EditLabel", "NameLabel", "TitleLabel", "EmailLabel", "RoleLabel"} {
		long[name] = strings.Repeat(strings.TrimSuffix(name, "Label"), 18)
	}
	for i, value := range long["Rows"].([]any) {
		row := value.(map[string]any)
		row["Name"] = "LongMember" + strings.Repeat("x", 220) + strconv.Itoa(i+1)
		row["Title"] = strings.Repeat("PlatformEngineering", 12) + strconv.Itoa(i+1)
		row["Email"] = "member" + strconv.Itoa(i+1) + "." + strings.Repeat("x", 285) + "@example.test"
		row["Role"] = strings.Repeat("Contributor", 9) + strconv.Itoa(i+1)
	}
	dir := t.TempDir()
	if retain == "1" {
		dir, err = os.MkdirTemp("", "appkit-p04-enhanced-")
		if err != nil {
			t.Fatal("private output directory")
		}
		t.Logf("private P04 enhanced output: %s", dir)
	}
	write := func(name string, data []byte) {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("private output creation")
		}
		_, we := f.Write(data)
		ce := f.Close()
		if we != nil || ce != nil {
			t.Fatal("private output write")
		}
	}
	for name, b := range map[string][]byte{
		"source.html": []byte(file.Content), "artifact.json": encode(first.Base), "enhanced-artifact.json": encode(first),
		"directory.templ": []byte(first.Base.Templ), "directory_model.go": []byte(first.Base.Model),
		"directory_enhanced.templ": []byte(first.Templ), "directory_enhanced_model.go": []byte(first.Model),
		"directory.css": []byte(first.CSS), "directory_test.go": []byte(directoryEnhancedLiveRuntimeTest),
	} {
		write(name, b)
	}
	for _, name := range scenarios {
		write(name+".json", encode(fixtures[name]))
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("kit root")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, phase := range []string{"unformatted", "formatted"} {
		baseTemplate, companionTemplate := "directory.templ", "directory_enhanced.templ"
		baseGenerated, companionGenerated := "directory_templ.go", "directory_enhanced_templ.go"
		run := func(step string, args ...string) {
			t.Helper()
			cmd := exec.CommandContext(ctx, "go", args...)
			cmd.Dir = repo
			cmd.Env = append(pricingOfflineGoEnv(), "APPKIT_DIRECTORY_PHASE="+phase, "APPKIT_DIRECTORY_OUTPUT="+dir)
			if cmd.Run() != nil {
				t.Fatalf("P04 enhancement %s %s failed; private diagnostics withheld", phase, step)
			}
		}
		if phase == "formatted" {
			// Preserve artifact template bytes as immutable inputs. Format and
			// compile explicit copies; both phases retain their actual sources.
			baseTemplate, companionTemplate = "directory_formatted.templ", "directory_enhanced_formatted.templ"
			baseGenerated, companionGenerated = "directory_formatted_templ.go", "directory_enhanced_formatted_templ.go"
			write(baseTemplate, []byte(first.Base.Templ))
			write(companionTemplate, []byte(first.Templ))
			run("format", "tool", "templ", "fmt", filepath.Join(dir, baseTemplate), filepath.Join(dir, companionTemplate))
		}
		for _, name := range []string{baseTemplate, companionTemplate} {
			run("generate", "tool", "templ", "generate", "-f", filepath.Join(dir, name), "-include-version=false")
		}
		run("compile-render", "test", "-count=1", "-run", "^TestDirectoryEnhancedLiveDOM$", filepath.Join(dir, "directory_model.go"), filepath.Join(dir, baseGenerated), filepath.Join(dir, "directory_enhanced_model.go"), filepath.Join(dir, companionGenerated), filepath.Join(dir, "directory_test.go"))
		if b, err := os.ReadFile(filepath.Join(dir, phase+".pass")); err != nil || string(b) != "passed\n" {
			t.Fatal("generated test completion missing")
		}
	}
	type identity struct {
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}
	identify := func(file string) identity {
		t.Helper()
		info, err := os.Lstat(file)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			t.Fatal("unsafe fixture identity")
		}
		hash, err := pricingFileSHA256(file)
		if err != nil {
			t.Fatal("fixture hash")
		}
		return identity{hash, info.Size()}
	}
	files := map[string]identity{}
	for _, name := range []string{"source.html", "artifact.json", "enhanced-artifact.json", "directory.templ", "directory_model.go", "directory_enhanced.templ", "directory_enhanced_model.go", "directory.css", "directory_test.go", "directory_templ.go", "directory_enhanced_templ.go", "directory_formatted.templ", "directory_enhanced_formatted.templ", "directory_formatted_templ.go", "directory_enhanced_formatted_templ.go", "unformatted.pass", "formatted.pass"} {
		files[name] = identify(filepath.Join(dir, name))
	}
	for _, name := range scenarios {
		files[name+".json"] = identify(filepath.Join(dir, name+".json"))
		for _, kind := range []string{"faithful", "enhanced"} {
			var before identity
			for _, phase := range []string{"unformatted", "formatted"} {
				file := name + "-" + kind + "-" + phase + ".html"
				got := identify(filepath.Join(dir, file))
				files[file] = got
				if phase == "unformatted" {
					before = got
				} else if got != before {
					t.Fatal("formatter changed rendered output")
				}
			}
		}
	}
	code := map[string]identity{}
	for _, name := range []string{"directory.go", "directory_model.go.txt", "directory.cue", "directory_enhanced.go", "directory_enhanced_model.go.txt", "directory_enhanced.templ.txt", "directory_enhanced.css", "directory_enhanced.cue", "directory_enhanced_live_test.go", "directory_enhanced_test.go"} {
		file := "internal/designadapt/" + name
		code[file] = identify(filepath.Join(repo, file))
	}
	write("enhanced-reference.json", encode(map[string]any{
		"schema": 1, "case_id": "P04-table", "scope": "enhanced-target-fixtures", "verification": "not-verified",
		"source": source, "scenarios": scenarios, "files": files, "code": code,
	}))
}

const directoryEnhancedLiveRuntimeTest = `package converted
import("bytes";"context";"encoding/json";"os";"path/filepath";"reflect";"strings";"testing";"golang.org/x/net/html")
func TestDirectoryEnhancedLiveDOM(t *testing.T) {
 dir,phase:=os.Getenv("APPKIT_DIRECTORY_OUTPUT"),os.Getenv("APPKIT_DIRECTORY_PHASE")
 if dir==""||(phase!="formatted"&&phase!="unformatted"){t.Fatal("output contract")}
 write:=func(name string,b []byte){t.Helper();f,e:=os.OpenFile(filepath.Join(dir,name),os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600);if e!=nil{t.Fatal("output exists")};_,we:=f.Write(b);ce:=f.Close();if we!=nil||ce!=nil{t.Fatal("write")}}
 for _,scenario:=range []string{"baseline6","empty0","partial3","long6"}{
  raw,e:=os.ReadFile(filepath.Join(dir,scenario+".json"));if e!=nil{t.Fatal("fixture unavailable")};var d Directory;if json.Unmarshal(raw,&d)!=nil{t.Fatal("fixture invalid")}
  faithful,e:=NewDirectoryNavigation(d);if e!=nil{t.Fatal("faithful constructor")};enhanced,e:=NewDirectoryEnhanced(d);if e!=nil{t.Fatal("enhanced constructor")}
  var a,b bytes.Buffer;if faithful.Render(context.Background(),&a)!=nil||enhanced.Render(context.Background(),&b)!=nil{t.Fatal("render")}
  got,e1:=html.Parse(strings.NewReader(b.String()));want,e2:=html.Parse(strings.NewReader("<div data-appkit-directory>"+a.String()+"</div>"));if e1!=nil||e2!=nil||!reflect.DeepEqual(got,want){t.Fatal("enhancement changed faithful content")}
  if strings.Count(b.String(),"data-appkit-directory")!=1{t.Fatal("wrapper count")}
  write(scenario+"-faithful-"+phase+".html",a.Bytes());write(scenario+"-enhanced-"+phase+".html",b.Bytes())
 }
 write(phase+".pass",[]byte("passed\n"))
}
`
