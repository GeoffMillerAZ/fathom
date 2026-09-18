// Package external is the findings-loop's governed import path (vision
// docs/vision/30-findings-loop.md §3.3, ADR-040 decision 2, plan
// docs/plans/findings-loop.md FL-4..FL-9): it decodes the review kit's
// #FathomProjectionOutbox projection, validates it strictly, and imports
// each event into the claims store as a content-addressed, producer-
// namespaced finding — never touching a native (review-verb/v1) claim, and
// never trusting a producer whose findings already exist as native fathom
// claims (origin "fathom" is refused as circular proof).
//
// A leaf package (ADR-040's hexagonal ownership: fathom is a PROJECTION
// ADAPTER, never the organizational record): it may import internal/core,
// internal/store, internal/semantic (the producer catalog and CapTier), and
// agentic-kit's read-manifest — never internal/analysis, never cmd.
package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// hex64Pattern matches a lowercase sha256 hex digest — the shape every
// findingId/payloadDigest/anchor.digest in the contract carries (the same
// encoding core.ClaimID/core.HashLines produce via hex.EncodeToString).
var hex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func isHex64(s string) bool { return hex64Pattern.MatchString(s) }

// validRelPath reports whether p is a well-formed repo-relative path: no
// leading "/", no ".." or "." path segment.
func validRelPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}

// Producer is one event's producer identity (name, version) — decoded from
// the wire's producer{name,version} object.
type Producer struct {
	Name    string
	Version string
}

// Subject is the org/repo/component triple the contract carries twice per
// event (subject and visibility, required equal by ParseProjection).
type Subject struct {
	Organization string
	Repository   string
	Component    string
}

// String renders the "org/repo/component" form ParseProjection compares
// against the provisioned `[findings] subject`.
func (s Subject) String() string {
	return s.Organization + "/" + s.Repository + "/" + s.Component
}

// Anchor is an event's optional code anchor. A nil *Anchor on Event means
// the finding is a process subject — carried unanchored, served as process
// context, never attached to a file (vision 30 §3.1, §4 item 4).
type Anchor struct {
	Path   string
	Digest string
	Start  int // 0 means "not given"
	End    int // 0 means "not given"
	Symbol string
}

// Rule is an event's rule/remedy statement. Tier is normalized by
// ParseProjection: an absent wire tier decodes to core.TierAssumed's string
// value ("assumed"), per the input contract ("absent means assumed") — so
// every other reader of Event.Rule.Tier sees a concrete tier, never "".
// Class stays "" when absent (validated, never defaulted): "" is counted
// `unclassified` in the import receipt, never silently bucketed.
type Rule struct {
	ID      string
	Version string
	Text    string
	Remedy  string
	Closure string
	Class   string
	Tier    string
}

// Event is one decoded, validated projection event — the Go-typed mirror of
// the review kit's #FathomProjectionOutbox event shape plus the additive
// anchor.start/end/symbol fields (vision 30 §3.3).
type Event struct {
	SchemaVersion  int
	Kind           string // "observation" | "closure" | "annotation" | "revocation"
	FindingID      string // 64-hex
	Producer       Producer
	Subject        Subject
	SourceID       string
	SourceVersion  uint64
	SourceRevision string
	PayloadDigest  string // 64-hex
	ObservedAt     time.Time
	Outcome        string // "pass" | "fail" | "unknown"
	Coverage       string // "complete" | "incomplete"
	Anchor         *Anchor
	SourceStatus   string
	Rule           Rule
	Visibility     Subject
	Annotation     string // set only for kind == "annotation"
}

// Projection is the decoded, validated top-level projection.
type Projection struct {
	SchemaVersion int
	Delivery      string
	Reason        string
	Target        string // "" or "fathom"
	Events        []Event
}

// --- wire (raw JSON) shapes: decoded strictly, then validated into the
// clean Projection/Event shapes above. ---

type wireProducer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type wireSubject struct {
	Organization string `json:"organization"`
	Repository   string `json:"repository"`
	Component    string `json:"component"`
}

type wireAnchor struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Start  *int   `json:"start,omitempty"`
	End    *int   `json:"end,omitempty"`
	Symbol string `json:"symbol,omitempty"`
}

type wireRule struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Text    string `json:"text"`
	Remedy  string `json:"remedy"`
	Closure string `json:"closure"`
	Class   string `json:"class,omitempty"`
	Tier    string `json:"tier,omitempty"`
}

type wireEvent struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Kind           string       `json:"kind"`
	FindingID      string       `json:"findingId"`
	Producer       wireProducer `json:"producer"`
	Subject        wireSubject  `json:"subject"`
	SourceID       string       `json:"sourceId"`
	SourceVersion  uint64       `json:"sourceVersion"`
	SourceRevision string       `json:"sourceRevision,omitempty"`
	PayloadDigest  string       `json:"payloadDigest"`
	ObservedAt     string       `json:"observedAt"`
	Outcome        string       `json:"outcome"`
	Coverage       string       `json:"coverage"`
	Anchor         *wireAnchor  `json:"anchor,omitempty"`
	SourceStatus   string       `json:"sourceStatus"`
	Rule           wireRule     `json:"rule"`
	Visibility     wireSubject  `json:"visibility"`
	Annotation     string       `json:"annotation,omitempty"`
}

type wireProjection struct {
	SchemaVersion int         `json:"schemaVersion"`
	Delivery      string      `json:"delivery"`
	Reason        string      `json:"reason,omitempty"`
	Target        string      `json:"target,omitempty"`
	Events        []wireEvent `json:"events"`
}

// validationError names the event index (or -1 for a projection-level
// error), the finding id (when known), and the field — every ParseProjection
// failure is one of these, formatted uniformly.
func validationError(index int, findingID, field, reason string) error {
	if index < 0 {
		return fmt.Errorf("findings projection: field %q: %s", field, reason)
	}
	return fmt.Errorf("findings projection: event[%d] (findingId=%s): field %q: %s", index, findingID, field, reason)
}

var (
	validKinds    = map[string]bool{"observation": true, "closure": true, "annotation": true, "revocation": true}
	validOutcomes = map[string]bool{"pass": true, "fail": true, "unknown": true}
	validCoverage = map[string]bool{"complete": true, "incomplete": true}
	validTiers    = map[string]bool{
		string(core.TierVerified): true, string(core.TierPreScan): true, string(core.TierDocClaimed): true,
		string(core.TierSMEStated): true, string(core.TierAssumed): true,
	}
)

// ParseProjection strictly decodes raw as a #FathomProjectionOutbox
// projection (schema_version 1): unknown fields refuse (json.Decoder.
// DisallowUnknownFields), an unsupported schema_version refuses naming the
// supported list, and every other validation failure names the event index,
// findingId, and field (FL-4). Nothing is written on ANY failure here — the
// caller (Importer.Import) only ever sees a fully-valid Projection.
func ParseProjection(raw []byte) (Projection, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var w wireProjection
	if err := dec.Decode(&w); err != nil {
		return Projection{}, fmt.Errorf("findings projection: decode: %w", err)
	}
	// A second value on the stream (trailing garbage) is also a refusal.
	if dec.More() {
		return Projection{}, fmt.Errorf("findings projection: decode: trailing data after the top-level object")
	}

	if w.SchemaVersion != 1 {
		return Projection{}, validationError(-1, "", "schemaVersion", fmt.Sprintf("unsupported schema_version %d (supported: [1])", w.SchemaVersion))
	}
	if w.Target != "" && w.Target != "fathom" {
		return Projection{}, validationError(-1, "", "target", fmt.Sprintf("unsupported target %q (supported: fathom, or omitted)", w.Target))
	}

	p := Projection{SchemaVersion: w.SchemaVersion, Delivery: w.Delivery, Reason: w.Reason, Target: w.Target}
	for i, we := range w.Events {
		ev, err := validateEvent(i, we)
		if err != nil {
			return Projection{}, err
		}
		p.Events = append(p.Events, ev)
	}
	return p, nil
}

func validateEvent(i int, we wireEvent) (Event, error) {
	fid := we.FindingID
	errf := func(field, reason string) error { return validationError(i, fid, field, reason) }

	if we.SchemaVersion != 1 {
		return Event{}, errf("schemaVersion", fmt.Sprintf("unsupported schema_version %d (supported: [1])", we.SchemaVersion))
	}
	if !validKinds[we.Kind] {
		return Event{}, errf("kind", fmt.Sprintf("unrecognized kind %q (want observation|closure|annotation|revocation)", we.Kind))
	}
	if !isHex64(we.FindingID) {
		return Event{}, errf("findingId", "must be 64 lowercase hex characters")
	}
	if we.Producer.Name == "" {
		return Event{}, errf("producer.name", "must not be empty")
	}
	if we.Subject.Organization == "" || we.Subject.Repository == "" || we.Subject.Component == "" {
		return Event{}, errf("subject", "organization, repository, and component must all be set")
	}
	if we.SourceID == "" {
		return Event{}, errf("sourceId", "must not be empty")
	}
	if we.SourceVersion < 1 {
		return Event{}, errf("sourceVersion", "must be >= 1")
	}
	if !isHex64(we.PayloadDigest) {
		return Event{}, errf("payloadDigest", "must be 64 lowercase hex characters")
	}
	observedAt, err := time.Parse(time.RFC3339, we.ObservedAt)
	if err != nil {
		return Event{}, errf("observedAt", "must be RFC3339: "+err.Error())
	}
	if !validOutcomes[we.Outcome] {
		return Event{}, errf("outcome", fmt.Sprintf("unrecognized outcome %q (want pass|fail|unknown)", we.Outcome))
	}
	if !validCoverage[we.Coverage] {
		return Event{}, errf("coverage", fmt.Sprintf("unrecognized coverage %q (want complete|incomplete)", we.Coverage))
	}
	if we.SourceStatus == "" {
		return Event{}, errf("sourceStatus", "must not be empty")
	}

	var anchor *Anchor
	if we.Anchor != nil {
		wa := we.Anchor
		if !validRelPath(wa.Path) {
			return Event{}, errf("anchor.path", "must be a repo-relative path with no leading \"/\" and no \"..\" segment")
		}
		if !isHex64(wa.Digest) {
			return Event{}, errf("anchor.digest", "must be 64 lowercase hex characters")
		}
		a := Anchor{Path: wa.Path, Digest: wa.Digest, Symbol: wa.Symbol}
		if wa.Start != nil {
			if *wa.Start < 1 {
				return Event{}, errf("anchor.start", "must be >= 1")
			}
			a.Start = *wa.Start
		}
		if wa.End != nil {
			if wa.Start == nil {
				return Event{}, errf("anchor.end", "must not be set without anchor.start")
			}
			if *wa.End < *wa.Start {
				return Event{}, errf("anchor.end", "must be >= anchor.start")
			}
			a.End = *wa.End
		}
		anchor = &a
	}

	if we.Rule.ID == "" {
		return Event{}, errf("rule.id", "must not be empty")
	}
	if we.Rule.Version == "" {
		return Event{}, errf("rule.version", "must not be empty")
	}
	if we.Rule.Text == "" {
		return Event{}, errf("rule.text", "must not be empty")
	}
	if we.Rule.Remedy == "" {
		return Event{}, errf("rule.remedy", "must not be empty")
	}
	if we.Rule.Closure == "" {
		return Event{}, errf("rule.closure", "must not be empty")
	}
	if we.Rule.Class != "" {
		valid := false
		for _, c := range core.ValidFindingClasses {
			if string(c) == we.Rule.Class {
				valid = true
				break
			}
		}
		if !valid {
			return Event{}, errf("rule.class", fmt.Sprintf("unrecognized class %q", we.Rule.Class))
		}
	}
	tier := we.Rule.Tier
	if tier == "" {
		// "absent means assumed" (INPUT CONTRACT) — normalized here, once,
		// so every downstream reader of Event.Rule.Tier sees a concrete
		// value.
		tier = string(core.TierAssumed)
	} else if !validTiers[tier] {
		return Event{}, errf("rule.tier", fmt.Sprintf("unrecognized tier %q", we.Rule.Tier))
	}

	if we.Visibility.Organization == "" || we.Visibility.Repository == "" || we.Visibility.Component == "" {
		return Event{}, errf("visibility", "organization, repository, and component must all be set")
	}
	subj := Subject{Organization: we.Subject.Organization, Repository: we.Subject.Repository, Component: we.Subject.Component}
	vis := Subject{Organization: we.Visibility.Organization, Repository: we.Visibility.Repository, Component: we.Visibility.Component}
	if vis != subj {
		return Event{}, errf("visibility", fmt.Sprintf("visibility %q must equal subject %q", vis, subj))
	}

	if we.Kind == "annotation" {
		if we.Annotation == "" {
			return Event{}, errf("annotation", "must not be empty for kind \"annotation\"")
		}
	} else if we.Annotation != "" {
		return Event{}, errf("annotation", "must be empty for kind != \"annotation\"")
	}

	return Event{
		SchemaVersion:  we.SchemaVersion,
		Kind:           we.Kind,
		FindingID:      we.FindingID,
		Producer:       Producer{Name: we.Producer.Name, Version: we.Producer.Version},
		Subject:        subj,
		SourceID:       we.SourceID,
		SourceVersion:  we.SourceVersion,
		SourceRevision: we.SourceRevision,
		PayloadDigest:  we.PayloadDigest,
		ObservedAt:     observedAt.UTC(),
		Outcome:        we.Outcome,
		Coverage:       we.Coverage,
		Anchor:         anchor,
		SourceStatus:   we.SourceStatus,
		Rule: Rule{
			ID: we.Rule.ID, Version: we.Rule.Version, Text: we.Rule.Text,
			Remedy: we.Rule.Remedy, Closure: we.Rule.Closure, Class: we.Rule.Class, Tier: tier,
		},
		Visibility: vis,
		Annotation: we.Annotation,
	}, nil
}
