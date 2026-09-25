package designevidence

import (
	"bytes"
	"fmt"
	"image/png"
	"os"

	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

const (
	maxImageBytes  = 8 << 20
	maxImageEdge   = 8192
	maxImagePixels = 16_000_000
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// Image is one exact, receipt-declared PNG from a freshly checked private
// evidence bundle. PNG is detached from the stored bundle and safe to mutate.
type Image struct {
	Evidence designreceipts.Evidence
	PNG      []byte
	Width    int
	Height   int
}

// ReadImage reuses the same bundle, catalog, receipt and original-membership
// checks as Read. It never exposes inputs or non-image evidence bytes.
func ReadImage(root, id, slot string) (Image, error) {
	_, checked, err := readCheckedBundle(root, id)
	if err != nil {
		return Image{}, err
	}
	var evidence designreceipts.Evidence
	found := false
	for _, result := range checked.Receipt.Results {
		for _, candidate := range result.Evidence {
			if candidate.ID == slot && candidate.Role == "image" {
				evidence = candidate
				found = true
			}
		}
	}
	if !found {
		return Image{}, fmt.Errorf("designevidence: image evidence not found: %w", os.ErrNotExist)
	}
	body, ok := checked.Bundle.Content[slot]
	if !ok || len(body) == 0 || len(body) > maxImageBytes || !bytes.HasPrefix(body, pngSignature) {
		return Image{}, fmt.Errorf("designevidence: image evidence is not a bounded PNG")
	}
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > maxImageEdge || config.Height > maxImageEdge || config.Width > maxImagePixels/config.Height {
		return Image{}, fmt.Errorf("designevidence: image evidence has invalid PNG dimensions")
	}
	if _, err := png.Decode(bytes.NewReader(body)); err != nil {
		return Image{}, fmt.Errorf("designevidence: image evidence failed PNG validation")
	}
	return Image{Evidence: evidence, PNG: bytes.Clone(body), Width: config.Width, Height: config.Height}, nil
}
