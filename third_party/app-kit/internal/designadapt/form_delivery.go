package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrFormDeliveryDrift = errors.New("form delivery differs from current replay")

// WriteFormDelivery exports frozen P05 into a new private directory. Verify original
// archive access first; no compiler, dependencies, backend or certification is supplied.
func WriteFormDelivery(dir string, artifact FormArtifact, source []byte) error {
	if err := VerifyFormReplay(artifact, source); err != nil {
		return err
	}
	files, err := formDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "form delivery")
}

// CheckFormDelivery compares all five immutable files against fresh source replay.
func CheckFormDelivery(dir string, artifact FormArtifact, source []byte) error {
	if err := VerifyFormReplay(artifact, source); err != nil {
		return err
	}
	files, err := formDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "form delivery", ErrFormDeliveryDrift)
}

const formDeliveryReadme = `# Private P05 email field

Licensed derivative: keep input and generated output private with source obligations.
This is a presentation adapter, not an email validator, account backend or certificate.

Copy this immutable directory into an app-owned private package before building.
Compile form.templ with github.com/a-h/templ/cmd/templ v0.3.1020 via a pinned go.mod
tool directive. Package name: converted. Runtime dependency: github.com/a-h/templ
v0.3.1020 only, not app-kit. The app owns Go/tool pins, go.sum checksums, Tailwind 4
CSS scanning BOTH form.templ and form.go (generated class constants), and fonts.
No CSS generation, dependency installation, compiler or web server runs in export.

Decode defaults.json into EmailField to reproduce the source error-state specimen.
Set ID, Name, Label, Value, Placeholder, Error, Required and Autofocus for app use.
NewEmailField validates bounded presentation data before rendering; handle its error
before writing an HTTP response. Malformed email strings can be displayed as rejected
input. Error empty selects the explicitly authored neutral palette and omits error
icon/description/invalid attributes. Error nonempty retains source-error styling.
Reserve both ID and ID-error uniquely in the page. Autofocus is opt-in, never automatic.

The app owns native form action/method, validation, CSRF, authorization, save/retry
semantics and success truth. A server failure must not falsely mark a valid field as
invalid. Preserve input and offer actionable recovery. Neither a native submit nor
an address passing browser constraints proves an authorized durable save occurred.
Source sizing remains unchanged; measure target size, contrast and long labels in
the app. No Javascript runtime or generic form/state framework is required or added.

Check with appkit data assets-adapt-form -check DIRECTORY using the original archive.
All five files, exact modes and regular single-link identity must match current replay.
artifact.json is written last, not crash-atomic publication. Never build/edit in place;
incomplete output is retained for inspection. Choose a new directory after failures.
Matching proves delivery replay, not browser quality, real adoption or distribution rights.
`

func formDeliveryFiles(artifact FormArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"form.templ", []byte(artifact.Templ)}, {"form.go", []byte(artifact.Model)},
		{"defaults.json", artifact.Defaults}, {"README.md", []byte(formDeliveryReadme)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
