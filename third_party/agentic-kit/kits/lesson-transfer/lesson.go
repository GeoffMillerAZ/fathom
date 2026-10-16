package lessontransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const (
	maxLessonBytes         = 64 << 10
	maxReproducerBytes     = 256 << 10
	maxAttestationBytes    = 64 << 10
	maxOutcomeDetailBytes  = 4 << 10
	maxJSONNesting         = 32
	payloadDigestDomain    = "agentic-kit.lesson-transfer.payload/v1"
	reproducerDigestDomain = "agentic-kit.lesson-transfer.reproducer/v1"
)

// ValidateBundle validates strict, bounded public-synthetic payload, reproducer, and
// attestation bytes. Digest inputs contain an actual NUL domain separator, not the
// four printable characters "\\x00".
func ValidateBundle(lesson, reproducer, attestations []byte) (Bundle, error) {
	var payload Payload
	var att Attestations
	var repro Reproducer
	if err := decodeStrict(lesson, maxLessonBytes, &payload); err != nil {
		return Bundle{}, err
	}
	if err := decodeStrict(reproducer, maxReproducerBytes, &repro); err != nil {
		return Bundle{}, err
	}
	if err := decodeStrict(attestations, maxAttestationBytes, &att); err != nil {
		return Bundle{}, err
	}
	payloadDigest, reproducerDigest := digest(payloadDigestDomain, lesson), digest(reproducerDigestDomain, reproducer)
	if att.PayloadDigest != payloadDigest || att.ReproducerDigest != reproducerDigest {
		return Bundle{}, fmt.Errorf("%w: attestation byte bindings differ", ErrInvalid)
	}
	if err := validatePayload(payload, reproducerDigest); err != nil {
		return Bundle{}, err
	}
	if len(reproducer) > payload.Reproducer.MaxBytes || repro.Schema != payload.Reproducer.Schema || repro.Version != payload.Reproducer.Version || repro.Expected != payload.Reproducer.Expected || strings.TrimSpace(repro.Invariant) == "" || !validSyntheticInput(repro.Input) {
		return Bundle{}, fmt.Errorf("%w: reproducer is not the declared bounded synthetic shape", ErrInvalid)
	}
	if strings.TrimSpace(att.Reviewer) == "" || strings.TrimSpace(att.Scope) == "" {
		return Bundle{}, fmt.Errorf("%w: bounded descriptive attestation required", ErrInvalid)
	}
	if parsed, err := time.Parse(time.RFC3339, att.ReviewedAt); err != nil || parsed.IsZero() {
		return Bundle{}, fmt.Errorf("%w: reviewed_at must be RFC3339", ErrInvalid)
	}
	return Bundle{Payload: clonePayload(payload), Reproducer: cloneReproducer(repro), Attestations: att, PayloadDigest: payloadDigest, ReproducerDigest: reproducerDigest, payloadRaw: append([]byte(nil), lesson...), reproducerRaw: append([]byte(nil), reproducer...), attestationsRaw: append([]byte(nil), attestations...)}, nil
}

// CheckAdoption uses time.Now as its documented production clock default.
func CheckAdoption(ctx context.Context, bundle Bundle, receiver ReceiverSnapshot, accept ReviewAcceptance, current CurrentReceiver, revocation CurrentRevocation, run Reproduce) (AdoptionReceipt, error) {
	return CheckAdoptionAt(ctx, bundle, receiver, accept, current, revocation, run, time.Now)
}

// CheckAdoptionAt performs a receiver-owned adoption check with an injectable clock.
func CheckAdoptionAt(ctx context.Context, bundle Bundle, receiver ReceiverSnapshot, accept ReviewAcceptance, current CurrentReceiver, revocation CurrentRevocation, run Reproduce, now func() time.Time) (AdoptionReceipt, error) {
	if ctx == nil || accept == nil || current == nil || revocation == nil || run == nil || now == nil {
		return AdoptionReceipt{}, fmt.Errorf("%w: required receiver callback missing", ErrUntrusted)
	}
	if err := ctx.Err(); err != nil {
		return AdoptionReceipt{}, err
	}
	b, receiver := cloneBundle(bundle), receiver
	var err error
	if b, err = trustedBundle(b); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := validateReceiver(receiver); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := applicable(b.Payload, receiver); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := adoptionEligible(b.Payload); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := checkCurrent(ctx, b, receiver, current, revocation); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := accept(ctx, cloneBundle(b), receiver); err != nil {
		return AdoptionReceipt{}, fmt.Errorf("%w: %v", ErrUntrusted, err)
	}
	if err := ctx.Err(); err != nil {
		return AdoptionReceipt{}, err
	}
	outcome, err := run(ctx, cloneBundle(b), receiver)
	if err != nil {
		return AdoptionReceipt{}, err
	}
	outcome = cloneOutcome(outcome)
	if outcome.Classification != b.Payload.Reproducer.Expected || !validDigest(outcome.Evidence) || strings.TrimSpace(outcome.Detail) == "" || len(outcome.Detail) > maxOutcomeDetailBytes || !utf8.ValidString(outcome.Detail) {
		return AdoptionReceipt{}, fmt.Errorf("%w: reproduction outcome is not independently expected", ErrInvalid)
	}
	if err := checkCurrent(ctx, b, receiver, current, revocation); err != nil {
		return AdoptionReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return AdoptionReceipt{}, err
	}
	checkedAt := now().UTC()
	if checkedAt.IsZero() {
		return AdoptionReceipt{}, fmt.Errorf("%w: clock returned zero time", ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return AdoptionReceipt{}, err
	}
	return AdoptionReceipt{Version: Version, PayloadDigest: b.PayloadDigest, ReproducerDigest: b.ReproducerDigest, Receiver: receiver, Outcome: outcome, Integrity: "pass", Reproduction: "pass", Fresh: true, CheckedAt: checkedAt}, nil
}

func checkCurrent(ctx context.Context, b Bundle, want ReceiverSnapshot, current CurrentReceiver, revocation CurrentRevocation) error {
	seen, err := current(ctx, cloneReceiver(want))
	if err != nil || validateReceiver(seen) != nil || seen != want {
		return fmt.Errorf("%w: receiver snapshot is unavailable or changed", ErrInvalid)
	}
	status, err := revocation(ctx, cloneBundle(b))
	if err != nil || status.Status != "active" {
		return fmt.Errorf("%w: current revocation status is not active", ErrRevoked)
	}
	return ctx.Err()
}

func applicable(p Payload, receiver ReceiverSnapshot) error {
	if !contains(p.Applicability.Workloads, receiver.Workload) || contains(p.Exclusions, receiver.Workload) {
		return fmt.Errorf("%w: receiver is excluded or outside applicability", ErrInvalid)
	}
	if len(p.Applicability.ConfigDigests) != 0 && !contains(p.Applicability.ConfigDigests, receiver.ConfigDigest) {
		return fmt.Errorf("%w: receiver configuration is outside applicability", ErrInvalid)
	}
	return nil
}

// ValidateApplicability checks a validated bundle against a local receiver
// identity without running review, revocation, or reproduction callbacks.
func ValidateApplicability(bundle Bundle, receiver ReceiverSnapshot) error {
	b, err := trustedBundle(cloneBundle(bundle))
	if err != nil {
		return err
	}
	if err := validateReceiver(receiver); err != nil {
		return err
	}
	return applicable(b.Payload, receiver)
}

func adoptionEligible(p Payload) error {
	if p.Status == "revoked" || p.Status == "superseded" || p.Revocation.Status == "revoked" {
		return fmt.Errorf("%w: lesson lifecycle is no longer active", ErrRevoked)
	}
	if p.Status != "reviewed" || p.Revocation.Status != "active" {
		return fmt.Errorf("%w: lesson is not reviewed and active", ErrUntrusted)
	}
	return nil
}

func validateBundleValue(b Bundle) error {
	_, err := trustedBundle(b)
	return err
}

// trustedBundle recreates callback inputs from raw bytes retained by validation.
// Mutating exported display fields after ValidateBundle cannot alter the check.
func trustedBundle(b Bundle) (Bundle, error) {
	if len(b.payloadRaw) == 0 || len(b.reproducerRaw) == 0 || len(b.attestationsRaw) == 0 || digest(payloadDigestDomain, b.payloadRaw) != b.PayloadDigest || digest(reproducerDigestDomain, b.reproducerRaw) != b.ReproducerDigest {
		return Bundle{}, fmt.Errorf("%w: missing or changed validation proof", ErrInvalid)
	}
	fresh, err := ValidateBundle(b.payloadRaw, b.reproducerRaw, b.attestationsRaw)
	if err != nil || fresh.PayloadDigest != b.PayloadDigest || fresh.ReproducerDigest != b.ReproducerDigest || !reflect.DeepEqual(b.Payload, fresh.Payload) || !reflect.DeepEqual(b.Reproducer, fresh.Reproducer) || !reflect.DeepEqual(b.Attestations, fresh.Attestations) {
		return Bundle{}, fmt.Errorf("%w: validation proof no longer binds payload", ErrInvalid)
	}
	return fresh, nil
}

func validateReceiver(s ReceiverSnapshot) error {
	if strings.TrimSpace(s.Workload) == "" || !validDigest(s.Revision) || !validDigest(s.ConfigDigest) {
		return fmt.Errorf("%w: receiver snapshot requires workload and digest revisions", ErrInvalid)
	}
	return nil
}

// ValidateReceiverSnapshot checks the local identity binding required before a
// receiver can record an adoption result. It does not authenticate that a
// caller obtained the values from a particular host.
func ValidateReceiverSnapshot(s ReceiverSnapshot) error {
	return validateReceiver(s)
}

// ParseReceiverSnapshot strictly decodes a bounded receiver assertion and
// validates its local digest bindings. It does not authenticate their origin.
func ParseReceiverSnapshot(raw []byte) (ReceiverSnapshot, error) {
	var snapshot ReceiverSnapshot
	if err := decodeStrict(raw, maxAttestationBytes, &snapshot); err != nil {
		return ReceiverSnapshot{}, err
	}
	if err := validateReceiver(snapshot); err != nil {
		return ReceiverSnapshot{}, err
	}
	return snapshot, nil
}

// ParseReproductionOutcome strictly decodes a bounded receiver-supplied
// outcome. A caller must still use CheckAdoption to establish it independently.
func ParseReproductionOutcome(raw []byte) (ReproductionOutcome, error) {
	var outcome ReproductionOutcome
	if err := decodeStrict(raw, maxAttestationBytes, &outcome); err != nil {
		return ReproductionOutcome{}, err
	}
	if (outcome.Classification != "unknown" && outcome.Classification != "fail" && outcome.Classification != "pass") || !validDigest(outcome.Evidence) || strings.TrimSpace(outcome.Detail) == "" || len(outcome.Detail) > maxOutcomeDetailBytes || !utf8.ValidString(outcome.Detail) {
		return ReproductionOutcome{}, fmt.Errorf("%w: invalid reproduction assertion", ErrInvalid)
	}
	return outcome, nil
}

func validatePayload(p Payload, reproducerDigest string) error {
	if p.Version != Version || p.Revision <= 0 || !validLifecycleStatus(p.Status) || p.Classification != "public-synthetic" || !validRevocationStatus(p.Revocation.Status) || !p.Revocation.Recheck || strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Origin.Workload) == "" || !validDigest(p.Origin.SnapshotDigest) || !validDigest(p.Origin.IncidentDigest) || !validDigest(p.Origin.ScenarioDigest) || len(p.Applicability.Workloads) == 0 || p.Applicability.ConfigDigests == nil || p.Exclusions == nil || p.Supersedes == nil || p.Reproducer.Schema != "lesson-reproducer/v1" || p.Reproducer.Version != 1 || p.Reproducer.File != "reproducer.json" || p.Reproducer.RawDigest != reproducerDigest || p.Reproducer.MaxBytes <= 0 || p.Reproducer.MaxBytes > maxReproducerBytes || (p.Reproducer.Expected != "unknown" && p.Reproducer.Expected != "fail") || len(p.Remedies) == 0 {
		return fmt.Errorf("%w: payload shape or lifecycle", ErrInvalid)
	}
	for _, list := range [][]string{p.Applicability.Workloads, p.Applicability.ConfigDigests, p.Exclusions, p.Supersedes} {
		if !uniqueNonempty(list) {
			return fmt.Errorf("%w: duplicate or empty payload value", ErrInvalid)
		}
	}
	for _, digest := range p.Applicability.ConfigDigests {
		if !validDigest(digest) {
			return fmt.Errorf("%w: configuration digest", ErrInvalid)
		}
	}
	remedies := make(map[string]struct{}, len(p.Remedies))
	for _, remedy := range p.Remedies {
		if strings.TrimSpace(remedy.ID) == "" || remedy.Revision <= 0 || !validDigest(remedy.Digest) || !validDigest(remedy.TestDigest) {
			return fmt.Errorf("%w: remedy binding", ErrInvalid)
		}
		if _, exists := remedies[remedy.ID]; exists {
			return fmt.Errorf("%w: duplicate remedy ID", ErrInvalid)
		}
		remedies[remedy.ID] = struct{}{}
	}
	return nil
}

func validSyntheticInput(input SyntheticInput) bool {
	return input.Case == "unknown-family" || input.Case == "unobserved-control"
}

func validLifecycleStatus(status string) bool {
	return status == "candidate" || status == "reviewed" || status == "superseded" || status == "revoked"
}

func validRevocationStatus(status string) bool {
	return status == "active" || status == "revoked"
}

func digest(domain string, raw []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}
func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && s == strings.ToLower(s)
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func uniqueNonempty(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if strings.TrimSpace(v) == "" || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func decodeStrict(raw []byte, limit int, out any) error {
	if err := validateJSON(raw, limit); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("%w: JSON shape", ErrInvalid)
	}
	return nil
}
func validateJSON(raw []byte, limit int) error {
	if len(raw) == 0 || len(raw) > limit || !jsonwire.ValidUnicode(raw) {
		return fmt.Errorf("%w: JSON bound or Unicode", ErrInvalid)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := validateJSONValue(d, 0); err != nil {
		return fmt.Errorf("%w: JSON syntax", ErrInvalid)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%w: trailing JSON", ErrInvalid)
	}
	return nil
}
func validateJSONValue(d *json.Decoder, depth int) error {
	if depth > maxJSONNesting {
		return fmt.Errorf("depth")
	}
	tok, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return fmt.Errorf("delimiter")
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			s = strings.ToLower(s)
			if !ok || seen[s] {
				return fmt.Errorf("duplicate")
			}
			seen[s] = true
		}
		if err := validateJSONValue(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func clonePayload(in Payload) Payload {
	out := in
	out.Applicability.Workloads = cloneStrings(in.Applicability.Workloads)
	out.Applicability.ConfigDigests = cloneStrings(in.Applicability.ConfigDigests)
	out.Exclusions = cloneStrings(in.Exclusions)
	out.Remedies = cloneRemedies(in.Remedies)
	out.Supersedes = cloneStrings(in.Supersedes)
	return out
}
func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}
func cloneRemedies(in []RemedyRef) []RemedyRef {
	if in == nil {
		return nil
	}
	return append([]RemedyRef{}, in...)
}
func cloneReproducer(in Reproducer) Reproducer { return in }
func cloneBundle(in Bundle) Bundle {
	out := in
	out.Payload = clonePayload(in.Payload)
	out.Reproducer = cloneReproducer(in.Reproducer)
	out.payloadRaw = append([]byte(nil), in.payloadRaw...)
	out.reproducerRaw = append([]byte(nil), in.reproducerRaw...)
	out.attestationsRaw = append([]byte(nil), in.attestationsRaw...)
	return out
}
func cloneReceiver(in ReceiverSnapshot) ReceiverSnapshot      { return in }
func cloneOutcome(in ReproductionOutcome) ReproductionOutcome { return in }
