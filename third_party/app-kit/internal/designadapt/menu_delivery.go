package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrMenuDeliveryDrift = errors.New("menu delivery differs from current replay")

func WriteMenuDelivery(dir string, artifact MenuArtifact, source []byte) error {
	if err := VerifyMenuReplay(artifact, source); err != nil {
		return err
	}
	files, err := menuDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "menu delivery")
}

func CheckMenuDelivery(dir string, artifact MenuArtifact, source []byte) error {
	if err := VerifyMenuReplay(artifact, source); err != nil {
		return err
	}
	files, err := menuDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "menu delivery", ErrMenuDeliveryDrift)
}

const menuDeliveryUsage = `# Private P07 menu adaptation

Licensed derivative: keep this directory and generated outputs private. Preserve
source/runtime license obligations. Matching delivery is not target certification.

Copy this immutable directory into an app-owned private build directory. Compile
menu.templ with github.com/a-h/templ/cmd/templ v0.3.1020 using the app's pinned Go
module/go.sum. Runtime Go imports need only templ and the standard library, not
app-kit. Source CSS generation, fonts and the exact Elements 1.0.22 module named in
artifact.json are app-owned. No runtime, tool, font or backend is installed here.

MenuReference() is the unchanged constant source reference, with placeholder links
and POST action. It is not an account workflow. Serve reference demonstrations with
form submission denied and do not expose them as working product controls.

Decode defaults.json into Menu, then fill each link URL and the optional Submit's
Action, CSRFName and CSRFToken. Blank defaults deliberately cannot construct a real
menu. NewMenu validates/snapshots these inputs and retains the source styling while
adding a non-submit trigger, repeated links and a conditional POST form with hidden
CSRF field and disabled state. NewMenuEnhanced separately opts into menu.css, loaded
after source CSS, for larger targets, wrapping, viewport bounds and painted focus.

Links accept root-relative or explicit HTTPS destinations. POST endpoints accept
same-origin absolute local paths without query or fragment. Zero to twelve links
and one optional POST action are supported; at least one action must exist. Labels
and tokens are bounded escaped plain text. Handle errors before writing headers.

Authentication, authorization, CSRF issuance/checking, duplicate-effect prevention
and meaningful success/error/retry pages belong to the app. Disabled is presentation,
not permission enforcement. Do not nest the POST-bearing menu inside another form.
Do not mutate Elements-owned DOM with another renderer. No persisted/morph state or
no-script menu support is promised; provide essential destinations elsewhere when
the runtime cannot load. No inline handlers or new JavaScript runtime are generated.

All six files must match fresh appkit data assets-adapt-menu -check DIRECTORY replay
against the original archive. Existing output is never overwritten or repaired.
artifact.json is written last, not crash-atomic publication. Failed exports remain
for inspection. Export/check does not prove compilation, browser quality or adoption.
`

func menuDeliveryFiles(a MenuArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"menu.templ", []byte(a.Templ)}, {"menu.go", []byte(a.Model)},
		{"menu.css", []byte(a.CSS)}, {"defaults.json", a.Defaults},
		{"USAGE.md", []byte(menuDeliveryUsage)}, {"artifact.json", append(body, '\n')},
	}, nil
}
