package designadapt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDirectoryEnhancedSourceLineageAndCSS(t *testing.T) {
	body := syntheticDirectory(6)
	source := syntheticSource(body)
	if _, err := AdaptDirectoryEnhanced(source, []byte(body)); err == nil {
		t.Fatal("enhancement accepted arbitrary source identity")
	}
	spoofed := source
	spoofed.ArchiveID, spoofed.Path, spoofed.SHA256 = DirectoryArchive, DirectoryMember, DirectorySourceSHA
	if _, err := AdaptDirectoryEnhanced(spoofed, []byte(body)); err == nil {
		t.Fatal("enhancement accepted spoofed source bytes")
	}
	base, err := adaptDirectory(source, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	first, err := enhanceDirectory(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := enhanceDirectory(base)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("enhancement is not deterministic")
	}
	if !reflect.DeepEqual(first.Base, base) || first.SchemaVersion != 1 || first.Adapter == "" || first.Adapter == base.Adapter || first.Verification != "not-verified" {
		t.Fatal("enhancement lost or obscured base lineage")
	}
	digest := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	for name, value := range map[string]string{
		"rule": first.RuleSHA256, "templ": first.TemplSHA256,
		"model": first.ModelSHA256, "css": first.CSSSHA256,
	} {
		if len(value) != 64 {
			t.Fatalf("missing %s digest", name)
		}
	}
	if first.TemplSHA256 != digest(first.Templ) || first.ModelSHA256 != digest(first.Model) || first.CSSSHA256 != digest(first.CSS) {
		t.Fatal("enhancement payload digest mismatch")
	}
	css := first.CSS
	for _, required := range []string{"[data-appkit-directory]", "outline: 2px", "overflow-wrap: anywhere", "th:nth-child(4)", "td:nth-child(4)", "min-inline-size: 5rem"} {
		if !strings.Contains(css, required) {
			t.Fatalf("enhancement CSS missing %q", required)
		}
	}
	for _, property := range []string{"min-block-size", "min-inline-size"} {
		if !strings.Contains(css, property+": 44px") && !strings.Contains(css, property+": 2.75rem") {
			t.Fatalf("enhancement CSS does not give actions a 44px %s", property)
		}
	}
	for _, forbidden := range []string{"#000", "#fff", "rgb(", "rgba(", "oklch(", "html {", "body {", ":root {"} {
		if strings.Contains(strings.ToLower(css), forbidden) {
			t.Fatalf("enhancement CSS contains unscoped/raw palette token %q", forbidden)
		}
	}
	withoutComments := css
	for {
		start := strings.Index(withoutComments, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(withoutComments[start+2:], "*/")
		if end < 0 {
			t.Fatal("unterminated CSS comment")
		}
		withoutComments = withoutComments[:start] + withoutComments[start+2+end+2:]
	}
	for _, block := range strings.Split(withoutComments, "}") {
		selector, _, found := strings.Cut(block, "{")
		if strings.TrimSpace(selector) != "" && (!found || !strings.Contains(selector, "[data-appkit-directory]")) {
			t.Fatalf("unscoped enhancement selector %q", strings.TrimSpace(selector))
		}
	}
}

func TestDirectoryEnhancedSyntheticCompilerAndRuntime(t *testing.T) {
	body := syntheticDirectory(6)
	base, err := adaptDirectory(syntheticSource(body), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	enhanced, err := enhanceDirectory(base)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{
		"directory.templ":             base.Templ,
		"directory_model.go":          base.Model,
		"directory_enhanced.templ":    enhanced.Templ,
		"directory_enhanced_model.go": enhanced.Model,
		"directory_enhanced_test.go":  directoryEnhancedRuntimeTests,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic enhanced directory compiler test: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "directory.templ"), filepath.Join(dir, "directory_enhanced.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "directory.templ"), "-include-version=false")
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "directory_enhanced.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "directory_model.go"), filepath.Join(dir, "directory_templ.go"), filepath.Join(dir, "directory_enhanced_model.go"), filepath.Join(dir, "directory_enhanced_templ.go"), filepath.Join(dir, "directory_enhanced_test.go"))
	}
}

const directoryEnhancedRuntimeTests = `package converted
import("bytes";"context";"errors";"fmt";"reflect";"strings";"testing";"golang.org/x/net/html")
type failWriter struct{err error}
func(w failWriter)Write([]byte)(int,error){return 0,w.err}
func fixture(n int) Directory {d:=Directory{Heading:"Roster",Introduction:"App-owned state copy",AddLabel:"Add",EditLabel:"Edit",NameLabel:"Name",TitleLabel:"Title",EmailLabel:"Email",RoleLabel:"Role",AddURL:"/new"};for i:=1;i<=n;i++{d.Rows=append(d.Rows,DirectoryRow{Name:fmt.Sprintf("Name %d",i),Title:fmt.Sprintf("Title %d",i),Email:fmt.Sprintf("person%d@example.test",i),Role:fmt.Sprintf("Role %d",i),EditURL:fmt.Sprintf("/edit/%d",i)})};return d}
func rendered(t *testing.T,d Directory,enhanced bool)string{t.Helper();if enhanced{c,e:=NewDirectoryEnhanced(d);if e!=nil{t.Fatal(e)};var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil{t.Fatal(e)};return b.String()};c,e:=NewDirectoryNavigation(d);if e!=nil{t.Fatal(e)};var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil{t.Fatal(e)};return b.String()}
func TestEnhancedWrapsExactBaseContent(t *testing.T){for _,n:=range []int{0,3,6,100}{d:=fixture(n);if n==0{d.Introduction="No records are available."}else if n<6{d.Introduction="Only part of the app-owned result is shown."};base:=rendered(t,d,false);got:=rendered(t,d,true);want:=` + "`<div data-appkit-directory>`" + `+base+` + "`</div>`" + `;gd,e1:=html.Parse(strings.NewReader(got));wd,e2:=html.Parse(strings.NewReader(want));if e1!=nil||e2!=nil||!reflect.DeepEqual(gd,wd){t.Fatalf("wrapper changed base DOM for %d rows: got=%q want=%q",n,got,want)};if strings.Count(got,"data-appkit-directory")!=1{t.Fatal("wrapper count")}}}
func TestEnhancedPreservesValidationSnapshotAndFailures(t *testing.T){bad:=fixture(1);bad.AddURL="javascript:alert(1)";base,be:=NewDirectoryNavigation(bad);enh,ee:=NewDirectoryEnhanced(bad);if be==nil||ee==nil||base!=nil||enh!=nil||be.Error()!=ee.Error(){t.Fatal("enhancement validation diverged")};d:=fixture(3);c,e:=NewDirectoryEnhanced(d);if e!=nil{t.Fatal(e)};d.Rows[0].Name="mutation";d.Rows=append(d.Rows,DirectoryRow{Name:"new row"});var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil||strings.Contains(b.String(),"mutation")||strings.Contains(b.String(),"new row"){t.Fatal("enhancement lost base snapshot")};sentinel:=errors.New("writer failure");if e=c.Render(context.Background(),failWriter{sentinel});!errors.Is(e,sentinel){t.Fatal("writer error lost")};ctx,cancel:=context.WithCancel(context.Background());cancel();if e=c.Render(ctx,&b);!errors.Is(e,context.Canceled){t.Fatal("cancellation lost")}}
func TestEnhancedEscapesAppOwnedStateCopy(t *testing.T){for _,state:=range []struct{rows int;copy string}{{0,"<b>Empty</b> & waiting"},{3,"Partial <script>state</script>"}}{d:=fixture(state.rows);d.Introduction=state.copy;out:=rendered(t,d,true);doc,e:=html.Parse(strings.NewReader(out));if e!=nil{t.Fatal(e)};text:="";bad:=false;var walk func(*html.Node);walk=func(n *html.Node){if n.Type==html.ElementNode&&(n.Data=="b"||n.Data=="script"){bad=true};if n.Type==html.TextNode{text+=n.Data};for c:=n.FirstChild;c!=nil;c=c.NextSibling{walk(c)}};walk(doc);if bad||!strings.Contains(text,state.copy){t.Fatal("state copy was executable or changed")}}}
`
