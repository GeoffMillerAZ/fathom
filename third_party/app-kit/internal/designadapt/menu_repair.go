package designadapt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Keep the repair implementation independent from the immutable P07 base inputs.
// Each delivered executable/style asset has its own identity.
//
//go:embed menu_repair.go
var menuRepairRule string

//go:embed menu_repair.mjs
var menuRepairModule string

//go:embed menu_repair.css
var menuRepairCSS string

// MenuRepairArtifact is the explicit private P07 repair delivery envelope. It is
// generation evidence, not target certification or a runtime installation.
type MenuRepairArtifact struct {
	SchemaVersion int          `json:"schema_version"`
	Adapter       string       `json:"adapter"`
	Base          MenuArtifact `json:"base"`
	RuleSHA256    string       `json:"rule_sha256"`
	ModuleSHA256  string       `json:"module_sha256"`
	CSSSHA256     string       `json:"css_sha256"`
	Module        string       `json:"module"`
	CSS           string       `json:"css"`
	Mount         struct {
		Gap  int `json:"gap"`
		Edge int `json:"edge"`
	} `json:"mount"`
	Verification string `json:"verification"`
}

// ErrMenuRepairDeliveryDrift means an inspected repair delivery differs from
// current source replay. Source/replay and operational errors are distinct.
var ErrMenuRepairDeliveryDrift = errors.New("menu repair delivery differs from current replay")

// AdaptMenuRepair accepts only the frozen P07 source and independently rebuilds
// its unchanged base before binding the reviewed repair companion.
func AdaptMenuRepair(source designconvert.Source, body []byte) (MenuRepairArtifact, error) {
	base, err := AdaptMenu(source, body)
	if err != nil {
		return MenuRepairArtifact{}, err
	}
	return newMenuRepairArtifact(base), nil
}

// VerifyMenuRepairReplay checks the original source, nested base and companion
// bytes against current embedded inputs. It does not certify browser behavior.
func VerifyMenuRepairReplay(artifact MenuRepairArtifact, body []byte) error {
	return verifyMenuRepairReplayWith(artifact, body, AdaptMenu)
}

func verifyMenuRepairReplayWith(artifact MenuRepairArtifact, body []byte, generate func(designconvert.Source, []byte) (MenuArtifact, error)) error {
	if len(artifact.Module) > 64<<10 || len(artifact.CSS) > 16<<10 {
		return fmt.Errorf("menu repair: artifact exceeds replay bounds")
	}
	if err := verifyMenuReplayWith(artifact.Base, body, generate); err != nil {
		return err
	}
	base, err := generate(artifact.Base.Source, body)
	if err != nil {
		return err
	}
	want := newMenuRepairArtifact(base)
	if !reflect.DeepEqual(artifact, want) {
		return fmt.Errorf("menu repair: replay mismatch")
	}
	return nil
}

func newMenuRepairArtifact(base MenuArtifact) MenuRepairArtifact {
	digest := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	artifact := MenuRepairArtifact{
		SchemaVersion: 1,
		Adapter:       "p07-menu-repair-v1",
		Base:          base,
		RuleSHA256:    digest(menuRepairRule),
		ModuleSHA256:  digest(menuRepairModule),
		CSSSHA256:     digest(menuRepairCSS),
		Module:        menuRepairModule,
		CSS:           menuRepairCSS,
		Verification:  "not-verified",
	}
	artifact.Mount.Gap = 8
	artifact.Mount.Edge = 16
	return artifact
}

// WriteMenuRepairDelivery exports a new exact private eight-file repair
// inventory. Replay completes before any filesystem access.
func WriteMenuRepairDelivery(dir string, artifact MenuRepairArtifact, body []byte) error {
	return writeMenuRepairDeliveryWith(dir, artifact, body, AdaptMenu)
}

func writeMenuRepairDeliveryWith(dir string, artifact MenuRepairArtifact, body []byte, generate func(designconvert.Source, []byte) (MenuArtifact, error)) error {
	if err := verifyMenuRepairReplayWith(artifact, body, generate); err != nil {
		return err
	}
	files, err := menuRepairDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "menu repair delivery")
}

// CheckMenuRepairDelivery checks the exact repair inventory without modifying it.
func CheckMenuRepairDelivery(dir string, artifact MenuRepairArtifact, body []byte) error {
	return checkMenuRepairDeliveryWith(dir, artifact, body, AdaptMenu)
}

func checkMenuRepairDeliveryWith(dir string, artifact MenuRepairArtifact, body []byte, generate func(designconvert.Source, []byte) (MenuArtifact, error)) error {
	if err := verifyMenuRepairReplayWith(artifact, body, generate); err != nil {
		return err
	}
	files, err := menuRepairDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "menu repair delivery", ErrMenuRepairDeliveryDrift)
}

const menuRepairDeliveryUsage = `# Private P07 explicit menu repair

Licensed derivative: keep this directory and generated output private. Preserve
the original source and Elements runtime license obligations. A matching replay
is not target certification or real-consumer acceptance.

Copy this immutable directory to an app-owned private build directory. Compile
menu.templ with github.com/a-h/templ/cmd/templ v0.3.1020 using the app's pinned
module. Generated Go depends only on templ and the standard library, not app-kit.
The app supplies the pinned Elements 1.0.22 module, fonts and Tailwind 4 build.
Its source CSS build scans both menu.templ and menu.go.

Load source CSS first, menu.css second, and menu_repair.css last. Import and mount
the repair explicitly after Elements is defined; retain its disposer and call it
before removing or remounting the wrapper:

    import { mountMenuRepair } from "./menu_repair.mjs";
    await customElements.whenDefined("el-menu");
    const root = document.querySelector("[data-appkit-menu]");
    if (!root) throw new Error("menu wrapper unavailable");
    const disposeMenuRepair = mountMenuRepair(root, { gap: 8, edge: 16 });
    // Call disposeMenuRepair() before removing or replacing root.

There is no automatic bootstrap, global installation or test handle. Mount each
connected selected wrapper exactly once. The reviewed defaults are an 8px anchor
gap and 16px viewport edge; changes require separate consumer evidence. Elements
continues to own placement, keyboard input, selection and dismissal.

NewMenuEnhanced renders the selected [data-appkit-menu] wrapper used by this
companion. NewMenu remains the unmounted source-styled typed choice, and
MenuReference remains the unmounted constant original. Serve MenuReference only
with form submission denied; its placeholder action is not a product workflow.

Decode defaults.json into Menu and fill app-owned destinations and, when used,
the local POST action and CSRF name/token. The app owns authentication,
authorization, CSRF issuance/checking, duplicate-effect prevention, results and
recovery. Disabled is not authorization. Keep an app-owned fallback for essential
destinations when the runtime cannot load. Do not nest a POST-bearing menu inside
another form or expose reference placeholder actions as working controls.

All eight files must match fresh appkit data assets-adapt-menu -repair -check
DIRECTORY replay. Existing output is never overwritten, repaired or upgraded.
artifact.json is written last; interrupted output remains for inspection. No
tool, runtime, dependency, backend or global configuration is installed here.
`

func menuRepairDeliveryFiles(artifact MenuRepairArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"menu.templ", []byte(artifact.Base.Templ)},
		{"menu.go", []byte(artifact.Base.Model)},
		{"menu.css", []byte(artifact.Base.CSS)},
		{"defaults.json", artifact.Base.Defaults},
		{"menu_repair.mjs", []byte(artifact.Module)},
		{"menu_repair.css", []byte(artifact.CSS)},
		{"USAGE.md", []byte(menuRepairDeliveryUsage)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
