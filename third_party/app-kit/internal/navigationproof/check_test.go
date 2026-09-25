package navigationproof

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func validContract() Contract {
	hash := func(text string) string { return digest([]byte(text)) }
	return Contract{SchemaVersion: 1, Task: Task, SourceSHA256: SourceSHA256,
		ArtifactSHA256: hash("artifact"), PrepareSHA256: hash("prepare"), BinarySHA256: hash("binary"),
		CSSSHA256: hash("css"), RunnerSHA256: hash("runner"), OracleSHA256: OracleSHA256(),
		ChromeVersion: ChromeVersion, PlaywrightVersion: PlaywrightVersion,
		Platform: "darwin", Arch: "arm64", Origin: "http://127.0.0.1:18780", Cases: requiredCases()}
}

func validReport(t *testing.T, contract Contract) ([]byte, Report) {
	t.Helper()
	contractBody, err := EncodeContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	report := Report{SchemaVersion: 1, ContractSHA256: digest(contractBody), ChromeVersion: ChromeVersion,
		PlaywrightVersion: PlaywrightVersion, Cases: []Observation{}, Errors: []string{}, ForbiddenRequests: []string{}}
	for _, spec := range contract.Cases {
		black, white := [3]float64{0, 0, 0}, [3]float64{255, 255, 255}
		outer := 1440.0
		if spec.Width == 390 {
			outer = 500
		}
		o := Observation{Spec: spec, InnerWidth: float64(spec.Width) / float64(spec.Zoom),
			OuterWidth: outer, DPR: float64(spec.Zoom), ViewportScale: 1,
			FontsLoaded: true, ScrollWidth: float64(spec.Width) / float64(spec.Zoom), Controls: []Control{}, Steps: []Step{}}
		for page := 1; page <= 4; page++ {
			path := pagePath(spec.Theme, page)
			o.Controls = append(o.Controls, Control{Href: path, Label: "Page " + string(rune('0'+page)), Visible: true,
				Width: 80, Height: 44, ClientWidth: 78, ScrollWidth: 78,
				NormalFG: black, NormalBG: white, HoverFG: black, HoverBG: white,
				FocusFG: black, FocusBG: white, OutlineBG: white, OutlineRGB: black, OutlineWidth: 2,
				OutlineStyle: "solid", FocusVisible: true, FocusedHref: path})
		}
		from := []int{3, 1, 2, 3}
		for index, page := range []int{1, 2, 3, 4} {
			tabs := 0
			if spec.Input == "keyboard" {
				tabs = 2
			}
			o.Steps = append(o.Steps, validStep(spec, from[index], page, spec.Input, 200, tabs))
		}
		o.Back = validStep(spec, 4, 3, "history", 200, 0)
		report.Cases = append(report.Cases, o)
	}
	return contractBody, report
}

func validStep(spec CaseSpec, from, target int, input string, status, tabs int) Step {
	return Step{FromPath: pagePath(spec.Theme, from), TargetKey: pageKey(target), Input: input,
		Path: pagePath(spec.Theme, target), Status: status, SpecimenCurrent: pageKey(target),
		CurrentHref: pagePath(spec.Theme, target), CurrentCount: 1, TabsToTarget: tabs}
}

func encodeReport(t *testing.T, report Report) []byte {
	t.Helper()
	body, err := EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func requireFailedFacts(t *testing.T, mutate func(*Report)) Result {
	t.Helper()
	contractBody, report := validReport(t, validContract())
	mutate(&report)
	result, err := Check(digest(contractBody), contractBody, encodeReport(t, report))
	if err != nil {
		t.Fatalf("task fact returned structural error: %v", err)
	}
	if result.Status != "failed" || result.Verification != "not-verified" || len(result.Findings) == 0 {
		t.Fatalf("invalid task facts accepted: %#v", result)
	}
	return result
}

func TestCheckDerivesPassedFactsWithoutCertification(t *testing.T) {
	contractBody, report := validReport(t, validContract())
	result, err := Check(digest(contractBody), contractBody, encodeReport(t, report))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || result.Verification != "not-verified" || len(result.Findings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for left, right := 0, len(report.Cases)-1; left < right; left, right = left+1, right-1 {
		report.Cases[left], report.Cases[right] = report.Cases[right], report.Cases[left]
	}
	result, err = Check(digest(contractBody), contractBody, encodeReport(t, report))
	if err != nil || result.Status != "passed" {
		t.Fatalf("case set depended on report order: %#v, %v", result, err)
	}
}

func TestCheckRejectsAdversarialTaskFacts(t *testing.T) {
	for name, mutate := range map[string]func(*Report){
		"deleted case":      func(r *Report) { r.Cases = r.Cases[1:] },
		"duplicate case":    func(r *Report) { r.Cases = append(r.Cases, r.Cases[0]) },
		"unexpected case":   func(r *Report) { o := r.Cases[0]; o.Spec.ID = "unexpected"; r.Cases = append(r.Cases, o) },
		"wrong axes":        func(r *Report) { r.Cases[0].Spec.Width = 1440 },
		"zoom lie":          func(r *Report) { r.Cases[0].InnerWidth++ },
		"DPR lie":           func(r *Report) { r.Cases[0].DPR = 1.5 },
		"pinch zoom":        func(r *Report) { r.Cases[0].ViewportScale = 2 },
		"outer below inner": func(r *Report) { r.Cases[0].OuterWidth = r.Cases[0].InnerWidth - 1 },
		"outer changes in pair": func(r *Report) {
			r.Cases[1].OuterWidth++
		},
		"font failure":        func(r *Report) { r.Cases[0].FontsLoaded = false },
		"page overflow":       func(r *Report) { r.Cases[0].ScrollWidth = r.Cases[0].InnerWidth + 2 },
		"hidden target":       func(r *Report) { r.Cases[0].Controls[0].Visible = false },
		"small target":        func(r *Report) { r.Cases[0].Controls[0].Height = 43 },
		"wrapped overflow":    func(r *Report) { r.Cases[0].Controls[0].ScrollWidth = r.Cases[0].Controls[0].ClientWidth + 2 },
		"blank label":         func(r *Report) { r.Cases[0].Controls[0].Label = " \t" },
		"long UTF8 label":     func(r *Report) { r.Cases[0].Controls[0].Label = strings.Repeat("é", 129) },
		"low normal contrast": func(r *Report) { r.Cases[0].Controls[0].NormalFG = [3]float64{250, 250, 250} },
		"low hover contrast":  func(r *Report) { r.Cases[0].Controls[0].HoverFG = [3]float64{250, 250, 250} },
		"low focus contrast":  func(r *Report) { r.Cases[0].Controls[0].FocusFG = [3]float64{250, 250, 250} },
		"missing focus":       func(r *Report) { r.Cases[0].Controls[0].FocusVisible = false },
		"weak outline":        func(r *Report) { r.Cases[0].Controls[0].OutlineWidth = 1 },
		"outline lost outside anchor": func(r *Report) {
			r.Cases[0].Controls[0].FocusFG = [3]float64{255, 255, 255}
			r.Cases[0].Controls[0].FocusBG = [3]float64{0, 0, 0}
			r.Cases[0].Controls[0].OutlineRGB = [3]float64{255, 255, 255}
			r.Cases[0].Controls[0].OutlineBG = [3]float64{255, 255, 255}
		},
		"wrong current":     func(r *Report) { r.Cases[0].Steps[0].SpecimenCurrent = "page-2" },
		"wrong input":       func(r *Report) { r.Cases[0].Steps[0].Input = "pointer" },
		"wrong status":      func(r *Report) { r.Cases[0].Steps[0].Status = 404 },
		"missing step":      func(r *Report) { r.Cases[0].Steps = r.Cases[0].Steps[:3] },
		"wrong back":        func(r *Report) { r.Cases[0].Back.CurrentHref = pagePath("light", 4) },
		"browser error":     func(r *Report) { r.Errors = []string{"page crashed"} },
		"forbidden request": func(r *Report) { r.ForbiddenRequests = []string{"https://example.invalid/"} },
	} {
		t.Run(name, func(t *testing.T) { requireFailedFacts(t, mutate) })
	}
}

func TestContractAndReportBindingsFailClosed(t *testing.T) {
	contract := validContract()
	contractBody, report := validReport(t, contract)
	reportBody := encodeReport(t, report)
	for name, expected := range map[string]string{"empty": "", "stale": digest([]byte("stale"))} {
		t.Run(name, func(t *testing.T) {
			result, err := Check(expected, contractBody, reportBody)
			if err == nil || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("bad expected pin accepted: %#v, %v", result, err)
			}
		})
	}
	report.ContractSHA256 = digest([]byte("other contract"))
	if result, err := Check(digest(contractBody), contractBody, encodeReport(t, report)); err == nil || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("stale report binding accepted: %#v, %v", result, err)
	}
	for name, mutate := range map[string]func(*Contract){
		"oracle":       func(c *Contract) { c.OracleSHA256 = digest([]byte("old oracle")) },
		"origin":       func(c *Contract) { c.Origin = "http://127.0.0.1:018780" },
		"missing case": func(c *Contract) { c.Cases = c.Cases[1:] },
		"extra case":   func(c *Contract) { c.Cases = append(c.Cases, c.Cases[0]) },
		"case axis":    func(c *Contract) { c.Cases[0].Width = 1440 },
	} {
		t.Run(name, func(t *testing.T) {
			c := validContract()
			mutate(&c)
			body, err := jsonMarshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := Check(digest(body), body, reportBody); err == nil || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("bad contract accepted: %#v, %v", result, err)
			}
		})
	}
}

func TestCanonicalWireAndOracleIdentity(t *testing.T) {
	contractBody, report := validReport(t, validContract())
	reportBody := encodeReport(t, report)
	if !bytes.HasPrefix(contractBody, []byte(`{"schema_version":1,"task":"p06-links-input-contrast-v1","source_sha256":"`)) {
		t.Fatalf("contract field order changed: %.100s", contractBody)
	}
	casePrefix := []byte(`"cases":[{"spec":{"id":"390-light-keyboard-1","width":390,"theme":"light","input":"keyboard","zoom":1},"inner_width":390`)
	if !bytes.Contains(reportBody, casePrefix) {
		t.Fatalf("observation field order changed: %.300s", reportBody)
	}
	checkBody, err := os.ReadFile("check.go")
	if err != nil {
		t.Fatal(err)
	}
	schemaBody, err := os.ReadFile("schema.cue")
	if err != nil {
		t.Fatal(err)
	}
	wantOracle := digest(append(append(append([]byte{}, checkBody...), 0), schemaBody...))
	if OracleSHA256() != wantOracle {
		t.Fatalf("oracle identity differs: got %s want %s", OracleSHA256(), wantOracle)
	}

	for name, body := range map[string][]byte{
		"malformed":     contractBody[:len(contractBody)-1],
		"case alias":    bytes.Replace(contractBody, []byte(`"schema_version"`), []byte(`"SCHEMA_VERSION"`), 1),
		"duplicate":     bytes.Replace(contractBody, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"lossy Unicode": bytes.Replace(contractBody, []byte(`"task":"p06-links-input-contrast-v1"`), []byte(`"task":"\ud800"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if result, err := Check(digest(body), body, reportBody); err == nil || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("noncanonical contract accepted: %#v, %v", result, err)
			}
		})
	}
	for name, body := range map[string][]byte{
		"report alias":         bytes.Replace(reportBody, []byte(`"contract_sha256"`), []byte(`"CONTRACT_SHA256"`), 1),
		"report duplicate":     bytes.Replace(reportBody, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"report lossy Unicode": bytes.Replace(reportBody, []byte(`"label":"Page 1"`), []byte(`"label":"\ud800"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if result, err := Check(digest(contractBody), contractBody, body); err == nil || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("noncanonical report accepted: %#v, %v", result, err)
			}
		})
	}
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
