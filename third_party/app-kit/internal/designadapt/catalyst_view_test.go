package designadapt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestCatalystViewStandalonePolicy compiles the current first-party model and
// view companions in a fresh, offline module. Its settingsForm is an explicit
// authored test stub; it is not evidence for the private complete-form markup.
func TestCatalystViewStandalonePolicy(t *testing.T) {
	model, err := os.ReadFile("catalyst_model.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	view, err := os.ReadFile("catalyst_view.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":       "module example.test/converted\n\ngo 1.26.2\n\nrequire github.com/a-h/templ v0.3.1020\n",
		"go.sum":       "github.com/a-h/templ v0.3.1020 h1:ypAT/L5ySWEnZ6Zft/5yfoWXYYkhFNvEFOeeqecg4tw=\ngithub.com/a-h/templ v0.3.1020/go.mod h1:A2DlK61v+K+NRoGnhmYbNYVmtYHcFO5/AisMvBdDxTM=\n",
		"settings.go":  string(model),
		"view.go":      string(view),
		"view_stub.go": catalystSettingsViewStub,
		"view_test.go": catalystSettingsViewTests,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-count=1", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone Catalyst view test: %v\n%s", err, output)
	}
}

const catalystSettingsViewStub = `package converted

import (
 "context"
 "fmt"
 "io"

 "github.com/a-h/templ"
)

// Explicit authored test stub only; it does not stand in for the private form.
func settingsForm(data SettingsView) templ.Component {
 return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
  if err:=ctx.Err(); err!=nil{return err}
  _,err:=fmt.Fprintf(w,"id=%s action=%s reset=%s csrf=%s focus=%s status=%s",data.ID,data.Action,data.ResetAction,data.CSRFToken,data.Focus,data.Status)
  return err
 })
}
`

const catalystSettingsViewTests = `package converted

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "html"
 "io"
 "strings"
 "testing"

 "github.com/a-h/templ"
)

type failWriter struct{ err error }
func (w failWriter) Write([]byte)(int,error){return 0,w.err}
func render(t *testing.T,c templ.Component) string { t.Helper(); var b bytes.Buffer; if err:=c.Render(context.Background(),&b);err!=nil{t.Fatal(err)};return b.String() }
func policy(t *testing.T)*SettingsPolicy { t.Helper(); p,err:=NewSettingsPolicy([]SettingsCountry{{Code:"US",Name:"United States",Regions:[]string{"Arizona","Shared"}},{Code:"FR",Name:"France",Regions:[]string{"Shared","Ile-de-France"}}},[]SettingsCurrency{{Code:"USD",Label:"US Dollar"},{Code:"EUR",Label:"Euro"}},SettingsValues{Country:"US",Region:"Arizona",Currency:"USD"});if err!=nil{t.Fatal(err)};return p }
func presentation() SettingsPresentation{return SettingsPresentation{ID:"settings-1",Action:"/settings/save",ResetAction:"/settings/reset",CSRFToken:"csrf",Focus:"name",ErrorClass:"field-error"}}

func TestViewPreflightAndLocalActions(t *testing.T){
 p:=policy(t); v:=SettingsValues{Country:"US",Region:"Arizona",Currency:"USD"}
 if _,err:=NewSettingsForm(nil,v,presentation());err==nil{t.Fatal("nil policy accepted")}
 bad:=presentation();bad.CSRFToken=string([]byte{0xff});if _,err:=NewSettingsForm(p,v,bad);err==nil{t.Fatal("lossy presentation accepted")}
 for _,action:=range []string{"https://example.test/x","//example.test/x","/a\\b","/a%2fb","/a?x=1","/a#x","/a\x00"}{ bad:=presentation();bad.Action=action;if _,err:=NewSettingsForm(p,v,bad);err==nil{t.Fatalf("unsafe action accepted: %q",action)} }
 for _,mutate:=range []func(*SettingsPresentation){func(x *SettingsPresentation){x.ID="1bad"},func(x *SettingsPresentation){x.CSRFToken=""},func(x *SettingsPresentation){x.Focus="unknown"},func(x *SettingsPresentation){x.Status=strings.Repeat("x",1025)},func(x *SettingsPresentation){x.ErrorClass=strings.Repeat("x",2049)}} { bad:=presentation();mutate(&bad);if _,err:=NewSettingsForm(p,v,bad);err==nil{t.Fatal("invalid bounded presentation accepted")} }
 c,err:=NewSettingsForm(p,v,presentation());if err!=nil{t.Fatal(err)};out:=render(t,c);for _,want:=range []string{"action=/settings/save","reset=/settings/reset","csrf=csrf","focus=name"}{if !strings.Contains(out,want){t.Fatalf("stub did not receive checked presentation: %q",want)}}
 retry:=presentation();retry.Focus="status";retry.Status="Draft retained. Try again.";c,err=NewSettingsForm(p,v,retry);if err!=nil{t.Fatal(err)};if !strings.Contains(render(t,c),"focus=status"){t.Fatal("host-selected retry status focus lost")}
}

func TestViewRetentionCountFocusAndErrors(t *testing.T){
 p:=policy(t); over:=strings.Repeat("\uFFFD",241);in:=SettingsValues{Bio:over,Country:"US",Region:"Arizona",Currency:"USD"};x:=presentation();x.ShowErrors=true;x.Focus="reset";x.Status="Saved"
 data,err:=newSettingsView(p,in,x);if err!=nil{t.Fatal(err)};if data.Values.Bio!=over||data.Focus!="bio"||data.Status!=""||!data.HasError("bio"){t.Fatalf("retained draft/focus/status: %#v",data)}
 if got:=render(t,settingsBioCount(SettingsView{Values:SettingsValues{Bio:strings.Repeat("\uFFFD",240)}}));got!="240 / 240 Unicode code points"{t.Fatalf("code point count: %q",got)}
 multiline:=strings.Repeat("x",119)+"\r\n"+strings.Repeat("x",120)
 if got:=render(t,settingsBioCount(SettingsView{Values:SettingsValues{Bio:multiline}}));got!="240 / 240 Unicode code points"{t.Fatalf("CRLF textarea count: %q",got)}
 if got:=settingsBioText(SettingsView{Values:SettingsValues{Bio:"\nfirst\r\nlast\n"}});got!="\n\nfirst\r\nlast\n"{t.Fatalf("textarea sacrificial newline lost retained draft: %q",got)}
 bio:=render(t,settingsError(data,"bio"));email:=render(t,settingsError(data,"email"));if !strings.Contains(bio,"id=\"settings-1-bio-error\"")||strings.Contains(bio," hidden")||!strings.Contains(email,"id=\"settings-1-email-error\"")||!strings.Contains(email," hidden"){t.Fatalf("separate error IDs/hidden state: %q %q",bio,email)}
}

func TestNativeOptionsRetentionEscapingAndIsolation(t *testing.T){
 p:=policy(t); data:=SettingsView{SettingsPresentation:presentation(),Values:SettingsValues{Country:"US",Region:"Shared",Currency:"USD"},policy:p}
 regions:=render(t,settingsOptions(data,"region"));if strings.Count(regions,"<optgroup")!=2||strings.Count(regions," selected")!=1||!strings.Contains(regions,"data-p10-country=\"US\""){t.Fatalf("native grouped regions/single selection: %q",regions)}
 data.Values.Region="Missing";regions=render(t,settingsOptions(data,"region"));if strings.Count(regions,"value=\"Missing\"")!=1||strings.Count(regions," selected")!=1||!strings.Contains(regions,">Missing (invalid)</option>"){t.Fatalf("unknown draft was not retained exactly once: %q",regions)}
 escaped,err:=NewSettingsPolicy([]SettingsCountry{{Code:"US",Name:"<country>",Regions:[]string{"<region>"}}},[]SettingsCurrency{{Code:"USD",Label:"<currency>"}},SettingsValues{Country:"US",Region:"<region>",Currency:"USD"});if err!=nil{t.Fatal(err)}
 evil:=SettingsView{SettingsPresentation:SettingsPresentation{ID:"f",ErrorClass:"x\"><script>",Status:"<status>"},Values:SettingsValues{Country:"US",Region:"<region>",Currency:"USD"},policy:escaped,errors:map[string]string{"bio":"<error>"}}
 out:=render(t,settingsOptions(evil,"region"))+render(t,settingsOptions(evil,"currency"))+render(t,settingsError(evil,"bio"))+render(t,settingsStatus(evil));if strings.Contains(out,"<country>")||strings.Contains(out,"<region>")||strings.Contains(out,"<currency>")||strings.Contains(out,"<error>")||strings.Contains(out,"<status>")||strings.Contains(out,"<script>"){t.Fatalf("unescaped presentation/configuration: %q",out)}
 snapshot:=p.Countries();snapshot[0].Name="changed";snapshot[0].Regions[0]="changed";again:=render(t,settingsOptions(SettingsView{Values:SettingsValues{Country:"US",Region:"Arizona",Currency:"USD"},policy:p},"region"));if strings.Contains(again,"changed"){t.Fatal("configuration snapshot mutation leaked")}
}

func TestComponentContextAndWriterErrors(t *testing.T){
 p:=policy(t);data:=SettingsView{SettingsPresentation:presentation(),Values:SettingsValues{Country:"US",Region:"Arizona",Currency:"USD"},policy:p}
 boom:=errors.New("writer");for _,c:=range []templ.Component{settingsViewText("x"),settingsOptions(data,"country"),settingsError(data,"bio")} {if err:=c.Render(context.Background(),failWriter{boom});!errors.Is(err,boom){t.Fatalf("writer error: %v",err)}}
 ctx,cancel:=context.WithCancel(context.Background());cancel();if err:=settingsOptions(data,"country").Render(ctx,io.Discard);!errors.Is(err,context.Canceled){t.Fatalf("context error: %v",err)}
}

func TestEnhancementConfigUsesInitialEscapedSnapshot(t *testing.T){
 p:=policy(t);initial:=p.Initial();initial.Name="</template><script>alert(1)</script>&\"\u2028\u2029";initial.Bio="😀e\u0301"
 p,err:=NewSettingsPolicy(p.Countries(),p.Currencies(),initial);if err!=nil{t.Fatal(err)}
 draft:=initial;draft.Name="retained but not reset snapshot";draft.Bio=strings.Repeat("😀",241)
 x:=presentation();data,err:=newSettingsView(p,draft,x);if err!=nil{t.Fatal(err)}
 if got:=render(t,settingsEnhancementConfig(data));got!=""{t.Fatal("native view emitted config")}
 data.Enhance=true;out:=render(t,settingsEnhancementConfig(data))
 if strings.Count(out,"<template ")!=1||strings.Count(out,"</template>")!=1||strings.Contains(out,"<script>")||strings.Contains(out,draft.Name){t.Fatalf("unsafe or draft reset snapshot: %q",out)}
 if !strings.Contains(out,"id=\"settings-1-config\" data-p10-config-for=\"settings-1-form\""){t.Fatal("config instance identity missing")}
 start:=strings.Index(out,">")+1;end:=strings.LastIndex(out,"</template>");wire:=html.UnescapeString(out[start:end])
 var config struct{SchemaVersion int ` + "`json:\"schema_version\"`" + `; BioLimit int ` + "`json:\"bio_limit\"`" + `;Initial map[string]any ` + "`json:\"initial\"`" + `}
 if err:=json.Unmarshal([]byte(wire),&config);err!=nil{t.Fatal(err)}
 if config.SchemaVersion!=1||config.BioLimit!=240||len(config.Initial)!=10||config.Initial["name"]!=initial.Name||config.Initial["bio"]!=initial.Bio||config.Initial["country"]!=initial.Country||config.Initial["email_is_public"]!=initial.PublicEmail{t.Fatalf("initial snapshot roundtrip: %#v",config)}
 if second:=render(t,settingsEnhancementConfig(data));second!=out{t.Fatal("nondeterministic config")}
 boom:=errors.New("config writer");if err:=settingsEnhancementConfig(data).Render(context.Background(),failWriter{boom});!errors.Is(err,boom){t.Fatalf("writer error lost: %v",err)}
 ctx,cancel:=context.WithCancel(context.Background());cancel();if err:=settingsEnhancementConfig(data).Render(ctx,io.Discard);!errors.Is(err,context.Canceled){t.Fatalf("cancellation lost: %v",err)}
}
`
