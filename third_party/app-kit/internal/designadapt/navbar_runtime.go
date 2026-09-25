package designadapt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const navbarOriginalRuntimeSHA256 = "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"

// Selection is captured while the menu is still mounted, not after its timer.
// Elements remains the only interaction owner. A selected menu only receives
// delayed container focus if no item has since been selected and it is still open.
const navbarFocusBefore = `setTimeout(()=>this.focus({preventScroll:!0}))`
const navbarFocusAfter = `setTimeout((selected=>()=>{(!selected||this.isConnected&&this.matches(":popover-open")&&this.getActiveItem()===null)&&this.focus({preventScroll:!0})})(this.closest("[data-appkit-navbar-repair]")!==null))`

//go:embed navbar_runtime.go
var navbarRuntimeRule string

//go:embed navbar_runtime.cue
var navbarRuntimeSchema string

// NavbarRuntimeArtifact is an explicitly selected private derivative, never a
// replacement for the original runtime or a certification of the paired navbar.
type NavbarRuntimeArtifact struct {
	SchemaVersion         int    `json:"schema_version"`
	Adapter               string `json:"adapter"`
	RepairSHA256          string `json:"repair_sha256"`
	OriginalRuntimeSHA256 string `json:"original_runtime_sha256"`
	RuntimeSHA256         string `json:"runtime_sha256"`
	RuleSHA256            string `json:"rule_sha256"`
	Runtime               string `json:"runtime"`
	WrapperAttribute      string `json:"wrapper_attribute"`
	Verification          string `json:"verification"`
}

func navbarRuntimeDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func patchNavbarMenuFocus(runtime string) (string, error) {
	if strings.Count(runtime, navbarFocusBefore) != 1 {
		return "", fmt.Errorf("navbar runtime: expected one deferred menu-focus site")
	}
	return strings.Replace(runtime, navbarFocusBefore, navbarFocusAfter, 1), nil
}

// AdaptNavbarRuntime verifies the exact original and paired repair before applying
// the source-specific guard. It performs no I/O and never installs a dependency.
func AdaptNavbarRuntime(repair NavbarRepairArtifact, source, runtime []byte) (NavbarRuntimeArtifact, error) {
	if len(runtime) > 256<<10 || navbarRuntimeDigest(runtime) != navbarOriginalRuntimeSHA256 {
		return NavbarRuntimeArtifact{}, fmt.Errorf("navbar runtime: original runtime identity differs")
	}
	if err := VerifyNavbarRepairReplay(repair, source); err != nil {
		return NavbarRuntimeArtifact{}, err
	}
	patched, err := patchNavbarMenuFocus(string(runtime))
	if err != nil {
		return NavbarRuntimeArtifact{}, err
	}
	paired, err := json.Marshal(repair)
	if err != nil {
		return NavbarRuntimeArtifact{}, fmt.Errorf("navbar runtime: repair encoding failed")
	}
	a := NavbarRuntimeArtifact{
		SchemaVersion: 1, Adapter: "p09-navbar-runtime-focus-v1",
		RepairSHA256:          navbarRuntimeDigest(append(paired, '\n')),
		OriginalRuntimeSHA256: navbarOriginalRuntimeSHA256,
		RuntimeSHA256:         navbarRuntimeDigest([]byte(patched)),
		RuleSHA256:            navbarRuntimeDigest([]byte(navbarRuntimeRule + "\x00" + navbarRuntimeSchema)),
		Runtime:               patched, WrapperAttribute: "data-appkit-navbar-repair", Verification: "not-verified",
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		return NavbarRuntimeArtifact{}, fmt.Errorf("navbar runtime: envelope encoding failed")
	}
	ctx := cuecontext.New()
	contract := ctx.CompileString(navbarRuntimeSchema).LookupPath(cue.ParsePath("#NavbarRuntimeArtifact"))
	if err := contract.Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return NavbarRuntimeArtifact{}, fmt.Errorf("navbar runtime: generated envelope failed schema")
	}
	return a, nil
}

// VerifyNavbarRuntimeReplay replays both the unchanged companion and runtime guard;
// self-rehashed edits cannot pass simply by supplying their own new digest.
func VerifyNavbarRuntimeReplay(a NavbarRuntimeArtifact, repair NavbarRepairArtifact, source, runtime []byte) error {
	if len(a.Runtime) > 256<<10 {
		return fmt.Errorf("navbar runtime: artifact exceeds replay bounds")
	}
	want, err := AdaptNavbarRuntime(repair, source, runtime)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(a, want) {
		return fmt.Errorf("navbar runtime: replay mismatch")
	}
	return nil
}
