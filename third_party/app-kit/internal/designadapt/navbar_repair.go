package designadapt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// These inputs remain independent from the immutable five-file P09 base.
// Each app-owned companion has a separately replayed identity.
//
//go:embed navbar_repair.go
var navbarRepairRule string

//go:embed navbar_repair.cue
var navbarRepairSchema string

//go:embed navbar_repair.css
var navbarRepairCSS string

//go:embed navbar_routes.go.txt
var navbarRepairRoutesGo string

//go:embed navbar_routes.templ.txt
var navbarRepairRoutesTempl string

// NavbarRepairArtifact binds the unchanged private P09 base to an explicit
// app-owned route directory and scoped stylesheet. It is not certification.
type NavbarRepairArtifact struct {
	SchemaVersion     int            `json:"schema_version"`
	Adapter           string         `json:"adapter"`
	Base              NavbarArtifact `json:"base"`
	RuleSHA256        string         `json:"rule_sha256"`
	CSSSHA256         string         `json:"css_sha256"`
	CSS               string         `json:"css"`
	RoutesGoSHA256    string         `json:"routes_go_sha256"`
	RoutesGo          string         `json:"routes_go"`
	RoutesTemplSHA256 string         `json:"routes_templ_sha256"`
	RoutesTempl       string         `json:"routes_templ"`
	WrapperAttribute  string         `json:"wrapper_attribute"`
	Verification      string         `json:"verification"`
}

// AdaptNavbarRepair admits only the frozen P09 source, rebuilds its exact base,
// and binds the reviewed app-owned companions without changing that base.
func AdaptNavbarRepair(source designconvert.Source, body []byte) (NavbarRepairArtifact, error) {
	base, err := AdaptNavbar(source, body)
	if err != nil {
		return NavbarRepairArtifact{}, err
	}
	return newNavbarRepairArtifact(base)
}

func newNavbarRepairArtifact(base NavbarArtifact) (NavbarRepairArtifact, error) {
	digest := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	artifact := NavbarRepairArtifact{
		SchemaVersion:     1,
		Adapter:           "p09-navbar-repair-v1",
		Base:              base,
		RuleSHA256:        digest(navbarRepairRule + "\x00" + navbarRepairSchema),
		CSSSHA256:         digest(navbarRepairCSS),
		CSS:               navbarRepairCSS,
		RoutesGoSHA256:    digest(navbarRepairRoutesGo),
		RoutesGo:          navbarRepairRoutesGo,
		RoutesTemplSHA256: digest(navbarRepairRoutesTempl),
		RoutesTempl:       navbarRepairRoutesTempl,
		WrapperAttribute:  "data-appkit-navbar-repair",
		Verification:      "not-verified",
	}
	baseSchema, err := navbarImplementation.ReadFile("navbar.cue")
	if err != nil {
		return NavbarRepairArtifact{}, fmt.Errorf("navbar repair: base schema unavailable")
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return NavbarRepairArtifact{}, fmt.Errorf("navbar repair: envelope encoding failed")
	}
	ctx := cuecontext.New()
	contract := ctx.CompileString(string(baseSchema) + "\n" + navbarRepairSchema).LookupPath(cue.ParsePath("#NavbarRepairArtifact"))
	if err := contract.Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return NavbarRepairArtifact{}, fmt.Errorf("navbar repair: generated envelope failed schema")
	}
	return artifact, nil
}

// VerifyNavbarRepairReplay checks the frozen source, unchanged nested base and
// all companion bytes against current embedded inputs.
func VerifyNavbarRepairReplay(artifact NavbarRepairArtifact, body []byte) error {
	return verifyNavbarRepairReplayWith(artifact, body, AdaptNavbar)
}

func verifyNavbarRepairReplayWith(artifact NavbarRepairArtifact, body []byte, generate func(designconvert.Source, []byte) (NavbarArtifact, error)) error {
	if len(artifact.CSS) > 16<<10 || len(artifact.RoutesGo) > 32<<10 || len(artifact.RoutesTempl) > 64<<10 {
		return fmt.Errorf("navbar repair: artifact exceeds replay bounds")
	}
	if err := verifyNavbarReplayWith(artifact.Base, body, generate); err != nil {
		return err
	}
	base, err := generate(artifact.Base.Source, body)
	if err != nil {
		return err
	}
	want, err := newNavbarRepairArtifact(base)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(artifact, want) {
		return fmt.Errorf("navbar repair: replay mismatch")
	}
	return nil
}
