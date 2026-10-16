package adoption

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

// DraftOptions contains only explicit author choices. Artifacts names existing
// root-relative files; Module and Agents declare requirements, not observations.
type DraftOptions struct {
	Consumer, Profile, Contract string
	ExpectedCatalogRevision     string
	Layers, Capabilities        []string
	Artifacts                   []string
	Module                      *ModuleRequirement
	Agents                      bool
}

// Draft prepares a version-2 selection without reading an existing manifest or
// writing any file. It does not check the contract, module or router files: the
// caller reviews the draft, saves it deliberately, then runs Check. Current pins
// and observed artifact hashes are not upgrade approval or runtime proof.
func Draft(ctx context.Context, root string, opts DraftOptions) (Selection, error) {
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	if root == "" {
		return Selection{}, errors.New("adoption draft: an explicit nonempty root is required")
	}
	// Validate all authored fields through the existing schema before any file
	// reads. Placeholder hashes are internal only and can never be returned.
	if len(opts.Layers) > 6 || len(opts.Capabilities) > 64 || len(opts.Artifacts) > 64 {
		return Selection{}, errors.New("adoption draft: selection exceeds item limits")
	}
	// JSON replaces invalid UTF-8; refuse rather than silently selecting another
	// spelling or path. Decode handles the remaining schema and path constraints.
	values := []string{opts.Consumer, opts.Profile, opts.Contract, opts.ExpectedCatalogRevision}
	values = append(values, opts.Layers...)
	values = append(values, opts.Capabilities...)
	values = append(values, opts.Artifacts...)
	if opts.Module != nil {
		values = append(values, opts.Module.File, opts.Module.Version)
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return Selection{}, errors.New("adoption draft: fields must be valid UTF-8")
		}
	}
	pins := make([]CapabilityRevision, 0, len(opts.Capabilities))
	s := Selection{SchemaVersion: 2, Consumer: opts.Consumer, Profile: opts.Profile,
		Contract: opts.Contract, Layers: append([]string{}, opts.Layers...),
		Capabilities: append([]string{}, opts.Capabilities...), CapabilityRevisions: &pins,
		Module: opts.Module, Agents: opts.Agents, Artifacts: []Artifact{}}
	placeholder := strings.Repeat("0", 64)
	for _, id := range opts.Capabilities {
		pins = append(pins, CapabilityRevision{ID: id, SHA256: placeholder})
	}
	for _, path := range opts.Artifacts {
		s.Artifacts = append(s.Artifacts, Artifact{Path: path, SHA256: placeholder})
	}
	body, err := json.Marshal(s)
	if err != nil {
		return Selection{}, err
	}
	s, err = Decode(body)
	if err != nil {
		return Selection{}, err
	}
	if s.Agents {
		if _, err := Router(s.Contract); err != nil {
			return Selection{}, err
		}
	}
	c, err := catalog.Default()
	if err != nil {
		return Selection{}, fmt.Errorf("adoption draft: catalog: %w", err)
	}
	if opts.ExpectedCatalogRevision != "" && opts.ExpectedCatalogRevision != c.Revision {
		return Selection{}, errors.New("adoption draft: catalog revision changed; review discovery with this checker before drafting")
	}
	fit := newReport("draft")
	fit.checkSelectionFit(c, &s)
	for _, finding := range fit.Checks {
		if finding.Status != "pass" {
			return Selection{}, fmt.Errorf("adoption draft: %s: %s", finding.ID, finding.Detail)
		}
	}
	for i, id := range s.Capabilities {
		entry, _ := c.Get(id) // checkSelectionFit already refused absent entries.
		pin, err := PinCapability(c.SchemaVersion, entry)
		if err != nil {
			return Selection{}, err
		}
		(*s.CapabilityRevisions)[i] = pin
	}
	// Reuse the checker reader and one total read budget. Even a metadata-only
	// draft requires an accessible explicit root, but it reads no ambient files.
	r, rd, _ := open(ctx, root, "draft")
	if rd == nil {
		return Selection{}, errors.New("adoption draft: " + r.Checks[0].Detail)
	}
	defer rd.root.Close()
	for i, artifact := range s.Artifacts {
		body, err := rd.read(artifact.Path, MaxFileBytes)
		if err != nil {
			return Selection{}, fmt.Errorf("adoption draft: artifact %q: %w", artifact.Path, err)
		}
		s.Artifacts[i].SHA256 = hash(body)
	}
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	return s, nil
}
