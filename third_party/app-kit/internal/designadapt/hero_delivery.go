package designadapt

import (
	"encoding/json"
	"errors"
	"os"
)

// ErrHeroDeliveryDrift means an inspected delivery is absent or differs from
// current replay. Other errors mean the check could not be completed.
var ErrHeroDeliveryDrift = errors.New("hero delivery differs from current replay")

// WriteHeroDelivery projects an independently verified private artifact into a
// new directory. It does not download source assets, install tools or compile.
func WriteHeroDelivery(dir string, artifact HeroArtifact, source []byte) error {
	if err := VerifyHeroReplay(artifact, source); err != nil {
		return err
	}
	files, err := heroDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writeHeroDelivery(dir, files)
}

// CheckHeroDelivery compares every delivered byte to fresh frozen-source replay.
// A matching delivery is not target, visual or task-quality certification.
func CheckHeroDelivery(dir string, artifact HeroArtifact, source []byte) error {
	if err := VerifyHeroReplay(artifact, source); err != nil {
		return err
	}
	files, err := heroDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkHeroDelivery(dir, files)
}

type heroDeliveryFile = privateDeliveryFile

const heroDeliveryUsage = `# Private P01 hero component

Licensed derivative: keep this directory and generated outputs private, and
preserve the source license obligations. This is not a certified target.

This immutable directory contains package converted. Copy it to an app-owned
private build directory before compiling hero.templ with
github.com/a-h/templ/cmd/templ v0.3.1020. Both constructors depend at runtime only
on github.com/a-h/templ v0.3.1020, never app-kit. The consumer owns its Go module,
tool pin and go.sum. Do not compile or edit inside the private source archive store.

Decode defaults.json into Hero, then supply all six required URLs: three action
destinations and the light logo, dark logo and hero image. They are intentionally
blank in defaults. Each must be a local absolute path or explicit HTTPS URL.
SourceAssets records the original external asset URLs as provenance only; export
does not download or bundle them. The app owns asset selection, rights, pinning,
delivery, routes and destination behavior.

Set ImageDecorative deliberately. A decorative image requires an exactly empty
ImageAlt; a non-decorative image requires bounded plain alt text. Other copy is
plain escaped text. Handle constructor errors before writing a response.

NewHero preserves the original responsive and dark source composition.
NewHeroEnhanced is an explicit opt-in to hero.css; include it after source CSS.
Neither variant currently claims visual fidelity, task quality or accessibility
certification. Tailwind 4 CSS generation, source scanning and fonts are app-owned.

artifact.json is written last but does not make publication crash-atomic. All six
files must match a fresh appkit data assets-adapt-hero -check DIRECTORY against the
verified original archive. Checks reject missing, modified, extra or linked files.
An interrupted export remains incomplete; inspect it and choose a new path.
Delivery matching does not prove downstream compilation, adoption or asset access.
`

func heroDeliveryFiles(artifact HeroArtifact) ([]heroDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []heroDeliveryFile{
		{"hero.templ", []byte(artifact.Templ)},
		{"hero.go", []byte(artifact.Model)},
		{"hero.css", []byte(artifact.CSS)},
		{"defaults.json", artifact.Defaults},
		{"USAGE.md", []byte(heroDeliveryUsage)},
		{"artifact.json", append(body, '\n')}, // Completion marker, never a certificate.
	}, nil
}

func writeHeroDelivery(dir string, files []heroDeliveryFile) error {
	return writePrivateDelivery(dir, files, "hero delivery")
}

func writeHeroFiles(parent *os.Root, leaf string, files []heroDeliveryFile) error {
	return writePrivateFiles(parent, leaf, files)
}

func checkHeroDelivery(dir string, files []heroDeliveryFile) error {
	return checkPrivateDelivery(dir, files, "hero delivery", ErrHeroDeliveryDrift)
}
