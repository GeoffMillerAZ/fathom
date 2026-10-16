package styleguide

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/tokens"
)

func draftFixture(t *testing.T) EngineOptions {
	t.Helper()
	baseline := tokens.Tokens{Name: "synthetic", Description: "Synthetic app-owned tokens",
		Fonts:  tokens.Fonts{Sans: "system-ui, sans-serif", Mono: "ui-monospace, monospace"},
		Radius: tokens.Radius{SM: "2px", MD: "4px", LG: "8px", XL: "12px", Full: "9999px"},
		Colors: map[string]tokens.ColorPair{}}
	for _, role := range tokens.ColorNames {
		baseline.Colors[role] = tokens.ColorPair{Light: "#345678", Dark: "#abcdef"}
	}
	baseline.Colors["surface"] = tokens.ColorPair{Light: "#ffffff", Dark: "#000000"}
	baseline.Colors["ink"] = tokens.ColorPair{Light: "#000000", Dark: "#ffffff"}
	baseline.Colors["app-brand"] = tokens.ColorPair{Light: "#123456", Dark: "#fedcba"}
	return EngineOptions{Source: draftSource(t, baseline), SourceLabel: "app/tokens.cue", KitRevision: "synthetic-revision-1",
		Fonts: map[string]string{"system": "system-ui, sans-serif", "serif": "Georgia, serif", "mono": "ui-monospace, monospace"},
		Checks: []ContrastCheck{{ID: "body-light", Foreground: "ink", Background: "surface", Scheme: "light", Minimum: 4.5},
			{ID: "body-dark", Foreground: "ink", Background: "surface", Scheme: "dark", Minimum: 4.5}}}
}

func draftSource(t *testing.T, value tokens.Tokens) []byte {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Comments and an expression must survive original.cue, not be reconstructed.
	return []byte("// Exact original: retain this comment.\nlet unit = \"px\"\ntokens: " +
		strings.Replace(string(body), `"sm": "2px"`, `"sm": "2" + unit`, 1) + "\n")
}

func draftEngine(t *testing.T, options EngineOptions) *Engine {
	t.Helper()
	e, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func draftCandidate(t *testing.T, e *Engine, edits map[string]string) Candidate {
	t.Helper()
	c, err := e.Prepare(edits)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDraftFieldsAndDefensiveCopies(t *testing.T) {
	options := draftFixture(t)
	original := bytes.Clone(options.Source)
	e := draftEngine(t, options)
	before := draftCandidate(t, e, nil)
	if !before.Exportable || len(before.Checks) != 2 || len(before.Changes) != 0 {
		t.Fatal("synthetic baseline should pass exactly the configured checks")
	}
	fields := e.Fields()
	if len(fields) != (len(tokens.ColorNames)+1)*2+8 || !sort.SliceIsSorted(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path }) {
		t.Fatal("fields must cover all schemes, app roles, fonts and radii in sorted order")
	}
	unchanged := make(map[string]string, len(fields))
	for _, field := range fields {
		unchanged[field.Path] = field.Value
		if field.Path == "fonts.display" && (field.Value != "" || field.Kind != "font") {
			t.Fatal("absent display must remain an explicit empty inherited field")
		}
	}
	if c := draftCandidate(t, e, unchanged); !reflect.DeepEqual(c, before) {
		t.Fatal("posting unchanged literals changed the baseline")
	}
	options.Source[0] = '!'
	options.Fonts["serif"] = "changed externally"
	options.Checks[0].Minimum = 21
	fields[0].Value = "changed externally"
	fonts := e.Fonts()
	fonts["serif"] = "changed externally"
	if e.Fonts()["serif"] != "Georgia, serif" || e.SourceSHA256() != digest(original) ||
		e.SourceLabel() != "app/tokens.cue" || e.KitRevision() != "synthetic-revision-1" {
		t.Fatal("engine exposed or retained mutable caller-owned options")
	}
	before.CUE[0], before.CSS[0] = '!', '!'
	before.Checks[0].Passed = false
	if after := draftCandidate(t, e, nil); bytes.Equal(after.CUE, before.CUE) || !after.Checks[0].Passed {
		t.Fatal("candidate data aliases the engine baseline")
	}
}

func TestDraftColorRadiusFontsAndDisplayFallback(t *testing.T) {
	e := draftEngine(t, draftFixture(t))
	edits := map[string]string{"colors.app-brand.light": "#AbC", "radius.md": ".500rem", "radius.sm": "0px",
		"radius.lg": "100rem", "radius.xl": "9999px", "fonts.sans": "serif", "fonts.display": "mono"}
	c := draftCandidate(t, e, edits)
	loaded, err := tokens.Load(c.CUE)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Colors["app-brand"].Light != "#abc" || loaded.Radius.MD != "0.5rem" || loaded.Radius.SM != "0px" ||
		loaded.Fonts.Sans != "Georgia, serif" || loaded.Fonts.Display != "ui-monospace, monospace" {
		t.Fatal("bounded edits were not canonicalized or resolved through approved fonts")
	}
	if !sort.SliceIsSorted(c.Changes, func(i, j int) bool { return c.Changes[i].Path < c.Changes[j].Path }) {
		t.Fatal("diff is not sorted")
	}
	foundDisplay := false
	for _, change := range c.Changes {
		if change.Path == "fonts.display" {
			foundDisplay = change.Before == "" && change.After == "ui-monospace, monospace"
		}
	}
	if !foundDisplay {
		t.Fatal("semantic diff dropped display typography")
	}
	if !bytes.Equal(c.CSS, []byte(tokens.EmitThemeCSSOpts(loaded, tokens.EmitOptions{DarkVariant: tokens.DarkVariantNone}))) {
		t.Fatal("candidate CSS does not equal existing emitter output")
	}
	baseline, err := tokens.Load(draftFixture(t).Source)
	if err != nil {
		t.Fatal(err)
	}
	baseline.Fonts.Display = "Georgia, serif"
	options := draftFixture(t)
	options.Source = draftSource(t, baseline)
	reset := draftCandidate(t, draftEngine(t, options), map[string]string{"fonts.display": ""})
	if !reflect.DeepEqual(reset.Changes, []Change{{Path: "fonts.display", Before: "Georgia, serif", After: ""}}) ||
		!bytes.Contains(reset.CSS, []byte("--ak-font-display: system-ui, sans-serif;")) || bytes.Contains(reset.CUE, []byte(`"display"`)) {
		t.Fatal("display reset did not preserve absent-field inheritance")
	}
	inherited := draftCandidate(t, e, map[string]string{"fonts.sans": "serif"})
	if len(inherited.Changes) != 1 || !bytes.Contains(inherited.CSS, []byte("--ak-font-display: Georgia, serif;")) {
		t.Fatal("sans edit lost display inheritance or invented a display literal")
	}
}

func TestDraftRejectsUnknownInjectionAndOversizedEdits(t *testing.T) {
	e := draftEngine(t, draftFixture(t))
	for _, edits := range []map[string]string{
		{"colors.missing.light": "#000"}, {"tokens": "{}"}, {"colors.ink.light.extra": "#000"},
		{"colors.ink.light": "#000; color:red"}, {"colors.ink.light": "url(https://example.com)"},
		{"colors.ink.light": " #000"}, {"colors.ink.light": "#0000"}, {"colors.ink.light": "oklch(0 0 0)"},
		{"radius.md": "-1px"}, {"radius.md": "10000px"}, {"radius.md": "100.01rem"},
		{"radius.md": "NaNpx"}, {"radius.md": "Infrem"}, {"radius.md": "1e2px"},
		{"radius.md": "calc(1rem)"}, {"radius.md": "1px; background:url(x)"}, {"radius.md": "1%"},
		{"fonts.sans": "Georgia, serif"}, {"fonts.sans": "unknown"}, {"fonts.sans": ""},
		{"fonts.mono": ""}, {"fonts.display": "</style><script>bad</script>"},
		{"colors.ink.light": strings.Repeat("a", maxValue+1)}, {"colors.ink.light": "\xff"},
	} {
		if _, err := e.Prepare(edits); err == nil {
			t.Errorf("accepted unsupported edit: %v", edits)
		}
	}
	edits := make(map[string]string, maxEdits+1)
	for i := range maxEdits + 1 {
		edits[fmt.Sprint(i)] = "#000"
	}
	if _, err := e.Prepare(edits); err == nil {
		t.Fatal("accepted too many edits")
	}
	if c := draftCandidate(t, e, nil); len(c.Changes) != 0 || !c.Exportable {
		t.Fatal("refused edits changed the baseline")
	}
}

func TestDraftConstructorRefusals(t *testing.T) {
	tests := map[string]func(*EngineOptions){
		"empty source":       func(o *EngineOptions) { o.Source = nil },
		"invalid CUE":        func(o *EngineOptions) { o.Source = []byte("tokens: {") },
		"incomplete CUE":     func(o *EngineOptions) { o.Source = []byte("tokens: {name: string}") },
		"oversized source":   func(o *EngineOptions) { o.Source = bytes.Repeat([]byte(" "), maxSource+1) },
		"empty label":        func(o *EngineOptions) { o.SourceLabel = "" },
		"metadata injection": func(o *EngineOptions) { o.KitRevision = "revision\nother" },
		"unknown variant":    func(o *EngineOptions) { o.DarkVariant = "class" },
		"no checks":          func(o *EngineOptions) { o.Checks = nil },
		"duplicate IDs":      func(o *EngineOptions) { o.Checks[1].ID = o.Checks[0].ID },
		"unknown role":       func(o *EngineOptions) { o.Checks[0].Foreground = "missing" },
		"unknown scheme":     func(o *EngineOptions) { o.Checks[0].Scheme = "system" },
		"zero floor":         func(o *EngineOptions) { o.Checks[0].Minimum = 0 },
		"large floor":        func(o *EngineOptions) { o.Checks[0].Minimum = 22 },
		"NaN floor":          func(o *EngineOptions) { o.Checks[0].Minimum = math.NaN() },
		"infinite floor":     func(o *EngineOptions) { o.Checks[0].Minimum = math.Inf(1) },
		"font ID injection":  func(o *EngineOptions) { o.Fonts["</style>"] = "serif" },
		"oversized font":     func(o *EngineOptions) { o.Fonts["serif"] = strings.Repeat("x", maxValue+1) },
		"many fonts": func(o *EngineOptions) {
			for i := range 33 {
				o.Fonts[fmt.Sprintf("font-%d", i)] = "serif"
			}
		},
		"many checks": func(o *EngineOptions) { o.Checks = make([]ContrastCheck, 65) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			options := draftFixture(t)
			mutate(&options)
			if _, err := NewEngine(options); err == nil {
				t.Fatal("invalid options accepted")
			}
		})
	}
	options := draftFixture(t)
	baseline, err := tokens.Load(options.Source)
	if err != nil {
		t.Fatal(err)
	}
	for i := range maxColors + 1 {
		baseline.Colors[fmt.Sprintf("extra-%d", i)] = tokens.ColorPair{Light: "#000", Dark: "#fff"}
	}
	options.Source = draftSource(t, baseline)
	if _, err := NewEngine(options); err == nil {
		t.Fatal("too many colors accepted")
	}
}

func TestDraftUnknownAndNonfiniteChecksCannotExport(t *testing.T) {
	for _, value := range []string{"var(--brand)", "oklch(NaN 0 0)", "oklch(1 0 0 / NaN)", "oklch(1 0 0 / +Inf)",
		"oklch(0 0 0", "oklch(0 0 0) trailing", "oklch(0 0 0));color:red", "oklch(1e999 0 0)", "oklch(0 0 1e999)",
		"oklch(0 0 0 / 1e999)", "oklch(0, 0, 0)", "oklch(0 -1 0)", "#0000"} {
		t.Run(value, func(t *testing.T) {
			options := draftFixture(t)
			baseline, err := tokens.Load(options.Source)
			if err != nil {
				t.Fatal(err)
			}
			pair := baseline.Colors["ink"]
			pair.Light = value
			baseline.Colors["ink"] = pair
			options.Source = draftSource(t, baseline)
			e := draftEngine(t, options)
			c := draftCandidate(t, e, nil)
			if c.Exportable {
				t.Fatal("unknown contrast became an exportable pass")
			}
			for _, check := range c.Checks {
				if !finite(check.Ratio) {
					t.Fatal("nonfinite result leaked into candidate metadata")
				}
				if check.Scheme == "light" && (check.Passed || check.Error == "") {
					t.Fatal("unknown check has no explicit failure")
				}
			}
			if _, err := e.Export(nil, c.ReviewSHA256); err == nil {
				t.Fatal("unknown checks exported")
			}
		})
	}
	e := draftEngine(t, draftFixture(t))
	edits := map[string]string{"colors.ink.light": "#ffffff"}
	c := draftCandidate(t, e, edits)
	if c.Exportable {
		t.Fatal("1:1 contrast passed")
	}
	if _, err := e.Export(edits, c.ReviewSHA256); err == nil {
		t.Fatal("failed check exported")
	}
}

func TestDraftCheckedColorSupportedForms(t *testing.T) {
	for _, value := range []string{"#000", "#000000", "#000000ff", "oklch(0 0 0)", "oklch(0% 0% 0deg / 100%)",
		"OKLCH(0e0 .0 -360deg / 1)", "oklch( 0 0 0 )"} {
		t.Run(value, func(t *testing.T) {
			options := draftFixture(t)
			baseline, err := tokens.Load(options.Source)
			if err != nil {
				t.Fatal(err)
			}
			pair := baseline.Colors["ink"]
			pair.Light = value
			baseline.Colors["ink"] = pair
			options.Source = draftSource(t, baseline)
			c := draftCandidate(t, draftEngine(t, options), nil)
			if !c.Exportable {
				t.Fatal("supported finite color was not measured", c.Checks)
			}
		})
	}
}

func TestDraftExportReplayOriginalAndReceiptAgreement(t *testing.T) {
	options := draftFixture(t)
	e := draftEngine(t, options)
	a := map[string]string{"radius.md": "0.5rem", "colors.app-brand.light": "#ABC", "fonts.sans": "serif"}
	b := make(map[string]string)
	for _, path := range []string{"fonts.sans", "colors.app-brand.light", "radius.md"} {
		b[path] = a[path]
	}
	c := draftCandidate(t, e, a)
	if other := draftCandidate(t, e, b); !reflect.DeepEqual(c, other) {
		t.Fatal("edit order changed the candidate")
	}
	first, err := e.Export(a, c.ReviewSHA256)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.Export(b, c.ReviewSHA256)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("export is not byte-repeatable", err)
	}
	if len(first) > maxBundle {
		t.Fatal("bundle exceeded declared ceiling")
	}
	archive, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 5 {
		t.Fatal("unexpected bundle members")
	}
	files := map[string][]byte{}
	for _, file := range archive.File {
		if file.Mode().Perm() != 0600 || !file.Modified.Equal(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatal("archive mode or timestamp is not fixed", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = body
	}
	if !bytes.Equal(files["original.cue"], options.Source) || !bytes.Contains(files["original.cue"], []byte(`"2" + unit`)) {
		t.Fatal("original comments or expressions were rewritten")
	}
	if !bytes.Equal(files["candidate.cue"], c.CUE) || !bytes.Equal(files["candidate.css"], c.CSS) || digest(files["receipt.json"]) != c.ReviewSHA256 {
		t.Fatal("archive is not bound to the reviewed candidate")
	}
	loaded, err := tokens.Load(files["candidate.cue"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["candidate.css"], []byte(tokens.EmitThemeCSSOpts(loaded, tokens.EmitOptions{DarkVariant: tokens.DarkVariantNone}))) {
		t.Fatal("exported CUE and CSS disagree")
	}
	var receipt struct {
		SourceLabel string                  `json:"source_label"`
		KitRevision string                  `json:"kit_revision"`
		DarkVariant tokens.DarkVariant      `json:"dark_variant"`
		Files       map[string]fileIdentity `json:"files"`
		Definitions []ContrastCheck         `json:"check_definitions"`
		Checks      []CheckResult           `json:"checks"`
		Exportable  bool                    `json:"exportable"`
	}
	if err := json.Unmarshal(files["receipt.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	if !receipt.Exportable || !reflect.DeepEqual(receipt.Checks, c.Checks) {
		t.Fatal("receipt changed selected checks")
	}
	if receipt.SourceLabel != options.SourceLabel || receipt.KitRevision != options.KitRevision ||
		receipt.DarkVariant != tokens.DarkVariantNone || len(receipt.Files) != 4 || !reflect.DeepEqual(receipt.Definitions, e.checks) {
		t.Fatal("receipt does not record the exact source, revision, strategy and check definitions")
	}
	for name, pin := range receipt.Files {
		if pin.SHA256 != digest(files[name]) || pin.Size != len(files[name]) {
			t.Fatal("receipt identity mismatch", name)
		}
	}
	var diff []Change
	if err := json.Unmarshal(files["diff.json"], &diff); err != nil || !reflect.DeepEqual(diff, c.Changes) {
		t.Fatal("diff disagreement", err)
	}
	if _, err := e.Export(a, c.SHA256); err == nil {
		t.Fatal("candidate-only hash bypassed review binding")
	}
	if _, err := e.Export(map[string]string{"radius.md": "9px"}, c.ReviewSHA256); err == nil {
		t.Fatal("stale edit review accepted")
	}
}

func TestDraftReviewDigestBindsSourceRevisionCSSAndChecks(t *testing.T) {
	options := draftFixture(t)
	baseline := draftCandidate(t, draftEngine(t, options), nil)
	mutations := map[string]func(*EngineOptions){
		"source comments": func(o *EngineOptions) {
			o.Source = append(bytes.Clone(o.Source), []byte("// different source bytes\n")...)
		},
		"source label":   func(o *EngineOptions) { o.SourceLabel = "other/tokens.cue" },
		"kit revision":   func(o *EngineOptions) { o.KitRevision = "synthetic-revision-2" },
		"CSS strategy":   func(o *EngineOptions) { o.DarkVariant = tokens.DarkVariantAttribute },
		"minimum floor":  func(o *EngineOptions) { o.Checks[0].Minimum = 7 },
		"check identity": func(o *EngineOptions) { o.Checks[0].ID = "other-check" },
		"check set":      func(o *EngineOptions) { o.Checks = o.Checks[:1] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := draftFixture(t)
			mutate(&changed)
			e := draftEngine(t, changed)
			c := draftCandidate(t, e, nil)
			if c.SHA256 != baseline.SHA256 || c.ReviewSHA256 == baseline.ReviewSHA256 {
				t.Fatal("review did not bind metadata independently from CUE bytes")
			}
			if _, err := e.Export(nil, baseline.ReviewSHA256); err == nil {
				t.Fatal("stale review binding exported")
			}
		})
	}
	reordered := draftFixture(t)
	reordered.Checks[0], reordered.Checks[1] = reordered.Checks[1], reordered.Checks[0]
	if got := draftCandidate(t, draftEngine(t, reordered), nil); got.ReviewSHA256 != baseline.ReviewSHA256 {
		t.Fatal("check ordering is not deterministic")
	}
}
