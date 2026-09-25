// Package designreceipts checks revision-bound evidence integrity, not target
// compatibility. Report claims and reviewer names are not authenticated proof.
// It neither executes a report nor reads paths, fetches assets or certifies UI.
package designreceipts

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// MaxDocumentBytes bounds metadata; asset bytes are bounded separately.
const MaxDocumentBytes = 256 << 10
const MaxContentBytes = 64 << 20

// MaxGateEvidence bounds both authored gate slots and retained result evidence.
const MaxGateEvidence = 64

//go:embed schema.cue
var schema []byte

type Asset struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type Subject struct {
	ID             string `json:"id"`
	Profile        string `json:"profile"`
	Variant        string `json:"variant"`
	SourceArchive  string `json:"source_archive"`
	SourceRevision string `json:"source_revision"`
	ArtifactSHA256 string `json:"artifact_sha256"`
}

type Inputs struct {
	Source      []Asset `json:"source"`
	Transformer []Asset `json:"transformer"`
	Config      []Asset `json:"config"`
	Patch       []Asset `json:"patch"`
	Runtime     []Asset `json:"runtime"`
	Output      []Asset `json:"output"`
	Fixture     []Asset `json:"fixture"`
	Toolchain   []Asset `json:"toolchain"`
	Baseline    []Asset `json:"baseline"`
}

func (i Inputs) groups() [][]Asset {
	return [][]Asset{i.Source, i.Transformer, i.Config, i.Patch, i.Runtime, i.Output, i.Fixture, i.Toolchain, i.Baseline}
}

type Slot struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type Gate struct {
	Applicable      bool   `json:"applicable"`
	Scope           string `json:"scope"`
	Reason          string `json:"reason"`
	ReviewAssertion string `json:"review_assertion"`
	Slots           []Slot `json:"slots"`
}

type Contract struct {
	SchemaVersion int             `json:"schema_version"`
	Subject       Subject         `json:"subject"`
	Mode          string          `json:"mode"`
	Inputs        Inputs          `json:"inputs"`
	Gates         map[string]Gate `json:"gates"`
}

type Evidence struct {
	Asset
	Role string `json:"role"`
}

type Result struct {
	Status   string     `json:"status"`
	Reason   string     `json:"reason"`
	Evidence []Evidence `json:"evidence"`
}

type Receipt struct {
	SchemaVersion  int               `json:"schema_version"`
	ContractSHA256 string            `json:"contract_sha256"`
	SourceRevision string            `json:"source_revision"`
	ArtifactSHA256 string            `json:"artifact_sha256"`
	Results        map[string]Result `json:"results"`
}

// Expected is supplied by a trusted caller independently of the receipt.
// Computing it from the receipt would only check self-consistency, not freshness.
type Expected struct {
	ContractSHA256 string
	SourceRevision string
	ArtifactSHA256 string
}

type Report struct {
	Integrity        string            `json:"integrity"`
	Verification     string            `json:"verification"`
	ContractSHA256   string            `json:"contract_sha256"`
	ReceiptSHA256    string            `json:"receipt_sha256"`
	Subject          Subject           `json:"subject"`
	Mode             string            `json:"mode"`
	ClaimedStatuses  map[string]string `json:"claimed_statuses"`
	IncompleteGates  []string          `json:"incomplete_gates"`
	MatchedArtifacts int               `json:"matched_artifacts"`
}

// Digest hashes exact bytes, not a path, parsed document or command string.
func Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// EncodeContract creates the canonical representation required by Check.
func EncodeContract(c Contract) ([]byte, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var checked Contract
	if err := decode(body, &checked, "#Contract"); err != nil {
		return nil, err
	}
	return body, nil
}

func EncodeReceipt(r Receipt) ([]byte, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var checked Receipt
	if err := decode(body, &checked, "#Receipt"); err != nil {
		return nil, err
	}
	return body, nil
}

// Check requires an exact content inventory keyed by opaque ASCII asset IDs.
// Callers own safe private loading and review of the trusted contract. No path
// is interpreted. Even an all-passed receipt remains explicitly not-verified.
// Failed/partial evidence is valid history, not an integrity error or a pass.
func Check(contractBody, receiptBody []byte, expected Expected, content map[string][]byte) (Report, error) {
	fail := func(msg string) (Report, error) { return Report{}, fmt.Errorf("designreceipts: %s", msg) }
	if Digest(contractBody) != expected.ContractSHA256 {
		return fail("contract differs from expected revision")
	}
	var c Contract
	if err := decode(contractBody, &c, "#Contract"); err != nil {
		return Report{}, err
	}
	var r Receipt
	if err := decode(receiptBody, &r, "#Receipt"); err != nil {
		return Report{}, err
	}
	if r.ContractSHA256 != expected.ContractSHA256 || r.SourceRevision != expected.SourceRevision ||
		r.ArtifactSHA256 != expected.ArtifactSHA256 || c.Subject.SourceRevision != expected.SourceRevision ||
		c.Subject.ArtifactSHA256 != expected.ArtifactSHA256 {
		return fail("stale source, artifact or contract binding")
	}
	if !containsHash(c.Inputs.Source, expected.SourceRevision) || !containsHash(c.Inputs.Output, expected.ArtifactSHA256) {
		return fail("subject source/output missing from role inventory")
	}
	assets := map[string]Asset{}
	add := func(a Asset) error {
		if old, ok := assets[a.ID]; ok && old != a {
			return fmt.Errorf("designreceipts: conflicting artifact identity %s", a.ID)
		}
		assets[a.ID] = a
		return nil
	}
	for _, group := range c.Inputs.groups() {
		seen := map[string]bool{}
		for _, a := range group {
			if seen[a.ID] {
				return fail("duplicate input in role inventory")
			}
			seen[a.ID] = true
			if err := add(a); err != nil {
				return Report{}, err
			}
		}
	}
	inputIDs := map[string]bool{}
	for id := range assets {
		inputIDs[id] = true
	}
	declaredRoles := map[string]string{}
	report := Report{Integrity: "matched", Verification: "not-verified", ContractSHA256: expected.ContractSHA256,
		ReceiptSHA256: Digest(receiptBody), Subject: c.Subject, Mode: c.Mode, ClaimedStatuses: map[string]string{}, IncompleteGates: []string{}}
	// Sorted traversal also keeps errors and output reproducible across runs.
	keys := make([]string, 0, len(c.Gates))
	for key := range c.Gates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		gate, result := c.Gates[key], r.Results[key]
		if gate.Applicable == (result.Status == "not-applicable") {
			return fail("gate applicability differs: " + key)
		}
		if !gate.Applicable && result.Reason != gate.Reason {
			return fail("exclusion reason differs: " + key)
		}
		slots := map[string]string{}
		for _, s := range gate.Slots {
			if _, ok := slots[s.ID]; ok {
				return fail("duplicate evidence slot: " + key)
			}
			if inputIDs[s.ID] {
				return fail("evidence slot shadows an input: " + key)
			}
			if role, ok := declaredRoles[s.ID]; ok && role != s.Role {
				return fail("shared evidence slot changes role: " + key)
			}
			declaredRoles[s.ID] = s.Role
			slots[s.ID] = s.Role
		}
		seen := map[string]bool{}
		for _, e := range result.Evidence {
			if slots[e.ID] != e.Role || seen[e.ID] {
				return fail("undeclared, wrong-role or duplicate evidence: " + key)
			}
			seen[e.ID] = true
			if err := add(e.Asset); err != nil {
				return Report{}, err
			}
		}
		if result.Status == "passed" && len(seen) != len(slots) {
			return fail("claimed pass lacks evidence: " + key)
		}
		report.ClaimedStatuses[key] = result.Status
		if gate.Applicable && result.Status != "passed" {
			report.IncompleteGates = append(report.IncompleteGates, key)
		}
	}
	if len(content) != len(assets) {
		return fail("content inventory differs from declared artifacts")
	}
	total := 0
	assetKeys := make([]string, 0, len(assets))
	for key := range assets {
		assetKeys = append(assetKeys, key)
	}
	sort.Strings(assetKeys)
	for _, key := range assetKeys {
		a := assets[key]
		body, ok := content[key]
		if !ok || len(body) != a.Bytes {
			return fail("missing or wrong-sized artifact: " + key)
		}
		if len(body) > MaxContentBytes-total {
			return fail("content exceeds byte budget")
		}
		total += len(body)
		if Digest(body) != a.SHA256 {
			return fail("artifact hash mismatch: " + key)
		}
	}
	report.MatchedArtifacts = len(assets)
	return report, nil
}

func containsHash(assets []Asset, hash string) bool {
	for _, a := range assets {
		if a.SHA256 == hash {
			return true
		}
	}
	return false
}

// Canonical re-encoding rejects duplicate/unknown/case-aliased keys, trailing
// JSON and lossy Unicode (including unpaired escaped UTF-16 surrogates) before
// CUE admission. Map ordering is stable through the Encode helpers.
func decode(body []byte, target any, definition string) error {
	if len(body) > MaxDocumentBytes || !utf8.Valid(body) || !json.Valid(body) {
		return fmt.Errorf("designreceipts: invalid JSON encoding or metadata size")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("designreceipts: invalid typed JSON: %w", err)
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(body, canonical) {
		return fmt.Errorf("designreceipts: noncanonical JSON")
	}
	// Bound collection amplification before invoking the schema evaluator.
	switch v := target.(type) {
	case *Contract:
		if len(v.Gates) != 9 {
			return fmt.Errorf("designreceipts: expected nine gates")
		}
		for _, group := range v.Inputs.groups() {
			if len(group) > 64 {
				return fmt.Errorf("designreceipts: input count exceeds limit")
			}
		}
		for _, gate := range v.Gates {
			if len(gate.Slots) > MaxGateEvidence {
				return fmt.Errorf("designreceipts: slot count exceeds limit")
			}
		}
	case *Receipt:
		if len(v.Results) != 9 {
			return fmt.Errorf("designreceipts: expected nine results")
		}
		for _, result := range v.Results {
			if len(result.Evidence) > MaxGateEvidence {
				return fmt.Errorf("designreceipts: evidence count exceeds limit")
			}
		}
	}
	ctx := cuecontext.New()
	value := ctx.CompileBytes(body).Unify(ctx.CompileBytes(schema).LookupPath(cue.ParsePath(definition)))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("designreceipts: %s schema rejected: %w", definition, err)
	}
	return nil
}
