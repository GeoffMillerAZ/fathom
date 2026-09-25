package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrDrawerDeliveryDrift = errors.New("drawer delivery differs from current replay")

func WriteDrawerDelivery(dir string, artifact DrawerArtifact, source []byte) error {
	if err := VerifyDrawerReplay(artifact, source); err != nil {
		return err
	}
	files, err := drawerDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "drawer delivery")
}

func CheckDrawerDelivery(dir string, artifact DrawerArtifact, source []byte) error {
	if err := VerifyDrawerReplay(artifact, source); err != nil {
		return err
	}
	files, err := drawerDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "drawer delivery", ErrDrawerDeliveryDrift)
}

const drawerDeliveryUsage = `# Private P08 profile drawer adaptation

Licensed derivative: retain this directory and compiled output privately. Preserve
source/runtime license obligations. Matching delivery is not target certification.

Copy immutable input to a separate app-owned private build directory. Compile
drawer.templ with pinned templ v0.3.1020. Runtime Go imports require only templ and
the standard library, not app-kit. The app supplies source-scanned Tailwind 4 CSS,
fonts, approved images and the exact Elements 1.0.22 module named in artifact.json.
No compiler, runtime, asset or backend is downloaded, installed or started here.

DrawerReference() preserves the constant original, including inert placeholder
actions and original external image URL. It is a design specimen, not a working
profile. NewDrawer(Drawer) validates and snapshots typed app-owned presentation.
Give each instance a document-unique IDPrefix and an explicit ImageURL. Decorative
images require empty ImageAlt; meaningful images require nonblank descriptive alt.
Defaults leave IDs/images unbound and omit actions and claimed online state.

Primary/Secondary are optional GET destinations, not send/call actions. MenuLinks
are navigation only; no clipboard or mutation behavior is implemented. Set honest
labels describing the linked task. Never use a GET destination to perform a sensitive
mutation. An empty action model is a valid read-only drawer. OnlineLabel is shown
with the original online indicator only when the app supplies a truthful nonblank
label; no presence provider or freshness inference is included.

Details retain definition-list semantics. Kind is text, paragraph or date; Date must
be an actual canonical YYYY-MM-DD for date rows and empty otherwise. All values are
bounded escaped UTF-8 plain text; labels/details reject control characters. Primary,
Secondary, MenuLinks and Details are copied before rendering. The constructor does
not check document-wide uniqueness: the app owns the instance ID namespace.

Elements owns dialog modality, nested menu focus, Escape and dismissal. Keep those
DOM subtrees under one owner. No Datastar morph/state persistence, no-script access,
screen-reader/native-host conformance or complete focus/zoom claim is supplied.
Provide essential profile/destination access elsewhere when runtime loading fails.
Source styles remain unchanged apart from explicit IDs/naming, typed image/content,
optional sections and action-button-to-link adaptations. No automatic enhancement
stylesheet, runtime patch, image download or public catalog target is introduced.

The five files are drawer.templ, drawer.go, defaults.json, USAGE.md and artifact.json.
Existing output is never overwritten/repaired. Exact checking regenerates expectations
from the independently authorized original, not delivered hashes alone. artifact.json
is written last; interrupted export can leave an incomplete directory for inspection.
Compilation, browser tasks and actual consumer adoption are separate evidence gates.
`

func drawerDeliveryFiles(a DrawerArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"drawer.templ", []byte(a.Templ)}, {"drawer.go", []byte(a.Model)},
		{"defaults.json", a.Defaults}, {"USAGE.md", []byte(drawerDeliveryUsage)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
