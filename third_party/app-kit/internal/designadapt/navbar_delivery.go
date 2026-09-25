package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrNavbarDeliveryDrift = errors.New("navbar delivery differs from current replay")

func WriteNavbarDelivery(dir string, artifact NavbarArtifact, source []byte) error {
	if err := VerifyNavbarReplay(artifact, source); err != nil {
		return err
	}
	files, err := navbarDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "navbar delivery")
}

func CheckNavbarDelivery(dir string, artifact NavbarArtifact, source []byte) error {
	if err := VerifyNavbarReplay(artifact, source); err != nil {
		return err
	}
	files, err := navbarDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "navbar delivery", ErrNavbarDeliveryDrift)
}

const navbarDeliveryUsage = `# Private P09 responsive navbar adaptation

Licensed derivative: retain this directory and compiled output privately. Preserve
source/runtime license obligations. Matching delivery is not target certification.

Copy immutable input into a separate app-owned private build directory. Compile
navbar.templ with pinned templ v0.3.1020. Generated Go imports only templ and the
standard library, not app-kit. The host app supplies source-scanned Tailwind 4 CSS,
fonts, approved images and exact Elements 1.0.22 runtime identified by artifact.json.
No compiler, runtime, image, dependency or backend is downloaded or installed here.

NavbarReference() preserves the constant original, including placeholder links and
external imagery. It is a private design specimen, not functioning application
navigation. Deny placeholder actions when serving a reference demonstration.

NewNavbar(Navbar) validates, snapshots and escapes app-owned presentation. IDPrefix
must be document-unique. Label names the navigation landmark and MobileLabel names
the disclosure toggle. Brand requires explicit light/dark image URLs and one meaningful
Alt; it does not invent a home destination. Links contains one to twelve items, each
with a unique identifier Key, visible Label and GET URL. CurrentKey selects exactly
one matching link; empty CurrentKey deliberately marks none active.

Notifications is an optional labeled GET destination. Account is optional; when
present it requires a labeled decorative avatar and one to twelve labeled GET links.
Its ImageURL is decorative because Account.Label names the control. Nil options omit
their controls entirely. Empty defaults are intentionally unconstructable until the
app supplies identifiers, labels, route keys, URLs and image choices. No authentication,
notification provider, POST, sign-out semantics, mutation or account backend is implied.

Elements owns mobile disclosure, account menu focus, keyboard movement, Escape,
dismissal and responsive lifecycle. Keep each subtree under one runtime owner. Supply
essential destinations elsewhere if the runtime fails. No inline handlers, alternative
state machine, auto-bootstrap, Datastar morph ownership or no-script menu is delivered.

All labels are bounded escaped plain UTF-8. Destinations accept root-relative or
explicit HTTPS GET URLs without userinfo. They reject malformed escapes, decoded
controls, backslashes and protocol-relative values. Handle constructor errors before
writing response headers. Caller slices and optional objects are snapshotted.

The five files are navbar.templ, navbar.go, defaults.json, USAGE.md and artifact.json.
Existing output is never overwritten or repaired. Exact checking regenerates expected
bytes from the independently authorized original. artifact.json is written last;
interrupted output remains for inspection. Compilation, browser behavior, imagery,
accessibility, target quality and consumer adoption remain separate evidence gates.
`

func navbarDeliveryFiles(a NavbarArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"navbar.templ", []byte(a.Templ)}, {"navbar.go", []byte(a.Model)},
		{"defaults.json", a.Defaults}, {"USAGE.md", []byte(navbarDeliveryUsage)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
