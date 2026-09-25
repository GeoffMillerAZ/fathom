package designadapt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalystModelStandalonePolicy(t *testing.T) {
	model, err := os.ReadFile("catalyst_model.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/converted\n\ngo 1.26.2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.go"), model, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings_test.go"), []byte(catalystStandaloneTests), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-count=1", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("standalone Catalyst policy test: %v\n%s", err, output)
	}
}

const catalystStandaloneTests = `package converted

import (
 "net/url"
 "strings"
 "testing"
)

func policy(t *testing.T) *SettingsPolicy {
 t.Helper()
 countries := []SettingsCountry{{Code:"US", Name:"United States", Regions:[]string{"Arizona", "Oregon"}}, {Code:"FR", Name:"France", Regions:[]string{"Ile-de-France"}}}
 currencies := []SettingsCurrency{{Code:"USD", Label:"US Dollar"}, {Code:"EUR", Label:"Euro"}}
 initial := SettingsValues{Country:"US", Region:"Arizona", Currency:"USD", Email:"person@example.test"}
 p, err := NewSettingsPolicy(countries, currencies, initial); if err != nil { t.Fatal(err) }; return p
}
func validate(t *testing.T, p *SettingsPolicy, v SettingsValues) SettingsValidation {
 t.Helper(); got, err := p.Validate(v); if err != nil { t.Fatal(err) }; return got
}
func TestUnicodeLimitAndRetention(t *testing.T) {
 p := policy(t)
 for _, bio := range []string{strings.Repeat("😀", 240), strings.Repeat("e\u0301", 120), strings.Repeat("👩\u200d💻", 80), strings.Repeat("👍🏽", 120), strings.Repeat("\uFFFD", 240)} {
  if got := validate(t, p, SettingsValues{Bio:bio, Country:"US", Region:"Arizona", Currency:"USD"}); got.FirstError != "" { t.Fatalf("240 code points rejected: %#v", got) }
 }
 over := strings.Repeat("😀", 241); got := validate(t, p, SettingsValues{Bio:over, Country:"US", Region:"Arizona", Currency:"USD"})
 if got.FirstError != "bio" || got.Values.Bio != over { t.Fatalf("over-limit bio was not retained: %#v", got) }
 exact := strings.Repeat("x",119)+"\r\n"+strings.Repeat("x",120)
 if got := validate(t,p,SettingsValues{Bio:exact,Country:"US",Region:"Arizona",Currency:"USD"});got.FirstError!=""||got.Values.Bio!=exact { t.Fatalf("240 textarea code points rejected or normalized: %#v",got) }
 overLines := strings.Repeat("x",120)+"\r\n"+strings.Repeat("x",120)
 if got := validate(t,p,SettingsValues{Bio:overLines,Country:"US",Region:"Arizona",Currency:"USD"});got.FirstError!="bio"||got.Values.Bio!=overLines { t.Fatalf("241 textarea code points accepted or normalized: %#v",got) }
 for raw,want := range map[string]int{"\r":1,"\n":1,"\r\n":1,"\r\r\n":2} { if got:=settingsBioCodePoints(raw);got!=want { t.Fatalf("textarea newline count %q=%d, want %d",raw,got,want) } }
}
func TestWireBoundsAndLossyData(t *testing.T) {
 if _, err := ParseSettingsValues(url.Values{"name":{string([]byte{0xff})}}); err == nil { t.Fatal("malformed wire accepted") }
 if _, err := ParseSettingsValues(url.Values{"bio":{"\uFFFD"}}); err != nil { t.Fatalf("literal replacement character rejected: %v", err) }
 if _, err := ParseSettingsValues(url.Values{"name":{"x\x00"}}); err == nil { t.Fatal("control wire accepted") }
 if _, err := ParseSettingsValues(url.Values{"bio":{"one\ntwo\tthree\rfour"}}); err != nil { t.Fatal(err) }
 if _, err := ParseSettingsValues(url.Values{"bio":{strings.Repeat("x", 16<<10+1)}}); err == nil { t.Fatal("oversized bio accepted") }
 if _, err := ParseSettingsValues(url.Values{strings.Repeat("k", 32<<10+1):{"x"}}); err == nil { t.Fatal("oversized request accepted") }
 if _, err := ParseSettingsValues(url.Values{"city":{strings.Repeat("x", 256+1)}}); err == nil { t.Fatal("oversized field accepted") }
 if _, err := ParseSettingsValues(url.Values{"unknown":{"x"}}); err == nil { t.Fatal("unknown field accepted") }
 if _, err := ParseSettingsValues(url.Values{"country[code]":{"US", "FR"}}); err == nil { t.Fatal("duplicate field accepted") }
 v, err := ParseSettingsValues(url.Values{"email_is_public":{"on"}}); if err != nil || !v.PublicEmail { t.Fatalf("checkbox mismatch: %#v %v", v, err) }
 v, err = ParseSettingsValues(url.Values{}); if err != nil || v.PublicEmail { t.Fatalf("absent checkbox mismatch: %#v %v", v, err) }
}
func TestMembershipAndFocusOrder(t *testing.T) {
 p := policy(t)
 bad := validate(t, p, SettingsValues{Bio:strings.Repeat("x", 241), Email:"Name <person@example.test>", Country:"ZZ", Region:"Elsewhere", Currency:"GBP"})
 if bad.FirstError != "bio" || len(bad.Errors) != 5 { t.Fatalf("focus order or errors mismatch: %#v", bad) }
 bad = validate(t, p, SettingsValues{Email:"Name <person@example.test>", Country:"ZZ", Region:"Elsewhere", Currency:"GBP"})
 if bad.FirstError != "email" { t.Fatalf("email did not follow bio: %#v", bad) }
 if got := p.RegionAfterCountryChange("US", "Oregon"); got != "Oregon" { t.Fatalf("member reset lost: %q", got) }
 if got := p.RegionAfterCountryChange("FR", "Oregon"); got != "" { t.Fatalf("region auto-chosen/retained: %q", got) }
 if got := p.RegionAfterCountryChange("ZZ", "Arizona"); got != "" { t.Fatalf("unknown country retained region: %q", got) }
 if got := validate(t, p, SettingsValues{Country:"US", Region:"Arizona", Currency:"USD", Email:"person@example.test"}); got.FirstError != "" { t.Fatalf("valid values rejected: %#v", got) }
 if got := validate(t, p, SettingsValues{Country:"US", Region:"Arizona", Currency:"USD", Email:""}); got.FirstError != "" { t.Fatalf("empty optional email rejected: %#v", got) }
 for _, v := range []SettingsValues{{Name:string([]byte{0xff}), Country:"US", Region:"Arizona", Currency:"USD"}, {City:strings.Repeat("x", 257), Country:"US", Region:"Arizona", Currency:"USD"}} {
  got, err := p.Validate(v); if err == nil || got.Values != v || len(got.Errors) != 0 || got.FirstError != "" { t.Fatalf("direct framing error lost draft: %#v %v", got, err) }
 }
}
func TestConfigurationAndSnapshotIsolation(t *testing.T) {
 countries := []SettingsCountry{{Code:"US", Name:"United States", Regions:[]string{"Arizona"}}}
 currencies := []SettingsCurrency{{Code:"USD", Label:"Dollar"}}
 initial := SettingsValues{Country:"US", Region:"Arizona", Currency:"USD"}
 p, err := NewSettingsPolicy(countries, currencies, initial); if err != nil { t.Fatal(err) }
 countries[0].Code, countries[0].Regions[0], currencies[0].Code, initial.Region = "ZZ", "Elsewhere", "EUR", "Elsewhere"
 snapshot := p.Countries(); snapshot[0].Code, snapshot[0].Regions[0] = "ZZ", "Elsewhere"
 if got := p.Currencies(); got[0].Code != "USD" { t.Fatalf("currency mutation leaked: %#v", got) }
 if p.Initial().Region != "Arizona" || validate(t, p, SettingsValues{Country:"US", Region:"Arizona", Currency:"USD"}).FirstError != "" { t.Fatal("configuration mutation leaked") }
 for _, tc := range []struct{ countries []SettingsCountry; currencies []SettingsCurrency }{
  {[]SettingsCountry{{Code:"US", Name:"US", Regions:[]string{"Arizona"}}, {Code:"US", Name:"US 2", Regions:[]string{"Oregon"}}}, []SettingsCurrency{{Code:"USD", Label:"Dollar"}}},
  {[]SettingsCountry{{Code:"US", Name:"US", Regions:[]string{"Arizona", "Arizona"}}}, []SettingsCurrency{{Code:"USD", Label:"Dollar"}}},
  {[]SettingsCountry{{Code:"US", Name:"US", Regions:[]string{"Arizona"}}}, []SettingsCurrency{{Code:"USD", Label:"Dollar"}, {Code:"USD", Label:"Dollar 2"}}},
  {[]SettingsCountry{{Code:"US", Name:"US"}}, []SettingsCurrency{{Code:"USD", Label:"Dollar"}}},
 } { if _, err := NewSettingsPolicy(tc.countries, tc.currencies, SettingsValues{Country:"US", Currency:"USD"}); err == nil { t.Fatal("duplicate option accepted") } }
}
`
