package designreceipts_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

func TestBuildOverlayBundleSyntheticDeterministicAndIncomplete(t *testing.T) {
	spec, read := syntheticOverlay(t, syntheticPNG(t), "capture.png")
	one, first, err := buildOverlayBundle(spec, read)
	if err != nil {
		t.Fatal(err)
	}
	two, second, err := buildOverlayBundle(spec, read)
	if err != nil || !bytes.Equal(one, two) || first != second || first.Images != 1 || first.BundleSHA != designreceipts.Digest(one) {
		t.Fatalf("nondeterministic bundle: %v %#v %#v", err, first, second)
	}
	var stored struct {
		Contract json.RawMessage   `json:"contract"`
		Receipt  json.RawMessage   `json:"receipt"`
		Content  map[string][]byte `json:"content"`
	}
	if err := json.Unmarshal(one, &stored); err != nil {
		t.Fatal(err)
	}
	report, err := designreceipts.Check(stored.Contract, stored.Receipt, first.Expected, stored.Content)
	if err != nil || report.Verification != "not-verified" || len(report.IncompleteGates) != 9 || report.ClaimedStatuses["interaction"] != "failed" || report.ClaimedStatuses["visual"] != "not-run" {
		t.Fatalf("historical packet promoted: %v %#v", err, report)
	}
	if !bytes.Equal(stored.Content["synthetic-run-capture"], syntheticPNG(t)) {
		t.Fatal("capture bytes changed")
	}
}

func TestBuildOverlayBundleSyntheticRefusals(t *testing.T) {
	tests := map[string]func(*overlaySpec, map[string][]byte){
		"count":        func(s *overlaySpec, _ map[string][]byte) { s.Runs[0].Images++ },
		"asset hash":   func(s *overlaySpec, _ map[string][]byte) { s.Assets[0].SHA = strings.Repeat("0", 64) },
		"duplicate ID": func(s *overlaySpec, _ map[string][]byte) { s.Assets = append(s.Assets, s.Assets[0]) },
		"source pin":   func(s *overlaySpec, _ map[string][]byte) { s.SourceSHA = strings.Repeat("a", 64) },
		"output pin":   func(s *overlaySpec, _ map[string][]byte) { s.ArtifactSHA = strings.Repeat("b", 64) },
		"capture hash": func(s *overlaySpec, files map[string][]byte) {
			resetSyntheticReport(t, s, files, overlayCapture{File: "capture.png", SHA256: strings.Repeat("c", 64), Bytes: len(files["proof/capture.png"])})
		},
		"capture path": func(s *overlaySpec, files map[string][]byte) {
			resetSyntheticReport(t, s, files, overlayCapture{File: "../capture.png", SHA256: designreceipts.Digest(files["proof/capture.png"]), Bytes: len(files["proof/capture.png"])})
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			spec, _, files := syntheticOverlayFiles(t, syntheticPNG(t), "capture.png")
			mutate(&spec, files)
			if _, _, err := buildOverlayBundle(spec, mapReader(files)); err == nil {
				t.Fatal("invalid overlay accepted")
			}
		})
	}
	t.Run("invalid PNG", func(t *testing.T) {
		spec, read := syntheticOverlay(t, []byte("not a PNG"), "capture.png")
		if _, _, err := buildOverlayBundle(spec, read); err == nil {
			t.Fatal("invalid PNG accepted")
		}
	})
}

func TestReadOverlayFileRefusesEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := readOverlayFile(root, "ok"); err != nil || string(body) != "ok" {
		t.Fatalf("regular read: %q %v", body, err)
	}
	if _, err := readOverlayFile("relative", "ok"); err == nil {
		t.Fatal("relative root accepted")
	}
	if _, err := readOverlayFile(root, "../ok"); err == nil {
		t.Fatal("escaping path accepted")
	}
	if err := os.Symlink("ok", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := readOverlayFile(root, "link"); err == nil {
		t.Fatal("symlink accepted")
	}
}

func syntheticOverlay(t *testing.T, imageBody []byte, name string) (overlaySpec, func(string, string) ([]byte, error)) {
	t.Helper()
	spec, read, _ := syntheticOverlayFiles(t, imageBody, name)
	return spec, read
}

func syntheticOverlayFiles(t *testing.T, imageBody []byte, name string) (overlaySpec, func(string, string) ([]byte, error), map[string][]byte) {
	t.Helper()
	files := map[string][]byte{"proof/source": []byte("source"), "proof/adapter": []byte("adapter"), "proof/artifact": []byte("artifact"), "proof/harness": []byte("harness"), "proof/tool": []byte("tool"), "proof/baseline": []byte("baseline"), "proof/" + name: imageBody}
	asset := func(id, group, path string) overlayAsset {
		return overlayAsset{ID: id, Group: group, Root: "proof", Path: path, SHA: designreceipts.Digest(files["proof/"+path])}
	}
	spec := overlaySpec{Name: "synthetic", Subject: "reference.synthetic", Member: "synthetic/member.html", SourceSHA: designreceipts.Digest(files["proof/source"]), ArtifactSHA: designreceipts.Digest(files["proof/artifact"]), InteractionReason: "A retained interaction observation failed; no full-gate acceptance is claimed.", Assets: []overlayAsset{asset("synthetic-source", "source", "source"), asset("synthetic-adapter", "transformer", "adapter"), asset("synthetic-artifact", "output", "artifact"), asset("synthetic-harness", "fixture", "harness"), asset("synthetic-tool", "toolchain", "tool"), asset("synthetic-baseline", "baseline", "baseline")}, Runs: []overlayRun{{ID: "synthetic-run", Gate: "visual", Root: "proof", Path: "receipt.json", Images: 1}}}
	resetSyntheticReport(t, &spec, files, overlayCapture{File: name, SHA256: designreceipts.Digest(imageBody), Bytes: len(imageBody)})
	return spec, mapReader(files), files
}

func resetSyntheticReport(t *testing.T, spec *overlaySpec, files map[string][]byte, capture overlayCapture) {
	t.Helper()
	body, err := json.Marshal(struct {
		Captures []overlayCapture `json:"captures"`
	}{[]overlayCapture{capture}})
	if err != nil {
		t.Fatal(err)
	}
	files["proof/receipt.json"] = body
	spec.Runs[0].SHA = designreceipts.Digest(body)
}

func mapReader(files map[string][]byte) func(string, string) ([]byte, error) {
	return func(root, path string) ([]byte, error) {
		body, ok := files[root+"/"+path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return bytes.Clone(body), nil
	}
}

func syntheticPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
