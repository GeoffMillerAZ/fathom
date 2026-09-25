// Package navigationproof checks one source-specific P06 browser task. It
// validates reported facts, but never executes a report or certifies a target.
package navigationproof

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const (
	MaxDocumentBytes  = 256 << 10
	Task              = "p06-links-input-contrast-v1"
	SourceSHA256      = "01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31"
	ChromeVersion     = "152.0.7977.84"
	PlaywrightVersion = "1.63.0"
)

//go:embed check.go
var checkSource []byte

//go:embed schema.cue
var schema []byte

type CaseSpec struct {
	ID string `json:"id"`
	// Width is the requested Playwright viewport before browser zoom, not window.outerWidth.
	Width int    `json:"width"`
	Theme string `json:"theme"`
	Input string `json:"input"`
	Zoom  int    `json:"zoom"`
}

type Contract struct {
	SchemaVersion     int        `json:"schema_version"`
	Task              string     `json:"task"`
	SourceSHA256      string     `json:"source_sha256"`
	ArtifactSHA256    string     `json:"artifact_sha256"`
	PrepareSHA256     string     `json:"prepare_sha256"`
	BinarySHA256      string     `json:"binary_sha256"`
	CSSSHA256         string     `json:"css_sha256"`
	RunnerSHA256      string     `json:"runner_sha256"`
	OracleSHA256      string     `json:"oracle_sha256"`
	ChromeVersion     string     `json:"chrome_version"`
	PlaywrightVersion string     `json:"playwright_version"`
	Platform          string     `json:"platform"`
	Arch              string     `json:"arch"`
	Origin            string     `json:"origin"`
	Cases             []CaseSpec `json:"cases"`
}

type Control struct {
	Href         string     `json:"href"`
	Label        string     `json:"label"`
	Visible      bool       `json:"visible"`
	Width        float64    `json:"width"`
	Height       float64    `json:"height"`
	ClientWidth  float64    `json:"client_width"`
	ScrollWidth  float64    `json:"scroll_width"`
	NormalFG     [3]float64 `json:"normal_fg"`
	NormalBG     [3]float64 `json:"normal_bg"`
	HoverFG      [3]float64 `json:"hover_fg"`
	HoverBG      [3]float64 `json:"hover_bg"`
	FocusFG      [3]float64 `json:"focus_fg"`
	FocusBG      [3]float64 `json:"focus_bg"`
	OutlineBG    [3]float64 `json:"outline_bg"`
	OutlineRGB   [3]float64 `json:"outline_rgb"`
	OutlineWidth float64    `json:"outline_width"`
	OutlineStyle string     `json:"outline_style"`
	FocusVisible bool       `json:"focus_visible"`
	FocusedHref  string     `json:"focused_href"`
}

type Step struct {
	FromPath        string `json:"from_path"`
	TargetKey       string `json:"target_key"`
	Input           string `json:"input"`
	Path            string `json:"path"`
	Status          int    `json:"status"`
	SpecimenCurrent string `json:"specimen_current"`
	CurrentHref     string `json:"current_href"`
	CurrentCount    int    `json:"current_count"`
	TabsToTarget    int    `json:"tabs_to_target"`
}

type Observation struct {
	Spec          CaseSpec  `json:"spec"`
	InnerWidth    float64   `json:"inner_width"`
	OuterWidth    float64   `json:"outer_width"`
	DPR           float64   `json:"dpr"`
	ViewportScale float64   `json:"viewport_scale"`
	FontsLoaded   bool      `json:"fonts_loaded"`
	ScrollWidth   float64   `json:"scroll_width"`
	Controls      []Control `json:"controls"`
	Steps         []Step    `json:"steps"`
	Back          Step      `json:"back"`
}

type Report struct {
	SchemaVersion     int           `json:"schema_version"`
	ContractSHA256    string        `json:"contract_sha256"`
	ChromeVersion     string        `json:"chrome_version"`
	PlaywrightVersion string        `json:"playwright_version"`
	Cases             []Observation `json:"cases"`
	Errors            []string      `json:"errors"`
	ForbiddenRequests []string      `json:"forbidden_requests"`
}

type Result struct {
	Status       string   `json:"status"`
	Verification string   `json:"verification"`
	Findings     []string `json:"findings"`
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// OracleSHA256 binds the exact checker and CUE schema source used by this build.
func OracleSHA256() string {
	body := make([]byte, 0, len(checkSource)+1+len(schema))
	body = append(body, checkSource...)
	body = append(body, 0)
	body = append(body, schema...)
	return digest(body)
}

func EncodeContract(contract Contract) ([]byte, error) {
	body, err := encode(contract, &Contract{}, "#Contract")
	if err != nil {
		return nil, err
	}
	if err := validateContract(contract); err != nil {
		return nil, err
	}
	return body, nil
}

func EncodeReport(report Report) ([]byte, error) {
	return encode(report, &Report{}, "#Report")
}

// Check validates exact bindings first, then derives pass/fail from observed
// task facts. Both outcomes remain explicitly not verified target support.
func Check(expectedContractSHA string, contractBody, reportBody []byte) (Result, error) {
	if !validHash(expectedContractSHA) || digest(contractBody) != expectedContractSHA {
		return Result{}, fmt.Errorf("navigationproof: contract differs from expected revision")
	}
	var contract Contract
	if err := decode(contractBody, &contract, "#Contract"); err != nil {
		return Result{}, err
	}
	if err := validateContract(contract); err != nil {
		return Result{}, err
	}
	var report Report
	if err := decode(reportBody, &report, "#Report"); err != nil {
		return Result{}, err
	}
	if report.ContractSHA256 != expectedContractSHA || report.ChromeVersion != contract.ChromeVersion ||
		report.PlaywrightVersion != contract.PlaywrightVersion {
		return Result{}, fmt.Errorf("navigationproof: stale contract or toolchain binding")
	}

	findings := map[string]struct{}{}
	add := func(format string, args ...any) { findings[fmt.Sprintf(format, args...)] = struct{}{} }
	for _, message := range report.Errors {
		add("browser error: %s", message)
	}
	for _, request := range report.ForbiddenRequests {
		add("forbidden request: %s", request)
	}
	expected := requiredCases()
	expectedByID := make(map[string]CaseSpec, len(expected))
	for _, spec := range expected {
		expectedByID[spec.ID] = spec
	}
	seen := map[string]bool{}
	observedByID := map[string]Observation{}
	for _, observation := range report.Cases {
		spec, ok := expectedByID[observation.Spec.ID]
		if !ok {
			add("case %s: unexpected", observation.Spec.ID)
			continue
		}
		if seen[observation.Spec.ID] {
			add("case %s: duplicate", observation.Spec.ID)
		} else {
			observedByID[observation.Spec.ID] = observation
		}
		seen[observation.Spec.ID] = true
		checkObservation(observation, spec, add)
	}
	for _, spec := range expected {
		if !seen[spec.ID] {
			add("case %s: missing", spec.ID)
		}
	}
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"light", "dark"} {
			for _, input := range []string{"keyboard", "pointer", "touch"} {
				one, oneOK := observedByID[fmt.Sprintf("%d-%s-%s-1", width, theme, input)]
				two, twoOK := observedByID[fmt.Sprintf("%d-%s-%s-2", width, theme, input)]
				if oneOK && twoOK && (one.OuterWidth != two.OuterWidth || one.InnerWidth != two.InnerWidth*2 || two.DPR != one.DPR*2) {
					add("%d-%s-%s: zoom pair geometry differs", width, theme, input)
				}
			}
		}
	}
	ordered := make([]string, 0, len(findings))
	for finding := range findings {
		ordered = append(ordered, finding)
	}
	sort.Strings(ordered)
	status := "passed"
	if len(ordered) != 0 {
		status = "failed"
	}
	return Result{Status: status, Verification: "not-verified", Findings: ordered}, nil
}

func validateContract(contract Contract) error {
	if contract.Task != Task || contract.SourceSHA256 != SourceSHA256 ||
		contract.OracleSHA256 != OracleSHA256() || contract.ChromeVersion != ChromeVersion ||
		contract.PlaywrightVersion != PlaywrightVersion || contract.Platform != "darwin" || contract.Arch != "arm64" {
		return fmt.Errorf("navigationproof: contract identity differs")
	}
	u, err := url.Parse(contract.Origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" || u.String() != contract.Origin {
		return fmt.Errorf("navigationproof: origin is not an exact loopback origin")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != u.Port() {
		return fmt.Errorf("navigationproof: origin port is not canonical")
	}
	want := requiredCases()
	if len(contract.Cases) != len(want) {
		return fmt.Errorf("navigationproof: contract must contain exact cases")
	}
	for i := range want {
		if contract.Cases[i] != want[i] {
			return fmt.Errorf("navigationproof: contract case %d differs", i)
		}
	}
	return nil
}

func requiredCases() []CaseSpec {
	cases := make([]CaseSpec, 0, 24)
	for _, width := range []int{390, 1440} {
		for _, theme := range []string{"light", "dark"} {
			for _, input := range []string{"keyboard", "pointer", "touch"} {
				for _, zoom := range []int{1, 2} {
					cases = append(cases, CaseSpec{ID: fmt.Sprintf("%d-%s-%s-%d", width, theme, input, zoom), Width: width, Theme: theme, Input: input, Zoom: zoom})
				}
			}
		}
	}
	return cases
}

func checkObservation(o Observation, spec CaseSpec, add func(string, ...any)) {
	prefix := "case " + spec.ID
	if o.Spec != spec {
		add("%s: axes differ", prefix)
	}
	if !finite(o.InnerWidth, o.OuterWidth, o.DPR, o.ViewportScale, o.ScrollWidth) ||
		o.InnerWidth != float64(spec.Width)/float64(spec.Zoom) || o.OuterWidth <= 0 || o.OuterWidth < o.InnerWidth ||
		o.DPR != float64(spec.Zoom) || o.ViewportScale != 1 {
		add("%s: zoom geometry differs", prefix)
	}
	if !o.FontsLoaded {
		add("%s: fonts not loaded", prefix)
	}
	if o.ScrollWidth < 0 || o.ScrollWidth > o.InnerWidth {
		add("%s: page overflows", prefix)
	}
	if len(o.Controls) != 4 {
		add("%s: expected four controls", prefix)
	}
	for index, control := range o.Controls {
		if index >= 4 {
			add("%s: extra control", prefix)
			continue
		}
		path := pagePath(spec.Theme, index+1)
		if control.Href != path {
			add("%s control %d: href differs", prefix, index+1)
		}
		if !utf8.ValidString(control.Label) || len(control.Label) > 256 || strings.TrimSpace(control.Label) == "" {
			add("%s control %d: invalid label", prefix, index+1)
		}
		if !control.Visible {
			add("%s control %d: hidden", prefix, index+1)
		}
		if !finite(control.Width, control.Height, control.ClientWidth, control.ScrollWidth, control.OutlineWidth) ||
			control.Width < 44 || control.Height < 44 {
			add("%s control %d: target below 44px", prefix, index+1)
		}
		if control.ClientWidth < 0 || control.ScrollWidth < 0 || control.ScrollWidth > control.ClientWidth+1 {
			add("%s control %d: label overflows", prefix, index+1)
		}
		if !validColor(control.NormalFG) || !validColor(control.NormalBG) || contrast(control.NormalFG, control.NormalBG) < 4.5 {
			add("%s control %d: normal contrast below 4.5", prefix, index+1)
		}
		if !validColor(control.HoverFG) || !validColor(control.HoverBG) || contrast(control.HoverFG, control.HoverBG) < 4.5 {
			add("%s control %d: hover contrast below 4.5", prefix, index+1)
		}
		if !validColor(control.FocusFG) || !validColor(control.FocusBG) || contrast(control.FocusFG, control.FocusBG) < 4.5 {
			add("%s control %d: focus contrast below 4.5", prefix, index+1)
		}
		if !validColor(control.OutlineRGB) || !validColor(control.OutlineBG) || contrast(control.OutlineRGB, control.OutlineBG) < 3 ||
			control.OutlineWidth < 2 || control.OutlineStyle != "solid" {
			add("%s control %d: focus indicator differs", prefix, index+1)
		}
		if !control.FocusVisible || control.FocusedHref != control.Href {
			add("%s control %d: focus target differs", prefix, index+1)
		}
	}
	fromPages := []int{3, 1, 2, 3}
	if len(o.Steps) != 4 {
		add("%s: expected four forward steps", prefix)
	}
	for index, step := range o.Steps {
		if index >= 4 {
			add("%s: extra forward step", prefix)
			continue
		}
		checkStep(prefix, step, spec, fromPages[index], index+1, false, add)
	}
	checkStep(prefix, o.Back, spec, 4, 3, true, add)
}

func checkStep(prefix string, step Step, spec CaseSpec, fromPage, targetPage int, back bool, add func(string, ...any)) {
	wantInput, wantStatus, wantTabs := spec.Input, 200, 0
	if spec.Input == "keyboard" {
		wantTabs = 1
	}
	if back {
		wantInput, wantStatus, wantTabs = "history", step.Status, 0
	}
	validStatus := step.Status == wantStatus
	if back {
		validStatus = step.Status == 0 || step.Status == 200
	}
	if step.FromPath != pagePath(spec.Theme, fromPage) || step.TargetKey != pageKey(targetPage) ||
		step.Input != wantInput || step.Path != pagePath(spec.Theme, targetPage) || !validStatus ||
		step.SpecimenCurrent != pageKey(targetPage) || step.CurrentHref != pagePath(spec.Theme, targetPage) ||
		step.CurrentCount != 1 || (spec.Input == "keyboard" && !back && (step.TabsToTarget < 1 || step.TabsToTarget > 8)) ||
		(spec.Input != "keyboard" && !back && step.TabsToTarget != wantTabs) || (back && step.TabsToTarget != 0) {
		add("%s: %s step to %s differs", prefix, map[bool]string{true: "back", false: "forward"}[back], pageKey(targetPage))
	}
}

func pageKey(page int) string                { return fmt.Sprintf("page-%d", page) }
func pagePath(theme string, page int) string { return fmt.Sprintf("/%s/links/page-%d", theme, page) }

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func validColor(color [3]float64) bool {
	return finite(color[:]...) && color[0] >= 0 && color[0] <= 255 && color[1] >= 0 && color[1] <= 255 && color[2] >= 0 && color[2] <= 255
}

func luminance(color [3]float64) float64 {
	channel := func(value float64) float64 {
		value /= 255
		if value <= .04045 {
			return value / 12.92
		}
		return math.Pow((value+.055)/1.055, 2.4)
	}
	return .2126*channel(color[0]) + .7152*channel(color[1]) + .0722*channel(color[2])
}

func contrast(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + .05) / (lb + .05)
}

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func encode(value, target any, definition string) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := decode(body, target, definition); err != nil {
		return nil, err
	}
	return body, nil
}

func decode(body []byte, target any, definition string) error {
	if len(body) > MaxDocumentBytes || !utf8.Valid(body) || !json.Valid(body) {
		return fmt.Errorf("navigationproof: invalid JSON encoding or metadata size")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("navigationproof: invalid typed JSON: %w", err)
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(body, canonical) {
		return fmt.Errorf("navigationproof: noncanonical JSON")
	}
	ctx := cuecontext.New()
	value := ctx.CompileBytes(body).Unify(ctx.CompileBytes(schema).LookupPath(cue.ParsePath(definition)))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("navigationproof: %s schema rejected: %w", definition, err)
	}
	return nil
}
