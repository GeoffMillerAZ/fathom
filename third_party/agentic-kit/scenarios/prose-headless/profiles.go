// Package proseheadless composes the session-governed prose engine with the
// existing Catalog Kit, Assignment Kit and HITL commit/delivery machinery for a
// headless consumer. Prose core imports none of them. All state is caller-owned:
// types here are safe for concurrent use only where stated. Everything is
// synthetic-fixture friendly and contacts no model, network or identity provider;
// host ports (sources, truth, identity, correction, destination) are interfaces
// the consumer implements. This is a composition proof, not production adoption.
package proseheadless

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	catalogkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// ErrProfile identifies an invalid or falsely claimed profile source.
var ErrProfile = errors.New("proseheadless: invalid profile source")

// DefinitionFacet is the catalog node facet that pins a profile definition: the
// node's value is the SHA-256 of the exact definition bytes held by the host.
const DefinitionFacet = "definition_digest"

const maxDefinitionBytes = 1 << 20

func profileErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrProfile, fmt.Sprintf(format, args...))
}

// decodeDefinition strictly decodes one profile definition. A definition is data:
// it carries no origin (the host stamps provenance) and no digest claims.
func decodeDefinition(def []byte) (prose.DocumentTypeProfile, error) {
	var p prose.DocumentTypeProfile
	if len(def) == 0 || len(def) > maxDefinitionBytes || !utf8.Valid(def) {
		return p, profileErr("definition empty, oversized or invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(def))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, profileErr("definition shape: %v", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return p, profileErr("trailing definition content")
	}
	if p.Origin.Kind != "" || p.Origin.CatalogKey != "" || p.Origin.DefinitionDigest != "" || len(p.Origin.LayerEvidence) != 0 || p.Digest != "" {
		return p, profileErr("a definition cannot carry origin or digest claims")
	}
	return p, nil
}

// SuppliedProfile builds a profile from caller-supplied definition JSON with a
// "supplied" origin. It claims nothing about a catalog.
func SuppliedProfile(def []byte) (prose.DocumentTypeProfile, error) {
	p, err := decodeDefinition(def)
	if err != nil {
		return p, err
	}
	p.Origin = prose.ProfileOrigin{Kind: "supplied"}
	return prose.NewDocumentTypeProfile(p)
}

// CatalogProfile resolves a profile through an actual Catalog Kit bundle. The
// bundle is integrity-verified, the stable key must exist, and the node must pin
// the exact definition bytes through DefinitionFacet. Origin evidence records the
// bundle, node and policy digests plus source pins, so a profile cannot claim a
// catalog it was not resolved from. The definition's key must equal the stable key.
func CatalogProfile(b catalogkit.Bundle, stableKey string, def []byte) (prose.DocumentTypeProfile, error) {
	node, err := catalogkit.LookupStableKey(b, stableKey)
	if err != nil {
		return prose.DocumentTypeProfile{}, profileErr("catalog lookup: %v", err)
	}
	want := prose.ContentDigest(string(def))
	if f := node.Facets[DefinitionFacet]; len(f) != 1 || f[0] != want {
		return prose.DocumentTypeProfile{}, profileErr("catalog node does not pin these definition bytes")
	}
	p, err := decodeDefinition(def)
	if err != nil {
		return p, err
	}
	if p.Key != stableKey {
		return p, profileErr("definition key %q differs from stable key %q", p.Key, stableKey)
	}
	ev := []string{"bundle:" + b.Digest, "node:" + catalogkit.NodeDigest(node), "policy:" + b.PolicyDigest}
	for _, s := range b.Sources {
		ev = append(ev, "source:"+s.Name+"="+s.Digest)
	}
	p.Origin = prose.ProfileOrigin{Kind: "catalog", CatalogKey: stableKey, DefinitionDigest: want, LayerEvidence: ev}
	return prose.NewDocumentTypeProfile(p)
}

// ProfileSource is a profile with the evidence needed to re-verify its origin.
type ProfileSource struct {
	Profile    prose.DocumentTypeProfile
	Bundle     *catalogkit.Bundle // required for a catalog origin
	Definition []byte             // exact definition bytes
}

// Verify re-derives the profile from its claimed origin. A supplied origin must
// match a fresh SuppliedProfile; a catalog origin must match a fresh resolution
// from the bundle. A falsely claimed or stale catalog origin fails.
func (s ProfileSource) Verify() error {
	if err := prose.VerifyProfile(s.Profile); err != nil {
		return err
	}
	var fresh prose.DocumentTypeProfile
	var err error
	switch s.Profile.Origin.Kind {
	case "supplied":
		if s.Bundle != nil {
			return profileErr("supplied profile must not carry a catalog bundle")
		}
		fresh, err = SuppliedProfile(s.Definition)
	case "catalog":
		if s.Bundle == nil {
			return profileErr("catalog origin without a bundle is a claim, not a resolution")
		}
		fresh, err = CatalogProfile(*s.Bundle, s.Profile.Origin.CatalogKey, s.Definition)
	default:
		return profileErr("unknown origin")
	}
	if err != nil {
		return err
	}
	if fresh.Digest != s.Profile.Digest {
		return profileErr("profile does not match its claimed origin")
	}
	return nil
}

// NewSession opens a session revision only for a verified profile source.
func NewSession(src ProfileSource, in prose.SessionInput, parent *prose.DocumentSession) (prose.DocumentSession, error) {
	if err := src.Verify(); err != nil {
		return prose.DocumentSession{}, err
	}
	return prose.NewDocumentSession(src.Profile, in, parent)
}
