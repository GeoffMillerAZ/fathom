package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrFAQDeliveryDrift = errors.New("FAQ delivery differs from current replay")

func WriteFAQDelivery(dir string, artifact FAQArtifact, source []byte) error {
	if err := VerifyFAQReplay(artifact, source); err != nil {
		return err
	}
	files, err := faqDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "FAQ delivery")
}

func CheckFAQDelivery(dir string, artifact FAQArtifact, source []byte) error {
	if err := VerifyFAQReplay(artifact, source); err != nil {
		return err
	}
	files, err := faqDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "FAQ delivery", ErrFAQDeliveryDrift)
}

const faqDeliveryUsage = `# Private P03 FAQ

Licensed derivative: keep sources, defaults and generated outputs private; retain
the source obligations. This is a candidate, not a certified target or help backend.
Copy the immutable export to a separate app-owned private build directory. Package:
converted. Compile faq.templ with github.com/a-h/templ/cmd/templ v0.3.1020; runtime Go
dependency is github.com/a-h/templ v0.3.1020, not app-kit. The app owns Go/tools/go.sum,
Tailwind 4 source scanning, fonts, CSP and serving. Nothing is installed or run here.

Decode defaults.json into FAQ for NewFAQReference: its source DOM/classes require
host-provided @tailwindplus/elements 1.0.22 with the exact ESM/tarball hashes recorded
in artifact.json. No script is included or installed. This preserves the reference,
including its small targets and runtime dependency; it is not a usability upgrade.

NewFAQDetails(data) deliberately uses native details/summary, with no Elements module
or JavaScript requirement. Include faq.css with source-appropriate Tailwind output.
Its scoped rules add wrapping, keyboard focus, 44px targets and native open icons.
Do not present this reviewed semantic change as an exact source-DOM conversion.
Neither constructor silently falls back to the other or imposes exclusive opening.

FAQ has IDPrefix, Heading and Items; FAQItem has Key, Question, Answer and Open.
Use distinct ID prefixes across the complete document and unique item keys. Text
is plain escaped content, not HTML. Inputs are bounded and snapshotted; handle
constructor errors before writing a response. The app owns content policy, empty
states and persistence. Browser disclosure state resets to the model on remount;
no Datastar morph, persisted state, rich answers or deep-link contract is implied.

appkit data assets-adapt-faq -check DIRECTORY compares all six immutable files and
private modes against fresh frozen-source replay. artifact.json is written last.
This is not crash-atomic publication; incomplete output stays for inspection.
Never compile/edit in the immutable export; choose a new path after a failed write.
Delivery matching does not prove browser quality, actual adoption or license rights.
`

func faqDeliveryFiles(artifact FAQArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"faq.templ", []byte(artifact.Templ)}, {"faq_model.go", []byte(artifact.Model)},
		{"faq.css", []byte(artifact.CSS)}, {"defaults.json", artifact.Defaults},
		{"USAGE.md", []byte(faqDeliveryUsage)}, {"artifact.json", append(body, '\n')},
	}, nil
}
