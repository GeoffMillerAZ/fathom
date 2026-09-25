package designadapt

import (
	"encoding/json"
	"errors"
)

var ErrNavigationDeliveryDrift = errors.New("navigation delivery differs from current replay")

func WriteNavigationDelivery(dir string, artifact NavigationArtifact, source []byte) error {
	if err := VerifyNavigationReplay(artifact, source); err != nil {
		return err
	}
	files, err := navigationDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "navigation delivery")
}

func CheckNavigationDelivery(dir string, artifact NavigationArtifact, source []byte) error {
	if err := VerifyNavigationReplay(artifact, source); err != nil {
		return err
	}
	files, err := navigationDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "navigation delivery", ErrNavigationDeliveryDrift)
}

const navigationDeliveryReadme = `# Private P06 page navigation

Licensed derivative: retain source obligations and keep input/output private.
This is a candidate, not a target certificate, router, account backend or ARIA tabs.
Copy the immutable export to an app-owned private build directory. Compile
navigation.templ with pinned github.com/a-h/templ/cmd/templ v0.3.1020. Runtime
dependency is github.com/a-h/templ v0.3.1020, not app-kit. Package name: converted.
The app owns Go/tools/go.sum, fonts and Tailwind 4 scanning BOTH navigation.templ
and navigation.go. No installation, compiler, CSS generation or server runs here.

Decode defaults.json into PageNavigation for NewPageNavigationReference: this preserves
the source's unbound narrow select and '#' placeholders, NOT usable navigation.
NewPageNavigationGET(data, NavigationGET{Action, FieldName, SubmitLabel}) adds native
GET selection + explicit submit; NewPageNavigationLinks(data) uses ordinary wrapping
links for static pages. Both add explicit focus/wrapping/44px-target styles; they are
adaptations, not exact source fidelity. Reference does not receive these classes.
Use native page links, not role=tab or tabpanel keyboard expectations.

Operational constructors require 2–12 items with distinct Key and URL, a CurrentKey
matching one item, and nonblank labels. Only canonical local ASCII path destinations
are accepted: no query, fragment, encoded path, scheme, host, dot segments or backslash.
Reference alone permits repeated '#'. Items are snapshotted; text is escaped.
Handle constructor errors before writing a response. Long native select options may
truncate on some hosts; prefer Links when full long labels must remain visible.

The app owns authorization, route/current identity and GET request validation. Map
exactly one submitted stable key to an allowlisted route; never redirect using an
arbitrary submitted URL. Preserve application context explicitly in paths; this
adapter does not synthesize hidden fields or drop existing URL query state.

appkit data assets-adapt-navigation -check DIRECTORY compares all five immutable
files and modes against fresh frozen-source replay. artifact.json is written last;
this is not crash-atomic publication. Never build/edit in place. Incomplete output
is retained for inspection; choose a new directory after a failed export.
Delivery matching does not prove browser quality, app adoption or distribution rights.
`

func navigationDeliveryFiles(artifact NavigationArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"navigation.templ", []byte(artifact.Templ)}, {"navigation.go", []byte(artifact.Model)},
		{"defaults.json", artifact.Defaults}, {"README.md", []byte(navigationDeliveryReadme)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
