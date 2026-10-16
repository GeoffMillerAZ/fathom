package designreceipts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

const (
	p06SourceSHA   = "01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31"
	p06ArtifactSHA = "8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6"
	p06ContractSHA = "854eaeb9483e5df1422a46fd6ab9b5a44e7ba070212dd519914a6baf257430ee"
	p06ReceiptSHA  = "4e5df913973635c9e6d51ec6ee184bc4dc790e6c2a7d7492331f3c9e26e44c91"
)

func retainedAsset(id, sha string, size int) designreceipts.Asset {
	return designreceipts.Asset{ID: id, SHA256: sha, Bytes: size}
}

func p06RetrospectiveContract() designreceipts.Contract {
	scopes := map[string]string{
		"reproducibility": "Exact frozen source, transformer, config, output, fixture, toolchain and baseline replay.",
		"structure":       "Valid semantic navigation markup, identities, labels, current state and safe destinations.",
		"visual":          "Source and Links target under identical viewport, theme, density, font and motion conditions.",
		"interaction":     "Static Links keyboard, pointer, touch, focus, history, long-label and zoom tasks.",
		"lifecycle":       "Applicable navigation lifecycle, cancellation and restoration behavior.",
		"security":        "Escaping, route authority, CSP, licensed-output and request-boundary checks.",
		"data":            "Ordered labels, routes and truthful current-page state across the task.",
		"performance":     "Named cold and warm payload, first-use and navigation workload measurements.",
		"integration":     "Clean real-app build and one real navigation task without a private fork.",
	}
	gates := make(map[string]designreceipts.Gate, len(scopes))
	for name, scope := range scopes {
		role := "report"
		if name == "performance" {
			role = "measurement"
		}
		gates[name] = designreceipts.Gate{
			Applicable:      true,
			Scope:           scope,
			ReviewAssertion: "No retrospective N/A approval; this full gate remains applicable.",
			Slots:           []designreceipts.Slot{{ID: "p06-" + name + "-evidence", Role: role}},
		}
	}
	return designreceipts.Contract{
		SchemaVersion: 1,
		Subject: designreceipts.Subject{
			ID:             "reference.navigation-adapter",
			Profile:        "static-web",
			Variant:        "links",
			SourceArchive:  "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8",
			SourceRevision: p06SourceSHA,
			ArtifactSHA256: p06ArtifactSHA,
		},
		Mode: "retrospective",
		Inputs: designreceipts.Inputs{
			Source: []designreceipts.Asset{retainedAsset("p06-source-html", p06SourceSHA, 2483)},
			Transformer: []designreceipts.Asset{retainedAsset("p06-transformer-source",
				"78cf0ac2d71ecb332b2b28da0b88c00d7313244479066f27889e097f12b7a9d8", 12555)},
			Config: []designreceipts.Asset{retainedAsset("p06-artifact-schema",
				"40e22b355afaa98e550ff17a535394a89540072d7507d927ebda9eafd9eb9bcf", 1085)},
			// Links needs no browser runtime and this retained candidate has no separate patch.
			Patch:   []designreceipts.Asset{},
			Runtime: []designreceipts.Asset{},
			Output:  []designreceipts.Asset{retainedAsset("p06-adapter-output", p06ArtifactSHA, 12165)},
			Fixture: []designreceipts.Asset{
				retainedAsset("p06-reference-fixture", "b6d1f21030ab09678fb64d0f48ca56f5e920fdde209e9b31547cdb9d8942e1d9", 6804),
				retainedAsset("p06-browser-oracle", "0d398043b13cec8e6d83e6233b8f81d879b850eb925ddaecd29ba5a0940e0de7", 15389),
			},
			Toolchain: []designreceipts.Asset{retainedAsset("p06-toolchain-report",
				"5f51f1d29a9b27a154ec2b5e1239743a543c492c33676ab775d5b9aea2f5c3e9", 1912)},
			Baseline: []designreceipts.Asset{retainedAsset("p06-baseline-css",
				"8f9f11836291945e9b08fb92f82e94a6a2704ca9fa0c49ea92388aa5cd615cf3", 484717)},
		},
		Gates: gates,
	}
}

func p06RetrospectiveReceipt() designreceipts.Receipt {
	reasons := map[string]string{
		"reproducibility": "A retained 14-check preparation report exists, but the full current gate was not run under this contract.",
		"structure":       "Scoped adapter oracles exist; the complete current structure gate was not run under this contract.",
		"visual":          "Retained captures include two source target-size failures; complete visual fidelity review was not run.",
		"interaction":     "The retained report covers scoped Links, GET, history and zoom tasks but preserves source failures; touch and screen-reader tasks were not run.",
		"lifecycle":       "No complete lifecycle, cancellation and restoration gate was run for the static Links target.",
		"security":        "Retained loopback containment is partial; the complete current security gate was not run.",
		"data":            "No N/A review exists; current-key and route checks do not complete the full data gate.",
		"performance":     "Payload metadata exists, but no named cold/warm or latency workload gate was run.",
		"integration":     "The standalone fixture is not a real consumer task; no real-app integration gate was run.",
	}
	results := make(map[string]designreceipts.Result, len(reasons))
	for name, reason := range reasons {
		results[name] = designreceipts.Result{Status: "not-run", Reason: reason, Evidence: []designreceipts.Evidence{}}
	}
	repro := results["reproducibility"]
	repro.Evidence = []designreceipts.Evidence{{
		Asset: retainedAsset("p06-reproducibility-evidence", "5f51f1d29a9b27a154ec2b5e1239743a543c492c33676ab775d5b9aea2f5c3e9", 1912),
		Role:  "report",
	}}
	results["reproducibility"] = repro
	interaction := results["interaction"]
	interaction.Evidence = []designreceipts.Evidence{{
		Asset: retainedAsset("p06-interaction-evidence", "2abd2c775278ce90f0e81bb2c877ff3c23690dd672e8f200484e88df9aa995d3", 72820),
		Role:  "report",
	}}
	results["interaction"] = interaction
	return designreceipts.Receipt{SchemaVersion: 1, ContractSHA256: p06ContractSHA,
		SourceRevision: p06SourceSHA, ArtifactSHA256: p06ArtifactSHA, Results: results}
}

func TestP06RetrospectiveMetadataIsCanonicalAndIncomplete(t *testing.T) {
	contractBody, err := designreceipts.EncodeContract(p06RetrospectiveContract())
	if err != nil {
		t.Fatal(err)
	}
	if got := designreceipts.Digest(contractBody); got != p06ContractSHA {
		t.Fatalf("contract SHA = %s, want %s", got, p06ContractSHA)
	}
	receiptBody, err := designreceipts.EncodeReceipt(p06RetrospectiveReceipt())
	if err != nil {
		t.Fatal(err)
	}
	if got := designreceipts.Digest(receiptBody); got != p06ReceiptSHA {
		t.Fatalf("receipt SHA = %s, want %s", got, p06ReceiptSHA)
	}
	var contract designreceipts.Contract
	var receipt designreceipts.Receipt
	if json.Unmarshal(contractBody, &contract) != nil || json.Unmarshal(receiptBody, &receipt) != nil {
		t.Fatal("canonical metadata did not round-trip")
	}
	roundContract, _ := designreceipts.EncodeContract(contract)
	roundReceipt, _ := designreceipts.EncodeReceipt(receipt)
	if !bytes.Equal(contractBody, roundContract) || !bytes.Equal(receiptBody, roundReceipt) {
		t.Fatal("retrospective metadata round-trip changed bytes")
	}
	for name, gate := range contract.Gates {
		if !gate.Applicable || receipt.Results[name].Status != "not-run" {
			t.Fatalf("gate %s = applicable %v/status %q; want true/not-run", name, gate.Applicable, receipt.Results[name].Status)
		}
	}
	if strings.Contains(string(contractBody), "data/design-assets") || strings.Contains(string(contractBody), "<") {
		t.Fatal("tracked metadata contains a private path or source body")
	}
	t.Logf("P06 retrospective contract=%s receipt=%s gates=%d all-not-run", p06ContractSHA, p06ReceiptSHA, len(contract.Gates))
}

func TestP06RetrospectiveRetainedArtifactsMatch(t *testing.T) {
	root, present := os.LookupEnv("APPKIT_P06_RECEIPT_ROOT")
	if !present {
		t.Skip("set APPKIT_P06_RECEIPT_ROOT to the retained private P06 directory")
	}
	if strings.TrimSpace(root) == "" {
		t.Fatal("APPKIT_P06_RECEIPT_ROOT is present but empty")
	}
	if !filepath.IsAbs(root) {
		packageDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("resolve package directory: %v", err)
		}
		root = filepath.Join(packageDir, "..", "..", root)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("stat APPKIT_P06_RECEIPT_ROOT: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("APPKIT_P06_RECEIPT_ROOT is not a directory")
	}
	contractBody, err := designreceipts.EncodeContract(p06RetrospectiveContract())
	if err != nil {
		t.Fatal(err)
	}
	receiptBody, err := designreceipts.EncodeReceipt(p06RetrospectiveReceipt())
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{
		"p06-source-html":              "source.html",
		"p06-transformer-source":       "implementation/navigation.go",
		"p06-artifact-schema":          "implementation/navigation.cue",
		"p06-adapter-output":           "delivery/artifact.json",
		"p06-reference-fixture":        "implementation/preview_test.go.txt",
		"p06-browser-oracle":           "implementation/browser-final.mjs",
		"p06-toolchain-report":         "prepare.json",
		"p06-baseline-css":             "navigation.css",
		"p06-reproducibility-evidence": "prepare.json",
		"p06-interaction-evidence":     "browser-YOPTyP/report.json",
	}
	content := make(map[string][]byte, len(paths))
	for id, path := range paths {
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatalf("read retained %s: %v", id, readErr)
		}
		content[id] = body
	}
	expected := designreceipts.Expected{ContractSHA256: p06ContractSHA,
		SourceRevision: p06SourceSHA, ArtifactSHA256: p06ArtifactSHA}
	report, err := designreceipts.Check(contractBody, receiptBody, expected, content)
	if err != nil {
		t.Fatal(err)
	}
	wantIncomplete := []string{"data", "integration", "interaction", "lifecycle", "performance", "reproducibility", "security", "structure", "visual"}
	if report.Integrity != "matched" || report.Verification != "not-verified" ||
		report.MatchedArtifacts != len(content) || !reflect.DeepEqual(report.IncompleteGates, wantIncomplete) {
		t.Fatalf("unexpected integrity report: %+v", report)
	}
	for name, status := range report.ClaimedStatuses {
		if status != "not-run" {
			t.Fatalf("gate %s status = %q, want not-run", name, status)
		}
	}
	tamperedSource := cloneContent(content)
	tamperedSource["p06-source-html"][0] ^= 1
	if _, err := designreceipts.Check(contractBody, receiptBody, expected, tamperedSource); err == nil {
		t.Fatal("tampered retained source matched")
	}
	tamperedReport := cloneContent(content)
	tamperedReport["p06-interaction-evidence"][0] ^= 1
	if _, err := designreceipts.Check(contractBody, receiptBody, expected, tamperedReport); err == nil {
		t.Fatal("tampered retained browser report matched")
	}
	t.Logf("P06 retained contract=%s receipt=%s matched=%d gates=%d all-not-run",
		report.ContractSHA256, report.ReceiptSHA256, report.MatchedArtifacts, len(report.ClaimedStatuses))
}

func cloneContent(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for id, body := range in {
		out[id] = append([]byte(nil), body...)
	}
	return out
}
