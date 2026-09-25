package designreceipts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
	"github.com/geoffmilleraz/app-kit/internal/navigationproof"
)

// Export only into an explicitly selected existing private directory. The CLI
// performs the separate, expected-pin-gated durable publication step.
func TestP06EvidenceBundleExport(t *testing.T) {
	out, requested := os.LookupEnv("APPKIT_P06_BUNDLE_OUT")
	if !requested {
		t.Skip("set APPKIT_P06_BUNDLE_OUT to a fresh private output directory")
	}
	oldRoot, freshRoot := os.Getenv("APPKIT_P06_RECEIPT_ROOT"), os.Getenv("APPKIT_P06_FRESH_ROOT")
	for _, name := range []string{out, oldRoot, freshRoot} {
		info, err := os.Lstat(name)
		if !filepath.IsAbs(name) || err != nil || !info.IsDir() {
			t.Fatalf("explicit existing real private directories required: %q", name)
		}
	}
	read := func(root, name string) []byte {
		t.Helper()
		f, err := os.OpenRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		info, err := f.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
			t.Fatalf("invalid bounded input %s: %v", name, err)
		}
		body, err := f.ReadFile(name)
		if err != nil || int64(len(body)) != info.Size() {
			t.Fatalf("read %s: %v", name, err)
		}
		return body
	}
	content := map[string][]byte{}
	for id, name := range map[string]string{"p06-source-html": "source.html", "p06-transformer-source": "implementation/navigation.go", "p06-artifact-schema": "implementation/navigation.cue", "p06-adapter-output": "delivery/artifact.json", "p06-reference-fixture": "implementation/preview_test.go.txt", "p06-browser-oracle": "implementation/browser-final.mjs", "p06-toolchain-report": "prepare.json", "p06-baseline-css": "navigation.css", "p06-reproducibility-evidence": "prepare.json", "p06-interaction-evidence": "browser-YOPTyP/report.json"} {
		content[id] = read(oldRoot, name)
	}
	c, r := p06RetrospectiveContract(), p06RetrospectiveReceipt()
	write := func(name string, c designreceipts.Contract, r designreceipts.Receipt, content map[string][]byte) {
		t.Helper()
		cb, err := designreceipts.EncodeContract(c)
		if err != nil {
			t.Fatal(err)
		}
		r.ContractSHA256 = designreceipts.Digest(cb)
		rb, err := designreceipts.EncodeReceipt(r)
		if err != nil {
			t.Fatal(err)
		}
		report, err := designreceipts.Check(cb, rb, designreceipts.Expected{ContractSHA256: r.ContractSHA256, SourceRevision: p06SourceSHA, ArtifactSHA256: p06ArtifactSHA}, content)
		if err != nil {
			t.Fatal(err)
		}
		if report.Verification != "not-verified" || len(report.IncompleteGates) != 9 {
			t.Fatal("partial evidence promoted a gate")
		}
		body, err := designevidence.EncodeBundle("html/navigation/tabs/01-tabs-with-underline.html", cb, rb, content)
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(filepath.Join(out, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(body); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s bundle=%s contract=%s receipt=%s artifacts=%d nine gates incomplete", name, designreceipts.Digest(body), r.ContractSHA256, designreceipts.Digest(rb), report.MatchedArtifacts)
	}
	write("p06-retained.json", c, r, content)

	// This aggregate is retrospective; its nested task contract was separately
	// frozen prospectively. Neither mode is relabeled by importing it here.
	run := "quality-2DrLVH"
	cb, rb := read(freshRoot, run+"/contract.json"), read(freshRoot, run+"/report.json")
	const expected = "5430b2afce1cb923f965bfc564a77277e9f7439a095e30fb1c082ecbdb6489f7"
	result, err := navigationproof.Check(expected, cb, rb)
	if err != nil || result.Status != "passed" || result.Verification != "not-verified" {
		t.Fatalf("nested scoped proof: %+v %v", result, err)
	}
	var task navigationproof.Contract
	if json.Unmarshal(cb, &task) != nil {
		t.Fatal("task contract")
	}
	var report navigationproof.Report
	if json.Unmarshal(rb, &report) != nil {
		t.Fatal("task report")
	}
	if task.SourceSHA256 != p06SourceSHA || task.ArtifactSHA256 != p06ArtifactSHA {
		t.Fatal("nested subject mismatch")
	}
	add := func(id string, body []byte) designreceipts.Asset {
		content[id] = body
		return retainedAsset(id, designreceipts.Digest(body), len(body))
	}
	c.Inputs.Fixture = append(c.Inputs.Fixture, add("p06-links-prospective-contract", cb))
	observed := add("p06-links-observations", rb)
	gate := c.Gates["interaction"]
	gate.Slots = append(gate.Slots, designreceipts.Slot{ID: observed.ID, Role: "report"})
	c.Gates["interaction"] = gate
	partial := r.Results["interaction"]
	partial.Reason = "A separately frozen 24-case Links input/zoom/contrast task passed. This full gate still lacks screen-reader, physical touch and other applicable interaction proof."
	partial.Evidence = append(partial.Evidence, designreceipts.Evidence{Asset: observed, Role: "report"})
	r.Results["interaction"] = partial
	for _, input := range []struct{ id, path, sha string }{{"p06-current-preparation", "prepare.json", task.PrepareSHA256}, {"p06-current-preview-binary", "preview.test", task.BinarySHA256}} {
		body := read(freshRoot, input.path)
		if designreceipts.Digest(body) != input.sha {
			t.Fatalf("nested input drift: %s", input.id)
		}
		c.Inputs.Toolchain = append(c.Inputs.Toolchain, add(input.id, body))
	}
	kitRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	collector := read(kitRoot, "docs/assessments/fixtures/navigation-adapter/quality.mjs")
	if designreceipts.Digest(collector) != task.RunnerSHA256 {
		t.Fatal("collector drift")
	}
	c.Inputs.Fixture = append(c.Inputs.Fixture, add("p06-links-collector", collector), add("p06-links-fact-oracle", read(kitRoot, "internal/navigationproof/check.go")), add("p06-links-fact-schema", read(kitRoot, "internal/navigationproof/schema.cue")))
	var images []struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(read(freshRoot, run+"/screenshots.json"), &images); err != nil || len(images) != 24 {
		t.Fatal("screenshot inventory")
	}
	seen := map[string]bool{}
	for _, image := range images {
		if seen[image.Name] || filepath.Base(image.Name) != image.Name || !strings.HasSuffix(image.Name, ".png") {
			t.Fatal("unexpected image name")
		}
		seen[image.Name] = true
		body := read(freshRoot, run+"/"+image.Name)
		if designreceipts.Digest(body) != image.SHA256 {
			t.Fatal("screenshot drift")
		}
		a := add("p06-task-"+strings.TrimSuffix(image.Name, ".png"), body)
		g := c.Gates["visual"]
		g.Slots = append(g.Slots, designreceipts.Slot{ID: a.ID, Role: "image"})
		c.Gates["visual"] = g
		v := r.Results["visual"]
		v.Evidence = append(v.Evidence, designreceipts.Evidence{Asset: a, Role: "image"})
		v.Reason = "Retained adapted Links task captures are not complete source-fidelity proof; original source limitations remain."
		r.Results["visual"] = v
	}
	write("p06-with-links-task.json", c, r, content)
}
