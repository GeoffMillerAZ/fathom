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
)

// syntheticDirectory contains only invented structure and copy. It exercises the
// P04 shape without reproducing purchased markup, classes, icons, or wording.
func syntheticDirectory(rows int) string {
	var b strings.Builder
	b.WriteString(`<section data-synthetic="directory"><header><h1>Test roster</h1><p>Invented fixture copy.</p><button type="button" data-action="new">Create record</button></header><table><thead><tr><th scope="col">Person</th><th scope="col">Position</th><th scope="col">Mailbox</th><th scope="col">Group</th><th scope="col"><span>Change</span></th></tr></thead><tbody>`)
	for i := 1; i <= rows; i++ {
		if i > 1 {
			b.WriteByte('\n')
		}
		name := "Fixture person " + strconv.Itoa(i)
		title := "Fixture title " + strconv.Itoa(i)
		email := "person" + strconv.Itoa(i) + "@example.test"
		role := "Fixture role " + strconv.Itoa(i)
		b.WriteString(`<tr data-synthetic="row"><td>` + name + `</td><td><dl><dt>Position</dt><dd>` + title + `</dd><dt>Mailbox</dt><dd>` + email + `</dd></dl></td><td>` + title + `</td><td>` + email + `</td><td>` + role + `</td><td><a href="#">Change<span>, ` + name + `</span></a></td></tr>`)
	}
	b.WriteString(`</tbody></table></section>`)
	return b.String()
}

func replaceNth(s, old, replacement string, n int) string {
	start := 0
	for i := 0; i < n; i++ {
		at := strings.Index(s[start:], old)
		if at < 0 {
			return s
		}
		start += at
		if i == n-1 {
			return s[:start] + replacement + s[start+len(old):]
		}
		start += len(old)
	}
	return s
}

func TestDirectorySourceAndStructureRefusals(t *testing.T) {
	body := syntheticDirectory(6)
	s := syntheticSource(body)
	if _, err := AdaptDirectory(s, []byte(body)); err == nil {
		t.Fatal("arbitrary source accepted as frozen P04")
	}
	s.ArchiveID, s.Path, s.SHA256 = DirectoryArchive, DirectoryMember, DirectorySourceSHA
	if _, err := AdaptDirectory(s, []byte(body)); err == nil {
		t.Fatal("spoofed source identity accepted")
	}

	row := `<tr data-synthetic="row">`
	cases := map[string]string{
		"missing-outer-field":      strings.Replace(body, `<h1>Test roster</h1>`, "", 1),
		"wrong-outer-parent":       strings.Replace(body, `<h1>Test roster</h1>`, `<h2>Test roster</h2>`, 1),
		"extra-outer-field":        strings.Replace(body, `<table>`, `<p>Unexpected copy</p><table>`, 1),
		"wrong-add-type":           strings.Replace(body, `type="button"`, `type="submit"`, 1),
		"extra-add-control":        strings.Replace(body, `</header>`, `<button type="button">Other</button></header>`, 1),
		"unsafe-behavior":          strings.Replace(body, `<section data-synthetic="directory">`, `<section data-synthetic="directory" onclick="bad()">`, 1),
		"self-closing-extension":   strings.Replace(body, `<table>`, `<hr/><table>`, 1),
		"missing-row":              syntheticDirectory(5),
		"extra-row":                syntheticDirectory(7),
		"row-attribute-variant":    replaceNth(body, row, `<tr data-synthetic="row" data-variant="yes">`, 2),
		"row-separator-variant":    strings.Replace(body, "</tr>\n<tr", "</tr>\n \n<tr", 1),
		"missing-row-field":        strings.Replace(body, `<td>Fixture role 1</td>`, "", 1),
		"wrong-row-parent":         strings.Replace(body, `<dt>Position</dt>`, `<dd>Position</dd>`, 1),
		"title-repeat-mismatch":    strings.Replace(body, `<td>Fixture title 1</td>`, `<td>Different title</td>`, 1),
		"email-repeat-mismatch":    strings.Replace(body, `<td>person1@example.test</td>`, `<td>other@example.test</td>`, 1),
		"mobile-label-mismatch":    strings.Replace(body, `<dt>Position</dt>`, `<dt>Different label</dt>`, 1),
		"accessible-name-mismatch": strings.Replace(body, `<span>, Fixture person 1</span>`, `<span>, Different person</span>`, 1),
		"non-placeholder-edit":     strings.Replace(body, `href="#"`, `href="/edit/1"`, 1),
		"extra-link":               strings.Replace(body, `</section>`, `<a href="#">Extra</a></section>`, 1),
	}
	for name, changed := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := adaptDirectory(syntheticSource(changed), []byte(changed))
			if err == nil || got.Templ != "" || got.Model != "" {
				t.Fatal("unsupported shape emitted partial output")
			}
		})
	}
}

func TestDirectorySyntheticReplayAndCompiler(t *testing.T) {
	body := syntheticDirectory(6)
	first, err := adaptDirectory(syntheticSource(body), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptDirectory(syntheticSource(body), []byte(body))
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("non-deterministic adaptation")
	}
	if first.Verification != "not-verified" || first.Adapter != "p04-directory-navigation-v1" || len(first.RuleSHA256) != 64 || len(first.SyntaxSHA256) != 64 {
		t.Fatal("missing adapter provenance boundary")
	}
	var defaults struct {
		AddURL string
		Rows   []map[string]string
	}
	if err := json.Unmarshal(first.Defaults, &defaults); err != nil || defaults.AddURL != "" || len(defaults.Rows) != 6 {
		t.Fatal("defaults invented a destination or lost rows")
	}
	for _, row := range defaults.Rows {
		if row["EditURL"] != "" {
			t.Fatal("defaults invented an Edit destination")
		}
	}

	expected := strings.Replace(body, `<button type="button"`, `<a href="/new"`, 1)
	expected = strings.Replace(expected, `</button>`, `</a>`, 1)
	for i := 1; i <= 6; i++ {
		expected = strings.Replace(expected, `href="#"`, `href="/edit/`+strconv.Itoa(i)+`"`, 1)
	}
	dir := t.TempDir()
	files := map[string]string{
		"directory.templ":   first.Templ,
		"directory.go":      first.Model,
		"directory_test.go": directoryRuntimeTests + "\nconst sourceExpected = " + strconv.Quote(expected) + "\n",
	}
	for name, content := range files {
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
			t.Fatalf("synthetic directory compiler test: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "directory.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "directory.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "directory.go"), filepath.Join(dir, "directory_templ.go"), filepath.Join(dir, "directory_test.go"))
	}
}

const directoryRuntimeTests = `package converted
import("bytes";"context";"errors";"fmt";"reflect";"strings";"testing";"golang.org/x/net/html")
type failWriter struct{err error}
func(w failWriter)Write([]byte)(int,error){return 0,w.err}
func good(n int) Directory { d:=Directory{Heading:"Roster",Introduction:"Fixture intro",AddLabel:"Add person",EditLabel:"Edit",NameLabel:"Name",TitleLabel:"Title",EmailLabel:"Email",RoleLabel:"Role",AddURL:"/new"}; for i:=0;i<n;i++ {j:=i+1;d.Rows=append(d.Rows,DirectoryRow{Name:fmt.Sprintf("⟦name:%03d⟧",j),Title:fmt.Sprintf("Title %d",j),Email:fmt.Sprintf("email%d@example.test",j),Role:fmt.Sprintf("Role %d",j),EditURL:fmt.Sprintf("/edit/%d",j)})};return d }
func render(t *testing.T,d Directory)string{t.Helper();c,e:=NewDirectoryNavigation(d);if e!=nil{t.Fatal(e)};var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil{t.Fatal(e)};return b.String()}
func walk(n *html.Node, fn func(*html.Node)){fn(n);for c:=n.FirstChild;c!=nil;c=c.NextSibling{walk(c,fn)}}
func TestRuntimeExactSyntheticDOM(t *testing.T){d:=Directory{Heading:"Test roster",Introduction:"Invented fixture copy.",AddLabel:"Create record",NameLabel:"Person",TitleLabel:"Position",EmailLabel:"Mailbox",RoleLabel:"Group",EditLabel:"Change",AddURL:"/new"};for i:=1;i<=6;i++{d.Rows=append(d.Rows,DirectoryRow{Name:fmt.Sprintf("Fixture person %d",i),Title:fmt.Sprintf("Fixture title %d",i),Email:fmt.Sprintf("person%d@example.test",i),Role:fmt.Sprintf("Fixture role %d",i),EditURL:fmt.Sprintf("/edit/%d",i)})};actual:=render(t,d);got,e1:=html.Parse(strings.NewReader(actual));want,e2:=html.Parse(strings.NewReader(sourceExpected));if e1!=nil||e2!=nil||!reflect.DeepEqual(got,want){t.Fatalf("synthetic source DOM mismatch: got=%q want=%q",actual,sourceExpected)}}
func TestRuntimeBindingsCountsAndEscaping(t *testing.T){for _,n:=range []int{0,1,6,100}{d:=good(n);d.Heading="<script>heading</script> & text";if n>0{d.Rows[0].Name="<img src=x>";d.Rows[0].Title="日本語 café"};out:=render(t,d);doc,e:=html.Parse(strings.NewReader(out));if e!=nil{t.Fatal(e)};anchors,buttons,rows:=0,0,0;texts:=map[string]int{};allText:="";walk(doc,func(node *html.Node){if node.Type==html.ElementNode{switch node.Data{case "a":anchors++;case "button","script","img","form":buttons++;case "tbody":for c:=node.FirstChild;c!=nil;c=c.NextSibling{if c.Type==html.ElementNode&&c.Data=="tr"{rows++}}}};if node.Type==html.TextNode{texts[node.Data]++;allText+=node.Data}});if anchors!=n+1||buttons!=0||rows!=n||texts[d.Heading]!=1{t.Fatalf("structure/count mismatch n=%d",n)};for i,r:=range d.Rows{if strings.Count(allText,r.Name)!=2||texts[r.Title]!=2||texts[r.Email]!=2||texts[r.Role]!=1{t.Fatalf("row bindings mismatch n=%d row=%d texts=%v",n,i,texts)}};if texts[d.NameLabel]!=1||texts[d.TitleLabel]!=n+1||texts[d.EmailLabel]!=n+1||texts[d.RoleLabel]!=1||texts[d.EditLabel]!=n+1||texts[d.AddLabel]!=1{t.Fatalf("label bindings mismatch n=%d",n)}}}
func TestRuntimeSnapshotWriterAndCancellation(t *testing.T){d:=good(2);c,e:=NewDirectoryNavigation(d);if e!=nil{t.Fatal(e)};d.Rows[0].Name="mutation";d.Rows=append(d.Rows,DirectoryRow{Name:"new"});var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil||strings.Contains(b.String(),"mutation")||strings.Contains(b.String(),">new<"){t.Fatal("lost row-slice snapshot")};sentinel:=errors.New("writer failure");if e=c.Render(context.Background(),failWriter{sentinel});!errors.Is(e,sentinel){t.Fatal("writer error lost")};ctx,cancel:=context.WithCancel(context.Background());cancel();if e=c.Render(ctx,&b);!errors.Is(e,context.Canceled){t.Fatal("cancellation lost")}}
func TestRuntimeInvalidTextCountsAndURLs(t *testing.T){for _,field:=range []string{"Heading","Introduction","AddLabel","EditLabel","NameLabel","TitleLabel","EmailLabel","RoleLabel"}{for _,bad:=range []string{"","   ","bad\ncontrol",string([]byte{255}),strings.Repeat("x",1201)}{d:=good(1);reflect.ValueOf(&d).Elem().FieldByName(field).SetString(bad);if c,e:=NewDirectoryNavigation(d);e==nil||c!=nil{t.Fatalf("accepted %s=%q",field,bad)}}};for _,field:=range []string{"Name","Title","Email","Role"}{for _,bad:=range []string{""," \t ","bad\x00control",string([]byte{255}),strings.Repeat("x",321)}{d:=good(1);reflect.ValueOf(&d.Rows[0]).Elem().FieldByName(field).SetString(bad);if c,e:=NewDirectoryNavigation(d);e==nil||c!=nil{t.Fatalf("accepted row %s",field)}}};d:=good(101);if c,e:=NewDirectoryNavigation(d);e==nil||c!=nil{t.Fatal("accepted 101 rows")};for _,target:=range []string{"add","edit"}{for _,bad:=range []string{"","#","relative","//evil.example/x","/%2fevil","/\\evil","/%5cevil","/%00","/x?q=%0a","javascript:alert(1)","http://example.com","https://","https://u:p@example.com","https://example.com:bad","https://example.com:0"," https://example.com","https://example.com/#fragment"}{d:=good(1);if target=="add"{d.AddURL=bad}else{d.Rows[0].EditURL=bad};if c,e:=NewDirectoryNavigation(d);e==nil||c!=nil{t.Fatalf("accepted %s URL %q",target,bad)}}};for _,valid:=range []string{"/new","/new?q=a%20b#part","https://example.com/path?q=a","https://example.com:443/path"}{d:=good(1);d.AddURL=valid;d.Rows[0].EditURL=valid;_ = render(t,d)}}
`
