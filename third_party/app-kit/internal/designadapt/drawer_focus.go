package designadapt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

// These inputs remain independent from the immutable five-file P08 base.
//
//go:embed drawer_focus.go
var drawerFocusRule string

//go:embed drawer_focus.cue
var drawerFocusSchema string

//go:embed drawer_focus.css
var drawerFocusCSS string

// DrawerFocusArtifact selects a private stylesheet and app-owned wrapper. It adds
// no constructor or interaction runtime and is not target certification.
type DrawerFocusArtifact struct {
	SchemaVersion    int            `json:"schema_version"`
	Adapter          string         `json:"adapter"`
	Base             DrawerArtifact `json:"base"`
	RuleSHA256       string         `json:"rule_sha256"`
	CSSSHA256        string         `json:"css_sha256"`
	CSS              string         `json:"css"`
	WrapperAttribute string         `json:"wrapper_attribute"`
	Verification     string         `json:"verification"`
}

var ErrDrawerFocusDeliveryDrift = errors.New("drawer focus delivery differs from current replay")

func AdaptDrawerFocus(source designconvert.Source, body []byte) (DrawerFocusArtifact, error) {
	base, err := AdaptDrawer(source, body)
	if err != nil {
		return DrawerFocusArtifact{}, err
	}
	return newDrawerFocusArtifact(base)
}

func newDrawerFocusArtifact(base DrawerArtifact) (DrawerFocusArtifact, error) {
	digest := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	a := DrawerFocusArtifact{
		SchemaVersion: 1, Adapter: "p08-drawer-focus-v1", Base: base,
		RuleSHA256: digest(drawerFocusRule + "\x00" + drawerFocusSchema),
		CSSSHA256:  digest(drawerFocusCSS), CSS: drawerFocusCSS,
		WrapperAttribute: "data-appkit-drawer-focus", Verification: "not-verified",
	}
	baseSchema, err := drawerImplementation.ReadFile("drawer.cue")
	if err != nil {
		return DrawerFocusArtifact{}, fmt.Errorf("drawer focus: base schema unavailable")
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		return DrawerFocusArtifact{}, fmt.Errorf("drawer focus: envelope encoding failed")
	}
	ctx := cuecontext.New()
	contract := ctx.CompileString(string(baseSchema) + "\n" + drawerFocusSchema).LookupPath(cue.ParsePath("#DrawerFocusArtifact"))
	if err := contract.Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return DrawerFocusArtifact{}, fmt.Errorf("drawer focus: generated envelope failed schema")
	}
	return a, nil
}

func VerifyDrawerFocusReplay(a DrawerFocusArtifact, body []byte) error {
	return verifyDrawerFocusReplayWith(a, body, AdaptDrawer)
}

func verifyDrawerFocusReplayWith(a DrawerFocusArtifact, body []byte, generate func(designconvert.Source, []byte) (DrawerArtifact, error)) error {
	if len(a.CSS) > 16<<10 {
		return fmt.Errorf("drawer focus: artifact exceeds replay bounds")
	}
	if err := verifyDrawerReplayWith(a.Base, body, generate); err != nil {
		return err
	}
	base, err := generate(a.Base.Source, body)
	if err != nil {
		return err
	}
	want, err := newDrawerFocusArtifact(base)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(a, want) {
		return fmt.Errorf("drawer focus: replay mismatch")
	}
	return nil
}

func WriteDrawerFocusDelivery(dir string, a DrawerFocusArtifact, body []byte) error {
	return writeDrawerFocusDeliveryWith(dir, a, body, AdaptDrawer)
}

func writeDrawerFocusDeliveryWith(dir string, a DrawerFocusArtifact, body []byte, generate func(designconvert.Source, []byte) (DrawerArtifact, error)) error {
	if err := verifyDrawerFocusReplayWith(a, body, generate); err != nil {
		return err
	}
	files, err := drawerFocusDeliveryFiles(a)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "drawer focus delivery")
}

func CheckDrawerFocusDelivery(dir string, a DrawerFocusArtifact, body []byte) error {
	return checkDrawerFocusDeliveryWith(dir, a, body, AdaptDrawer)
}

func checkDrawerFocusDeliveryWith(dir string, a DrawerFocusArtifact, body []byte, generate func(designconvert.Source, []byte) (DrawerArtifact, error)) error {
	if err := verifyDrawerFocusReplayWith(a, body, generate); err != nil {
		return err
	}
	files, err := drawerFocusDeliveryFiles(a)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "drawer focus delivery", ErrDrawerFocusDeliveryDrift)
}

const drawerFocusDeliveryUsage = `# Private P08 explicit focus repair

Licensed derivative: keep this directory and generated output private. Source/runtime
license obligations remain. Matching replay is not certification or consumer acceptance.

Copy this immutable directory to a separate app-owned private build directory. Compile
drawer.templ using pinned templ v0.3.1020 in the app module. Generated Go imports only
templ and the standard library, not app-kit. The app supplies source-scanned Tailwind 4
CSS, fonts, approved images and the exact Elements 1.0.22 module identified in base.runtime.
Nothing is installed, downloaded or started by this export.

NewDrawer(Drawer) remains the same validating, snapshotting constructor. Supply a
document-unique IDPrefix and explicit ImageURL; meaningful images require ImageAlt,
decorative images require empty alt. Defaults omit IDs/images, actions and online
presence claims. Primary/Secondary/MenuLinks are honestly labeled native GET destinations,
never inferred messaging/call/clipboard effects or sensitive mutations. The app owns
identity, presence truth/freshness, routes, authentication, authorization and recovery.

Opt in at BOTH seams: load drawer_focus.css after source CSS and wrap only the chosen
NewDrawer component in an app-owned <div data-appkit-drawer-focus>. The wrapper uses
display:contents; do not give it an interactive role or focus target. Do not wrap
DrawerReference(), unrelated content or other component families. No new constructor,
automatic enhancement, JavaScript mount/disposer or P07 repair module is supplied.

The stylesheet gives selected controls 44px minima, wrapping and inset 2px currentColor
focus rings. The nested trigger's outline uses source --color-gray-900/100 tokens
and the source prefers-color-scheme dark condition; text/icons keep their source color.
The already-tested close control keeps its source styling. Review final app CSS order,
contrast, clipping and narrow/zoom states; inherited colors are not universal guarantees.
Elements continues to own modal/menu focus, selection, Escape and placement. Do not add
a second state machine. Provide essential profile/destination access elsewhere when
runtime loading fails; this paint-only repair cannot supply runtime-failure recovery.

The original five-file export and DrawerReference remain unchanged when this variant
is not selected. This six-file inventory contains drawer.templ, drawer.go, defaults.json,
drawer_focus.css, USAGE.md and artifact.json. Its nested base is the exact original
artifact, not a renamed or modified constructor. Exact checking replays the authorized
source before inspecting output and refuses rehashed mutations, extras and mode drift.
Use appkit data assets-adapt-drawer -root PRIVATE_ASSETS -repair -check DIRECTORY.
Existing output is never overwritten or repaired; artifact.json is written last.
Compilation, browser tasks, full accessibility and real adoption remain separate gates.
`

func drawerFocusDeliveryFiles(a DrawerFocusArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"drawer.templ", []byte(a.Base.Templ)}, {"drawer.go", []byte(a.Base.Model)},
		{"defaults.json", a.Base.Defaults}, {"drawer_focus.css", []byte(a.CSS)},
		{"USAGE.md", []byte(drawerFocusDeliveryUsage)}, {"artifact.json", append(body, '\n')},
	}, nil
}
