package designconvert

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyRuleDigestBeforeElements = "0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c"
const elementsRuleDigestV1 = "2cfaf2a258aa4be609834e6453e796effd7d8d68ee227ed3b596e1b5e0e450e5"

func elementsSource(body []byte, member string) Source {
	return Source{
		ArchiveID: "application-ui@" + strings.Repeat("a", 64),
		Path:      member,
		SHA256:    digest(body),
		Size:      len(body),
	}
}

func TestTranslateElementsSyntheticFamilyReplayAndLiteralOutput(t *testing.T) {
	body := []byte(`<el-disclosure id="faq" class="one  two"><button command="--toggle" commandfor="faq">Question</button><div>Answer &amp; more</div></el-disclosure><!--kept--><button command="show-modal" commandfor="dialog">Open</button><el-dialog><dialog id="dialog"><el-dialog-panel><el-dropdown><button>Account</button><el-menu anchor="bottom end"><a href="/profile">Profile</a></el-menu></el-dropdown><button command="close" commandfor="dialog">Close</button></el-dialog-panel></dialog></el-dialog>`)
	result, err := TranslateElements(elementsSource(body, "html/synthetic-elements.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != ElementsSyntaxKind || result.Status != "translated" || result.Target != "go-templ" ||
		result.Verification != "not-verified" || result.Artifact == nil {
		t.Fatalf("unexpected Elements receipt: kind=%q status=%q target=%q verification=%q", result.Kind, result.Status, result.Target, result.Verification)
	}
	if result.Transformer.ID != ElementsRuleID || result.Transformer.SHA256 != elementsRuleDigest() {
		t.Fatal("translation lost the versioned Elements rule identity")
	}
	wantRuntime := ElementsRuntime{
		Package: ElementsRuntimePackage, Version: ElementsRuntimeVersion,
		ESMSHA256: ElementsRuntimeESMSHA256, TGZSHA256: ElementsRuntimeTGZSHA256,
		Delivery: ElementsRuntimeDelivery,
	}
	if result.Runtime != wantRuntime {
		t.Fatalf("unexpected runtime metadata: %#v", result.Runtime)
	}
	for _, literal := range []string{
		`<el-disclosure id={ "faq" } class={ "one  two" }>`,
		`<el-menu anchor={ "bottom end" }>`,
		`{ "Answer & more" }`,
		`<!--kept-->`,
	} {
		if !strings.Contains(result.Artifact.Content, literal) {
			t.Errorf("generated literal missing %q", literal)
		}
	}
	if len(result.Artifact.Mappings) == 0 || result.Artifact.SHA256 != digest([]byte(result.Artifact.Content)) {
		t.Fatal("translation lost byte mappings or output identity")
	}
	if err := VerifyElementsReplay(result, body); err != nil {
		t.Fatal(err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	provePilotRenderedDOM(t, filepath.Clean(filepath.Join(workingDir, "../..")), result.Artifact.Content, string(body))
	second, err := TranslateElements(elementsSource(body, "html/synthetic-elements.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(result)
	secondJSON, _ := json.Marshal(second)
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("Elements translation is nondeterministic")
	}
}

func TestTranslateElementsRefusesOutsideBoundedGrammarWithoutOutput(t *testing.T) {
	tests := map[string]string{
		"other custom element":  `<el-tabs></el-tabs>`,
		"script":                `<el-menu><script></script></el-menu>`,
		"style":                 `<el-menu style="display:block"></el-menu>`,
		"event":                 `<el-menu oncommand="alert(1)"></el-menu>`,
		"expression namespace":  `<el-menu data-on:click="danger"></el-menu>`,
		"anchor on other tag":   `<el-dropdown anchor="bottom"></el-dropdown>`,
		"anchor expression":     `<el-menu anchor="bottom calc(1)"></el-menu>`,
		"anchor punctuation":    `<el-menu anchor="bottom-end"></el-menu>`,
		"duplicate attribute":   `<el-menu class="one" class="two"></el-menu>`,
		"unquoted attribute":    `<el-menu class=one></el-menu>`,
		"uppercase custom":      `<EL-MENU></EL-MENU>`,
		"self closing custom":   `<el-menu/>`,
		"browser tree repair":   `<el-menu><p><div></div></p></el-menu>`,
		"no Elements element":   `<div>native only</div>`,
		"missing commandfor":    `<el-disclosure id="x"><button command="--toggle">Open</button></el-disclosure>`,
		"missing command":       `<el-disclosure id="x"><button commandfor="x">Open</button></el-disclosure>`,
		"command on link":       `<el-disclosure id="x"><a command="--toggle" commandfor="x">Open</a></el-disclosure>`,
		"unsupported command":   `<el-disclosure id="x"><button command="remove" commandfor="x">Open</button></el-disclosure>`,
		"missing target":        `<el-menu><button command="--toggle" commandfor="x">Open</button></el-menu>`,
		"wrong target kind":     `<el-menu id="x"><button command="--toggle" commandfor="x">Open</button></el-menu>`,
		"wrong dialog action":   `<el-disclosure id="x"><button command="close" commandfor="x">Close</button></el-disclosure>`,
		"duplicate target ID":   `<el-disclosure id="x"></el-disclosure><el-disclosure id="x"><button command="--toggle" commandfor="x">Open</button></el-disclosure>`,
		"remote command target": `<el-disclosure><button command="--toggle" commandfor="https://example.test/x">Open</button></el-disclosure>`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			body := []byte(source)
			result, err := TranslateElements(elementsSource(body, "html/refusal.html"), body)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "unsupported" || len(result.Diagnostics) == 0 || result.Artifact != nil {
				t.Fatalf("outside grammar produced output: status=%q diagnostics=%d artifact=%v", result.Status, len(result.Diagnostics), result.Artifact != nil)
			}
			if err := VerifyElementsReplay(result, body); err != nil {
				t.Fatalf("unsupported assessment must replay exactly: %v", err)
			}
		})
	}
}

func TestElementsReplayRejectsTamperingIncludingRehashedOutputAndRuntime(t *testing.T) {
	body := []byte(`<el-menu anchor="bottom end"><span class="stable">Item</span></el-menu>`)
	base, err := TranslateElements(elementsSource(body, "html/menu.html"), body)
	if err != nil || base.Artifact == nil {
		t.Fatalf("fixture translation failed: %v", err)
	}
	mutations := map[string]func(*ElementsTranslation){
		"kind":                  func(r *ElementsTranslation) { r.Kind = "other" },
		"rule":                  func(r *ElementsTranslation) { r.Transformer.SHA256 = strings.Repeat("0", 64) },
		"runtime package":       func(r *ElementsTranslation) { r.Runtime.Package = "other" },
		"runtime version":       func(r *ElementsTranslation) { r.Runtime.Version = "1.0.23" },
		"runtime ESM":           func(r *ElementsTranslation) { r.Runtime.ESMSHA256 = strings.Repeat("0", 64) },
		"runtime tarball":       func(r *ElementsTranslation) { r.Runtime.TGZSHA256 = strings.Repeat("0", 64) },
		"runtime delivery":      func(r *ElementsTranslation) { r.Runtime.Delivery = "embedded" },
		"invented verification": func(r *ElementsTranslation) { r.Verification = "verified" },
		"source map":            func(r *ElementsTranslation) { r.Artifact.Mappings = []Mapping{} },
		"changed output rehashed": func(r *ElementsTranslation) {
			r.Artifact.Content = strings.ReplaceAll(r.Artifact.Content, "stable", "changed")
			r.Artifact.SHA256 = digest([]byte(r.Artifact.Content))
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			result := base
			artifact := *base.Artifact
			artifact.Mappings = append([]Mapping{}, base.Artifact.Mappings...)
			result.Artifact = &artifact
			mutate(&result)
			if err := VerifyElementsReplay(result, body); err == nil {
				t.Fatal("tampered Elements receipt passed exact replay")
			}
		})
	}
}

func TestElementsReceiptBoundsRunBeforeCUE(t *testing.T) {
	body := []byte(`<el-menu></el-menu>`)
	base, err := TranslateElements(elementsSource(body, "html/menu.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ElementsTranslation){
		"runtime.package": func(r *ElementsTranslation) { r.Runtime.Package = strings.Repeat("a", maxElementsRuntimeBytes+1) },
		"kind":            func(r *ElementsTranslation) { r.Kind = strings.Repeat("a", maxElementsKindBytes+1) },
		"diagnostics":     func(r *ElementsTranslation) { r.Diagnostics = make([]Diagnostic, MaxDiagnostics+1) },
		"artifact.content": func(r *ElementsTranslation) {
			r.Status = "translated"
			r.Diagnostics = nil
			r.Artifact = &Artifact{Content: strings.Repeat("a", maxOutputBytes+1)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := base
			mutate(&result)
			err := validateElementsShape(result)
			if err == nil || !strings.HasPrefix(err.Error(), "designconvert: Elements receipt limit: "+name+" exceeds ") {
				t.Fatalf("receipt did not stop at pre-CUE bound: %v", err)
			}
		})
	}
}

func TestElementsTemplGuardIsLiteralOnly(t *testing.T) {
	for name, source := range map[string]string{
		"call":               "package converted\n\ntempl Specimen() { @Other() }\n",
		"raw HTML":           "package converted\n\ntempl Specimen() { @templ.Raw(\"<b>x</b>\") }\n",
		"expression":         "package converted\n\ntempl Specimen() { { dangerous() } }\n",
		"unknown custom tag": "package converted\n\ntempl Specimen() { <el-tabs></el-tabs> }\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateElementsTempl(source); err == nil {
				t.Fatal("literal AST guard accepted executable or unknown output")
			}
		})
	}
}

func TestLegacyRuleDigestAndCustomElementRefusalRemainUnchanged(t *testing.T) {
	if got := elementsRuleDigest(); got != elementsRuleDigestV1 {
		t.Fatalf("Elements v1 rule digest changed: got %s want %s", got, elementsRuleDigestV1)
	}
	if got := ruleDigest(); got != legacyRuleDigestBeforeElements {
		t.Fatalf("legacy rule bytes changed: got %s want %s", got, legacyRuleDigestBeforeElements)
	}
	body := []byte(`<el-menu><button>Open</button></el-menu>`)
	result, err := Translate(elementsSource(body, "html/legacy-refusal.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "unsupported" || result.Artifact != nil || len(result.Diagnostics) == 0 {
		t.Fatal("legacy translator no longer refuses Elements custom tags")
	}
}

func TestTranslateElementsFrozenSourcesLive(t *testing.T) {
	root := os.Getenv("APPKIT_ELEMENTS_LIVE_ROOT")
	if root == "" {
		t.Skip("set APPKIT_ELEMENTS_LIVE_ROOT to the absolute app-kit or retained Elements reference root")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("APPKIT_ELEMENTS_LIVE_ROOT must be absolute")
	}
	reference := filepath.Join(root, "data/design-assets/reference-assets/elements-1.0.22-pilot")
	if _, err := os.Stat(filepath.Join(reference, "manifest.json")); err != nil {
		reference = root
	}
	manifestBody, err := os.ReadFile(filepath.Join(reference, "manifest.json"))
	if err != nil {
		t.Fatal("private Elements manifest unavailable")
	}
	var manifest struct {
		Runtime struct {
			Package        string `json:"package"`
			Version        string `json:"version"`
			ElementsSHA256 string `json:"elements_sha256"`
			ArchiveSHA256  string `json:"archive_sha256"`
		} `json:"runtime"`
		Cases []struct {
			Source     Source `json:"source"`
			SourceFile string `json:"source_file"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatal("private Elements manifest invalid")
	}
	if manifest.Runtime.Package != ElementsRuntimePackage || manifest.Runtime.Version != ElementsRuntimeVersion ||
		manifest.Runtime.ElementsSHA256 != ElementsRuntimeESMSHA256 || manifest.Runtime.ArchiveSHA256 != ElementsRuntimeTGZSHA256 ||
		len(manifest.Cases) != 4 {
		t.Fatal("private Elements denominator or runtime pin changed")
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	kitRoot := filepath.Clean(filepath.Join(workingDir, "../.."))
	for _, test := range manifest.Cases {
		t.Run(filepath.Base(test.Source.Path), func(t *testing.T) {
			if filepath.Base(test.SourceFile) != test.SourceFile || !strings.HasSuffix(test.SourceFile, "-original.html") {
				t.Fatal("private manifest source file is not a bounded original member")
			}
			body, err := os.ReadFile(filepath.Join(reference, test.SourceFile))
			if err != nil || len(body) != test.Source.Size || digest(body) != test.Source.SHA256 {
				t.Fatal("private original is missing or differs from its manifest; body omitted")
			}
			result, err := TranslateElements(test.Source, body)
			if err != nil || result.Status != "translated" || result.Artifact == nil {
				codes := map[string]int{}
				for _, diagnostic := range result.Diagnostics {
					codes[diagnostic.Code]++
				}
				t.Fatalf("private Elements syntax translation failed; source/output omitted; diagnostic_codes=%v operational_error=%v", codes, err != nil)
			}
			if err := VerifyElementsReplay(result, body); err != nil {
				t.Fatal("private Elements replay failed; source/output omitted")
			}
			provePilotRenderedDOM(t, kitRoot, result.Artifact.Content, string(body))
		})
	}
}
