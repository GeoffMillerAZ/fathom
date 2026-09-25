package designevidence

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

func TestReadImageReturnsExactDetachedDeclaredPNG(t *testing.T) {
	body := testPNG(t)
	root, entry := publishImageFixture(t, body, "image")
	got, err := ReadImage(root, entry.ID, "capture")
	if err != nil {
		t.Fatal(err)
	}
	if got.Evidence.ID != "capture" || got.Evidence.Role != "image" || got.Width != 2 || got.Height != 3 || !bytes.Equal(got.PNG, body) {
		t.Fatalf("unexpected image: %#v", got)
	}
	got.PNG[0] ^= 0xff
	again, err := ReadImage(root, entry.ID, "capture")
	if err != nil || !bytes.Equal(again.PNG, body) {
		t.Fatal("returned image aliases stored evidence")
	}
	if _, err := ReadImage(root, entry.ID, "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing slot error = %v", err)
	}

	reportRoot, reportEntry := publishImageFixture(t, body, "report")
	if _, err := ReadImage(reportRoot, reportEntry.ID, "capture"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-image evidence exposed: %v", err)
	}
}

func TestReadImageRejectsInvalidOrUnboundedPNG(t *testing.T) {
	valid := testPNG(t)
	badCRC := bytes.Clone(valid)
	idat := bytes.Index(badCRC, []byte("IDAT"))
	if idat < 0 {
		t.Fatal("test PNG lacks IDAT")
	}
	badCRC[idat+4] ^= 0xff
	oversized := append(bytes.Clone(pngSignature), make([]byte, maxImageBytes+1-len(pngSignature))...)
	for name, body := range map[string][]byte{
		"unsupported": []byte("not a PNG"),
		"crc":         badCRC,
		"edge":        pngDimensions(t, valid, maxImageEdge+1, 1),
		"pixels":      pngDimensions(t, valid, 5000, 4000),
		"bytes":       oversized,
	} {
		t.Run(name, func(t *testing.T) {
			root, entry := publishImageFixture(t, body, "image")
			if _, err := ReadImage(root, entry.ID, "capture"); err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid PNG accepted or misclassified: %v", err)
			}
		})
	}
}

func TestReadImageRechecksBundleAndOriginal(t *testing.T) {
	t.Run("bundle", func(t *testing.T) {
		root, entry := publishImageFixture(t, testPNG(t), "image")
		name := filepath.Join(root, "evidence", "entries", strings.TrimPrefix(entry.ID, "evidence@")+".json")
		if err := os.WriteFile(name, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadImage(root, entry.ID, "capture"); err == nil {
			t.Fatal("tampered bundle accepted")
		}
	})
	t.Run("source", func(t *testing.T) {
		root, entry := publishImageFixture(t, testPNG(t), "image")
		assets, err := designassets.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "archives", assets.List()[0].SHA256+".zip"), []byte("tampered source archive"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadImage(root, entry.ID, "capture"); err == nil {
			t.Fatal("tampered original membership accepted")
		}
	})
}

func publishImageFixture(t *testing.T, imageBody []byte, role string) (string, Entry) {
	t.Helper()
	root, member, contractBody, receiptBody, _, content := evidenceFixture(t)
	var contract designreceipts.Contract
	var receipt designreceipts.Receipt
	if err := json.Unmarshal(contractBody, &contract); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(receiptBody, &receipt); err != nil {
		t.Fatal(err)
	}
	asset := designreceipts.Asset{ID: "capture", SHA256: designreceipts.Digest(imageBody), Bytes: len(imageBody)}
	contract.Gates["visual"] = designreceipts.Gate{Applicable: true, Scope: "bounded image test", Slots: []designreceipts.Slot{{ID: asset.ID, Role: role}}}
	receipt.Results["visual"] = designreceipts.Result{Status: "passed", Evidence: []designreceipts.Evidence{{Asset: asset, Role: role}}}
	content[asset.ID] = bytes.Clone(imageBody)
	var err error
	contractBody, err = designreceipts.EncodeContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	receipt.ContractSHA256 = designreceipts.Digest(contractBody)
	receiptBody, err = designreceipts.EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := Publish(root, member, contractBody, receiptBody, designreceipts.Expected{ContractSHA256: receipt.ContractSHA256, SourceRevision: receipt.SourceRevision, ArtifactSHA256: receipt.ArtifactSHA256}, content)
	if err != nil {
		t.Fatal(err)
	}
	return root, entry
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 0x7f, G: 0x31, B: 0xc2, A: 0xff})
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func pngDimensions(t *testing.T, body []byte, width, height uint32) []byte {
	t.Helper()
	out := bytes.Clone(body)
	if len(out) < 33 || !bytes.Equal(out[12:16], []byte("IHDR")) {
		t.Fatal("invalid test PNG")
	}
	binary.BigEndian.PutUint32(out[16:20], width)
	binary.BigEndian.PutUint32(out[20:24], height)
	binary.BigEndian.PutUint32(out[29:33], crc32.ChecksumIEEE(out[12:29]))
	return out
}
