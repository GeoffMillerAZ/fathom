package designreceipts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

// This is a maintainer fixture, not a general importer or production API.
// Independently reviewed receipt pins select historical bytes. Captures are not
// replayed, authored observations are not authenticated, and no gate is promoted.
const overlayArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"

type overlayAsset struct{ ID, Group, Root, Path, SHA string }
type overlayRun struct {
	ID, Gate, Root, Path, SHA string
	Images                    int
}
type overlaySpec struct {
	Name, Subject, Member, SourceSHA, ArtifactSHA, InteractionReason string
	Assets                                                           []overlayAsset
	Runs                                                             []overlayRun
}
type overlayExport struct {
	Name              string                  `json:"name"`
	BundleSHA         string                  `json:"bundle_sha256"`
	Expected          designreceipts.Expected `json:"expected"`
	Images, Artifacts int
}
type overlayCapture struct {
	File, SHA256 string
	Bytes        int
}

var overlayPNGName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*\.png$`)

func buildOverlayBundle(spec overlaySpec, read func(string, string) ([]byte, error)) ([]byte, overlayExport, error) {
	var summary overlayExport
	content := map[string][]byte{}
	add := func(id string, body []byte) (designreceipts.Asset, error) {
		if _, ok := content[id]; ok {
			return designreceipts.Asset{}, fmt.Errorf("duplicate logical artifact %s", id)
		}
		content[id] = bytes.Clone(body)
		return designreceipts.Asset{ID: id, SHA256: designreceipts.Digest(body), Bytes: len(body)}, nil
	}
	load := func(root, file, sha string) ([]byte, error) {
		body, err := read(root, file)
		if err != nil {
			return nil, err
		}
		if len(body) > 32<<20 || designreceipts.Digest(body) != sha {
			return nil, fmt.Errorf("pinned input drift: %s/%s", root, file)
		}
		return body, nil
	}
	c := designreceipts.Contract{SchemaVersion: 1, Mode: "retrospective", Subject: designreceipts.Subject{ID: spec.Subject, Profile: "go-web", Variant: "original-typed-repair-history", SourceArchive: overlayArchive, SourceRevision: spec.SourceSHA, ArtifactSHA256: spec.ArtifactSHA}, Gates: map[string]designreceipts.Gate{}}
	c.Inputs = designreceipts.Inputs{Source: []designreceipts.Asset{}, Transformer: []designreceipts.Asset{}, Config: []designreceipts.Asset{}, Patch: []designreceipts.Asset{}, Runtime: []designreceipts.Asset{}, Output: []designreceipts.Asset{}, Fixture: []designreceipts.Asset{}, Toolchain: []designreceipts.Asset{}, Baseline: []designreceipts.Asset{}}
	groups := map[string]*[]designreceipts.Asset{"source": &c.Inputs.Source, "transformer": &c.Inputs.Transformer, "config": &c.Inputs.Config, "patch": &c.Inputs.Patch, "runtime": &c.Inputs.Runtime, "output": &c.Inputs.Output, "fixture": &c.Inputs.Fixture, "toolchain": &c.Inputs.Toolchain, "baseline": &c.Inputs.Baseline}
	for _, item := range spec.Assets {
		group, ok := groups[item.Group]
		if !ok {
			return nil, summary, fmt.Errorf("unknown group %q", item.Group)
		}
		body, err := load(item.Root, item.Path, item.SHA)
		if err != nil {
			return nil, summary, err
		}
		a, err := add(item.ID, body)
		if err != nil {
			return nil, summary, err
		}
		*group = append(*group, a)
	}
	r := designreceipts.Receipt{SchemaVersion: 1, SourceRevision: spec.SourceSHA, ArtifactSHA256: spec.ArtifactSHA, Results: map[string]designreceipts.Result{}}
	for _, name := range []string{"reproducibility", "structure", "visual", "interaction", "lifecycle", "security", "data", "performance", "integration"} {
		c.Gates[name] = designreceipts.Gate{Applicable: true, Scope: "Historical comparison packet; full " + name + " acceptance remains incomplete. Original, typed and explicit repair variants are separate observations.", Slots: []designreceipts.Slot{{ID: spec.Name + "-future-" + name, Role: "review"}}}
		r.Results[name] = designreceipts.Result{Status: "not-run", Reason: "No full-gate acceptance or authenticated review is retained. Selected inputs are not complete dependency closure or a fresh replay.", Evidence: []designreceipts.Evidence{}}
	}
	interaction := r.Results["interaction"]
	interaction.Status, interaction.Reason = "failed", spec.InteractionReason
	r.Results["interaction"] = interaction
	for _, run := range spec.Runs {
		gate, ok := c.Gates[run.Gate]
		if !ok {
			return nil, summary, fmt.Errorf("unknown gate %q", run.Gate)
		}
		body, err := load(run.Root, run.Path, run.SHA)
		if err != nil {
			return nil, summary, err
		}
		var report struct {
			Captures []overlayCapture `json:"captures"`
		}
		if err := json.Unmarshal(body, &report); err != nil || len(report.Captures) != run.Images {
			return nil, summary, fmt.Errorf("unexpected pinned capture inventory: %s", run.ID)
		}
		result := r.Results[run.Gate]
		appendEvidence := func(id, role string, body []byte) error {
			a, err := add(id, body)
			if err != nil {
				return err
			}
			gate.Slots = append(gate.Slots, designreceipts.Slot{ID: id, Role: role})
			result.Evidence = append(result.Evidence, designreceipts.Evidence{Asset: a, Role: role})
			return nil
		}
		if err := appendEvidence(run.ID+"-report", "report", body); err != nil {
			return nil, summary, err
		}
		for _, capture := range report.Captures {
			if !overlayPNGName.MatchString(capture.File) || filepath.Base(capture.File) != capture.File {
				return nil, summary, fmt.Errorf("unsafe capture name")
			}
			pngBody, err := load(run.Root, filepath.Join(filepath.Dir(run.Path), capture.File), capture.SHA256)
			if err != nil {
				return nil, summary, err
			}
			if len(pngBody) != capture.Bytes || len(pngBody) > 8<<20 {
				return nil, summary, fmt.Errorf("capture size mismatch")
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(pngBody))
			if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width > 16_000_000/cfg.Height {
				return nil, summary, fmt.Errorf("invalid bounded PNG")
			}
			if _, err := png.Decode(bytes.NewReader(pngBody)); err != nil {
				return nil, summary, err
			}
			if err := appendEvidence(run.ID+"-"+strings.TrimSuffix(capture.File, ".png"), "image", pngBody); err != nil {
				return nil, summary, err
			}
			summary.Images++
		}
		c.Gates[run.Gate], r.Results[run.Gate] = gate, result
	}
	cb, err := designreceipts.EncodeContract(c)
	if err != nil {
		return nil, summary, err
	}
	r.ContractSHA256 = designreceipts.Digest(cb)
	rb, err := designreceipts.EncodeReceipt(r)
	if err != nil {
		return nil, summary, err
	}
	expected := designreceipts.Expected{ContractSHA256: r.ContractSHA256, SourceRevision: spec.SourceSHA, ArtifactSHA256: spec.ArtifactSHA}
	checked, err := designreceipts.Check(cb, rb, expected, content)
	if err != nil {
		return nil, summary, err
	}
	if checked.Verification != "not-verified" || len(checked.IncompleteGates) != 9 || checked.ClaimedStatuses["interaction"] != "failed" {
		return nil, summary, fmt.Errorf("historical evidence was promoted")
	}
	bundle, err := designevidence.EncodeBundle(spec.Member, cb, rb, content)
	if err != nil {
		return nil, summary, err
	}
	summary.Name = spec.Name
	summary.BundleSHA, summary.Expected, summary.Artifacts = designreceipts.Digest(bundle), expected, checked.MatchedArtifacts
	return bundle, summary, nil
}

// Reads only a named relative file beneath an explicitly supplied real root.
func readOverlayFile(root, name string) ([]byte, error) {
	if !filepath.IsAbs(root) || !filepath.IsLocal(name) {
		return nil, fmt.Errorf("explicit absolute root and local path required")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("real input root required")
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	info, err = r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<20 {
		return nil, fmt.Errorf("bounded regular input required")
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("input changed during open")
	}
	body, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) != info.Size() {
		return nil, fmt.Errorf("input changed during read")
	}
	return body, nil
}

func TestOverlayEvidenceBundleExport(t *testing.T) {
	out, requested := os.LookupEnv("APPKIT_OVERLAY_BUNDLE_OUT")
	if !requested {
		t.Skip("set explicit private output and input roots for reviewed overlay retention")
	}
	info, err := os.Lstat(out)
	if !filepath.IsAbs(out) || err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		t.Fatal("existing private 0700 output root required")
	}
	roots := map[string]string{"kit": os.Getenv("APPKIT_OVERLAY_KIT"), "runtime": os.Getenv("APPKIT_OVERLAY_RUNTIME"), "drawer": os.Getenv("APPKIT_OVERLAY_DRAWER"), "drawer-repair": os.Getenv("APPKIT_OVERLAY_DRAWER_REPAIR"), "menu": os.Getenv("APPKIT_OVERLAY_MENU"), "menu-repair": os.Getenv("APPKIT_OVERLAY_MENU_REPAIR")}
	read := func(root, name string) ([]byte, error) { return readOverlayFile(roots[root], name) }
	// Prepare both before publishing any output. Durable saving is a separate CLI
	// action with reviewed expected pins; this test never edits the private store.
	type prepared struct {
		Body    []byte
		Summary overlayExport
	}
	var packets []prepared
	for _, spec := range []overlaySpec{drawerOverlaySpec(), menuOverlaySpec()} {
		body, summary, err := buildOverlayBundle(spec, read)
		if err != nil {
			t.Fatalf("%s: %v", spec.Name, err)
		}
		packets = append(packets, prepared{body, summary})
	}
	r, err := os.OpenRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, packet := range packets {
		f, err := r.OpenFile(packet.Summary.Name+".json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(packet.Body)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatalf("write: %v; close: %v", writeErr, closeErr)
		}
		meta, _ := json.Marshal(packet.Summary)
		t.Log(string(meta))
	}
}
