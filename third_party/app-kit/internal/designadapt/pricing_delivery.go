package designadapt

import (
	"encoding/json"
	"errors"
	"os"
)

// ErrPricingDeliveryDrift means an inspected delivery is absent or differs from
// current replay. Other errors mean the check could not be completed.
var ErrPricingDeliveryDrift = errors.New("pricing delivery differs from current replay")

// WritePricingDelivery projects an independently verified private artifact into
// a NEW directory. Callers must verify archive access before calling. It neither
// installs dependencies nor compiles code. Existing paths are never overwritten;
// an I/O failure leaves incomplete private output for explicit operator review.
func WritePricingDelivery(dir string, artifact PricingArtifact, source []byte) error {
	if err := VerifyPricingReplay(artifact, source); err != nil {
		return err
	}
	files, err := pricingDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePricingDelivery(dir, files)
}

// CheckPricingDelivery compares every delivered byte to fresh source replay, not
// to hashes asserted by the delivered artifact. This is not target certification.
func CheckPricingDelivery(dir string, artifact PricingArtifact, source []byte) error {
	if err := VerifyPricingReplay(artifact, source); err != nil {
		return err
	}
	files, err := pricingDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPricingDelivery(dir, files)
}

type pricingDeliveryFile = privateDeliveryFile

const pricingDeliveryReadme = `# Private P02 pricing component

Licensed derivative: keep this directory and generated outputs private. Preserve
the source license obligations. This is not a certified target or billing system.

This immutable directory contains package converted. Keep app wrappers outside it.
Copy its contents to an app-owned build directory before compiling pricing.templ
with github.com/a-h/templ/cmd/templ v0.3.1020 (a pinned go.mod tool directive).
The only component runtime dependency is github.com/a-h/templ v0.3.1020; app-kit
is not a runtime dependency. The consumer owns its Go module, tool pins and go.sum.
Do not install tools or build inside the private source archive store.

Call NewPricing(Pricing{...}) or explicitly NewPricingEnhanced(Pricing{...}); handle
validation errors before writing a response. defaults.json contains private example
copy, not application data; URL is deliberately empty. Supply a real local absolute
path or explicit HTTPS destination. Values are plain text; price/currency are
app-formatted strings, never computed money. Features accepts 1..12 items.

Supply pinned Tailwind 4 CSS scanning pricing.templ and the intended font. Neither
is bundled or installed. For NewPricingEnhanced include pricing.css AFTER source
CSS; its wrapper scopes larger link targets and long-copy containment. Faithful
NewPricing preserves the original 36px link and hostile-copy overflow behavior.
No JavaScript, checkout, auth, backend or application framework is included.

artifact.json is written last but does not make publication crash-atomic. All six
files must match a fresh appkit data assets-adapt-pricing -check DIRECTORY against
the verified original archive. Checks reject missing, modified, extra or linked
files. An interrupted export remains incomplete; inspect it and choose a new path.
Checks do not certify visual/behavioral quality, downstream compilation or adoption.
`

func pricingDeliveryFiles(artifact PricingArtifact) ([]pricingDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []pricingDeliveryFile{
		{"pricing.templ", []byte(artifact.Templ)},
		{"pricing.go", []byte(artifact.Model)},
		{"pricing.css", []byte(artifact.CSS)},
		{"defaults.json", artifact.Defaults},
		{"README.md", []byte(pricingDeliveryReadme)},
		{"artifact.json", append(body, '\n')}, // Completion marker, never a certificate.
	}, nil
}

func writePricingDelivery(dir string, files []pricingDeliveryFile) error {
	return writePrivateDelivery(dir, files, "pricing delivery")
}

func writePricingFiles(parent *os.Root, leaf string, files []pricingDeliveryFile) error {
	return writePrivateFiles(parent, leaf, files)
}

func checkPricingDelivery(dir string, files []pricingDeliveryFile) error {
	return checkPrivateDelivery(dir, files, "pricing delivery", ErrPricingDeliveryDrift)
}
