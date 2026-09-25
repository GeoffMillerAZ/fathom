package designadapt

import (
	"encoding/json"
	"errors"
)

// ErrDirectoryDeliveryDrift means an inspected P04 delivery differs from current
// replay. Source access/replay and operational failures are not delivery drift.
var ErrDirectoryDeliveryDrift = errors.New("directory delivery differs from current replay")

// WriteDirectoryDelivery exports the frozen source-styled P04 to an absent private
// directory. Callers must independently verify archive access first. It does not
// build, install dependencies or certify a target; incomplete writes are retained.
func WriteDirectoryDelivery(dir string, artifact DirectoryArtifact, source []byte) error {
	if err := VerifyDirectoryReplay(artifact, source); err != nil {
		return err
	}
	files, err := directoryDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "directory delivery")
}

// CheckDirectoryDelivery checks all five source-styled files against fresh replay.
// It never trusts the delivery's own hashes or modifies its contents.
func CheckDirectoryDelivery(dir string, artifact DirectoryArtifact, source []byte) error {
	if err := VerifyDirectoryReplay(artifact, source); err != nil {
		return err
	}
	files, err := directoryDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "directory delivery", ErrDirectoryDeliveryDrift)
}

// WriteDirectoryEnhancedDelivery exports both the unchanged base and the explicit
// P04 usability companion. It has the same no-clobber boundary as the base export.
func WriteDirectoryEnhancedDelivery(dir string, artifact DirectoryEnhancedArtifact, source []byte) error {
	if err := VerifyDirectoryEnhancedReplay(artifact, source); err != nil {
		return err
	}
	files, err := directoryEnhancedDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return writePrivateDelivery(dir, files, "directory delivery")
}

// CheckDirectoryEnhancedDelivery checks the exact eight-file enhanced inventory;
// source-styled exports cannot silently pass as enhanced deliveries, or vice versa.
func CheckDirectoryEnhancedDelivery(dir string, artifact DirectoryEnhancedArtifact, source []byte) error {
	if err := VerifyDirectoryEnhancedReplay(artifact, source); err != nil {
		return err
	}
	files, err := directoryEnhancedDeliveryFiles(artifact)
	if err != nil {
		return err
	}
	return checkPrivateDelivery(dir, files, "directory delivery", ErrDirectoryDeliveryDrift)
}

const directoryDeliveryReadme = `# Private P04 responsive directory/table

Licensed derivative: keep this directory and generated outputs private, retaining
the source license obligations. This is not a certified target or account backend.

This is immutable input, not a build directory. Copy to an app-owned private package
before compiling directory.templ with github.com/a-h/templ/cmd/templ v0.3.1020 using
a pinned go.mod tool directive. Package name is converted. Its only runtime module
dependency is github.com/a-h/templ v0.3.1020, not app-kit. The app owns Go/tool pins,
go.sum, checksums, Tailwind 4 CSS scanning the templates, and deliberate font assets.
Neither CSS generation nor fonts/tools/dependency installation is included here.

Decode defaults.json into Directory; supply real AddURL and every row's EditURL.
The deliberately empty default URLs fail validation. NewDirectoryNavigation(data)
validates labels, 0..100 rows and local absolute paths or explicit HTTPS destinations
before rendering; handle its error before writing an HTTP response. All data is plain
text and escaped; repeated desktop/mobile values share the same fields. Keep auth,
route policy, persistence, empty/partial explanation and data selection in the app.
No sort/filter/page/export engine, JavaScript or mutation handlers are included.

Source-styled NewDirectoryNavigation preserves the purchased structure and styling,
including its recorded small-target limitations. The enhanced variant is a separate
explicit delivery choice, never an implicit repair or token remap.

All five files must match appkit data assets-adapt-directory -check DIRECTORY against
the independently verified original archive. artifact.json is written last, not a
crash-atomic publication or certificate. Checks reject missing/extra/modified/linked
files and mode drift. Never build or edit this directory in place. Interrupted output
is retained: inspect it, then choose a new path. Matching is not proof of compilation,
browser quality, real app adoption or permission to distribute licensed derivatives.
`

const directoryEnhancedDeliveryReadme = `# Private P04 enhanced responsive directory/table

Licensed derivative: keep all input and generated output private and preserve the
source license obligations. This is not a certified target or account backend.

Copy this immutable input to an app-owned private build package named converted.
Compile directory.templ AND directory_enhanced.templ using the pinned go.mod tool
github.com/a-h/templ/cmd/templ v0.3.1020. The only runtime module dependency is
github.com/a-h/templ v0.3.1020; app-kit is not required. The app owns Go/tool pins,
go.sum, checksums, Tailwind 4 scanning both templates, and deliberate font assets.
Load directory_enhanced.css AFTER the source stylesheet. No tool, dependency, font
or CSS build is installed/executed by delivery. Never build in the source store.

Decode defaults.json into Directory and replace empty AddURL and every row EditURL
with app-owned local absolute paths or explicit HTTPS destinations. Call either
NewDirectoryEnhanced(data) for the scoped usability wrapper or the unchanged
NewDirectoryNavigation(data) for the source-styled rendering. Handle validation
errors before writing a response. Both use the same bounded labels, 0..100 rows,
escaped values and copied slices; defaults are examples, not real application data.
Use Introduction to explain empty/partial data; auth, routes and persistence remain
app-owned. No JavaScript, account service or sort/filter/page/export engine is added.

The companion retains the purchased palette and data, adding 44px minimum links,
focus treatment and readable long-content wrapping. The base remains independent;
it retains its recorded small-target limitations. Enhancement is not a token remap.

Check all eight files with appkit data assets-adapt-directory -enhanced -check DIRECTORY
against the independently verified original. Omitting -enhanced checks the distinct
five-file inventory and must fail. artifact.json contains the complete enhanced
envelope and unchanged base; it is written last, not a crash-atomic certificate.
Missing/extra/modified/linked files and mode drift fail; interrupted output remains
for inspection. Choose a new path, never repair or build in place. Matching proves
delivery replay, not downstream compilation, browser quality or actual app adoption.
`

func directoryDeliveryFiles(artifact DirectoryArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"directory.templ", []byte(artifact.Templ)},
		{"directory.go", []byte(artifact.Model)},
		{"defaults.json", artifact.Defaults},
		{"README.md", []byte(directoryDeliveryReadme)},
		{"artifact.json", append(body, '\n')},
	}, nil
}

func directoryEnhancedDeliveryFiles(artifact DirectoryEnhancedArtifact) ([]privateDeliveryFile, error) {
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	return []privateDeliveryFile{
		{"directory.templ", []byte(artifact.Base.Templ)},
		{"directory.go", []byte(artifact.Base.Model)},
		{"directory_enhanced.templ", []byte(artifact.Templ)},
		{"directory_enhanced.go", []byte(artifact.Model)},
		{"directory_enhanced.css", []byte(artifact.CSS)},
		{"defaults.json", artifact.Base.Defaults},
		{"README.md", []byte(directoryEnhancedDeliveryReadme)},
		{"artifact.json", append(body, '\n')},
	}, nil
}
