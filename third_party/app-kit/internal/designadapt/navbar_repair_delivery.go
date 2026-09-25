package designadapt

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

//go:embed navbar_repair_delivery.go
var navbarRepairDeliveryRule string

//go:embed navbar_repair_delivery.cue
var navbarRepairDeliverySchema string

// NavbarRepairDeliveryArtifact pairs independently replayed companions without
// changing either one. This private delivery identity is not target certification.
type NavbarRepairDeliveryArtifact struct {
	SchemaVersion int                   `json:"schema_version"`
	Adapter       string                `json:"adapter"`
	Repair        NavbarRepairArtifact  `json:"repair"`
	Runtime       NavbarRuntimeArtifact `json:"runtime"`
	RuleSHA256    string                `json:"rule_sha256"`
	Verification  string                `json:"verification"`
}

var ErrNavbarRepairDeliveryDrift = errors.New("navbar repair delivery differs from current replay")

// ReadNavbarRuntimeOriginal reads only an explicitly supplied, exact pinned ESM.
// Original inputs need not have delivery modes, but cannot be links or special
// files. No-follow/nonblocking opens and bounded reads preserve the CLI boundary.
func ReadNavbarRuntimeOriginal(filename string) ([]byte, error) {
	if !filepath.IsAbs(filename) {
		return nil, fmt.Errorf("navbar runtime: absolute original ESM path required")
	}
	flags, err := privateDeliveryReadFlags("navbar runtime")
	if err != nil {
		return nil, err
	}
	filename = filepath.Clean(filename)
	root, err := openPrivateDirectory(filepath.Dir(filename))
	if err != nil {
		return nil, fmt.Errorf("navbar runtime: real source parent unavailable")
	}
	defer root.Close()
	name := filepath.Base(filename)
	before, err := root.Lstat(name)
	if err != nil || !privateDeliveryRegular(before) || before.Size() <= 0 || before.Size() > 256<<10 {
		return nil, fmt.Errorf("navbar runtime: bounded single-link regular ESM required")
	}
	f, err := root.OpenFile(name, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("navbar runtime: source open refused")
	}
	actual, err := f.Stat()
	if err != nil || !privateDeliveryRegular(actual) || !os.SameFile(before, actual) || actual.Size() != before.Size() {
		f.Close()
		return nil, fmt.Errorf("navbar runtime: source changed during open")
	}
	body, readErr := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	closeErr := f.Close()
	if errors.Join(readErr, closeErr) != nil || int64(len(body)) != before.Size() || navbarRuntimeDigest(body) != navbarOriginalRuntimeSHA256 {
		return nil, fmt.Errorf("navbar runtime: exact original ESM identity required")
	}
	return body, nil
}

func AdaptNavbarRepairDelivery(source designconvert.Source, body, runtime []byte) (NavbarRepairDeliveryArtifact, error) {
	repair, err := AdaptNavbarRepair(source, body)
	if err != nil {
		return NavbarRepairDeliveryArtifact{}, err
	}
	guard, err := AdaptNavbarRuntime(repair, body, runtime)
	if err != nil {
		return NavbarRepairDeliveryArtifact{}, err
	}
	a := NavbarRepairDeliveryArtifact{
		SchemaVersion: 1, Adapter: "p09-navbar-repair-delivery-v1", Repair: repair, Runtime: guard,
		RuleSHA256:   navbarRuntimeDigest([]byte(navbarRepairDeliveryRule + "\x00" + navbarRepairDeliverySchema)),
		Verification: "not-verified",
	}
	baseSchema, err := navbarImplementation.ReadFile("navbar.cue")
	if err != nil {
		return NavbarRepairDeliveryArtifact{}, fmt.Errorf("navbar repair delivery: base schema unavailable")
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		return NavbarRepairDeliveryArtifact{}, err
	}
	ctx := cuecontext.New()
	schema := string(baseSchema) + "\n" + navbarRepairSchema + "\n" + strings.TrimPrefix(navbarRuntimeSchema, "package designadapt\n") + "\n" + navbarRepairDeliverySchema
	contract := ctx.CompileString(schema).LookupPath(cue.ParsePath("#NavbarRepairDeliveryArtifact"))
	if err := contract.Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return NavbarRepairDeliveryArtifact{}, fmt.Errorf("navbar repair delivery: generated envelope failed schema")
	}
	return a, nil
}

func VerifyNavbarRepairDeliveryReplay(a NavbarRepairDeliveryArtifact, body, runtime []byte) error {
	if err := VerifyNavbarRuntimeReplay(a.Runtime, a.Repair, body, runtime); err != nil {
		return err
	}
	want, err := AdaptNavbarRepairDelivery(a.Repair.Base.Source, body, runtime)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(a, want) {
		return fmt.Errorf("navbar repair delivery: replay mismatch")
	}
	return nil
}

// WriteNavbarRepairDelivery verifies both original inputs before output I/O.
func WriteNavbarRepairDelivery(dir string, a NavbarRepairDeliveryArtifact, body, runtime []byte) error {
	if err := VerifyNavbarRepairDeliveryReplay(a, body, runtime); err != nil {
		return err
	}
	files, err := navbarRepairDeliveryFiles(a)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "navbar repair delivery")
}

func CheckNavbarRepairDelivery(dir string, a NavbarRepairDeliveryArtifact, body, runtime []byte) error {
	if err := VerifyNavbarRepairDeliveryReplay(a, body, runtime); err != nil {
		return err
	}
	files, err := navbarRepairDeliveryFiles(a)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "navbar repair delivery", ErrNavbarRepairDeliveryDrift)
}

const navbarRepairDeliveryUsage = `# Private P09 navbar with explicit route access and runtime repair

Licensed derivative: keep all delivered and compiled files private and retain source
and runtime license obligations. Matching delivery is not target certification.

Copy this immutable directory into a separate private app-owned build directory.
Compile navbar.templ and navbar_routes.templ with pinned templ v0.3.1020. Generated Go
uses only templ and the standard library; no sibling app-kit checkout is required.
Build source Tailwind4 CSS scanning both templates. Load that CSS first and the
delivered navbar_repair.css last. The app supplies fonts, approved image assets,
HTTP routing, auth and its own data. No tools, dependencies or backend are installed.

Construct NewNavbarWithRoutes(Navbar, summary) after binding defaults.json with real
app-owned values. It snapshots the same validated Navbar as NewNavbar and adds an
always-available native details/summary directory inside data-appkit-navbar-repair.
Choose a clear summary/landmark name, at most256 UTF-8 bytes. CurrentKey semantics,
safe GET destinations and optional account/notification fields remain app-owned.
Handle errors before writing response headers. Do not invent a sign-out GET mutation.

Explicitly serve elements-navbar.js as a module, preserving its exact bytes:

    <script type="module" src="/assets/elements-navbar.js"></script>

Load this derivative ONCE in the document, not alongside the original Elements module.
It retains Elements1.0.22 interaction ownership, guarding only deferred container focus
for selected navbar wrappers. There is no extra mount/dispose listener or app-kit
bootstrap. Other Elements interactions and unselected menus retain their original
behavior. Keep each subtree under one owner; do not morph it from a second renderer.

The native directory stays available during healthy, failed and late module loading.
It never auto-hides on runtime registration. Review its placement/wording and density
in the actual application. Account and notification destinations need real safe routes;
no account backend, permissions, persistence, mutation or notification state is supplied.
NavbarReference remains the unchanged inert private source specimen; do not expose
placeholder actions or external demonstration images as functioning app navigation.

Nine fixed files must match fresh assets-adapt-navbar -repair -runtime ORIGINAL_ESM
-check DIRECTORY replay: navbar.templ, navbar.go, defaults.json, navbar_repair.css,
navbar_routes.go, navbar_routes.templ, elements-navbar.js, USAGE.md and artifact.json.
The runtime input must be the exact original pinned ESM at an absolute path, not this
derivative. Replay independently verifies original source, base, repair and runtime.
Default five-file delivery and repair delivery cannot cross-check or silently upgrade.
Existing paths are never overwritten; artifact.json is written last and incomplete
output stays for inspection. Browser/zoom/lifecycle, visual/accessibility/performance
and actual consumer adoption remain separate evidence gates.
`

func navbarRepairDeliveryFiles(a NavbarRepairDeliveryArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"navbar.templ", []byte(a.Repair.Base.Templ)}, {"navbar.go", []byte(a.Repair.Base.Model)},
		{"defaults.json", a.Repair.Base.Defaults}, {"navbar_repair.css", []byte(a.Repair.CSS)},
		{"navbar_routes.go", []byte(a.Repair.RoutesGo)}, {"navbar_routes.templ", []byte(a.Repair.RoutesTempl)},
		{"elements-navbar.js", []byte(a.Runtime.Runtime)}, {"USAGE.md", []byte(navbarRepairDeliveryUsage)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
