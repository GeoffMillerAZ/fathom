// The runner copies this program into a separate temporary Go module.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	catalogkit "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	specPath := flag.String("spec", "", "CUE-exported spec JSON")
	profilePath := flag.String("profile", "", "CUE-exported profile JSON")
	policyPath := flag.String("policy", "", "CUE-exported policy JSON")
	identityPath := flag.String("identity", "", "CUE-exported identity JSON")
	definitionPath := flag.String("definition", "", "current definition bytes")
	evidencePath := flag.String("evidence", "", "captured evidence bytes")
	flag.Parse()
	read := func(path string) ([]byte, error) {
		if path == "" {
			return nil, fmt.Errorf("missing input path")
		}
		return os.ReadFile(path)
	}
	decode := func(path string, fn func([]byte) error) error {
		data, err := read(path)
		if err != nil {
			return err
		}
		return fn(data)
	}
	var spec catalogkit.Spec
	if err := decode(*specPath, func(b []byte) (err error) { spec, err = catalogkit.DecodeSpec(b); return }); err != nil {
		return err
	}
	var profile catalogkit.Profile
	if err := decode(*profilePath, func(b []byte) (err error) { profile, err = catalogkit.DecodeProfile(b); return }); err != nil {
		return err
	}
	var policy catalogkit.Policy
	if err := decode(*policyPath, func(b []byte) (err error) { policy, err = catalogkit.DecodePolicy(b); return }); err != nil {
		return err
	}
	var identity catalogkit.DecisionIdentity
	if err := decode(*identityPath, func(b []byte) (err error) { identity, err = catalogkit.DecodeDecisionIdentity(b); return }); err != nil {
		return err
	}
	bundle, err := catalogkit.ResolveProfile(spec, policy, profile)
	if err != nil {
		return err
	}
	node, err := catalogkit.LookupStableKey(bundle, "review.evidence")
	if err != nil {
		return err
	}
	definition, err := read(*definitionPath)
	if err != nil {
		return err
	}
	evidenceText, err := read(*evidencePath)
	if err != nil {
		return err
	}
	// The binding pins the reviewed definition. A changed current definition
	// must fail before any prepared request or reuse key exists.
	approved, err := os.ReadFile("approved-definition.txt")
	if err != nil {
		return err
	}
	bind := catalogkit.Binding{
		NodeID: node.ID(), NodeDigest: catalogkit.NodeDigest(node),
		DefinitionDigest: hash(approved), Version: "1",
		Question: decision.Question{ID: node.StableKey, Kind: decision.KindChoice,
			Instructions: "Do the release notes cite their supporting evidence?",
			Options:      []decision.Option{{Name: "yes", Criteria: "A specific supporting record is cited"}, {Name: "no", Criteria: "No supporting record is cited"}, {Name: "unknown", Criteria: "Evidence is insufficient to determine this"}}},
		RequiredEvidence: []string{"release-note"},
	}
	evidence := []catalogkit.Evidence{{ID: "release-note", Source: "local-fixture", Revision: "1", Text: string(evidenceText), Digest: hash(evidenceText)}}
	prepared, err := catalogkit.PrepareDecision(bundle, bind, definition, evidence, 0)
	if err != nil {
		return err
	}
	key, err := catalogkit.DecisionCacheKey(prepared, identity)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Profile      string   `json:"profile"`
		Order        []string `json:"order"`
		StableKey    string   `json:"stable_key"`
		NodeID       string   `json:"node_id"`
		PacketDigest string   `json:"packet_digest"`
		ReuseKey     string   `json:"reuse_key"`
	}{profile.Name, bundle.Order, node.StableKey, node.ID(), prepared.Digest, key})
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
