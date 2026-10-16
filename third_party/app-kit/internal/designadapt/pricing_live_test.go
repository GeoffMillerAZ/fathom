package designadapt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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

// TestAdaptPricingLive is an opt-in, read-only purchase probe. Licensed source
// and generated derivatives exist only in private test memory and t.TempDir.
// Subprocess output is withheld because compiler diagnostics may quote source.
func TestAdaptPricingLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to the existing absolute private app-kit checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("APPKIT_PROBE_CONVERSION_KIT_ROOT must be absolute")
	}
	designRoot := filepath.Join(root, "data/design-assets")
	_, archiveSHA, ok := strings.Cut(PricingArchive, "@")
	if !ok || len(archiveSHA) != sha256.Size*2 {
		t.Fatal("frozen P02 archive identity is invalid")
	}
	identityPaths := []string{
		filepath.Join(designRoot, "catalog.json"),
		filepath.Join(designRoot, "archives", archiveSHA+".zip"),
	}
	identities := make(map[string]string, len(identityPaths))
	for _, path := range identityPaths {
		identity, err := pricingFileSHA256(path)
		if err != nil {
			t.Fatal("private P02 catalog or archive could not be hashed")
		}
		identities[path] = identity
	}
	if identities[identityPaths[1]] != archiveSHA {
		t.Fatal("private P02 archive bytes do not match its identity")
	}
	t.Cleanup(func() {
		for _, path := range identityPaths {
			identity, err := pricingFileSHA256(path)
			if err != nil || identity != identities[path] {
				t.Error("private P02 catalog or archive changed during the read-only probe")
			}
		}
	})
	file, err := designassets.ReadFile(designRoot, PricingArchive, PricingMember)
	if err != nil || file.ArchiveID != PricingArchive || file.Path != PricingMember || file.SHA256 != PricingSourceSHA {
		t.Fatal("frozen P02 source verification failed; source and error bodies omitted")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	first, err := AdaptPricing(source, []byte(file.Content))
	if err != nil {
		t.Fatal("P02 adaptation failed; source and generated bodies omitted")
	}
	second, err := AdaptPricing(source, []byte(file.Content))
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("P02 adaptation is not repeatable")
	}
	if first.SchemaVersion != 1 || first.Adapter != "p02-pricing-v1" || first.Source != source || first.Syntax.ID != designconvert.RuleID || first.Syntax.SHA256 != first.SyntaxSHA256 || first.Verification != "not-verified" || first.Templ == "" || first.Model == "" || first.CSS == "" || first.CSSSHA256 != pricingBytesSHA256([]byte(first.CSS)) || len(first.Defaults) == 0 {
		t.Fatal("P02 adaptation returned an invalid evidence boundary")
	}
	if err := VerifyPricingReplay(first, []byte(file.Content)); err != nil {
		t.Fatal("unchanged P02 adaptation did not replay")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*PricingArtifact)
	}{
		{"templ", func(a *PricingArtifact) { a.Templ += " " }},
		{"model", func(a *PricingArtifact) { a.Model += "\n" }},
		{"css", func(a *PricingArtifact) { a.CSS += " " }},
		{"defaults", func(a *PricingArtifact) { a.Defaults = append(append(json.RawMessage(nil), a.Defaults...), ' ') }},
		{"rule-hash", func(a *PricingArtifact) { a.RuleSHA256 = strings.Repeat("0", 64) }},
		{"verification", func(a *PricingArtifact) { a.Verification = "verified" }},
	} {
		t.Run("replay-refuses-"+tc.name, func(t *testing.T) {
			changed := first
			changed.Defaults = append(json.RawMessage(nil), first.Defaults...)
			tc.mutate(&changed)
			if VerifyPricingReplay(changed, []byte(file.Content)) == nil {
				t.Fatal("changed P02 adaptation passed replay")
			}
		})
	}

	var defaults map[string]any
	if err := json.Unmarshal(first.Defaults, &defaults); err != nil || defaults["URL"] != "" {
		t.Fatal("P02 defaults are invalid or contain a usable destination")
	}
	defaults["URL"] = "/pricing/access"
	fixtureDefaults, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal("P02 fixture defaults could not be encoded")
	}

	outputDir := t.TempDir()
	if retained := os.Getenv("APPKIT_PROBE_PRICING_OUTPUT"); retained != "" {
		clean, err := filepath.Abs(retained)
		resolved, resolveErr := filepath.EvalSymlinks(clean)
		tempRoot, tempErr := filepath.Abs(os.TempDir())
		resolvedTempRoot, resolvedTempErr := filepath.EvalSymlinks(tempRoot)
		rel, relErr := filepath.Rel(tempRoot, clean)
		info, statErr := os.Lstat(clean)
		entries, readErr := os.ReadDir(resolved)
		if err != nil || resolveErr != nil || tempErr != nil || resolvedTempErr != nil || relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || resolved != filepath.Join(resolvedTempRoot, rel) || statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || readErr != nil || len(entries) != 0 {
			t.Fatal("APPKIT_PROBE_PRICING_OUTPUT must be an existing empty real child of the system temp directory")
		}
		outputDir = resolved
	}
	artifactJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal("P02 artifact could not be encoded")
	}
	for name, body := range map[string][]byte{
		"source.html":   []byte(file.Content),
		"defaults.json": fixtureDefaults,
		"artifact.json": artifactJSON,
		"pricing.css":   []byte(first.CSS),
	} {
		if err := os.WriteFile(filepath.Join(outputDir, name), body, 0o600); err != nil {
			t.Fatal("private P02 output could not be written")
		}
	}
	compileDir := t.TempDir()
	templPath := filepath.Join(compileDir, "pricing.templ")
	modelPath := filepath.Join(compileDir, "pricing_model.go")
	testPath := filepath.Join(compileDir, "pricing_test.go")
	fixture := strings.NewReplacer(
		"ORIGINAL_LITERAL", strconv.Quote(file.Content),
		"DEFAULTS_LITERAL", strconv.Quote(string(fixtureDefaults)),
	).Replace(pricingRenderFixture)
	for path, body := range map[string]string{templPath: first.Templ, modelPath: first.Model, testPath: fixture} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal("private P02 compiler fixture could not be written")
		}
	}

	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("app-kit root could not be resolved")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	currentMarker := ""
	run := func(step string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = repo
		cmd.Env = append(pricingOfflineGoEnv(), "APPKIT_PRICING_FIXTURE_OUTPUT="+outputDir, "APPKIT_PRICING_FIXTURE_MARKER="+currentMarker)
		if err := cmd.Run(); err != nil {
			t.Fatalf("private P02 compiler proof failed at %s; output deliberately omitted", step)
		}
	}
	for _, formatted := range []bool{false, true} {
		if formatted {
			run("format", "tool", "templ", "fmt", templPath)
		}
		run("generate", "tool", "templ", "generate", "-f", templPath, "-include-version=false")
		markerName := "unformatted.pass"
		if formatted {
			markerName = "formatted.pass"
		}
		currentMarker = filepath.Join(compileDir, markerName)
		run("compile-render-compare", "test", "-count=1", "-run", "^TestPricingDefaultsRenderOriginalDOMWithDeclaredDestination$", filepath.Join(compileDir, "pricing_templ.go"), modelPath, testPath)
		if marker, err := os.ReadFile(currentMarker); err != nil || string(marker) != "passed\n" {
			t.Fatal("private P02 generated test did not execute exactly through its completion marker")
		}
		currentMarker = ""
	}
}

func pricingFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func pricingBytesSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func pricingOfflineGoEnv() []string {
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOPROXY" && key != "GOSUMDB" && key != "GOTOOLCHAIN" && key != "GOFLAGS" && key != "GOWORK" && key != "APPKIT_PRICING_FIXTURE_OUTPUT" && key != "APPKIT_PRICING_FIXTURE_MARKER" {
			env = append(env, item)
		}
	}
	return append(env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
}

const pricingRenderFixture = `package converted

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type nodeShape struct {
	Type html.NodeType
	Data, Namespace string
	Attr []html.Attribute
	Children []nodeShape
}

func shape(n *html.Node) nodeShape {
	s := nodeShape{Type: n.Type, Data: n.Data, Namespace: n.Namespace, Attr: n.Attr}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.Children = append(s.Children, shape(child))
	}
	return s
}

func shapeDifference(path string, want, got nodeShape) string {
	if want.Type != got.Type { return path + ".type" }
	if want.Data != got.Data {
		kind := "content"
		if strings.TrimSpace(want.Data) == strings.TrimSpace(got.Data) { kind = "whitespace-only" }
		return fmt.Sprintf("%s.data-%s(length %d != %d; trimmed %d != %d)", path, kind, len(want.Data), len(got.Data), len(strings.TrimSpace(want.Data)), len(strings.TrimSpace(got.Data)))
	}
	if want.Namespace != got.Namespace { return path + ".namespace" }
	if !reflect.DeepEqual(want.Attr, got.Attr) { return path + ".attributes" }
	if len(want.Children) != len(got.Children) { return path + ".child-count" }
	for i := range want.Children {
		if difference := shapeDifference(fmt.Sprintf("%s/%d", path, i), want.Children[i], got.Children[i]); difference != "" { return difference }
	}
	return ""
}

func fragment(t *testing.T, source string) []*html.Node {
	t.Helper()
	nodes, err := html.ParseFragment(strings.NewReader(source), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil { t.Fatal("fixture HTML did not parse") }
	return nodes
}

func replacePricingDestination(nodes []*html.Node) int {
	changed := 0
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for i := range n.Attr {
				if n.Attr[i].Key == "href" && n.Attr[i].Val == "#" {
					n.Attr[i].Val = "/pricing/access"
					changed++
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling { visit(child) }
	}
	for _, node := range nodes { visit(node) }
	return changed
}

func corruptPricingListWhitespace(nodes []*html.Node) bool {
	var visit func(*html.Node) bool
	visit = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "ul" {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode && strings.TrimSpace(child.Data) == "" {
					child.Data += " "
					return true
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if visit(child) { return true }
		}
		return false
	}
	for _, node := range nodes {
		if visit(node) { return true }
	}
	return false
}

func renderPricing(t *testing.T, data Pricing) []byte {
	t.Helper()
	component, err := NewPricing(data)
	if err != nil { t.Fatal("fixture model refused") }
	var out bytes.Buffer
	if err := component.Render(context.Background(), &out); err != nil { t.Fatal("fixture render failed") }
	return out.Bytes()
}

func renderPricingEnhanced(t *testing.T, data Pricing) []byte {
	t.Helper()
	component, err := NewPricingEnhanced(data)
	if err != nil { t.Fatal("enhanced fixture model refused") }
	var out bytes.Buffer
	if err := component.Render(context.Background(), &out); err != nil { t.Fatal("enhanced fixture render failed") }
	return out.Bytes()
}

func assertEnhancedWrapper(t *testing.T, faithful, enhanced []byte) {
	t.Helper()
	faithfulNodes, enhancedNodes := fragment(t, string(faithful)), fragment(t, string(enhanced))
	onlyElement := func(nodes []*html.Node) *html.Node {
		var element *html.Node
		for _, node := range nodes {
			if node.Type == html.TextNode && strings.TrimSpace(node.Data) == "" { continue }
			if node.Type != html.ElementNode || element != nil { return nil }
			element = node
		}
		return element
	}
	faithfulRoot, wrapper := onlyElement(faithfulNodes), onlyElement(enhancedNodes)
	if faithfulRoot == nil || wrapper == nil { t.Fatal("enhanced wrapper changed the rendered root tree count") }
	wantAttr := []html.Attribute{{Key: "data-appkit-pricing"}}
	if wrapper.Type != html.ElementNode || wrapper.Data != "div" || wrapper.Namespace != "" || !reflect.DeepEqual(wrapper.Attr, wantAttr) {
		t.Fatal("enhanced wrapper element or attributes changed")
	}
	var children []*html.Node
	for child := wrapper.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}
	content := onlyElement(children)
	if content == nil || !reflect.DeepEqual(shape(faithfulRoot), shape(content)) {
		t.Fatal("enhanced wrapper changed the faithful rendered child tree or its attributes")
	}
}

func writePricingOutput(t *testing.T, name string, body []byte) {
	t.Helper()
	path := filepath.Join(os.Getenv("APPKIT_PRICING_FIXTURE_OUTPUT"), name)
	if current, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(current, body) { t.Fatal("formatted replay changed retained output") }
		return
	} else if !os.IsNotExist(err) {
		t.Fatal("retained output could not be inspected")
	}
	if err := os.WriteFile(path, body, 0600); err != nil { t.Fatal("retained output could not be written") }
}

func TestPricingDefaultsRenderOriginalDOMWithDeclaredDestination(t *testing.T) {
	const original = ORIGINAL_LITERAL
	const defaults = DEFAULTS_LITERAL
	var data Pricing
	if err := json.Unmarshal([]byte(defaults), &data); err != nil || data.URL != "/pricing/access" {
		t.Fatal("fixture defaults invalid")
	}
	baseline := renderPricing(t, data)
	writePricingOutput(t, "baseline.html", baseline)
	enhancedBaseline := renderPricingEnhanced(t, data)
	assertEnhancedWrapper(t, baseline, enhancedBaseline)
	writePricingOutput(t, "enhanced.html", enhancedBaseline)
	wantNodes := fragment(t, original)
	if replacePricingDestination(wantNodes) != 1 { t.Fatal("original CTA structure changed") }
	gotNodes := fragment(t, string(baseline))
	want, got := make([]nodeShape, len(wantNodes)), make([]nodeShape, len(gotNodes))
	for i := range wantNodes { want[i] = shape(wantNodes[i]) }
	for i := range gotNodes { got[i] = shape(gotNodes[i]) }
	if !reflect.DeepEqual(want, got) {
		difference := "root-count"
		if len(want) == len(got) {
			for i := range want {
				if at := shapeDifference(fmt.Sprintf("root/%d", i), want[i], got[i]); at != "" { difference = at; break }
			}
		}
		t.Fatalf("rendered DOM differs beyond the declared CTA destination at %s", difference)
	}
	corruptedNodes := fragment(t, string(baseline))
	if !corruptPricingListWhitespace(corruptedNodes) { t.Fatal("rendered list has no whitespace node for the planted oracle canary") }
	corrupted := make([]nodeShape, len(corruptedNodes))
	for i := range corruptedNodes { corrupted[i] = shape(corruptedNodes[i]) }
	if reflect.DeepEqual(want, corrupted) { t.Fatal("planted list-whitespace corruption escaped the DOM oracle") }
	difference := "root-count"
	if len(want) == len(corrupted) {
		for i := range want {
			if at := shapeDifference(fmt.Sprintf("root/%d", i), want[i], corrupted[i]); at != "" { difference = at; break }
		}
	}
	if !strings.Contains(difference, "whitespace-only") { t.Fatal("planted list-whitespace corruption was not identified as whitespace-only") }
	long := Pricing{
		Heading: strings.Repeat("Synthetic pricing heading ", 6),
		Introduction: strings.Repeat("Synthetic introduction retains a deliberately long line for wrapping review. ", 12),
		Plan: strings.Repeat("Synthetic plan ", 8),
		Description: strings.Repeat("Synthetic plan description exercises responsive wrapping without changing the component contract. ", 10),
		Included: strings.Repeat("Synthetic inclusion ", 4),
		Payment: strings.Repeat("Synthetic payment context ", 5),
		Price: strings.Repeat("1234567890", 5),
		Currency: "SYNTHETIC",
		Action: strings.Repeat("Review synthetic access ", 4),
		Note: strings.Repeat("Synthetic note for bounded long-content layout review. ", 6),
		URL: "/pricing/access?fixture=long",
	}
	for i := 1; i <= 12; i++ {
		long.Features = append(long.Features, strings.Repeat("Synthetic feature ", 8)+string(rune('A'-1+i)))
	}
	longBaseline := renderPricing(t, long)
	writePricingOutput(t, "long.html", longBaseline)
	enhancedLong := renderPricingEnhanced(t, long)
	assertEnhancedWrapper(t, longBaseline, enhancedLong)
	writePricingOutput(t, "enhanced-long.html", enhancedLong)
	marker := os.Getenv("APPKIT_PRICING_FIXTURE_MARKER")
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil { t.Fatal("completion marker could not be created") }
	if _, err := file.WriteString("passed\n"); err != nil { file.Close(); t.Fatal("completion marker could not be written") }
	if err := file.Close(); err != nil { t.Fatal("completion marker could not be closed") }
}
`
